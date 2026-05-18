package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mlhiter/mbox/internal/config"
	"github.com/mlhiter/mbox/internal/controller"
	"github.com/mlhiter/mbox/internal/httpapi"
	"github.com/mlhiter/mbox/internal/postgres"
	"github.com/mlhiter/mbox/internal/runtime/agentsandbox"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return err
	}
	if err := postgres.Migrate(ctx, pool); err != nil {
		return err
	}

	store := postgres.NewStore(pool)
	if cfg.RuntimeControllerEnabled {
		restConfig, err := agentsandbox.BuildRESTConfig(agentsandbox.Config{
			KubeconfigPath: cfg.KubeconfigPath,
			Context:        cfg.KubeContext,
		})
		if err != nil {
			return err
		}
		adapter, err := agentsandbox.New(restConfig, agentsandbox.Config{
			WarmPoolPolicy: cfg.AgentSandboxWarmPool,
		})
		if err != nil {
			return err
		}
		reconciler := controller.NewSandboxReconciler(store, adapter, cfg.RuntimeReconcileInterval, slog.Default())
		go func() {
			if err := reconciler.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				slog.Error("sandbox reconciler exited", "error", err)
			}
		}()
	} else {
		slog.Info("sandbox runtime controller disabled")
	}

	api := httpapi.New(store)
	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           requestLogger(api),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("mbox server listening", "addr", cfg.ListenAddr)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start))
	})
}

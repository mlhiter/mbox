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
	mboxruntime "github.com/mlhiter/mbox/internal/runtime"
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
	var runtimeAccess mboxruntime.Access
	var runtimeAuditor mboxruntime.Auditor
	var runtimeCleaner mboxruntime.Cleaner
	if cfg.RuntimeControllerEnabled || cfg.RuntimeAccessEnabled {
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
		runtimeAuditor = adapter
		runtimeCleaner = adapter
		if cfg.RuntimeAccessEnabled {
			runtimeAccess = adapter
		} else {
			slog.Info("sandbox runtime access disabled")
		}
		if cfg.RuntimeControllerEnabled {
			reconciler := controller.NewSandboxReconciler(store, adapter, cfg.RuntimeReconcileInterval, slog.Default())
			go func() {
				if err := reconciler.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
					slog.Error("sandbox reconciler exited", "error", err)
				}
			}()
		} else {
			slog.Info("sandbox runtime controller disabled")
		}
	} else {
		slog.Info("sandbox runtime controller disabled")
		slog.Info("sandbox runtime access disabled")
	}

	var artifactContentBackend httpapi.ArtifactContentBackend
	switch cfg.ArtifactContentBackend {
	case "filesystem":
		artifactContentBackend, err = httpapi.NewFilesystemArtifactContentBackend(cfg.ArtifactContentDir)
		if err != nil {
			return err
		}
		slog.Info("artifact content backend enabled", "backend", cfg.ArtifactContentBackend, "dir", cfg.ArtifactContentDir)
	case "s3":
		artifactContentBackend, err = httpapi.NewS3ArtifactContentBackend(httpapi.S3ArtifactContentBackendOptions{
			Endpoint:        cfg.ArtifactContentS3.Endpoint,
			Region:          cfg.ArtifactContentS3.Region,
			Bucket:          cfg.ArtifactContentS3.Bucket,
			Prefix:          cfg.ArtifactContentS3.Prefix,
			AccessKeyID:     cfg.ArtifactContentS3.AccessKeyID,
			SecretAccessKey: cfg.ArtifactContentS3.SecretAccessKey,
			ForcePathStyle:  cfg.ArtifactContentS3.ForcePathStyle,
		})
		if err != nil {
			return err
		}
		slog.Info("artifact content backend enabled", "backend", cfg.ArtifactContentBackend, "endpoint", cfg.ArtifactContentS3.Endpoint, "bucket", cfg.ArtifactContentS3.Bucket, "prefix", cfg.ArtifactContentS3.Prefix)
	default:
		slog.Info("artifact content backend enabled", "backend", cfg.ArtifactContentBackend)
	}

	api := httpapi.NewWithOptions(store, httpapi.Options{
		RuntimeAccess:          runtimeAccess,
		RuntimeAuditor:         runtimeAuditor,
		RuntimeCleaner:         runtimeCleaner,
		ArtifactContentBackend: artifactContentBackend,
		APIToken:               cfg.APIToken,
		Info: httpapi.InfoOptions{
			ServerVersion:            cfg.ServerVersion,
			RuntimeControllerEnabled: cfg.RuntimeControllerEnabled,
			RuntimeAccessEnabled:     cfg.RuntimeAccessEnabled,
			RuntimeAdapter:           "agent-sandbox",
			ArtifactStorageProvider:  cfg.ArtifactContentBackend,
		},
	})
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
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "request_id", w.Header().Get(httpapi.RequestIDHeader), "duration", time.Since(start))
	})
}

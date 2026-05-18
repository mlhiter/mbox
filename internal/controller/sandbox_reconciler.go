package controller

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/mlhiter/mbox/internal/domain"
	mboxruntime "github.com/mlhiter/mbox/internal/runtime"
)

type SandboxReconciler struct {
	store    domain.Store
	adapter  mboxruntime.Adapter
	interval time.Duration
	logger   *slog.Logger
}

func NewSandboxReconciler(store domain.Store, adapter mboxruntime.Adapter, interval time.Duration, logger *slog.Logger) *SandboxReconciler {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &SandboxReconciler{
		store:    store,
		adapter:  adapter,
		interval: interval,
		logger:   logger,
	}
}

func (r *SandboxReconciler) Run(ctx context.Context) error {
	r.logger.Info("sandbox reconciler started", "interval", r.interval)
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	if err := r.Reconcile(ctx); err != nil {
		r.logger.Error("initial sandbox reconcile failed", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("sandbox reconciler stopped")
			return ctx.Err()
		case <-ticker.C:
			if err := r.Reconcile(ctx); err != nil {
				r.logger.Error("sandbox reconcile failed", "error", err)
			}
		}
	}
}

func (r *SandboxReconciler) Reconcile(ctx context.Context) error {
	sandboxes, err := r.store.ListSandboxesForReconcile(ctx)
	if err != nil {
		return err
	}
	for _, sandbox := range sandboxes {
		if err := r.reconcileSandbox(ctx, sandbox); err != nil {
			r.logger.Error("sandbox reconcile item failed", "sandbox_id", sandbox.ID, "error", err)
		}
	}
	return nil
}

func (r *SandboxReconciler) reconcileSandbox(ctx context.Context, sandbox domain.Sandbox) error {
	if sandbox.DeletedAt != nil {
		return r.reconcileDeleted(ctx, sandbox)
	}

	if sandbox.RuntimeRef == nil {
		return r.createRuntime(ctx, sandbox)
	}

	status, err := r.adapter.GetRuntimeStatus(ctx, *sandbox.RuntimeRef)
	if err != nil {
		return err
	}
	return r.applyRuntimeStatus(ctx, sandbox, status)
}

func (r *SandboxReconciler) createRuntime(ctx context.Context, sandbox domain.Sandbox) error {
	template, err := r.store.GetTemplate(ctx, sandbox.TemplateID)
	if err != nil {
		return err
	}
	ref, err := r.adapter.CreateRuntime(ctx, mboxruntime.CreateRequest{
		Sandbox:  sandbox,
		Template: template,
	})
	if err != nil {
		failed := domain.SandboxStatusFailed
		_, updateErr := r.store.UpdateSandbox(ctx, sandbox.ID, domain.SandboxUpdate{Status: &failed})
		if updateErr != nil {
			return errors.Join(err, updateErr)
		}
		return err
	}
	pending := domain.SandboxStatusPending
	runtimeRef := &ref
	_, err = r.store.UpdateSandbox(ctx, sandbox.ID, domain.SandboxUpdate{
		Status:     &pending,
		RuntimeRef: &runtimeRef,
	})
	return err
}

func (r *SandboxReconciler) applyRuntimeStatus(ctx context.Context, sandbox domain.Sandbox, status mboxruntime.Status) error {
	next := sandbox.Status
	switch status.Status {
	case mboxruntime.RuntimeStatusRunning:
		next = domain.SandboxStatusRunning
	case mboxruntime.RuntimeStatusFailed:
		next = domain.SandboxStatusFailed
	case mboxruntime.RuntimeStatusDeleted:
		return r.store.MarkSandboxRuntimeDeleted(ctx, sandbox.ID)
	default:
		next = domain.SandboxStatusPending
	}

	_, err := r.store.UpdateSandbox(ctx, sandbox.ID, domain.SandboxUpdate{
		Status: &next,
	})
	return err
}

func (r *SandboxReconciler) reconcileDeleted(ctx context.Context, sandbox domain.Sandbox) error {
	if sandbox.RuntimeRef == nil {
		return nil
	}
	if err := r.adapter.DeleteRuntime(ctx, *sandbox.RuntimeRef); err != nil {
		return err
	}
	return r.store.MarkSandboxRuntimeDeleted(ctx, sandbox.ID)
}

package runtime

import (
	"context"

	"github.com/mlhiter/mbox/internal/domain"
)

type CreateRequest struct {
	Sandbox  domain.Sandbox
	Template domain.EnvironmentTemplate
}

type RuntimeStatus string

const (
	RuntimeStatusPending RuntimeStatus = "pending"
	RuntimeStatusRunning RuntimeStatus = "running"
	RuntimeStatusFailed  RuntimeStatus = "failed"
	RuntimeStatusDeleted RuntimeStatus = "deleted"
)

type Status struct {
	Status     RuntimeStatus
	RuntimeRef domain.RuntimeRef
	Ports      []domain.SandboxPort
	Message    string
}

type Adapter interface {
	CreateRuntime(ctx context.Context, request CreateRequest) (domain.RuntimeRef, error)
	DeleteRuntime(ctx context.Context, ref domain.RuntimeRef) error
	GetRuntimeStatus(ctx context.Context, ref domain.RuntimeRef) (Status, error)
}

type NoopAdapter struct{}

func (NoopAdapter) CreateRuntime(context.Context, CreateRequest) (domain.RuntimeRef, error) {
	return domain.RuntimeRef{}, nil
}

func (NoopAdapter) DeleteRuntime(context.Context, domain.RuntimeRef) error {
	return nil
}

func (NoopAdapter) GetRuntimeStatus(context.Context, domain.RuntimeRef) (Status, error) {
	return Status{Status: RuntimeStatusPending}, nil
}

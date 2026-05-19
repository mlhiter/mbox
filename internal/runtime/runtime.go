package runtime

import (
	"context"
	"io"
	"time"

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

type RuntimeTarget struct {
	Namespace string           `json:"namespace"`
	PodName   string           `json:"podName"`
	Container string           `json:"container"`
	Phase     string           `json:"phase"`
	Selector  string           `json:"selector"`
	Commands  []string         `json:"commands,omitempty"`
	Storage   []RuntimeStorage `json:"storage,omitempty"`
}

type RuntimeStorage struct {
	Name             string `json:"name"`
	MountPath        string `json:"mountPath"`
	ClaimName        string `json:"claimName,omitempty"`
	Phase            string `json:"phase,omitempty"`
	Capacity         string `json:"capacity,omitempty"`
	StorageClassName string `json:"storageClassName,omitempty"`
	Message          string `json:"message,omitempty"`
}

type PreviewPort struct {
	Name       string `json:"name"`
	Port       int    `json:"port"`
	Protocol   string `json:"protocol"`
	PreviewURL string `json:"previewUrl,omitempty"`
	Available  bool   `json:"available"`
	Message    string `json:"message,omitempty"`
}

type PreviewPortsResult struct {
	Target RuntimeTarget `json:"target"`
	Items  []PreviewPort `json:"items"`
}

type PreviewProxyRequest struct {
	Port  int
	Path  string
	Query string
}

type PreviewProxyResponse struct {
	StatusCode int
	Header     map[string][]string
	Body       io.ReadCloser
}

type LogOptions struct {
	Container string
	TailLines int64
}

type LogResult struct {
	Target RuntimeTarget `json:"target"`
	Logs   string        `json:"logs"`
}

type RuntimeEvent struct {
	Type           string    `json:"type,omitempty"`
	Reason         string    `json:"reason,omitempty"`
	Message        string    `json:"message,omitempty"`
	Count          int32     `json:"count,omitempty"`
	FirstTimestamp time.Time `json:"firstTimestamp,omitempty"`
	LastTimestamp  time.Time `json:"lastTimestamp,omitempty"`
}

type ExecOptions struct {
	Container string
	Command   []string
	Stdin     io.Reader
	Stdout    io.Writer
	Stderr    io.Writer
	TTY       bool
}

type Adapter interface {
	CreateRuntime(ctx context.Context, request CreateRequest) (domain.RuntimeRef, error)
	DeleteRuntime(ctx context.Context, ref domain.RuntimeRef) error
	GetRuntimeStatus(ctx context.Context, ref domain.RuntimeRef) (Status, error)
}

type Access interface {
	ResolveRuntime(ctx context.Context, ref domain.RuntimeRef) (RuntimeTarget, error)
	ReadLogs(ctx context.Context, ref domain.RuntimeRef, options LogOptions) (LogResult, error)
	ListEvents(ctx context.Context, ref domain.RuntimeRef) ([]RuntimeEvent, error)
	ProxyPreview(ctx context.Context, ref domain.RuntimeRef, request PreviewProxyRequest) (PreviewProxyResponse, error)
	Exec(ctx context.Context, ref domain.RuntimeRef, options ExecOptions) error
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

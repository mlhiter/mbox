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

type FileReadRequest struct {
	Path          string
	MaxBytes      int64
	WorkspaceOnly bool
}

type FileReadResult struct {
	Target      RuntimeTarget `json:"target"`
	Path        string        `json:"path"`
	ContentType string        `json:"contentType,omitempty"`
	SizeBytes   int64         `json:"sizeBytes"`
	Truncated   bool          `json:"truncated"`
	Body        io.ReadCloser `json:"-"`
}

type RuntimeEvent struct {
	Type           string    `json:"type,omitempty"`
	Reason         string    `json:"reason,omitempty"`
	Message        string    `json:"message,omitempty"`
	Count          int32     `json:"count,omitempty"`
	FirstTimestamp time.Time `json:"firstTimestamp,omitempty"`
	LastTimestamp  time.Time `json:"lastTimestamp,omitempty"`
}

type ManagedResource struct {
	Adapter   string                `json:"adapter"`
	Kind      string                `json:"kind"`
	Namespace string                `json:"namespace,omitempty"`
	Name      string                `json:"name"`
	Owner     *ManagedResourceOwner `json:"owner,omitempty"`
	Labels    map[string]string     `json:"labels,omitempty"`
	CreatedAt time.Time             `json:"createdAt,omitempty"`
}

type ManagedResourceList struct {
	Adapter   string                 `json:"adapter"`
	CheckedAt time.Time              `json:"checkedAt"`
	Summary   ManagedResourceSummary `json:"summary"`
	Items     []ManagedResource      `json:"items"`
}

type ManagedResourceSummary struct {
	Total       int                    `json:"total"`
	ByKind      []ManagedResourceCount `json:"byKind"`
	ByNamespace []ManagedResourceCount `json:"byNamespace"`
	ByOwner     []ManagedResourceCount `json:"byOwner"`
}

type ManagedResourceCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type ManagedResourceOwner struct {
	Kind       string `json:"kind"`
	ProjectID  string `json:"projectId,omitempty"`
	SandboxID  string `json:"sandboxId,omitempty"`
	TemplateID string `json:"templateId,omitempty"`
}

type ManagedResourceRef struct {
	Adapter   string `json:"adapter"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
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
	StartRuntime(ctx context.Context, ref domain.RuntimeRef) error
	StopRuntime(ctx context.Context, ref domain.RuntimeRef) error
	DeleteRuntime(ctx context.Context, ref domain.RuntimeRef) error
	GetRuntimeStatus(ctx context.Context, ref domain.RuntimeRef) (Status, error)
}

type Access interface {
	ResolveRuntime(ctx context.Context, ref domain.RuntimeRef) (RuntimeTarget, error)
	ReadLogs(ctx context.Context, ref domain.RuntimeRef, options LogOptions) (LogResult, error)
	ListEvents(ctx context.Context, ref domain.RuntimeRef) ([]RuntimeEvent, error)
	ProxyPreview(ctx context.Context, ref domain.RuntimeRef, request PreviewProxyRequest) (PreviewProxyResponse, error)
	Exec(ctx context.Context, ref domain.RuntimeRef, options ExecOptions) error
	ReadFile(ctx context.Context, ref domain.RuntimeRef, request FileReadRequest) (FileReadResult, error)
}

type Auditor interface {
	ListManagedResources(ctx context.Context) (ManagedResourceList, error)
}

type Cleaner interface {
	DeleteManagedResource(ctx context.Context, ref ManagedResourceRef) error
}

type NoopAdapter struct{}

func (NoopAdapter) CreateRuntime(context.Context, CreateRequest) (domain.RuntimeRef, error) {
	return domain.RuntimeRef{}, nil
}

func (NoopAdapter) DeleteRuntime(context.Context, domain.RuntimeRef) error {
	return nil
}

func (NoopAdapter) StartRuntime(context.Context, domain.RuntimeRef) error {
	return nil
}

func (NoopAdapter) StopRuntime(context.Context, domain.RuntimeRef) error {
	return nil
}

func (NoopAdapter) GetRuntimeStatus(context.Context, domain.RuntimeRef) (Status, error) {
	return Status{Status: RuntimeStatusPending}, nil
}

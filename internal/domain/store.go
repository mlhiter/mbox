package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ProjectCreate struct {
	Name             string
	Slug             string
	RepositoryURL    string
	DefaultNamespace string
	Metadata         []byte
}

type ProjectUpdate struct {
	Name              *string
	RepositoryURL     *string
	DefaultNamespace  *string
	DefaultTemplateID **uuid.UUID
	Metadata          *[]byte
}

type ProjectPolicyUpsert struct {
	Enforcement            ProjectPolicyEnforcement
	AllowedImagePrefixes   []string
	AllowedServiceAccounts []string
	AllowedSecretRefs      []string
}

type ProjectQuotaPolicyUpsert struct {
	Enforcement              ProjectQuotaPolicyEnforcement
	MaxActiveSandboxes       *int
	MaxRetainedArtifactBytes *int64
}

type ProjectCredentialCreate struct {
	ProjectID uuid.UUID
	Name      string
	Slug      string
	Type      ProjectCredentialType
	Target    string
	SecretRef SecretRef
	Usage     []string
	Metadata  []byte
}

type TemplateCreate struct {
	ProjectID       *uuid.UUID
	Name            string
	Slug            string
	Image           string
	StartupCommand  []string
	WorkingDir      string
	CPURequest      string
	MemoryRequest   string
	StorageRequest  string
	ExposedPorts    []TemplatePort
	Env             []byte
	SecretRefs      []SecretRef
	NetworkPolicy   string
	LifecyclePolicy []byte
	Metadata        []byte
}

type TemplateUpdate struct {
	Name            *string
	Image           *string
	StartupCommand  *[]string
	WorkingDir      *string
	CPURequest      *string
	MemoryRequest   *string
	StorageRequest  *string
	ExposedPorts    *[]TemplatePort
	Env             *[]byte
	SecretRefs      *[]SecretRef
	NetworkPolicy   *string
	LifecyclePolicy *[]byte
	Metadata        *[]byte
}

type SandboxCreate struct {
	ProjectID          uuid.UUID
	TemplateID         uuid.UUID
	Name               string
	Slug               string
	Namespace          string
	ServiceAccountName string
	Ports              []SandboxPort
	Metadata           []byte
}

type SandboxUpdate struct {
	Name               *string
	Status             *SandboxStatus
	Namespace          *string
	ServiceAccountName *string
	RuntimeRef         **RuntimeRef
	Ports              *[]SandboxPort
	Metadata           *[]byte
}

type ExecutionTaskCreate struct {
	ProjectID      uuid.UUID
	SandboxID      uuid.UUID
	Command        []string
	TimeoutSeconds int
	RuntimeRef     *RuntimeRef
	Metadata       []byte
}

type RuntimeSessionCreate struct {
	ProjectID  uuid.UUID
	SandboxID  uuid.UUID
	Type       RuntimeSessionType
	Client     string
	UserAgent  string
	RuntimeRef *RuntimeRef
	Metadata   []byte
}

type RuntimeSessionUpdate struct {
	Status  *RuntimeSessionStatus
	EndedAt *time.Time
}

type ExecutionTaskUpdate struct {
	Status          *ExecutionTaskStatus
	ExitCode        *int
	Stdout          *string
	Stderr          *string
	OutputTruncated *bool
	Error           *string
	RuntimeRef      **RuntimeRef
	StartedAt       *time.Time
	FinishedAt      *time.Time
}

type ArtifactCreate struct {
	ProjectID   uuid.UUID
	SandboxID   uuid.UUID
	TaskID      *uuid.UUID
	Kind        ArtifactKind
	Name        string
	URI         string
	ContentType string
	SizeBytes   *int64
	Metadata    []byte
}

type ArtifactContentCapture struct {
	ArtifactID      uuid.UUID
	Content         []byte
	ContentType     string
	SizeBytes       int64
	SHA256          string
	SourceURI       string
	StorageProvider ArtifactContentStorageProvider
	StorageKey      string
}

type AuditEventCreate struct {
	ProjectID    *uuid.UUID
	Action       string
	ResourceType string
	ResourceID   *uuid.UUID
	ResourceName string
	Actor        string
	Source       string
	Metadata     []byte
}

type AuditEventFilter struct {
	ProjectID    *uuid.UUID
	Action       string
	ResourceType string
	ResourceID   *uuid.UUID
	Actor        string
	Source       string
	RequestID    string
	Operation    string
	Since        *time.Time
	Until        *time.Time
	Limit        int
}

type Store interface {
	ListProjects(ctx context.Context) ([]Project, error)
	CreateProject(ctx context.Context, input ProjectCreate) (Project, error)
	GetProject(ctx context.Context, id uuid.UUID) (Project, error)
	UpdateProject(ctx context.Context, id uuid.UUID, input ProjectUpdate) (Project, error)
	DeleteProject(ctx context.Context, id uuid.UUID) error
	GetProjectPolicy(ctx context.Context, projectID uuid.UUID) (ProjectPolicy, error)
	UpsertProjectPolicy(ctx context.Context, projectID uuid.UUID, input ProjectPolicyUpsert) (ProjectPolicy, error)
	GetProjectQuotaPolicy(ctx context.Context, projectID uuid.UUID) (ProjectQuotaPolicy, error)
	UpsertProjectQuotaPolicy(ctx context.Context, projectID uuid.UUID, input ProjectQuotaPolicyUpsert) (ProjectQuotaPolicy, error)
	ListProjectCredentials(ctx context.Context, projectID uuid.UUID) ([]ProjectCredential, error)
	CreateProjectCredential(ctx context.Context, input ProjectCredentialCreate) (ProjectCredential, error)
	GetProjectCredential(ctx context.Context, id uuid.UUID) (ProjectCredential, error)
	DeleteProjectCredential(ctx context.Context, id uuid.UUID) error
	GetProjectUsage(ctx context.Context, projectID uuid.UUID) (ProjectUsage, error)
	ListAuditEvents(ctx context.Context, filter AuditEventFilter) ([]AuditEvent, error)
	CreateAuditEvent(ctx context.Context, input AuditEventCreate) (AuditEvent, error)

	ListTemplates(ctx context.Context, projectID *uuid.UUID) ([]EnvironmentTemplate, error)
	ListAllTemplates(ctx context.Context) ([]EnvironmentTemplate, error)
	CreateTemplate(ctx context.Context, input TemplateCreate) (EnvironmentTemplate, error)
	GetTemplate(ctx context.Context, id uuid.UUID) (EnvironmentTemplate, error)
	UpdateTemplate(ctx context.Context, id uuid.UUID, input TemplateUpdate) (EnvironmentTemplate, error)
	DeleteTemplate(ctx context.Context, id uuid.UUID) error

	ListSandboxes(ctx context.Context, projectID *uuid.UUID) ([]Sandbox, error)
	ListAllSandboxes(ctx context.Context) ([]Sandbox, error)
	CreateSandbox(ctx context.Context, input SandboxCreate) (Sandbox, error)
	GetSandbox(ctx context.Context, id uuid.UUID) (Sandbox, error)
	UpdateSandbox(ctx context.Context, id uuid.UUID, input SandboxUpdate) (Sandbox, error)
	DeleteSandbox(ctx context.Context, id uuid.UUID) error
	ListSandboxesForReconcile(ctx context.Context) ([]Sandbox, error)
	MarkSandboxRuntimeDeleted(ctx context.Context, id uuid.UUID) error

	ListRuntimeSessions(ctx context.Context, sandboxID uuid.UUID) ([]RuntimeSession, error)
	CreateRuntimeSession(ctx context.Context, input RuntimeSessionCreate) (RuntimeSession, error)
	GetRuntimeSession(ctx context.Context, id uuid.UUID) (RuntimeSession, error)
	UpdateRuntimeSession(ctx context.Context, id uuid.UUID, input RuntimeSessionUpdate) (RuntimeSession, error)

	ListExecutionTasks(ctx context.Context, sandboxID uuid.UUID) ([]ExecutionTask, error)
	CreateExecutionTask(ctx context.Context, input ExecutionTaskCreate) (ExecutionTask, error)
	GetExecutionTask(ctx context.Context, id uuid.UUID) (ExecutionTask, error)
	UpdateExecutionTask(ctx context.Context, id uuid.UUID, input ExecutionTaskUpdate) (ExecutionTask, error)

	ListArtifacts(ctx context.Context, sandboxID uuid.UUID, taskID *uuid.UUID) ([]Artifact, error)
	CreateArtifact(ctx context.Context, input ArtifactCreate) (Artifact, error)
	GetArtifact(ctx context.Context, id uuid.UUID) (Artifact, error)
	CaptureArtifactContent(ctx context.Context, input ArtifactContentCapture) (Artifact, error)
	GetArtifactContent(ctx context.Context, id uuid.UUID) (ArtifactContent, error)
}

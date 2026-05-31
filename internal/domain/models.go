package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Project struct {
	ID                uuid.UUID       `json:"id"`
	Name              string          `json:"name"`
	Slug              string          `json:"slug"`
	RepositoryURL     string          `json:"repositoryUrl,omitempty"`
	DefaultNamespace  string          `json:"defaultNamespace"`
	DefaultTemplateID *uuid.UUID      `json:"defaultTemplateId,omitempty"`
	Metadata          json.RawMessage `json:"metadata,omitempty"`
	CreatedAt         time.Time       `json:"createdAt"`
	UpdatedAt         time.Time       `json:"updatedAt"`
}

type ProjectPolicyEnforcement string

const (
	ProjectPolicyEnforcementDisabled ProjectPolicyEnforcement = "disabled"
	ProjectPolicyEnforcementEnforced ProjectPolicyEnforcement = "enforced"
)

type ProjectPolicy struct {
	ProjectID              uuid.UUID                `json:"projectId"`
	Enforcement            ProjectPolicyEnforcement `json:"enforcement"`
	AllowedImagePrefixes   []string                 `json:"allowedImagePrefixes,omitempty"`
	AllowedServiceAccounts []string                 `json:"allowedServiceAccounts,omitempty"`
	AllowedSecretRefs      []string                 `json:"allowedSecretRefs,omitempty"`
	CreatedAt              time.Time                `json:"createdAt"`
	UpdatedAt              time.Time                `json:"updatedAt"`
}

type ProjectQuotaPolicyEnforcement string

const (
	ProjectQuotaPolicyEnforcementDisabled ProjectQuotaPolicyEnforcement = "disabled"
	ProjectQuotaPolicyEnforcementEnforced ProjectQuotaPolicyEnforcement = "enforced"
)

type ProjectQuotaPolicy struct {
	ProjectID                uuid.UUID                     `json:"projectId"`
	Enforcement              ProjectQuotaPolicyEnforcement `json:"enforcement"`
	MaxActiveSandboxes       *int                          `json:"maxActiveSandboxes,omitempty"`
	MaxRetainedArtifactBytes *int64                        `json:"maxRetainedArtifactBytes,omitempty"`
	CreatedAt                time.Time                     `json:"createdAt"`
	UpdatedAt                time.Time                     `json:"updatedAt"`
}

type ProjectCredentialType string

const (
	ProjectCredentialTypeGit        ProjectCredentialType = "git"
	ProjectCredentialTypeRegistry   ProjectCredentialType = "registry"
	ProjectCredentialTypeKubernetes ProjectCredentialType = "kubernetes"
	ProjectCredentialTypeSSH        ProjectCredentialType = "ssh"
	ProjectCredentialTypeGeneric    ProjectCredentialType = "generic"
)

type ProjectCredential struct {
	ID        uuid.UUID             `json:"id"`
	ProjectID uuid.UUID             `json:"projectId"`
	Name      string                `json:"name"`
	Slug      string                `json:"slug"`
	Type      ProjectCredentialType `json:"type"`
	Target    string                `json:"target,omitempty"`
	SecretRef SecretRef             `json:"secretRef"`
	Usage     []string              `json:"usage,omitempty"`
	Metadata  json.RawMessage       `json:"metadata,omitempty"`
	CreatedAt time.Time             `json:"createdAt"`
	UpdatedAt time.Time             `json:"updatedAt"`
}

type ProjectUsage struct {
	ProjectID       uuid.UUID              `json:"projectId"`
	GeneratedAt     time.Time              `json:"generatedAt"`
	Sandboxes       ProjectSandboxUsage    `json:"sandboxes"`
	RuntimeSessions ProjectSessionUsage    `json:"runtimeSessions"`
	ExecutionTasks  ProjectTaskUsage       `json:"executionTasks"`
	Artifacts       ProjectArtifactUsage   `json:"artifacts"`
	Templates       ProjectTemplateUsage   `json:"templates"`
	Credentials     ProjectCredentialUsage `json:"credentials"`
	Notes           []string               `json:"notes,omitempty"`
}

type ProjectSandboxUsage struct {
	Total           int                         `json:"total"`
	Active          int                         `json:"active"`
	Pending         int                         `json:"pending"`
	Running         int                         `json:"running"`
	Stopped         int                         `json:"stopped"`
	Failed          int                         `json:"failed"`
	Deleted         int                         `json:"deleted"`
	CleanupPending  int                         `json:"cleanupPending"`
	ActiveRequests  SandboxResourceRequestUsage `json:"activeRequests"`
	RunningRequests SandboxResourceRequestUsage `json:"runningRequests"`
}

type SandboxResourceRequestUsage struct {
	Count   int                   `json:"count"`
	CPU     ResourceQuantityUsage `json:"cpu"`
	Memory  ResourceQuantityUsage `json:"memory"`
	Storage ResourceQuantityUsage `json:"storage"`
}

type ResourceQuantityUsage struct {
	Total    string `json:"total,omitempty"`
	Declared int    `json:"declared"`
	Missing  int    `json:"missing"`
	Invalid  int    `json:"invalid"`
}

type ProjectSessionUsage struct {
	Total    int `json:"total"`
	Active   int `json:"active"`
	Ended    int `json:"ended"`
	Failed   int `json:"failed"`
	Terminal int `json:"terminal"`
	IDE      int `json:"ide"`
	Notebook int `json:"notebook"`
	Browser  int `json:"browser"`
	Command  int `json:"command"`
	Custom   int `json:"custom"`
}

type ProjectTaskUsage struct {
	Total     int `json:"total"`
	Queued    int `json:"queued"`
	Running   int `json:"running"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
	Canceled  int `json:"canceled"`
	TimedOut  int `json:"timedOut"`
}

type ProjectArtifactUsage struct {
	Total           int   `json:"total"`
	RetainedContent int   `json:"retainedContent"`
	ReferencedBytes int64 `json:"referencedBytes"`
	RetainedBytes   int64 `json:"retainedBytes"`
	File            int   `json:"file"`
	Directory       int   `json:"directory"`
	Log             int   `json:"log"`
	Report          int   `json:"report"`
	Screenshot      int   `json:"screenshot"`
	Image           int   `json:"image"`
	Link            int   `json:"link"`
	Other           int   `json:"other"`
}

type ProjectTemplateUsage struct {
	ProjectScoped   int                  `json:"projectScoped"`
	GlobalVisible   int                  `json:"globalVisible"`
	CPURequests     []ResourceUsageValue `json:"cpuRequests,omitempty"`
	MemoryRequests  []ResourceUsageValue `json:"memoryRequests,omitempty"`
	StorageRequests []ResourceUsageValue `json:"storageRequests,omitempty"`
}

type ResourceUsageValue struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

type ProjectCredentialUsage struct {
	Total      int `json:"total"`
	Git        int `json:"git"`
	Registry   int `json:"registry"`
	Kubernetes int `json:"kubernetes"`
	SSH        int `json:"ssh"`
	Generic    int `json:"generic"`
}

type AuditEvent struct {
	ID           uuid.UUID       `json:"id"`
	ProjectID    *uuid.UUID      `json:"projectId,omitempty"`
	Action       string          `json:"action"`
	ResourceType string          `json:"resourceType"`
	ResourceID   *uuid.UUID      `json:"resourceId,omitempty"`
	ResourceName string          `json:"resourceName,omitempty"`
	Actor        string          `json:"actor,omitempty"`
	Source       string          `json:"source,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
}

type EnvironmentTemplate struct {
	ID              uuid.UUID       `json:"id"`
	ProjectID       *uuid.UUID      `json:"projectId,omitempty"`
	Name            string          `json:"name"`
	Slug            string          `json:"slug"`
	Image           string          `json:"image"`
	StartupCommand  []string        `json:"startupCommand,omitempty"`
	WorkingDir      string          `json:"workingDir"`
	CPURequest      string          `json:"cpuRequest,omitempty"`
	MemoryRequest   string          `json:"memoryRequest,omitempty"`
	StorageRequest  string          `json:"storageRequest,omitempty"`
	ExposedPorts    []TemplatePort  `json:"exposedPorts,omitempty"`
	Env             json.RawMessage `json:"env,omitempty"`
	SecretRefs      []SecretRef     `json:"secretRefs,omitempty"`
	NetworkPolicy   string          `json:"networkPolicy,omitempty"`
	LifecyclePolicy json.RawMessage `json:"lifecyclePolicy,omitempty"`
	Metadata        json.RawMessage `json:"metadata,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

type TemplatePort struct {
	Name     string `json:"name"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
}

type SecretRef struct {
	Name string `json:"name"`
	Key  string `json:"key,omitempty"`
}

type SandboxStatus string

const (
	SandboxStatusPending SandboxStatus = "pending"
	SandboxStatusRunning SandboxStatus = "running"
	SandboxStatusStopped SandboxStatus = "stopped"
	SandboxStatusFailed  SandboxStatus = "failed"
	SandboxStatusDeleted SandboxStatus = "deleted"
)

type Sandbox struct {
	ID                 uuid.UUID       `json:"id"`
	ProjectID          uuid.UUID       `json:"projectId"`
	TemplateID         uuid.UUID       `json:"templateId"`
	Name               string          `json:"name"`
	Slug               string          `json:"slug"`
	Status             SandboxStatus   `json:"status"`
	Namespace          string          `json:"namespace"`
	ServiceAccountName string          `json:"serviceAccountName"`
	RuntimeRef         *RuntimeRef     `json:"runtimeRef,omitempty"`
	Ports              []SandboxPort   `json:"ports,omitempty"`
	Metadata           json.RawMessage `json:"metadata,omitempty"`
	CreatedAt          time.Time       `json:"createdAt"`
	UpdatedAt          time.Time       `json:"updatedAt"`
	DeletedAt          *time.Time      `json:"deletedAt,omitempty"`
}

type RuntimeRef struct {
	Adapter   string `json:"adapter"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type RuntimeSessionType string

const (
	RuntimeSessionTypeTerminal RuntimeSessionType = "terminal"
	RuntimeSessionTypeIDE      RuntimeSessionType = "ide"
	RuntimeSessionTypeNotebook RuntimeSessionType = "notebook"
	RuntimeSessionTypeBrowser  RuntimeSessionType = "browser"
	RuntimeSessionTypeCommand  RuntimeSessionType = "command"
	RuntimeSessionTypeCustom   RuntimeSessionType = "custom"
)

type RuntimeSessionStatus string

const (
	RuntimeSessionStatusActive RuntimeSessionStatus = "active"
	RuntimeSessionStatusEnded  RuntimeSessionStatus = "ended"
	RuntimeSessionStatusFailed RuntimeSessionStatus = "failed"
)

type RuntimeSession struct {
	ID         uuid.UUID            `json:"id"`
	ProjectID  uuid.UUID            `json:"projectId"`
	SandboxID  uuid.UUID            `json:"sandboxId"`
	Type       RuntimeSessionType   `json:"type"`
	Status     RuntimeSessionStatus `json:"status"`
	Client     string               `json:"client,omitempty"`
	UserAgent  string               `json:"userAgent,omitempty"`
	RuntimeRef *RuntimeRef          `json:"runtimeRef,omitempty"`
	Metadata   json.RawMessage      `json:"metadata,omitempty"`
	StartedAt  time.Time            `json:"startedAt"`
	EndedAt    *time.Time           `json:"endedAt,omitempty"`
	CreatedAt  time.Time            `json:"createdAt"`
	UpdatedAt  time.Time            `json:"updatedAt"`
}

type SandboxPort struct {
	Name       string `json:"name"`
	Port       int    `json:"port"`
	Protocol   string `json:"protocol"`
	PreviewURL string `json:"previewUrl,omitempty"`
}

type ExecutionTaskStatus string

const (
	ExecutionTaskStatusQueued    ExecutionTaskStatus = "queued"
	ExecutionTaskStatusRunning   ExecutionTaskStatus = "running"
	ExecutionTaskStatusSucceeded ExecutionTaskStatus = "succeeded"
	ExecutionTaskStatusFailed    ExecutionTaskStatus = "failed"
	ExecutionTaskStatusCanceled  ExecutionTaskStatus = "canceled"
	ExecutionTaskStatusTimedOut  ExecutionTaskStatus = "timed_out"
)

type ExecutionTask struct {
	ID              uuid.UUID           `json:"id"`
	ProjectID       uuid.UUID           `json:"projectId"`
	SandboxID       uuid.UUID           `json:"sandboxId"`
	Status          ExecutionTaskStatus `json:"status"`
	Command         []string            `json:"command"`
	TimeoutSeconds  int                 `json:"timeoutSeconds"`
	ExitCode        *int                `json:"exitCode,omitempty"`
	Stdout          string              `json:"stdout"`
	Stderr          string              `json:"stderr"`
	OutputTruncated bool                `json:"outputTruncated"`
	Error           string              `json:"error,omitempty"`
	RuntimeRef      *RuntimeRef         `json:"runtimeRef,omitempty"`
	Metadata        json.RawMessage     `json:"metadata,omitempty"`
	StartedAt       *time.Time          `json:"startedAt,omitempty"`
	FinishedAt      *time.Time          `json:"finishedAt,omitempty"`
	CreatedAt       time.Time           `json:"createdAt"`
	UpdatedAt       time.Time           `json:"updatedAt"`
}

type ArtifactKind string

const (
	ArtifactKindFile       ArtifactKind = "file"
	ArtifactKindDirectory  ArtifactKind = "directory"
	ArtifactKindLog        ArtifactKind = "log"
	ArtifactKindReport     ArtifactKind = "report"
	ArtifactKindScreenshot ArtifactKind = "screenshot"
	ArtifactKindImage      ArtifactKind = "image"
	ArtifactKindLink       ArtifactKind = "link"
	ArtifactKindOther      ArtifactKind = "other"
)

type Artifact struct {
	ID              uuid.UUID        `json:"id"`
	ProjectID       uuid.UUID        `json:"projectId"`
	SandboxID       uuid.UUID        `json:"sandboxId"`
	TaskID          *uuid.UUID       `json:"taskId,omitempty"`
	Kind            ArtifactKind     `json:"kind"`
	Name            string           `json:"name"`
	URI             string           `json:"uri"`
	ContentType     string           `json:"contentType,omitempty"`
	SizeBytes       *int64           `json:"sizeBytes,omitempty"`
	Metadata        json.RawMessage  `json:"metadata,omitempty"`
	RetainedContent *ArtifactContent `json:"retainedContent,omitempty"`
	CreatedAt       time.Time        `json:"createdAt"`
	UpdatedAt       time.Time        `json:"updatedAt"`
}

type ArtifactContent struct {
	ArtifactID      uuid.UUID                      `json:"artifactId"`
	ContentType     string                         `json:"contentType,omitempty"`
	SizeBytes       int64                          `json:"sizeBytes"`
	SHA256          string                         `json:"sha256"`
	SourceURI       string                         `json:"sourceUri"`
	StorageProvider ArtifactContentStorageProvider `json:"storageProvider"`
	StorageKey      string                         `json:"storageKey,omitempty"`
	CapturedAt      time.Time                      `json:"capturedAt"`
	Content         []byte                         `json:"-"`
}

type ArtifactContentStorageProvider string

const (
	ArtifactContentStorageProviderPostgres   ArtifactContentStorageProvider = "postgres"
	ArtifactContentStorageProviderFilesystem ArtifactContentStorageProvider = "filesystem"
	ArtifactContentStorageProviderS3         ArtifactContentStorageProvider = "s3"
)

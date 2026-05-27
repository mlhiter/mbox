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
	ID          uuid.UUID       `json:"id"`
	ProjectID   uuid.UUID       `json:"projectId"`
	SandboxID   uuid.UUID       `json:"sandboxId"`
	TaskID      *uuid.UUID      `json:"taskId,omitempty"`
	Kind        ArtifactKind    `json:"kind"`
	Name        string          `json:"name"`
	URI         string          `json:"uri"`
	ContentType string          `json:"contentType,omitempty"`
	SizeBytes   *int64          `json:"sizeBytes,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

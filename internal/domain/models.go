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

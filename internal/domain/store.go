package domain

import (
	"context"

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
}

type SandboxCreate struct {
	ProjectID          uuid.UUID
	TemplateID         uuid.UUID
	Name               string
	Slug               string
	Namespace          string
	ServiceAccountName string
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

type Store interface {
	ListProjects(ctx context.Context) ([]Project, error)
	CreateProject(ctx context.Context, input ProjectCreate) (Project, error)
	GetProject(ctx context.Context, id uuid.UUID) (Project, error)
	UpdateProject(ctx context.Context, id uuid.UUID, input ProjectUpdate) (Project, error)
	DeleteProject(ctx context.Context, id uuid.UUID) error

	ListTemplates(ctx context.Context, projectID *uuid.UUID) ([]EnvironmentTemplate, error)
	CreateTemplate(ctx context.Context, input TemplateCreate) (EnvironmentTemplate, error)
	GetTemplate(ctx context.Context, id uuid.UUID) (EnvironmentTemplate, error)
	UpdateTemplate(ctx context.Context, id uuid.UUID, input TemplateUpdate) (EnvironmentTemplate, error)
	DeleteTemplate(ctx context.Context, id uuid.UUID) error

	ListSandboxes(ctx context.Context, projectID *uuid.UUID) ([]Sandbox, error)
	CreateSandbox(ctx context.Context, input SandboxCreate) (Sandbox, error)
	GetSandbox(ctx context.Context, id uuid.UUID) (Sandbox, error)
	UpdateSandbox(ctx context.Context, id uuid.UUID, input SandboxUpdate) (Sandbox, error)
	DeleteSandbox(ctx context.Context, id uuid.UUID) error
	ListSandboxesForReconcile(ctx context.Context) ([]Sandbox, error)
	MarkSandboxRuntimeDeleted(ctx context.Context, id uuid.UUID) error
}

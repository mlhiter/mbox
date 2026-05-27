package postgres

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/mlhiter/mbox/internal/domain"
)

type projectRow struct {
	ID                uuid.UUID       `db:"id"`
	Name              string          `db:"name"`
	Slug              string          `db:"slug"`
	RepositoryURL     string          `db:"repository_url"`
	DefaultNamespace  string          `db:"default_namespace"`
	DefaultTemplateID *uuid.UUID      `db:"default_template_id"`
	Metadata          json.RawMessage `db:"metadata"`
	CreatedAt         time.Time       `db:"created_at"`
	UpdatedAt         time.Time       `db:"updated_at"`
}

func (row projectRow) toDomain() domain.Project {
	return domain.Project{
		ID:                row.ID,
		Name:              row.Name,
		Slug:              row.Slug,
		RepositoryURL:     row.RepositoryURL,
		DefaultNamespace:  row.DefaultNamespace,
		DefaultTemplateID: row.DefaultTemplateID,
		Metadata:          row.Metadata,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}
}

type scanner interface {
	Scan(dest ...any) error
}

func scanProject(row scanner) (domain.Project, error) {
	var project projectRow
	err := row.Scan(
		&project.ID,
		&project.Name,
		&project.Slug,
		&project.RepositoryURL,
		&project.DefaultNamespace,
		&project.DefaultTemplateID,
		&project.Metadata,
		&project.CreatedAt,
		&project.UpdatedAt,
	)
	return project.toDomain(), err
}

func scanTemplate(row scanner) (domain.EnvironmentTemplate, error) {
	var template domain.EnvironmentTemplate
	var exposedPorts json.RawMessage
	var secretRefs json.RawMessage
	err := row.Scan(
		&template.ID,
		&template.ProjectID,
		&template.Name,
		&template.Slug,
		&template.Image,
		&template.StartupCommand,
		&template.WorkingDir,
		&template.CPURequest,
		&template.MemoryRequest,
		&template.StorageRequest,
		&exposedPorts,
		&template.Env,
		&secretRefs,
		&template.NetworkPolicy,
		&template.LifecyclePolicy,
		&template.Metadata,
		&template.CreatedAt,
		&template.UpdatedAt,
	)
	if err != nil {
		return domain.EnvironmentTemplate{}, err
	}
	if len(exposedPorts) > 0 {
		if err := json.Unmarshal(exposedPorts, &template.ExposedPorts); err != nil {
			return domain.EnvironmentTemplate{}, err
		}
	}
	if len(secretRefs) > 0 {
		if err := json.Unmarshal(secretRefs, &template.SecretRefs); err != nil {
			return domain.EnvironmentTemplate{}, err
		}
	}
	return template, nil
}

func scanSandbox(row scanner) (domain.Sandbox, error) {
	var sandbox domain.Sandbox
	var runtimeRef *json.RawMessage
	var ports json.RawMessage
	err := row.Scan(
		&sandbox.ID,
		&sandbox.ProjectID,
		&sandbox.TemplateID,
		&sandbox.Name,
		&sandbox.Slug,
		&sandbox.Status,
		&sandbox.Namespace,
		&sandbox.ServiceAccountName,
		&runtimeRef,
		&ports,
		&sandbox.Metadata,
		&sandbox.CreatedAt,
		&sandbox.UpdatedAt,
		&sandbox.DeletedAt,
	)
	if err != nil {
		return domain.Sandbox{}, err
	}
	if runtimeRef != nil && len(*runtimeRef) > 0 {
		var ref domain.RuntimeRef
		if err := json.Unmarshal(*runtimeRef, &ref); err != nil {
			return domain.Sandbox{}, err
		}
		sandbox.RuntimeRef = &ref
	}
	if len(ports) > 0 {
		if err := json.Unmarshal(ports, &sandbox.Ports); err != nil {
			return domain.Sandbox{}, err
		}
	}
	return sandbox, nil
}

func scanExecutionTask(row scanner) (domain.ExecutionTask, error) {
	var task domain.ExecutionTask
	var runtimeRef *json.RawMessage
	err := row.Scan(
		&task.ID,
		&task.ProjectID,
		&task.SandboxID,
		&task.Status,
		&task.Command,
		&task.TimeoutSeconds,
		&task.ExitCode,
		&task.Stdout,
		&task.Stderr,
		&task.OutputTruncated,
		&task.Error,
		&runtimeRef,
		&task.Metadata,
		&task.StartedAt,
		&task.FinishedAt,
		&task.CreatedAt,
		&task.UpdatedAt,
	)
	if err != nil {
		return domain.ExecutionTask{}, err
	}
	if runtimeRef != nil && len(*runtimeRef) > 0 {
		var ref domain.RuntimeRef
		if err := json.Unmarshal(*runtimeRef, &ref); err != nil {
			return domain.ExecutionTask{}, err
		}
		task.RuntimeRef = &ref
	}
	return task, nil
}

func scanArtifact(row scanner) (domain.Artifact, error) {
	var artifact domain.Artifact
	err := row.Scan(
		&artifact.ID,
		&artifact.ProjectID,
		&artifact.SandboxID,
		&artifact.TaskID,
		&artifact.Kind,
		&artifact.Name,
		&artifact.URI,
		&artifact.ContentType,
		&artifact.SizeBytes,
		&artifact.Metadata,
		&artifact.CreatedAt,
		&artifact.UpdatedAt,
	)
	return artifact, err
}

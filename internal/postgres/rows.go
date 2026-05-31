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

func scanProjectPolicy(row scanner) (domain.ProjectPolicy, error) {
	var policy domain.ProjectPolicy
	err := row.Scan(
		&policy.ProjectID,
		&policy.Enforcement,
		&policy.AllowedImagePrefixes,
		&policy.AllowedServiceAccounts,
		&policy.AllowedSecretRefs,
		&policy.CreatedAt,
		&policy.UpdatedAt,
	)
	return policy, err
}

func scanProjectQuotaPolicy(row scanner) (domain.ProjectQuotaPolicy, error) {
	var policy domain.ProjectQuotaPolicy
	err := row.Scan(
		&policy.ProjectID,
		&policy.Enforcement,
		&policy.MaxActiveSandboxes,
		&policy.MaxRetainedArtifactBytes,
		&policy.CreatedAt,
		&policy.UpdatedAt,
	)
	return policy, err
}

func scanProjectCredential(row scanner) (domain.ProjectCredential, error) {
	var credential domain.ProjectCredential
	var secretRef json.RawMessage
	err := row.Scan(
		&credential.ID,
		&credential.ProjectID,
		&credential.Name,
		&credential.Slug,
		&credential.Type,
		&credential.Target,
		&secretRef,
		&credential.Usage,
		&credential.Metadata,
		&credential.CreatedAt,
		&credential.UpdatedAt,
	)
	if err != nil {
		return domain.ProjectCredential{}, err
	}
	if len(secretRef) > 0 {
		if err := json.Unmarshal(secretRef, &credential.SecretRef); err != nil {
			return domain.ProjectCredential{}, err
		}
	}
	return credential, nil
}

func scanAuditEvent(row scanner) (domain.AuditEvent, error) {
	var event domain.AuditEvent
	err := row.Scan(
		&event.ID,
		&event.ProjectID,
		&event.Action,
		&event.ResourceType,
		&event.ResourceID,
		&event.ResourceName,
		&event.Actor,
		&event.Source,
		&event.Metadata,
		&event.CreatedAt,
	)
	return event, err
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
	if isSetJSON(runtimeRef) {
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

func scanRuntimeSession(row scanner) (domain.RuntimeSession, error) {
	var session domain.RuntimeSession
	var runtimeRef *json.RawMessage
	err := row.Scan(
		&session.ID,
		&session.ProjectID,
		&session.SandboxID,
		&session.Type,
		&session.Status,
		&session.Client,
		&session.UserAgent,
		&runtimeRef,
		&session.Metadata,
		&session.StartedAt,
		&session.EndedAt,
		&session.CreatedAt,
		&session.UpdatedAt,
	)
	if err != nil {
		return domain.RuntimeSession{}, err
	}
	if isSetJSON(runtimeRef) {
		var ref domain.RuntimeRef
		if err := json.Unmarshal(*runtimeRef, &ref); err != nil {
			return domain.RuntimeSession{}, err
		}
		session.RuntimeRef = &ref
	}
	return session, nil
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
	if isSetJSON(runtimeRef) {
		var ref domain.RuntimeRef
		if err := json.Unmarshal(*runtimeRef, &ref); err != nil {
			return domain.ExecutionTask{}, err
		}
		task.RuntimeRef = &ref
	}
	return task, nil
}

func isSetJSON(raw *json.RawMessage) bool {
	return raw != nil && len(*raw) > 0 && string(*raw) != "null"
}

func scanArtifact(row scanner) (domain.Artifact, error) {
	var artifact domain.Artifact
	var retainedContentType *string
	var retainedSizeBytes *int64
	var retainedSHA256 *string
	var retainedSourceURI *string
	var retainedStorageProvider *string
	var retainedStorageKey *string
	var retainedCapturedAt *time.Time
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
		&retainedContentType,
		&retainedSizeBytes,
		&retainedSHA256,
		&retainedSourceURI,
		&retainedStorageProvider,
		&retainedStorageKey,
		&retainedCapturedAt,
	)
	if err != nil {
		return domain.Artifact{}, err
	}
	if retainedCapturedAt != nil {
		content := domain.ArtifactContent{
			ArtifactID:  artifact.ID,
			CapturedAt:  *retainedCapturedAt,
			ContentType: stringPointerValue(retainedContentType),
			SizeBytes:   int64PointerValue(retainedSizeBytes),
			SHA256:      stringPointerValue(retainedSHA256),
			SourceURI:   stringPointerValue(retainedSourceURI),
			StorageProvider: domain.ArtifactContentStorageProvider(
				storageProviderPointerValue(retainedStorageProvider),
			),
			StorageKey: stringPointerValue(retainedStorageKey),
		}
		artifact.RetainedContent = &content
	}
	return artifact, nil
}

func scanArtifactContent(row scanner) (domain.ArtifactContent, error) {
	var content domain.ArtifactContent
	err := row.Scan(
		&content.ArtifactID,
		&content.Content,
		&content.ContentType,
		&content.SizeBytes,
		&content.SHA256,
		&content.SourceURI,
		&content.StorageProvider,
		&content.StorageKey,
		&content.CapturedAt,
	)
	if content.StorageProvider == "" {
		content.StorageProvider = domain.ArtifactContentStorageProviderPostgres
	}
	return content, err
}

func stringPointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func int64PointerValue(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func storageProviderPointerValue(value *string) string {
	if value == nil || *value == "" {
		return string(domain.ArtifactContentStorageProviderPostgres)
	}
	return *value
}

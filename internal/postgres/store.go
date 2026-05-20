package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mlhiter/mbox/internal/domain"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) ListProjects(ctx context.Context) ([]domain.Project, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, slug, repository_url, default_namespace, default_template_id, metadata, created_at, updated_at
		FROM projects
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	projects := []domain.Project{}
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

func (s *Store) CreateProject(ctx context.Context, input domain.ProjectCreate) (domain.Project, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO projects (name, slug, repository_url, default_namespace, metadata)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, slug, repository_url, default_namespace, default_template_id, metadata, created_at, updated_at
	`, input.Name, input.Slug, input.RepositoryURL, input.DefaultNamespace, jsonDefaultObject(input.Metadata))

	project, err := scanProject(row)
	return project, mapWriteError(err)
}

func (s *Store) GetProject(ctx context.Context, id uuid.UUID) (domain.Project, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, name, slug, repository_url, default_namespace, default_template_id, metadata, created_at, updated_at
		FROM projects
		WHERE id = $1
	`, id)
	project, err := scanProject(row)
	return project, mapReadError(err)
}

func (s *Store) UpdateProject(ctx context.Context, id uuid.UUID, input domain.ProjectUpdate) (domain.Project, error) {
	project, err := s.GetProject(ctx, id)
	if err != nil {
		return domain.Project{}, err
	}

	name := project.Name
	repositoryURL := project.RepositoryURL
	defaultNamespace := project.DefaultNamespace
	defaultTemplateID := project.DefaultTemplateID
	metadata := []byte(project.Metadata)

	if input.Name != nil {
		name = *input.Name
	}
	if input.RepositoryURL != nil {
		repositoryURL = *input.RepositoryURL
	}
	if input.DefaultNamespace != nil {
		defaultNamespace = *input.DefaultNamespace
	}
	if input.DefaultTemplateID != nil {
		defaultTemplateID = *input.DefaultTemplateID
	}
	if input.Metadata != nil {
		metadata = *input.Metadata
	}

	row := s.pool.QueryRow(ctx, `
		UPDATE projects
		SET name = $2, repository_url = $3, default_namespace = $4, default_template_id = $5, metadata = $6
		WHERE id = $1
		RETURNING id, name, slug, repository_url, default_namespace, default_template_id, metadata, created_at, updated_at
	`, id, name, repositoryURL, defaultNamespace, defaultTemplateID, jsonDefaultObject(metadata))
	updated, err := scanProject(row)
	return updated, mapWriteError(err)
}

func (s *Store) DeleteProject(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM projects WHERE id = $1`, id)
	if err != nil {
		return mapWriteError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) ListTemplates(ctx context.Context, projectID *uuid.UUID) ([]domain.EnvironmentTemplate, error) {
	query := `
		SELECT id, project_id, name, slug, image, startup_command, working_dir, cpu_request,
			memory_request, storage_request, exposed_ports, env, secret_refs, network_policy,
			lifecycle_policy, created_at, updated_at
		FROM environment_templates
	`
	args := []any{}
	if projectID != nil {
		query += ` WHERE project_id = $1 OR project_id IS NULL`
		args = append(args, *projectID)
	}
	query += ` ORDER BY project_id NULLS FIRST, created_at DESC`

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	templates := []domain.EnvironmentTemplate{}
	for rows.Next() {
		template, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}
	return templates, rows.Err()
}

func (s *Store) CreateTemplate(ctx context.Context, input domain.TemplateCreate) (domain.EnvironmentTemplate, error) {
	exposedPorts, err := json.Marshal(input.ExposedPorts)
	if err != nil {
		return domain.EnvironmentTemplate{}, err
	}
	secretRefs, err := json.Marshal(input.SecretRefs)
	if err != nil {
		return domain.EnvironmentTemplate{}, err
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO environment_templates (
			project_id, name, slug, image, startup_command, working_dir, cpu_request,
			memory_request, storage_request, exposed_ports, env, secret_refs, network_policy,
			lifecycle_policy
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id, project_id, name, slug, image, startup_command, working_dir, cpu_request,
			memory_request, storage_request, exposed_ports, env, secret_refs, network_policy,
			lifecycle_policy, created_at, updated_at
	`, input.ProjectID, input.Name, input.Slug, input.Image, stringSliceDefault(input.StartupCommand), defaultString(input.WorkingDir, "/workspace"),
		input.CPURequest, input.MemoryRequest, input.StorageRequest, exposedPorts, jsonDefaultObject(input.Env), secretRefs,
		defaultString(input.NetworkPolicy, "default"), jsonDefaultObject(input.LifecyclePolicy))

	template, err := scanTemplate(row)
	return template, mapWriteError(err)
}

func (s *Store) GetTemplate(ctx context.Context, id uuid.UUID) (domain.EnvironmentTemplate, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, project_id, name, slug, image, startup_command, working_dir, cpu_request,
			memory_request, storage_request, exposed_ports, env, secret_refs, network_policy,
			lifecycle_policy, created_at, updated_at
		FROM environment_templates
		WHERE id = $1
	`, id)
	template, err := scanTemplate(row)
	return template, mapReadError(err)
}

func (s *Store) UpdateTemplate(ctx context.Context, id uuid.UUID, input domain.TemplateUpdate) (domain.EnvironmentTemplate, error) {
	template, err := s.GetTemplate(ctx, id)
	if err != nil {
		return domain.EnvironmentTemplate{}, err
	}

	name := template.Name
	image := template.Image
	startupCommand := template.StartupCommand
	workingDir := template.WorkingDir
	cpuRequest := template.CPURequest
	memoryRequest := template.MemoryRequest
	storageRequest := template.StorageRequest
	exposedPorts := template.ExposedPorts
	env := []byte(template.Env)
	secretRefs := template.SecretRefs
	networkPolicy := template.NetworkPolicy
	lifecyclePolicy := []byte(template.LifecyclePolicy)

	if input.Name != nil {
		name = *input.Name
	}
	if input.Image != nil {
		image = *input.Image
	}
	if input.StartupCommand != nil {
		startupCommand = *input.StartupCommand
	}
	if input.WorkingDir != nil {
		workingDir = *input.WorkingDir
	}
	if input.CPURequest != nil {
		cpuRequest = *input.CPURequest
	}
	if input.MemoryRequest != nil {
		memoryRequest = *input.MemoryRequest
	}
	if input.StorageRequest != nil {
		storageRequest = *input.StorageRequest
	}
	if input.ExposedPorts != nil {
		exposedPorts = *input.ExposedPorts
	}
	if input.Env != nil {
		env = *input.Env
	}
	if input.SecretRefs != nil {
		secretRefs = *input.SecretRefs
	}
	if input.NetworkPolicy != nil {
		networkPolicy = *input.NetworkPolicy
	}
	if input.LifecyclePolicy != nil {
		lifecyclePolicy = *input.LifecyclePolicy
	}

	exposedPortsJSON, err := json.Marshal(exposedPorts)
	if err != nil {
		return domain.EnvironmentTemplate{}, err
	}
	secretRefsJSON, err := json.Marshal(secretRefs)
	if err != nil {
		return domain.EnvironmentTemplate{}, err
	}

	row := s.pool.QueryRow(ctx, `
		UPDATE environment_templates
		SET name = $2, image = $3, startup_command = $4, working_dir = $5, cpu_request = $6,
			memory_request = $7, storage_request = $8, exposed_ports = $9, env = $10,
			secret_refs = $11, network_policy = $12, lifecycle_policy = $13
		WHERE id = $1
		RETURNING id, project_id, name, slug, image, startup_command, working_dir, cpu_request,
			memory_request, storage_request, exposed_ports, env, secret_refs, network_policy,
			lifecycle_policy, created_at, updated_at
	`, id, name, image, startupCommand, workingDir, cpuRequest, memoryRequest, storageRequest,
		exposedPortsJSON, jsonDefaultObject(env), secretRefsJSON, networkPolicy, jsonDefaultObject(lifecyclePolicy))

	updated, err := scanTemplate(row)
	return updated, mapWriteError(err)
}

func (s *Store) DeleteTemplate(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM environment_templates WHERE id = $1`, id)
	if err != nil {
		return mapWriteError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) ListSandboxes(ctx context.Context, projectID *uuid.UUID) ([]domain.Sandbox, error) {
	query := `
		SELECT id, project_id, template_id, name, slug, status, namespace, service_account_name,
			runtime_ref, ports, metadata, created_at, updated_at, deleted_at
		FROM sandboxes
		WHERE deleted_at IS NULL
	`
	args := []any{}
	if projectID != nil {
		query += ` AND project_id = $1`
		args = append(args, *projectID)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sandboxes := []domain.Sandbox{}
	for rows.Next() {
		sandbox, err := scanSandbox(rows)
		if err != nil {
			return nil, err
		}
		sandboxes = append(sandboxes, sandbox)
	}
	return sandboxes, rows.Err()
}

func (s *Store) ListSandboxesForReconcile(ctx context.Context) ([]domain.Sandbox, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, project_id, template_id, name, slug, status, namespace, service_account_name,
			runtime_ref, ports, metadata, created_at, updated_at, deleted_at
		FROM sandboxes
		WHERE status IN ('pending', 'running', 'failed')
			OR (status = 'stopped' AND runtime_ref IS NOT NULL)
			OR (deleted_at IS NOT NULL AND runtime_ref IS NOT NULL)
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sandboxes := []domain.Sandbox{}
	for rows.Next() {
		sandbox, err := scanSandbox(rows)
		if err != nil {
			return nil, err
		}
		sandboxes = append(sandboxes, sandbox)
	}
	return sandboxes, rows.Err()
}

func (s *Store) CreateSandbox(ctx context.Context, input domain.SandboxCreate) (domain.Sandbox, error) {
	ports, err := json.Marshal(input.Ports)
	if err != nil {
		return domain.Sandbox{}, err
	}
	row := s.pool.QueryRow(ctx, `
		INSERT INTO sandboxes (project_id, template_id, name, slug, namespace, service_account_name, ports, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, project_id, template_id, name, slug, status, namespace, service_account_name,
			runtime_ref, ports, metadata, created_at, updated_at, deleted_at
	`, input.ProjectID, input.TemplateID, input.Name, input.Slug, input.Namespace, input.ServiceAccountName, ports, jsonDefaultObject(input.Metadata))

	sandbox, err := scanSandbox(row)
	return sandbox, mapWriteError(err)
}

func (s *Store) GetSandbox(ctx context.Context, id uuid.UUID) (domain.Sandbox, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, project_id, template_id, name, slug, status, namespace, service_account_name,
			runtime_ref, ports, metadata, created_at, updated_at, deleted_at
		FROM sandboxes
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	sandbox, err := scanSandbox(row)
	return sandbox, mapReadError(err)
}

func (s *Store) UpdateSandbox(ctx context.Context, id uuid.UUID, input domain.SandboxUpdate) (domain.Sandbox, error) {
	sandbox, err := s.GetSandbox(ctx, id)
	if err != nil {
		return domain.Sandbox{}, err
	}

	name := sandbox.Name
	status := sandbox.Status
	namespace := sandbox.Namespace
	serviceAccountName := sandbox.ServiceAccountName
	runtimeRef := sandbox.RuntimeRef
	ports := sandbox.Ports
	metadata := []byte(sandbox.Metadata)

	if input.Name != nil {
		name = *input.Name
	}
	if input.Status != nil {
		status = *input.Status
	}
	if input.Namespace != nil {
		namespace = *input.Namespace
	}
	if input.ServiceAccountName != nil {
		serviceAccountName = *input.ServiceAccountName
	}
	if input.RuntimeRef != nil {
		runtimeRef = *input.RuntimeRef
	}
	if input.Ports != nil {
		ports = *input.Ports
	}
	if input.Metadata != nil {
		metadata = *input.Metadata
	}

	runtimeRefJSON, err := jsonOrNil(runtimeRef)
	if err != nil {
		return domain.Sandbox{}, err
	}
	portsJSON, err := json.Marshal(ports)
	if err != nil {
		return domain.Sandbox{}, err
	}

	row := s.pool.QueryRow(ctx, `
		UPDATE sandboxes
		SET name = $2, status = $3, namespace = $4, service_account_name = $5,
			runtime_ref = $6, ports = $7, metadata = $8
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, project_id, template_id, name, slug, status, namespace, service_account_name,
			runtime_ref, ports, metadata, created_at, updated_at, deleted_at
	`, id, name, status, namespace, serviceAccountName, runtimeRefJSON, portsJSON, jsonDefaultObject(metadata))

	updated, err := scanSandbox(row)
	return updated, mapWriteError(err)
}

func (s *Store) DeleteSandbox(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE sandboxes
		SET status = 'deleted', deleted_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	if err != nil {
		return mapWriteError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) MarkSandboxRuntimeDeleted(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE sandboxes
		SET runtime_ref = NULL
		WHERE id = $1
	`, id)
	if err != nil {
		return mapWriteError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func mapReadError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

func mapWriteError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return fmt.Errorf("%w: %s", domain.ErrConflict, pgErr.ConstraintName)
		case "23503":
			return fmt.Errorf("%w: %s", domain.ErrNotFound, pgErr.ConstraintName)
		}
	}
	return err
}

func jsonDefaultObject(data []byte) []byte {
	if len(data) == 0 {
		return []byte(`{}`)
	}
	return data
}

func stringSliceDefault(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func jsonOrNil(value any) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	return json.Marshal(value)
}

func defaultString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

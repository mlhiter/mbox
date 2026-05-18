package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/mlhiter/mbox/internal/domain"
)

func TestCreateProject(t *testing.T) {
	store := newFakeStore()
	api := New(store)

	res := request(api, http.MethodPost, "/v1/projects", map[string]any{
		"name":             "Demo Project",
		"slug":             "demo-project",
		"repositoryUrl":    "https://github.com/example/demo",
		"defaultNamespace": "mbox-demo",
	})

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, res.Code, res.Body.String())
	}
	var project domain.Project
	decodeResponse(t, res, &project)
	if project.ID == uuid.Nil || project.Slug != "demo-project" || project.DefaultNamespace != "mbox-demo" {
		t.Fatalf("unexpected project response: %+v", project)
	}
}

func TestCreateTemplateRejectsInvalidPort(t *testing.T) {
	api := New(newFakeStore())

	res := request(api, http.MethodPost, "/v1/templates", map[string]any{
		"name":  "Go",
		"slug":  "go",
		"image": "golang:1.25",
		"exposedPorts": []map[string]any{
			{"name": "web", "port": 70000},
		},
	})

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
	}
}

func TestSandboxLifecycle(t *testing.T) {
	store := newFakeStore()
	api := New(store)
	project := store.mustProject(t)
	template := store.mustTemplate(t, &project.ID)

	createRes := request(api, http.MethodPost, "/v1/sandboxes", map[string]any{
		"projectId":          project.ID,
		"templateId":         template.ID,
		"name":               "Dev",
		"slug":               "dev",
		"namespace":          "mbox-demo",
		"serviceAccountName": "mbox-sandbox",
	})
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected create status %d, got %d: %s", http.StatusCreated, createRes.Code, createRes.Body.String())
	}
	var sandbox domain.Sandbox
	decodeResponse(t, createRes, &sandbox)
	if sandbox.Status != domain.SandboxStatusPending {
		t.Fatalf("expected pending sandbox, got %q", sandbox.Status)
	}

	patchRes := request(api, http.MethodPatch, "/v1/sandboxes/"+sandbox.ID.String(), map[string]any{
		"status": "running",
		"runtimeRef": map[string]any{
			"adapter":   "agent-sandbox",
			"kind":      "SandboxClaim",
			"namespace": "mbox-demo",
			"name":      "dev",
		},
	})
	if patchRes.Code != http.StatusOK {
		t.Fatalf("expected patch status %d, got %d: %s", http.StatusOK, patchRes.Code, patchRes.Body.String())
	}
	decodeResponse(t, patchRes, &sandbox)
	if sandbox.Status != domain.SandboxStatusRunning || sandbox.RuntimeRef == nil {
		t.Fatalf("unexpected patched sandbox: %+v", sandbox)
	}

	clearRuntimeRes := request(api, http.MethodPatch, "/v1/sandboxes/"+sandbox.ID.String(), map[string]any{
		"runtimeRef": nil,
	})
	if clearRuntimeRes.Code != http.StatusOK {
		t.Fatalf("expected clear runtime status %d, got %d: %s", http.StatusOK, clearRuntimeRes.Code, clearRuntimeRes.Body.String())
	}
	var clearedSandbox domain.Sandbox
	decodeResponse(t, clearRuntimeRes, &clearedSandbox)
	if clearedSandbox.RuntimeRef != nil {
		t.Fatalf("expected runtimeRef to be cleared, got %+v", clearedSandbox.RuntimeRef)
	}

	deleteRes := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/v1/sandboxes/"+sandbox.ID.String(), nil)
	api.ServeHTTP(deleteRes, req)
	if deleteRes.Code != http.StatusNoContent {
		t.Fatalf("expected delete status %d, got %d", http.StatusNoContent, deleteRes.Code)
	}

	getRes := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/sandboxes/"+sandbox.ID.String(), nil)
	api.ServeHTTP(getRes, req)
	if getRes.Code != http.StatusNotFound {
		t.Fatalf("expected get after delete status %d, got %d", http.StatusNotFound, getRes.Code)
	}
}

func request(handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}

func decodeResponse(t *testing.T, res *httptest.ResponseRecorder, dest any) {
	t.Helper()
	if err := json.NewDecoder(res.Body).Decode(dest); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

type fakeStore struct {
	projects  map[uuid.UUID]domain.Project
	templates map[uuid.UUID]domain.EnvironmentTemplate
	sandboxes map[uuid.UUID]domain.Sandbox
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		projects:  map[uuid.UUID]domain.Project{},
		templates: map[uuid.UUID]domain.EnvironmentTemplate{},
		sandboxes: map[uuid.UUID]domain.Sandbox{},
	}
}

func (s *fakeStore) mustProject(t *testing.T) domain.Project {
	t.Helper()
	project, err := s.CreateProject(context.Background(), domain.ProjectCreate{
		Name:             "Demo",
		Slug:             "demo",
		DefaultNamespace: "mbox-demo",
	})
	if err != nil {
		t.Fatal(err)
	}
	return project
}

func (s *fakeStore) mustTemplate(t *testing.T, projectID *uuid.UUID) domain.EnvironmentTemplate {
	t.Helper()
	template, err := s.CreateTemplate(context.Background(), domain.TemplateCreate{
		ProjectID:  projectID,
		Name:       "Linux",
		Slug:       "linux",
		Image:      "ubuntu:24.04",
		WorkingDir: "/workspace",
	})
	if err != nil {
		t.Fatal(err)
	}
	return template
}

func (s *fakeStore) ListProjects(context.Context) ([]domain.Project, error) {
	items := make([]domain.Project, 0, len(s.projects))
	for _, project := range s.projects {
		items = append(items, project)
	}
	return items, nil
}

func (s *fakeStore) CreateProject(_ context.Context, input domain.ProjectCreate) (domain.Project, error) {
	now := time.Now()
	project := domain.Project{
		ID:               uuid.New(),
		Name:             input.Name,
		Slug:             input.Slug,
		RepositoryURL:    input.RepositoryURL,
		DefaultNamespace: input.DefaultNamespace,
		Metadata:         input.Metadata,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	s.projects[project.ID] = project
	return project, nil
}

func (s *fakeStore) GetProject(_ context.Context, id uuid.UUID) (domain.Project, error) {
	project, ok := s.projects[id]
	if !ok {
		return domain.Project{}, domain.ErrNotFound
	}
	return project, nil
}

func (s *fakeStore) UpdateProject(_ context.Context, id uuid.UUID, input domain.ProjectUpdate) (domain.Project, error) {
	project, ok := s.projects[id]
	if !ok {
		return domain.Project{}, domain.ErrNotFound
	}
	if input.Name != nil {
		project.Name = *input.Name
	}
	if input.RepositoryURL != nil {
		project.RepositoryURL = *input.RepositoryURL
	}
	if input.DefaultNamespace != nil {
		project.DefaultNamespace = *input.DefaultNamespace
	}
	if input.DefaultTemplateID != nil {
		project.DefaultTemplateID = *input.DefaultTemplateID
	}
	if input.Metadata != nil {
		project.Metadata = *input.Metadata
	}
	s.projects[id] = project
	return project, nil
}

func (s *fakeStore) DeleteProject(_ context.Context, id uuid.UUID) error {
	if _, ok := s.projects[id]; !ok {
		return domain.ErrNotFound
	}
	delete(s.projects, id)
	return nil
}

func (s *fakeStore) ListTemplates(_ context.Context, projectID *uuid.UUID) ([]domain.EnvironmentTemplate, error) {
	items := []domain.EnvironmentTemplate{}
	for _, template := range s.templates {
		if projectID == nil || template.ProjectID == nil || *template.ProjectID == *projectID {
			items = append(items, template)
		}
	}
	return items, nil
}

func (s *fakeStore) CreateTemplate(_ context.Context, input domain.TemplateCreate) (domain.EnvironmentTemplate, error) {
	now := time.Now()
	template := domain.EnvironmentTemplate{
		ID:              uuid.New(),
		ProjectID:       input.ProjectID,
		Name:            input.Name,
		Slug:            input.Slug,
		Image:           input.Image,
		StartupCommand:  input.StartupCommand,
		WorkingDir:      input.WorkingDir,
		CPURequest:      input.CPURequest,
		MemoryRequest:   input.MemoryRequest,
		StorageRequest:  input.StorageRequest,
		ExposedPorts:    input.ExposedPorts,
		Env:             input.Env,
		SecretRefs:      input.SecretRefs,
		NetworkPolicy:   input.NetworkPolicy,
		LifecyclePolicy: input.LifecyclePolicy,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	s.templates[template.ID] = template
	return template, nil
}

func (s *fakeStore) GetTemplate(_ context.Context, id uuid.UUID) (domain.EnvironmentTemplate, error) {
	template, ok := s.templates[id]
	if !ok {
		return domain.EnvironmentTemplate{}, domain.ErrNotFound
	}
	return template, nil
}

func (s *fakeStore) UpdateTemplate(_ context.Context, id uuid.UUID, input domain.TemplateUpdate) (domain.EnvironmentTemplate, error) {
	template, ok := s.templates[id]
	if !ok {
		return domain.EnvironmentTemplate{}, domain.ErrNotFound
	}
	if input.Name != nil {
		template.Name = *input.Name
	}
	if input.Image != nil {
		template.Image = *input.Image
	}
	if input.StartupCommand != nil {
		template.StartupCommand = *input.StartupCommand
	}
	if input.WorkingDir != nil {
		template.WorkingDir = *input.WorkingDir
	}
	if input.CPURequest != nil {
		template.CPURequest = *input.CPURequest
	}
	if input.MemoryRequest != nil {
		template.MemoryRequest = *input.MemoryRequest
	}
	if input.StorageRequest != nil {
		template.StorageRequest = *input.StorageRequest
	}
	if input.ExposedPorts != nil {
		template.ExposedPorts = *input.ExposedPorts
	}
	if input.Env != nil {
		template.Env = *input.Env
	}
	if input.SecretRefs != nil {
		template.SecretRefs = *input.SecretRefs
	}
	if input.NetworkPolicy != nil {
		template.NetworkPolicy = *input.NetworkPolicy
	}
	if input.LifecyclePolicy != nil {
		template.LifecyclePolicy = *input.LifecyclePolicy
	}
	s.templates[id] = template
	return template, nil
}

func (s *fakeStore) DeleteTemplate(_ context.Context, id uuid.UUID) error {
	if _, ok := s.templates[id]; !ok {
		return domain.ErrNotFound
	}
	delete(s.templates, id)
	return nil
}

func (s *fakeStore) ListSandboxes(_ context.Context, projectID *uuid.UUID) ([]domain.Sandbox, error) {
	items := []domain.Sandbox{}
	for _, sandbox := range s.sandboxes {
		if sandbox.DeletedAt == nil && (projectID == nil || sandbox.ProjectID == *projectID) {
			items = append(items, sandbox)
		}
	}
	return items, nil
}

func (s *fakeStore) CreateSandbox(_ context.Context, input domain.SandboxCreate) (domain.Sandbox, error) {
	now := time.Now()
	if _, ok := s.projects[input.ProjectID]; !ok {
		return domain.Sandbox{}, domain.ErrNotFound
	}
	if _, ok := s.templates[input.TemplateID]; !ok {
		return domain.Sandbox{}, domain.ErrNotFound
	}
	sandbox := domain.Sandbox{
		ID:                 uuid.New(),
		ProjectID:          input.ProjectID,
		TemplateID:         input.TemplateID,
		Name:               input.Name,
		Slug:               input.Slug,
		Status:             domain.SandboxStatusPending,
		Namespace:          input.Namespace,
		ServiceAccountName: input.ServiceAccountName,
		Metadata:           input.Metadata,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	s.sandboxes[sandbox.ID] = sandbox
	return sandbox, nil
}

func (s *fakeStore) GetSandbox(_ context.Context, id uuid.UUID) (domain.Sandbox, error) {
	sandbox, ok := s.sandboxes[id]
	if !ok || sandbox.DeletedAt != nil {
		return domain.Sandbox{}, domain.ErrNotFound
	}
	return sandbox, nil
}

func (s *fakeStore) UpdateSandbox(_ context.Context, id uuid.UUID, input domain.SandboxUpdate) (domain.Sandbox, error) {
	sandbox, ok := s.sandboxes[id]
	if !ok || sandbox.DeletedAt != nil {
		return domain.Sandbox{}, domain.ErrNotFound
	}
	if input.Name != nil {
		sandbox.Name = *input.Name
	}
	if input.Status != nil {
		sandbox.Status = *input.Status
	}
	if input.Namespace != nil {
		sandbox.Namespace = *input.Namespace
	}
	if input.ServiceAccountName != nil {
		sandbox.ServiceAccountName = *input.ServiceAccountName
	}
	if input.RuntimeRef != nil {
		sandbox.RuntimeRef = nil
		if *input.RuntimeRef != nil {
			value := **input.RuntimeRef
			sandbox.RuntimeRef = &value
		}
	}
	if input.Ports != nil {
		sandbox.Ports = *input.Ports
	}
	if input.Metadata != nil {
		sandbox.Metadata = *input.Metadata
	}
	s.sandboxes[id] = sandbox
	return sandbox, nil
}

func (s *fakeStore) DeleteSandbox(_ context.Context, id uuid.UUID) error {
	sandbox, ok := s.sandboxes[id]
	if !ok || sandbox.DeletedAt != nil {
		return domain.ErrNotFound
	}
	now := time.Now()
	sandbox.Status = domain.SandboxStatusDeleted
	sandbox.DeletedAt = &now
	s.sandboxes[id] = sandbox
	return nil
}

func (s *fakeStore) ListSandboxesForReconcile(context.Context) ([]domain.Sandbox, error) {
	items := []domain.Sandbox{}
	for _, sandbox := range s.sandboxes {
		items = append(items, sandbox)
	}
	return items, nil
}

func (s *fakeStore) MarkSandboxRuntimeDeleted(_ context.Context, id uuid.UUID) error {
	sandbox, ok := s.sandboxes[id]
	if !ok {
		return domain.ErrNotFound
	}
	sandbox.RuntimeRef = nil
	s.sandboxes[id] = sandbox
	return nil
}

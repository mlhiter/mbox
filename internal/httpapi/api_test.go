package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/mlhiter/mbox/internal/domain"
	mboxruntime "github.com/mlhiter/mbox/internal/runtime"
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

func TestCreateSandboxUsesProjectDefaults(t *testing.T) {
	store := newFakeStore()
	api := New(store)
	project := store.mustProject(t)
	template := store.mustTemplate(t, &project.ID)
	project.DefaultTemplateID = &template.ID
	store.projects[project.ID] = project

	res := request(api, http.MethodPost, "/v1/sandboxes", map[string]any{
		"projectId": project.ID,
		"name":      "Defaulted Dev",
		"slug":      "defaulted-dev",
	})
	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, res.Code, res.Body.String())
	}
	var sandbox domain.Sandbox
	decodeResponse(t, res, &sandbox)
	if sandbox.TemplateID != template.ID {
		t.Fatalf("expected template %s, got %s", template.ID, sandbox.TemplateID)
	}
	if sandbox.Namespace != project.DefaultNamespace {
		t.Fatalf("expected namespace %q, got %q", project.DefaultNamespace, sandbox.Namespace)
	}
	if sandbox.ServiceAccountName != defaultSandboxServiceAccountName {
		t.Fatalf("expected service account %q, got %q", defaultSandboxServiceAccountName, sandbox.ServiceAccountName)
	}
}

func TestCreateSandboxCopiesTemplatePorts(t *testing.T) {
	store := newFakeStore()
	api := New(store)
	project := store.mustProject(t)
	template := store.mustTemplate(t, &project.ID)
	template.ExposedPorts = []domain.TemplatePort{{
		Name:     "web",
		Port:     3000,
		Protocol: "TCP",
	}}
	store.templates[template.ID] = template

	res := request(api, http.MethodPost, "/v1/sandboxes", map[string]any{
		"projectId":  project.ID,
		"templateId": template.ID,
		"name":       "Preview Dev",
		"slug":       "preview-dev",
	})
	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, res.Code, res.Body.String())
	}
	var sandbox domain.Sandbox
	decodeResponse(t, res, &sandbox)
	if len(sandbox.Ports) != 1 || sandbox.Ports[0].Name != "web" || sandbox.Ports[0].Port != 3000 {
		t.Fatalf("expected template port copied to sandbox, got %+v", sandbox.Ports)
	}
}

func TestCreateSandboxRequiresTemplateWithoutProjectDefault(t *testing.T) {
	store := newFakeStore()
	api := New(store)
	project := store.mustProject(t)

	res := request(api, http.MethodPost, "/v1/sandboxes", map[string]any{
		"projectId": project.ID,
		"name":      "Defaulted Dev",
		"slug":      "defaulted-dev",
	})
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, res.Code, res.Body.String())
	}
}

func TestRuntimeRoutesRequireConfiguredAccess(t *testing.T) {
	store := newFakeStore()
	api := New(store)
	project := store.mustProject(t)
	template := store.mustTemplate(t, &project.ID)
	sandbox, err := store.CreateSandbox(context.Background(), domain.SandboxCreate{
		ProjectID:          project.ID,
		TemplateID:         template.ID,
		Name:               "Dev",
		Slug:               "dev",
		Namespace:          "mbox-demo",
		ServiceAccountName: "mbox-sandbox",
	})
	if err != nil {
		t.Fatal(err)
	}

	res := request(api, http.MethodGet, "/v1/sandboxes/"+sandbox.ID.String()+"/runtime", nil)
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, res.Code)
	}
}

func TestRuntimeRoutesReturnTargetLogsAndEvents(t *testing.T) {
	store := newFakeStore()
	access := &fakeRuntimeAccess{}
	api := NewWithRuntimeAccess(store, access)
	project := store.mustProject(t)
	template := store.mustTemplate(t, &project.ID)
	sandbox, err := store.CreateSandbox(context.Background(), domain.SandboxCreate{
		ProjectID:          project.ID,
		TemplateID:         template.ID,
		Name:               "Dev",
		Slug:               "dev",
		Namespace:          "mbox-demo",
		ServiceAccountName: "mbox-sandbox",
	})
	if err != nil {
		t.Fatal(err)
	}
	runtimeRef := &domain.RuntimeRef{
		Adapter:   "agent-sandbox",
		Kind:      "SandboxClaim",
		Namespace: "mbox-demo",
		Name:      "dev",
	}
	sandbox.RuntimeRef = runtimeRef
	store.sandboxes[sandbox.ID] = sandbox

	runtimeRes := request(api, http.MethodGet, "/v1/sandboxes/"+sandbox.ID.String()+"/runtime", nil)
	if runtimeRes.Code != http.StatusOK {
		t.Fatalf("expected runtime status %d, got %d: %s", http.StatusOK, runtimeRes.Code, runtimeRes.Body.String())
	}
	var target mboxruntime.RuntimeTarget
	decodeResponse(t, runtimeRes, &target)
	if target.PodName != "pod-dev" {
		t.Fatalf("unexpected target: %+v", target)
	}

	logsRes := request(api, http.MethodGet, "/v1/sandboxes/"+sandbox.ID.String()+"/logs?tailLines=12", nil)
	if logsRes.Code != http.StatusOK {
		t.Fatalf("expected logs status %d, got %d: %s", http.StatusOK, logsRes.Code, logsRes.Body.String())
	}
	var logs mboxruntime.LogResult
	decodeResponse(t, logsRes, &logs)
	if logs.Logs != "ready\n" || access.lastTailLines != 12 {
		t.Fatalf("unexpected logs response: %+v tail=%d", logs, access.lastTailLines)
	}

	eventsRes := request(api, http.MethodGet, "/v1/sandboxes/"+sandbox.ID.String()+"/events", nil)
	if eventsRes.Code != http.StatusOK {
		t.Fatalf("expected events status %d, got %d: %s", http.StatusOK, eventsRes.Code, eventsRes.Body.String())
	}
	var events ListResponse[mboxruntime.RuntimeEvent]
	decodeResponse(t, eventsRes, &events)
	if len(events.Items) != 1 || events.Items[0].Reason != "Started" {
		t.Fatalf("unexpected events response: %+v", events)
	}
}

func TestSandboxPortsRouteReturnsPreviewURLs(t *testing.T) {
	store := newFakeStore()
	api := NewWithRuntimeAccess(store, &fakeRuntimeAccess{})
	project := store.mustProject(t)
	template := store.mustTemplate(t, &project.ID)
	sandbox, err := store.CreateSandbox(context.Background(), domain.SandboxCreate{
		ProjectID:          project.ID,
		TemplateID:         template.ID,
		Name:               "Dev",
		Slug:               "dev",
		Namespace:          "mbox-demo",
		ServiceAccountName: "mbox-sandbox",
	})
	if err != nil {
		t.Fatal(err)
	}
	runtimeRef := &domain.RuntimeRef{
		Adapter:   "agent-sandbox",
		Kind:      "SandboxClaim",
		Namespace: "mbox-demo",
		Name:      "dev",
	}
	sandbox.Status = domain.SandboxStatusRunning
	sandbox.RuntimeRef = runtimeRef
	sandbox.Ports = []domain.SandboxPort{{Name: "web", Port: 3000, Protocol: "TCP"}}
	store.sandboxes[sandbox.ID] = sandbox

	res := request(api, http.MethodGet, "/v1/sandboxes/"+sandbox.ID.String()+"/ports", nil)
	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, res.Code, res.Body.String())
	}
	var ports mboxruntime.PreviewPortsResult
	decodeResponse(t, res, &ports)
	if len(ports.Items) != 1 || !ports.Items[0].Available || ports.Items[0].PreviewURL == "" {
		t.Fatalf("unexpected ports response: %+v", ports)
	}
}

func TestSandboxPortProxyRequiresDeclaredTCPPort(t *testing.T) {
	store := newFakeStore()
	api := NewWithRuntimeAccess(store, &fakeRuntimeAccess{})
	project := store.mustProject(t)
	template := store.mustTemplate(t, &project.ID)
	sandbox, err := store.CreateSandbox(context.Background(), domain.SandboxCreate{
		ProjectID:          project.ID,
		TemplateID:         template.ID,
		Name:               "Dev",
		Slug:               "dev",
		Namespace:          "mbox-demo",
		ServiceAccountName: "mbox-sandbox",
	})
	if err != nil {
		t.Fatal(err)
	}
	runtimeRef := &domain.RuntimeRef{
		Adapter:   "agent-sandbox",
		Kind:      "SandboxClaim",
		Namespace: "mbox-demo",
		Name:      "dev",
	}
	sandbox.Status = domain.SandboxStatusRunning
	sandbox.RuntimeRef = runtimeRef
	sandbox.Ports = []domain.SandboxPort{{Name: "web", Port: 3000, Protocol: "TCP"}}
	store.sandboxes[sandbox.ID] = sandbox

	res := request(api, http.MethodGet, "/v1/sandboxes/"+sandbox.ID.String()+"/ports/3001/proxy/", nil)
	if res.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, res.Code, res.Body.String())
	}
}

func TestSandboxPortProxyStreamsRuntimeResponse(t *testing.T) {
	store := newFakeStore()
	access := &fakeRuntimeAccess{}
	api := NewWithRuntimeAccess(store, access)
	project := store.mustProject(t)
	template := store.mustTemplate(t, &project.ID)
	sandbox, err := store.CreateSandbox(context.Background(), domain.SandboxCreate{
		ProjectID:          project.ID,
		TemplateID:         template.ID,
		Name:               "Dev",
		Slug:               "dev",
		Namespace:          "mbox-demo",
		ServiceAccountName: "mbox-sandbox",
	})
	if err != nil {
		t.Fatal(err)
	}
	runtimeRef := &domain.RuntimeRef{
		Adapter:   "agent-sandbox",
		Kind:      "SandboxClaim",
		Namespace: "mbox-demo",
		Name:      "dev",
	}
	sandbox.Status = domain.SandboxStatusRunning
	sandbox.RuntimeRef = runtimeRef
	sandbox.Ports = []domain.SandboxPort{{Name: "web", Port: 3000, Protocol: "TCP"}}
	store.sandboxes[sandbox.ID] = sandbox

	res := request(api, http.MethodGet, "/v1/sandboxes/"+sandbox.ID.String()+"/ports/3000/proxy/healthz?ready=true", nil)
	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, res.Code, res.Body.String())
	}
	if res.Body.String() != "preview:/healthz?ready=true" {
		t.Fatalf("unexpected proxy response: %q", res.Body.String())
	}
	if access.lastPreviewPort != 3000 || access.lastPreviewPath != "/healthz" || access.lastPreviewQuery != "ready=true" {
		t.Fatalf("unexpected proxy request: port=%d path=%q query=%q", access.lastPreviewPort, access.lastPreviewPath, access.lastPreviewQuery)
	}
}

func TestTerminalRejectsNonRunningSandbox(t *testing.T) {
	store := newFakeStore()
	api := NewWithRuntimeAccess(store, &fakeRuntimeAccess{})
	project := store.mustProject(t)
	template := store.mustTemplate(t, &project.ID)
	sandbox, err := store.CreateSandbox(context.Background(), domain.SandboxCreate{
		ProjectID:          project.ID,
		TemplateID:         template.ID,
		Name:               "Dev",
		Slug:               "dev",
		Namespace:          "mbox-demo",
		ServiceAccountName: "mbox-sandbox",
	})
	if err != nil {
		t.Fatal(err)
	}
	runtimeRef := &domain.RuntimeRef{
		Adapter:   "agent-sandbox",
		Kind:      "SandboxClaim",
		Namespace: "mbox-demo",
		Name:      "dev",
	}
	sandbox.RuntimeRef = runtimeRef
	store.sandboxes[sandbox.ID] = sandbox

	res := request(api, http.MethodGet, "/v1/sandboxes/"+sandbox.ID.String()+"/terminal", nil)
	if res.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d: %s", http.StatusConflict, res.Code, res.Body.String())
	}
}

func TestTerminalRejectsUnsupportedShell(t *testing.T) {
	store := newFakeStore()
	api := NewWithRuntimeAccess(store, &fakeRuntimeAccess{})
	project := store.mustProject(t)
	template := store.mustTemplate(t, &project.ID)
	sandbox, err := store.CreateSandbox(context.Background(), domain.SandboxCreate{
		ProjectID:          project.ID,
		TemplateID:         template.ID,
		Name:               "Dev",
		Slug:               "dev",
		Namespace:          "mbox-demo",
		ServiceAccountName: "mbox-sandbox",
	})
	if err != nil {
		t.Fatal(err)
	}
	running := domain.SandboxStatusRunning
	runtimeRef := &domain.RuntimeRef{
		Adapter:   "agent-sandbox",
		Kind:      "SandboxClaim",
		Namespace: "mbox-demo",
		Name:      "dev",
	}
	sandbox.Status = running
	sandbox.RuntimeRef = runtimeRef
	store.sandboxes[sandbox.ID] = sandbox

	res := request(api, http.MethodGet, "/v1/sandboxes/"+sandbox.ID.String()+"/terminal?shell=/bin/zsh", nil)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, res.Code, res.Body.String())
	}
}

func TestTerminalOriginCheckAllowsOnlySameHostOrLoopbackDev(t *testing.T) {
	tests := []struct {
		name        string
		requestHost string
		origin      string
		want        bool
	}{
		{
			name:        "same host and port",
			requestHost: "app.example.com:18080",
			origin:      "https://app.example.com:18080",
			want:        true,
		},
		{
			name:        "same host different port denied",
			requestHost: "app.example.com:18080",
			origin:      "https://app.example.com:5174",
			want:        false,
		},
		{
			name:        "loopback dev proxy allowed across ports",
			requestHost: "127.0.0.1:18080",
			origin:      "http://localhost:5174",
			want:        true,
		},
		{
			name:        "cross host denied",
			requestHost: "app.example.com",
			origin:      "https://evil.example.com",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/sandboxes/id/terminal", nil)
			req.Host = tt.requestHost
			req.Header.Set("Origin", tt.origin)
			if got := terminalUpgrader.CheckOrigin(req); got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
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

type ListResponse[T any] struct {
	Items []T `json:"items"`
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
		Ports:              input.Ports,
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

type fakeRuntimeAccess struct {
	lastTailLines    int64
	lastPreviewPort  int
	lastPreviewPath  string
	lastPreviewQuery string
}

func (f *fakeRuntimeAccess) ResolveRuntime(context.Context, domain.RuntimeRef) (mboxruntime.RuntimeTarget, error) {
	return mboxruntime.RuntimeTarget{
		Namespace: "mbox-demo",
		PodName:   "pod-dev",
		Container: "workspace",
		Phase:     "Running",
		Selector:  "app=pod-dev",
	}, nil
}

func (f *fakeRuntimeAccess) ReadLogs(_ context.Context, ref domain.RuntimeRef, options mboxruntime.LogOptions) (mboxruntime.LogResult, error) {
	f.lastTailLines = options.TailLines
	target, _ := f.ResolveRuntime(context.Background(), ref)
	return mboxruntime.LogResult{
		Target: target,
		Logs:   "ready\n",
	}, nil
}

func (f *fakeRuntimeAccess) ListEvents(context.Context, domain.RuntimeRef) ([]mboxruntime.RuntimeEvent, error) {
	return []mboxruntime.RuntimeEvent{{
		Type:    "Normal",
		Reason:  "Started",
		Message: "Started container",
		Count:   1,
	}}, nil
}

func (f *fakeRuntimeAccess) ProxyPreview(_ context.Context, _ domain.RuntimeRef, request mboxruntime.PreviewProxyRequest) (mboxruntime.PreviewProxyResponse, error) {
	f.lastPreviewPort = request.Port
	f.lastPreviewPath = request.Path
	f.lastPreviewQuery = request.Query
	return mboxruntime.PreviewProxyResponse{
		StatusCode: http.StatusOK,
		Header:     map[string][]string{"Content-Type": {"text/plain"}},
		Body:       io.NopCloser(strings.NewReader("preview:" + request.Path + "?" + request.Query)),
	}, nil
}

func (f *fakeRuntimeAccess) Exec(_ context.Context, _ domain.RuntimeRef, options mboxruntime.ExecOptions) error {
	if options.Stdin != nil {
		_, _ = io.Copy(options.Stdout, options.Stdin)
	}
	return nil
}

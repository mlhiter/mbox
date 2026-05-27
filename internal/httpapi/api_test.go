package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	k8sexec "k8s.io/client-go/util/exec"

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

func TestTemplateMetadataRoundTrip(t *testing.T) {
	store := newFakeStore()
	api := New(store)

	createRes := request(api, http.MethodPost, "/v1/templates", map[string]any{
		"name":  "Node.js Web App",
		"slug":  "nodejs-web-app",
		"image": "node:22-bookworm-slim",
		"metadata": map[string]any{
			"runtimeType":      "Node.js",
			"useCase":          "Web app preview",
			"resourcePreset":   "Small",
			"validationStatus": "not_tested",
		},
	})
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, createRes.Code, createRes.Body.String())
	}
	var created domain.EnvironmentTemplate
	decodeResponse(t, createRes, &created)
	if !strings.Contains(string(created.Metadata), `"runtimeType":"Node.js"`) {
		t.Fatalf("expected runtime metadata, got %s", created.Metadata)
	}

	updateRes := request(api, http.MethodPatch, "/v1/templates/"+created.ID.String(), map[string]any{
		"metadata": map[string]any{
			"runtimeType":      "Node.js",
			"useCase":          "API service",
			"resourcePreset":   "Medium",
			"validationStatus": "passed",
		},
	})
	if updateRes.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, updateRes.Code, updateRes.Body.String())
	}
	var updated domain.EnvironmentTemplate
	decodeResponse(t, updateRes, &updated)
	if !strings.Contains(string(updated.Metadata), `"validationStatus":"passed"`) {
		t.Fatalf("expected updated metadata, got %s", updated.Metadata)
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

func TestSandboxStartStopRoutesSetLifecycleStatus(t *testing.T) {
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
	runtimeRef := &domain.RuntimeRef{
		Adapter:   "agent-sandbox",
		Kind:      "SandboxClaim",
		Namespace: "mbox-demo",
		Name:      "dev",
	}
	running := domain.SandboxStatusRunning
	if _, err := store.UpdateSandbox(context.Background(), sandbox.ID, domain.SandboxUpdate{
		Status:     &running,
		RuntimeRef: &runtimeRef,
	}); err != nil {
		t.Fatal(err)
	}

	stopRes := request(api, http.MethodPost, "/v1/sandboxes/"+sandbox.ID.String()+"/stop", nil)
	if stopRes.Code != http.StatusOK {
		t.Fatalf("expected stop status %d, got %d: %s", http.StatusOK, stopRes.Code, stopRes.Body.String())
	}
	var stopped domain.Sandbox
	decodeResponse(t, stopRes, &stopped)
	if stopped.Status != domain.SandboxStatusStopped || stopped.RuntimeRef == nil {
		t.Fatalf("expected stopped sandbox with runtime ref, got %+v", stopped)
	}

	startRes := request(api, http.MethodPost, "/v1/sandboxes/"+sandbox.ID.String()+"/start", nil)
	if startRes.Code != http.StatusOK {
		t.Fatalf("expected start status %d, got %d: %s", http.StatusOK, startRes.Code, startRes.Body.String())
	}
	var pending domain.Sandbox
	decodeResponse(t, startRes, &pending)
	if pending.Status != domain.SandboxStatusPending || pending.RuntimeRef == nil {
		t.Fatalf("expected pending sandbox with runtime ref, got %+v", pending)
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

func TestCreateSandboxDefaultsSlugFromName(t *testing.T) {
	store := newFakeStore()
	api := New(store)
	project := store.mustProject(t)
	template := store.mustTemplate(t, &project.ID)
	project.DefaultTemplateID = &template.ID
	store.projects[project.ID] = project

	res := request(api, http.MethodPost, "/v1/sandboxes", map[string]any{
		"projectId": project.ID,
		"name":      "Test Node.js",
	})
	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, res.Code, res.Body.String())
	}
	var sandbox domain.Sandbox
	decodeResponse(t, res, &sandbox)
	if sandbox.Slug != "test-node-js" {
		t.Fatalf("expected generated slug, got %q", sandbox.Slug)
	}
	if sandbox.Namespace != project.DefaultNamespace || sandbox.ServiceAccountName != defaultSandboxServiceAccountName {
		t.Fatalf("unexpected runtime defaults: namespace=%q serviceAccount=%q", sandbox.Namespace, sandbox.ServiceAccountName)
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

func TestPatchSandboxPortsEnablesPreviewMetadata(t *testing.T) {
	store := newFakeStore()
	api := NewWithRuntimeAccess(store, &fakeRuntimeAccess{})
	project := store.mustProject(t)
	template := store.mustTemplate(t, &project.ID)
	sandbox, err := store.CreateSandbox(context.Background(), domain.SandboxCreate{
		ProjectID:          project.ID,
		TemplateID:         template.ID,
		Name:               "Manual Preview",
		Slug:               "manual-preview",
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
		Name:      "manual-preview",
	}
	sandbox.Status = domain.SandboxStatusRunning
	sandbox.RuntimeRef = runtimeRef
	store.sandboxes[sandbox.ID] = sandbox

	patchRes := request(api, http.MethodPatch, "/v1/sandboxes/"+sandbox.ID.String(), map[string]any{
		"ports": []map[string]any{
			{"name": "web", "port": 3000, "protocol": "TCP"},
		},
	})
	if patchRes.Code != http.StatusOK {
		t.Fatalf("expected patch status %d, got %d: %s", http.StatusOK, patchRes.Code, patchRes.Body.String())
	}
	var patched domain.Sandbox
	decodeResponse(t, patchRes, &patched)
	if len(patched.Ports) != 1 || patched.Ports[0].Port != 3000 {
		t.Fatalf("expected patched sandbox port, got %+v", patched.Ports)
	}

	portsRes := request(api, http.MethodGet, "/v1/sandboxes/"+sandbox.ID.String()+"/ports", nil)
	if portsRes.Code != http.StatusOK {
		t.Fatalf("expected ports status %d, got %d: %s", http.StatusOK, portsRes.Code, portsRes.Body.String())
	}
	var ports mboxruntime.PreviewPortsResult
	decodeResponse(t, portsRes, &ports)
	if len(ports.Items) != 1 || ports.Items[0].Port != 3000 || !ports.Items[0].Available || ports.Items[0].PreviewURL == "" {
		t.Fatalf("unexpected ports response: %+v", ports)
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
	if len(target.Storage) != 1 || target.Storage[0].MountPath != "/workspace" || target.Storage[0].Phase != "Bound" {
		t.Fatalf("unexpected target storage: %+v", target.Storage)
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

func TestExecutionTaskRunsCommandAndRecordsOutput(t *testing.T) {
	store := newFakeStore()
	access := &fakeRuntimeAccess{}
	api := NewWithRuntimeAccess(store, access)
	sandbox := store.mustRunningSandbox(t)

	res := request(api, http.MethodPost, "/v1/sandboxes/"+sandbox.ID.String()+"/tasks", map[string]any{
		"command":        []string{"echo", "hello"},
		"timeoutSeconds": 10,
	})
	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, res.Code, res.Body.String())
	}
	var task domain.ExecutionTask
	decodeResponse(t, res, &task)
	if task.Status != domain.ExecutionTaskStatusQueued {
		t.Fatalf("expected queued task response, got %+v", task)
	}
	task = waitForTaskStatus(t, store, task.ID, domain.ExecutionTaskStatusSucceeded)
	if task.ExitCode == nil || *task.ExitCode != 0 {
		t.Fatalf("expected succeeded task with exit 0, got %+v", task)
	}
	if task.Stdout != "exec:echo hello\n" || task.Stderr != "" || task.StartedAt == nil || task.FinishedAt == nil {
		t.Fatalf("unexpected task output or timing: %+v", task)
	}
	if len(access.lastExecCommand) != 2 || access.lastExecCommand[0] != "echo" || access.lastExecCommand[1] != "hello" {
		t.Fatalf("unexpected exec command: %+v", access.lastExecCommand)
	}

	listRes := request(api, http.MethodGet, "/v1/sandboxes/"+sandbox.ID.String()+"/tasks", nil)
	if listRes.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d: %s", http.StatusOK, listRes.Code, listRes.Body.String())
	}
	var list ListResponse[domain.ExecutionTask]
	decodeResponse(t, listRes, &list)
	if len(list.Items) != 1 || list.Items[0].ID != task.ID {
		t.Fatalf("unexpected task list: %+v", list)
	}

	getRes := request(api, http.MethodGet, "/v1/tasks/"+task.ID.String(), nil)
	if getRes.Code != http.StatusOK {
		t.Fatalf("expected get status %d, got %d: %s", http.StatusOK, getRes.Code, getRes.Body.String())
	}
	var fetched domain.ExecutionTask
	decodeResponse(t, getRes, &fetched)
	if fetched.ID != task.ID || fetched.SandboxID != sandbox.ID || fetched.Status != domain.ExecutionTaskStatusSucceeded {
		t.Fatalf("unexpected fetched task: %+v", fetched)
	}
}

func TestExecutionTaskRecordsFailingCommand(t *testing.T) {
	store := newFakeStore()
	access := &fakeRuntimeAccess{execExitCode: 7}
	api := NewWithRuntimeAccess(store, access)
	sandbox := store.mustRunningSandbox(t)

	res := request(api, http.MethodPost, "/v1/sandboxes/"+sandbox.ID.String()+"/tasks", map[string]any{
		"command": []string{"false"},
	})
	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, res.Code, res.Body.String())
	}
	var task domain.ExecutionTask
	decodeResponse(t, res, &task)
	task = waitForTaskStatus(t, store, task.ID, domain.ExecutionTaskStatusFailed)
	if task.Status != domain.ExecutionTaskStatusFailed || task.ExitCode == nil || *task.ExitCode != 7 {
		t.Fatalf("expected failed task with exit 7, got %+v", task)
	}
	if task.Error == "" {
		t.Fatalf("expected task error, got %+v", task)
	}
}

func TestExecutionTaskCreateReturnsBeforeCommandCompletes(t *testing.T) {
	store := newFakeStore()
	access := newBlockingRuntimeAccess()
	api := NewWithRuntimeAccess(store, access)
	sandbox := store.mustRunningSandbox(t)

	res := request(api, http.MethodPost, "/v1/sandboxes/"+sandbox.ID.String()+"/tasks", map[string]any{
		"command": []string{"sleep", "30"},
	})
	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, res.Code, res.Body.String())
	}
	var task domain.ExecutionTask
	decodeResponse(t, res, &task)
	if task.Status != domain.ExecutionTaskStatusQueued || task.FinishedAt != nil {
		t.Fatalf("expected queued unfinished task response, got %+v", task)
	}
	waitForExecStart(t, access)
	running := waitForTaskStatus(t, store, task.ID, domain.ExecutionTaskStatusRunning)
	if running.StartedAt == nil {
		t.Fatalf("expected task start time, got %+v", running)
	}

	access.release()
	completed := waitForTaskStatus(t, store, task.ID, domain.ExecutionTaskStatusSucceeded)
	if completed.FinishedAt == nil || completed.Stdout != "released\n" {
		t.Fatalf("expected released task output, got %+v", completed)
	}
}

func TestExecutionTaskCancelRunningCommand(t *testing.T) {
	store := newFakeStore()
	access := newBlockingRuntimeAccess()
	api := NewWithRuntimeAccess(store, access)
	sandbox := store.mustRunningSandbox(t)

	res := request(api, http.MethodPost, "/v1/sandboxes/"+sandbox.ID.String()+"/tasks", map[string]any{
		"command": []string{"sleep", "30"},
	})
	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, res.Code, res.Body.String())
	}
	var task domain.ExecutionTask
	decodeResponse(t, res, &task)
	waitForExecStart(t, access)

	cancelRes := request(api, http.MethodPost, "/v1/tasks/"+task.ID.String()+"/cancel", nil)
	if cancelRes.Code != http.StatusOK {
		t.Fatalf("expected cancel status %d, got %d: %s", http.StatusOK, cancelRes.Code, cancelRes.Body.String())
	}
	canceled := waitForTaskStatus(t, store, task.ID, domain.ExecutionTaskStatusCanceled)
	if canceled.Error != "task canceled" || canceled.FinishedAt == nil {
		t.Fatalf("expected canceled task, got %+v", canceled)
	}
}

func TestExecutionTaskCancelRejectsFinishedTask(t *testing.T) {
	store := newFakeStore()
	access := &fakeRuntimeAccess{}
	api := NewWithRuntimeAccess(store, access)
	sandbox := store.mustRunningSandbox(t)

	res := request(api, http.MethodPost, "/v1/sandboxes/"+sandbox.ID.String()+"/tasks", map[string]any{
		"command": []string{"echo", "done"},
	})
	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, res.Code, res.Body.String())
	}
	var task domain.ExecutionTask
	decodeResponse(t, res, &task)
	waitForTaskStatus(t, store, task.ID, domain.ExecutionTaskStatusSucceeded)

	cancelRes := request(api, http.MethodPost, "/v1/tasks/"+task.ID.String()+"/cancel", nil)
	if cancelRes.Code != http.StatusConflict {
		t.Fatalf("expected cancel conflict, got %d: %s", cancelRes.Code, cancelRes.Body.String())
	}
}

func TestExecutionTaskRejectsNonRunningSandbox(t *testing.T) {
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

	res := request(api, http.MethodPost, "/v1/sandboxes/"+sandbox.ID.String()+"/tasks", map[string]any{
		"command": []string{"pwd"},
	})
	if res.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d: %s", http.StatusConflict, res.Code, res.Body.String())
	}
}

func TestExecutionTaskValidation(t *testing.T) {
	store := newFakeStore()
	api := NewWithRuntimeAccess(store, &fakeRuntimeAccess{})
	sandbox := store.mustRunningSandbox(t)

	res := request(api, http.MethodPost, "/v1/sandboxes/"+sandbox.ID.String()+"/tasks", map[string]any{
		"command":        []string{},
		"timeoutSeconds": 601,
	})
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, res.Code, res.Body.String())
	}
}

func TestArtifactCreateListAndGet(t *testing.T) {
	store := newFakeStore()
	api := NewWithRuntimeAccess(store, &fakeRuntimeAccess{})
	sandbox := store.mustRunningSandbox(t)
	task, err := store.CreateExecutionTask(context.Background(), domain.ExecutionTaskCreate{
		ProjectID:      sandbox.ProjectID,
		SandboxID:      sandbox.ID,
		Command:        []string{"sh", "-lc", "npm test"},
		TimeoutSeconds: 60,
		RuntimeRef:     sandbox.RuntimeRef,
	})
	if err != nil {
		t.Fatal(err)
	}

	res := request(api, http.MethodPost, "/v1/sandboxes/"+sandbox.ID.String()+"/artifacts", map[string]any{
		"taskId":      task.ID,
		"kind":        "report",
		"name":        "Test report",
		"uri":         "workspace:///workspace/reports/test.json",
		"contentType": "application/json",
		"sizeBytes":   128,
		"metadata": map[string]any{
			"source": "npm test",
		},
	})
	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, res.Code, res.Body.String())
	}
	var artifact domain.Artifact
	decodeResponse(t, res, &artifact)
	if artifact.ID == uuid.Nil || artifact.TaskID == nil || *artifact.TaskID != task.ID || artifact.Kind != domain.ArtifactKindReport {
		t.Fatalf("unexpected artifact: %+v", artifact)
	}
	if artifact.Name != "Test report" || artifact.URI != "workspace:///workspace/reports/test.json" || artifact.SizeBytes == nil || *artifact.SizeBytes != 128 {
		t.Fatalf("unexpected artifact fields: %+v", artifact)
	}

	listRes := request(api, http.MethodGet, "/v1/sandboxes/"+sandbox.ID.String()+"/artifacts", nil)
	if listRes.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d: %s", http.StatusOK, listRes.Code, listRes.Body.String())
	}
	var list ListResponse[domain.Artifact]
	decodeResponse(t, listRes, &list)
	if len(list.Items) != 1 || list.Items[0].ID != artifact.ID {
		t.Fatalf("unexpected artifact list: %+v", list)
	}

	taskListRes := request(api, http.MethodGet, "/v1/tasks/"+task.ID.String()+"/artifacts", nil)
	if taskListRes.Code != http.StatusOK {
		t.Fatalf("expected task list status %d, got %d: %s", http.StatusOK, taskListRes.Code, taskListRes.Body.String())
	}
	var taskList ListResponse[domain.Artifact]
	decodeResponse(t, taskListRes, &taskList)
	if len(taskList.Items) != 1 || taskList.Items[0].ID != artifact.ID {
		t.Fatalf("unexpected task artifact list: %+v", taskList)
	}

	getRes := request(api, http.MethodGet, "/v1/artifacts/"+artifact.ID.String(), nil)
	if getRes.Code != http.StatusOK {
		t.Fatalf("expected get status %d, got %d: %s", http.StatusOK, getRes.Code, getRes.Body.String())
	}
	var fetched domain.Artifact
	decodeResponse(t, getRes, &fetched)
	if fetched.ID != artifact.ID || fetched.SandboxID != sandbox.ID {
		t.Fatalf("unexpected fetched artifact: %+v", fetched)
	}
}

func TestArtifactRejectsTaskFromDifferentSandbox(t *testing.T) {
	store := newFakeStore()
	api := NewWithRuntimeAccess(store, &fakeRuntimeAccess{})
	first := store.mustRunningSandbox(t)
	second := store.mustRunningSandbox(t)
	task, err := store.CreateExecutionTask(context.Background(), domain.ExecutionTaskCreate{
		ProjectID:      first.ProjectID,
		SandboxID:      first.ID,
		Command:        []string{"pwd"},
		TimeoutSeconds: 60,
		RuntimeRef:     first.RuntimeRef,
	})
	if err != nil {
		t.Fatal(err)
	}

	res := request(api, http.MethodPost, "/v1/sandboxes/"+second.ID.String()+"/artifacts", map[string]any{
		"taskId": task.ID,
		"kind":   "file",
		"name":   "wrong sandbox",
		"uri":    "workspace:///workspace/out.txt",
	})
	if res.Code != http.StatusConflict {
		t.Fatalf("expected conflict, got %d: %s", res.Code, res.Body.String())
	}
}

func TestArtifactValidation(t *testing.T) {
	store := newFakeStore()
	api := NewWithRuntimeAccess(store, &fakeRuntimeAccess{})
	sandbox := store.mustRunningSandbox(t)

	res := request(api, http.MethodPost, "/v1/sandboxes/"+sandbox.ID.String()+"/artifacts", map[string]any{
		"kind":      "package",
		"name":      "",
		"uri":       "",
		"sizeBytes": -1,
	})
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, res.Code, res.Body.String())
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

func waitForTaskStatus(t *testing.T, store *fakeStore, id uuid.UUID, status domain.ExecutionTaskStatus) domain.ExecutionTask {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		task, err := store.GetExecutionTask(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if task.Status == status {
			return task
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for task %s status %s, last status %s", id, status, task.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitForExecStart(t *testing.T, access *blockingRuntimeAccess) {
	t.Helper()
	select {
	case <-access.started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for exec start")
	}
}

type ListResponse[T any] struct {
	Items []T `json:"items"`
}

type fakeStore struct {
	mu        sync.Mutex
	projects  map[uuid.UUID]domain.Project
	templates map[uuid.UUID]domain.EnvironmentTemplate
	sandboxes map[uuid.UUID]domain.Sandbox
	tasks     map[uuid.UUID]domain.ExecutionTask
	artifacts map[uuid.UUID]domain.Artifact
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		projects:  map[uuid.UUID]domain.Project{},
		templates: map[uuid.UUID]domain.EnvironmentTemplate{},
		sandboxes: map[uuid.UUID]domain.Sandbox{},
		tasks:     map[uuid.UUID]domain.ExecutionTask{},
		artifacts: map[uuid.UUID]domain.Artifact{},
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

func (s *fakeStore) mustRunningSandbox(t *testing.T) domain.Sandbox {
	t.Helper()
	project := s.mustProject(t)
	template := s.mustTemplate(t, &project.ID)
	sandbox, err := s.CreateSandbox(context.Background(), domain.SandboxCreate{
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
	s.sandboxes[sandbox.ID] = sandbox
	return sandbox
}

func (s *fakeStore) ListProjects(context.Context) ([]domain.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]domain.Project, 0, len(s.projects))
	for _, project := range s.projects {
		items = append(items, project)
	}
	return items, nil
}

func (s *fakeStore) CreateProject(_ context.Context, input domain.ProjectCreate) (domain.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
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
	s.mu.Lock()
	defer s.mu.Unlock()
	project, ok := s.projects[id]
	if !ok {
		return domain.Project{}, domain.ErrNotFound
	}
	return project, nil
}

func (s *fakeStore) UpdateProject(_ context.Context, id uuid.UUID, input domain.ProjectUpdate) (domain.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.projects[id]; !ok {
		return domain.ErrNotFound
	}
	delete(s.projects, id)
	return nil
}

func (s *fakeStore) ListTemplates(_ context.Context, projectID *uuid.UUID) ([]domain.EnvironmentTemplate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := []domain.EnvironmentTemplate{}
	for _, template := range s.templates {
		if projectID == nil || template.ProjectID == nil || *template.ProjectID == *projectID {
			items = append(items, template)
		}
	}
	return items, nil
}

func (s *fakeStore) CreateTemplate(_ context.Context, input domain.TemplateCreate) (domain.EnvironmentTemplate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
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
		Metadata:        input.Metadata,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	s.templates[template.ID] = template
	return template, nil
}

func (s *fakeStore) GetTemplate(_ context.Context, id uuid.UUID) (domain.EnvironmentTemplate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	template, ok := s.templates[id]
	if !ok {
		return domain.EnvironmentTemplate{}, domain.ErrNotFound
	}
	return template, nil
}

func (s *fakeStore) UpdateTemplate(_ context.Context, id uuid.UUID, input domain.TemplateUpdate) (domain.EnvironmentTemplate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
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
	if input.Metadata != nil {
		template.Metadata = *input.Metadata
	}
	s.templates[id] = template
	return template, nil
}

func (s *fakeStore) DeleteTemplate(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.templates[id]; !ok {
		return domain.ErrNotFound
	}
	delete(s.templates, id)
	return nil
}

func (s *fakeStore) ListSandboxes(_ context.Context, projectID *uuid.UUID) ([]domain.Sandbox, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := []domain.Sandbox{}
	for _, sandbox := range s.sandboxes {
		if sandbox.DeletedAt == nil && (projectID == nil || sandbox.ProjectID == *projectID) {
			items = append(items, sandbox)
		}
	}
	return items, nil
}

func (s *fakeStore) CreateSandbox(_ context.Context, input domain.SandboxCreate) (domain.Sandbox, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
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
	s.mu.Lock()
	defer s.mu.Unlock()
	sandbox, ok := s.sandboxes[id]
	if !ok || sandbox.DeletedAt != nil {
		return domain.Sandbox{}, domain.ErrNotFound
	}
	return sandbox, nil
}

func (s *fakeStore) UpdateSandbox(_ context.Context, id uuid.UUID, input domain.SandboxUpdate) (domain.Sandbox, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
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
	s.mu.Lock()
	defer s.mu.Unlock()
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
	s.mu.Lock()
	defer s.mu.Unlock()
	items := []domain.Sandbox{}
	for _, sandbox := range s.sandboxes {
		items = append(items, sandbox)
	}
	return items, nil
}

func (s *fakeStore) MarkSandboxRuntimeDeleted(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sandbox, ok := s.sandboxes[id]
	if !ok {
		return domain.ErrNotFound
	}
	sandbox.RuntimeRef = nil
	s.sandboxes[id] = sandbox
	return nil
}

func (s *fakeStore) ListExecutionTasks(_ context.Context, sandboxID uuid.UUID) ([]domain.ExecutionTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := []domain.ExecutionTask{}
	for _, task := range s.tasks {
		if task.SandboxID == sandboxID {
			items = append(items, task)
		}
	}
	return items, nil
}

func (s *fakeStore) CreateExecutionTask(_ context.Context, input domain.ExecutionTaskCreate) (domain.ExecutionTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if _, ok := s.projects[input.ProjectID]; !ok {
		return domain.ExecutionTask{}, domain.ErrNotFound
	}
	if _, ok := s.sandboxes[input.SandboxID]; !ok {
		return domain.ExecutionTask{}, domain.ErrNotFound
	}
	task := domain.ExecutionTask{
		ID:             uuid.New(),
		ProjectID:      input.ProjectID,
		SandboxID:      input.SandboxID,
		Status:         domain.ExecutionTaskStatusQueued,
		Command:        input.Command,
		TimeoutSeconds: input.TimeoutSeconds,
		RuntimeRef:     input.RuntimeRef,
		Metadata:       input.Metadata,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	s.tasks[task.ID] = task
	return task, nil
}

func (s *fakeStore) GetExecutionTask(_ context.Context, id uuid.UUID) (domain.ExecutionTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[id]
	if !ok {
		return domain.ExecutionTask{}, domain.ErrNotFound
	}
	return task, nil
}

func (s *fakeStore) UpdateExecutionTask(_ context.Context, id uuid.UUID, input domain.ExecutionTaskUpdate) (domain.ExecutionTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[id]
	if !ok {
		return domain.ExecutionTask{}, domain.ErrNotFound
	}
	if input.Status != nil {
		task.Status = *input.Status
	}
	if input.ExitCode != nil {
		value := *input.ExitCode
		task.ExitCode = &value
	}
	if input.Stdout != nil {
		task.Stdout = *input.Stdout
	}
	if input.Stderr != nil {
		task.Stderr = *input.Stderr
	}
	if input.OutputTruncated != nil {
		task.OutputTruncated = *input.OutputTruncated
	}
	if input.Error != nil {
		task.Error = *input.Error
	}
	if input.RuntimeRef != nil {
		task.RuntimeRef = nil
		if *input.RuntimeRef != nil {
			value := **input.RuntimeRef
			task.RuntimeRef = &value
		}
	}
	if input.StartedAt != nil {
		value := *input.StartedAt
		task.StartedAt = &value
	}
	if input.FinishedAt != nil {
		value := *input.FinishedAt
		task.FinishedAt = &value
	}
	task.UpdatedAt = time.Now()
	s.tasks[id] = task
	return task, nil
}

func (s *fakeStore) ListArtifacts(_ context.Context, sandboxID uuid.UUID, taskID *uuid.UUID) ([]domain.Artifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := []domain.Artifact{}
	for _, artifact := range s.artifacts {
		if artifact.SandboxID != sandboxID {
			continue
		}
		if taskID != nil && (artifact.TaskID == nil || *artifact.TaskID != *taskID) {
			continue
		}
		items = append(items, artifact)
	}
	return items, nil
}

func (s *fakeStore) CreateArtifact(_ context.Context, input domain.ArtifactCreate) (domain.Artifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if _, ok := s.projects[input.ProjectID]; !ok {
		return domain.Artifact{}, domain.ErrNotFound
	}
	if _, ok := s.sandboxes[input.SandboxID]; !ok {
		return domain.Artifact{}, domain.ErrNotFound
	}
	if input.TaskID != nil {
		task, ok := s.tasks[*input.TaskID]
		if !ok {
			return domain.Artifact{}, domain.ErrNotFound
		}
		if task.SandboxID != input.SandboxID {
			return domain.Artifact{}, domain.ErrConflict
		}
	}
	artifact := domain.Artifact{
		ID:          uuid.New(),
		ProjectID:   input.ProjectID,
		SandboxID:   input.SandboxID,
		TaskID:      input.TaskID,
		Kind:        input.Kind,
		Name:        input.Name,
		URI:         input.URI,
		ContentType: input.ContentType,
		SizeBytes:   input.SizeBytes,
		Metadata:    input.Metadata,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.artifacts[artifact.ID] = artifact
	return artifact, nil
}

func (s *fakeStore) GetArtifact(_ context.Context, id uuid.UUID) (domain.Artifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	artifact, ok := s.artifacts[id]
	if !ok {
		return domain.Artifact{}, domain.ErrNotFound
	}
	return artifact, nil
}

type fakeRuntimeAccess struct {
	lastTailLines    int64
	lastPreviewPort  int
	lastPreviewPath  string
	lastPreviewQuery string
	lastExecCommand  []string
	execExitCode     int
}

func (f *fakeRuntimeAccess) ResolveRuntime(context.Context, domain.RuntimeRef) (mboxruntime.RuntimeTarget, error) {
	return mboxruntime.RuntimeTarget{
		Namespace: "mbox-demo",
		PodName:   "pod-dev",
		Container: "workspace",
		Phase:     "Running",
		Selector:  "app=pod-dev",
		Storage: []mboxruntime.RuntimeStorage{{
			Name:      "workspace",
			MountPath: "/workspace",
			ClaimName: "workspace-dev",
			Phase:     "Bound",
			Capacity:  "1Gi",
		}},
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
	f.lastExecCommand = append([]string{}, options.Command...)
	if options.Stdout != nil && len(options.Command) > 0 {
		_, _ = fmt.Fprintf(options.Stdout, "exec:%s\n", strings.Join(options.Command, " "))
	}
	if f.execExitCode != 0 {
		if options.Stderr != nil {
			_, _ = fmt.Fprintln(options.Stderr, "command failed")
		}
		return k8sexec.CodeExitError{
			Err:  fmt.Errorf("command exited with status %d", f.execExitCode),
			Code: f.execExitCode,
		}
	}
	return nil
}

type blockingRuntimeAccess struct {
	fakeRuntimeAccess
	started   chan struct{}
	releaseCh chan struct{}
	once      sync.Once
}

func newBlockingRuntimeAccess() *blockingRuntimeAccess {
	return &blockingRuntimeAccess{
		started:   make(chan struct{}),
		releaseCh: make(chan struct{}),
	}
}

func (f *blockingRuntimeAccess) release() {
	f.once.Do(func() {
		close(f.releaseCh)
	})
}

func (f *blockingRuntimeAccess) Exec(ctx context.Context, _ domain.RuntimeRef, options mboxruntime.ExecOptions) error {
	f.lastExecCommand = append([]string{}, options.Command...)
	close(f.started)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-f.releaseCh:
		if options.Stdout != nil {
			_, _ = io.WriteString(options.Stdout, "released\n")
		}
		return nil
	}
}

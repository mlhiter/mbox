package controller

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/mlhiter/mbox/internal/domain"
	mboxruntime "github.com/mlhiter/mbox/internal/runtime"
)

func TestReconcilerCreatesRuntimeForPendingSandbox(t *testing.T) {
	store := newFakeStore()
	adapter := &fakeAdapter{}
	project := store.mustProject()
	template := store.mustTemplate(&project.ID)
	sandbox := store.mustSandbox(project.ID, template.ID)

	reconciler := NewSandboxReconciler(store, adapter, time.Second, nil)
	if err := reconciler.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}

	updated, err := store.GetSandbox(context.Background(), sandbox.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.RuntimeRef == nil {
		t.Fatal("expected runtime ref to be set")
	}
	if adapter.createCalls != 1 {
		t.Fatalf("expected 1 create call, got %d", adapter.createCalls)
	}
}

func TestReconcilerAppliesRuntimeStatus(t *testing.T) {
	store := newFakeStore()
	adapter := &fakeAdapter{status: mboxruntime.Status{
		Status: mboxruntime.RuntimeStatusRunning,
		Ports: []domain.SandboxPort{{
			Name:     "web",
			Port:     3000,
			Protocol: "TCP",
		}},
	}}
	project := store.mustProject()
	template := store.mustTemplate(&project.ID)
	sandbox := store.mustSandbox(project.ID, template.ID)
	ref := domain.RuntimeRef{Adapter: "agent-sandbox", Kind: "SandboxClaim", Namespace: sandbox.Namespace, Name: "runtime"}
	refPtr := &ref
	if _, err := store.UpdateSandbox(context.Background(), sandbox.ID, domain.SandboxUpdate{RuntimeRef: &refPtr}); err != nil {
		t.Fatal(err)
	}

	reconciler := NewSandboxReconciler(store, adapter, time.Second, nil)
	if err := reconciler.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}

	updated, err := store.GetSandbox(context.Background(), sandbox.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != domain.SandboxStatusRunning {
		t.Fatalf("expected running status, got %q", updated.Status)
	}
	if len(updated.Ports) != 1 || updated.Ports[0].Port != 3000 {
		t.Fatalf("expected runtime ports to be stored, got %+v", updated.Ports)
	}
	if adapter.statusCalls != 1 {
		t.Fatalf("expected 1 status call, got %d", adapter.statusCalls)
	}
}

func TestReconcilerDeletesRuntimeForDeletedSandbox(t *testing.T) {
	store := newFakeStore()
	adapter := &fakeAdapter{}
	project := store.mustProject()
	template := store.mustTemplate(&project.ID)
	sandbox := store.mustSandbox(project.ID, template.ID)
	ref := domain.RuntimeRef{Adapter: "agent-sandbox", Kind: "SandboxClaim", Namespace: sandbox.Namespace, Name: "runtime"}
	refPtr := &ref
	if _, err := store.UpdateSandbox(context.Background(), sandbox.ID, domain.SandboxUpdate{RuntimeRef: &refPtr}); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteSandbox(context.Background(), sandbox.ID); err != nil {
		t.Fatal(err)
	}

	reconciler := NewSandboxReconciler(store, adapter, time.Second, nil)
	if err := reconciler.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}

	updated := store.sandboxes[sandbox.ID]
	if updated.RuntimeRef != nil {
		t.Fatalf("expected runtime ref to be cleared, got %+v", updated.RuntimeRef)
	}
	if adapter.deleteCalls != 1 {
		t.Fatalf("expected 1 delete call, got %d", adapter.deleteCalls)
	}
}

func TestReconcilerStopsRuntimeForStoppedSandbox(t *testing.T) {
	store := newFakeStore()
	adapter := &fakeAdapter{}
	project := store.mustProject()
	template := store.mustTemplate(&project.ID)
	sandbox := store.mustSandbox(project.ID, template.ID)
	ref := domain.RuntimeRef{Adapter: "agent-sandbox", Kind: "SandboxClaim", Namespace: sandbox.Namespace, Name: "runtime"}
	refPtr := &ref
	stopped := domain.SandboxStatusStopped
	if _, err := store.UpdateSandbox(context.Background(), sandbox.ID, domain.SandboxUpdate{
		Status:     &stopped,
		RuntimeRef: &refPtr,
	}); err != nil {
		t.Fatal(err)
	}

	reconciler := NewSandboxReconciler(store, adapter, time.Second, nil)
	if err := reconciler.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}

	updated, err := store.GetSandbox(context.Background(), sandbox.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != domain.SandboxStatusStopped || updated.RuntimeRef == nil {
		t.Fatalf("expected stopped sandbox to keep runtime ref, got %+v", updated)
	}
	if adapter.stopCalls != 1 {
		t.Fatalf("expected 1 stop call, got %d", adapter.stopCalls)
	}
	if adapter.statusCalls != 0 {
		t.Fatalf("expected no status calls while stopped, got %d", adapter.statusCalls)
	}
}

func TestReconcilerStartsRuntimeForPendingSandboxWithRuntimeRef(t *testing.T) {
	store := newFakeStore()
	adapter := &fakeAdapter{}
	project := store.mustProject()
	template := store.mustTemplate(&project.ID)
	sandbox := store.mustSandbox(project.ID, template.ID)
	ref := domain.RuntimeRef{Adapter: "agent-sandbox", Kind: "SandboxClaim", Namespace: sandbox.Namespace, Name: "runtime"}
	refPtr := &ref
	if _, err := store.UpdateSandbox(context.Background(), sandbox.ID, domain.SandboxUpdate{RuntimeRef: &refPtr}); err != nil {
		t.Fatal(err)
	}

	reconciler := NewSandboxReconciler(store, adapter, time.Second, nil)
	if err := reconciler.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}

	if adapter.startCalls != 1 {
		t.Fatalf("expected 1 start call, got %d", adapter.startCalls)
	}
	if adapter.statusCalls != 1 {
		t.Fatalf("expected 1 status call, got %d", adapter.statusCalls)
	}
	if adapter.createCalls != 0 {
		t.Fatalf("expected no create calls, got %d", adapter.createCalls)
	}
}

type fakeAdapter struct {
	createCalls int
	startCalls  int
	stopCalls   int
	statusCalls int
	deleteCalls int
	status      mboxruntime.Status
}

func (a *fakeAdapter) CreateRuntime(context.Context, mboxruntime.CreateRequest) (domain.RuntimeRef, error) {
	a.createCalls++
	return domain.RuntimeRef{
		Adapter:   "agent-sandbox",
		Kind:      "SandboxClaim",
		Namespace: "mbox-test",
		Name:      "runtime",
	}, nil
}

func (a *fakeAdapter) DeleteRuntime(context.Context, domain.RuntimeRef) error {
	a.deleteCalls++
	return nil
}

func (a *fakeAdapter) StartRuntime(context.Context, domain.RuntimeRef) error {
	a.startCalls++
	return nil
}

func (a *fakeAdapter) StopRuntime(context.Context, domain.RuntimeRef) error {
	a.stopCalls++
	return nil
}

func (a *fakeAdapter) GetRuntimeStatus(context.Context, domain.RuntimeRef) (mboxruntime.Status, error) {
	a.statusCalls++
	if a.status.Status == "" {
		return mboxruntime.Status{Status: mboxruntime.RuntimeStatusPending}, nil
	}
	return a.status, nil
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

func (s *fakeStore) mustProject() domain.Project {
	project, _ := s.CreateProject(context.Background(), domain.ProjectCreate{
		Name:             "Demo",
		Slug:             "demo",
		DefaultNamespace: "mbox-demo",
	})
	return project
}

func (s *fakeStore) mustTemplate(projectID *uuid.UUID) domain.EnvironmentTemplate {
	template, _ := s.CreateTemplate(context.Background(), domain.TemplateCreate{
		ProjectID:  projectID,
		Name:       "Linux",
		Slug:       "linux",
		Image:      "ubuntu:24.04",
		WorkingDir: "/workspace",
	})
	return template
}

func (s *fakeStore) mustSandbox(projectID uuid.UUID, templateID uuid.UUID) domain.Sandbox {
	sandbox, _ := s.CreateSandbox(context.Background(), domain.SandboxCreate{
		ProjectID:          projectID,
		TemplateID:         templateID,
		Name:               "Dev",
		Slug:               "dev",
		Namespace:          "mbox-test",
		ServiceAccountName: "mbox-sandbox",
	})
	return sandbox
}

func (s *fakeStore) ListProjects(context.Context) ([]domain.Project, error) {
	return nil, nil
}

func (s *fakeStore) CreateProject(_ context.Context, input domain.ProjectCreate) (domain.Project, error) {
	now := time.Now()
	project := domain.Project{
		ID:               uuid.New(),
		Name:             input.Name,
		Slug:             input.Slug,
		DefaultNamespace: input.DefaultNamespace,
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

func (s *fakeStore) UpdateProject(context.Context, uuid.UUID, domain.ProjectUpdate) (domain.Project, error) {
	return domain.Project{}, nil
}

func (s *fakeStore) DeleteProject(context.Context, uuid.UUID) error {
	return nil
}

func (s *fakeStore) ListTemplates(context.Context, *uuid.UUID) ([]domain.EnvironmentTemplate, error) {
	return nil, nil
}

func (s *fakeStore) CreateTemplate(_ context.Context, input domain.TemplateCreate) (domain.EnvironmentTemplate, error) {
	now := time.Now()
	template := domain.EnvironmentTemplate{
		ID:         uuid.New(),
		ProjectID:  input.ProjectID,
		Name:       input.Name,
		Slug:       input.Slug,
		Image:      input.Image,
		WorkingDir: input.WorkingDir,
		CreatedAt:  now,
		UpdatedAt:  now,
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

func (s *fakeStore) UpdateTemplate(context.Context, uuid.UUID, domain.TemplateUpdate) (domain.EnvironmentTemplate, error) {
	return domain.EnvironmentTemplate{}, nil
}

func (s *fakeStore) DeleteTemplate(context.Context, uuid.UUID) error {
	return nil
}

func (s *fakeStore) ListSandboxes(context.Context, *uuid.UUID) ([]domain.Sandbox, error) {
	return nil, nil
}

func (s *fakeStore) CreateSandbox(_ context.Context, input domain.SandboxCreate) (domain.Sandbox, error) {
	now := time.Now()
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
	sandbox := s.sandboxes[id]
	if input.Status != nil {
		sandbox.Status = *input.Status
	}
	if input.RuntimeRef != nil {
		sandbox.RuntimeRef = *input.RuntimeRef
	}
	if input.Ports != nil {
		sandbox.Ports = *input.Ports
	}
	s.sandboxes[id] = sandbox
	return sandbox, nil
}

func (s *fakeStore) DeleteSandbox(_ context.Context, id uuid.UUID) error {
	sandbox := s.sandboxes[id]
	now := time.Now()
	sandbox.Status = domain.SandboxStatusDeleted
	sandbox.DeletedAt = &now
	s.sandboxes[id] = sandbox
	return nil
}

func (s *fakeStore) ListSandboxesForReconcile(context.Context) ([]domain.Sandbox, error) {
	items := []domain.Sandbox{}
	for _, sandbox := range s.sandboxes {
		if sandbox.Status == domain.SandboxStatusPending || sandbox.Status == domain.SandboxStatusRunning || sandbox.Status == domain.SandboxStatusFailed || (sandbox.Status == domain.SandboxStatusStopped && sandbox.RuntimeRef != nil) || (sandbox.DeletedAt != nil && sandbox.RuntimeRef != nil) {
			items = append(items, sandbox)
		}
	}
	return items, nil
}

func (s *fakeStore) MarkSandboxRuntimeDeleted(_ context.Context, id uuid.UUID) error {
	sandbox := s.sandboxes[id]
	sandbox.RuntimeRef = nil
	s.sandboxes[id] = sandbox
	return nil
}

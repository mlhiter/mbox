package postgres

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mlhiter/mbox/internal/domain"
)

func TestStoreIntegrationSandboxCRUD(t *testing.T) {
	databaseURL := os.Getenv("MBOX_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set MBOX_TEST_DATABASE_URL to run Postgres integration tests")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		TRUNCATE sandboxes, environment_templates, projects RESTART IDENTITY CASCADE
	`); err != nil {
		t.Fatal(err)
	}

	store := NewStore(pool)
	project, err := store.CreateProject(ctx, domain.ProjectCreate{
		Name:             "Integration Project",
		Slug:             "integration-project",
		DefaultNamespace: "mbox-integration",
	})
	if err != nil {
		t.Fatal(err)
	}
	template, err := store.CreateTemplate(ctx, domain.TemplateCreate{
		ProjectID:      &project.ID,
		Name:           "Ubuntu Terminal",
		Slug:           "ubuntu-terminal",
		Image:          "ubuntu:24.04",
		WorkingDir:     "/workspace",
		StorageRequest: "10Gi",
		ExposedPorts: []domain.TemplatePort{{
			Name:     "web",
			Port:     3000,
			Protocol: "TCP",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	sandbox, err := store.CreateSandbox(ctx, domain.SandboxCreate{
		ProjectID:          project.ID,
		TemplateID:         template.ID,
		Name:               "Dev",
		Slug:               "dev",
		Namespace:          "mbox-integration",
		ServiceAccountName: "mbox-sandbox",
	})
	if err != nil {
		t.Fatal(err)
	}
	if sandbox.Status != domain.SandboxStatusPending {
		t.Fatalf("expected pending, got %q", sandbox.Status)
	}

	running := domain.SandboxStatusRunning
	runtimeRef := &domain.RuntimeRef{
		Adapter:   "agent-sandbox",
		Kind:      "SandboxClaim",
		Namespace: "mbox-integration",
		Name:      "dev",
	}
	sandbox, err = store.UpdateSandbox(ctx, sandbox.ID, domain.SandboxUpdate{
		Status:     &running,
		RuntimeRef: &runtimeRef,
	})
	if err != nil {
		t.Fatal(err)
	}
	if sandbox.Status != domain.SandboxStatusRunning || sandbox.RuntimeRef == nil {
		t.Fatalf("unexpected updated sandbox: %+v", sandbox)
	}

	task, err := store.CreateExecutionTask(ctx, domain.ExecutionTaskCreate{
		ProjectID:      project.ID,
		SandboxID:      sandbox.ID,
		Command:        []string{"echo", "hello"},
		TimeoutSeconds: 30,
		RuntimeRef:     runtimeRef,
	})
	if err != nil {
		t.Fatal(err)
	}
	runningTask := domain.ExecutionTaskStatusRunning
	task, err = store.UpdateExecutionTask(ctx, task.ID, domain.ExecutionTaskUpdate{
		Status: &runningTask,
	})
	if err != nil {
		t.Fatal(err)
	}
	succeeded := domain.ExecutionTaskStatusSucceeded
	exitCode := 0
	stdout := "hello\n"
	truncated := false
	task, err = store.UpdateExecutionTask(ctx, task.ID, domain.ExecutionTaskUpdate{
		Status:          &succeeded,
		ExitCode:        &exitCode,
		Stdout:          &stdout,
		OutputTruncated: &truncated,
	})
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != domain.ExecutionTaskStatusSucceeded || task.ExitCode == nil || *task.ExitCode != 0 || task.Stdout != "hello\n" {
		t.Fatalf("unexpected execution task: %+v", task)
	}
	tasks, err := store.ListExecutionTasks(ctx, sandbox.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].ID != task.ID {
		t.Fatalf("unexpected task list: %+v", tasks)
	}

	sizeBytes := int64(256)
	artifact, err := store.CreateArtifact(ctx, domain.ArtifactCreate{
		ProjectID:   project.ID,
		SandboxID:   sandbox.ID,
		TaskID:      &task.ID,
		Kind:        domain.ArtifactKindReport,
		Name:        "Test report",
		URI:         "workspace:///workspace/reports/test.json",
		ContentType: "application/json",
		SizeBytes:   &sizeBytes,
	})
	if err != nil {
		t.Fatal(err)
	}
	if artifact.ID == uuid.Nil || artifact.TaskID == nil || *artifact.TaskID != task.ID || artifact.Kind != domain.ArtifactKindReport {
		t.Fatalf("unexpected artifact: %+v", artifact)
	}
	artifacts, err := store.ListArtifacts(ctx, sandbox.ID, &task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 1 || artifacts[0].ID != artifact.ID {
		t.Fatalf("unexpected artifact list: %+v", artifacts)
	}

	otherProject, err := store.CreateProject(ctx, domain.ProjectCreate{
		Name:             "Other Project",
		Slug:             "other-project",
		DefaultNamespace: "mbox-other",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateExecutionTask(ctx, domain.ExecutionTaskCreate{
		ProjectID:      otherProject.ID,
		SandboxID:      sandbox.ID,
		Command:        []string{"echo", "wrong-project"},
		TimeoutSeconds: 30,
	}); err != domain.ErrNotFound {
		t.Fatalf("expected ErrNotFound for task project/sandbox mismatch, got %v", err)
	}
	otherTemplate, err := store.CreateTemplate(ctx, domain.TemplateCreate{
		ProjectID:  &otherProject.ID,
		Name:       "Other Template",
		Slug:       "other-template",
		Image:      "ubuntu:24.04",
		WorkingDir: "/workspace",
	})
	if err != nil {
		t.Fatal(err)
	}
	otherSandbox, err := store.CreateSandbox(ctx, domain.SandboxCreate{
		ProjectID:          otherProject.ID,
		TemplateID:         otherTemplate.ID,
		Name:               "Other Dev",
		Slug:               "other-dev",
		Namespace:          "mbox-other",
		ServiceAccountName: "mbox-sandbox",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateArtifact(ctx, domain.ArtifactCreate{
		ProjectID: otherProject.ID,
		SandboxID: otherSandbox.ID,
		TaskID:    &task.ID,
		Kind:      domain.ArtifactKindReport,
		Name:      "Wrong task",
		URI:       "workspace:///workspace/reports/wrong.json",
	}); err != domain.ErrNotFound {
		t.Fatalf("expected ErrNotFound for artifact task/sandbox mismatch, got %v", err)
	}

	if err := store.DeleteSandbox(ctx, sandbox.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetSandbox(ctx, sandbox.ID); err != domain.ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

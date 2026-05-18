package postgres

import (
	"context"
	"os"
	"testing"

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

	if err := store.DeleteSandbox(ctx, sandbox.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetSandbox(ctx, sandbox.ID); err != domain.ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

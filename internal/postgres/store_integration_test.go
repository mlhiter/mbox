package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

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
	policy, err := store.UpsertProjectPolicy(ctx, project.ID, domain.ProjectPolicyUpsert{
		Enforcement:            domain.ProjectPolicyEnforcementEnforced,
		AllowedImagePrefixes:   []string{"ubuntu:", "busybox:"},
		AllowedServiceAccounts: []string{"mbox-sandbox"},
		AllowedSecretRefs:      []string{"git-token"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if policy.ProjectID != project.ID ||
		policy.Enforcement != domain.ProjectPolicyEnforcementEnforced ||
		len(policy.AllowedImagePrefixes) != 2 ||
		policy.AllowedServiceAccounts[0] != "mbox-sandbox" ||
		policy.AllowedSecretRefs[0] != "git-token" {
		t.Fatalf("unexpected project policy: %+v", policy)
	}
	fetchedPolicy, err := store.GetProjectPolicy(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if fetchedPolicy.Enforcement != policy.Enforcement || fetchedPolicy.AllowedImagePrefixes[1] != "busybox:" {
		t.Fatalf("unexpected fetched policy: %+v", fetchedPolicy)
	}
	credential, err := store.CreateProjectCredential(ctx, domain.ProjectCredentialCreate{
		ProjectID: project.ID,
		Name:      "GitHub App",
		Slug:      "github-app",
		Type:      domain.ProjectCredentialTypeGit,
		Target:    "https://github.com/mlhiter/mbox",
		SecretRef: domain.SecretRef{Name: "github-app-token", Key: "token"},
		Usage:     []string{"clone", "fetch"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if credential.ProjectID != project.ID ||
		credential.Type != domain.ProjectCredentialTypeGit ||
		credential.SecretRef.Name != "github-app-token" ||
		credential.Usage[0] != "clone" {
		t.Fatalf("unexpected credential: %+v", credential)
	}
	credentials, err := store.ListProjectCredentials(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(credentials) != 1 || credentials[0].ID != credential.ID {
		t.Fatalf("unexpected credential list: %+v", credentials)
	}
	fetchedCredential, err := store.GetProjectCredential(ctx, credential.ID)
	if err != nil {
		t.Fatal(err)
	}
	if fetchedCredential.SecretRef.Key != "token" {
		t.Fatalf("unexpected fetched credential: %+v", fetchedCredential)
	}
	globalTemplate, err := store.CreateTemplate(ctx, domain.TemplateCreate{
		Name:           "Global BusyBox",
		Slug:           "global-busybox",
		Image:          "busybox:1.36",
		WorkingDir:     "/workspace",
		CPURequest:     "250m",
		MemoryRequest:  "512Mi",
		StorageRequest: "1Gi",
	})
	if err != nil {
		t.Fatal(err)
	}
	if globalTemplate.ProjectID != nil {
		t.Fatalf("expected global template, got %+v", globalTemplate)
	}
	template, err := store.CreateTemplate(ctx, domain.TemplateCreate{
		ProjectID:      &project.ID,
		Name:           "Ubuntu Terminal",
		Slug:           "ubuntu-terminal",
		Image:          "ubuntu:24.04",
		WorkingDir:     "/workspace",
		CPURequest:     "500m",
		MemoryRequest:  "1Gi",
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
	failedTask, err := store.CreateExecutionTask(ctx, domain.ExecutionTaskCreate{
		ProjectID:      project.ID,
		SandboxID:      sandbox.ID,
		Command:        []string{"false"},
		TimeoutSeconds: 30,
		RuntimeRef:     runtimeRef,
	})
	if err != nil {
		t.Fatal(err)
	}
	failed := domain.ExecutionTaskStatusFailed
	failedTask, err = store.UpdateExecutionTask(ctx, failedTask.ID, domain.ExecutionTaskUpdate{
		Status: &failed,
	})
	if err != nil {
		t.Fatal(err)
	}
	if failedTask.Status != domain.ExecutionTaskStatusFailed {
		t.Fatalf("unexpected failed task: %+v", failedTask)
	}
	tasks, err := store.ListExecutionTasks(ctx, sandbox.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
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
	retained, err := store.CaptureArtifactContent(ctx, domain.ArtifactContentCapture{
		ArtifactID:  artifact.ID,
		Content:     []byte(`{"ok":true}`),
		ContentType: "application/json",
		SizeBytes:   int64(len(`{"ok":true}`)),
		SHA256:      "4062edaf750fb8074e7e83e0c9028c94e32468a8b6f1614774328ef045150f93",
		SourceURI:   artifact.URI,
	})
	if err != nil {
		t.Fatal(err)
	}
	if retained.RetainedContent == nil || retained.RetainedContent.SizeBytes != int64(len(`{"ok":true}`)) {
		t.Fatalf("unexpected retained artifact metadata: %+v", retained)
	}
	if retained.RetainedContent.StorageProvider != domain.ArtifactContentStorageProviderPostgres {
		t.Fatalf("unexpected retained content storage provider: %+v", retained.RetainedContent)
	}
	retainedS3, err := store.CaptureArtifactContent(ctx, domain.ArtifactContentCapture{
		ArtifactID:      artifact.ID,
		ContentType:     "application/json",
		SizeBytes:       2,
		SHA256:          "44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a",
		SourceURI:       artifact.URI,
		StorageProvider: domain.ArtifactContentStorageProviderS3,
		StorageKey:      "artifacts/" + artifact.ID.String() + "/44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if retainedS3.RetainedContent == nil ||
		retainedS3.RetainedContent.StorageProvider != domain.ArtifactContentStorageProviderS3 ||
		retainedS3.RetainedContent.StorageKey == "" {
		t.Fatalf("unexpected S3 retained content metadata: %+v", retainedS3.RetainedContent)
	}
	content, err := store.GetArtifactContent(ctx, artifact.ID)
	if err != nil {
		t.Fatal(err)
	}
	if content.StorageProvider != domain.ArtifactContentStorageProviderS3 || content.StorageKey != retainedS3.RetainedContent.StorageKey || content.Content != nil {
		t.Fatalf("unexpected retained content: %+v", content)
	}
	fetchedArtifact, err := store.GetArtifact(ctx, artifact.ID)
	if err != nil {
		t.Fatal(err)
	}
	if fetchedArtifact.RetainedContent == nil || fetchedArtifact.RetainedContent.SHA256 != content.SHA256 {
		t.Fatalf("expected fetched artifact to include retained metadata: %+v", fetchedArtifact)
	}
	session, err := store.CreateRuntimeSession(ctx, domain.RuntimeSessionCreate{
		ProjectID: project.ID,
		SandboxID: sandbox.ID,
		Type:      domain.RuntimeSessionTypeTerminal,
		Client:    "integration",
	})
	if err != nil {
		t.Fatal(err)
	}
	ended := domain.RuntimeSessionStatusEnded
	endedAt := session.StartedAt.Add(time.Second)
	session, err = store.UpdateRuntimeSession(ctx, session.ID, domain.RuntimeSessionUpdate{
		Status:  &ended,
		EndedAt: &endedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if session.Status != domain.RuntimeSessionStatusEnded || session.EndedAt == nil {
		t.Fatalf("unexpected runtime session: %+v", session)
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

	usage, err := store.GetProjectUsage(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if usage.Sandboxes.Total != 1 || usage.Sandboxes.Active != 1 || usage.Sandboxes.Running != 1 {
		t.Fatalf("unexpected sandbox usage before delete: %+v", usage.Sandboxes)
	}
	if usage.Sandboxes.ActiveRequests.Count != 1 ||
		usage.Sandboxes.ActiveRequests.CPU.Total != "500m" ||
		usage.Sandboxes.ActiveRequests.Memory.Total != "1Gi" ||
		usage.Sandboxes.ActiveRequests.Storage.Total != "10Gi" ||
		usage.Sandboxes.RunningRequests.Count != 1 ||
		usage.Sandboxes.RunningRequests.CPU.Total != "500m" {
		t.Fatalf("unexpected sandbox request usage before delete: %+v", usage.Sandboxes)
	}
	if usage.RuntimeSessions.Total != 1 || usage.RuntimeSessions.Ended != 1 || usage.RuntimeSessions.Terminal != 1 {
		t.Fatalf("unexpected session usage: %+v", usage.RuntimeSessions)
	}
	if usage.ExecutionTasks.Total != 2 || usage.ExecutionTasks.Succeeded != 1 || usage.ExecutionTasks.Failed != 1 {
		t.Fatalf("unexpected task usage: %+v", usage.ExecutionTasks)
	}
	if usage.Artifacts.Total != 1 || usage.Artifacts.Report != 1 || usage.Artifacts.ReferencedBytes != 256 ||
		usage.Artifacts.RetainedContent != 1 || usage.Artifacts.RetainedBytes != int64(len(`{"ok":true}`)) {
		t.Fatalf("unexpected artifact usage: %+v", usage.Artifacts)
	}
	if usage.Templates.ProjectScoped != 1 || usage.Templates.GlobalVisible != 1 ||
		!hasResourceUsage(usage.Templates.CPURequests, "500m", 1) ||
		!hasResourceUsage(usage.Templates.CPURequests, "250m", 1) {
		t.Fatalf("unexpected template usage: %+v", usage.Templates)
	}
	if usage.Credentials.Total != 1 || usage.Credentials.Git != 1 {
		t.Fatalf("unexpected credential usage: %+v", usage.Credentials)
	}
	if _, err := store.GetProjectUsage(ctx, uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing project usage, got %v", err)
	}

	if err := store.DeleteProject(ctx, project.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected ErrConflict while project sandbox cleanup is pending, got %v", err)
	}

	if err := store.DeleteSandbox(ctx, sandbox.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetSandbox(ctx, sandbox.ID); err != domain.ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
	usage, err = store.GetProjectUsage(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if usage.Sandboxes.Total != 1 || usage.Sandboxes.Deleted != 1 || usage.Sandboxes.CleanupPending != 1 {
		t.Fatalf("unexpected sandbox usage after soft delete: %+v", usage.Sandboxes)
	}
	if usage.Sandboxes.ActiveRequests.Count != 0 || usage.Sandboxes.RunningRequests.Count != 0 {
		t.Fatalf("expected deleted sandbox to be excluded from request usage: %+v", usage.Sandboxes)
	}
	if err := store.DeleteProject(ctx, project.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected ErrConflict until runtime ref is cleared, got %v", err)
	}
	if err := store.MarkSandboxRuntimeDeleted(ctx, sandbox.ID); err != nil {
		t.Fatal(err)
	}

	clearedRefSandbox, err := store.CreateSandbox(ctx, domain.SandboxCreate{
		ProjectID:          project.ID,
		TemplateID:         template.ID,
		Name:               "Cleared Runtime Ref",
		Slug:               "cleared-runtime-ref",
		Namespace:          "mbox-test",
		ServiceAccountName: "mbox-sandbox",
	})
	if err != nil {
		t.Fatal(err)
	}
	runtimeRef.Name = "cleared-runtime-ref"
	if _, err := store.UpdateSandbox(ctx, clearedRefSandbox.ID, domain.SandboxUpdate{RuntimeRef: &runtimeRef}); err != nil {
		t.Fatal(err)
	}
	nilRuntimeRef := (*domain.RuntimeRef)(nil)
	clearedRefSandbox, err = store.UpdateSandbox(ctx, clearedRefSandbox.ID, domain.SandboxUpdate{RuntimeRef: &nilRuntimeRef})
	if err != nil {
		t.Fatal(err)
	}
	if clearedRefSandbox.RuntimeRef != nil {
		t.Fatalf("expected runtime ref to scan as nil after explicit clear, got %+v", clearedRefSandbox.RuntimeRef)
	}
	if err := store.DeleteSandbox(ctx, clearedRefSandbox.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteProject(ctx, project.ID); err != nil {
		t.Fatalf("expected project delete after sandbox cleanup, got %v", err)
	}
}

func hasResourceUsage(values []domain.ResourceUsageValue, value string, count int) bool {
	for _, item := range values {
		if item.Value == value && item.Count == count {
			return true
		}
	}
	return false
}

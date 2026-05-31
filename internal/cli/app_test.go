package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectsCreatePostsExpectedPayload(t *testing.T) {
	var method string
	var path string
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"project-1","name":"Demo"}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: stderr})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"projects", "create",
		"--name", "Demo",
		"--namespace", "mbox-demo",
		"--slug", "demo",
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPost || path != "/v1/projects" {
		t.Fatalf("unexpected request %s %s", method, path)
	}
	if payload["name"] != "Demo" || payload["defaultNamespace"] != "mbox-demo" || payload["slug"] != "demo" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
	if !strings.Contains(stdout.String(), `"id": "project-1"`) {
		t.Fatalf("expected JSON response, got %q", stdout.String())
	}
}

func TestInfoUsesInfoRoute(t *testing.T) {
	var method string
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"mbox","apiVersion":"v1alpha1","capabilities":["sandboxes"]}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	if err := app.Run(context.Background(), []string{"--api-url", server.URL, "info"}); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || path != "/v1/info" {
		t.Fatalf("unexpected request %s %s", method, path)
	}
	if !strings.Contains(stdout.String(), `"apiVersion": "v1alpha1"`) {
		t.Fatalf("expected JSON info response, got %q", stdout.String())
	}
}

func TestCompatSucceedsForCompatibleInfo(t *testing.T) {
	var method string
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"name":"mbox",
			"apiVersion":"v1alpha1",
			"serverVersion":"test",
			"capabilities":["sandboxes","execution-tasks"],
			"compatibility":{"minimumCliApiVersion":"v1alpha1","minimumSdkApiVersion":"v1alpha1"}
		}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	if err := app.Run(context.Background(), []string{"--api-url", server.URL, "compat", "--require-capability", "sandboxes"}); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || path != "/v1/info" {
		t.Fatalf("unexpected request %s %s", method, path)
	}
	if !strings.Contains(stdout.String(), `"ok": true`) ||
		!strings.Contains(stdout.String(), `"clientApiVersion": "v1alpha1"`) ||
		!strings.Contains(stdout.String(), `"sandboxes"`) {
		t.Fatalf("expected compatible JSON response, got %q", stdout.String())
	}
}

func TestCompatFailsForUnsupportedClientVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"name":"mbox",
			"apiVersion":"v1alpha2",
			"serverVersion":"test",
			"capabilities":["sandboxes"],
			"compatibility":{"minimumCliApiVersion":"v1alpha2","minimumSdkApiVersion":"v1alpha2"}
		}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{"--api-url", server.URL, "compat", "--client-api-version", "v1alpha1"})
	if err == nil {
		t.Fatal("expected incompatible client version to fail")
	}
	if !strings.Contains(err.Error(), "not compatible") {
		t.Fatalf("expected compatibility error, got %v", err)
	}
	if !strings.Contains(stdout.String(), `"ok": false`) ||
		!strings.Contains(stdout.String(), `"minimumApiVersion": "v1alpha2"`) {
		t.Fatalf("expected incompatible JSON response, got %q", stdout.String())
	}
}

func TestCompatFailsForMissingRequiredCapability(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"name":"mbox",
			"apiVersion":"v1alpha1",
			"serverVersion":"test",
			"capabilities":["sandboxes"],
			"compatibility":{"minimumCliApiVersion":"v1alpha1","minimumSdkApiVersion":"v1alpha1"}
		}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"compat",
		"--require-capability", "sandboxes",
		"--require-capability", "task-events",
	})
	if err == nil {
		t.Fatal("expected missing capability to fail")
	}
	if !strings.Contains(err.Error(), "missing required capabilities") {
		t.Fatalf("expected missing capability error, got %v", err)
	}
	if !strings.Contains(stdout.String(), `"ok": false`) ||
		!strings.Contains(stdout.String(), `"missingCapabilities": [`) ||
		!strings.Contains(stdout.String(), `"task-events"`) {
		t.Fatalf("expected missing capability JSON response, got %q", stdout.String())
	}
}

func TestCheckCLICompatibilityVersionPolicy(t *testing.T) {
	base := apiInfo{
		APIVersion:   "v1beta1",
		Capabilities: []string{"sandboxes", "execution-tasks"},
	}
	base.Compatibility.MinimumCLIAPIVersion = "v1alpha2"
	base.Compatibility.MinimumSDKAPIVersion = "v1alpha2"

	tests := []struct {
		name                 string
		info                 apiInfo
		clientAPIVersion     string
		requiredCapabilities []string
		wantOK               bool
		wantMissing          []string
		wantMessage          string
	}{
		{
			name:             "same family beta client satisfies alpha minimum",
			info:             base,
			clientAPIVersion: "v1beta1",
			wantOK:           true,
		},
		{
			name:             "same family stable client satisfies beta minimum",
			info:             withMinimumCLI(base, "v1beta1"),
			clientAPIVersion: "v1",
			wantOK:           true,
		},
		{
			name:             "older prerelease fails minimum",
			info:             base,
			clientAPIVersion: "v1alpha1",
			wantOK:           false,
			wantMessage:      "server requires v1alpha2",
		},
		{
			name:             "cross major family fails",
			info:             withMinimumCLI(base, "v2alpha1"),
			clientAPIVersion: "v1",
			wantOK:           false,
			wantMessage:      "server requires v2alpha1",
		},
		{
			name:             "invalid version label fails",
			info:             base,
			clientAPIVersion: "dev",
			wantOK:           false,
			wantMessage:      "not compatible",
		},
		{
			name:                 "missing capabilities fail after version match",
			info:                 base,
			clientAPIVersion:     "v1beta1",
			requiredCapabilities: []string{"sandboxes", "task-events", "task-events", " "},
			wantOK:               false,
			wantMissing:          []string{"task-events"},
			wantMessage:          "missing required capabilities",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckCLICompatibility(tt.info, tt.clientAPIVersion, tt.requiredCapabilities)
			if result.OK != tt.wantOK {
				t.Fatalf("OK = %v, want %v; result: %#v", result.OK, tt.wantOK, result)
			}
			if strings.Join(result.MissingCapabilities, ",") != strings.Join(tt.wantMissing, ",") {
				t.Fatalf("missing capabilities = %#v, want %#v", result.MissingCapabilities, tt.wantMissing)
			}
			if tt.wantMessage != "" && !strings.Contains(result.Message, tt.wantMessage) {
				t.Fatalf("message %q does not contain %q", result.Message, tt.wantMessage)
			}
		})
	}
}

func withMinimumCLI(info apiInfo, minimum string) apiInfo {
	info.Compatibility.MinimumCLIAPIVersion = minimum
	return info
}

func TestOpenAPIUsesOpenAPIRoute(t *testing.T) {
	var method string
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"openapi":"3.1.0","info":{"title":"mbox API","version":"v1alpha1"},"paths":{"/v1/projects":{}}}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	if err := app.Run(context.Background(), []string{"--api-url", server.URL, "openapi"}); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || path != "/v1/openapi.json" {
		t.Fatalf("unexpected request %s %s", method, path)
	}
	if !strings.Contains(stdout.String(), `"openapi": "3.1.0"`) ||
		!strings.Contains(stdout.String(), `"/v1/projects"`) {
		t.Fatalf("expected JSON OpenAPI response, got %q", stdout.String())
	}
}

func TestRuntimeOrphansUsesRuntimeOrphansRoute(t *testing.T) {
	var method string
	var uri string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		uri = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"adapter":"agent-sandbox","resourceCount":0,"orphanCount":0,"expectedClean":true,"items":[]}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	if err := app.Run(context.Background(), []string{"--api-url", server.URL, "runtime", "orphans", "--namespace", "mbox-smoke", "--kind", "SandboxTemplate"}); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || uri != "/v1/runtime/orphans?kind=SandboxTemplate&namespace=mbox-smoke" {
		t.Fatalf("unexpected request %s %s", method, uri)
	}
	if !strings.Contains(stdout.String(), `"expectedClean": true`) {
		t.Fatalf("expected JSON runtime orphan response, got %q", stdout.String())
	}
}

func TestRuntimeResourcesUsesRuntimeResourcesRoute(t *testing.T) {
	var method string
	var uri string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		uri = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"adapter":"agent-sandbox","items":[{"kind":"SandboxClaim","namespace":"mbox-smoke","name":"claim"}]}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	if err := app.Run(context.Background(), []string{"--api-url", server.URL, "runtime", "resources", "--namespace", "mbox-smoke", "--kind", "SandboxClaim"}); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || uri != "/v1/runtime/resources?kind=SandboxClaim&namespace=mbox-smoke" {
		t.Fatalf("unexpected request %s %s", method, uri)
	}
	if !strings.Contains(stdout.String(), `"SandboxClaim"`) {
		t.Fatalf("expected JSON runtime resource response, got %q", stdout.String())
	}
}

func TestRuntimeCleanupOrphanPostsExpectedPayload(t *testing.T) {
	var method string
	var path string
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"deleted":true,"reason":"missing-sandbox-record"}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"runtime", "cleanup-orphan",
		"--adapter", "agent-sandbox",
		"--kind", "SandboxClaim",
		"--namespace", "mbox-old",
		"--name", "claim",
		"--reason", "missing-sandbox-record",
		"--confirm", "delete-orphan-runtime-resource",
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPost || path != "/v1/runtime/orphans/cleanup" {
		t.Fatalf("unexpected request %s %s", method, path)
	}
	resource, ok := payload["resource"].(map[string]any)
	if !ok || resource["adapter"] != "agent-sandbox" || resource["kind"] != "SandboxClaim" ||
		resource["namespace"] != "mbox-old" || resource["name"] != "claim" {
		t.Fatalf("unexpected resource payload: %#v", payload["resource"])
	}
	if payload["reason"] != "missing-sandbox-record" || payload["confirm"] != "delete-orphan-runtime-resource" ||
		payload["deleteOrphan"] != true {
		t.Fatalf("unexpected cleanup payload: %#v", payload)
	}
	if !strings.Contains(stdout.String(), `"deleted": true`) {
		t.Fatalf("expected JSON cleanup response, got %q", stdout.String())
	}
}

func TestProjectsSetPolicyPutsExpectedPayload(t *testing.T) {
	var method string
	var path string
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"projectId":"project-1","enforcement":"enforced"}`))
	}))
	defer server.Close()

	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"projects", "set-policy", "project-1",
		"--enforcement", "enforced",
		"--allowed-image-prefix", "busybox:",
		"--allowed-image-prefix", "node:",
		"--allowed-service-account", "mbox-sandbox",
		"--allowed-secret-ref", "git-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPut || path != "/v1/projects/project-1/policy" {
		t.Fatalf("unexpected request %s %s", method, path)
	}
	if payload["enforcement"] != "enforced" {
		t.Fatalf("unexpected enforcement: %#v", payload)
	}
	prefixes, ok := payload["allowedImagePrefixes"].([]any)
	if !ok || len(prefixes) != 2 || prefixes[0] != "busybox:" || prefixes[1] != "node:" {
		t.Fatalf("unexpected image prefixes: %#v", payload["allowedImagePrefixes"])
	}
	serviceAccounts, ok := payload["allowedServiceAccounts"].([]any)
	if !ok || len(serviceAccounts) != 1 || serviceAccounts[0] != "mbox-sandbox" {
		t.Fatalf("unexpected service accounts: %#v", payload["allowedServiceAccounts"])
	}
	secretRefs, ok := payload["allowedSecretRefs"].([]any)
	if !ok || len(secretRefs) != 1 || secretRefs[0] != "git-token" {
		t.Fatalf("unexpected secret refs: %#v", payload["allowedSecretRefs"])
	}
}

func TestProjectsPolicyUsesPolicyRoute(t *testing.T) {
	var method string
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"projectId":"project-1","enforcement":"disabled"}`))
	}))
	defer server.Close()

	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"projects", "policy", "project-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || path != "/v1/projects/project-1/policy" {
		t.Fatalf("unexpected request %s %s", method, path)
	}
}

func TestProjectsSetQuotaPolicyPutsExpectedPayload(t *testing.T) {
	var method string
	var path string
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"projectId":"project-1","enforcement":"enforced","maxActiveSandboxes":3,"maxRetainedArtifactBytes":1024}`))
	}))
	defer server.Close()

	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"projects", "set-quota-policy", "project-1",
		"--enforcement", "enforced",
		"--max-active-sandboxes", "3",
		"--max-retained-artifact-bytes", "1024",
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPut || path != "/v1/projects/project-1/quota-policy" {
		t.Fatalf("unexpected request %s %s", method, path)
	}
	if payload["enforcement"] != "enforced" || payload["maxActiveSandboxes"] != float64(3) ||
		payload["maxRetainedArtifactBytes"] != float64(1024) {
		t.Fatalf("unexpected quota payload: %#v", payload)
	}
}

func TestProjectsQuotaPolicyUsesQuotaPolicyRoute(t *testing.T) {
	var method string
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"projectId":"project-1","enforcement":"disabled"}`))
	}))
	defer server.Close()

	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"projects", "quota-policy", "project-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || path != "/v1/projects/project-1/quota-policy" {
		t.Fatalf("unexpected request %s %s", method, path)
	}
}

func TestProjectsUsageUsesUsageRoute(t *testing.T) {
	var method string
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"projectId":"project-1","sandboxes":{"running":1}}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"projects", "usage", "project-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || path != "/v1/projects/project-1/usage" {
		t.Fatalf("unexpected request %s %s", method, path)
	}
	if !strings.Contains(stdout.String(), `"running": 1`) {
		t.Fatalf("expected JSON usage response, got %q", stdout.String())
	}
}

func TestAuditEventsUsesAuditEventsRoute(t *testing.T) {
	var method string
	var uri string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		uri = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"action":"sandbox.created"}]}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"audit-events",
		"--project-id", "project-1",
		"--resource-type", "sandbox",
		"--resource-id", "sandbox-1",
		"--action", "sandbox.created",
		"--actor", "alice",
		"--source", "mbox-cli",
		"--filter-request-id", "cli-request-1",
		"--operation", "sandbox.launch",
		"--since", "2026-05-30T00:00:00Z",
		"--until", "2026-05-30T01:00:00Z",
		"--limit", "5",
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || uri != "/v1/audit-events?action=sandbox.created&actor=alice&limit=5&operation=sandbox.launch&projectId=project-1&requestId=cli-request-1&resourceId=sandbox-1&resourceType=sandbox&since=2026-05-30T00%3A00%3A00Z&source=mbox-cli&until=2026-05-30T01%3A00%3A00Z" {
		t.Fatalf("unexpected request %s %s", method, uri)
	}
	if !strings.Contains(stdout.String(), `"sandbox.created"`) {
		t.Fatalf("expected audit response, got %q", stdout.String())
	}
}

func TestProjectsAuditEventsUsesProjectAuditEventsRoute(t *testing.T) {
	var method string
	var uri string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		uri = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"projects", "audit-events", "project-1",
		"--action", "artifact.content.uploaded",
		"--resource-type", "artifact",
		"--actor", "agent-runner",
		"--source", "sdk",
		"--filter-request-id", "sdk-run-1",
		"--operation", "artifact.content.upload",
		"--since", "2026-05-30T00:00:00Z",
		"--until", "2026-05-30T01:00:00Z",
		"--limit", "10",
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || uri != "/v1/projects/project-1/audit-events?action=artifact.content.uploaded&actor=agent-runner&limit=10&operation=artifact.content.upload&requestId=sdk-run-1&resourceType=artifact&since=2026-05-30T00%3A00%3A00Z&source=sdk&until=2026-05-30T01%3A00%3A00Z" {
		t.Fatalf("unexpected request %s %s", method, uri)
	}
}

func TestProjectsAddCredentialPostsExpectedPayload(t *testing.T) {
	var method string
	var path string
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"credential-1","projectId":"project-1","name":"GitHub App"}`))
	}))
	defer server.Close()

	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"projects", "add-credential", "project-1",
		"--name", "GitHub App",
		"--slug", "github-app",
		"--type", "git",
		"--target", "https://github.com/mlhiter/mbox",
		"--secret-ref", "github-app-token",
		"--secret-key", "token",
		"--usage", "clone",
		"--usage", "fetch",
		"--metadata", `{"owner":"platform"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPost || path != "/v1/projects/project-1/credentials" {
		t.Fatalf("unexpected request %s %s", method, path)
	}
	if payload["name"] != "GitHub App" || payload["slug"] != "github-app" || payload["type"] != "git" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
	secretRef, ok := payload["secretRef"].(map[string]any)
	if !ok || secretRef["name"] != "github-app-token" || secretRef["key"] != "token" {
		t.Fatalf("unexpected secret ref: %#v", payload["secretRef"])
	}
	usage, ok := payload["usage"].([]any)
	if !ok || len(usage) != 2 || usage[0] != "clone" || usage[1] != "fetch" {
		t.Fatalf("unexpected usage: %#v", payload["usage"])
	}
}

func TestCredentialsGetAndDeleteUseCredentialRoute(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"credential-1"}`))
	}))
	defer server.Close()

	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	if err := app.Run(context.Background(), []string{"--api-url", server.URL, "credentials", "get", "credential-1"}); err != nil {
		t.Fatal(err)
	}
	if err := app.Run(context.Background(), []string{"--api-url", server.URL, "credentials", "delete", "credential-1"}); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 2 || requests[0] != "GET /v1/credentials/credential-1" || requests[1] != "DELETE /v1/credentials/credential-1" {
		t.Fatalf("unexpected requests: %#v", requests)
	}
}

func TestTasksCreateParsesCommaSeparatedCommand(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/sandboxes/sandbox-1/tasks" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"task-1","status":"queued"}`))
	}))
	defer server.Close()

	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"tasks", "create", "sandbox-1",
		"--command", "sh,-lc,echo ok",
		"--timeout", "45",
	})
	if err != nil {
		t.Fatal(err)
	}
	command, ok := payload["command"].([]any)
	if !ok {
		t.Fatalf("expected command array, got %#v", payload["command"])
	}
	if len(command) != 3 || command[0] != "sh" || command[1] != "-lc" || command[2] != "echo ok" {
		t.Fatalf("unexpected command: %#v", command)
	}
	if payload["timeoutSeconds"] != float64(45) {
		t.Fatalf("unexpected timeout: %#v", payload["timeoutSeconds"])
	}
}

func TestTasksCreateParsesRepeatedArgs(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"task-1","status":"queued"}`))
	}))
	defer server.Close()

	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"tasks", "create", "sandbox-1",
		"--arg", "sh",
		"--arg", "-lc",
		"--arg", "echo ok",
	})
	if err != nil {
		t.Fatal(err)
	}
	command, ok := payload["command"].([]any)
	if !ok {
		t.Fatalf("expected command array, got %#v", payload["command"])
	}
	if len(command) != 3 || command[0] != "sh" || command[1] != "-lc" || command[2] != "echo ok" {
		t.Fatalf("unexpected command: %#v", command)
	}
}

func TestTasksWaitPollsUntilTerminalStatus(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/tasks/task-1" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		requests++
		w.Header().Set("Content-Type", "application/json")
		if requests == 1 {
			_, _ = w.Write([]byte(`{"id":"task-1","status":"running"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"task-1","status":"succeeded","exitCode":0}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"tasks", "wait", "task-1",
		"--interval", "1ms",
		"--timeout", "1s",
	})
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
	if !strings.Contains(stdout.String(), `"status": "succeeded"`) {
		t.Fatalf("expected final task JSON, got %q", stdout.String())
	}
}

func TestTasksWaitTimesOut(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"task-1","status":"running"}`))
	}))
	defer server.Close()

	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"tasks", "wait", "task-1",
		"--interval", "1ms",
		"--timeout", "2ms",
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out waiting for task task-1") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTemplatesValidatePostsExpectedPayload(t *testing.T) {
	var method string
	var path string
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"template":{"id":"template-1"},"sandbox":{"id":"sandbox-1"}}`))
	}))
	defer server.Close()

	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"templates", "validate", "template-1",
		"--project-id", "project-1",
		"--name", "Validate Node",
		"--metadata", `{"caller":"cli-test"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPost || path != "/v1/templates/template-1/validation-runs" {
		t.Fatalf("unexpected request %s %s", method, path)
	}
	if payload["projectId"] != "project-1" || payload["name"] != "Validate Node" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
	metadata, ok := payload["metadata"].(map[string]any)
	if !ok || metadata["caller"] != "cli-test" {
		t.Fatalf("unexpected metadata: %#v", payload["metadata"])
	}
}

func TestTemplatesBoundaryUsesProjectQuery(t *testing.T) {
	var method string
	var rawQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		rawQuery = r.URL.RawQuery
		if r.URL.Path != "/v1/templates/template-1/boundary" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"kind":"template","templateId":"template-1"}`))
	}))
	defer server.Close()

	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"templates", "boundary", "template-1",
		"--project-id", "project-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || rawQuery != "projectId=project-1" {
		t.Fatalf("unexpected request method=%s query=%s", method, rawQuery)
	}
}

func TestSandboxesBoundaryUsesSandboxRoute(t *testing.T) {
	var method string
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"kind":"sandbox","sandboxId":"sandbox-1"}`))
	}))
	defer server.Close()

	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"sandboxes", "boundary", "sandbox-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || path != "/v1/sandboxes/sandbox-1/boundary" {
		t.Fatalf("unexpected request %s %s", method, path)
	}
}

func TestArtifactsCaptureUsesCaptureRoute(t *testing.T) {
	var method string
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"artifact-1","retainedContent":{"sizeBytes":7}}`))
	}))
	defer server.Close()

	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"artifacts", "capture", "artifact-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPost || path != "/v1/artifacts/artifact-1/capture" {
		t.Fatalf("unexpected request %s %s", method, path)
	}
}

func TestArtifactsUploadUsesContentRoute(t *testing.T) {
	var method string
	var path string
	var contentType string
	var body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		contentType = r.Header.Get("Content-Type")
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		body = string(data)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"artifact-1","retainedContent":{"sizeBytes":13}}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{
		Stdin:  strings.NewReader("client-report"),
		Stdout: stdout,
		Stderr: &bytes.Buffer{},
	})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"artifacts", "upload", "artifact-1",
		"--stdin",
		"--content-type", "text/plain",
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPut || path != "/v1/artifacts/artifact-1/content" {
		t.Fatalf("unexpected request %s %s", method, path)
	}
	if contentType != "text/plain" {
		t.Fatalf("unexpected content type %q", contentType)
	}
	if body != "client-report" {
		t.Fatalf("unexpected body %q", body)
	}
	if !strings.Contains(stdout.String(), `"retainedContent"`) {
		t.Fatalf("expected JSON response, got %q", stdout.String())
	}
}

func TestTemplatesDecideValidationPostsExpectedPayload(t *testing.T) {
	var method string
	var path string
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"template":{"id":"template-1"},"sandbox":{"id":"sandbox-1"}}`))
	}))
	defer server.Close()

	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"templates", "decide-validation", "template-1", "sandbox-1",
		"--status", "failed",
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPost || path != "/v1/templates/template-1/validation-runs/sandbox-1/decision" {
		t.Fatalf("unexpected request %s %s", method, path)
	}
	if payload["status"] != "failed" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestSandboxesCreateOmitsEmptyOptionalUUIDs(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"sandbox-1","name":"Demo"}`))
	}))
	defer server.Close()

	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"sandboxes", "create",
		"--project-id", "project-1",
		"--name", "Demo",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["templateId"]; ok {
		t.Fatalf("expected templateId to be omitted, got %#v", payload)
	}
	if _, ok := payload["metadata"]; ok {
		t.Fatalf("expected metadata to be omitted, got %#v", payload)
	}
}

func TestUsesBearerToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("unexpected Authorization header %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	if err := app.Run(context.Background(), []string{"--api-url", server.URL, "--token", "secret", "health"}); err != nil {
		t.Fatal(err)
	}
}

func TestUsesCLIContextFromConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer context-secret" {
			t.Fatalf("unexpected Authorization header %q", got)
		}
		if got := r.Header.Get("X-Mbox-Audit-Actor"); got != "context-user" {
			t.Fatalf("unexpected audit actor header %q", got)
		}
		if got := r.Header.Get("X-Mbox-Audit-Source"); got != "context-source" {
			t.Fatalf("unexpected audit source header %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	configPath := writeCLIConfig(t, map[string]any{
		"currentContext": "remote",
		"contexts": map[string]any{
			"local": map[string]any{
				"apiUrl": server.URL + "/wrong",
			},
			"remote": map[string]any{
				"apiUrl":      server.URL,
				"tokenEnv":    "MBOX_TEST_TOKEN",
				"auditActor":  "context-user",
				"auditSource": "context-source",
			},
		},
	})
	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	app.getenv = func(key string) string {
		if key == "MBOX_TEST_TOKEN" {
			return "context-secret"
		}
		return ""
	}

	if err := app.Run(context.Background(), []string{"--config", configPath, "health"}); err != nil {
		t.Fatal(err)
	}
}

func TestCLIContextCanBeSelectedAndOverridden(t *testing.T) {
	contextServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request to context server %s", r.URL.Path)
	}))
	defer contextServer.Close()

	overrideServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer flag-token" {
			t.Fatalf("unexpected Authorization header %q", got)
		}
		if got := r.Header.Get("X-Mbox-Audit-Actor"); got != "flag-user" {
			t.Fatalf("unexpected audit actor header %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer overrideServer.Close()

	configPath := writeCLIConfig(t, map[string]any{
		"currentContext": "local",
		"contexts": map[string]any{
			"remote": map[string]any{
				"apiUrl":     contextServer.URL,
				"token":      "context-token",
				"auditActor": "context-user",
			},
		},
	})
	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--config", configPath,
		"--context", "remote",
		"--api-url", overrideServer.URL,
		"--token", "flag-token",
		"--audit-actor", "flag-user",
		"health",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUsesDefaultConfigFromHome(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	home := t.TempDir()
	configDir := filepath.Join(home, ".mbox")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeJSONFile(t, filepath.Join(configDir, "config.json"), map[string]any{
		"currentContext": "local",
		"contexts": map[string]any{
			"local": map[string]any{"apiUrl": server.URL},
		},
	})
	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	app.homeDir = func() (string, error) { return home, nil }
	if err := app.Run(context.Background(), []string{"health"}); err != nil {
		t.Fatal(err)
	}
}

func TestContextCurrentRedactsToken(t *testing.T) {
	configPath := writeCLIConfig(t, map[string]any{
		"currentContext": "local",
		"contexts": map[string]any{
			"local": map[string]any{
				"apiUrl":      "http://127.0.0.1:18080",
				"token":       "secret",
				"auditActor":  "context-user",
				"auditSource": "context-source",
			},
		},
	})
	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	if err := app.Run(context.Background(), []string{"--config", configPath, "context", "current"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout.String(), "secret") {
		t.Fatalf("expected token to be redacted, got %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"name": "local"`) ||
		!strings.Contains(stdout.String(), `"hasToken": true`) ||
		!strings.Contains(stdout.String(), `"auditActor": "context-user"`) {
		t.Fatalf("unexpected context current output: %s", stdout.String())
	}
}

func TestContextListMarksCurrent(t *testing.T) {
	configPath := writeCLIConfig(t, map[string]any{
		"currentContext": "local",
		"contexts": map[string]any{
			"local": map[string]any{
				"apiUrl": "http://127.0.0.1:18080",
			},
			"remote": map[string]any{
				"apiUrl":   "https://mbox.example.test",
				"tokenEnv": "MBOX_TEST_TOKEN",
			},
		},
	})
	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	app.getenv = func(key string) string {
		if key == "MBOX_TEST_TOKEN" {
			return "secret"
		}
		return ""
	}
	if err := app.Run(context.Background(), []string{"--config", configPath, "--context", "remote", "context", "list"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout.String(), "secret") {
		t.Fatalf("expected token to be redacted, got %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"name": "remote"`) ||
		!strings.Contains(stdout.String(), `"current": true`) ||
		!strings.Contains(stdout.String(), `"hasToken": true`) {
		t.Fatalf("unexpected context list output: %s", stdout.String())
	}
}

func TestContextSetWritesConfigAndRedactsToken(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "nested", "config.json")
	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--config", configPath,
		"context", "set", "local",
		"--api-url", "http://127.0.0.1:18080",
		"--token", "secret",
		"--audit-actor", "local-user",
		"--audit-source", "mbox-cli",
		"--current",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout.String(), "secret") {
		t.Fatalf("expected token to be redacted, got %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"name": "local"`) ||
		!strings.Contains(stdout.String(), `"hasToken": true`) ||
		!strings.Contains(stdout.String(), `"auditActor": "local-user"`) {
		t.Fatalf("unexpected context set output: %s", stdout.String())
	}
	file := readCLIConfig(t, configPath)
	if file.CurrentContext != "local" {
		t.Fatalf("currentContext = %q, want local", file.CurrentContext)
	}
	entry := file.Contexts["local"]
	if entry.APIURL != "http://127.0.0.1:18080" || entry.Token != "secret" ||
		entry.AuditActor != "local-user" || entry.AuditSource != "mbox-cli" {
		t.Fatalf("unexpected context entry: %#v", entry)
	}
}

func TestContextSetTreatsEmptyConfigFileAsNewConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--config", configPath,
		"context", "set", "local",
		"--api-url", "http://127.0.0.1:18080",
	})
	if err != nil {
		t.Fatal(err)
	}
	file := readCLIConfig(t, configPath)
	if file.CurrentContext != "local" || file.Contexts["local"].APIURL != "http://127.0.0.1:18080" {
		t.Fatalf("unexpected context config: %#v", file)
	}
}

func TestContextSetAcceptsGlobalFlagStyle(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--config", configPath,
		"--api-url", "https://mbox.example.test",
		"--token", "flag-token",
		"context", "set", "remote",
		"--token-env", "MBOX_TOKEN",
	})
	if err == nil {
		t.Fatal("expected mutually exclusive token sources to fail")
	}
	err = app.Run(context.Background(), []string{
		"--config", configPath,
		"--api-url", "https://mbox.example.test",
		"--token", "flag-token",
		"--audit-actor", "flag-user",
		"context", "set", "remote",
	})
	if err != nil {
		t.Fatal(err)
	}
	file := readCLIConfig(t, configPath)
	entry := file.Contexts["remote"]
	if entry.APIURL != "https://mbox.example.test" || entry.Token != "flag-token" || entry.AuditActor != "flag-user" {
		t.Fatalf("unexpected context entry: %#v", entry)
	}
}

func TestContextUseAndRemoveUpdateCurrent(t *testing.T) {
	configPath := writeCLIConfig(t, map[string]any{
		"currentContext": "local",
		"contexts": map[string]any{
			"local":  map[string]any{"apiUrl": "http://127.0.0.1:18080"},
			"remote": map[string]any{"apiUrl": "https://mbox.example.test"},
		},
	})
	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	if err := app.Run(context.Background(), []string{"--config", configPath, "context", "use", "remote"}); err != nil {
		t.Fatal(err)
	}
	file := readCLIConfig(t, configPath)
	if file.CurrentContext != "remote" {
		t.Fatalf("currentContext = %q, want remote", file.CurrentContext)
	}
	stdout.Reset()
	if err := app.Run(context.Background(), []string{"--config", configPath, "context", "remove", "remote"}); err != nil {
		t.Fatal(err)
	}
	file = readCLIConfig(t, configPath)
	if _, ok := file.Contexts["remote"]; ok {
		t.Fatalf("expected remote context to be removed: %#v", file.Contexts)
	}
	if file.CurrentContext != "local" {
		t.Fatalf("currentContext = %q, want local", file.CurrentContext)
	}
	if !strings.Contains(stdout.String(), `"removed": true`) {
		t.Fatalf("unexpected context remove output: %s", stdout.String())
	}
}

func TestUsesAuditAttributionHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Mbox-Request-ID"); got != "cli-request-1" {
			t.Fatalf("unexpected request id header %q", got)
		}
		if got := r.Header.Get("X-Mbox-Audit-Actor"); got != "alice" {
			t.Fatalf("unexpected audit actor header %q", got)
		}
		if got := r.Header.Get("X-Mbox-Audit-Source"); got != "mbox-cli-test" {
			t.Fatalf("unexpected audit source header %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"--request-id", "cli-request-1",
		"--audit-actor", "alice",
		"--audit-source", "mbox-cli-test",
		"health",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUsesAuditAttributionEnv(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Mbox-Request-ID"); got != "env-request" {
			t.Fatalf("unexpected request id header %q", got)
		}
		if got := r.Header.Get("X-Mbox-Audit-Actor"); got != "env-user" {
			t.Fatalf("unexpected audit actor header %q", got)
		}
		if got := r.Header.Get("X-Mbox-Audit-Source"); got != "env-source" {
			t.Fatalf("unexpected audit source header %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	app.getenv = func(key string) string {
		switch key {
		case "MBOX_REQUEST_ID":
			return "env-request"
		case "MBOX_AUDIT_ACTOR":
			return "env-user"
		case "MBOX_AUDIT_SOURCE":
			return "env-source"
		default:
			return ""
		}
	}
	if err := app.Run(context.Background(), []string{"--api-url", server.URL, "health"}); err != nil {
		t.Fatal(err)
	}
}

func writeCLIConfig(t *testing.T, value map[string]any) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	writeJSONFile(t, path, value)
	return path
}

func readCLIConfig(t *testing.T, path string) contextConfigFile {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var file contextConfigFile
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	return file
}

func writeJSONFile(t *testing.T, path string, value map[string]any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

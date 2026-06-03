package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHelpListsAuditSummaryFlags(t *testing.T) {
	stderr := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: stderr})
	if err := app.Run(context.Background(), []string{"help"}); err != nil {
		t.Fatal(err)
	}
	output := stderr.String()
	for _, expected := range []string{
		"audit-events [--project-id PROJECT]",
		"[--limit N] [--summary] [--policy-denied-summary]",
		"projects audit-events <project-id>",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected help to include %q, got %q", expected, output)
		}
	}
}

func TestNoArgsPrintsHelpWithAuditSummaryFlags(t *testing.T) {
	stderr := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: stderr})
	err := app.Run(context.Background(), nil)
	if err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
	if !strings.Contains(stderr.String(), "[--limit N] [--summary] [--policy-denied-summary]") {
		t.Fatalf("expected no-args help to include audit summary flags, got %q", stderr.String())
	}
}

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

func TestAuthCallerUsesCallerRoute(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"authenticated":true,"authenticationRequired":true,"mode":"shared_token","principalType":"shared_token","principal":"shared-token","rbacTrusted":false,"projectRolesEnforced":false,"notes":["shared token"]}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	if err := app.Run(context.Background(), []string{"--api-url", server.URL, "auth", "caller"}); err != nil {
		t.Fatal(err)
	}
	if err := app.Run(context.Background(), []string{"--api-url", server.URL, "caller"}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(requests, ",") != "GET /v1/auth/caller,GET /v1/auth/caller" {
		t.Fatalf("unexpected requests: %#v", requests)
	}
	if !strings.Contains(stdout.String(), `"mode": "shared_token"`) ||
		!strings.Contains(stdout.String(), `"rbacTrusted": false`) {
		t.Fatalf("expected caller JSON response, got %q", stdout.String())
	}
}

func TestAuthCallerSummaryUsesCallerRoute(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"authenticated": true,
			"authenticationRequired": true,
			"mode": "trusted_header",
			"principalType": "automation",
			"principal": "nightly-runner",
			"rbacTrusted": true,
			"projectRolesEnforced": true,
			"notes": [
				"request included trusted principal headers",
				"project member roles are enforced for sandbox.launch, runtime.operate, artifact.write, policy.manage, credential.manage, and member.manage starter routes only"
			]
		}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	if err := app.Run(context.Background(), []string{"--api-url", server.URL, "auth", "caller", "--summary"}); err != nil {
		t.Fatal(err)
	}
	if err := app.Run(context.Background(), []string{"--api-url", server.URL, "caller", "--summary"}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(requests, ",") != "GET /v1/auth/caller,GET /v1/auth/caller" {
		t.Fatalf("unexpected requests: %#v", requests)
	}
	output := stdout.String()
	for _, expected := range []string{
		"CALLER BOUNDARY",
		"Caller\tautomation:nightly-runner (trusted_header, authenticated, auth required, rbac trusted, roles enforced)",
		"Mode\ttrusted_header",
		"Principal\tautomation:nightly-runner",
		"Authenticated\ttrue",
		"Authentication required\ttrue",
		"RBAC trusted\ttrue",
		"Project roles enforced\ttrue",
		"- request included trusted principal headers",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected caller summary to contain %q, got %q", expected, output)
		}
	}
	if strings.Contains(output, `"mode"`) || strings.Contains(output, `"rbacTrusted"`) {
		t.Fatalf("expected human summary output without raw caller JSON, got %q", output)
	}
}

func TestTasksUsageListsRunSubcommand(t *testing.T) {
	for _, args := range [][]string{
		{"tasks"},
		{"tasks", "mystery"},
	} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
			err := app.Run(context.Background(), args)
			if err == nil {
				t.Fatal("expected tasks usage error")
			}
			if !strings.Contains(err.Error(), "usage: mbox tasks list|create|run|get|cancel|watch|wait|artifacts") {
				t.Fatalf("expected tasks usage to list run subcommand, got %v", err)
			}
		})
	}
}

func TestTasksCreateUsageListsCommandInputModes(t *testing.T) {
	for _, args := range [][]string{
		{"tasks", "create"},
		{"tasks", "create", "sandbox-1", "unexpected"},
	} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
			err := app.Run(context.Background(), args)
			if err == nil {
				t.Fatal("expected tasks create usage error")
			}
			for _, expected := range []string{
				"usage: mbox tasks create <sandbox-id>",
				"--arg ARG",
				"--command CMD",
				"--command-json JSON",
			} {
				if !strings.Contains(err.Error(), expected) {
					t.Fatalf("expected tasks create usage to include %q, got %v", expected, err)
				}
			}
		})
	}
}

func TestTasksRunUsageListsCommandInputModes(t *testing.T) {
	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{"tasks", "run"})
	if err == nil {
		t.Fatal("expected tasks run usage error")
	}
	for _, expected := range []string{
		"usage: mbox tasks run <sandbox-id>",
		"--arg ARG",
		"--command CMD",
		"--command-json JSON",
		"-- COMMAND",
	} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("expected tasks run usage to include %q, got %v", expected, err)
		}
	}
}

func TestTemplatesValidateRunUsageListsCommandInputModes(t *testing.T) {
	for _, args := range [][]string{
		{"templates", "validate-run"},
		{"templates", "validate-run", "template-1", "--project-id", "project-1"},
	} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
			err := app.Run(context.Background(), args)
			if err == nil {
				t.Fatal("expected templates validate-run usage error")
			}
			for _, expected := range []string{
				"usage: mbox templates validate-run <template-id>",
				"--arg ARG",
				"--command CMD",
				"--command-json JSON",
				"-- COMMAND",
			} {
				if !strings.Contains(err.Error(), expected) {
					t.Fatalf("expected templates validate-run usage to include %q, got %v", expected, err)
				}
			}
		})
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
	if err := app.Run(context.Background(), []string{"--api-url", server.URL, "runtime", "orphans", "--namespace", "mbox-smoke", "--project-id", "project-1", "--kind", "SandboxTemplate"}); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || uri != "/v1/runtime/orphans?kind=SandboxTemplate&namespace=mbox-smoke&projectId=project-1" {
		t.Fatalf("unexpected request %s %s", method, uri)
	}
	if !strings.Contains(stdout.String(), `"expectedClean": true`) {
		t.Fatalf("expected JSON runtime orphan response, got %q", stdout.String())
	}
}

func TestRuntimeOrphansSummaryTablePrintsReadableAudit(t *testing.T) {
	var method string
	var uri string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		uri = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"adapter":"agent-sandbox",
			"checkedAt":"2026-06-02T08:00:00Z",
			"namespace":"mbox-smoke",
			"resourceCount":4,
			"orphanCount":2,
			"expectedClean":false,
			"items":[
				{
					"reason":"missing-sandbox-record",
					"resource":{"adapter":"agent-sandbox","kind":"SandboxClaim","namespace":"mbox-smoke","name":"claim-old"},
					"projectId":"project-1",
					"status":"deleted",
					"message":"sandbox product record was not found",
					"evidence":["label sandbox-id=sandbox-missing"]
				},
				{
					"reason":"cleanup-pending",
					"resource":{"adapter":"agent-sandbox","kind":"SandboxClaim","namespace":"mbox-smoke","name":"claim-cleanup"},
					"projectId":"project-1",
					"status":"deleted",
					"message":"sandbox cleanup is pending"
				}
			]
		}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	if err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"runtime", "orphans",
		"--summary-table",
		"--namespace", "mbox-smoke",
		"--project-id", "project-1",
		"--kind", "SandboxClaim",
	}); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || uri != "/v1/runtime/orphans?kind=SandboxClaim&namespace=mbox-smoke&projectId=project-1" {
		t.Fatalf("unexpected request %s %s", method, uri)
	}
	output := stdout.String()
	for _, expected := range []string{
		"RUNTIME ORPHANS SUMMARY",
		"adapter\tagent-sandbox",
		"checkedAt\t2026-06-02T08:00:00Z",
		"namespace\tmbox-smoke",
		"resources\t4",
		"orphans\t2",
		"expectedClean\tfalse",
		"BY REASON",
		"cleanup-pending\t1",
		"missing-sandbox-record\t1",
		"ORPHANS",
		"REASON\tRESOURCE\tPROJECT\tSTATUS\tMESSAGE",
		"cleanup-pending\tSandboxClaim mbox-smoke/claim-cleanup\tproject-1\tdeleted\tsandbox cleanup is pending",
		"missing-sandbox-record\tSandboxClaim mbox-smoke/claim-old\tproject-1\tdeleted\tsandbox product record was not found",
		"evidence\tlabel sandbox-id=sandbox-missing",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in orphan summary output, got %q", expected, output)
		}
	}
	if strings.Contains(output, `"items"`) || strings.Contains(output, `"expectedClean"`) {
		t.Fatalf("expected human-readable orphan summary without raw JSON, got %q", output)
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
	if err := app.Run(context.Background(), []string{"--api-url", server.URL, "runtime", "resources", "--namespace", "mbox-smoke", "--project-id", "project-1", "--kind", "SandboxClaim"}); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || uri != "/v1/runtime/resources?kind=SandboxClaim&namespace=mbox-smoke&projectId=project-1" {
		t.Fatalf("unexpected request %s %s", method, uri)
	}
	if !strings.Contains(stdout.String(), `"SandboxClaim"`) {
		t.Fatalf("expected JSON runtime resource response, got %q", stdout.String())
	}
}

func TestRuntimeResourcesSummaryPrintsSummaryOnly(t *testing.T) {
	var method string
	var uri string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		uri = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"adapter":"agent-sandbox",
			"summary":{
				"total":2,
				"workload":{
					"observedResources":1,
					"observedPods":1,
					"requests":{"cpu":"250m"}
				}
			},
			"items":[{"kind":"SandboxClaim","namespace":"mbox-smoke","name":"claim"}]
		}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	if err := app.Run(context.Background(), []string{"--api-url", server.URL, "runtime", "resources", "--summary", "--namespace", "mbox-smoke", "--project-id", "project-1", "--kind", "SandboxClaim"}); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || uri != "/v1/runtime/resources?kind=SandboxClaim&namespace=mbox-smoke&projectId=project-1" {
		t.Fatalf("unexpected request %s %s", method, uri)
	}
	output := stdout.String()
	if !strings.Contains(output, `"total": 2`) ||
		!strings.Contains(output, `"observedResources": 1`) ||
		!strings.Contains(output, `"cpu": "250m"`) ||
		strings.Contains(output, `"items"`) {
		t.Fatalf("expected summary-only runtime resource output, got %q", output)
	}
}

func TestRuntimeResourcesSummaryTablePrintsReadableSummary(t *testing.T) {
	var method string
	var uri string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		uri = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"adapter":"agent-sandbox",
			"summary":{
				"total":2,
				"byKind":[{"name":"SandboxClaim","count":1},{"name":"SandboxTemplate","count":1}],
				"byNamespace":[{"name":"mbox-smoke","count":1}],
				"byProject":[{"name":"project-1","count":1}],
				"byOwner":[{"name":"project/project-1/sandbox/sandbox-1","count":1},{"name":"template/template-1","count":1}],
				"workload":{
					"observedResources":1,
					"desiredPods":1,
					"observedPods":1,
					"runningPods":1,
					"containersReady":1,
					"containersTotal":1,
					"restartCount":0,
					"requests":{"cpu":"250m","memory":"512Mi"},
					"limits":{"cpu":"500m"},
					"storageCapacity":"2Gi",
					"storage":[{"phase":"Bound","count":1,"capacity":"2Gi"}]
				}
			},
			"items":[{"kind":"SandboxClaim","namespace":"mbox-smoke","name":"claim"}]
		}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	if err := app.Run(context.Background(), []string{"--api-url", server.URL, "runtime", "resources", "--summary-table", "--namespace", "mbox-smoke", "--project-id", "project-1", "--kind", "SandboxClaim"}); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || uri != "/v1/runtime/resources?kind=SandboxClaim&namespace=mbox-smoke&projectId=project-1" {
		t.Fatalf("unexpected request %s %s", method, uri)
	}
	output := stdout.String()
	for _, expected := range []string{
		"RUNTIME RESOURCES SUMMARY",
		"TOTAL\t2",
		"BY KIND",
		"SandboxClaim\t1",
		"BY PROJECT",
		"project-1\t1",
		"WORKLOAD",
		"observedResources\t1",
		"requests\tcpu=250m memory=512Mi",
		"storage\tBound=1(2Gi)",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in summary table output, got %q", expected, output)
		}
	}
	if strings.Contains(output, `"items"`) || strings.Contains(output, `"summary"`) {
		t.Fatalf("expected human-readable summary table without raw response JSON, got %q", output)
	}
}

func TestRuntimeResourcesSummaryTableCanResolveProjectNames(t *testing.T) {
	var requests []string
	projectID := "11111111-1111-4111-8111-111111111111"
	unknownProjectID := "22222222-2222-4222-8222-222222222222"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/runtime/resources":
			_, _ = w.Write([]byte(`{
				"adapter":"agent-sandbox",
				"summary":{
					"total":3,
					"byKind":[{"name":"SandboxClaim","count":3}],
					"byNamespace":[{"name":"mbox-alpha","count":2},{"name":"mbox-beta","count":1}],
					"byProject":[
						{"name":"` + projectID + `","count":2},
						{"name":"` + unknownProjectID + `","count":1}
					],
					"byOwner":[],
					"workload":{
						"observedResources":3,
						"desiredPods":3,
						"observedPods":3,
						"runningPods":3,
						"containersReady":3,
						"containersTotal":3,
						"restartCount":0,
						"requests":{"cpu":"750m"},
						"limits":{},
						"storage":[]
					}
				},
				"items":[]
			}`))
		case "/v1/projects":
			_, _ = w.Write([]byte(`{"items":[{"id":"` + projectID + `","name":"Runtime Alpha"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	if err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"runtime", "resources",
		"--summary-table",
		"--resolve-project-names",
	}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(requests, ",") != "GET /v1/runtime/resources,GET /v1/projects" {
		t.Fatalf("unexpected requests: %#v", requests)
	}
	output := stdout.String()
	for _, expected := range []string{
		"BY PROJECT",
		"Runtime Alpha (11111111...1111)\t2",
		unknownProjectID + "\t1",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in summary table output, got %q", expected, output)
		}
	}
	if strings.Contains(output, `"items"`) || strings.Contains(output, `"summary"`) {
		t.Fatalf("expected human-readable summary table without raw response JSON, got %q", output)
	}
}

func TestRuntimeResourcesSummaryFlagsAreMutuallyExclusive(t *testing.T) {
	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{"--api-url", "http://127.0.0.1:18080", "runtime", "resources", "--summary", "--summary-table"})
	if err == nil || !strings.Contains(err.Error(), "only one of --summary or --summary-table") {
		t.Fatalf("expected mutually exclusive summary flag error, got %v", err)
	}
}

func TestRuntimeResourcesResolveProjectNamesRequiresSummaryTable(t *testing.T) {
	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{"--api-url", "http://127.0.0.1:18080", "runtime", "resources", "--resolve-project-names"})
	if err == nil || !strings.Contains(err.Error(), "--resolve-project-names requires --summary-table") {
		t.Fatalf("expected resolve-project-names summary-table error, got %v", err)
	}
}

func TestRuntimeResourcesSummaryRequiresSummaryField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"adapter":"agent-sandbox","items":[]}`))
	}))
	defer server.Close()

	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{"--api-url", server.URL, "runtime", "resources", "--summary"})
	if err == nil || !strings.Contains(err.Error(), "runtime resources response did not include summary") {
		t.Fatalf("expected missing summary error, got %v", err)
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

func TestProjectsPolicySummaryPrintsReadablePolicy(t *testing.T) {
	var method string
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"projectId":"project-1",
			"enforcement":"enforced",
			"allowedImagePrefixes":["node:","busybox:"],
			"allowedServiceAccounts":["mbox-sandbox"],
			"allowedSecretRefs":["git-token"],
			"createdAt":"2026-06-02T09:00:00Z",
			"updatedAt":"2026-06-02T10:00:00Z"
		}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"projects", "policy", "project-1",
		"--summary",
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || path != "/v1/projects/project-1/policy" {
		t.Fatalf("unexpected request %s %s", method, path)
	}
	output := stdout.String()
	for _, expected := range []string{
		"PROJECT LAUNCH POLICY SUMMARY",
		"Project\tproject-1",
		"Enforcement\tenforced",
		"Allowed image prefixes\tbusybox:,node:",
		"Allowed service accounts\tmbox-sandbox",
		"Allowed secret refs\tgit-token",
		"Created at\t2026-06-02T09:00:00Z",
		"Updated at\t2026-06-02T10:00:00Z",
		"Boundary\tlaunch-policy gate for sandbox creation and template validation launches",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in policy summary output, got %q", expected, output)
		}
	}
	if strings.Contains(output, `"allowedImagePrefixes"`) || strings.Contains(output, `"projectId"`) {
		t.Fatalf("expected human-readable policy summary without raw JSON, got %q", output)
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

func TestProjectsQuotaPolicySummaryPrintsReadablePolicy(t *testing.T) {
	var method string
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"projectId":"project-1",
			"enforcement":"enforced",
			"maxActiveSandboxes":5,
			"maxRetainedArtifactBytes":1048576,
			"createdAt":"2026-06-02T09:00:00Z",
			"updatedAt":"2026-06-02T10:00:00Z"
		}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"projects", "quota-policy", "project-1",
		"--summary",
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || path != "/v1/projects/project-1/quota-policy" {
		t.Fatalf("unexpected request %s %s", method, path)
	}
	output := stdout.String()
	for _, expected := range []string{
		"PROJECT QUOTA POLICY SUMMARY",
		"Project\tproject-1",
		"Enforcement\tenforced",
		"Max active sandboxes\t5",
		"Max retained artifact bytes\t1048576",
		"Created at\t2026-06-02T09:00:00Z",
		"Updated at\t2026-06-02T10:00:00Z",
		"Boundary\tproduct-record quota gate for active sandbox count and retained artifact bytes",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in quota policy summary output, got %q", expected, output)
		}
	}
	if strings.Contains(output, `"maxActiveSandboxes"`) || strings.Contains(output, `"projectId"`) {
		t.Fatalf("expected human-readable quota policy summary without raw JSON, got %q", output)
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

func TestProjectsUsageSummaryPrintsReadableUsage(t *testing.T) {
	var method string
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"projectId":"project-1",
			"generatedAt":"2026-06-02T09:00:00Z",
			"sandboxes":{
				"total":4,
				"active":3,
				"pending":1,
				"running":2,
				"stopped":0,
				"failed":0,
				"deleted":1,
				"cleanupPending":1,
				"activeRequests":{
					"count":3,
					"cpu":{"total":"1500m","declared":3,"missing":0,"invalid":0},
					"memory":{"total":"3Gi","declared":3,"missing":0,"invalid":0},
					"storage":{"total":"6Gi","declared":2,"missing":1,"invalid":0}
				},
				"runningRequests":{
					"count":2,
					"cpu":{"total":"1000m","declared":2,"missing":0,"invalid":0},
					"memory":{"total":"2Gi","declared":2,"missing":0,"invalid":0},
					"storage":{"total":"4Gi","declared":2,"missing":0,"invalid":0}
				}
			},
			"runtimeSessions":{"total":3,"active":1,"ended":2,"failed":0,"terminal":1,"custom":2},
			"executionTasks":{"total":4,"queued":0,"running":1,"succeeded":2,"failed":1,"canceled":0,"timedOut":0},
			"artifacts":{"total":3,"retainedContent":2,"referencedBytes":4096,"retainedBytes":512,"report":1,"log":2},
			"templates":{
				"projectScoped":2,
				"globalVisible":1,
				"cpuRequests":[{"value":"250m","count":1},{"value":"500m","count":2}],
				"memoryRequests":[{"value":"512Mi","count":1},{"value":"1Gi","count":2}],
				"storageRequests":[{"value":"2Gi","count":3}]
			},
			"credentials":{"total":2,"registry":1,"ssh":1},
			"notes":["product-record usage only; not live metrics"]
		}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"projects", "usage", "project-1",
		"--summary",
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || path != "/v1/projects/project-1/usage" {
		t.Fatalf("unexpected request %s %s", method, path)
	}
	output := stdout.String()
	for _, expected := range []string{
		"PROJECT USAGE SUMMARY",
		"Project\tproject-1",
		"Generated at\t2026-06-02T09:00:00Z",
		"SANDBOXES",
		"total\t4",
		"active\t3",
		"cleanupPending\t1",
		"DECLARED REQUESTS",
		"active\tcount=3 cpu=1500m(declared=3 missing=0 invalid=0) memory=3Gi(declared=3 missing=0 invalid=0) storage=6Gi(declared=2 missing=1 invalid=0)",
		"running\tcount=2 cpu=1000m(declared=2 missing=0 invalid=0) memory=2Gi(declared=2 missing=0 invalid=0) storage=4Gi(declared=2 missing=0 invalid=0)",
		"RUNTIME SESSIONS",
		"types\tcustom=2 terminal=1",
		"EXECUTION TASKS",
		"succeeded\t2",
		"ARTIFACTS",
		"referencedBytes\t4096",
		"kinds\tlog=2 report=1",
		"TEMPLATES",
		"cpuRequests\t250m=1 500m=2",
		"CREDENTIAL REFERENCES",
		"types\tregistry=1 ssh=1",
		"NOTES",
		"- product-record usage only; not live metrics",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in usage summary output, got %q", expected, output)
		}
	}
	if strings.Contains(output, `"sandboxes"`) || strings.Contains(output, `"projectId"`) {
		t.Fatalf("expected human-readable usage summary without raw JSON, got %q", output)
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
		"--reason", "active sandbox quota exceeded",
		"--since", "2026-05-30T00:00:00Z",
		"--until", "2026-05-30T01:00:00Z",
		"--limit", "5",
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || uri != "/v1/audit-events?action=sandbox.created&actor=alice&limit=5&operation=sandbox.launch&projectId=project-1&reason=active+sandbox+quota+exceeded&requestId=cli-request-1&resourceId=sandbox-1&resourceType=sandbox&since=2026-05-30T00%3A00%3A00Z&source=mbox-cli&until=2026-05-30T01%3A00%3A00Z" {
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
		"--reason", "retained artifact quota exceeded",
		"--since", "2026-05-30T00:00:00Z",
		"--until", "2026-05-30T01:00:00Z",
		"--limit", "10",
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || uri != "/v1/projects/project-1/audit-events?action=artifact.content.uploaded&actor=agent-runner&limit=10&operation=artifact.content.upload&reason=retained+artifact+quota+exceeded&requestId=sdk-run-1&resourceType=artifact&since=2026-05-30T00%3A00%3A00Z&source=sdk&until=2026-05-30T01%3A00%3A00Z" {
		t.Fatalf("unexpected request %s %s", method, uri)
	}
}

func TestAuditEventsSummaryUsesExistingFilters(t *testing.T) {
	var method string
	var uri string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		uri = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"items": [
				{
					"action": "sandbox.created",
					"resourceType": "sandbox",
					"resourceName": "smoke sandbox",
					"actor": "alice",
					"source": "mbox-cli",
					"metadata": {"requestId": "req-create-1"},
					"createdAt": "2026-06-02T03:00:00Z"
				},
				{
					"action": "sandbox.created",
					"resourceType": "sandbox",
					"resourceName": "retry sandbox",
					"actor": "bob",
					"source": "sdk",
					"metadata": {"requestId": "req-create-2"},
					"createdAt": "2026-06-02T02:30:00Z"
				},
				{
					"action": "artifact.content.uploaded",
					"resourceType": "artifact",
					"resourceName": "report",
					"actor": "alice",
					"source": "mbox-cli",
					"createdAt": "2026-06-02T04:00:00Z"
				}
			]
		}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"audit-events",
		"--project-id", "project-1",
		"--summary",
		"--actor", "alice",
		"--source", "mbox-cli",
		"--limit", "20",
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || uri != "/v1/audit-events?actor=alice&limit=20&projectId=project-1&source=mbox-cli" {
		t.Fatalf("unexpected request %s %s", method, uri)
	}
	output := stdout.String()
	for _, expected := range []string{
		"AUDIT SUMMARY",
		"ACTION\tCOUNT\tLATEST\tRESOURCE TYPES\tACTORS\tSOURCES\tREQUEST IDS",
		"sandbox.created\t2\t2026-06-02T03:00:00Z\tsandbox\talice,bob\tmbox-cli,sdk\treq-create-1,req-create-2",
		"artifact.content.uploaded\t1\t2026-06-02T04:00:00Z\tartifact\talice\tmbox-cli\t-",
		"Summary\tread-only over returned best-effort audit events; not a transactional audit log or trusted identity source",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in audit summary output, got %q", expected, output)
		}
	}
	if strings.Contains(output, `"items"`) || strings.Contains(output, `"metadata"`) {
		t.Fatalf("expected human summary output without raw audit JSON, got %q", output)
	}
}

func TestProjectsAuditEventsSummaryUsesProjectRoute(t *testing.T) {
	var method string
	var uri string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		uri = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"projects", "audit-events", "project-1",
		"--summary",
		"--limit", "3",
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || uri != "/v1/projects/project-1/audit-events?limit=3" {
		t.Fatalf("unexpected request %s %s", method, uri)
	}
	if !strings.Contains(stdout.String(), "AUDIT SUMMARY\n  (none)\n") {
		t.Fatalf("expected empty audit summary, got %q", stdout.String())
	}
}

func TestAuditEventsPolicyDeniedSummaryUsesExistingFilters(t *testing.T) {
	var method string
	var uri string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		uri = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"items": [
				{
					"action": "policy.denied",
					"resourceType": "sandbox",
					"resourceName": "smoke sandbox",
					"actor": "cli-smoke",
					"source": "mbox-cli",
					"metadata": {"operation": "sandbox.launch", "reason": "active sandbox quota exceeded", "requestId": "cli-smoke-request"},
					"createdAt": "2026-06-02T03:00:00Z"
				},
				{
					"action": "policy.denied",
					"resourceType": "sandbox",
					"resourceName": "quota retry",
					"actor": "cli-smoke",
					"source": "mbox-cli",
					"metadata": {"operation": "sandbox.launch", "reason": "active sandbox quota exceeded", "requestId": "cli-retry-request"},
					"createdAt": "2026-06-02T02:30:00Z"
				},
				{
					"action": "sandbox.created",
					"resourceType": "sandbox",
					"resourceName": "ignored sandbox",
					"actor": "cli-smoke",
					"source": "mbox-cli",
					"metadata": {"operation": "sandbox.launch", "reason": "active sandbox quota exceeded"},
					"createdAt": "2026-06-02T04:00:00Z"
				}
			]
		}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"audit-events",
		"--project-id", "project-1",
		"--policy-denied-summary",
		"--operation", "sandbox.launch",
		"--reason", "active sandbox quota exceeded",
		"--actor", "cli-smoke",
		"--source", "mbox-cli",
		"--filter-request-id", "cli-smoke-request",
		"--since", "2026-06-02T00:00:00Z",
		"--until", "2026-06-02T04:00:00Z",
		"--limit", "20",
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || uri != "/v1/audit-events?action=policy.denied&actor=cli-smoke&limit=20&operation=sandbox.launch&projectId=project-1&reason=active+sandbox+quota+exceeded&requestId=cli-smoke-request&since=2026-06-02T00%3A00%3A00Z&source=mbox-cli&until=2026-06-02T04%3A00%3A00Z" {
		t.Fatalf("unexpected request %s %s", method, uri)
	}
	output := stdout.String()
	if !strings.Contains(output, "POLICY DENIED SUMMARY") ||
		!strings.Contains(output, "OPERATION\tREASON\tCOUNT\tLATEST\tACTORS\tSOURCES\tRESOURCES\tREQUEST IDS") ||
		!strings.Contains(output, "sandbox.launch\tactive sandbox quota exceeded\t2\t2026-06-02T03:00:00Z\tcli-smoke\tmbox-cli\tquota retry,smoke sandbox\tcli-retry-request,cli-smoke-request") {
		t.Fatalf("expected grouped policy denial summary, got %q", output)
	}
	if strings.Contains(output, `"items"`) || strings.Contains(output, "sandbox.created") || strings.Contains(output, "ignored sandbox") {
		t.Fatalf("expected human summary output without raw audit JSON, got %q", output)
	}
}

func TestProjectsAuditEventsPolicyDeniedSummaryUsesProjectRoute(t *testing.T) {
	var method string
	var uri string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		uri = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"projects", "audit-events", "project-1",
		"--policy-denied-summary",
		"--action", "policy.denied",
		"--operation", "runtime.logs",
		"--limit", "3",
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || uri != "/v1/projects/project-1/audit-events?action=policy.denied&limit=3&operation=runtime.logs" {
		t.Fatalf("unexpected request %s %s", method, uri)
	}
	if !strings.Contains(stdout.String(), "POLICY DENIED SUMMARY\n  (none)\n") {
		t.Fatalf("expected empty policy denial summary, got %q", stdout.String())
	}
}

func TestAuditEventsPolicyDeniedSummaryRejectsNonPolicyAction(t *testing.T) {
	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", "http://127.0.0.1:1",
		"audit-events",
		"--policy-denied-summary",
		"--action", "sandbox.created",
	})
	if err == nil || !strings.Contains(err.Error(), "requires action policy.denied") {
		t.Fatalf("expected policy denied summary action error, got %v", err)
	}
}

func TestAuditEventsSummaryFlagsAreMutuallyExclusive(t *testing.T) {
	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", "http://127.0.0.1:1",
		"audit-events",
		"--summary",
		"--policy-denied-summary",
	})
	if err == nil || !strings.Contains(err.Error(), "only one of --summary or --policy-denied-summary") {
		t.Fatalf("expected mutually exclusive audit summary error, got %v", err)
	}
}

func TestProjectsAuditEventsUsageListsSummaryFlags(t *testing.T) {
	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", "http://127.0.0.1:1",
		"projects",
		"audit-events",
	})
	if err == nil ||
		!strings.Contains(err.Error(), "projects audit-events <project-id>") ||
		!strings.Contains(err.Error(), "[--summary] [--policy-denied-summary]") {
		t.Fatalf("expected projects audit-events usage with summary flags, got %v", err)
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

func TestProjectCredentialsSummaryPrintsReadableReferences(t *testing.T) {
	var method string
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"items": [
				{
					"id":"credential-2",
					"projectId":"project-1",
					"name":"Registry Pull",
					"slug":"registry-pull",
					"type":"registry",
					"target":"registry.example.com",
					"secretRef":{"name":"registry-token","key":"password"},
					"usage":["pull","push"]
				},
				{
					"id":"credential-1",
					"projectId":"project-1",
					"name":"GitHub App",
					"slug":"github-app",
					"type":"git",
					"target":"https://github.com/mlhiter/mbox",
					"secretRef":{"name":"github-app-token","key":"token"},
					"usage":["clone"]
				}
			]
		}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	if err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"projects", "credentials", "project-1",
		"--summary",
	}); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || path != "/v1/projects/project-1/credentials" {
		t.Fatalf("unexpected request %s %s", method, path)
	}
	output := stdout.String()
	for _, expected := range []string{
		"PROJECT CREDENTIAL REFERENCES SUMMARY",
		"Project\tproject-1",
		"Total\t2",
		"Types\tgit=1 registry=1",
		"Usage\tclone=1 pull=1 push=1",
		"Secret refs\tgithub-app-token/token=1 registry-token/password=1",
		"CREDENTIAL REFERENCES",
		"TYPE\tNAME\tTARGET\tSECRET_REF\tUSAGE\tID",
		"git\tGitHub App\thttps://github.com/mlhiter/mbox\tgithub-app-token/token\tclone\tcredential-1",
		"registry\tRegistry Pull\tregistry.example.com\tregistry-token/password\tpull,push\tcredential-2",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in credentials summary output, got %q", expected, output)
		}
	}
	if strings.Contains(output, `"items"`) || strings.Contains(output, `"secretRef"`) {
		t.Fatalf("expected human-readable credentials summary without raw JSON, got %q", output)
	}
}

func TestProjectsAddMemberPostsExpectedPayload(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"id":"member-1","projectId":"project-1","principal":"alice@example.com","role":"operator"}`))
	}))
	defer server.Close()

	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"projects", "add-member", "project-1",
		"--principal-type", "user",
		"--principal", "alice@example.com",
		"--role", "operator",
		"--metadata", `{"source":"test"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPost || path != "/v1/projects/project-1/members" {
		t.Fatalf("unexpected request %s %s", method, path)
	}
	if payload["principalType"] != "user" || payload["principal"] != "alice@example.com" || payload["role"] != "operator" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
	metadata, ok := payload["metadata"].(map[string]any)
	if !ok || metadata["source"] != "test" {
		t.Fatalf("unexpected metadata: %#v", payload["metadata"])
	}
}

func TestProjectMembersListAndMemberGetDeleteUseMemberRoutes(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/projects/project-1/members" {
			_, _ = w.Write([]byte(`{"items":[{"id":"member-1"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"member-1"}`))
	}))
	defer server.Close()

	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	if err := app.Run(context.Background(), []string{"--api-url", server.URL, "projects", "members", "project-1"}); err != nil {
		t.Fatal(err)
	}
	if err := app.Run(context.Background(), []string{"--api-url", server.URL, "members", "get", "member-1"}); err != nil {
		t.Fatal(err)
	}
	if err := app.Run(context.Background(), []string{"--api-url", server.URL, "members", "delete", "member-1"}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(requests, ",") != "GET /v1/projects/project-1/members,GET /v1/members/member-1,DELETE /v1/members/member-1" {
		t.Fatalf("unexpected requests: %#v", requests)
	}
}

func TestProjectMembersSummaryPrintsReadableRoleRegistry(t *testing.T) {
	var method string
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"items": [
				{"id":"member-2","projectId":"project-1","principalType":"automation","principal":"nightly-runner","role":"operator"},
				{"id":"member-1","projectId":"project-1","principalType":"user","principal":"alice@example.com","role":"owner"},
				{"id":"member-3","projectId":"project-1","principalType":"service_account","principal":"reader","role":"viewer"}
			]
		}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	if err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"projects", "members", "project-1",
		"--summary",
	}); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || path != "/v1/projects/project-1/members" {
		t.Fatalf("unexpected request %s %s", method, path)
	}
	output := stdout.String()
	for _, expected := range []string{
		"PROJECT MEMBERS SUMMARY",
		"Project\tproject-1",
		"Total\t3",
		"Roles\toperator=1 owner=1 viewer=1",
		"Principal types\tautomation=1 service_account=1 user=1",
		"MEMBERS",
		"ROLE\tPRINCIPAL\tTYPE\tID",
		"operator\tnightly-runner\tautomation\tmember-2",
		"owner\talice@example.com\tuser\tmember-1",
		"viewer\treader\tservice_account\tmember-3",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in members summary output, got %q", expected, output)
		}
	}
	if strings.Contains(output, `"items"`) || strings.Contains(output, `"principalType"`) {
		t.Fatalf("expected human-readable members summary without raw JSON, got %q", output)
	}
}

func TestProjectAuthorizationUsesPreflightRoute(t *testing.T) {
	var method string
	var path string
	var action string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		action = r.URL.Query().Get("action")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"projectId":"project-1","action":"sandbox.launch","allowed":false,"enforced":false,"evaluation":"not_enforceable","requiredRoles":["owner","operator"],"caller":{"authenticated":false,"authenticationRequired":false,"mode":"anonymous","principalType":"anonymous","principal":"anonymous","rbacTrusted":false,"projectRolesEnforced":false,"notes":[]},"memberCount":0,"availableActions":["project.view","sandbox.launch"],"notes":["route-level project RBAC is not enforced yet"]}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	if err := app.Run(context.Background(), []string{"--api-url", server.URL, "projects", "authorization", "project-1", "--action", "sandbox.launch"}); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || path != "/v1/projects/project-1/authorization" || action != "sandbox.launch" {
		t.Fatalf("unexpected request %s %s?action=%s", method, path, action)
	}
	if !strings.Contains(stdout.String(), `"evaluation": "not_enforceable"`) ||
		!strings.Contains(stdout.String(), `"requiredRoles": [`) {
		t.Fatalf("expected authorization preflight JSON response, got %q", stdout.String())
	}
}

func TestProjectAuthorizationSummaryUsesPreflightRoute(t *testing.T) {
	var method string
	var uri string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		uri = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"projectId": "project-1",
			"action": "member.manage",
			"allowed": true,
			"enforced": true,
			"evaluation": "allowed",
			"requiredRoles": ["owner"],
			"caller": {
				"authenticated": true,
				"authenticationRequired": false,
				"mode": "trusted_header",
				"principalType": "user",
				"principal": "alice@example.com",
				"rbacTrusted": true,
				"projectRolesEnforced": true,
				"notes": ["trusted principal headers are enabled"]
			},
			"matchedMember": {
				"id": "member-1",
				"principalType": "user",
				"principal": "alice@example.com",
				"role": "owner"
			},
			"memberCount": 2,
			"availableActions": ["project.view", "member.manage"],
			"notes": ["caller matches a project member role and this action is route-enforced"]
		}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	if err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"projects", "authorization", "project-1",
		"--action", "member.manage",
		"--summary",
	}); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || uri != "/v1/projects/project-1/authorization?action=member.manage" {
		t.Fatalf("unexpected request %s %s", method, uri)
	}
	output := stdout.String()
	for _, expected := range []string{
		"PROJECT AUTHORIZATION",
		"Project\tproject-1",
		"Action\tmember.manage",
		"Decision\tallowed / route enforced",
		"Required roles\towner",
		"Caller\tuser:alice@example.com (trusted_header, authenticated, rbac trusted, roles enforced)",
		"Matched member\tuser:alice@example.com role=owner id=member-1",
		"Member records\t2",
		"Available actions\tproject.view,member.manage",
		"- caller matches a project member role and this action is route-enforced",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected authorization summary to contain %q, got %q", expected, output)
		}
	}
	if strings.Contains(output, `"projectId"`) || strings.Contains(output, `"matchedMember"`) {
		t.Fatalf("expected human summary output without raw authorization JSON, got %q", output)
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

func TestTasksArtifactsUsesTaskArtifactsRoute(t *testing.T) {
	var method string
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"id":"artifact-1","taskId":"task-1","kind":"report"}]}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"tasks", "artifacts", "task-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || path != "/v1/tasks/task-1/artifacts" {
		t.Fatalf("unexpected request %s %s", method, path)
	}
	if !strings.Contains(stdout.String(), `"artifact-1"`) {
		t.Fatalf("expected JSON response, got %q", stdout.String())
	}
}

func TestTasksRunCreatesAndWaitsForTask(t *testing.T) {
	var requests []string
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/sandboxes/sandbox-1/tasks":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected create method %s", r.Method)
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"task-1","status":"queued"}`))
		case "/v1/tasks/task-1":
			if r.Method != http.MethodGet {
				t.Fatalf("unexpected get method %s", r.Method)
			}
			_, _ = w.Write([]byte(`{"id":"task-1","status":"succeeded","exitCode":0}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"tasks", "run", "sandbox-1",
		"--timeout", "45",
		"--interval", "1ms",
		"--wait-timeout", "1s",
		"--metadata", `{"source":"cli-test"}`,
		"--",
		"sh", "-lc", "echo ok",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(requests, ",") != "POST /v1/sandboxes/sandbox-1/tasks,GET /v1/tasks/task-1" {
		t.Fatalf("unexpected requests: %#v", requests)
	}
	command, ok := payload["command"].([]any)
	if !ok || len(command) != 3 || command[0] != "sh" || command[1] != "-lc" || command[2] != "echo ok" {
		t.Fatalf("unexpected command: %#v", payload["command"])
	}
	if payload["timeoutSeconds"] != float64(45) {
		t.Fatalf("unexpected task timeout: %#v", payload["timeoutSeconds"])
	}
	metadata, ok := payload["metadata"].(map[string]any)
	if !ok || metadata["source"] != "cli-test" {
		t.Fatalf("unexpected metadata: %#v", payload["metadata"])
	}
	if !strings.Contains(stdout.String(), `"status": "succeeded"`) || strings.Contains(stdout.String(), `"status": "queued"`) {
		t.Fatalf("expected only final task JSON, got %q", stdout.String())
	}
}

func TestTasksRunRequireSuccessReturnsErrorForFailedTask(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/sandboxes/sandbox-1/tasks":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"task-1","status":"queued"}`))
		case "/v1/tasks/task-1":
			_, _ = w.Write([]byte(`{"id":"task-1","status":"failed","exitCode":7}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"tasks", "run", "sandbox-1",
		"--interval", "1ms",
		"--wait-timeout", "1s",
		"--require-success",
		"--arg", "sh",
		"--arg", "-lc",
		"--arg", "exit 7",
	})
	if err == nil {
		t.Fatal("expected failed task to return an error")
	}
	if !strings.Contains(err.Error(), "task task-1 finished with status failed") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), `"status": "failed"`) {
		t.Fatalf("expected final task JSON before error, got %q", stdout.String())
	}
}

func TestTasksRunRejectsMixedCommandInputs(t *testing.T) {
	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", "http://127.0.0.1:18080",
		"tasks", "run", "sandbox-1",
		"--arg", "sh",
		"--",
		"echo", "ok",
	})
	if err == nil {
		t.Fatal("expected mixed command inputs to return an error")
	}
	if !strings.Contains(err.Error(), "use only one of --arg, --command, --command-json, or positional command after --") {
		t.Fatalf("unexpected error: %v", err)
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

func TestTasksWaitRequireSuccessReturnsErrorForUnsuccessfulTerminalStatuses(t *testing.T) {
	for _, status := range []string{"failed", "canceled", "timed_out"} {
		t.Run(status, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/v1/tasks/task-1" {
					t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":"task-1","status":"` + status + `","exitCode":2}`))
			}))
			defer server.Close()

			stdout := &bytes.Buffer{}
			app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
			err := app.Run(context.Background(), []string{
				"--api-url", server.URL,
				"tasks", "wait", "task-1",
				"--interval", "1ms",
				"--timeout", "1s",
				"--require-success",
			})
			if err == nil {
				t.Fatalf("expected %s task to return an error", status)
			}
			if !strings.Contains(err.Error(), "task task-1 finished with status "+status) {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(stdout.String(), `"status": "`+status+`"`) {
				t.Fatalf("expected final task JSON before error, got %q", stdout.String())
			}
		})
	}
}

func TestTasksWaitRequireSuccessAllowsSucceededTask(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/tasks/task-1" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"task-1","status":"succeeded","exitCode":0}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"tasks", "wait",
		"--require-success",
		"--interval", "1ms",
		"--timeout", "1s",
		"task-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), `"status": "succeeded"`) {
		t.Fatalf("expected final task JSON, got %q", stdout.String())
	}
}

func TestTasksWaitWithoutRequireSuccessAllowsFailedTask(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/tasks/task-1" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"task-1","status":"failed","exitCode":2}`))
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
	if !strings.Contains(stdout.String(), `"status": "failed"`) {
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

func TestTemplatesValidateRunRunsTaskAndDecidesPassed(t *testing.T) {
	var calls []string
	var validationPayload map[string]any
	var taskPayload map[string]any
	var decisionPayload map[string]any
	var taskPolls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/templates/template-1/validation-runs":
			if err := json.NewDecoder(r.Body).Decode(&validationPayload); err != nil {
				t.Fatal(err)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"template":{"id":"template-1"},"sandbox":{"id":"sandbox-1","status":"pending"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/sandboxes/sandbox-1":
			_, _ = w.Write([]byte(`{"id":"sandbox-1","status":"running","runtimeRef":{"name":"claim-1","namespace":"mbox-smoke"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/sandboxes/sandbox-1/tasks":
			if err := json.NewDecoder(r.Body).Decode(&taskPayload); err != nil {
				t.Fatal(err)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"task-1","status":"queued"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/tasks/task-1":
			taskPolls++
			if taskPolls == 1 {
				_, _ = w.Write([]byte(`{"id":"task-1","status":"running"}`))
				return
			}
			_, _ = w.Write([]byte(`{"id":"task-1","status":"succeeded","stdout":"ok"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/templates/template-1/validation-runs/sandbox-1/decision":
			if err := json.NewDecoder(r.Body).Decode(&decisionPayload); err != nil {
				t.Fatal(err)
			}
			_, _ = w.Write([]byte(`{"template":{"id":"template-1","metadata":{"validationStatus":"passed"}},"sandbox":{"id":"sandbox-1","metadata":{"validationResult":"passed"}}}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"templates", "validate-run", "template-1",
		"--project-id", "project-1",
		"--name", "Validate Node",
		"--metadata", `{"caller":"cli-test"}`,
		"--task-metadata", `{"kind":"smoke"}`,
		"--interval", "1ms",
		"--wait-timeout", "1s",
		"--task-timeout", "30",
		"--require-success",
		"--",
		"sh", "-lc", "echo ok",
	})
	if err != nil {
		t.Fatal(err)
	}
	wantCalls := []string{
		"POST /v1/templates/template-1/validation-runs",
		"GET /v1/sandboxes/sandbox-1",
		"POST /v1/sandboxes/sandbox-1/tasks",
		"GET /v1/tasks/task-1",
		"GET /v1/tasks/task-1",
		"POST /v1/templates/template-1/validation-runs/sandbox-1/decision",
	}
	if strings.Join(calls, "\n") != strings.Join(wantCalls, "\n") {
		t.Fatalf("unexpected calls:\n%s", strings.Join(calls, "\n"))
	}
	if validationPayload["projectId"] != "project-1" || validationPayload["name"] != "Validate Node" {
		t.Fatalf("unexpected validation payload: %#v", validationPayload)
	}
	if metadata, ok := validationPayload["metadata"].(map[string]any); !ok || metadata["caller"] != "cli-test" {
		t.Fatalf("unexpected validation metadata: %#v", validationPayload["metadata"])
	}
	if taskPayload["timeoutSeconds"] != float64(30) {
		t.Fatalf("unexpected task payload: %#v", taskPayload)
	}
	command, ok := taskPayload["command"].([]any)
	if !ok || len(command) != 3 || command[0] != "sh" || command[2] != "echo ok" {
		t.Fatalf("unexpected task command: %#v", taskPayload["command"])
	}
	if metadata, ok := taskPayload["metadata"].(map[string]any); !ok || metadata["kind"] != "smoke" {
		t.Fatalf("unexpected task metadata: %#v", taskPayload["metadata"])
	}
	if decisionPayload["status"] != "passed" {
		t.Fatalf("unexpected decision payload: %#v", decisionPayload)
	}
	if !strings.Contains(stdout.String(), `"status": "passed"`) || !strings.Contains(stdout.String(), `"task"`) {
		t.Fatalf("expected combined JSON output, got %q", stdout.String())
	}
}

func TestTemplatesValidateRunDecidesFailedForFailedTask(t *testing.T) {
	var decisionPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/templates/template-1/validation-runs":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"template":{"id":"template-1"},"sandbox":{"id":"sandbox-1","status":"pending"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/sandboxes/sandbox-1":
			_, _ = w.Write([]byte(`{"id":"sandbox-1","status":"running","runtimeRef":{"name":"claim-1","namespace":"mbox-smoke"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/sandboxes/sandbox-1/tasks":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"task-1","status":"queued"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/tasks/task-1":
			_, _ = w.Write([]byte(`{"id":"task-1","status":"failed","exitCode":7}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/templates/template-1/validation-runs/sandbox-1/decision":
			if err := json.NewDecoder(r.Body).Decode(&decisionPayload); err != nil {
				t.Fatal(err)
			}
			_, _ = w.Write([]byte(`{"template":{"id":"template-1","metadata":{"validationStatus":"failed"}},"sandbox":{"id":"sandbox-1","metadata":{"validationResult":"failed"}}}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"templates", "validate-run", "template-1",
		"--project-id", "project-1",
		"--interval", "1ms",
		"--wait-timeout", "1s",
		"--require-success",
		"--command", "sh,-lc,exit 7",
	})
	if err == nil {
		t.Fatal("expected failed validation task to return an error")
	}
	if !strings.Contains(err.Error(), "template validation task task-1 finished with status failed") {
		t.Fatalf("unexpected error: %v", err)
	}
	if decisionPayload["status"] != "failed" {
		t.Fatalf("unexpected decision payload: %#v", decisionPayload)
	}
	if !strings.Contains(stdout.String(), `"status": "failed"`) || !strings.Contains(stdout.String(), `"exitCode": 7`) {
		t.Fatalf("expected failed combined JSON output, got %q", stdout.String())
	}
}

func TestTemplatesValidateRunDecidesFailedWhenSandboxFails(t *testing.T) {
	var calls []string
	var decisionPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/templates/template-1/validation-runs":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"template":{"id":"template-1"},"sandbox":{"id":"sandbox-1","status":"pending"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/sandboxes/sandbox-1":
			_, _ = w.Write([]byte(`{"id":"sandbox-1","status":"failed"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/templates/template-1/validation-runs/sandbox-1/decision":
			if err := json.NewDecoder(r.Body).Decode(&decisionPayload); err != nil {
				t.Fatal(err)
			}
			_, _ = w.Write([]byte(`{"template":{"id":"template-1","metadata":{"validationStatus":"failed"}},"sandbox":{"id":"sandbox-1","metadata":{"validationResult":"failed"}}}`))
		case r.URL.Path == "/v1/sandboxes/sandbox-1/tasks":
			t.Fatal("task should not be created when validation sandbox fails")
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"templates", "validate-run", "template-1",
		"--project-id", "project-1",
		"--interval", "1ms",
		"--wait-timeout", "1s",
		"--command", "sh,-lc,echo ok",
	})
	if err == nil {
		t.Fatal("expected failed sandbox wait to return an error")
	}
	if !strings.Contains(err.Error(), "sandbox sandbox-1 reached terminal status failed while waiting for running") {
		t.Fatalf("unexpected error: %v", err)
	}
	if decisionPayload["status"] != "failed" {
		t.Fatalf("unexpected decision payload: %#v", decisionPayload)
	}
	if strings.Contains(strings.Join(calls, "\n"), "/tasks") {
		t.Fatalf("task route should not be called:\n%s", strings.Join(calls, "\n"))
	}
	if !strings.Contains(stdout.String(), `"decisionStatus": "failed"`) || !strings.Contains(stdout.String(), `"decision"`) {
		t.Fatalf("expected failed combined JSON output, got %q", stdout.String())
	}
}

func TestTemplatesCreatePostsExpectedPayload(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"id":"template-1","name":"Node"}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"templates", "create",
		"--project-id", "project-1",
		"--name", "Node",
		"--slug", "node",
		"--image", "node:22",
		"--arg", "sh",
		"--arg", "-lc",
		"--arg", "npm test",
		"--working-dir", "/workspace",
		"--cpu-request", "250m",
		"--memory-request", "512Mi",
		"--storage-request", "2Gi",
		"--env", `{"NODE_ENV":"test"}`,
		"--metadata", `{"runtimeType":"node"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPost || path != "/v1/templates" {
		t.Fatalf("unexpected request %s %s", method, path)
	}
	if payload["projectId"] != "project-1" ||
		payload["name"] != "Node" ||
		payload["slug"] != "node" ||
		payload["image"] != "node:22" ||
		payload["workingDir"] != "/workspace" ||
		payload["cpuRequest"] != "250m" ||
		payload["memoryRequest"] != "512Mi" ||
		payload["storageRequest"] != "2Gi" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
	command, ok := payload["startupCommand"].([]any)
	if !ok || len(command) != 3 || command[0] != "sh" || command[1] != "-lc" || command[2] != "npm test" {
		t.Fatalf("unexpected startupCommand: %#v", payload["startupCommand"])
	}
	env, ok := payload["env"].(map[string]any)
	if !ok || env["NODE_ENV"] != "test" {
		t.Fatalf("unexpected env: %#v", payload["env"])
	}
	metadata, ok := payload["metadata"].(map[string]any)
	if !ok || metadata["runtimeType"] != "node" {
		t.Fatalf("unexpected metadata: %#v", payload["metadata"])
	}
	if !strings.Contains(stdout.String(), `"template-1"`) {
		t.Fatalf("expected JSON response, got %q", stdout.String())
	}
}

func TestTemplatesCreateRejectsMixedCommandFlags(t *testing.T) {
	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", "http://127.0.0.1:1",
		"templates", "create",
		"--name", "Node",
		"--image", "node:22",
		"--arg", "sh",
		"--command-json", `["sh","-lc","npm test"]`,
	})
	if err == nil {
		t.Fatal("expected mixed command flag error")
	}
	if !strings.Contains(err.Error(), "use only one of --arg, --command, or --command-json") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTemplatesDeleteUsesTemplateRoute(t *testing.T) {
	var method string
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"templates", "delete", "template-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodDelete || path != "/v1/templates/template-1" {
		t.Fatalf("unexpected request %s %s", method, path)
	}
	if strings.TrimSpace(stdout.String()) != "deleted" {
		t.Fatalf("expected deleted output, got %q", stdout.String())
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

func TestTemplatesBoundarySummaryPrintsReadableBoundary(t *testing.T) {
	var method string
	var rawQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		rawQuery = r.URL.RawQuery
		if r.URL.Path != "/v1/templates/template-1/boundary" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"kind":"template",
			"projectId":"project-1",
			"projectName":"Demo Project",
			"templateId":"template-1",
			"templateName":"Node Workspace",
			"namespace":"mbox-demo",
			"serviceAccountName":"mbox-sandbox",
			"serviceAccountTokenAutomount":false,
			"image":"node:22-bookworm-slim",
			"workingDir":"/workspace",
			"resourceRequests":{"cpu":"250m","memory":"512Mi"},
			"storageRequest":"2Gi",
			"previewPorts":[{"name":"web","port":3000,"protocol":"TCP"}],
			"envVarCount":2,
			"secretRefs":[{"name":"git-token","key":"token"}],
			"secretProjection":"references-recorded-not-mounted",
			"networkPolicy":"default",
			"networkPolicyProjection":"agent-sandbox-managed-baseline",
			"lifecyclePolicyProjection":"ttl-enforced",
			"policyEnforcement":"enforced",
			"allowedImagePrefixes":["node:"],
			"allowedServiceAccounts":["mbox-sandbox"],
			"allowedSecretRefs":["git-token"],
			"credentialRefs":[{"id":"cred-1","name":"Git Token","slug":"git-token","type":"git","secretRef":"git-secret/token","usage":["clone"]}],
			"credentialProjection":"references-recorded-not-mounted",
			"checks":[
				{"id":"namespace","label":"Namespace","status":"pass","message":"Runtime namespace is resolved.","evidence":["mbox-demo"]},
				{"id":"secret-refs","label":"Secret references","status":"warn","message":"Secret references are visible by name/key only and are not mounted by the current runtime adapter.","evidence":["secretProjection=references-recorded-not-mounted"]}
			]
		}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"templates", "boundary", "template-1",
		"--project-id", "project-1",
		"--summary",
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || rawQuery != "projectId=project-1" {
		t.Fatalf("unexpected request method=%s query=%s", method, rawQuery)
	}
	output := stdout.String()
	for _, expected := range []string{
		"BOUNDARY SUMMARY",
		"Kind\ttemplate",
		"Project\tDemo Project (project-1)",
		"Template\tNode Workspace (template-1)",
		"Namespace\tmbox-demo",
		"ServiceAccount\tmbox-sandbox",
		"Token automount\tfalse",
		"Image\tnode:22-bookworm-slim",
		"Resource requests\tcpu=250m,memory=512Mi",
		"Preview ports\tweb:3000/TCP",
		"Template secret refs\tgit-token/token",
		"Credential refs\tgit-token type=git secret=git-secret/token usage=clone",
		"Launch policy\tenforced",
		"STATUS\tCHECK\tMESSAGE\tEVIDENCE",
		"warn\tSecret references\tSecret references are visible by name/key only and are not mounted by the current runtime adapter.\tsecretProjection=references-recorded-not-mounted",
		"Boundary\tread-only runtime safety view; not secret-value access, full RBAC, billing, capacity, or live utilization",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in template boundary summary output, got %q", expected, output)
		}
	}
	if strings.Contains(output, `"templateId"`) || strings.Contains(output, `"checks"`) {
		t.Fatalf("expected human-readable template boundary summary without raw JSON, got %q", output)
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

func TestSandboxesBoundarySummaryPrintsReadableBoundary(t *testing.T) {
	var method string
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"kind":"sandbox",
			"projectId":"project-1",
			"projectName":"Demo Project",
			"templateId":"template-1",
			"templateName":"Node Workspace",
			"sandboxId":"sandbox-1",
			"sandboxName":"demo-sandbox",
			"sandboxStatus":"running",
			"namespace":"mbox-demo",
			"serviceAccountName":"mbox-sandbox",
			"serviceAccountTokenAutomount":false,
			"runtimeRef":{"adapter":"agent-sandbox","kind":"Sandbox","namespace":"mbox-demo","name":"demo-runtime"},
			"image":"node:22-bookworm-slim",
			"workingDir":"/workspace",
			"policyEnforcement":"disabled",
			"secretProjection":"none",
			"credentialProjection":"none",
			"networkPolicy":"default",
			"networkPolicyProjection":"agent-sandbox-managed-baseline",
			"lifecyclePolicyProjection":"not-configured",
			"checks":[
				{"id":"runtime-ref","label":"Runtime reference","status":"pass","message":"Sandbox has a runtime reference."},
				{"id":"launch-policy","label":"Launch policy","status":"warn","message":"Project launch policy is disabled; sandbox launches use default platform checks only.","evidence":["policyEnforcement=disabled"]}
			]
		}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"sandboxes", "boundary", "sandbox-1",
		"--summary",
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet || path != "/v1/sandboxes/sandbox-1/boundary" {
		t.Fatalf("unexpected request %s %s", method, path)
	}
	output := stdout.String()
	for _, expected := range []string{
		"BOUNDARY SUMMARY",
		"Kind\tsandbox",
		"Sandbox\tdemo-sandbox (sandbox-1)",
		"Sandbox status\trunning",
		"Runtime ref\tadapter=agent-sandbox kind=Sandbox resource=Sandbox mbox-demo/demo-runtime",
		"Launch policy\tdisabled",
		"pass\tRuntime reference\tSandbox has a runtime reference.\t-",
		"warn\tLaunch policy\tProject launch policy is disabled; sandbox launches use default platform checks only.\tpolicyEnforcement=disabled",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in sandbox boundary summary output, got %q", expected, output)
		}
	}
	if strings.Contains(output, `"sandboxId"`) || strings.Contains(output, `"runtimeRef"`) {
		t.Fatalf("expected human-readable sandbox boundary summary without raw JSON, got %q", output)
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

func TestSandboxesWaitPollsUntilExpectedStatus(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/sandboxes/sandbox-1" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		requests++
		w.Header().Set("Content-Type", "application/json")
		if requests == 1 {
			_, _ = w.Write([]byte(`{"id":"sandbox-1","status":"pending"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"sandbox-1","status":"running","runtimeRef":{"name":"claim-1","namespace":"mbox-smoke"}}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"sandboxes", "wait", "sandbox-1",
		"--interval", "1ms",
		"--timeout", "1s",
		"--require-runtime-ref",
	})
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
	if !strings.Contains(stdout.String(), `"status": "running"`) || !strings.Contains(stdout.String(), `"runtimeRef"`) {
		t.Fatalf("expected final sandbox JSON, got %q", stdout.String())
	}
}

func TestSandboxesWaitRequiresRuntimeRefBeforeSuccess(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/sandboxes/sandbox-1" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		requests++
		w.Header().Set("Content-Type", "application/json")
		if requests == 1 {
			_, _ = w.Write([]byte(`{"id":"sandbox-1","status":"running"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"sandbox-1","status":"running","runtimeRef":{"name":"claim-1","namespace":"mbox-smoke"}}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"sandboxes", "wait",
		"--status", "running",
		"--require-runtime-ref",
		"--interval", "1ms",
		"--timeout", "1s",
		"sandbox-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
	if strings.Count(stdout.String(), `"status": "running"`) != 1 {
		t.Fatalf("expected only final sandbox JSON, got %q", stdout.String())
	}
}

func TestSandboxesWaitReturnsErrorForTerminalStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/sandboxes/sandbox-1" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"sandbox-1","status":"failed"}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"sandboxes", "wait", "sandbox-1",
		"--interval", "1ms",
		"--timeout", "1s",
	})
	if err == nil {
		t.Fatal("expected terminal sandbox status to return an error")
	}
	if !strings.Contains(err.Error(), "sandbox sandbox-1 reached terminal status failed while waiting for running") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), `"status": "failed"`) {
		t.Fatalf("expected final sandbox JSON before error, got %q", stdout.String())
	}
}

func TestSandboxesWaitRejectsUnknownStatus(t *testing.T) {
	app := NewApp(Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", "http://127.0.0.1:18080",
		"sandboxes", "wait", "sandbox-1",
		"--status", "ready",
	})
	if err == nil {
		t.Fatal("expected unknown sandbox status to return an error")
	}
	if !strings.Contains(err.Error(), "status must be one of pending, running, stopped, failed, or deleted") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestArtifactsCreatePostsExpectedPayload(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"id":"artifact-1","kind":"report"}`))
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"artifacts", "create", "sandbox-1",
		"--task-id", "task-1",
		"--kind", "report",
		"--name", "Report",
		"--uri", "workspace:///workspace/report.txt",
		"--content-type", "text/plain",
		"--size-bytes", "128",
		"--metadata", `{"source":"cli-test"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPost || path != "/v1/sandboxes/sandbox-1/artifacts" {
		t.Fatalf("unexpected request %s %s", method, path)
	}
	if payload["taskId"] != "task-1" ||
		payload["kind"] != "report" ||
		payload["name"] != "Report" ||
		payload["uri"] != "workspace:///workspace/report.txt" ||
		payload["contentType"] != "text/plain" ||
		payload["sizeBytes"] != float64(128) {
		t.Fatalf("unexpected payload: %#v", payload)
	}
	metadata, ok := payload["metadata"].(map[string]any)
	if !ok || metadata["source"] != "cli-test" {
		t.Fatalf("unexpected metadata: %#v", payload["metadata"])
	}
	if !strings.Contains(stdout.String(), `"artifact-1"`) {
		t.Fatalf("expected JSON response, got %q", stdout.String())
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

func TestContextCheckReportsHealthInfoAndCompatibility(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("unexpected authorization header %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/healthz":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/v1/info":
			_, _ = w.Write([]byte(`{
				"name":"mbox",
				"apiVersion":"v1alpha1",
				"serverVersion":"test",
				"runtimeController":{"enabled":false},
				"runtimeAccess":{"enabled":true,"adapter":"agent-sandbox"},
				"artifactContent":{"retainedContentEnabled":true,"storageProvider":"postgres","maxBytes":8388608},
				"capabilities":["sandboxes","execution-tasks"],
				"compatibility":{"minimumCliApiVersion":"v1alpha1","minimumSdkApiVersion":"v1alpha1"},
				"authenticationRequired":true
			}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	configPath := writeCLIConfig(t, map[string]any{
		"currentContext": "local",
		"contexts": map[string]any{
			"local": map[string]any{
				"apiUrl":   server.URL,
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
	err := app.Run(context.Background(), []string{
		"--config", configPath,
		"context", "check",
		"--require-capability", "execution-tasks",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(paths, ",") != "/healthz,/v1/info" {
		t.Fatalf("unexpected paths: %#v", paths)
	}
	if strings.Contains(stdout.String(), "secret") {
		t.Fatalf("expected token to be redacted, got %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"ok": true`) ||
		!strings.Contains(stdout.String(), `"hasToken": true`) ||
		!strings.Contains(stdout.String(), `"apiVersion": "v1alpha1"`) ||
		!strings.Contains(stdout.String(), `"serverVersion": "test"`) ||
		!strings.Contains(stdout.String(), `"authenticationRequired": true`) ||
		!strings.Contains(stdout.String(), `"runtimeAccess": {`) ||
		!strings.Contains(stdout.String(), `"artifactContent": {`) ||
		!strings.Contains(stdout.String(), `"execution-tasks"`) {
		t.Fatalf("unexpected context check output: %s", stdout.String())
	}
}

func TestContextCheckReturnsJSONAndErrorForMissingCapability(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/healthz":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/v1/info":
			_, _ = w.Write([]byte(`{
				"name":"mbox",
				"apiVersion":"v1alpha1",
				"capabilities":["sandboxes"],
				"compatibility":{"minimumCliApiVersion":"v1alpha1","minimumSdkApiVersion":"v1alpha1"}
			}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"context", "check",
		"--require-capability", "task-events",
	})
	if err == nil {
		t.Fatal("expected missing capability to return an error")
	}
	if !strings.Contains(err.Error(), "missing required capabilities") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), `"ok": false`) ||
		!strings.Contains(stdout.String(), `"missingCapabilities": [`) ||
		!strings.Contains(stdout.String(), `"task-events"`) {
		t.Fatalf("expected diagnostic JSON before error, got %s", stdout.String())
	}
}

func TestContextCheckReturnsJSONAndErrorForUnreachableHealth(t *testing.T) {
	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", "http://127.0.0.1:1",
		"context", "check",
	})
	if err == nil {
		t.Fatal("expected unreachable API to return an error")
	}
	if !strings.Contains(err.Error(), "health check failed") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), `"ok": false`) ||
		!strings.Contains(stdout.String(), `"health": {`) ||
		!strings.Contains(stdout.String(), `"info": {`) {
		t.Fatalf("expected diagnostic JSON before error, got %s", stdout.String())
	}
}

func TestContextCheckReturnsJSONAndErrorForBadHealthStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/healthz":
			_, _ = w.Write([]byte(`{"status":"degraded"}`))
		case "/v1/info":
			_, _ = w.Write([]byte(`{
				"name":"mbox",
				"apiVersion":"v1alpha1",
				"capabilities":["sandboxes"],
				"compatibility":{"minimumCliApiVersion":"v1alpha1","minimumSdkApiVersion":"v1alpha1"}
			}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{"--api-url", server.URL, "context", "check"})
	if err == nil {
		t.Fatal("expected bad health status to return an error")
	}
	if !strings.Contains(err.Error(), "health check failed") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), `"status": "degraded"`) ||
		!strings.Contains(stdout.String(), `"message": "health status is not ok"`) {
		t.Fatalf("expected degraded health diagnostic JSON, got %s", stdout.String())
	}
}

func TestContextCheckSkipsCompatibilityWhenInfoFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/healthz":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/v1/info":
			http.Error(w, `{"error":"info unavailable"}`, http.StatusServiceUnavailable)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	app := NewApp(Streams{Stdout: stdout, Stderr: &bytes.Buffer{}})
	err := app.Run(context.Background(), []string{
		"--api-url", server.URL,
		"context", "check",
		"--require-capability", "execution-tasks",
	})
	if err == nil {
		t.Fatal("expected info failure to return an error")
	}
	if !strings.Contains(err.Error(), "info check failed") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), `"message": "skipped because /v1/info failed"`) ||
		!strings.Contains(stdout.String(), `"requiredCapabilities": [`) ||
		!strings.Contains(stdout.String(), `"execution-tasks"`) {
		t.Fatalf("expected skipped compatibility diagnostic JSON, got %s", stdout.String())
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

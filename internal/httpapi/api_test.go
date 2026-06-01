package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
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

func TestInfoReportsCapabilities(t *testing.T) {
	backend, err := NewFilesystemArtifactContentBackend(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	api := NewWithOptions(newFakeStore(), Options{
		RuntimeAccess:          &fakeRuntimeAccess{},
		ArtifactContentBackend: backend,
		Info: InfoOptions{
			ServerVersion:            "test-version",
			RuntimeControllerEnabled: true,
			RuntimeAdapter:           "agent-sandbox",
		},
	})

	res := request(api, http.MethodGet, "/v1/info", nil)
	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, res.Code, res.Body.String())
	}
	var info APIInfo
	decodeResponse(t, res, &info)
	if info.Name != "mbox" || info.APIVersion != currentAPIVersion || info.ServerVersion != "test-version" {
		t.Fatalf("unexpected info identity: %+v", info)
	}
	if !info.RuntimeController.Enabled || !info.RuntimeAccess.Enabled ||
		info.RuntimeController.Adapter != "agent-sandbox" ||
		info.RuntimeAccess.Adapter != "agent-sandbox" {
		t.Fatalf("unexpected runtime capabilities: %+v", info)
	}
	if !info.ArtifactContent.RetainedContentEnabled ||
		info.ArtifactContent.StorageProvider != string(domain.ArtifactContentStorageProviderFilesystem) ||
		info.ArtifactContent.MaxBytes != maxArtifactContentBytes {
		t.Fatalf("unexpected artifact content capability: %+v", info.ArtifactContent)
	}
	if info.AuthenticationRequired {
		t.Fatal("expected authenticationRequired to be false by default")
	}
	if !stringSliceContains(info.Capabilities, "execution-tasks") ||
		!stringSliceContains(info.Capabilities, "artifact-client-upload") ||
		!stringSliceContains(info.Capabilities, "openapi") ||
		!stringSliceContains(info.Capabilities, "project-usage") ||
		!stringSliceContains(info.Capabilities, "project-delete-cleanup-guard") ||
		!stringSliceContains(info.Capabilities, "runtime-orphan-audit") ||
		!stringSliceContains(info.Capabilities, "runtime-orphan-cleanup") {
		t.Fatalf("missing expected capabilities: %+v", info.Capabilities)
	}
}

func TestOptionalAPITokenProtectsPrivateRoutes(t *testing.T) {
	api := NewWithOptions(newFakeStore(), Options{APIToken: "secret"})

	infoRes := request(api, http.MethodGet, "/v1/info", nil)
	if infoRes.Code != http.StatusOK {
		t.Fatalf("expected info to stay public, got %d: %s", infoRes.Code, infoRes.Body.String())
	}
	var info APIInfo
	decodeResponse(t, infoRes, &info)
	if !info.AuthenticationRequired {
		t.Fatal("expected authenticationRequired when API token is configured")
	}

	healthRes := request(api, http.MethodGet, "/healthz", nil)
	if healthRes.Code != http.StatusOK {
		t.Fatalf("expected healthz to stay public, got %d: %s", healthRes.Code, healthRes.Body.String())
	}

	unauthorized := request(api, http.MethodGet, "/v1/projects", nil)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized status, got %d: %s", unauthorized.Code, unauthorized.Body.String())
	}

	wrong := requestWithHeaders(api, http.MethodGet, "/v1/projects", nil, map[string]string{
		"Authorization": "Bearer wrong",
	})
	if wrong.Code != http.StatusUnauthorized {
		t.Fatalf("expected wrong token to be unauthorized, got %d: %s", wrong.Code, wrong.Body.String())
	}

	authorized := requestWithHeaders(api, http.MethodGet, "/v1/projects", nil, map[string]string{
		"Authorization": "Bearer secret",
	})
	if authorized.Code != http.StatusOK {
		t.Fatalf("expected authorized request to pass, got %d: %s", authorized.Code, authorized.Body.String())
	}
}

func TestOpenAPIRoutePublishesCurrentContract(t *testing.T) {
	api := New(newFakeStore())

	res := request(api, http.MethodGet, "/v1/openapi.json", nil)
	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, res.Code, res.Body.String())
	}
	var doc map[string]any
	decodeResponse(t, res, &doc)
	if doc["openapi"] != "3.1.0" {
		t.Fatalf("unexpected openapi version: %#v", doc["openapi"])
	}
	info, ok := doc["info"].(map[string]any)
	if !ok || info["version"] != currentAPIVersion || info["title"] != "mbox API" {
		t.Fatalf("unexpected info object: %#v", doc["info"])
	}
	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		t.Fatalf("expected paths object, got %#v", doc["paths"])
	}
	for _, path := range []string{
		"/v1/info",
		"/v1/openapi.json",
		"/v1/projects/{projectID}/quota-policy",
		"/v1/sandboxes/{sandboxID}/tasks",
		"/v1/tasks/{taskID}/events",
		"/v1/artifacts/{artifactID}/content",
	} {
		if _, ok := paths[path]; !ok {
			t.Fatalf("expected OpenAPI path %s in %#v", path, paths)
		}
	}
	components, ok := doc["components"].(map[string]any)
	if !ok {
		t.Fatalf("expected components object, got %#v", doc["components"])
	}
	securitySchemes, ok := components["securitySchemes"].(map[string]any)
	if !ok {
		t.Fatalf("expected securitySchemes object, got %#v", components["securitySchemes"])
	}
	bearerAuth, ok := securitySchemes["bearerAuth"].(map[string]any)
	if !ok || bearerAuth["type"] != "http" || bearerAuth["scheme"] != "bearer" {
		t.Fatalf("expected bearerAuth security scheme, got %#v", securitySchemes["bearerAuth"])
	}
	infoPath, ok := paths["/v1/info"].(map[string]any)
	if !ok {
		t.Fatalf("expected info path, got %#v", paths["/v1/info"])
	}
	infoGet, ok := infoPath["get"].(map[string]any)
	if !ok {
		t.Fatalf("expected info get operation, got %#v", infoPath["get"])
	}
	if !operationHasPublicSecurity(infoGet) {
		t.Fatalf("expected info route to publish public security, got %#v", infoGet["security"])
	}
	projectPath, ok := paths["/v1/projects"].(map[string]any)
	if !ok {
		t.Fatalf("expected projects path, got %#v", paths["/v1/projects"])
	}
	projectGet, ok := projectPath["get"].(map[string]any)
	if !ok {
		t.Fatalf("expected projects get operation, got %#v", projectPath["get"])
	}
	if !operationRequiresBearerSecurity(projectGet) {
		t.Fatalf("expected projects route to require bearer security, got %#v", projectGet["security"])
	}
	projectResponses, ok := projectGet["responses"].(map[string]any)
	if !ok || projectResponses["401"] == nil {
		t.Fatalf("expected projects route to publish 401 response, got %#v", projectGet["responses"])
	}
	schemas, ok := components["schemas"].(map[string]any)
	if !ok {
		t.Fatalf("expected schemas object, got %#v", components["schemas"])
	}
	for _, name := range []string{"Project", "ProjectQuotaPolicy", "ProjectUsage", "ProjectSandboxUsage", "SandboxResourceRequestUsage", "ResourceQuantityUsage", "ExecutionTask", "Artifact", "Error"} {
		if _, ok := schemas[name]; !ok {
			t.Fatalf("expected schema %s in OpenAPI components", name)
		}
	}
	projectUsage, ok := schemas["ProjectUsage"].(map[string]any)
	if !ok {
		t.Fatalf("expected ProjectUsage schema in %#v", schemas["ProjectUsage"])
	}
	usageProperties, ok := projectUsage["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected ProjectUsage properties, got %#v", projectUsage["properties"])
	}
	if usageProperties["sandboxes"] == nil || usageProperties["templates"] == nil || usageProperties["artifacts"] == nil {
		t.Fatalf("expected ProjectUsage resource properties, got %#v", usageProperties)
	}
	sandboxUsage, ok := schemas["ProjectSandboxUsage"].(map[string]any)
	if !ok {
		t.Fatalf("expected ProjectSandboxUsage schema in %#v", schemas["ProjectSandboxUsage"])
	}
	sandboxProperties, ok := sandboxUsage["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected ProjectSandboxUsage properties, got %#v", sandboxUsage["properties"])
	}
	if sandboxProperties["activeRequests"] == nil || sandboxProperties["runningRequests"] == nil {
		t.Fatalf("expected sandbox request usage properties, got %#v", sandboxProperties)
	}
	quantityUsage, ok := schemas["ResourceQuantityUsage"].(map[string]any)
	if !ok {
		t.Fatalf("expected ResourceQuantityUsage schema in %#v", schemas["ResourceQuantityUsage"])
	}
	quantityRequired, ok := quantityUsage["required"].([]any)
	if !ok ||
		!anySliceContainsString(quantityRequired, "declared") ||
		!anySliceContainsString(quantityRequired, "missing") ||
		!anySliceContainsString(quantityRequired, "invalid") {
		t.Fatalf("expected quantity usage required fields, got %#v", quantityUsage["required"])
	}
	runtimeResourceList, ok := schemas["RuntimeResourceList"].(map[string]any)
	if !ok {
		t.Fatalf("expected RuntimeResourceList schema in %#v", schemas["RuntimeResourceList"])
	}
	runtimeResourceRequired, ok := runtimeResourceList["required"].([]any)
	if !ok ||
		!anySliceContainsString(runtimeResourceRequired, "adapter") ||
		!anySliceContainsString(runtimeResourceRequired, "checkedAt") ||
		!anySliceContainsString(runtimeResourceRequired, "summary") ||
		!anySliceContainsString(runtimeResourceRequired, "items") {
		t.Fatalf("expected runtime resource list required fields, got %#v", runtimeResourceList["required"])
	}
	runtimeResourceSummary, ok := schemas["RuntimeResourceSummary"].(map[string]any)
	if !ok {
		t.Fatalf("expected RuntimeResourceSummary schema in %#v", schemas["RuntimeResourceSummary"])
	}
	runtimeResourceSummaryRequired, ok := runtimeResourceSummary["required"].([]any)
	if !ok ||
		!anySliceContainsString(runtimeResourceSummaryRequired, "total") ||
		!anySliceContainsString(runtimeResourceSummaryRequired, "byKind") ||
		!anySliceContainsString(runtimeResourceSummaryRequired, "byNamespace") ||
		!anySliceContainsString(runtimeResourceSummaryRequired, "byOwner") ||
		!anySliceContainsString(runtimeResourceSummaryRequired, "workload") {
		t.Fatalf("expected runtime resource summary required fields, got %#v", runtimeResourceSummary["required"])
	}
	runtimeWorkloadSummary, ok := schemas["RuntimeWorkloadSummary"].(map[string]any)
	if !ok {
		t.Fatalf("expected RuntimeWorkloadSummary schema in %#v", schemas["RuntimeWorkloadSummary"])
	}
	runtimeWorkloadRequired, ok := runtimeWorkloadSummary["required"].([]any)
	if !ok ||
		!anySliceContainsString(runtimeWorkloadRequired, "observedPods") ||
		!anySliceContainsString(runtimeWorkloadRequired, "runningPods") ||
		!anySliceContainsString(runtimeWorkloadRequired, "containersReady") {
		t.Fatalf("expected runtime workload required fields, got %#v", runtimeWorkloadSummary["required"])
	}
	runtimeWorkloadProperties, ok := runtimeWorkloadSummary["properties"].(map[string]any)
	if !ok ||
		runtimeWorkloadProperties["requests"] == nil ||
		runtimeWorkloadProperties["limits"] == nil ||
		runtimeWorkloadProperties["storage"] == nil ||
		runtimeWorkloadProperties["quantityIssues"] == nil {
		t.Fatalf("expected runtime workload summary properties, got %#v", runtimeWorkloadSummary["properties"])
	}
	if _, ok := schemas["RuntimeQuantityIssue"].(map[string]any); !ok {
		t.Fatalf("expected RuntimeQuantityIssue schema in %#v", schemas["RuntimeQuantityIssue"])
	}
	if _, ok := schemas["RuntimeStorageSummary"].(map[string]any); !ok {
		t.Fatalf("expected RuntimeStorageSummary schema in %#v", schemas["RuntimeStorageSummary"])
	}
	runtimeResource, ok := schemas["RuntimeResource"].(map[string]any)
	if !ok {
		t.Fatalf("expected RuntimeResource schema in %#v", schemas["RuntimeResource"])
	}
	runtimeResourceProperties, ok := runtimeResource["properties"].(map[string]any)
	if !ok || runtimeResourceProperties["owner"] == nil || runtimeResourceProperties["observation"] == nil {
		t.Fatalf("expected runtime resource owner and observation properties, got %#v", runtimeResource["properties"])
	}
	runtimeResourceOwner, ok := schemas["RuntimeResourceOwner"].(map[string]any)
	if !ok {
		t.Fatalf("expected RuntimeResourceOwner schema in %#v", schemas["RuntimeResourceOwner"])
	}
	runtimeResourceOwnerRequired, ok := runtimeResourceOwner["required"].([]any)
	if !ok || !anySliceContainsString(runtimeResourceOwnerRequired, "kind") {
		t.Fatalf("expected runtime resource owner required fields, got %#v", runtimeResourceOwner["required"])
	}
	runtimeResourceObservation, ok := schemas["RuntimeResourceObservation"].(map[string]any)
	if !ok {
		t.Fatalf("expected RuntimeResourceObservation schema in %#v", schemas["RuntimeResourceObservation"])
	}
	runtimeResourceObservationProperties, ok := runtimeResourceObservation["properties"].(map[string]any)
	if !ok ||
		runtimeResourceObservationProperties["podPhase"] == nil ||
		runtimeResourceObservationProperties["requests"] == nil ||
		runtimeResourceObservationProperties["storage"] == nil {
		t.Fatalf("expected runtime observation properties, got %#v", runtimeResourceObservation["properties"])
	}
	runtimeStorage, ok := schemas["RuntimeStorage"].(map[string]any)
	if !ok {
		t.Fatalf("expected RuntimeStorage schema in %#v", schemas["RuntimeStorage"])
	}
	runtimeStorageRequired, ok := runtimeStorage["required"].([]any)
	if !ok ||
		!anySliceContainsString(runtimeStorageRequired, "name") ||
		!anySliceContainsString(runtimeStorageRequired, "mountPath") {
		t.Fatalf("expected runtime storage required fields, got %#v", runtimeStorage["required"])
	}
	runtimeOrphanAudit, ok := schemas["RuntimeOrphanAudit"].(map[string]any)
	if !ok {
		t.Fatalf("expected RuntimeOrphanAudit schema in %#v", schemas["RuntimeOrphanAudit"])
	}
	runtimeOrphanAuditRequired, ok := runtimeOrphanAudit["required"].([]any)
	if !ok ||
		!anySliceContainsString(runtimeOrphanAuditRequired, "adapter") ||
		!anySliceContainsString(runtimeOrphanAuditRequired, "checkedAt") ||
		!anySliceContainsString(runtimeOrphanAuditRequired, "resourceCount") ||
		!anySliceContainsString(runtimeOrphanAuditRequired, "orphanCount") ||
		!anySliceContainsString(runtimeOrphanAuditRequired, "expectedClean") ||
		!anySliceContainsString(runtimeOrphanAuditRequired, "items") {
		t.Fatalf("expected runtime orphan audit required fields, got %#v", runtimeOrphanAudit["required"])
	}
	runtimeOrphan, ok := schemas["RuntimeOrphan"].(map[string]any)
	if !ok {
		t.Fatalf("expected RuntimeOrphan schema in %#v", schemas["RuntimeOrphan"])
	}
	runtimeOrphanRequired, ok := runtimeOrphan["required"].([]any)
	if !ok ||
		!anySliceContainsString(runtimeOrphanRequired, "reason") ||
		!anySliceContainsString(runtimeOrphanRequired, "resource") ||
		!anySliceContainsString(runtimeOrphanRequired, "message") {
		t.Fatalf("expected runtime orphan required fields, got %#v", runtimeOrphan["required"])
	}
	managedResourceRef, ok := schemas["ManagedResourceRef"].(map[string]any)
	if !ok {
		t.Fatalf("expected ManagedResourceRef schema in %#v", schemas["ManagedResourceRef"])
	}
	managedResourceRefRequired, ok := managedResourceRef["required"].([]any)
	if !ok ||
		!anySliceContainsString(managedResourceRefRequired, "adapter") ||
		!anySliceContainsString(managedResourceRefRequired, "kind") ||
		!anySliceContainsString(managedResourceRefRequired, "namespace") ||
		!anySliceContainsString(managedResourceRefRequired, "name") {
		t.Fatalf("expected managed resource ref required fields, got %#v", managedResourceRef["required"])
	}
	actionSchema, ok := schemas["AuditEventAction"].(map[string]any)
	if !ok {
		t.Fatalf("expected AuditEventAction schema in %#v", schemas["AuditEventAction"])
	}
	actionValues, ok := actionSchema["enum"].([]any)
	if !ok || !anySliceContainsString(actionValues, "policy.denied") || !anySliceContainsString(actionValues, "sandbox.created") {
		t.Fatalf("expected audit action enum to include known actions, got %#v", actionSchema["enum"])
	}
	auditPath, ok := paths["/v1/audit-events"].(map[string]any)
	if !ok {
		t.Fatalf("expected audit events path, got %#v", paths["/v1/audit-events"])
	}
	auditGet, ok := auditPath["get"].(map[string]any)
	if !ok {
		t.Fatalf("expected audit events get operation, got %#v", auditPath["get"])
	}
	auditParams, ok := auditGet["parameters"].([]any)
	if !ok || !parametersContainName(auditParams, "action") {
		t.Fatalf("expected audit action query parameter, got %#v", auditGet["parameters"])
	}
	if !parametersContainName(auditParams, "operation") {
		t.Fatalf("expected audit operation query parameter, got %#v", auditGet["parameters"])
	}
	if !parametersContainName(auditParams, "since") || !parametersContainName(auditParams, "until") {
		t.Fatalf("expected audit time-window query parameters, got %#v", auditGet["parameters"])
	}
	if !parametersContainName(auditParams, "X-Mbox-Request-ID") ||
		!parametersContainName(auditParams, "X-Mbox-Audit-Actor") ||
		!parametersContainName(auditParams, "X-Mbox-Audit-Source") {
		t.Fatalf("expected common request/audit headers, got %#v", auditGet["parameters"])
	}
	deniedSchema, ok := schemas["PolicyDeniedAuditMetadata"].(map[string]any)
	if !ok {
		t.Fatalf("expected PolicyDeniedAuditMetadata schema in %#v", schemas["PolicyDeniedAuditMetadata"])
	}
	required, ok := deniedSchema["required"].([]any)
	if !ok || !anySliceContainsString(required, "operation") || !anySliceContainsString(required, "reason") {
		t.Fatalf("expected policy denied metadata requirements, got %#v", deniedSchema["required"])
	}
	properties, ok := deniedSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected policy denied metadata properties, got %#v", deniedSchema["properties"])
	}
	if _, ok := properties["requestId"].(map[string]any); !ok {
		t.Fatalf("expected requestId policy denied metadata property, got %#v", properties["requestId"])
	}
	operation, ok := properties["operation"].(map[string]any)
	if !ok {
		t.Fatalf("expected operation property, got %#v", properties["operation"])
	}
	operations, ok := operation["enum"].([]any)
	if !ok ||
		!anySliceContainsString(operations, "sandbox.launch") ||
		!anySliceContainsString(operations, "template.validation") ||
		!anySliceContainsString(operations, "artifact.content.capture") ||
		!anySliceContainsString(operations, "artifact.content.upload") {
		t.Fatalf("expected policy denied operation enum, got %#v", operation["enum"])
	}
}

func TestRuntimeOrphansRequiresAuditor(t *testing.T) {
	api := New(newFakeStore())

	res := request(api, http.MethodGet, "/v1/runtime/resources", nil)
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected resources status %d, got %d: %s", http.StatusServiceUnavailable, res.Code, res.Body.String())
	}

	res = request(api, http.MethodGet, "/v1/runtime/orphans", nil)
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d: %s", http.StatusServiceUnavailable, res.Code, res.Body.String())
	}

	res = request(api, http.MethodPost, "/v1/runtime/orphans/cleanup", map[string]any{})
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected cleanup status %d, got %d: %s", http.StatusServiceUnavailable, res.Code, res.Body.String())
	}
}

func TestRuntimeResourcesListsManagedResources(t *testing.T) {
	sandboxID := uuid.NewString()
	projectID := uuid.NewString()
	templateID := uuid.NewString()
	api := NewWithOptions(newFakeStore(), Options{
		RuntimeAuditor: &fakeRuntimeAuditor{resources: []mboxruntime.ManagedResource{
			{
				Adapter:   "agent-sandbox",
				Kind:      "SandboxClaim",
				Namespace: "mbox-demo",
				Name:      "claim",
				Owner: &mboxruntime.ManagedResourceOwner{
					Kind:      "sandbox",
					ProjectID: projectID,
					SandboxID: sandboxID,
				},
				Observation: &mboxruntime.ManagedResourceObservation{
					RuntimeName:     "resolved-sandbox",
					Selector:        "agents.x-k8s.io/sandbox=resolved-sandbox",
					PodName:         "runtime-pod",
					PodPhase:        "Running",
					PodCount:        1,
					RunningPodCount: 1,
					ContainersReady: 1,
					ContainersTotal: 1,
					RestartCount:    2,
					Requests:        map[string]string{"cpu": "250m", "memory": "512Mi"},
					Limits:          map[string]string{"memory": "1Gi"},
					Storage: []mboxruntime.RuntimeStorage{{
						Name:      "workspace",
						MountPath: "/workspace",
						ClaimName: "workspace-resolved-sandbox",
						Phase:     "Bound",
						Capacity:  "10Gi",
					}},
				},
				Labels: map[string]string{
					"mbox.dev/project-id": projectID,
					"mbox.dev/sandbox-id": sandboxID,
				},
			},
			{
				Adapter:   "agent-sandbox",
				Kind:      "SandboxTemplate",
				Namespace: "other",
				Name:      "template",
				Owner: &mboxruntime.ManagedResourceOwner{
					Kind:       "template",
					TemplateID: templateID,
				},
				Labels: map[string]string{"mbox.dev/template-id": templateID},
			},
		}},
	})

	res := request(api, http.MethodGet, "/v1/runtime/resources", nil)
	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, res.Code, res.Body.String())
	}
	var list mboxruntime.ManagedResourceList
	decodeResponse(t, res, &list)
	if list.Adapter != "agent-sandbox" || len(list.Items) != 2 {
		t.Fatalf("unexpected runtime resources list: %+v", list)
	}
	if list.Summary.Total != 2 ||
		!managedResourceCountsContain(list.Summary.ByKind, "SandboxClaim", 1) ||
		!managedResourceCountsContain(list.Summary.ByKind, "SandboxTemplate", 1) ||
		!managedResourceCountsContain(list.Summary.ByNamespace, "mbox-demo", 1) ||
		!managedResourceCountsContain(list.Summary.ByNamespace, "other", 1) ||
		!managedResourceCountsContain(list.Summary.ByOwner, "project/"+projectID+"/sandbox/"+sandboxID, 1) ||
		!managedResourceCountsContain(list.Summary.ByOwner, "template/"+templateID, 1) {
		t.Fatalf("unexpected runtime resource summary: %+v", list.Summary)
	}
	if list.Summary.Workload.ObservedResources != 1 ||
		list.Summary.Workload.ObservedPods != 1 ||
		list.Summary.Workload.RunningPods != 1 ||
		list.Summary.Workload.ContainersReady != 1 ||
		list.Summary.Workload.ContainersTotal != 1 ||
		list.Summary.Workload.RestartCount != 2 ||
		list.Summary.Workload.Requests["cpu"] != "250m" ||
		list.Summary.Workload.Requests["memory"] != "512Mi" ||
		list.Summary.Workload.Limits["memory"] != "1Gi" ||
		list.Summary.Workload.StorageCapacity != "10Gi" ||
		len(list.Summary.Workload.Storage) != 1 ||
		list.Summary.Workload.Storage[0].Phase != "Bound" ||
		list.Summary.Workload.Storage[0].Count != 1 ||
		list.Summary.Workload.Storage[0].Capacity != "10Gi" {
		t.Fatalf("unexpected runtime workload summary: %+v", list.Summary.Workload)
	}
	if list.Items[0].Owner == nil || list.Items[0].Owner.Kind != "sandbox" || list.Items[0].Owner.ProjectID != projectID || list.Items[0].Owner.SandboxID != sandboxID {
		t.Fatalf("expected sandbox owner projection, got %+v", list.Items[0].Owner)
	}
	if list.Items[0].Observation == nil ||
		list.Items[0].Observation.PodPhase != "Running" ||
		list.Items[0].Observation.Requests["cpu"] != "250m" ||
		len(list.Items[0].Observation.Storage) != 1 ||
		list.Items[0].Observation.Storage[0].Capacity != "10Gi" {
		t.Fatalf("expected runtime observation projection, got %+v", list.Items[0].Observation)
	}

	res = request(api, http.MethodGet, "/v1/runtime/resources?namespace=mbox-demo", nil)
	if res.Code != http.StatusOK {
		t.Fatalf("expected filtered status %d, got %d: %s", http.StatusOK, res.Code, res.Body.String())
	}
	list = mboxruntime.ManagedResourceList{}
	decodeResponse(t, res, &list)
	if len(list.Items) != 1 || list.Items[0].Namespace != "mbox-demo" || list.Items[0].Kind != "SandboxClaim" {
		t.Fatalf("unexpected namespace-filtered runtime resources: %+v", list)
	}
	if list.Summary.Total != 1 ||
		!managedResourceCountsContain(list.Summary.ByKind, "SandboxClaim", 1) ||
		!managedResourceCountsContain(list.Summary.ByNamespace, "mbox-demo", 1) ||
		!managedResourceCountsContain(list.Summary.ByOwner, "project/"+projectID+"/sandbox/"+sandboxID, 1) {
		t.Fatalf("unexpected namespace-filtered runtime resource summary: %+v", list.Summary)
	}
	if list.Summary.Workload.ObservedResources != 1 || list.Summary.Workload.Requests["cpu"] != "250m" {
		t.Fatalf("unexpected namespace-filtered runtime workload summary: %+v", list.Summary.Workload)
	}

	res = request(api, http.MethodGet, "/v1/runtime/resources?kind=SandboxTemplate", nil)
	if res.Code != http.StatusOK {
		t.Fatalf("expected kind-filtered status %d, got %d: %s", http.StatusOK, res.Code, res.Body.String())
	}
	list = mboxruntime.ManagedResourceList{}
	decodeResponse(t, res, &list)
	if len(list.Items) != 1 || list.Items[0].Kind != "SandboxTemplate" || list.Items[0].Namespace != "other" {
		t.Fatalf("unexpected kind-filtered runtime resources: %+v", list)
	}
	if list.Summary.Total != 1 ||
		!managedResourceCountsContain(list.Summary.ByKind, "SandboxTemplate", 1) ||
		!managedResourceCountsContain(list.Summary.ByNamespace, "other", 1) ||
		!managedResourceCountsContain(list.Summary.ByOwner, "template/"+templateID, 1) {
		t.Fatalf("unexpected kind-filtered runtime resource summary: %+v", list.Summary)
	}
	if list.Items[0].Owner == nil || list.Items[0].Owner.Kind != "template" || list.Items[0].Owner.TemplateID != templateID {
		t.Fatalf("expected template owner projection, got %+v", list.Items[0].Owner)
	}
	if list.Summary.Workload.ObservedResources != 0 || len(list.Summary.Workload.Requests) != 0 {
		t.Fatalf("unexpected template-only runtime workload summary: %+v", list.Summary.Workload)
	}
}

func TestRuntimeOrphansClassifiesManagedResources(t *testing.T) {
	store := newFakeStore()
	project := store.mustProject(t)
	template := store.mustTemplate(t, &project.ID)
	running, err := store.CreateSandbox(context.Background(), domain.SandboxCreate{
		ProjectID:          project.ID,
		TemplateID:         template.ID,
		Name:               "Running",
		Slug:               "running",
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
		Name:      "running",
	}
	running, err = store.UpdateSandbox(context.Background(), running.ID, domain.SandboxUpdate{
		Status:     ptr(domain.SandboxStatusRunning),
		RuntimeRef: &runtimeRef,
	})
	if err != nil {
		t.Fatal(err)
	}

	cleanupPending, err := store.CreateSandbox(context.Background(), domain.SandboxCreate{
		ProjectID:          project.ID,
		TemplateID:         template.ID,
		Name:               "Deleted",
		Slug:               "deleted",
		Namespace:          "mbox-demo",
		ServiceAccountName: "mbox-sandbox",
	})
	if err != nil {
		t.Fatal(err)
	}
	cleanupRef := &domain.RuntimeRef{
		Adapter:   "agent-sandbox",
		Kind:      "SandboxClaim",
		Namespace: "mbox-demo",
		Name:      "deleted",
	}
	if _, err := store.UpdateSandbox(context.Background(), cleanupPending.ID, domain.SandboxUpdate{RuntimeRef: &cleanupRef}); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteSandbox(context.Background(), cleanupPending.ID); err != nil {
		t.Fatal(err)
	}

	mismatch, err := store.CreateSandbox(context.Background(), domain.SandboxCreate{
		ProjectID:          project.ID,
		TemplateID:         template.ID,
		Name:               "Mismatch",
		Slug:               "mismatch",
		Namespace:          "mbox-demo",
		ServiceAccountName: "mbox-sandbox",
	})
	if err != nil {
		t.Fatal(err)
	}
	wrongRef := &domain.RuntimeRef{
		Adapter:   "agent-sandbox",
		Kind:      "SandboxClaim",
		Namespace: "mbox-demo",
		Name:      "other",
	}
	if _, err := store.UpdateSandbox(context.Background(), mismatch.ID, domain.SandboxUpdate{RuntimeRef: &wrongRef}); err != nil {
		t.Fatal(err)
	}

	missingSandboxID := uuid.New()
	missingTemplateID := uuid.New()
	api := NewWithOptions(store, Options{
		RuntimeAuditor: &fakeRuntimeAuditor{resources: []mboxruntime.ManagedResource{
			{
				Adapter:   "agent-sandbox",
				Kind:      "SandboxClaim",
				Namespace: "mbox-demo",
				Name:      "running",
				Labels:    map[string]string{"mbox.dev/sandbox-id": running.ID.String()},
			},
			{
				Adapter:   "agent-sandbox",
				Kind:      "SandboxClaim",
				Namespace: "mbox-demo",
				Name:      "deleted",
				Labels:    map[string]string{"mbox.dev/sandbox-id": cleanupPending.ID.String()},
			},
			{
				Adapter:   "agent-sandbox",
				Kind:      "SandboxClaim",
				Namespace: "mbox-demo",
				Name:      "missing",
				Labels:    map[string]string{"mbox.dev/sandbox-id": missingSandboxID.String()},
			},
			{
				Adapter:   "agent-sandbox",
				Kind:      "SandboxClaim",
				Namespace: "mbox-demo",
				Name:      "mismatch",
				Labels:    map[string]string{"mbox.dev/sandbox-id": mismatch.ID.String()},
			},
			{
				Adapter:   "agent-sandbox",
				Kind:      "SandboxTemplate",
				Namespace: "mbox-demo",
				Name:      "template-ok",
				Labels:    map[string]string{"mbox.dev/template-id": template.ID.String()},
			},
			{
				Adapter:   "agent-sandbox",
				Kind:      "SandboxTemplate",
				Namespace: "mbox-demo",
				Name:      "template-missing",
				Labels:    map[string]string{"mbox.dev/template-id": missingTemplateID.String()},
			},
			{
				Adapter:   "agent-sandbox",
				Kind:      "SandboxClaim",
				Namespace: "mbox-demo",
				Name:      "unlabeled",
				Labels:    map[string]string{"app.kubernetes.io/managed-by": "mbox"},
			},
		}},
	})

	res := request(api, http.MethodGet, "/v1/runtime/orphans", nil)
	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, res.Code, res.Body.String())
	}
	var audit RuntimeOrphanAudit
	decodeResponse(t, res, &audit)
	if audit.ResourceCount != 7 || audit.OrphanCount != 5 || audit.ExpectedClean {
		t.Fatalf("unexpected audit summary: %+v", audit)
	}
	reasons := map[RuntimeOrphanReason]int{}
	for _, item := range audit.Items {
		reasons[item.Reason]++
	}
	if reasons[RuntimeOrphanCleanupPending] != 1 ||
		reasons[RuntimeOrphanMissingSandboxRecord] != 1 ||
		reasons[RuntimeOrphanRuntimeRefMismatch] != 1 ||
		reasons[RuntimeOrphanMissingTemplateRecord] != 1 ||
		reasons[RuntimeOrphanUnlabeledOwner] != 1 {
		t.Fatalf("unexpected orphan reasons: %+v", reasons)
	}

	res = request(api, http.MethodGet, "/v1/runtime/orphans?namespace=other", nil)
	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, res.Code, res.Body.String())
	}
	decodeResponse(t, res, &audit)
	if audit.Namespace != "other" || audit.ResourceCount != 0 || audit.OrphanCount != 0 || !audit.ExpectedClean {
		t.Fatalf("unexpected namespace-filtered audit: %+v", audit)
	}

	res = request(api, http.MethodGet, "/v1/runtime/orphans?kind=SandboxTemplate", nil)
	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, res.Code, res.Body.String())
	}
	decodeResponse(t, res, &audit)
	if audit.ResourceCount != 2 || audit.OrphanCount != 1 || audit.ExpectedClean {
		t.Fatalf("unexpected kind-filtered audit: %+v", audit)
	}
	if len(audit.Items) != 1 || audit.Items[0].Reason != RuntimeOrphanMissingTemplateRecord {
		t.Fatalf("unexpected kind-filtered audit items: %+v", audit.Items)
	}
}

func TestCleanupRuntimeOrphanRequiresExplicitConfirmation(t *testing.T) {
	store := newFakeStore()
	api := NewWithOptions(store, Options{
		RuntimeAuditor: &fakeRuntimeAuditor{},
		RuntimeCleaner: &fakeRuntimeCleaner{},
	})

	res := request(api, http.MethodPost, "/v1/runtime/orphans/cleanup", map[string]any{
		"resource": map[string]any{
			"adapter":   "agent-sandbox",
			"kind":      "SandboxClaim",
			"namespace": "mbox-demo",
			"name":      "missing",
		},
		"reason":       "missing-sandbox-record",
		"deleteOrphan": true,
	})
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, res.Code, res.Body.String())
	}
}

func TestCleanupRuntimeOrphanDeletesOnlyCurrentMatchingOrphan(t *testing.T) {
	store := newFakeStore()
	missingSandboxID := uuid.New()
	cleaner := &fakeRuntimeCleaner{}
	api := NewWithOptions(store, Options{
		RuntimeAuditor: &fakeRuntimeAuditor{resources: []mboxruntime.ManagedResource{{
			Adapter:   "agent-sandbox",
			Kind:      "SandboxClaim",
			Namespace: "mbox-demo",
			Name:      "missing",
			Labels:    map[string]string{"mbox.dev/sandbox-id": missingSandboxID.String()},
		}}},
		RuntimeCleaner: cleaner,
	})

	res := request(api, http.MethodPost, "/v1/runtime/orphans/cleanup", map[string]any{
		"resource": map[string]any{
			"adapter":   "agent-sandbox",
			"kind":      "SandboxClaim",
			"namespace": "mbox-demo",
			"name":      "missing",
		},
		"reason":       "missing-sandbox-record",
		"deleteOrphan": true,
		"confirm":      "delete-orphan-runtime-resource",
	})
	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, res.Code, res.Body.String())
	}
	var result RuntimeOrphanCleanupResult
	decodeResponse(t, res, &result)
	if !result.Deleted || result.Reason != RuntimeOrphanMissingSandboxRecord {
		t.Fatalf("unexpected cleanup result: %+v", result)
	}
	if cleaner.deleted == nil || cleaner.deleted.Name != "missing" || cleaner.deleted.Kind != "SandboxClaim" {
		t.Fatalf("expected cleaner to delete missing claim, got %+v", cleaner.deleted)
	}
}

func TestCleanupRuntimeOrphanRejectsReasonDrift(t *testing.T) {
	store := newFakeStore()
	missingTemplateID := uuid.New()
	cleaner := &fakeRuntimeCleaner{}
	api := NewWithOptions(store, Options{
		RuntimeAuditor: &fakeRuntimeAuditor{resources: []mboxruntime.ManagedResource{{
			Adapter:   "agent-sandbox",
			Kind:      "SandboxTemplate",
			Namespace: "mbox-demo",
			Name:      "template-missing",
			Labels:    map[string]string{"mbox.dev/template-id": missingTemplateID.String()},
		}}},
		RuntimeCleaner: cleaner,
	})

	res := request(api, http.MethodPost, "/v1/runtime/orphans/cleanup", map[string]any{
		"resource": map[string]any{
			"adapter":   "agent-sandbox",
			"kind":      "SandboxTemplate",
			"namespace": "mbox-demo",
			"name":      "template-missing",
		},
		"reason":       "missing-sandbox-record",
		"deleteOrphan": true,
		"confirm":      "delete-orphan-runtime-resource",
	})
	if res.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d: %s", http.StatusConflict, res.Code, res.Body.String())
	}
	if cleaner.deleted != nil {
		t.Fatalf("expected reason drift to skip delete, got %+v", cleaner.deleted)
	}
}

func ptr[T any](value T) *T {
	return &value
}

func stringSliceContains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func anySliceContainsString(values []any, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func managedResourceCountsContain(values []mboxruntime.ManagedResourceCount, name string, count int) bool {
	for _, item := range values {
		if item.Name == name && item.Count == count {
			return true
		}
	}
	return false
}

func parametersContainName(values []any, name string) bool {
	for _, item := range values {
		parameter, ok := item.(map[string]any)
		if ok && parameter["name"] == name {
			return true
		}
	}
	return false
}

func operationHasPublicSecurity(operation map[string]any) bool {
	security, ok := operation["security"].([]any)
	return ok && len(security) == 0
}

func operationRequiresBearerSecurity(operation map[string]any) bool {
	security, ok := operation["security"].([]any)
	if !ok {
		return false
	}
	for _, item := range security {
		requirement, ok := item.(map[string]any)
		if ok && requirement["bearerAuth"] != nil {
			return true
		}
	}
	return false
}

func hasUsageValue(values []domain.ResourceUsageValue, value string, count int) bool {
	for _, item := range values {
		if item.Value == value && item.Count == count {
			return true
		}
	}
	return false
}

func TestDeleteProjectRejectsSandboxCleanupPending(t *testing.T) {
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
		Namespace: sandbox.Namespace,
		Name:      "dev",
	}
	if _, err := store.UpdateSandbox(context.Background(), sandbox.ID, domain.SandboxUpdate{RuntimeRef: &runtimeRef}); err != nil {
		t.Fatal(err)
	}

	res := request(api, http.MethodDelete, "/v1/projects/"+project.ID.String(), nil)
	if res.Code != http.StatusConflict {
		t.Fatalf("expected delete conflict, got %d: %s", res.Code, res.Body.String())
	}

	if err := store.DeleteSandbox(context.Background(), sandbox.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkSandboxRuntimeDeleted(context.Background(), sandbox.ID); err != nil {
		t.Fatal(err)
	}

	res = request(api, http.MethodDelete, "/v1/projects/"+project.ID.String(), nil)
	if res.Code != http.StatusNoContent {
		t.Fatalf("expected delete status %d after cleanup, got %d: %s", http.StatusNoContent, res.Code, res.Body.String())
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

func TestTemplateValidationRunRoutes(t *testing.T) {
	store := newFakeStore()
	api := New(store)
	project := store.mustProject(t)
	template := store.mustTemplate(t, &project.ID)

	createRes := request(api, http.MethodPost, "/v1/templates/"+template.ID.String()+"/validation-runs", map[string]any{})
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected create status %d, got %d: %s", http.StatusCreated, createRes.Code, createRes.Body.String())
	}
	var created templateValidationRunResponse
	decodeResponse(t, createRes, &created)
	if created.Template.ID != template.ID || created.Sandbox.TemplateID != template.ID {
		t.Fatalf("unexpected validation response: %+v", created)
	}
	if created.Sandbox.ProjectID != project.ID ||
		created.Sandbox.Status != domain.SandboxStatusPending ||
		created.Sandbox.ServiceAccountName != defaultSandboxServiceAccountName {
		t.Fatalf("unexpected validation sandbox: %+v", created.Sandbox)
	}
	var templateMetadata map[string]any
	if err := json.Unmarshal(created.Template.Metadata, &templateMetadata); err != nil {
		t.Fatal(err)
	}
	if templateMetadata["validationStatus"] != "testing" ||
		templateMetadata["validationSandboxId"] != created.Sandbox.ID.String() {
		t.Fatalf("unexpected template metadata: %#v", templateMetadata)
	}
	var sandboxMetadata map[string]any
	if err := json.Unmarshal(created.Sandbox.Metadata, &sandboxMetadata); err != nil {
		t.Fatal(err)
	}
	if sandboxMetadata["purpose"] != templateValidationPurpose ||
		sandboxMetadata["templateId"] != template.ID.String() {
		t.Fatalf("unexpected sandbox metadata: %#v", sandboxMetadata)
	}

	decisionRes := request(
		api,
		http.MethodPost,
		"/v1/templates/"+template.ID.String()+"/validation-runs/"+created.Sandbox.ID.String()+"/decision",
		map[string]any{"status": "passed"},
	)
	if decisionRes.Code != http.StatusOK {
		t.Fatalf("expected decision status %d, got %d: %s", http.StatusOK, decisionRes.Code, decisionRes.Body.String())
	}
	var decided templateValidationRunResponse
	decodeResponse(t, decisionRes, &decided)
	if err := json.Unmarshal(decided.Template.Metadata, &templateMetadata); err != nil {
		t.Fatal(err)
	}
	if templateMetadata["validationStatus"] != "passed" ||
		templateMetadata["validationSandboxId"] != created.Sandbox.ID.String() {
		t.Fatalf("unexpected decided template metadata: %#v", templateMetadata)
	}
	if err := json.Unmarshal(decided.Sandbox.Metadata, &sandboxMetadata); err != nil {
		t.Fatal(err)
	}
	if sandboxMetadata["validationResult"] != "passed" {
		t.Fatalf("unexpected decided sandbox metadata: %#v", sandboxMetadata)
	}
}

func TestGlobalTemplateValidationRequiresProject(t *testing.T) {
	store := newFakeStore()
	api := New(store)
	template := store.mustTemplate(t, nil)

	res := request(api, http.MethodPost, "/v1/templates/"+template.ID.String()+"/validation-runs", map[string]any{})
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, res.Code, res.Body.String())
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

func TestCreateSandboxRecordsAuditEvent(t *testing.T) {
	store := newFakeStore()
	api := New(store)
	project := store.mustProject(t)
	template := store.mustTemplate(t, &project.ID)

	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(map[string]any{
		"projectId":  project.ID,
		"templateId": template.ID,
		"name":       "Audit Dev",
		"slug":       "audit-dev",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/sandboxes", &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mbox-Audit-Actor", "cli-user")
	req.Header.Set("X-Mbox-Audit-Source", "mbox-cli")
	req.Header.Set("X-Mbox-Request-ID", " cli-smoke-request-1 ")
	res := httptest.NewRecorder()
	api.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, res.Code, res.Body.String())
	}
	if got := res.Header().Get("X-Mbox-Request-ID"); got != "cli-smoke-request-1" {
		t.Fatalf("expected echoed request id, got %q", got)
	}

	eventsRes := request(api, http.MethodGet, "/v1/projects/"+project.ID.String()+"/audit-events", nil)
	if eventsRes.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, eventsRes.Code, eventsRes.Body.String())
	}
	var events ListResponse[domain.AuditEvent]
	decodeResponse(t, eventsRes, &events)
	if len(events.Items) != 1 {
		t.Fatalf("expected one audit event, got %+v", events.Items)
	}
	event := events.Items[0]
	if event.Action != "sandbox.created" || event.ResourceType != "sandbox" || event.ProjectID == nil || *event.ProjectID != project.ID {
		t.Fatalf("unexpected audit event: %+v", event)
	}
	if event.Actor != "cli-user" || event.Source != "mbox-cli" {
		t.Fatalf("expected client-supplied audit attribution, got actor=%q source=%q", event.Actor, event.Source)
	}
	var metadata map[string]any
	if err := json.Unmarshal(event.Metadata, &metadata); err != nil {
		t.Fatalf("decode audit metadata: %v", err)
	}
	if got := metadata["requestId"]; got != "cli-smoke-request-1" {
		t.Fatalf("expected audit metadata requestId, got %#v in %#v", got, metadata)
	}

	filteredRes := request(
		api,
		http.MethodGet,
		"/v1/projects/"+project.ID.String()+"/audit-events?actor=cli-user&source=mbox-cli",
		nil,
	)
	if filteredRes.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, filteredRes.Code, filteredRes.Body.String())
	}
	var filtered ListResponse[domain.AuditEvent]
	decodeResponse(t, filteredRes, &filtered)
	if len(filtered.Items) != 1 || filtered.Items[0].ID != event.ID {
		t.Fatalf("expected attribution-filtered audit event, got %+v", filtered.Items)
	}

	actionRes := request(
		api,
		http.MethodGet,
		"/v1/projects/"+project.ID.String()+"/audit-events?action=sandbox.created",
		nil,
	)
	if actionRes.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, actionRes.Code, actionRes.Body.String())
	}
	var actionFiltered ListResponse[domain.AuditEvent]
	decodeResponse(t, actionRes, &actionFiltered)
	if len(actionFiltered.Items) != 1 || actionFiltered.Items[0].ID != event.ID {
		t.Fatalf("expected action-filtered audit event, got %+v", actionFiltered.Items)
	}

	requestIDRes := request(
		api,
		http.MethodGet,
		"/v1/projects/"+project.ID.String()+"/audit-events?requestId=cli-smoke-request-1",
		nil,
	)
	if requestIDRes.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, requestIDRes.Code, requestIDRes.Body.String())
	}
	var requestIDFiltered ListResponse[domain.AuditEvent]
	decodeResponse(t, requestIDRes, &requestIDFiltered)
	if len(requestIDFiltered.Items) != 1 || requestIDFiltered.Items[0].ID != event.ID {
		t.Fatalf("expected request-id-filtered audit event, got %+v", requestIDFiltered.Items)
	}

	wrongRequestIDRes := request(
		api,
		http.MethodGet,
		"/v1/projects/"+project.ID.String()+"/audit-events?requestId=other-request",
		nil,
	)
	if wrongRequestIDRes.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, wrongRequestIDRes.Code, wrongRequestIDRes.Body.String())
	}
	var wrongRequestID ListResponse[domain.AuditEvent]
	decodeResponse(t, wrongRequestIDRes, &wrongRequestID)
	if len(wrongRequestID.Items) != 0 {
		t.Fatalf("expected no audit events for other request id, got %+v", wrongRequestID.Items)
	}

	since := event.CreatedAt.Add(-time.Second).Format(time.RFC3339Nano)
	until := event.CreatedAt.Add(time.Second).Format(time.RFC3339Nano)
	timeWindowRes := request(
		api,
		http.MethodGet,
		"/v1/projects/"+project.ID.String()+"/audit-events?since="+url.QueryEscape(since)+"&until="+url.QueryEscape(until),
		nil,
	)
	if timeWindowRes.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, timeWindowRes.Code, timeWindowRes.Body.String())
	}
	var timeWindowFiltered ListResponse[domain.AuditEvent]
	decodeResponse(t, timeWindowRes, &timeWindowFiltered)
	if len(timeWindowFiltered.Items) != 1 || timeWindowFiltered.Items[0].ID != event.ID {
		t.Fatalf("expected time-window-filtered audit event, got %+v", timeWindowFiltered.Items)
	}

	futureRes := request(
		api,
		http.MethodGet,
		"/v1/projects/"+project.ID.String()+"/audit-events?since="+url.QueryEscape(event.CreatedAt.Add(time.Second).Format(time.RFC3339Nano)),
		nil,
	)
	if futureRes.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, futureRes.Code, futureRes.Body.String())
	}
	var futureFiltered ListResponse[domain.AuditEvent]
	decodeResponse(t, futureRes, &futureFiltered)
	if len(futureFiltered.Items) != 0 {
		t.Fatalf("expected no audit events for future since, got %+v", futureFiltered.Items)
	}

	invalidSinceRes := request(
		api,
		http.MethodGet,
		"/v1/projects/"+project.ID.String()+"/audit-events?since=not-a-time",
		nil,
	)
	if invalidSinceRes.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, invalidSinceRes.Code, invalidSinceRes.Body.String())
	}

	invertedWindowRes := request(
		api,
		http.MethodGet,
		"/v1/projects/"+project.ID.String()+"/audit-events?since="+url.QueryEscape(event.CreatedAt.Add(time.Second).Format(time.RFC3339Nano))+"&until="+url.QueryEscape(event.CreatedAt.Add(-time.Second).Format(time.RFC3339Nano)),
		nil,
	)
	if invertedWindowRes.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, invertedWindowRes.Code, invertedWindowRes.Body.String())
	}

	operationRes := request(
		api,
		http.MethodGet,
		"/v1/projects/"+project.ID.String()+"/audit-events?operation=sandbox.create",
		nil,
	)
	if operationRes.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, operationRes.Code, operationRes.Body.String())
	}
	var operationFiltered ListResponse[domain.AuditEvent]
	decodeResponse(t, operationRes, &operationFiltered)
	if len(operationFiltered.Items) != 0 {
		t.Fatalf("expected no audit events for operation-less event, got %+v", operationFiltered.Items)
	}

	wrongActionRes := request(
		api,
		http.MethodGet,
		"/v1/projects/"+project.ID.String()+"/audit-events?action=project.deleted",
		nil,
	)
	if wrongActionRes.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, wrongActionRes.Code, wrongActionRes.Body.String())
	}
	var wrongAction ListResponse[domain.AuditEvent]
	decodeResponse(t, wrongActionRes, &wrongAction)
	if len(wrongAction.Items) != 0 {
		t.Fatalf("expected no audit events for other action, got %+v", wrongAction.Items)
	}

	emptyRes := request(
		api,
		http.MethodGet,
		"/v1/projects/"+project.ID.String()+"/audit-events?actor=other&source=mbox-cli",
		nil,
	)
	if emptyRes.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, emptyRes.Code, emptyRes.Body.String())
	}
	var empty ListResponse[domain.AuditEvent]
	decodeResponse(t, emptyRes, &empty)
	if len(empty.Items) != 0 {
		t.Fatalf("expected no audit events for other actor, got %+v", empty.Items)
	}
}

func TestRequestIDGeneratedForResponsesAndAuditEvents(t *testing.T) {
	store := newFakeStore()
	api := New(store)
	project := store.mustProject(t)
	template := store.mustTemplate(t, &project.ID)

	res := request(api, http.MethodPost, "/v1/sandboxes", map[string]any{
		"projectId":  project.ID,
		"templateId": template.ID,
		"name":       "Generated Request ID",
		"slug":       "generated-request-id",
	})
	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, res.Code, res.Body.String())
	}
	requestID := res.Header().Get("X-Mbox-Request-ID")
	if _, err := uuid.Parse(requestID); err != nil {
		t.Fatalf("expected generated request id UUID, got %q: %v", requestID, err)
	}

	eventsRes := request(api, http.MethodGet, "/v1/projects/"+project.ID.String()+"/audit-events", nil)
	if eventsRes.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, eventsRes.Code, eventsRes.Body.String())
	}
	var events ListResponse[domain.AuditEvent]
	decodeResponse(t, eventsRes, &events)
	if len(events.Items) != 1 {
		t.Fatalf("expected one audit event, got %+v", events.Items)
	}
	var metadata map[string]any
	if err := json.Unmarshal(events.Items[0].Metadata, &metadata); err != nil {
		t.Fatalf("decode audit metadata: %v", err)
	}
	if got := metadata["requestId"]; got != requestID {
		t.Fatalf("expected generated request id in audit metadata, got %#v in %#v", got, metadata)
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

func TestProjectPolicyRoutes(t *testing.T) {
	store := newFakeStore()
	api := New(store)
	project := store.mustProject(t)

	defaultRes := request(api, http.MethodGet, "/v1/projects/"+project.ID.String()+"/policy", nil)
	if defaultRes.Code != http.StatusOK {
		t.Fatalf("expected default policy status %d, got %d: %s", http.StatusOK, defaultRes.Code, defaultRes.Body.String())
	}
	var defaultPolicy domain.ProjectPolicy
	decodeResponse(t, defaultRes, &defaultPolicy)
	if defaultPolicy.Enforcement != domain.ProjectPolicyEnforcementDisabled || defaultPolicy.ProjectID != project.ID {
		t.Fatalf("unexpected default policy: %+v", defaultPolicy)
	}

	putRes := request(api, http.MethodPut, "/v1/projects/"+project.ID.String()+"/policy", map[string]any{
		"enforcement":            "enforced",
		"allowedImagePrefixes":   []string{"busybox:", "busybox:"},
		"allowedServiceAccounts": []string{"mbox-sandbox"},
		"allowedSecretRefs":      []string{"git-token"},
	})
	if putRes.Code != http.StatusOK {
		t.Fatalf("expected put policy status %d, got %d: %s", http.StatusOK, putRes.Code, putRes.Body.String())
	}
	var policy domain.ProjectPolicy
	decodeResponse(t, putRes, &policy)
	if policy.Enforcement != domain.ProjectPolicyEnforcementEnforced ||
		len(policy.AllowedImagePrefixes) != 1 ||
		policy.AllowedImagePrefixes[0] != "busybox:" ||
		policy.AllowedServiceAccounts[0] != "mbox-sandbox" ||
		policy.AllowedSecretRefs[0] != "git-token" {
		t.Fatalf("unexpected policy: %+v", policy)
	}
}

func TestProjectCredentialRoutes(t *testing.T) {
	store := newFakeStore()
	api := New(store)
	project := store.mustProject(t)

	createRes := request(api, http.MethodPost, "/v1/projects/"+project.ID.String()+"/credentials", map[string]any{
		"name":   "GitHub App",
		"slug":   "github-app",
		"type":   "git",
		"target": "https://github.com/mlhiter/mbox",
		"secretRef": map[string]any{
			"name": "github-app-token",
			"key":  "token",
		},
		"usage":    []string{"clone", "fetch"},
		"metadata": map[string]any{"owner": "platform"},
	})
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected create credential status %d, got %d: %s", http.StatusCreated, createRes.Code, createRes.Body.String())
	}
	var credential domain.ProjectCredential
	decodeResponse(t, createRes, &credential)
	if credential.ProjectID != project.ID ||
		credential.Type != domain.ProjectCredentialTypeGit ||
		credential.SecretRef.Name != "github-app-token" ||
		len(credential.Usage) != 2 {
		t.Fatalf("unexpected credential: %+v", credential)
	}

	listRes := request(api, http.MethodGet, "/v1/projects/"+project.ID.String()+"/credentials", nil)
	if listRes.Code != http.StatusOK {
		t.Fatalf("expected list credential status %d, got %d: %s", http.StatusOK, listRes.Code, listRes.Body.String())
	}
	var list ListResponse[domain.ProjectCredential]
	decodeResponse(t, listRes, &list)
	if len(list.Items) != 1 || list.Items[0].ID != credential.ID {
		t.Fatalf("unexpected credential list: %+v", list.Items)
	}

	getRes := request(api, http.MethodGet, "/v1/credentials/"+credential.ID.String(), nil)
	if getRes.Code != http.StatusOK {
		t.Fatalf("expected get credential status %d, got %d: %s", http.StatusOK, getRes.Code, getRes.Body.String())
	}

	deleteRes := request(api, http.MethodDelete, "/v1/credentials/"+credential.ID.String(), nil)
	if deleteRes.Code != http.StatusNoContent {
		t.Fatalf("expected delete credential status %d, got %d: %s", http.StatusNoContent, deleteRes.Code, deleteRes.Body.String())
	}
}

func TestProjectCredentialCreateValidatesTypeAndSecretRef(t *testing.T) {
	store := newFakeStore()
	api := New(store)
	project := store.mustProject(t)

	res := request(api, http.MethodPost, "/v1/projects/"+project.ID.String()+"/credentials", map[string]any{
		"name":      "Bad Credential",
		"type":      "cluster-admin",
		"secretRef": map[string]any{"name": ""},
	})
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, res.Code, res.Body.String())
	}
}

func TestProjectUsageRouteSummarizesProductRecords(t *testing.T) {
	store := newFakeStore()
	api := New(store)
	project := store.mustProject(t)
	globalTemplate := store.mustTemplate(t, nil)
	globalTemplate.CPURequest = "250m"
	globalTemplate.MemoryRequest = "512Mi"
	globalTemplate.StorageRequest = "1Gi"
	store.templates[globalTemplate.ID] = globalTemplate
	template := store.mustTemplate(t, &project.ID)
	template.CPURequest = "500m"
	template.MemoryRequest = "1Gi"
	template.StorageRequest = "2Gi"
	store.templates[template.ID] = template
	otherProject := store.mustProject(t)
	otherTemplate := store.mustTemplate(t, &otherProject.ID)
	otherTemplate.CPURequest = "8"
	store.templates[otherTemplate.ID] = otherTemplate

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
	ref := &domain.RuntimeRef{Adapter: "agent-sandbox", Kind: "SandboxClaim", Namespace: "mbox-demo", Name: "dev"}
	if _, err := store.UpdateSandbox(context.Background(), sandbox.ID, domain.SandboxUpdate{Status: &running, RuntimeRef: &ref}); err != nil {
		t.Fatal(err)
	}
	pendingDelete, err := store.CreateSandbox(context.Background(), domain.SandboxCreate{
		ProjectID:          project.ID,
		TemplateID:         template.ID,
		Name:               "Deleted",
		Slug:               "deleted",
		Namespace:          "mbox-demo",
		ServiceAccountName: "mbox-sandbox",
	})
	if err != nil {
		t.Fatal(err)
	}
	deleteRef := &domain.RuntimeRef{Adapter: "agent-sandbox", Kind: "SandboxClaim", Namespace: "mbox-demo", Name: "deleted"}
	if _, err := store.UpdateSandbox(context.Background(), pendingDelete.ID, domain.SandboxUpdate{RuntimeRef: &deleteRef}); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteSandbox(context.Background(), pendingDelete.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateRuntimeSession(context.Background(), domain.RuntimeSessionCreate{
		ProjectID: project.ID,
		SandboxID: sandbox.ID,
		Type:      domain.RuntimeSessionTypeTerminal,
		Client:    "web-terminal",
	}); err != nil {
		t.Fatal(err)
	}
	session, err := store.CreateRuntimeSession(context.Background(), domain.RuntimeSessionCreate{
		ProjectID: project.ID,
		SandboxID: sandbox.ID,
		Type:      domain.RuntimeSessionTypeCustom,
		Client:    "sdk-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	ended := domain.RuntimeSessionStatusEnded
	endedAt := time.Now()
	if _, err := store.UpdateRuntimeSession(context.Background(), session.ID, domain.RuntimeSessionUpdate{Status: &ended, EndedAt: &endedAt}); err != nil {
		t.Fatal(err)
	}
	task, err := store.CreateExecutionTask(context.Background(), domain.ExecutionTaskCreate{
		ProjectID:      project.ID,
		SandboxID:      sandbox.ID,
		Command:        []string{"sh", "-lc", "echo ok"},
		TimeoutSeconds: 30,
	})
	if err != nil {
		t.Fatal(err)
	}
	succeeded := domain.ExecutionTaskStatusSucceeded
	if _, err := store.UpdateExecutionTask(context.Background(), task.ID, domain.ExecutionTaskUpdate{Status: &succeeded}); err != nil {
		t.Fatal(err)
	}
	failedTask, err := store.CreateExecutionTask(context.Background(), domain.ExecutionTaskCreate{
		ProjectID:      project.ID,
		SandboxID:      sandbox.ID,
		Command:        []string{"false"},
		TimeoutSeconds: 30,
	})
	if err != nil {
		t.Fatal(err)
	}
	failed := domain.ExecutionTaskStatusFailed
	if _, err := store.UpdateExecutionTask(context.Background(), failedTask.ID, domain.ExecutionTaskUpdate{Status: &failed}); err != nil {
		t.Fatal(err)
	}
	sizeBytes := int64(128)
	artifact, err := store.CreateArtifact(context.Background(), domain.ArtifactCreate{
		ProjectID:   project.ID,
		SandboxID:   sandbox.ID,
		TaskID:      &task.ID,
		Kind:        domain.ArtifactKindReport,
		Name:        "Report",
		URI:         "workspace:///workspace/report.json",
		ContentType: "application/json",
		SizeBytes:   &sizeBytes,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CaptureArtifactContent(context.Background(), domain.ArtifactContentCapture{
		ArtifactID:      artifact.ID,
		Content:         []byte("{}"),
		ContentType:     "application/json",
		SizeBytes:       2,
		SHA256:          "44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a",
		SourceURI:       artifact.URI,
		StorageProvider: domain.ArtifactContentStorageProviderPostgres,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateProjectCredential(context.Background(), domain.ProjectCredentialCreate{
		ProjectID: project.ID,
		Name:      "Registry",
		Slug:      "registry",
		Type:      domain.ProjectCredentialTypeRegistry,
		SecretRef: domain.SecretRef{Name: "registry-token"},
	}); err != nil {
		t.Fatal(err)
	}

	res := request(api, http.MethodGet, "/v1/projects/"+project.ID.String()+"/usage", nil)
	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, res.Code, res.Body.String())
	}
	var usage domain.ProjectUsage
	decodeResponse(t, res, &usage)
	if usage.ProjectID != project.ID || usage.GeneratedAt.IsZero() {
		t.Fatalf("unexpected usage identity: %+v", usage)
	}
	if usage.Sandboxes.Total != 2 || usage.Sandboxes.Active != 1 || usage.Sandboxes.Running != 1 ||
		usage.Sandboxes.Deleted != 1 || usage.Sandboxes.CleanupPending != 1 {
		t.Fatalf("unexpected sandbox usage: %+v", usage.Sandboxes)
	}
	if usage.Sandboxes.ActiveRequests.Count != 1 ||
		usage.Sandboxes.ActiveRequests.CPU.Total != "500m" ||
		usage.Sandboxes.ActiveRequests.Memory.Total != "1Gi" ||
		usage.Sandboxes.ActiveRequests.Storage.Total != "2Gi" ||
		usage.Sandboxes.RunningRequests.Count != 1 ||
		usage.Sandboxes.RunningRequests.CPU.Total != "500m" {
		t.Fatalf("unexpected sandbox request usage: %+v", usage.Sandboxes)
	}
	if usage.RuntimeSessions.Total != 2 || usage.RuntimeSessions.Active != 1 ||
		usage.RuntimeSessions.Ended != 1 || usage.RuntimeSessions.Terminal != 1 || usage.RuntimeSessions.Custom != 1 {
		t.Fatalf("unexpected session usage: %+v", usage.RuntimeSessions)
	}
	if usage.ExecutionTasks.Total != 2 || usage.ExecutionTasks.Succeeded != 1 || usage.ExecutionTasks.Failed != 1 {
		t.Fatalf("unexpected task usage: %+v", usage.ExecutionTasks)
	}
	if usage.Artifacts.Total != 1 || usage.Artifacts.Report != 1 || usage.Artifacts.ReferencedBytes != 128 ||
		usage.Artifacts.RetainedContent != 1 || usage.Artifacts.RetainedBytes != 2 {
		t.Fatalf("unexpected artifact usage: %+v", usage.Artifacts)
	}
	if usage.Templates.ProjectScoped != 1 || usage.Templates.GlobalVisible != 1 ||
		!hasUsageValue(usage.Templates.CPURequests, "500m", 1) ||
		!hasUsageValue(usage.Templates.CPURequests, "250m", 1) ||
		hasUsageValue(usage.Templates.CPURequests, "8", 1) {
		t.Fatalf("unexpected template usage: %+v", usage.Templates)
	}
	if usage.Credentials.Total != 1 || usage.Credentials.Registry != 1 {
		t.Fatalf("unexpected credential usage: %+v", usage.Credentials)
	}
}

func TestProjectUsageRouteReturnsNotFound(t *testing.T) {
	api := New(newFakeStore())
	res := request(api, http.MethodGet, "/v1/projects/"+uuid.NewString()+"/usage", nil)
	if res.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, res.Code, res.Body.String())
	}
}

func TestCreateSandboxRejectsCrossProjectTemplate(t *testing.T) {
	store := newFakeStore()
	api := New(store)
	project := store.mustProject(t)
	otherProject := store.mustProject(t)
	template := store.mustTemplate(t, &otherProject.ID)

	res := request(api, http.MethodPost, "/v1/sandboxes", map[string]any{
		"projectId":  project.ID,
		"templateId": template.ID,
		"name":       "Wrong Scope",
		"slug":       "wrong-scope",
	})
	if res.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "template belongs to a different project") {
		t.Fatalf("expected cross-project policy message, got %s", res.Body.String())
	}
}

func TestProjectQuotaPolicyRouteReturnsDefaultAndUpdates(t *testing.T) {
	store := newFakeStore()
	api := New(store)
	project := store.mustProject(t)

	getRes := request(api, http.MethodGet, "/v1/projects/"+project.ID.String()+"/quota-policy", nil)
	if getRes.Code != http.StatusOK {
		t.Fatalf("expected default status %d, got %d: %s", http.StatusOK, getRes.Code, getRes.Body.String())
	}
	var defaultPolicy domain.ProjectQuotaPolicy
	decodeResponse(t, getRes, &defaultPolicy)
	if defaultPolicy.ProjectID != project.ID || defaultPolicy.Enforcement != domain.ProjectQuotaPolicyEnforcementDisabled {
		t.Fatalf("unexpected default quota policy: %+v", defaultPolicy)
	}
	if defaultPolicy.MaxActiveSandboxes != nil || defaultPolicy.MaxRetainedArtifactBytes != nil {
		t.Fatalf("expected empty default limits, got %+v", defaultPolicy)
	}

	putRes := request(api, http.MethodPut, "/v1/projects/"+project.ID.String()+"/quota-policy", map[string]any{
		"enforcement":              "enforced",
		"maxActiveSandboxes":       3,
		"maxRetainedArtifactBytes": 1024,
	})
	if putRes.Code != http.StatusOK {
		t.Fatalf("expected update status %d, got %d: %s", http.StatusOK, putRes.Code, putRes.Body.String())
	}
	var updated domain.ProjectQuotaPolicy
	decodeResponse(t, putRes, &updated)
	if updated.Enforcement != domain.ProjectQuotaPolicyEnforcementEnforced ||
		updated.MaxActiveSandboxes == nil || *updated.MaxActiveSandboxes != 3 ||
		updated.MaxRetainedArtifactBytes == nil || *updated.MaxRetainedArtifactBytes != 1024 {
		t.Fatalf("unexpected updated quota policy: %+v", updated)
	}
}

func TestProjectQuotaPolicyRejectsInvalidInput(t *testing.T) {
	store := newFakeStore()
	api := New(store)
	project := store.mustProject(t)

	res := request(api, http.MethodPut, "/v1/projects/"+project.ID.String()+"/quota-policy", map[string]any{
		"enforcement":        "enforced",
		"maxActiveSandboxes": -1,
	})
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "maxActiveSandboxes cannot be negative") {
		t.Fatalf("expected validation message, got %s", res.Body.String())
	}
}

func TestProjectQuotaPolicyEnforcesActiveSandboxLimit(t *testing.T) {
	store := newFakeStore()
	api := New(store)
	project := store.mustProject(t)
	template := store.mustTemplate(t, &project.ID)
	maxActiveSandboxes := 0
	if _, err := store.UpsertProjectQuotaPolicy(context.Background(), project.ID, domain.ProjectQuotaPolicyUpsert{
		Enforcement:        domain.ProjectQuotaPolicyEnforcementEnforced,
		MaxActiveSandboxes: &maxActiveSandboxes,
	}); err != nil {
		t.Fatal(err)
	}

	res := request(api, http.MethodPost, "/v1/sandboxes", map[string]any{
		"projectId":  project.ID,
		"templateId": template.ID,
		"name":       "Denied Quota",
		"slug":       "denied-quota",
	})
	if res.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "active sandbox quota exceeded") {
		t.Fatalf("expected active sandbox quota message, got %s", res.Body.String())
	}

	auditRes := request(api, http.MethodGet, "/v1/projects/"+project.ID.String()+"/audit-events?resourceType=sandbox&limit=10", nil)
	if auditRes.Code != http.StatusOK {
		t.Fatalf("expected audit status %d, got %d: %s", http.StatusOK, auditRes.Code, auditRes.Body.String())
	}
	var auditList ListResponse[domain.AuditEvent]
	decodeResponse(t, auditRes, &auditList)
	if len(auditList.Items) == 0 {
		t.Fatal("expected policy denial audit event")
	}
	event := auditList.Items[0]
	if event.Action != "policy.denied" || event.ResourceType != "sandbox" || event.ResourceName != "Denied Quota" {
		t.Fatalf("unexpected audit event: %+v", event)
	}
	var metadata map[string]any
	if err := json.Unmarshal(event.Metadata, &metadata); err != nil {
		t.Fatalf("decode audit metadata: %v", err)
	}
	if metadata["operation"] != "sandbox.launch" || !strings.Contains(fmt.Sprint(metadata["reason"]), "active sandbox quota exceeded") {
		t.Fatalf("unexpected audit metadata: %#v", metadata)
	}

	operationRes := request(api, http.MethodGet, "/v1/projects/"+project.ID.String()+"/audit-events?action=policy.denied&operation=sandbox.launch&limit=10", nil)
	if operationRes.Code != http.StatusOK {
		t.Fatalf("expected audit status %d, got %d: %s", http.StatusOK, operationRes.Code, operationRes.Body.String())
	}
	var operationList ListResponse[domain.AuditEvent]
	decodeResponse(t, operationRes, &operationList)
	if len(operationList.Items) != 1 || operationList.Items[0].ID != event.ID {
		t.Fatalf("expected operation-filtered denial event, got %+v", operationList.Items)
	}

	wrongOperationRes := request(api, http.MethodGet, "/v1/projects/"+project.ID.String()+"/audit-events?action=policy.denied&operation=artifact.content.upload&limit=10", nil)
	if wrongOperationRes.Code != http.StatusOK {
		t.Fatalf("expected audit status %d, got %d: %s", http.StatusOK, wrongOperationRes.Code, wrongOperationRes.Body.String())
	}
	var wrongOperationList ListResponse[domain.AuditEvent]
	decodeResponse(t, wrongOperationRes, &wrongOperationList)
	if len(wrongOperationList.Items) != 0 {
		t.Fatalf("expected no events for other operation, got %+v", wrongOperationList.Items)
	}
}

func TestProjectQuotaPolicyEnforcesRetainedArtifactBytes(t *testing.T) {
	store := newFakeStore()
	api := New(store)
	sandbox := store.mustRunningSandbox(t)
	artifact, err := store.CreateArtifact(context.Background(), domain.ArtifactCreate{
		ProjectID: sandbox.ProjectID,
		SandboxID: sandbox.ID,
		Kind:      domain.ArtifactKindReport,
		Name:      "Quota report",
		URI:       "client://reports/quota.txt",
	})
	if err != nil {
		t.Fatal(err)
	}
	maxRetainedBytes := int64(2)
	if _, err := store.UpsertProjectQuotaPolicy(context.Background(), sandbox.ProjectID, domain.ProjectQuotaPolicyUpsert{
		Enforcement:              domain.ProjectQuotaPolicyEnforcementEnforced,
		MaxRetainedArtifactBytes: &maxRetainedBytes,
	}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPut, "/v1/artifacts/"+artifact.ID.String()+"/content", strings.NewReader("too large"))
	res := httptest.NewRecorder()
	api.ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "retained artifact quota exceeded") {
		t.Fatalf("expected retained artifact quota message, got %s", res.Body.String())
	}

	auditRes := request(api, http.MethodGet, "/v1/projects/"+sandbox.ProjectID.String()+"/audit-events?resourceType=artifact&limit=10", nil)
	if auditRes.Code != http.StatusOK {
		t.Fatalf("expected audit status %d, got %d: %s", http.StatusOK, auditRes.Code, auditRes.Body.String())
	}
	var auditList ListResponse[domain.AuditEvent]
	decodeResponse(t, auditRes, &auditList)
	if len(auditList.Items) == 0 {
		t.Fatal("expected artifact quota denial audit event")
	}
	event := auditList.Items[0]
	if event.Action != "policy.denied" || event.ResourceType != "artifact" || event.ResourceID == nil || *event.ResourceID != artifact.ID {
		t.Fatalf("unexpected audit event: %+v", event)
	}
	var metadata map[string]any
	if err := json.Unmarshal(event.Metadata, &metadata); err != nil {
		t.Fatalf("decode audit metadata: %v", err)
	}
	if metadata["operation"] != "artifact.content.upload" || !strings.Contains(fmt.Sprint(metadata["reason"]), "retained artifact quota exceeded") {
		t.Fatalf("unexpected audit metadata: %#v", metadata)
	}
}

func TestProjectPolicyEnforcesSandboxLaunch(t *testing.T) {
	store := newFakeStore()
	api := New(store)
	project := store.mustProject(t)
	allowedTemplate := store.mustTemplate(t, &project.ID)
	allowedTemplate.Image = "busybox:1.37"
	store.templates[allowedTemplate.ID] = allowedTemplate
	disallowedImageTemplate := store.mustTemplate(t, &project.ID)
	disallowedImageTemplate.Image = "ubuntu:24.04"
	store.templates[disallowedImageTemplate.ID] = disallowedImageTemplate
	secretTemplate := store.mustTemplate(t, &project.ID)
	secretTemplate.Image = "busybox:1.37"
	secretTemplate.SecretRefs = []domain.SecretRef{{Name: "git-token", Key: "token"}}
	store.templates[secretTemplate.ID] = secretTemplate

	if _, err := store.UpsertProjectPolicy(context.Background(), project.ID, domain.ProjectPolicyUpsert{
		Enforcement:            domain.ProjectPolicyEnforcementEnforced,
		AllowedImagePrefixes:   []string{"busybox:"},
		AllowedServiceAccounts: []string{defaultSandboxServiceAccountName},
	}); err != nil {
		t.Fatal(err)
	}

	allowedRes := request(api, http.MethodPost, "/v1/sandboxes", map[string]any{
		"projectId":  project.ID,
		"templateId": allowedTemplate.ID,
		"name":       "Allowed",
		"slug":       "allowed",
	})
	if allowedRes.Code != http.StatusCreated {
		t.Fatalf("expected allowed status %d, got %d: %s", http.StatusCreated, allowedRes.Code, allowedRes.Body.String())
	}

	imageRes := request(api, http.MethodPost, "/v1/sandboxes", map[string]any{
		"projectId":  project.ID,
		"templateId": disallowedImageTemplate.ID,
		"name":       "Denied Image",
		"slug":       "denied-image",
	})
	if imageRes.Code != http.StatusForbidden || !strings.Contains(imageRes.Body.String(), "image") {
		t.Fatalf("expected image denial, got %d: %s", imageRes.Code, imageRes.Body.String())
	}

	serviceAccountRes := request(api, http.MethodPost, "/v1/sandboxes", map[string]any{
		"projectId":          project.ID,
		"templateId":         allowedTemplate.ID,
		"name":               "Denied Identity",
		"slug":               "denied-identity",
		"serviceAccountName": "cluster-admin",
	})
	if serviceAccountRes.Code != http.StatusForbidden || !strings.Contains(serviceAccountRes.Body.String(), "serviceAccountName") {
		t.Fatalf("expected service account denial, got %d: %s", serviceAccountRes.Code, serviceAccountRes.Body.String())
	}

	secretRes := request(api, http.MethodPost, "/v1/sandboxes", map[string]any{
		"projectId":  project.ID,
		"templateId": secretTemplate.ID,
		"name":       "Denied Secret",
		"slug":       "denied-secret",
	})
	if secretRes.Code != http.StatusForbidden || !strings.Contains(secretRes.Body.String(), "secretRef") {
		t.Fatalf("expected secret denial, got %d: %s", secretRes.Code, secretRes.Body.String())
	}

	if _, err := store.UpsertProjectPolicy(context.Background(), project.ID, domain.ProjectPolicyUpsert{
		Enforcement:            domain.ProjectPolicyEnforcementEnforced,
		AllowedImagePrefixes:   []string{"busybox:"},
		AllowedServiceAccounts: []string{defaultSandboxServiceAccountName},
		AllowedSecretRefs:      []string{"git-token"},
	}); err != nil {
		t.Fatal(err)
	}
	allowedSecretRes := request(api, http.MethodPost, "/v1/sandboxes", map[string]any{
		"projectId":  project.ID,
		"templateId": secretTemplate.ID,
		"name":       "Allowed Secret",
		"slug":       "allowed-secret",
	})
	if allowedSecretRes.Code != http.StatusCreated {
		t.Fatalf("expected allowed secret status %d, got %d: %s", http.StatusCreated, allowedSecretRes.Code, allowedSecretRes.Body.String())
	}
}

func TestTemplateValidationRespectsProjectPolicy(t *testing.T) {
	store := newFakeStore()
	api := New(store)
	project := store.mustProject(t)
	template := store.mustTemplate(t, &project.ID)
	template.Image = "ubuntu:24.04"
	store.templates[template.ID] = template
	if _, err := store.UpsertProjectPolicy(context.Background(), project.ID, domain.ProjectPolicyUpsert{
		Enforcement:            domain.ProjectPolicyEnforcementEnforced,
		AllowedImagePrefixes:   []string{"busybox:"},
		AllowedServiceAccounts: []string{defaultSandboxServiceAccountName},
	}); err != nil {
		t.Fatal(err)
	}

	res := request(api, http.MethodPost, "/v1/templates/"+template.ID.String()+"/validation-runs", map[string]any{})
	if res.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "policy denied") {
		t.Fatalf("expected policy denial, got %s", res.Body.String())
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

func TestTemplateBoundarySummarizesSecurityContract(t *testing.T) {
	store := newFakeStore()
	api := New(store)
	project := store.mustProject(t)
	template := store.mustTemplate(t, &project.ID)
	template.NetworkPolicy = "default"
	template.SecretRefs = []domain.SecretRef{{Name: "git-credentials", Key: "token"}}
	template.LifecyclePolicy = json.RawMessage(`{"ttlSeconds":3600}`)
	template.CPURequest = "250m"
	template.MemoryRequest = "512Mi"
	template.ExposedPorts = []domain.TemplatePort{{Name: "web", Port: 3000, Protocol: "TCP"}}
	store.templates[template.ID] = template
	_, err := store.CreateProjectCredential(context.Background(), domain.ProjectCredentialCreate{
		ProjectID: project.ID,
		Name:      "GitHub App",
		Slug:      "github-app",
		Type:      domain.ProjectCredentialTypeGit,
		Target:    "https://github.com/mlhiter/mbox",
		SecretRef: domain.SecretRef{Name: "github-app-token", Key: "token"},
		Usage:     []string{"clone"},
	})
	if err != nil {
		t.Fatal(err)
	}

	res := request(api, http.MethodGet, "/v1/templates/"+template.ID.String()+"/boundary", nil)
	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, res.Code, res.Body.String())
	}
	var summary boundarySummary
	decodeResponse(t, res, &summary)
	if summary.ProjectID != project.ID.String() || summary.Namespace != project.DefaultNamespace {
		t.Fatalf("unexpected project boundary: %+v", summary)
	}
	if summary.ServiceAccountTokenAutomount {
		t.Fatal("expected token automount to be disabled")
	}
	if summary.SecretProjection != "references-recorded-not-mounted" || len(summary.SecretRefs) != 1 {
		t.Fatalf("unexpected secret projection: %+v", summary)
	}
	if summary.CredentialProjection != "references-recorded-not-mounted" ||
		len(summary.CredentialRefs) != 1 ||
		summary.CredentialRefs[0].SecretRef != "github-app-token" {
		t.Fatalf("unexpected credential projection: %+v", summary)
	}
	if summary.NetworkPolicyProjection != "agent-sandbox-managed-baseline" {
		t.Fatalf("unexpected network projection: %+v", summary)
	}
	if summary.LifecyclePolicyProjection != "ttl-enforced" {
		t.Fatalf("unexpected lifecycle projection: %+v", summary)
	}
	if len(summary.Checks) == 0 {
		t.Fatal("expected boundary checks")
	}
	if !hasBoundaryCheck(summary.Checks, "lifecycle-policy", "pass") {
		t.Fatalf("expected lifecycle policy check to pass for ttlSeconds, got %+v", summary.Checks)
	}
}

func TestGlobalTemplateBoundaryCanUseProjectQuery(t *testing.T) {
	store := newFakeStore()
	api := New(store)
	project := store.mustProject(t)
	template := store.mustTemplate(t, nil)

	res := request(api, http.MethodGet, "/v1/templates/"+template.ID.String()+"/boundary?projectId="+project.ID.String(), nil)
	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, res.Code, res.Body.String())
	}
	var summary boundarySummary
	decodeResponse(t, res, &summary)
	if summary.ProjectID != project.ID.String() || summary.Namespace != project.DefaultNamespace {
		t.Fatalf("unexpected global template boundary: %+v", summary)
	}
}

func TestSandboxBoundarySummarizesRuntimeIdentity(t *testing.T) {
	store := newFakeStore()
	api := New(store)
	project := store.mustProject(t)
	template := store.mustTemplate(t, &project.ID)
	sandbox, err := store.CreateSandbox(context.Background(), domain.SandboxCreate{
		ProjectID:          project.ID,
		TemplateID:         template.ID,
		Name:               "Dev",
		Slug:               "dev",
		Namespace:          "mbox-demo-runtime",
		ServiceAccountName: "mbox-sandbox",
	})
	if err != nil {
		t.Fatal(err)
	}
	runtimeRef := &domain.RuntimeRef{
		Adapter:   "agent-sandbox",
		Kind:      "SandboxClaim",
		Namespace: "mbox-demo-runtime",
		Name:      "dev",
	}
	sandbox.RuntimeRef = runtimeRef
	sandbox.Status = domain.SandboxStatusRunning
	store.sandboxes[sandbox.ID] = sandbox

	res := request(api, http.MethodGet, "/v1/sandboxes/"+sandbox.ID.String()+"/boundary", nil)
	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, res.Code, res.Body.String())
	}
	var summary boundarySummary
	decodeResponse(t, res, &summary)
	if summary.SandboxID != sandbox.ID.String() || summary.Namespace != "mbox-demo-runtime" || summary.RuntimeRef == nil {
		t.Fatalf("unexpected sandbox boundary: %+v", summary)
	}
	foundRuntimeRefCheck := false
	for _, check := range summary.Checks {
		if check.ID == "runtime-ref" && check.Status == "pass" {
			foundRuntimeRefCheck = true
		}
	}
	if !foundRuntimeRefCheck {
		t.Fatalf("expected passing runtime-ref check, got %+v", summary.Checks)
	}
}

func hasBoundaryCheck(checks []boundaryCheck, id string, status string) bool {
	for _, check := range checks {
		if check.ID == id && check.Status == status {
			return true
		}
	}
	return false
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

func TestExecutionTaskWatchStreamsStatusOutputAndDone(t *testing.T) {
	store := newFakeStore()
	access := newBlockingRuntimeAccess()
	api := NewWithRuntimeAccess(store, access)
	server := httptest.NewServer(api)
	defer server.Close()
	sandbox := store.mustRunningSandbox(t)

	createRes := request(api, http.MethodPost, "/v1/sandboxes/"+sandbox.ID.String()+"/tasks", map[string]any{
		"command": []string{"sh", "-lc", "printf ready"},
	})
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, createRes.Code, createRes.Body.String())
	}
	var task domain.ExecutionTask
	decodeResponse(t, createRes, &task)
	waitForExecStart(t, access)

	type taskEvent struct {
		Type   string                `json:"type"`
		Stream string                `json:"stream"`
		Data   string                `json:"data"`
		Task   *domain.ExecutionTask `json:"task"`
	}
	client := &http.Client{Timeout: 2 * time.Second}
	watchRes, err := client.Get(server.URL + "/v1/tasks/" + task.ID.String() + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer watchRes.Body.Close()
	if watchRes.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(watchRes.Body)
		t.Fatalf("expected watch status %d, got %d: %s", http.StatusOK, watchRes.StatusCode, string(body))
	}
	decoder := json.NewDecoder(watchRes.Body)
	events := []taskEvent{}
	var first taskEvent
	if err := decoder.Decode(&first); err != nil {
		t.Fatal(err)
	}
	events = append(events, first)
	access.release()
	for {
		var event taskEvent
		if err := decoder.Decode(&event); err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
		if event.Type == taskEventTypeDone {
			break
		}
	}
	if len(events) < 3 {
		t.Fatalf("expected snapshot, output, and done events, got %+v", events)
	}
	if events[0].Type != taskEventTypeSnapshot || events[0].Task == nil || events[0].Task.ID != task.ID {
		t.Fatalf("unexpected first event: %+v", events[0])
	}
	foundOutput := false
	foundDone := false
	for _, event := range events {
		if event.Type == taskEventTypeOutput && event.Stream == "stdout" && strings.Contains(event.Data, "released") {
			foundOutput = true
		}
		if event.Type == taskEventTypeDone && event.Task != nil && event.Task.Status == domain.ExecutionTaskStatusSucceeded {
			foundDone = true
		}
	}
	if !foundOutput || !foundDone {
		t.Fatalf("expected stdout output and done events, got %+v", events)
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

func TestArtifactContentReadsWorkspaceReference(t *testing.T) {
	store := newFakeStore()
	api := NewWithRuntimeAccess(store, &fakeRuntimeAccess{})
	sandbox := store.mustRunningSandbox(t)
	artifact, err := store.CreateArtifact(context.Background(), domain.ArtifactCreate{
		ProjectID:   sandbox.ProjectID,
		SandboxID:   sandbox.ID,
		Kind:        domain.ArtifactKindReport,
		Name:        "Test report",
		URI:         "workspace:///workspace/reports/test.json",
		ContentType: "application/json",
	})
	if err != nil {
		t.Fatal(err)
	}

	res := request(api, http.MethodGet, "/v1/artifacts/"+artifact.ID.String()+"/content", nil)
	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, res.Code, res.Body.String())
	}
	if got := res.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("expected content type application/json, got %q", got)
	}
	if res.Body.String() != "artifact:/workspace/reports/test.json\n" {
		t.Fatalf("unexpected artifact content: %q", res.Body.String())
	}
}

func TestArtifactCaptureRetainsWorkspaceContent(t *testing.T) {
	store := newFakeStore()
	api := NewWithRuntimeAccess(store, &fakeRuntimeAccess{})
	sandbox := store.mustRunningSandbox(t)
	artifact, err := store.CreateArtifact(context.Background(), domain.ArtifactCreate{
		ProjectID:   sandbox.ProjectID,
		SandboxID:   sandbox.ID,
		Kind:        domain.ArtifactKindReport,
		Name:        "Retained report",
		URI:         "workspace:///workspace/reports/retained.json",
		ContentType: "application/json",
	})
	if err != nil {
		t.Fatal(err)
	}

	captureRes := request(api, http.MethodPost, "/v1/artifacts/"+artifact.ID.String()+"/capture", nil)
	if captureRes.Code != http.StatusOK {
		t.Fatalf("expected capture status %d, got %d: %s", http.StatusOK, captureRes.Code, captureRes.Body.String())
	}
	var captured domain.Artifact
	decodeResponse(t, captureRes, &captured)
	if captured.RetainedContent == nil {
		t.Fatalf("expected retained content metadata: %+v", captured)
	}
	expectedBody := "artifact:/workspace/reports/retained.json\n"
	expectedHash := sha256.Sum256([]byte(expectedBody))
	if captured.RetainedContent.SizeBytes != int64(len(expectedBody)) {
		t.Fatalf("unexpected retained size: %+v", captured.RetainedContent)
	}
	if captured.RetainedContent.SHA256 != hex.EncodeToString(expectedHash[:]) {
		t.Fatalf("unexpected retained hash: %+v", captured.RetainedContent)
	}

	sandbox.Status = domain.SandboxStatusStopped
	store.sandboxes[sandbox.ID] = sandbox
	contentRes := request(api, http.MethodGet, "/v1/artifacts/"+artifact.ID.String()+"/content", nil)
	if contentRes.Code != http.StatusOK {
		t.Fatalf("expected retained content status %d, got %d: %s", http.StatusOK, contentRes.Code, contentRes.Body.String())
	}
	if contentRes.Body.String() != expectedBody {
		t.Fatalf("unexpected retained content: %q", contentRes.Body.String())
	}
	if contentRes.Header().Get("X-Mbox-Artifact-Retained") != "true" {
		t.Fatalf("expected retained content header, got %q", contentRes.Header().Get("X-Mbox-Artifact-Retained"))
	}
}

func TestArtifactUploadRetainsClientContent(t *testing.T) {
	store := newFakeStore()
	api := New(store)
	sandbox := store.mustRunningSandbox(t)
	artifact, err := store.CreateArtifact(context.Background(), domain.ArtifactCreate{
		ProjectID:   sandbox.ProjectID,
		SandboxID:   sandbox.ID,
		Kind:        domain.ArtifactKindReport,
		Name:        "Client report",
		URI:         "client://reports/client.json",
		ContentType: "application/json",
	})
	if err != nil {
		t.Fatal(err)
	}

	body := []byte(`{"source":"client"}`)
	req := httptest.NewRequest(http.MethodPut, "/v1/artifacts/"+artifact.ID.String()+"/content", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mbox-Artifact-Source-URI", "client://reports/client.json")
	res := httptest.NewRecorder()
	api.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected upload status %d, got %d: %s", http.StatusOK, res.Code, res.Body.String())
	}
	var uploaded domain.Artifact
	decodeResponse(t, res, &uploaded)
	if uploaded.RetainedContent == nil {
		t.Fatalf("expected retained content metadata: %+v", uploaded)
	}
	expectedHash := sha256.Sum256(body)
	if uploaded.RetainedContent.SizeBytes != int64(len(body)) {
		t.Fatalf("unexpected retained size: %+v", uploaded.RetainedContent)
	}
	if uploaded.RetainedContent.SHA256 != hex.EncodeToString(expectedHash[:]) {
		t.Fatalf("unexpected retained hash: %+v", uploaded.RetainedContent)
	}
	if uploaded.RetainedContent.SourceURI != "client://reports/client.json" {
		t.Fatalf("unexpected source URI: %+v", uploaded.RetainedContent)
	}

	contentRes := request(api, http.MethodGet, "/v1/artifacts/"+artifact.ID.String()+"/content", nil)
	if contentRes.Code != http.StatusOK {
		t.Fatalf("expected retained content status %d, got %d: %s", http.StatusOK, contentRes.Code, contentRes.Body.String())
	}
	if contentRes.Body.String() != string(body) {
		t.Fatalf("unexpected retained content: %q", contentRes.Body.String())
	}
	if contentRes.Header().Get("X-Mbox-Artifact-Retained") != "true" {
		t.Fatalf("expected retained content header, got %q", contentRes.Header().Get("X-Mbox-Artifact-Retained"))
	}
}

func TestArtifactUploadRejectsTooLargeContent(t *testing.T) {
	store := newFakeStore()
	api := New(store)
	sandbox := store.mustRunningSandbox(t)
	artifact, err := store.CreateArtifact(context.Background(), domain.ArtifactCreate{
		ProjectID: sandbox.ProjectID,
		SandboxID: sandbox.ID,
		Kind:      domain.ArtifactKindReport,
		Name:      "Huge report",
		URI:       "client://reports/huge.txt",
	})
	if err != nil {
		t.Fatal(err)
	}

	body := io.LimitReader(zeroReader{}, maxArtifactContentBytes+1)
	req := httptest.NewRequest(http.MethodPut, "/v1/artifacts/"+artifact.ID.String()+"/content", body)
	res := httptest.NewRecorder()
	api.ServeHTTP(res, req)
	if res.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected upload status %d, got %d: %s", http.StatusRequestEntityTooLarge, res.Code, res.Body.String())
	}
}

func TestArtifactUploadRejectsDirectory(t *testing.T) {
	store := newFakeStore()
	api := New(store)
	sandbox := store.mustRunningSandbox(t)
	artifact, err := store.CreateArtifact(context.Background(), domain.ArtifactCreate{
		ProjectID: sandbox.ProjectID,
		SandboxID: sandbox.ID,
		Kind:      domain.ArtifactKindDirectory,
		Name:      "Output directory",
		URI:       "client://reports",
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPut, "/v1/artifacts/"+artifact.ID.String()+"/content", strings.NewReader("nope"))
	res := httptest.NewRecorder()
	api.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected upload status %d, got %d: %s", http.StatusBadRequest, res.Code, res.Body.String())
	}
}

func TestArtifactCaptureUsesFilesystemBackend(t *testing.T) {
	store := newFakeStore()
	backend, err := NewFilesystemArtifactContentBackend(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	api := NewWithOptions(store, Options{
		RuntimeAccess:          &fakeRuntimeAccess{},
		ArtifactContentBackend: backend,
	})
	sandbox := store.mustRunningSandbox(t)
	artifact, err := store.CreateArtifact(context.Background(), domain.ArtifactCreate{
		ProjectID:   sandbox.ProjectID,
		SandboxID:   sandbox.ID,
		Kind:        domain.ArtifactKindReport,
		Name:        "Filesystem report",
		URI:         "workspace:///workspace/reports/filesystem.txt",
		ContentType: "text/plain",
	})
	if err != nil {
		t.Fatal(err)
	}

	captureRes := request(api, http.MethodPost, "/v1/artifacts/"+artifact.ID.String()+"/capture", nil)
	if captureRes.Code != http.StatusOK {
		t.Fatalf("expected capture status %d, got %d: %s", http.StatusOK, captureRes.Code, captureRes.Body.String())
	}
	var captured domain.Artifact
	decodeResponse(t, captureRes, &captured)
	if captured.RetainedContent == nil {
		t.Fatalf("expected retained content metadata: %+v", captured)
	}
	if captured.RetainedContent.StorageProvider != domain.ArtifactContentStorageProviderFilesystem {
		t.Fatalf("unexpected storage provider: %+v", captured.RetainedContent)
	}
	if captured.RetainedContent.StorageKey == "" {
		t.Fatalf("expected storage key: %+v", captured.RetainedContent)
	}
	if stored := store.artifactContents[artifact.ID]; stored.Content != nil {
		t.Fatalf("expected filesystem content to keep bytes out of store, got %d bytes", len(stored.Content))
	}

	sandbox.Status = domain.SandboxStatusStopped
	store.sandboxes[sandbox.ID] = sandbox
	contentRes := request(api, http.MethodGet, "/v1/artifacts/"+artifact.ID.String()+"/content", nil)
	if contentRes.Code != http.StatusOK {
		t.Fatalf("expected retained content status %d, got %d: %s", http.StatusOK, contentRes.Code, contentRes.Body.String())
	}
	if contentRes.Body.String() != "artifact:/workspace/reports/filesystem.txt\n" {
		t.Fatalf("unexpected retained content: %q", contentRes.Body.String())
	}
	if contentRes.Header().Get("X-Mbox-Artifact-Retained") != "true" {
		t.Fatalf("expected retained content header, got %q", contentRes.Header().Get("X-Mbox-Artifact-Retained"))
	}
}

func TestFilesystemArtifactContentBackendRejectsEscapingKey(t *testing.T) {
	backend, err := NewFilesystemArtifactContentBackend(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, err = backend.Read(context.Background(), domain.ArtifactContent{
		StorageProvider: domain.ArtifactContentStorageProviderFilesystem,
		StorageKey:      "../escape",
	})
	if err == nil {
		t.Fatal("expected escaping storage key to fail")
	}
}

func TestFilesystemArtifactContentBackendDetectsMissingFile(t *testing.T) {
	dir := t.TempDir()
	backend, err := NewFilesystemArtifactContentBackend(dir)
	if err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "artifact", "missing")
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("unexpected missing file state: %v", err)
	}
	_, err = backend.Read(context.Background(), domain.ArtifactContent{
		StorageProvider: domain.ArtifactContentStorageProviderFilesystem,
		StorageKey:      "artifact/missing",
	})
	if err != domain.ErrNotFound {
		t.Fatalf("expected domain.ErrNotFound, got %v", err)
	}
}

func TestS3ArtifactContentBackendCaptureAndRead(t *testing.T) {
	objects := map[string][]byte{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" || r.Header.Get("X-Amz-Date") == "" {
			t.Fatalf("expected signed S3 request headers, got %#v", r.Header)
		}
		switch r.Method {
		case http.MethodPut:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			objects[r.URL.Path] = body
			w.WriteHeader(http.StatusNoContent)
		case http.MethodGet:
			body, ok := objects[r.URL.Path]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write(body)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	backend, err := NewS3ArtifactContentBackend(S3ArtifactContentBackendOptions{
		Endpoint:        server.URL,
		Region:          "us-east-1",
		Bucket:          "artifacts",
		Prefix:          "mbox",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient:      server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("s3 retained")
	hash := sha256.Sum256(content)
	expectedPath := "/artifacts/mbox/00000000-0000-0000-0000-000000000001/" + hex.EncodeToString(hash[:])
	objects[expectedPath] = nil
	input := domain.ArtifactContentCapture{
		ArtifactID:  uuid.New(),
		ContentType: "text/plain",
		SizeBytes:   int64(len(content)),
		SHA256:      hex.EncodeToString(hash[:]),
		SourceURI:   "client://report.txt",
	}
	captured, err := backend.Capture(context.Background(), domain.Artifact{ID: uuid.MustParse("00000000-0000-0000-0000-000000000001")}, input, content)
	if err != nil {
		t.Fatal(err)
	}
	if captured.StorageProvider != domain.ArtifactContentStorageProviderS3 ||
		captured.StorageKey != "mbox/00000000-0000-0000-0000-000000000001/"+input.SHA256 ||
		captured.Content != nil {
		t.Fatalf("unexpected capture metadata: %+v", captured)
	}
	if string(objects[expectedPath]) != string(content) {
		t.Fatalf("expected stored object at %s, got %#v", expectedPath, objects)
	}
	read, err := backend.Read(context.Background(), domain.ArtifactContent{
		StorageProvider: captured.StorageProvider,
		StorageKey:      captured.StorageKey,
		SizeBytes:       int64(len(content)),
		SHA256:          input.SHA256,
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(read) != string(content) {
		t.Fatalf("unexpected content %q", string(read))
	}
}

func TestS3ArtifactContentBackendDetectsMissingObject(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	backend, err := NewS3ArtifactContentBackend(S3ArtifactContentBackendOptions{
		Endpoint:        server.URL,
		Bucket:          "artifacts",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		ForcePathStyle:  true,
		HTTPClient:      server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = backend.Read(context.Background(), domain.ArtifactContent{
		StorageProvider: domain.ArtifactContentStorageProviderS3,
		StorageKey:      "missing/object",
	})
	if err != domain.ErrNotFound {
		t.Fatalf("expected domain.ErrNotFound, got %v", err)
	}
}

func TestS3ArtifactContentBackendRejectsInvalidEndpoint(t *testing.T) {
	_, err := NewS3ArtifactContentBackend(S3ArtifactContentBackendOptions{
		Endpoint:        "ftp://example.com",
		Bucket:          "artifacts",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
	})
	if err == nil {
		t.Fatal("expected invalid endpoint to fail")
	}
}

func TestArtifactCaptureRejectsNonWorkspaceReference(t *testing.T) {
	store := newFakeStore()
	api := NewWithRuntimeAccess(store, &fakeRuntimeAccess{})
	sandbox := store.mustRunningSandbox(t)
	artifact, err := store.CreateArtifact(context.Background(), domain.ArtifactCreate{
		ProjectID: sandbox.ProjectID,
		SandboxID: sandbox.ID,
		Kind:      domain.ArtifactKindLink,
		Name:      "External report",
		URI:       "https://example.com/report.json",
	})
	if err != nil {
		t.Fatal(err)
	}

	res := request(api, http.MethodPost, "/v1/artifacts/"+artifact.ID.String()+"/capture", nil)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, res.Code, res.Body.String())
	}
}

func TestArtifactContentRejectsNonWorkspaceReference(t *testing.T) {
	store := newFakeStore()
	api := NewWithRuntimeAccess(store, &fakeRuntimeAccess{})
	sandbox := store.mustRunningSandbox(t)
	artifact, err := store.CreateArtifact(context.Background(), domain.ArtifactCreate{
		ProjectID: sandbox.ProjectID,
		SandboxID: sandbox.ID,
		Kind:      domain.ArtifactKindLink,
		Name:      "External report",
		URI:       "https://example.com/report.json",
	})
	if err != nil {
		t.Fatal(err)
	}

	res := request(api, http.MethodGet, "/v1/artifacts/"+artifact.ID.String()+"/content", nil)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, res.Code, res.Body.String())
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

func TestRuntimeSessionCreateListAndEnd(t *testing.T) {
	store := newFakeStore()
	api := NewWithRuntimeAccess(store, &fakeRuntimeAccess{})
	sandbox := store.mustRunningSandbox(t)

	createRes := request(api, http.MethodPost, "/v1/sandboxes/"+sandbox.ID.String()+"/sessions", map[string]any{
		"type":   "terminal",
		"client": "sdk-smoke",
		"metadata": map[string]any{
			"purpose": "audit",
		},
	})
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected create status %d, got %d: %s", http.StatusCreated, createRes.Code, createRes.Body.String())
	}
	var session domain.RuntimeSession
	decodeResponse(t, createRes, &session)
	if session.ID == uuid.Nil || session.Type != domain.RuntimeSessionTypeTerminal || session.Status != domain.RuntimeSessionStatusActive {
		t.Fatalf("unexpected session response: %+v", session)
	}
	if session.ProjectID != sandbox.ProjectID || session.SandboxID != sandbox.ID || session.RuntimeRef == nil {
		t.Fatalf("expected session to inherit sandbox runtime context, got %+v", session)
	}

	listRes := request(api, http.MethodGet, "/v1/sandboxes/"+sandbox.ID.String()+"/sessions", nil)
	if listRes.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d: %s", http.StatusOK, listRes.Code, listRes.Body.String())
	}
	var sessions ListResponse[domain.RuntimeSession]
	decodeResponse(t, listRes, &sessions)
	if len(sessions.Items) != 1 || sessions.Items[0].ID != session.ID {
		t.Fatalf("unexpected session list: %+v", sessions)
	}

	endRes := request(api, http.MethodPost, "/v1/sessions/"+session.ID.String()+"/end", nil)
	if endRes.Code != http.StatusOK {
		t.Fatalf("expected end status %d, got %d: %s", http.StatusOK, endRes.Code, endRes.Body.String())
	}
	var ended domain.RuntimeSession
	decodeResponse(t, endRes, &ended)
	if ended.Status != domain.RuntimeSessionStatusEnded || ended.EndedAt == nil {
		t.Fatalf("expected ended session, got %+v", ended)
	}
}

func TestRuntimeSessionRejectsInvalidType(t *testing.T) {
	store := newFakeStore()
	api := NewWithRuntimeAccess(store, &fakeRuntimeAccess{})
	sandbox := store.mustRunningSandbox(t)

	res := request(api, http.MethodPost, "/v1/sandboxes/"+sandbox.ID.String()+"/sessions", map[string]any{
		"type":   "agent-brain",
		"client": "sdk-smoke",
	})
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, res.Code, res.Body.String())
	}
}

func TestTerminalRecordsRuntimeSession(t *testing.T) {
	store := newFakeStore()
	api := NewWithRuntimeAccess(store, &fakeRuntimeAccess{})
	server := httptest.NewServer(api)
	defer server.Close()
	sandbox := store.mustRunningSandbox(t)

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/v1/sandboxes/" + sandbox.ID.String() + "/terminal"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"User-Agent": {"mbox-test"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, []byte("exit\n")); err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()

	deadline := time.Now().Add(2 * time.Second)
	for {
		sessions, err := store.ListRuntimeSessions(context.Background(), sandbox.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(sessions) == 1 && sessions[0].EndedAt != nil {
			if sessions[0].Type != domain.RuntimeSessionTypeTerminal ||
				sessions[0].Status != domain.RuntimeSessionStatusEnded ||
				sessions[0].Client != "web-terminal" ||
				sessions[0].UserAgent != "mbox-test" {
				t.Fatalf("unexpected terminal session: %+v", sessions[0])
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for terminal session, got %+v", sessions)
		}
		time.Sleep(10 * time.Millisecond)
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

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

func request(handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	return requestWithHeaders(handler, method, path, body, nil)
}

func requestWithHeaders(handler http.Handler, method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
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
	mu               sync.Mutex
	projects         map[uuid.UUID]domain.Project
	templates        map[uuid.UUID]domain.EnvironmentTemplate
	policies         map[uuid.UUID]domain.ProjectPolicy
	quotaPolicies    map[uuid.UUID]domain.ProjectQuotaPolicy
	credentials      map[uuid.UUID]domain.ProjectCredential
	sandboxes        map[uuid.UUID]domain.Sandbox
	sessions         map[uuid.UUID]domain.RuntimeSession
	tasks            map[uuid.UUID]domain.ExecutionTask
	artifacts        map[uuid.UUID]domain.Artifact
	artifactContents map[uuid.UUID]domain.ArtifactContent
	auditEvents      map[uuid.UUID]domain.AuditEvent
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		projects:         map[uuid.UUID]domain.Project{},
		templates:        map[uuid.UUID]domain.EnvironmentTemplate{},
		policies:         map[uuid.UUID]domain.ProjectPolicy{},
		quotaPolicies:    map[uuid.UUID]domain.ProjectQuotaPolicy{},
		credentials:      map[uuid.UUID]domain.ProjectCredential{},
		sandboxes:        map[uuid.UUID]domain.Sandbox{},
		sessions:         map[uuid.UUID]domain.RuntimeSession{},
		tasks:            map[uuid.UUID]domain.ExecutionTask{},
		artifacts:        map[uuid.UUID]domain.Artifact{},
		artifactContents: map[uuid.UUID]domain.ArtifactContent{},
		auditEvents:      map[uuid.UUID]domain.AuditEvent{},
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
	for _, sandbox := range s.sandboxes {
		if sandbox.ProjectID == id && (sandbox.DeletedAt == nil || sandbox.RuntimeRef != nil) {
			return domain.ErrConflict
		}
	}
	delete(s.projects, id)
	return nil
}

func (s *fakeStore) GetProjectPolicy(_ context.Context, projectID uuid.UUID) (domain.ProjectPolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	policy, ok := s.policies[projectID]
	if !ok {
		return domain.ProjectPolicy{}, domain.ErrNotFound
	}
	return policy, nil
}

func (s *fakeStore) UpsertProjectPolicy(_ context.Context, projectID uuid.UUID, input domain.ProjectPolicyUpsert) (domain.ProjectPolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.projects[projectID]; !ok {
		return domain.ProjectPolicy{}, domain.ErrNotFound
	}
	now := time.Now()
	policy, ok := s.policies[projectID]
	if !ok {
		policy = domain.ProjectPolicy{
			ProjectID: projectID,
			CreatedAt: now,
		}
	}
	policy.Enforcement = input.Enforcement
	policy.AllowedImagePrefixes = append([]string{}, input.AllowedImagePrefixes...)
	policy.AllowedServiceAccounts = append([]string{}, input.AllowedServiceAccounts...)
	policy.AllowedSecretRefs = append([]string{}, input.AllowedSecretRefs...)
	policy.UpdatedAt = now
	s.policies[projectID] = policy
	return policy, nil
}

func (s *fakeStore) GetProjectQuotaPolicy(_ context.Context, projectID uuid.UUID) (domain.ProjectQuotaPolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	policy, ok := s.quotaPolicies[projectID]
	if !ok {
		return domain.ProjectQuotaPolicy{}, domain.ErrNotFound
	}
	return policy, nil
}

func (s *fakeStore) UpsertProjectQuotaPolicy(_ context.Context, projectID uuid.UUID, input domain.ProjectQuotaPolicyUpsert) (domain.ProjectQuotaPolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.projects[projectID]; !ok {
		return domain.ProjectQuotaPolicy{}, domain.ErrNotFound
	}
	now := time.Now()
	policy, ok := s.quotaPolicies[projectID]
	if !ok {
		policy = domain.ProjectQuotaPolicy{
			ProjectID: projectID,
			CreatedAt: now,
		}
	}
	policy.Enforcement = input.Enforcement
	policy.MaxActiveSandboxes = input.MaxActiveSandboxes
	policy.MaxRetainedArtifactBytes = input.MaxRetainedArtifactBytes
	policy.UpdatedAt = now
	s.quotaPolicies[projectID] = policy
	return policy, nil
}

func (s *fakeStore) ListProjectCredentials(_ context.Context, projectID uuid.UUID) ([]domain.ProjectCredential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.projects[projectID]; !ok {
		return nil, domain.ErrNotFound
	}
	items := []domain.ProjectCredential{}
	for _, credential := range s.credentials {
		if credential.ProjectID == projectID {
			items = append(items, credential)
		}
	}
	return items, nil
}

func (s *fakeStore) CreateProjectCredential(_ context.Context, input domain.ProjectCredentialCreate) (domain.ProjectCredential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.projects[input.ProjectID]; !ok {
		return domain.ProjectCredential{}, domain.ErrNotFound
	}
	for _, credential := range s.credentials {
		if credential.ProjectID == input.ProjectID && credential.Slug == input.Slug {
			return domain.ProjectCredential{}, domain.ErrConflict
		}
	}
	now := time.Now()
	credential := domain.ProjectCredential{
		ID:        uuid.New(),
		ProjectID: input.ProjectID,
		Name:      input.Name,
		Slug:      input.Slug,
		Type:      input.Type,
		Target:    input.Target,
		SecretRef: input.SecretRef,
		Usage:     append([]string{}, input.Usage...),
		Metadata:  input.Metadata,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.credentials[credential.ID] = credential
	return credential, nil
}

func (s *fakeStore) GetProjectCredential(_ context.Context, id uuid.UUID) (domain.ProjectCredential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	credential, ok := s.credentials[id]
	if !ok {
		return domain.ProjectCredential{}, domain.ErrNotFound
	}
	return credential, nil
}

func (s *fakeStore) DeleteProjectCredential(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.credentials[id]; !ok {
		return domain.ErrNotFound
	}
	delete(s.credentials, id)
	return nil
}

func (s *fakeStore) GetProjectUsage(_ context.Context, projectID uuid.UUID) (domain.ProjectUsage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.projects[projectID]; !ok {
		return domain.ProjectUsage{}, domain.ErrNotFound
	}
	usage := domain.ProjectUsage{
		ProjectID:   projectID,
		GeneratedAt: time.Now().UTC(),
		Notes: []string{
			"usage is aggregated from mbox product records",
			"sandbox resource requests are summed from active sandbox templates, not live Kubernetes metrics",
			"quota enforcement is not implemented by this usage summary",
		},
	}
	cpu := map[string]int{}
	memory := map[string]int{}
	storage := map[string]int{}
	for _, template := range s.templates {
		if template.ProjectID == nil {
			usage.Templates.GlobalVisible++
		} else if *template.ProjectID == projectID {
			usage.Templates.ProjectScoped++
		} else {
			continue
		}
		addResourceUsage(cpu, template.CPURequest)
		addResourceUsage(memory, template.MemoryRequest)
		addResourceUsage(storage, template.StorageRequest)
	}
	usage.Templates.CPURequests = usageValuesFromMap(cpu)
	usage.Templates.MemoryRequests = usageValuesFromMap(memory)
	usage.Templates.StorageRequests = usageValuesFromMap(storage)
	activeRequests := domain.NewSandboxResourceRequestAccumulator()
	runningRequests := domain.NewSandboxResourceRequestAccumulator()
	for _, sandbox := range s.sandboxes {
		if sandbox.ProjectID != projectID {
			continue
		}
		usage.Sandboxes.Total++
		if sandbox.DeletedAt == nil {
			usage.Sandboxes.Active++
			if template, ok := s.templates[sandbox.TemplateID]; ok {
				activeRequests.Add(template.CPURequest, template.MemoryRequest, template.StorageRequest)
				if sandbox.Status == domain.SandboxStatusRunning {
					runningRequests.Add(template.CPURequest, template.MemoryRequest, template.StorageRequest)
				}
			}
		}
		switch sandbox.Status {
		case domain.SandboxStatusPending:
			if sandbox.DeletedAt == nil {
				usage.Sandboxes.Pending++
			}
		case domain.SandboxStatusRunning:
			if sandbox.DeletedAt == nil {
				usage.Sandboxes.Running++
			}
		case domain.SandboxStatusStopped:
			if sandbox.DeletedAt == nil {
				usage.Sandboxes.Stopped++
			}
		case domain.SandboxStatusFailed:
			if sandbox.DeletedAt == nil {
				usage.Sandboxes.Failed++
			}
		case domain.SandboxStatusDeleted:
			usage.Sandboxes.Deleted++
		}
		if sandbox.DeletedAt != nil {
			if sandbox.Status != domain.SandboxStatusDeleted {
				usage.Sandboxes.Deleted++
			}
			if sandbox.RuntimeRef != nil {
				usage.Sandboxes.CleanupPending++
			}
		}
	}
	usage.Sandboxes.ActiveRequests = activeRequests.Usage()
	usage.Sandboxes.RunningRequests = runningRequests.Usage()
	for _, session := range s.sessions {
		if session.ProjectID != projectID {
			continue
		}
		usage.RuntimeSessions.Total++
		switch session.Status {
		case domain.RuntimeSessionStatusActive:
			usage.RuntimeSessions.Active++
		case domain.RuntimeSessionStatusEnded:
			usage.RuntimeSessions.Ended++
		case domain.RuntimeSessionStatusFailed:
			usage.RuntimeSessions.Failed++
		}
		switch session.Type {
		case domain.RuntimeSessionTypeTerminal:
			usage.RuntimeSessions.Terminal++
		case domain.RuntimeSessionTypeIDE:
			usage.RuntimeSessions.IDE++
		case domain.RuntimeSessionTypeNotebook:
			usage.RuntimeSessions.Notebook++
		case domain.RuntimeSessionTypeBrowser:
			usage.RuntimeSessions.Browser++
		case domain.RuntimeSessionTypeCommand:
			usage.RuntimeSessions.Command++
		case domain.RuntimeSessionTypeCustom:
			usage.RuntimeSessions.Custom++
		}
	}
	for _, task := range s.tasks {
		if task.ProjectID != projectID {
			continue
		}
		usage.ExecutionTasks.Total++
		switch task.Status {
		case domain.ExecutionTaskStatusQueued:
			usage.ExecutionTasks.Queued++
		case domain.ExecutionTaskStatusRunning:
			usage.ExecutionTasks.Running++
		case domain.ExecutionTaskStatusSucceeded:
			usage.ExecutionTasks.Succeeded++
		case domain.ExecutionTaskStatusFailed:
			usage.ExecutionTasks.Failed++
		case domain.ExecutionTaskStatusCanceled:
			usage.ExecutionTasks.Canceled++
		case domain.ExecutionTaskStatusTimedOut:
			usage.ExecutionTasks.TimedOut++
		}
	}
	for _, artifact := range s.artifacts {
		if artifact.ProjectID != projectID {
			continue
		}
		usage.Artifacts.Total++
		if artifact.SizeBytes != nil {
			usage.Artifacts.ReferencedBytes += *artifact.SizeBytes
		}
		if content, ok := s.artifactContents[artifact.ID]; ok {
			usage.Artifacts.RetainedContent++
			usage.Artifacts.RetainedBytes += content.SizeBytes
		}
		switch artifact.Kind {
		case domain.ArtifactKindFile:
			usage.Artifacts.File++
		case domain.ArtifactKindDirectory:
			usage.Artifacts.Directory++
		case domain.ArtifactKindLog:
			usage.Artifacts.Log++
		case domain.ArtifactKindReport:
			usage.Artifacts.Report++
		case domain.ArtifactKindScreenshot:
			usage.Artifacts.Screenshot++
		case domain.ArtifactKindImage:
			usage.Artifacts.Image++
		case domain.ArtifactKindLink:
			usage.Artifacts.Link++
		case domain.ArtifactKindOther:
			usage.Artifacts.Other++
		}
	}
	for _, credential := range s.credentials {
		if credential.ProjectID != projectID {
			continue
		}
		usage.Credentials.Total++
		switch credential.Type {
		case domain.ProjectCredentialTypeGit:
			usage.Credentials.Git++
		case domain.ProjectCredentialTypeRegistry:
			usage.Credentials.Registry++
		case domain.ProjectCredentialTypeKubernetes:
			usage.Credentials.Kubernetes++
		case domain.ProjectCredentialTypeSSH:
			usage.Credentials.SSH++
		case domain.ProjectCredentialTypeGeneric:
			usage.Credentials.Generic++
		}
	}
	return usage, nil
}

func (s *fakeStore) ListAuditEvents(_ context.Context, filter domain.AuditEventFilter) ([]domain.AuditEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	items := []domain.AuditEvent{}
	for _, event := range s.auditEvents {
		if filter.ProjectID != nil && (event.ProjectID == nil || *event.ProjectID != *filter.ProjectID) {
			continue
		}
		if filter.Action != "" && event.Action != filter.Action {
			continue
		}
		if filter.ResourceType != "" && event.ResourceType != filter.ResourceType {
			continue
		}
		if filter.ResourceID != nil && (event.ResourceID == nil || *event.ResourceID != *filter.ResourceID) {
			continue
		}
		if filter.Actor != "" && event.Actor != filter.Actor {
			continue
		}
		if filter.Source != "" && event.Source != filter.Source {
			continue
		}
		if filter.RequestID != "" {
			var metadata map[string]any
			if json.Unmarshal(event.Metadata, &metadata) != nil || fmt.Sprint(metadata["requestId"]) != filter.RequestID {
				continue
			}
		}
		if filter.Operation != "" {
			var metadata map[string]any
			if json.Unmarshal(event.Metadata, &metadata) != nil || fmt.Sprint(metadata["operation"]) != filter.Operation {
				continue
			}
		}
		if filter.Since != nil && event.CreatedAt.Before(*filter.Since) {
			continue
		}
		if filter.Until != nil && event.CreatedAt.After(*filter.Until) {
			continue
		}
		items = append(items, event)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (s *fakeStore) CreateAuditEvent(_ context.Context, input domain.AuditEventCreate) (domain.AuditEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	event := domain.AuditEvent{
		ID:           uuid.New(),
		ProjectID:    input.ProjectID,
		Action:       input.Action,
		ResourceType: input.ResourceType,
		ResourceID:   input.ResourceID,
		ResourceName: input.ResourceName,
		Actor:        input.Actor,
		Source:       input.Source,
		Metadata:     input.Metadata,
		CreatedAt:    time.Now(),
	}
	s.auditEvents[event.ID] = event
	return event, nil
}

func addResourceUsage(values map[string]int, value string) {
	value = strings.TrimSpace(value)
	if value != "" {
		values[value]++
	}
}

func usageValuesFromMap(values map[string]int) []domain.ResourceUsageValue {
	out := make([]domain.ResourceUsageValue, 0, len(values))
	for value, count := range values {
		out = append(out, domain.ResourceUsageValue{Value: value, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Value < out[j].Value
		}
		return out[i].Count > out[j].Count
	})
	return out
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

func (s *fakeStore) ListAllTemplates(_ context.Context) ([]domain.EnvironmentTemplate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := []domain.EnvironmentTemplate{}
	for _, template := range s.templates {
		items = append(items, template)
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

func (s *fakeStore) ListAllSandboxes(_ context.Context) ([]domain.Sandbox, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := []domain.Sandbox{}
	for _, sandbox := range s.sandboxes {
		items = append(items, sandbox)
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

func (s *fakeStore) ListRuntimeSessions(_ context.Context, sandboxID uuid.UUID) ([]domain.RuntimeSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := []domain.RuntimeSession{}
	for _, session := range s.sessions {
		if session.SandboxID == sandboxID {
			items = append(items, session)
		}
	}
	return items, nil
}

func (s *fakeStore) CreateRuntimeSession(_ context.Context, input domain.RuntimeSessionCreate) (domain.RuntimeSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if _, ok := s.projects[input.ProjectID]; !ok {
		return domain.RuntimeSession{}, domain.ErrNotFound
	}
	if _, ok := s.sandboxes[input.SandboxID]; !ok {
		return domain.RuntimeSession{}, domain.ErrNotFound
	}
	session := domain.RuntimeSession{
		ID:         uuid.New(),
		ProjectID:  input.ProjectID,
		SandboxID:  input.SandboxID,
		Type:       input.Type,
		Status:     domain.RuntimeSessionStatusActive,
		Client:     input.Client,
		UserAgent:  input.UserAgent,
		RuntimeRef: input.RuntimeRef,
		Metadata:   input.Metadata,
		StartedAt:  now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	s.sessions[session.ID] = session
	return session, nil
}

func (s *fakeStore) GetRuntimeSession(_ context.Context, id uuid.UUID) (domain.RuntimeSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[id]
	if !ok {
		return domain.RuntimeSession{}, domain.ErrNotFound
	}
	return session, nil
}

func (s *fakeStore) UpdateRuntimeSession(_ context.Context, id uuid.UUID, input domain.RuntimeSessionUpdate) (domain.RuntimeSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[id]
	if !ok {
		return domain.RuntimeSession{}, domain.ErrNotFound
	}
	if input.Status != nil {
		session.Status = *input.Status
	}
	if input.EndedAt != nil {
		value := *input.EndedAt
		session.EndedAt = &value
	}
	session.UpdatedAt = time.Now()
	s.sessions[id] = session
	return session, nil
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
	sandbox, ok := s.sandboxes[input.SandboxID]
	if !ok || sandbox.ProjectID != input.ProjectID || sandbox.DeletedAt != nil {
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
		if content, ok := s.artifactContents[artifact.ID]; ok {
			metadata := content
			metadata.Content = nil
			artifact.RetainedContent = &metadata
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
	sandbox, ok := s.sandboxes[input.SandboxID]
	if !ok || sandbox.ProjectID != input.ProjectID || sandbox.DeletedAt != nil {
		return domain.Artifact{}, domain.ErrNotFound
	}
	if input.TaskID != nil {
		task, ok := s.tasks[*input.TaskID]
		if !ok {
			return domain.Artifact{}, domain.ErrNotFound
		}
		if task.SandboxID != input.SandboxID || task.ProjectID != input.ProjectID {
			return domain.Artifact{}, domain.ErrNotFound
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
	if content, ok := s.artifactContents[id]; ok {
		metadata := content
		metadata.Content = nil
		artifact.RetainedContent = &metadata
	}
	return artifact, nil
}

func (s *fakeStore) CaptureArtifactContent(_ context.Context, input domain.ArtifactContentCapture) (domain.Artifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	artifact, ok := s.artifacts[input.ArtifactID]
	if !ok {
		return domain.Artifact{}, domain.ErrNotFound
	}
	var retainedBytes []byte
	if input.Content != nil {
		retainedBytes = append([]byte{}, input.Content...)
	}
	content := domain.ArtifactContent{
		ArtifactID:      input.ArtifactID,
		Content:         retainedBytes,
		ContentType:     input.ContentType,
		SizeBytes:       input.SizeBytes,
		SHA256:          input.SHA256,
		SourceURI:       input.SourceURI,
		StorageProvider: fakeStorageProviderOrDefault(input.StorageProvider),
		StorageKey:      input.StorageKey,
		CapturedAt:      time.Now(),
	}
	s.artifactContents[input.ArtifactID] = content
	metadata := content
	metadata.Content = nil
	artifact.RetainedContent = &metadata
	return artifact, nil
}

func (s *fakeStore) GetArtifactContent(_ context.Context, id uuid.UUID) (domain.ArtifactContent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	content, ok := s.artifactContents[id]
	if !ok {
		return domain.ArtifactContent{}, domain.ErrNotFound
	}
	if content.Content != nil {
		content.Content = append([]byte{}, content.Content...)
	}
	return content, nil
}

func fakeStorageProviderOrDefault(provider domain.ArtifactContentStorageProvider) domain.ArtifactContentStorageProvider {
	if provider == "" {
		return domain.ArtifactContentStorageProviderPostgres
	}
	return provider
}

type fakeRuntimeAccess struct {
	lastTailLines    int64
	lastPreviewPort  int
	lastPreviewPath  string
	lastPreviewQuery string
	lastExecCommand  []string
	execExitCode     int
}

type fakeRuntimeAuditor struct {
	resources []mboxruntime.ManagedResource
	err       error
}

func (f *fakeRuntimeAuditor) ListManagedResources(context.Context) (mboxruntime.ManagedResourceList, error) {
	if f.err != nil {
		return mboxruntime.ManagedResourceList{}, f.err
	}
	return mboxruntime.ManagedResourceList{
		Adapter:   "agent-sandbox",
		CheckedAt: time.Now(),
		Items:     f.resources,
	}, nil
}

type fakeRuntimeCleaner struct {
	deleted *mboxruntime.ManagedResourceRef
	err     error
}

func (f *fakeRuntimeCleaner) DeleteManagedResource(_ context.Context, ref mboxruntime.ManagedResourceRef) error {
	if f.err != nil {
		return f.err
	}
	value := ref
	f.deleted = &value
	return nil
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

func (f *fakeRuntimeAccess) ReadFile(_ context.Context, ref domain.RuntimeRef, request mboxruntime.FileReadRequest) (mboxruntime.FileReadResult, error) {
	target, _ := f.ResolveRuntime(context.Background(), ref)
	cleanPath := path.Clean("/" + strings.TrimSpace(request.Path))
	if !strings.HasPrefix(cleanPath, "/workspace/") && cleanPath != "/workspace" {
		return mboxruntime.FileReadResult{}, fmt.Errorf("artifact path must stay inside workspace mount /workspace")
	}
	body := "artifact:" + cleanPath + "\n"
	return mboxruntime.FileReadResult{
		Target:      target,
		Path:        cleanPath,
		ContentType: "text/plain",
		SizeBytes:   int64(len(body)),
		Body:        io.NopCloser(strings.NewReader(body)),
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

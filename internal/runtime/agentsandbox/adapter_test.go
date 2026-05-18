package agentsandbox

import (
	"context"
	"testing"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/mlhiter/mbox/internal/domain"
	mboxruntime "github.com/mlhiter/mbox/internal/runtime"
)

func TestCreateRuntimeCreatesTemplateAndClaim(t *testing.T) {
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		claimsGVR:    "SandboxClaimList",
		templatesGVR: "SandboxTemplateList",
	})
	coreClient := k8sfake.NewSimpleClientset()
	adapter := NewWithClients(client, coreClient, Config{WarmPoolPolicy: "none"})
	projectID := uuid.New()
	templateID := uuid.New()
	sandboxID := uuid.New()

	ref, err := adapter.CreateRuntime(context.Background(), mboxruntime.CreateRequest{
		Sandbox: domain.Sandbox{
			ID:                 sandboxID,
			ProjectID:          projectID,
			TemplateID:         templateID,
			Slug:               "dev-box",
			Namespace:          "mbox-demo",
			ServiceAccountName: "mbox-sandbox",
		},
		Template: domain.EnvironmentTemplate{
			ID:             templateID,
			Slug:           "ubuntu-terminal",
			Image:          "ubuntu:24.04",
			WorkingDir:     "/workspace",
			StorageRequest: "10Gi",
			ExposedPorts: []domain.TemplatePort{{
				Name:     "web",
				Port:     3000,
				Protocol: "TCP",
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ref.Adapter != "agent-sandbox" || ref.Kind != "SandboxClaim" || ref.Namespace != "mbox-demo" {
		t.Fatalf("unexpected runtime ref: %+v", ref)
	}

	if _, err := coreClient.CoreV1().Namespaces().Get(context.Background(), "mbox-demo", metav1.GetOptions{}); err != nil {
		t.Fatal(err)
	}
	serviceAccount, err := coreClient.CoreV1().ServiceAccounts("mbox-demo").Get(context.Background(), "mbox-sandbox", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if serviceAccount.AutomountServiceAccountToken == nil || *serviceAccount.AutomountServiceAccountToken {
		t.Fatalf("expected service account token automount to be disabled: %+v", serviceAccount.AutomountServiceAccountToken)
	}

	template, err := client.Resource(templatesGVR).Namespace("mbox-demo").Get(context.Background(), "ubuntu-terminal-"+templateID.String()[:8], metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	containers, _, _ := unstructured.NestedSlice(template.Object, "spec", "podTemplate", "spec", "containers")
	if len(containers) != 1 {
		t.Fatalf("expected one container, got %d", len(containers))
	}
	serviceAccountName, _, _ := unstructured.NestedString(template.Object, "spec", "podTemplate", "spec", "serviceAccountName")
	if serviceAccountName != "mbox-sandbox" {
		t.Fatalf("expected pod template service account mbox-sandbox, got %q", serviceAccountName)
	}
	tokenAutomount, _, _ := unstructured.NestedBool(template.Object, "spec", "podTemplate", "spec", "automountServiceAccountToken")
	if tokenAutomount {
		t.Fatal("expected pod template token automount to be disabled")
	}
	container := containers[0].(map[string]any)
	if container["image"] != "ubuntu:24.04" {
		t.Fatalf("unexpected container image: %+v", container)
	}
	if _, ok, _ := unstructured.NestedSlice(template.Object, "spec", "volumeClaimTemplates"); !ok {
		t.Fatal("expected volumeClaimTemplates")
	}

	claim, err := client.Resource(claimsGVR).Namespace("mbox-demo").Get(context.Background(), ref.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	templateRef, _, _ := unstructured.NestedString(claim.Object, "spec", "sandboxTemplateRef", "name")
	if templateRef != template.GetName() {
		t.Fatalf("expected claim template ref %q, got %q", template.GetName(), templateRef)
	}
	warmPool, _, _ := unstructured.NestedString(claim.Object, "spec", "warmpool")
	if warmPool != "none" {
		t.Fatalf("expected warm pool policy none, got %q", warmPool)
	}
}

func TestGetRuntimeStatusMapsReadyCondition(t *testing.T) {
	scheme := runtime.NewScheme()
	claim := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": apiVersion,
		"kind":       "SandboxClaim",
		"metadata": map[string]any{
			"name":      "claim",
			"namespace": "mbox-demo",
		},
		"status": map[string]any{
			"conditions": []any{
				map[string]any{
					"type":    "Ready",
					"status":  "True",
					"reason":  "SandboxReady",
					"message": "Sandbox is ready",
				},
			},
		},
	}}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		claimsGVR: "SandboxClaimList",
	}, claim)
	adapter := NewWithClient(client, Config{})

	status, err := adapter.GetRuntimeStatus(context.Background(), domain.RuntimeRef{
		Adapter:   "agent-sandbox",
		Kind:      "SandboxClaim",
		Namespace: "mbox-demo",
		Name:      "claim",
	})
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != mboxruntime.RuntimeStatusRunning {
		t.Fatalf("expected running, got %q", status.Status)
	}
}

package agentsandbox

import (
	"context"
	"testing"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

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
			CPURequest:     "250m",
			MemoryRequest:  "256Mi",
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
	resources := container["resources"].(map[string]any)
	requests := resources["requests"].(map[string]any)
	if requests["cpu"] != "250m" || requests["memory"] != "256Mi" {
		t.Fatalf("unexpected resource requests: %+v", requests)
	}
	if _, ok, _ := unstructured.NestedSlice(template.Object, "spec", "volumeClaimTemplates"); !ok {
		t.Fatal("expected volumeClaimTemplates")
	}
	volumeMounts := container["volumeMounts"].([]any)
	if len(volumeMounts) != 1 {
		t.Fatalf("expected workspace volume mount, got %+v", volumeMounts)
	}
	mount := volumeMounts[0].(map[string]any)
	if mount["name"] != "workspace" || mount["mountPath"] != "/workspace" {
		t.Fatalf("unexpected workspace volume mount: %+v", mount)
	}
	volumeClaimTemplates, _, _ := unstructured.NestedSlice(template.Object, "spec", "volumeClaimTemplates")
	claimTemplate := volumeClaimTemplates[0].(map[string]any)
	request, _, _ := unstructured.NestedString(claimTemplate, "spec", "resources", "requests", "storage")
	if request != "10Gi" {
		t.Fatalf("expected storage request 10Gi, got %q", request)
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
			"ports": []any{
				map[string]any{
					"name":       "web",
					"port":       int64(3000),
					"protocol":   "TCP",
					"previewUrl": "https://preview.example.com",
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
	if len(status.Ports) != 1 || status.Ports[0].Port != 3000 || status.Ports[0].PreviewURL == "" {
		t.Fatalf("expected runtime ports, got %+v", status.Ports)
	}
}

func TestStartStopRuntimeScalesResolvedSandbox(t *testing.T) {
	scheme := runtime.NewScheme()
	claim := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": apiVersion,
		"kind":       "SandboxClaim",
		"metadata": map[string]any{
			"name":      "claim",
			"namespace": "mbox-demo",
		},
		"status": map[string]any{
			"sandbox": map[string]any{
				"name": "resolved-sandbox",
			},
		},
	}}
	runtimeSandbox := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "agents.x-k8s.io/v1alpha1",
		"kind":       "Sandbox",
		"metadata": map[string]any{
			"name":      "resolved-sandbox",
			"namespace": "mbox-demo",
		},
		"spec": map[string]any{
			"replicas": int64(1),
		},
	}}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		claimsGVR:    "SandboxClaimList",
		sandboxesGVR: "SandboxList",
	}, claim)
	if _, err := client.Resource(sandboxesGVR).Namespace("mbox-demo").Create(context.Background(), runtimeSandbox, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	adapter := NewWithClient(client, Config{})
	ref := domain.RuntimeRef{
		Adapter:   "agent-sandbox",
		Kind:      "SandboxClaim",
		Namespace: "mbox-demo",
		Name:      "claim",
	}

	if err := adapter.StopRuntime(context.Background(), ref); err != nil {
		t.Fatal(err)
	}
	stopped, err := client.Resource(sandboxesGVR).Namespace("mbox-demo").Get(context.Background(), "resolved-sandbox", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	replicas, ok, _ := unstructured.NestedInt64(stopped.Object, "spec", "replicas")
	if !ok || replicas != 0 {
		t.Fatalf("expected stopped replicas 0, got %d ok=%v", replicas, ok)
	}
	updateCount := countActions(client.Actions(), "update", sandboxesGVR.Resource)
	if err := adapter.StopRuntime(context.Background(), ref); err != nil {
		t.Fatal(err)
	}
	if nextUpdateCount := countActions(client.Actions(), "update", sandboxesGVR.Resource); nextUpdateCount != updateCount {
		t.Fatalf("expected repeated stop to skip update, got %d updates before and %d after", updateCount, nextUpdateCount)
	}

	if err := adapter.StartRuntime(context.Background(), ref); err != nil {
		t.Fatal(err)
	}
	started, err := client.Resource(sandboxesGVR).Namespace("mbox-demo").Get(context.Background(), "resolved-sandbox", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	replicas, ok, _ = unstructured.NestedInt64(started.Object, "spec", "replicas")
	if !ok || replicas != 1 {
		t.Fatalf("expected started replicas 1, got %d ok=%v", replicas, ok)
	}
}

func countActions(actions []k8stesting.Action, verb string, resource string) int {
	count := 0
	for _, action := range actions {
		if action.GetVerb() == verb && action.GetResource().Resource == resource {
			count++
		}
	}
	return count
}

func TestStartStopRuntimeIgnoresUnresolvedClaim(t *testing.T) {
	scheme := runtime.NewScheme()
	claim := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": apiVersion,
		"kind":       "SandboxClaim",
		"metadata": map[string]any{
			"name":      "claim",
			"namespace": "mbox-demo",
		},
	}}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		claimsGVR:    "SandboxClaimList",
		sandboxesGVR: "SandboxList",
	}, claim)
	adapter := NewWithClient(client, Config{})
	ref := domain.RuntimeRef{
		Adapter:   "agent-sandbox",
		Kind:      "SandboxClaim",
		Namespace: "mbox-demo",
		Name:      "claim",
	}

	if err := adapter.StartRuntime(context.Background(), ref); err != nil {
		t.Fatal(err)
	}
	if err := adapter.StopRuntime(context.Background(), ref); err != nil {
		t.Fatal(err)
	}
}

func TestResolveRuntimeFindsPodFromClaimSandboxSelector(t *testing.T) {
	scheme := runtime.NewScheme()
	claim := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": apiVersion,
		"kind":       "SandboxClaim",
		"metadata": map[string]any{
			"name":      "claim",
			"namespace": "mbox-demo",
		},
		"status": map[string]any{
			"sandbox": map[string]any{
				"name": "resolved-sandbox",
			},
		},
	}}
	runtimeSandbox := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "agents.x-k8s.io/v1alpha1",
		"kind":       "Sandbox",
		"metadata": map[string]any{
			"name":      "resolved-sandbox",
			"namespace": "mbox-demo",
		},
		"status": map[string]any{
			"selector": "agents.x-k8s.io/sandbox=resolved-sandbox",
		},
	}}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		claimsGVR:    "SandboxClaimList",
		sandboxesGVR: "SandboxList",
	}, claim)
	if _, err := client.Resource(sandboxesGVR).Namespace("mbox-demo").Create(context.Background(), runtimeSandbox, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	coreClient := k8sfake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "runtime-pod",
			Namespace: "mbox-demo",
			Labels: map[string]string{
				"agents.x-k8s.io/sandbox": "resolved-sandbox",
			},
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{{
				Name: "workspace",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "workspace-resolved-sandbox",
					},
				},
			}},
			Containers: []corev1.Container{{
				Name: "workspace",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "workspace",
					MountPath: "/workspace",
				}},
			}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}, &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "workspace-resolved-sandbox",
			Namespace: "mbox-demo",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: stringPtr("standard"),
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimBound,
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("10Gi"),
			},
		},
	})
	adapter := NewWithClients(client, coreClient, Config{})

	target, err := adapter.ResolveRuntime(context.Background(), domain.RuntimeRef{
		Adapter:   "agent-sandbox",
		Kind:      "SandboxClaim",
		Namespace: "mbox-demo",
		Name:      "claim",
	})
	if err != nil {
		t.Fatal(err)
	}
	if target.PodName != "runtime-pod" || target.Container != "workspace" || target.Phase != "Running" {
		t.Fatalf("unexpected runtime target: %+v", target)
	}
	if len(target.Storage) != 1 {
		t.Fatalf("expected one storage mount, got %+v", target.Storage)
	}
	storage := target.Storage[0]
	if storage.MountPath != "/workspace" || storage.ClaimName != "workspace-resolved-sandbox" || storage.Phase != "Bound" || storage.Capacity != "10Gi" {
		t.Fatalf("unexpected runtime storage: %+v", storage)
	}
}

func stringPtr(value string) *string {
	return &value
}

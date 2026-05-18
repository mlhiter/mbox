package agentsandbox

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/mlhiter/mbox/internal/domain"
	mboxruntime "github.com/mlhiter/mbox/internal/runtime"
)

const (
	adapterName = "agent-sandbox"
	apiVersion  = "extensions.agents.x-k8s.io/v1alpha1"
)

var (
	claimsGVR = schema.GroupVersionResource{
		Group:    "extensions.agents.x-k8s.io",
		Version:  "v1alpha1",
		Resource: "sandboxclaims",
	}
	templatesGVR = schema.GroupVersionResource{
		Group:    "extensions.agents.x-k8s.io",
		Version:  "v1alpha1",
		Resource: "sandboxtemplates",
	}
)

type Adapter struct {
	client         dynamic.Interface
	coreClient     kubernetes.Interface
	warmPoolPolicy string
}

func New(restConfig *rest.Config, cfg Config) (*Adapter, error) {
	client, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	coreClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	return NewWithClients(client, coreClient, cfg), nil
}

func NewWithClient(client dynamic.Interface, cfg Config) *Adapter {
	return NewWithClients(client, nil, cfg)
}

func NewWithClients(client dynamic.Interface, coreClient kubernetes.Interface, cfg Config) *Adapter {
	return &Adapter{
		client:         client,
		coreClient:     coreClient,
		warmPoolPolicy: cfg.WarmPoolPolicy,
	}
}

func (a *Adapter) CreateRuntime(ctx context.Context, request mboxruntime.CreateRequest) (domain.RuntimeRef, error) {
	claimName := runtimeName(request.Sandbox.Slug, request.Sandbox.ID.String())
	templateName := runtimeName(request.Template.Slug, request.Template.ID.String())
	namespace := request.Sandbox.Namespace

	if err := a.ensureNamespace(ctx, namespace); err != nil {
		return domain.RuntimeRef{}, err
	}
	if err := a.ensureServiceAccount(ctx, namespace, request.Sandbox.ServiceAccountName); err != nil {
		return domain.RuntimeRef{}, err
	}
	if err := a.ensureTemplate(ctx, namespace, templateName, request.Template, request.Sandbox.ServiceAccountName); err != nil {
		return domain.RuntimeRef{}, err
	}

	claim := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": apiVersion,
		"kind":       "SandboxClaim",
		"metadata": map[string]any{
			"name":      claimName,
			"namespace": namespace,
			"labels": map[string]any{
				"app.kubernetes.io/name":       "mbox",
				"app.kubernetes.io/managed-by": "mbox",
				"mbox.dev/project-id":          request.Sandbox.ProjectID.String(),
				"mbox.dev/sandbox-id":          request.Sandbox.ID.String(),
			},
		},
		"spec": map[string]any{
			"sandboxTemplateRef": map[string]any{
				"name": templateName,
			},
		},
	}}
	if a.warmPoolPolicy != "" {
		_ = unstructured.SetNestedField(claim.Object, a.warmPoolPolicy, "spec", "warmpool")
	}

	_, err := a.client.Resource(claimsGVR).Namespace(namespace).Create(ctx, claim, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return runtimeRef(namespace, claimName), nil
	}
	if err != nil {
		return domain.RuntimeRef{}, err
	}
	return runtimeRef(namespace, claimName), nil
}

func (a *Adapter) DeleteRuntime(ctx context.Context, ref domain.RuntimeRef) error {
	if ref.Adapter != adapterName || ref.Kind != "SandboxClaim" {
		return fmt.Errorf("unsupported runtime ref %s/%s", ref.Adapter, ref.Kind)
	}
	err := a.client.Resource(claimsGVR).Namespace(ref.Namespace).Delete(ctx, ref.Name, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (a *Adapter) GetRuntimeStatus(ctx context.Context, ref domain.RuntimeRef) (mboxruntime.Status, error) {
	if ref.Adapter != adapterName || ref.Kind != "SandboxClaim" {
		return mboxruntime.Status{}, fmt.Errorf("unsupported runtime ref %s/%s", ref.Adapter, ref.Kind)
	}
	claim, err := a.client.Resource(claimsGVR).Namespace(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return mboxruntime.Status{
			Status:     mboxruntime.RuntimeStatusDeleted,
			RuntimeRef: ref,
		}, nil
	}
	if err != nil {
		return mboxruntime.Status{}, err
	}

	ready, message := readyCondition(claim)
	status := mboxruntime.RuntimeStatusPending
	if ready == corev1.ConditionTrue {
		status = mboxruntime.RuntimeStatusRunning
	}
	if ready == corev1.ConditionFalse && hasFailureReason(message) {
		status = mboxruntime.RuntimeStatusFailed
	}

	return mboxruntime.Status{
		Status:     status,
		RuntimeRef: ref,
		Message:    message,
	}, nil
}

func (a *Adapter) ensureTemplate(ctx context.Context, namespace string, name string, template domain.EnvironmentTemplate, serviceAccountName string) error {
	desired := buildTemplate(namespace, name, template, serviceAccountName)
	existing, err := a.client.Resource(templatesGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, createErr := a.client.Resource(templatesGVR).Namespace(namespace).Create(ctx, desired, metav1.CreateOptions{})
		return createErr
	}
	if err != nil {
		return err
	}

	desired.SetResourceVersion(existing.GetResourceVersion())
	_, err = a.client.Resource(templatesGVR).Namespace(namespace).Update(ctx, desired, metav1.UpdateOptions{})
	return err
}

func (a *Adapter) ensureNamespace(ctx context.Context, namespace string) error {
	if a.coreClient == nil {
		return nil
	}
	_, err := a.coreClient.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, createErr := a.coreClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
				Labels: map[string]string{
					"app.kubernetes.io/name":       "mbox",
					"app.kubernetes.io/managed-by": "mbox",
				},
			},
		}, metav1.CreateOptions{})
		return createErr
	}
	return err
}

func (a *Adapter) ensureServiceAccount(ctx context.Context, namespace string, name string) error {
	if a.coreClient == nil {
		return nil
	}
	if name == "" {
		return fmt.Errorf("service account name is required")
	}
	automount := false
	desired := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "mbox-sandbox",
				"app.kubernetes.io/managed-by": "mbox",
			},
		},
		AutomountServiceAccountToken: &automount,
	}
	existing, err := a.coreClient.CoreV1().ServiceAccounts(namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, createErr := a.coreClient.CoreV1().ServiceAccounts(namespace).Create(ctx, desired, metav1.CreateOptions{})
		return createErr
	}
	if err != nil {
		return err
	}

	updated := existing.DeepCopy()
	if updated.Labels == nil {
		updated.Labels = map[string]string{}
	}
	updated.Labels["app.kubernetes.io/name"] = "mbox-sandbox"
	updated.Labels["app.kubernetes.io/managed-by"] = "mbox"
	updated.AutomountServiceAccountToken = &automount
	_, err = a.coreClient.CoreV1().ServiceAccounts(namespace).Update(ctx, updated, metav1.UpdateOptions{})
	return err
}

func buildTemplate(namespace string, name string, template domain.EnvironmentTemplate, serviceAccountName string) *unstructured.Unstructured {
	container := map[string]any{
		"name":  "workspace",
		"image": template.Image,
	}
	if len(template.StartupCommand) > 0 {
		container["command"] = stringSliceToAny(template.StartupCommand)
	}
	if template.WorkingDir != "" {
		container["workingDir"] = template.WorkingDir
	}
	if len(template.ExposedPorts) > 0 {
		ports := make([]any, 0, len(template.ExposedPorts))
		for _, port := range template.ExposedPorts {
			protocol := port.Protocol
			if protocol == "" {
				protocol = "TCP"
			}
			ports = append(ports, map[string]any{
				"name":          port.Name,
				"containerPort": int64(port.Port),
				"protocol":      protocol,
			})
		}
		container["ports"] = ports
	}
	if template.StorageRequest != "" {
		container["volumeMounts"] = []any{
			map[string]any{
				"name":      "workspace",
				"mountPath": defaultString(template.WorkingDir, "/workspace"),
			},
		}
	}

	podSpec := map[string]any{
		"automountServiceAccountToken": false,
		"containers":                   []any{container},
		"serviceAccountName":           serviceAccountName,
	}

	spec := map[string]any{
		"podTemplate": map[string]any{
			"metadata": map[string]any{
				"labels": map[string]any{
					"app.kubernetes.io/name":       "mbox-sandbox",
					"app.kubernetes.io/managed-by": "mbox",
				},
			},
			"spec": podSpec,
		},
		"networkPolicyManagement": "Managed",
		"envVarsInjectionPolicy":  "Allowed",
	}
	if template.StorageRequest != "" {
		spec["volumeClaimTemplates"] = []any{
			map[string]any{
				"metadata": map[string]any{
					"name": "workspace",
				},
				"spec": map[string]any{
					"accessModes": []any{"ReadWriteOnce"},
					"resources": map[string]any{
						"requests": map[string]any{
							"storage": template.StorageRequest,
						},
					},
				},
			},
		}
	}

	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": apiVersion,
		"kind":       "SandboxTemplate",
		"metadata": map[string]any{
			"name":      name,
			"namespace": namespace,
			"labels": map[string]any{
				"app.kubernetes.io/name":       "mbox",
				"app.kubernetes.io/managed-by": "mbox",
				"mbox.dev/template-id":         template.ID.String(),
			},
		},
		"spec": spec,
	}}
}

func runtimeRef(namespace string, name string) domain.RuntimeRef {
	return domain.RuntimeRef{
		Adapter:   adapterName,
		Kind:      "SandboxClaim",
		Namespace: namespace,
		Name:      name,
	}
}

func runtimeName(slug string, id string) string {
	suffix := id
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	name := strings.Trim(strings.ToLower(slug), "-")
	name = strings.ReplaceAll(name, "_", "-")
	if name == "" {
		name = "sandbox"
	}
	if len(name) > 52 {
		name = strings.Trim(name[:52], "-")
	}
	return name + "-" + suffix
}

func stringSliceToAny(values []string) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

func defaultString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func readyCondition(obj *unstructured.Unstructured) (corev1.ConditionStatus, string) {
	conditions, ok, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if !ok {
		return corev1.ConditionUnknown, "Ready condition not reported"
	}
	for _, item := range conditions {
		condition, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if condition["type"] != "Ready" {
			continue
		}
		status, _ := condition["status"].(string)
		message, _ := condition["message"].(string)
		reason, _ := condition["reason"].(string)
		if message == "" {
			message = reason
		}
		return corev1.ConditionStatus(status), message
	}
	return corev1.ConditionUnknown, "Ready condition not reported"
}

func hasFailureReason(message string) bool {
	failureHints := []string{"failed", "error", "invalid", "not found", "expired"}
	lower := strings.ToLower(message)
	for _, hint := range failureHints {
		if strings.Contains(lower, hint) {
			return true
		}
	}
	return false
}

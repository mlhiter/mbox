package agentsandbox

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8snet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/mlhiter/mbox/internal/domain"
	mboxruntime "github.com/mlhiter/mbox/internal/runtime"
)

const (
	adapterName   = "agent-sandbox"
	apiVersion    = "extensions.agents.x-k8s.io/v1alpha1"
	containerName = "workspace"
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
	sandboxesGVR = schema.GroupVersionResource{
		Group:    "agents.x-k8s.io",
		Version:  "v1alpha1",
		Resource: "sandboxes",
	}
)

type Adapter struct {
	client         dynamic.Interface
	coreClient     kubernetes.Interface
	restConfig     *rest.Config
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
	return NewWithClients(client, coreClient, cfg).WithRESTConfig(restConfig), nil
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

func (a *Adapter) WithRESTConfig(restConfig *rest.Config) *Adapter {
	a.restConfig = restConfig
	return a
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
		Ports:      runtimePorts(claim),
		Message:    message,
	}, nil
}

func (a *Adapter) ResolveRuntime(ctx context.Context, ref domain.RuntimeRef) (mboxruntime.RuntimeTarget, error) {
	if ref.Adapter != adapterName || ref.Kind != "SandboxClaim" {
		return mboxruntime.RuntimeTarget{}, fmt.Errorf("unsupported runtime ref %s/%s", ref.Adapter, ref.Kind)
	}
	if a.coreClient == nil {
		return mboxruntime.RuntimeTarget{}, fmt.Errorf("kubernetes core client is not configured")
	}

	claim, err := a.client.Resource(claimsGVR).Namespace(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return mboxruntime.RuntimeTarget{}, err
	}
	sandboxName, _, _ := unstructured.NestedString(claim.Object, "status", "sandbox", "name")
	if sandboxName == "" {
		return mboxruntime.RuntimeTarget{}, fmt.Errorf("sandbox claim %s/%s has no resolved sandbox", ref.Namespace, ref.Name)
	}
	runtimeSandbox, err := a.client.Resource(sandboxesGVR).Namespace(ref.Namespace).Get(ctx, sandboxName, metav1.GetOptions{})
	if err != nil {
		return mboxruntime.RuntimeTarget{}, err
	}
	selector, _, _ := unstructured.NestedString(runtimeSandbox.Object, "status", "selector")
	if selector == "" {
		return mboxruntime.RuntimeTarget{}, fmt.Errorf("runtime sandbox %s/%s has no pod selector", ref.Namespace, sandboxName)
	}
	pods, err := a.coreClient.CoreV1().Pods(ref.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return mboxruntime.RuntimeTarget{}, err
	}
	if len(pods.Items) == 0 {
		return mboxruntime.RuntimeTarget{}, fmt.Errorf("no pod found for runtime sandbox selector %q", selector)
	}
	pod := pickRuntimePod(pods.Items)
	container := containerName
	if len(pod.Spec.Containers) > 0 {
		container = pod.Spec.Containers[0].Name
		for _, item := range pod.Spec.Containers {
			if item.Name == containerName {
				container = item.Name
				break
			}
		}
	}
	return mboxruntime.RuntimeTarget{
		Namespace: ref.Namespace,
		PodName:   pod.Name,
		Container: container,
		Phase:     string(pod.Status.Phase),
		Selector:  selector,
		Commands:  []string{"/bin/bash", "/bin/sh", "sh"},
		Storage:   a.runtimeStorage(ctx, pod, container),
	}, nil
}

func (a *Adapter) ReadLogs(ctx context.Context, ref domain.RuntimeRef, options mboxruntime.LogOptions) (mboxruntime.LogResult, error) {
	if a.coreClient == nil {
		return mboxruntime.LogResult{}, fmt.Errorf("kubernetes core client is not configured")
	}
	target, err := a.ResolveRuntime(ctx, ref)
	if err != nil {
		return mboxruntime.LogResult{}, err
	}
	container := defaultString(options.Container, target.Container)
	podOptions := &corev1.PodLogOptions{Container: container}
	if options.TailLines > 0 {
		podOptions.TailLines = &options.TailLines
	}
	stream, err := a.coreClient.CoreV1().Pods(target.Namespace).GetLogs(target.PodName, podOptions).Stream(ctx)
	if err != nil {
		return mboxruntime.LogResult{}, err
	}
	defer stream.Close()
	var out bytes.Buffer
	if _, err := io.Copy(&out, stream); err != nil {
		return mboxruntime.LogResult{}, err
	}
	target.Container = container
	return mboxruntime.LogResult{Target: target, Logs: out.String()}, nil
}

func (a *Adapter) ListEvents(ctx context.Context, ref domain.RuntimeRef) ([]mboxruntime.RuntimeEvent, error) {
	if a.coreClient == nil {
		return nil, fmt.Errorf("kubernetes core client is not configured")
	}
	target, err := a.ResolveRuntime(ctx, ref)
	if err != nil {
		return nil, err
	}
	selector := fields.OneTermEqualSelector("involvedObject.name", target.PodName).String()
	events, err := a.coreClient.CoreV1().Events(target.Namespace).List(ctx, metav1.ListOptions{FieldSelector: selector})
	if err != nil {
		return nil, err
	}
	items := make([]mboxruntime.RuntimeEvent, 0, len(events.Items))
	for _, event := range events.Items {
		items = append(items, mboxruntime.RuntimeEvent{
			Type:           event.Type,
			Reason:         event.Reason,
			Message:        event.Message,
			Count:          event.Count,
			FirstTimestamp: event.FirstTimestamp.Time,
			LastTimestamp:  event.LastTimestamp.Time,
		})
	}
	return items, nil
}

func (a *Adapter) ProxyPreview(ctx context.Context, ref domain.RuntimeRef, request mboxruntime.PreviewProxyRequest) (mboxruntime.PreviewProxyResponse, error) {
	if a.coreClient == nil || a.restConfig == nil {
		return mboxruntime.PreviewProxyResponse{}, fmt.Errorf("kubernetes preview proxy client is not configured")
	}
	target, err := a.ResolveRuntime(ctx, ref)
	if err != nil {
		return mboxruntime.PreviewProxyResponse{}, err
	}
	if request.Port < 1 || request.Port > 65535 {
		return mboxruntime.PreviewProxyResponse{}, fmt.Errorf("preview port must be between 1 and 65535")
	}
	proxyPath := strings.TrimPrefix(request.Path, "/")
	proxyRequest := a.coreClient.CoreV1().RESTClient().Get().
		Namespace(target.Namespace).
		Resource("pods").
		SubResource("proxy").
		Name(k8snet.JoinSchemeNamePort("http", target.PodName, fmt.Sprintf("%d", request.Port))).
		Suffix(proxyPath)
	for key, value := range parseProxyQuery(request.Query) {
		proxyRequest = proxyRequest.Param(key, value)
	}
	if err := proxyRequest.Error(); err != nil {
		return mboxruntime.PreviewProxyResponse{}, err
	}
	client, err := rest.HTTPClientFor(a.restConfig)
	if err != nil {
		return mboxruntime.PreviewProxyResponse{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, proxyRequest.URL().String(), nil)
	if err != nil {
		return mboxruntime.PreviewProxyResponse{}, err
	}
	response, err := client.Do(httpRequest)
	if err != nil {
		return mboxruntime.PreviewProxyResponse{}, err
	}
	return mboxruntime.PreviewProxyResponse{
		StatusCode: response.StatusCode,
		Header:     response.Header.Clone(),
		Body:       response.Body,
	}, nil
}

func (a *Adapter) Exec(ctx context.Context, ref domain.RuntimeRef, options mboxruntime.ExecOptions) error {
	if a.coreClient == nil || a.restConfig == nil {
		return fmt.Errorf("kubernetes exec client is not configured")
	}
	target, err := a.ResolveRuntime(ctx, ref)
	if err != nil {
		return err
	}
	command := options.Command
	if len(command) == 0 {
		command = []string{"/bin/sh"}
	}
	container := defaultString(options.Container, target.Container)
	req := a.coreClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(target.PodName).
		Namespace(target.Namespace).
		SubResource("exec")
	req.VersionedParams(&corev1.PodExecOptions{
		Container: container,
		Command:   command,
		Stdin:     options.Stdin != nil,
		Stdout:    options.Stdout != nil,
		Stderr:    options.Stderr != nil,
		TTY:       options.TTY,
	}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(a.restConfig, http.MethodPost, req.URL())
	if err != nil {
		return err
	}
	return executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  options.Stdin,
		Stdout: options.Stdout,
		Stderr: options.Stderr,
		Tty:    options.TTY,
	})
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
	if template.CPURequest != "" || template.MemoryRequest != "" {
		requests := map[string]any{}
		if template.CPURequest != "" {
			requests["cpu"] = template.CPURequest
		}
		if template.MemoryRequest != "" {
			requests["memory"] = template.MemoryRequest
		}
		container["resources"] = map[string]any{
			"requests": requests,
		}
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

func parseProxyQuery(raw string) map[string]string {
	if raw == "" {
		return nil
	}
	query, err := url.ParseQuery(raw)
	if err != nil {
		return nil
	}
	values := map[string]string{}
	for key, items := range query {
		if len(items) > 0 {
			values[key] = items[0]
		}
	}
	return values
}

func pickRuntimePod(pods []corev1.Pod) corev1.Pod {
	for _, pod := range pods {
		if pod.Status.Phase == corev1.PodRunning {
			return pod
		}
	}
	return pods[0]
}

func (a *Adapter) runtimeStorage(ctx context.Context, pod corev1.Pod, containerName string) []mboxruntime.RuntimeStorage {
	container, ok := findContainer(pod, containerName)
	if !ok {
		return nil
	}

	volumes := map[string]corev1.Volume{}
	for _, volume := range pod.Spec.Volumes {
		volumes[volume.Name] = volume
	}

	items := make([]mboxruntime.RuntimeStorage, 0, len(container.VolumeMounts))
	for _, mount := range container.VolumeMounts {
		volume, ok := volumes[mount.Name]
		if !ok || volume.PersistentVolumeClaim == nil {
			continue
		}
		item := mboxruntime.RuntimeStorage{
			Name:      mount.Name,
			MountPath: mount.MountPath,
			ClaimName: volume.PersistentVolumeClaim.ClaimName,
		}
		if a.coreClient != nil {
			pvc, err := a.coreClient.CoreV1().PersistentVolumeClaims(pod.Namespace).Get(ctx, item.ClaimName, metav1.GetOptions{})
			if err == nil {
				item.Phase = string(pvc.Status.Phase)
				if storage := pvc.Status.Capacity.Storage(); storage != nil {
					item.Capacity = storage.String()
				}
				if pvc.Spec.StorageClassName != nil {
					item.StorageClassName = *pvc.Spec.StorageClassName
				}
			} else if !apierrors.IsNotFound(err) {
				item.Message = err.Error()
			}
		}
		items = append(items, item)
	}
	return items
}

func findContainer(pod corev1.Pod, name string) (corev1.Container, bool) {
	for _, container := range pod.Spec.Containers {
		if container.Name == name {
			return container, true
		}
	}
	return corev1.Container{}, false
}

func runtimePorts(claim *unstructured.Unstructured) []domain.SandboxPort {
	ports, ok, _ := unstructured.NestedSlice(claim.Object, "status", "ports")
	if !ok {
		return nil
	}
	out := make([]domain.SandboxPort, 0, len(ports))
	for _, item := range ports {
		port, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name, _ := port["name"].(string)
		protocol, _ := port["protocol"].(string)
		previewURL, _ := port["previewUrl"].(string)
		number := portNumber(port["port"])
		if number == 0 {
			number = portNumber(port["containerPort"])
		}
		if number == 0 {
			continue
		}
		out = append(out, domain.SandboxPort{
			Name:       defaultString(name, fmt.Sprintf("port-%d", number)),
			Port:       number,
			Protocol:   defaultString(protocol, "TCP"),
			PreviewURL: previewURL,
		})
	}
	return out
}

func portNumber(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case int32:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
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

package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/mlhiter/mbox/internal/domain"
)

type boundaryCheck struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Status   string   `json:"status"`
	Message  string   `json:"message"`
	Evidence []string `json:"evidence,omitempty"`
}

type boundaryPort struct {
	Name     string `json:"name"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
}

type boundarySummary struct {
	Kind                         string             `json:"kind"`
	ProjectID                    string             `json:"projectId,omitempty"`
	ProjectName                  string             `json:"projectName,omitempty"`
	TemplateID                   string             `json:"templateId"`
	TemplateName                 string             `json:"templateName"`
	SandboxID                    string             `json:"sandboxId,omitempty"`
	SandboxName                  string             `json:"sandboxName,omitempty"`
	SandboxStatus                string             `json:"sandboxStatus,omitempty"`
	Namespace                    string             `json:"namespace,omitempty"`
	ServiceAccountName           string             `json:"serviceAccountName,omitempty"`
	ServiceAccountTokenAutomount bool               `json:"serviceAccountTokenAutomount"`
	RuntimeRef                   *domain.RuntimeRef `json:"runtimeRef,omitempty"`
	Image                        string             `json:"image"`
	WorkingDir                   string             `json:"workingDir"`
	ResourceRequests             map[string]string  `json:"resourceRequests,omitempty"`
	StorageRequest               string             `json:"storageRequest,omitempty"`
	PreviewPorts                 []boundaryPort     `json:"previewPorts,omitempty"`
	EnvVarCount                  int                `json:"envVarCount"`
	SecretRefs                   []domain.SecretRef `json:"secretRefs,omitempty"`
	SecretProjection             string             `json:"secretProjection"`
	NetworkPolicy                string             `json:"networkPolicy"`
	NetworkPolicyProjection      string             `json:"networkPolicyProjection"`
	LifecyclePolicy              json.RawMessage    `json:"lifecyclePolicy,omitempty"`
	LifecyclePolicyProjection    string             `json:"lifecyclePolicyProjection"`
	PolicyEnforcement            string             `json:"policyEnforcement"`
	AllowedImagePrefixes         []string           `json:"allowedImagePrefixes,omitempty"`
	AllowedServiceAccounts       []string           `json:"allowedServiceAccounts,omitempty"`
	AllowedSecretRefs            []string           `json:"allowedSecretRefs,omitempty"`
	CredentialRefs               []credentialRef    `json:"credentialRefs,omitempty"`
	CredentialProjection         string             `json:"credentialProjection"`
	ControllerPermissions        []string           `json:"controllerPermissions"`
	RuntimeAccess                []string           `json:"runtimeAccess"`
	Cleanup                      []string           `json:"cleanup"`
	Checks                       []boundaryCheck    `json:"checks"`
}

type credentialRef struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Slug      string   `json:"slug"`
	Type      string   `json:"type"`
	Target    string   `json:"target,omitempty"`
	SecretRef string   `json:"secretRef"`
	Usage     []string `json:"usage,omitempty"`
}

func (api *API) getTemplateBoundary(w http.ResponseWriter, r *http.Request) {
	templateID, ok := parseUUIDParam(r, "templateID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid template id")
		return
	}
	template, err := api.store.GetTemplate(r.Context(), templateID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	project, ok := api.projectForTemplateBoundary(w, r, template)
	if !ok {
		return
	}
	policy, ok := api.policyForBoundary(w, r, project)
	if !ok {
		return
	}
	credentials, ok := api.credentialsForBoundary(w, r, project)
	if !ok {
		return
	}
	summary := buildBoundarySummary("template", project, template, nil, policy, credentials)
	writeJSON(w, http.StatusOK, summary)
}

func (api *API) getSandboxBoundary(w http.ResponseWriter, r *http.Request) {
	sandbox, ok := api.sandboxFromPath(w, r)
	if !ok {
		return
	}
	template, err := api.store.GetTemplate(r.Context(), sandbox.TemplateID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	project, err := api.store.GetProject(r.Context(), sandbox.ProjectID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	policy, ok := api.policyForBoundary(w, r, &project)
	if !ok {
		return
	}
	credentials, ok := api.credentialsForBoundary(w, r, &project)
	if !ok {
		return
	}
	summary := buildBoundarySummary("sandbox", &project, template, &sandbox, policy, credentials)
	writeJSON(w, http.StatusOK, summary)
}

func (api *API) projectForTemplateBoundary(w http.ResponseWriter, r *http.Request, template domain.EnvironmentTemplate) (*domain.Project, bool) {
	if template.ProjectID != nil {
		project, err := api.store.GetProject(r.Context(), *template.ProjectID)
		if err != nil {
			writeStoreError(w, err)
			return nil, false
		}
		return &project, true
	}
	queryProjectID, ok := parseOptionalUUIDQuery(r, "projectId")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid projectId query")
		return nil, false
	}
	if queryProjectID == nil {
		return nil, true
	}
	project, err := api.store.GetProject(r.Context(), *queryProjectID)
	if err != nil {
		writeStoreError(w, err)
		return nil, false
	}
	return &project, true
}

func (api *API) policyForBoundary(w http.ResponseWriter, r *http.Request, project *domain.Project) (*domain.ProjectPolicy, bool) {
	if project == nil {
		return nil, true
	}
	policy, err := api.effectiveProjectPolicy(r.Context(), project.ID)
	if err != nil {
		writeStoreError(w, err)
		return nil, false
	}
	return &policy, true
}

func (api *API) credentialsForBoundary(w http.ResponseWriter, r *http.Request, project *domain.Project) ([]domain.ProjectCredential, bool) {
	if project == nil {
		return nil, true
	}
	credentials, err := api.store.ListProjectCredentials(r.Context(), project.ID)
	if err != nil {
		writeStoreError(w, err)
		return nil, false
	}
	return credentials, true
}

func buildBoundarySummary(
	kind string,
	project *domain.Project,
	template domain.EnvironmentTemplate,
	sandbox *domain.Sandbox,
	policy *domain.ProjectPolicy,
	credentials []domain.ProjectCredential,
) boundarySummary {
	namespace := ""
	serviceAccountName := ""
	var runtimeRef *domain.RuntimeRef
	sandboxID := ""
	sandboxName := ""
	sandboxStatus := ""
	if project != nil {
		namespace = project.DefaultNamespace
	}
	if sandbox != nil {
		namespace = sandbox.Namespace
		serviceAccountName = sandbox.ServiceAccountName
		runtimeRef = sandbox.RuntimeRef
		sandboxID = sandbox.ID.String()
		sandboxName = sandbox.Name
		sandboxStatus = string(sandbox.Status)
	}
	if serviceAccountName == "" && project != nil {
		serviceAccountName = defaultSandboxServiceAccountName
	}

	summary := boundarySummary{
		Kind:                         kind,
		TemplateID:                   template.ID.String(),
		TemplateName:                 template.Name,
		SandboxID:                    sandboxID,
		SandboxName:                  sandboxName,
		SandboxStatus:                sandboxStatus,
		Namespace:                    namespace,
		ServiceAccountName:           serviceAccountName,
		ServiceAccountTokenAutomount: false,
		RuntimeRef:                   runtimeRef,
		Image:                        template.Image,
		WorkingDir:                   stringDefault(template.WorkingDir, "/workspace"),
		ResourceRequests:             boundaryResourceRequests(template),
		StorageRequest:               template.StorageRequest,
		PreviewPorts:                 boundaryPorts(template.ExposedPorts),
		EnvVarCount:                  boundaryObjectLen(template.Env),
		SecretRefs:                   template.SecretRefs,
		SecretProjection:             secretProjection(template.SecretRefs),
		NetworkPolicy:                stringDefault(template.NetworkPolicy, "default"),
		NetworkPolicyProjection:      networkPolicyProjection(template.NetworkPolicy),
		LifecyclePolicy:              nonEmptyJSONObject(template.LifecyclePolicy),
		LifecyclePolicyProjection:    lifecyclePolicyProjection(template.LifecyclePolicy),
		PolicyEnforcement:            boundaryPolicyEnforcement(policy),
		AllowedImagePrefixes:         boundaryPolicyStrings(policy, func(value domain.ProjectPolicy) []string { return value.AllowedImagePrefixes }),
		AllowedServiceAccounts:       boundaryPolicyStrings(policy, func(value domain.ProjectPolicy) []string { return value.AllowedServiceAccounts }),
		AllowedSecretRefs:            boundaryPolicyStrings(policy, func(value domain.ProjectPolicy) []string { return value.AllowedSecretRefs }),
		CredentialRefs:               boundaryCredentialRefs(credentials),
		CredentialProjection:         credentialProjection(credentials),
		ControllerPermissions: []string{
			"ensure namespace",
			"create or update sandbox ServiceAccount",
			"create or update agent-sandbox SandboxTemplate",
			"create, start, stop, and delete agent-sandbox SandboxClaim",
		},
		RuntimeAccess: []string{
			"resolve runtime target",
			"pods/exec for terminal and execution tasks when runtime access is enabled",
			"read pod logs and Kubernetes events when runtime access is enabled",
			"proxy declared TCP preview ports when runtime access is enabled",
			"read workspace:// artifact content inside the resolved workspace mount",
		},
		Cleanup: []string{
			"stop marks the mbox record stopped and scales the runtime Sandbox to zero",
			"start marks the mbox record pending and resumes the existing runtime when possible",
			"delete soft-deletes the mbox sandbox and deletes the agent-sandbox SandboxClaim",
			"PVC cleanup is owned by the runtime controller and must be smoke-tested per cluster",
		},
	}
	if project != nil {
		summary.ProjectID = project.ID.String()
		summary.ProjectName = project.Name
	}
	summary.Checks = boundaryChecks(summary, template, sandbox, policy, credentials)
	return summary
}

func boundaryResourceRequests(template domain.EnvironmentTemplate) map[string]string {
	requests := map[string]string{}
	if template.CPURequest != "" {
		requests["cpu"] = template.CPURequest
	}
	if template.MemoryRequest != "" {
		requests["memory"] = template.MemoryRequest
	}
	if len(requests) == 0 {
		return nil
	}
	return requests
}

func boundaryPorts(ports []domain.TemplatePort) []boundaryPort {
	if len(ports) == 0 {
		return nil
	}
	out := make([]boundaryPort, 0, len(ports))
	for _, port := range ports {
		out = append(out, boundaryPort{
			Name:     port.Name,
			Port:     port.Port,
			Protocol: defaultProtocol(port.Protocol),
		})
	}
	return out
}

func boundaryObjectLen(data json.RawMessage) int {
	object, err := metadataObject(data)
	if err != nil {
		return 0
	}
	return len(object)
}

func nonEmptyJSONObject(data json.RawMessage) json.RawMessage {
	object, err := metadataObject(data)
	if err != nil || len(object) == 0 {
		return nil
	}
	out, err := json.Marshal(object)
	if err != nil {
		return nil
	}
	return out
}

func secretProjection(secretRefs []domain.SecretRef) string {
	if len(secretRefs) == 0 {
		return "none"
	}
	return "references-recorded-not-mounted"
}

func networkPolicyProjection(networkPolicy string) string {
	if strings.TrimSpace(networkPolicy) == "" || strings.TrimSpace(networkPolicy) == "default" {
		return "agent-sandbox-managed-baseline"
	}
	return "recorded-not-custom-projected"
}

func lifecyclePolicyProjection(data json.RawMessage) string {
	if len(nonEmptyJSONObject(data)) == 0 {
		return "not-configured"
	}
	if _, ok := domain.LifecycleTTL(data); ok {
		return "ttl-enforced"
	}
	return "recorded-not-enforced"
}

func credentialProjection(credentials []domain.ProjectCredential) string {
	if len(credentials) == 0 {
		return "none"
	}
	return "references-recorded-not-mounted"
}

func boundaryCredentialRefs(credentials []domain.ProjectCredential) []credentialRef {
	if len(credentials) == 0 {
		return nil
	}
	out := make([]credentialRef, 0, len(credentials))
	for _, credential := range credentials {
		out = append(out, credentialRef{
			ID:        credential.ID.String(),
			Name:      credential.Name,
			Slug:      credential.Slug,
			Type:      string(credential.Type),
			Target:    credential.Target,
			SecretRef: credential.SecretRef.Name,
			Usage:     credential.Usage,
		})
	}
	return out
}

func boundaryChecks(
	summary boundarySummary,
	template domain.EnvironmentTemplate,
	sandbox *domain.Sandbox,
	policy *domain.ProjectPolicy,
	credentials []domain.ProjectCredential,
) []boundaryCheck {
	checks := []boundaryCheck{}
	checks = append(checks, boundaryCheck{
		ID:      "namespace",
		Label:   "Namespace",
		Status:  statusIf(summary.Namespace != "", "pass", "warn"),
		Message: messageIf(summary.Namespace != "", "Runtime namespace is resolved.", "Runtime namespace is not resolved until a project or sandbox is selected."),
		Evidence: []string{
			stringDefault(summary.Namespace, "not resolved"),
		},
	})
	checks = append(checks, boundaryCheck{
		ID:      "service-account",
		Label:   "Runtime identity",
		Status:  statusIf(summary.ServiceAccountName != "", "pass", "warn"),
		Message: messageIf(summary.ServiceAccountName != "", "Sandbox runtime identity is explicit.", "Sandbox runtime identity is not resolved yet."),
		Evidence: []string{
			stringDefault(summary.ServiceAccountName, "not resolved"),
		},
	})
	checks = append(checks, boundaryCheck{
		ID:      "service-account-token",
		Label:   "ServiceAccount token",
		Status:  "pass",
		Message: "Generated ServiceAccounts and pod templates disable token automount by default.",
		Evidence: []string{
			"serviceAccountTokenAutomount=false",
		},
	})
	if len(template.SecretRefs) == 0 {
		checks = append(checks, boundaryCheck{
			ID:      "secret-refs",
			Label:   "Secret references",
			Status:  "pass",
			Message: "No template secret references are declared.",
		})
	} else {
		checks = append(checks, boundaryCheck{
			ID:      "secret-refs",
			Label:   "Secret references",
			Status:  "warn",
			Message: "Secret references are visible by name/key only and are not mounted by the current runtime adapter.",
			Evidence: []string{
				"secretProjection=references-recorded-not-mounted",
			},
		})
	}
	if len(credentials) == 0 {
		checks = append(checks, boundaryCheck{
			ID:      "credential-refs",
			Label:   "Credential references",
			Status:  "pass",
			Message: "No project credential references are registered.",
		})
	} else {
		checks = append(checks, boundaryCheck{
			ID:      "credential-refs",
			Label:   "Credential references",
			Status:  "warn",
			Message: "Project credential references are cataloged by secret name only and are not mounted by the current runtime adapter.",
			Evidence: []string{
				"credentialProjection=references-recorded-not-mounted",
				"credentialRefs=" + strings.Join(boundaryCredentialNames(credentials), ","),
			},
		})
	}
	networkMessage := "agent-sandbox manages baseline NetworkPolicy resources; mbox does not yet project custom egress policy from the template field."
	checks = append(checks, boundaryCheck{
		ID:      "network-policy",
		Label:   "Network policy",
		Status:  "warn",
		Message: networkMessage,
		Evidence: []string{
			"networkPolicy=" + summary.NetworkPolicy,
			"projection=" + summary.NetworkPolicyProjection,
		},
	})
	if len(nonEmptyJSONObject(template.LifecyclePolicy)) == 0 {
		checks = append(checks, boundaryCheck{
			ID:      "lifecycle-policy",
			Label:   "Lifecycle policy",
			Status:  "warn",
			Message: "No TTL or idle cleanup policy is configured yet; stop/delete remain explicit operations.",
		})
	} else if _, ok := domain.LifecycleTTL(template.LifecyclePolicy); ok {
		checks = append(checks, boundaryCheck{
			ID:      "lifecycle-policy",
			Label:   "Lifecycle policy",
			Status:  "pass",
			Message: "ttlSeconds is enforced by the runtime reconciler; idle cleanup remains future policy work.",
			Evidence: []string{
				"lifecyclePolicyProjection=ttl-enforced",
			},
		})
	} else {
		checks = append(checks, boundaryCheck{
			ID:      "lifecycle-policy",
			Label:   "Lifecycle policy",
			Status:  "warn",
			Message: "Lifecycle policy is recorded on the template, but automatic enforcement is still future work.",
		})
	}
	if sandbox != nil {
		checks = append(checks, boundaryCheck{
			ID:      "runtime-ref",
			Label:   "Runtime reference",
			Status:  statusIf(sandbox.RuntimeRef != nil, "pass", "warn"),
			Message: messageIf(sandbox.RuntimeRef != nil, "Sandbox has a runtime reference.", "Runtime projection has not produced a reference yet."),
		})
	}
	checks = append(checks, boundaryPolicyCheck(policy, template, summary.ServiceAccountName))
	return checks
}

func boundaryCredentialNames(credentials []domain.ProjectCredential) []string {
	names := make([]string, 0, len(credentials))
	for _, credential := range credentials {
		names = append(names, credential.Slug)
	}
	return names
}

func boundaryPolicyEnforcement(policy *domain.ProjectPolicy) string {
	if policy == nil || policy.Enforcement == "" {
		return string(domain.ProjectPolicyEnforcementDisabled)
	}
	return string(policy.Enforcement)
}

func boundaryPolicyStrings(policy *domain.ProjectPolicy, pick func(domain.ProjectPolicy) []string) []string {
	if policy == nil {
		return nil
	}
	values := pick(*policy)
	if len(values) == 0 {
		return nil
	}
	return values
}

func boundaryPolicyCheck(
	policy *domain.ProjectPolicy,
	template domain.EnvironmentTemplate,
	serviceAccountName string,
) boundaryCheck {
	if policy == nil {
		return boundaryCheck{
			ID:      "launch-policy",
			Label:   "Launch policy",
			Status:  "warn",
			Message: "Project launch policy is not resolved until a project scope is selected.",
		}
	}
	if policy.Enforcement != domain.ProjectPolicyEnforcementEnforced {
		return boundaryCheck{
			ID:      "launch-policy",
			Label:   "Launch policy",
			Status:  "warn",
			Message: "Project launch policy is disabled; sandbox launches use default platform checks only.",
			Evidence: []string{
				"policyEnforcement=disabled",
			},
		}
	}
	violations := projectPolicyViolations(*policy, template, serviceAccountName)
	if len(violations) > 0 {
		return boundaryCheck{
			ID:       "launch-policy",
			Label:    "Launch policy",
			Status:   "fail",
			Message:  "Current runtime shape would be denied by the project launch policy.",
			Evidence: violations,
		}
	}
	return boundaryCheck{
		ID:      "launch-policy",
		Label:   "Launch policy",
		Status:  "pass",
		Message: "Project launch policy allows this runtime shape.",
		Evidence: []string{
			"policyEnforcement=enforced",
		},
	}
}

func statusIf(condition bool, whenTrue string, whenFalse string) string {
	if condition {
		return whenTrue
	}
	return whenFalse
}

func messageIf(condition bool, whenTrue string, whenFalse string) string {
	if condition {
		return whenTrue
	}
	return whenFalse
}

func stringDefault(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

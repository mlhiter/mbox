package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/mlhiter/mbox/internal/domain"
)

type projectPolicyRequest struct {
	Enforcement            domain.ProjectPolicyEnforcement `json:"enforcement"`
	AllowedImagePrefixes   []string                        `json:"allowedImagePrefixes"`
	AllowedServiceAccounts []string                        `json:"allowedServiceAccounts"`
	AllowedSecretRefs      []string                        `json:"allowedSecretRefs"`
}

type projectQuotaPolicyRequest struct {
	Enforcement              domain.ProjectQuotaPolicyEnforcement `json:"enforcement"`
	MaxActiveSandboxes       *int                                 `json:"maxActiveSandboxes"`
	MaxRetainedArtifactBytes *int64                               `json:"maxRetainedArtifactBytes"`
}

func (api *API) getProjectPolicy(w http.ResponseWriter, r *http.Request) {
	projectID, ok := parseUUIDParam(r, "projectID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	if _, err := api.store.GetProject(r.Context(), projectID); err != nil {
		writeStoreError(w, err)
		return
	}
	policy, err := api.effectiveProjectPolicy(r.Context(), projectID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, policy)
}

func (api *API) putProjectPolicy(w http.ResponseWriter, r *http.Request) {
	projectID, ok := parseUUIDParam(r, "projectID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	if _, err := api.store.GetProject(r.Context(), projectID); err != nil {
		writeStoreError(w, err)
		return
	}
	var req projectPolicyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	if !validProjectPolicyEnforcement(req.Enforcement) {
		writeError(w, http.StatusBadRequest, "enforcement must be disabled or enforced")
		return
	}
	allowedImagePrefixes := normalizeStringList(req.AllowedImagePrefixes)
	allowedServiceAccounts := normalizeStringList(req.AllowedServiceAccounts)
	allowedSecretRefs := normalizeStringList(req.AllowedSecretRefs)
	policy, err := api.store.UpsertProjectPolicy(r.Context(), projectID, domain.ProjectPolicyUpsert{
		Enforcement:            req.Enforcement,
		AllowedImagePrefixes:   allowedImagePrefixes,
		AllowedServiceAccounts: allowedServiceAccounts,
		AllowedSecretRefs:      allowedSecretRefs,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	api.recordAuditEvent(r.Context(), domain.AuditEventCreate{
		ProjectID:    &projectID,
		Action:       "project.policy.updated",
		ResourceType: "project-policy",
		ResourceID:   &projectID,
		ResourceName: projectID.String(),
		Metadata: auditMetadata(map[string]any{
			"enforcement":            policy.Enforcement,
			"allowedImagePrefixes":   policy.AllowedImagePrefixes,
			"allowedServiceAccounts": policy.AllowedServiceAccounts,
			"allowedSecretRefCount":  len(policy.AllowedSecretRefs),
		}),
	})
	writeJSON(w, http.StatusOK, policy)
}

func (api *API) getProjectQuotaPolicy(w http.ResponseWriter, r *http.Request) {
	projectID, ok := parseUUIDParam(r, "projectID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	if _, err := api.store.GetProject(r.Context(), projectID); err != nil {
		writeStoreError(w, err)
		return
	}
	policy, err := api.effectiveProjectQuotaPolicy(r.Context(), projectID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, policy)
}

func (api *API) putProjectQuotaPolicy(w http.ResponseWriter, r *http.Request) {
	projectID, ok := parseUUIDParam(r, "projectID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	if _, err := api.store.GetProject(r.Context(), projectID); err != nil {
		writeStoreError(w, err)
		return
	}
	var req projectQuotaPolicyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	if !validProjectQuotaPolicyEnforcement(req.Enforcement) {
		writeError(w, http.StatusBadRequest, "enforcement must be disabled or enforced")
		return
	}
	if req.MaxActiveSandboxes != nil && *req.MaxActiveSandboxes < 0 {
		writeError(w, http.StatusBadRequest, "maxActiveSandboxes cannot be negative")
		return
	}
	if req.MaxRetainedArtifactBytes != nil && *req.MaxRetainedArtifactBytes < 0 {
		writeError(w, http.StatusBadRequest, "maxRetainedArtifactBytes cannot be negative")
		return
	}
	policy, err := api.store.UpsertProjectQuotaPolicy(r.Context(), projectID, domain.ProjectQuotaPolicyUpsert{
		Enforcement:              req.Enforcement,
		MaxActiveSandboxes:       req.MaxActiveSandboxes,
		MaxRetainedArtifactBytes: req.MaxRetainedArtifactBytes,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	api.recordAuditEvent(r.Context(), domain.AuditEventCreate{
		ProjectID:    &projectID,
		Action:       "project.quota_policy.updated",
		ResourceType: "project-quota-policy",
		ResourceID:   &projectID,
		ResourceName: projectID.String(),
		Metadata: auditMetadata(map[string]any{
			"enforcement":              policy.Enforcement,
			"maxActiveSandboxes":       policy.MaxActiveSandboxes,
			"maxRetainedArtifactBytes": policy.MaxRetainedArtifactBytes,
		}),
	})
	writeJSON(w, http.StatusOK, policy)
}

func (api *API) effectiveProjectPolicy(ctx context.Context, projectID uuid.UUID) (domain.ProjectPolicy, error) {
	policy, err := api.store.GetProjectPolicy(ctx, projectID)
	if err == nil {
		return policy, nil
	}
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ProjectPolicy{
			ProjectID:   projectID,
			Enforcement: domain.ProjectPolicyEnforcementDisabled,
		}, nil
	}
	return domain.ProjectPolicy{}, err
}

func (api *API) effectiveProjectQuotaPolicy(ctx context.Context, projectID uuid.UUID) (domain.ProjectQuotaPolicy, error) {
	policy, err := api.store.GetProjectQuotaPolicy(ctx, projectID)
	if err == nil {
		return policy, nil
	}
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ProjectQuotaPolicy{
			ProjectID:   projectID,
			Enforcement: domain.ProjectQuotaPolicyEnforcementDisabled,
		}, nil
	}
	return domain.ProjectQuotaPolicy{}, err
}

func (api *API) enforceSandboxLaunchPolicy(
	ctx context.Context,
	project domain.Project,
	template domain.EnvironmentTemplate,
	serviceAccountName string,
) error {
	if template.ProjectID != nil && *template.ProjectID != project.ID {
		return policyDeny("template belongs to a different project")
	}
	policy, err := api.effectiveProjectPolicy(ctx, project.ID)
	if err != nil {
		return err
	}
	if policy.Enforcement != domain.ProjectPolicyEnforcementEnforced {
		return nil
	}
	if violations := projectPolicyViolations(policy, template, serviceAccountName); len(violations) > 0 {
		return policyDeny(strings.Join(violations, "; "))
	}
	return nil
}

func (api *API) enforceSandboxQuotaPolicy(ctx context.Context, projectID uuid.UUID) error {
	policy, err := api.effectiveProjectQuotaPolicy(ctx, projectID)
	if err != nil {
		return err
	}
	if policy.Enforcement != domain.ProjectQuotaPolicyEnforcementEnforced || policy.MaxActiveSandboxes == nil {
		return nil
	}
	usage, err := api.store.GetProjectUsage(ctx, projectID)
	if err != nil {
		return err
	}
	if usage.Sandboxes.Active >= *policy.MaxActiveSandboxes {
		return policyDeny(fmt.Sprintf("active sandbox quota exceeded: maxActiveSandboxes=%d active=%d", *policy.MaxActiveSandboxes, usage.Sandboxes.Active))
	}
	return nil
}

func (api *API) enforceArtifactRetentionQuotaPolicy(ctx context.Context, projectID uuid.UUID, incomingBytes int64) error {
	policy, err := api.effectiveProjectQuotaPolicy(ctx, projectID)
	if err != nil {
		return err
	}
	if policy.Enforcement != domain.ProjectQuotaPolicyEnforcementEnforced || policy.MaxRetainedArtifactBytes == nil {
		return nil
	}
	usage, err := api.store.GetProjectUsage(ctx, projectID)
	if err != nil {
		return err
	}
	nextBytes := usage.Artifacts.RetainedBytes + incomingBytes
	if nextBytes > *policy.MaxRetainedArtifactBytes {
		return policyDeny(fmt.Sprintf("retained artifact quota exceeded: maxRetainedArtifactBytes=%d retainedBytes=%d incomingBytes=%d", *policy.MaxRetainedArtifactBytes, usage.Artifacts.RetainedBytes, incomingBytes))
	}
	return nil
}

func projectPolicyViolations(
	policy domain.ProjectPolicy,
	template domain.EnvironmentTemplate,
	serviceAccountName string,
) []string {
	violations := []string{}
	if len(policy.AllowedImagePrefixes) > 0 && !hasAllowedPrefix(template.Image, policy.AllowedImagePrefixes) {
		violations = append(violations, fmt.Sprintf("image %q is not allowed", template.Image))
	}
	if len(policy.AllowedServiceAccounts) > 0 && !slices.Contains(policy.AllowedServiceAccounts, serviceAccountName) {
		violations = append(violations, fmt.Sprintf("serviceAccountName %q is not allowed", serviceAccountName))
	}
	if len(template.SecretRefs) > 0 {
		allowedSecrets := map[string]struct{}{}
		for _, name := range policy.AllowedSecretRefs {
			allowedSecrets[name] = struct{}{}
		}
		for _, secretRef := range template.SecretRefs {
			if _, ok := allowedSecrets[secretRef.Name]; !ok {
				violations = append(violations, fmt.Sprintf("secretRef %q is not allowed", secretRef.Name))
			}
		}
	}
	return violations
}

type policyDeniedError struct {
	reason string
}

func (e policyDeniedError) Error() string {
	return "policy denied: " + e.reason
}

func policyDeny(reason string) error {
	return policyDeniedError{reason: reason}
}

func writePolicyError(w http.ResponseWriter, err error) bool {
	var denied policyDeniedError
	if errors.As(err, &denied) {
		writeError(w, http.StatusForbidden, denied.Error())
		return true
	}
	return false
}

func (api *API) recordPolicyDeniedAuditEvent(
	ctx context.Context,
	projectID uuid.UUID,
	operation string,
	resourceType string,
	resourceID *uuid.UUID,
	resourceName string,
	err error,
	metadata map[string]any,
) {
	var denied policyDeniedError
	if !errors.As(err, &denied) {
		return
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["operation"] = operation
	metadata["reason"] = denied.reason
	api.recordAuditEvent(ctx, domain.AuditEventCreate{
		ProjectID:    &projectID,
		Action:       "policy.denied",
		ResourceType: resourceType,
		ResourceID:   resourceID,
		ResourceName: resourceName,
		Metadata:     auditMetadata(metadata),
	})
}

func validProjectPolicyEnforcement(value domain.ProjectPolicyEnforcement) bool {
	switch value {
	case domain.ProjectPolicyEnforcementDisabled, domain.ProjectPolicyEnforcementEnforced:
		return true
	default:
		return false
	}
}

func validProjectQuotaPolicyEnforcement(value domain.ProjectQuotaPolicyEnforcement) bool {
	switch value {
	case domain.ProjectQuotaPolicyEnforcementDisabled, domain.ProjectQuotaPolicyEnforcementEnforced:
		return true
	default:
		return false
	}
}

func normalizeStringList(values []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func hasAllowedPrefix(value string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

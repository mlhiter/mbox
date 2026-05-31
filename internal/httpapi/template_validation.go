package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/mlhiter/mbox/internal/domain"
)

const (
	templateValidationPurpose        = "environment-validation"
	templateValidationSandboxNameMax = 58
)

type createTemplateValidationRunRequest struct {
	ProjectID *uuid.UUID      `json:"projectId"`
	Name      string          `json:"name"`
	Metadata  json.RawMessage `json:"metadata"`
}

type decideTemplateValidationRunRequest struct {
	Status string `json:"status"`
}

type templateValidationRunResponse struct {
	Template domain.EnvironmentTemplate `json:"template"`
	Sandbox  domain.Sandbox             `json:"sandbox"`
}

func (api *API) createTemplateValidationRun(w http.ResponseWriter, r *http.Request) {
	templateID, ok := parseUUIDParam(r, "templateID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid template id")
		return
	}
	var req createTemplateValidationRunRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	if _, err := metadataObject(req.Metadata); err != nil {
		writeError(w, http.StatusBadRequest, "metadata must be a JSON object")
		return
	}
	template, err := api.store.GetTemplate(r.Context(), templateID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	project, ok := api.projectForTemplateValidation(w, r, template, req.ProjectID)
	if !ok {
		return
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	name := validationSandboxName(template.Name, req.Name)
	slug := validationSandboxSlug(template.Slug, name)
	namespace := project.DefaultNamespace
	if !validateRequired(namespace) {
		writeError(w, http.StatusBadRequest, "project defaultNamespace is required")
		return
	}
	if err := api.enforceSandboxLaunchPolicy(r.Context(), project, template, defaultSandboxServiceAccountName); err != nil {
		api.recordPolicyDeniedAuditEvent(r.Context(), project.ID, "template.validation", "template", &template.ID, template.Name, err, map[string]any{
			"templateId":         template.ID.String(),
			"image":              template.Image,
			"serviceAccountName": defaultSandboxServiceAccountName,
		})
		if writePolicyError(w, err) {
			return
		}
		writeStoreError(w, err)
		return
	}
	sandboxMetadata, err := mergeMetadataObjects(req.Metadata, map[string]any{
		"purpose":             templateValidationPurpose,
		"templateId":          template.ID.String(),
		"validationStatus":    "testing",
		"validationStartedAt": now,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, "metadata must be a JSON object")
		return
	}
	sandbox, err := api.store.CreateSandbox(r.Context(), domain.SandboxCreate{
		ProjectID:          project.ID,
		TemplateID:         template.ID,
		Name:               name,
		Slug:               slug,
		Namespace:          namespace,
		ServiceAccountName: defaultSandboxServiceAccountName,
		Ports:              sandboxPortsFromTemplate(template.ExposedPorts),
		Metadata:           sandboxMetadata,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}

	templateMetadata, err := mergeMetadataObjects(template.Metadata, map[string]any{
		"validationStatus":    "testing",
		"validationSandboxId": sandbox.ID.String(),
		"validationStartedAt": now,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "template metadata must be a JSON object")
		return
	}
	updatedTemplate, err := api.store.UpdateTemplate(r.Context(), template.ID, domain.TemplateUpdate{
		Metadata: &templateMetadata,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	api.recordAuditEvent(r.Context(), domain.AuditEventCreate{
		ProjectID:    &sandbox.ProjectID,
		Action:       "template.validation.started",
		ResourceType: "template",
		ResourceID:   &updatedTemplate.ID,
		ResourceName: updatedTemplate.Name,
		Metadata: auditMetadata(map[string]any{
			"sandboxId": sandbox.ID.String(),
		}),
	})
	writeJSON(w, http.StatusCreated, templateValidationRunResponse{Template: updatedTemplate, Sandbox: sandbox})
}

func (api *API) decideTemplateValidationRun(w http.ResponseWriter, r *http.Request) {
	templateID, ok := parseUUIDParam(r, "templateID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid template id")
		return
	}
	sandboxID, ok := parseUUIDParam(r, "sandboxID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid sandbox id")
		return
	}
	var req decideTemplateValidationRunRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	if req.Status != "passed" && req.Status != "failed" {
		writeError(w, http.StatusBadRequest, "status must be passed or failed")
		return
	}

	template, err := api.store.GetTemplate(r.Context(), templateID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	sandbox, err := api.store.GetSandbox(r.Context(), sandboxID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if sandbox.TemplateID != template.ID {
		writeError(w, http.StatusBadRequest, "validation sandbox does not belong to template")
		return
	}
	sandboxMetadataObject, err := metadataObject(sandbox.Metadata)
	if err != nil {
		writeError(w, http.StatusBadRequest, "sandbox metadata must be a JSON object")
		return
	}
	if sandboxMetadataObject["purpose"] != templateValidationPurpose {
		writeError(w, http.StatusBadRequest, "sandbox is not a template validation run")
		return
	}
	if metadataTemplateID, _ := sandboxMetadataObject["templateId"].(string); metadataTemplateID != "" && metadataTemplateID != template.ID.String() {
		writeError(w, http.StatusBadRequest, "validation sandbox template metadata does not match")
		return
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	templateMetadata, err := mergeMetadataObjects(template.Metadata, map[string]any{
		"validationStatus":    req.Status,
		"validationSandboxId": sandbox.ID.String(),
		"validationDecidedAt": now,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "template metadata must be a JSON object")
		return
	}
	sandboxMetadata, err := mergeMetadataObjects(sandbox.Metadata, map[string]any{
		"purpose":             templateValidationPurpose,
		"templateId":          template.ID.String(),
		"validationResult":    req.Status,
		"validationDecidedAt": now,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, "sandbox metadata must be a JSON object")
		return
	}
	updatedTemplate, err := api.store.UpdateTemplate(r.Context(), template.ID, domain.TemplateUpdate{
		Metadata: &templateMetadata,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	updatedSandbox, err := api.store.UpdateSandbox(r.Context(), sandbox.ID, domain.SandboxUpdate{
		Metadata: &sandboxMetadata,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	api.recordAuditEvent(r.Context(), domain.AuditEventCreate{
		ProjectID:    &updatedSandbox.ProjectID,
		Action:       "template.validation.decided",
		ResourceType: "template",
		ResourceID:   &updatedTemplate.ID,
		ResourceName: updatedTemplate.Name,
		Metadata: auditMetadata(map[string]any{
			"sandboxId": updatedSandbox.ID.String(),
			"status":    req.Status,
		}),
	})
	writeJSON(w, http.StatusOK, templateValidationRunResponse{Template: updatedTemplate, Sandbox: updatedSandbox})
}

func (api *API) projectForTemplateValidation(
	w http.ResponseWriter,
	r *http.Request,
	template domain.EnvironmentTemplate,
	requestProjectID *uuid.UUID,
) (domain.Project, bool) {
	var projectID uuid.UUID
	if template.ProjectID != nil {
		projectID = *template.ProjectID
		if requestProjectID != nil && *requestProjectID != projectID {
			writeError(w, http.StatusBadRequest, "projectId must match template scope")
			return domain.Project{}, false
		}
	} else {
		if requestProjectID == nil || *requestProjectID == uuid.Nil {
			writeError(w, http.StatusBadRequest, "projectId is required for global template validation")
			return domain.Project{}, false
		}
		projectID = *requestProjectID
	}
	project, err := api.store.GetProject(r.Context(), projectID)
	if err != nil {
		writeStoreError(w, err)
		return domain.Project{}, false
	}
	return project, true
}

func validationSandboxName(templateName string, requestedName string) string {
	name := strings.TrimSpace(requestedName)
	if name == "" {
		name = "Validate " + strings.TrimSpace(templateName)
	}
	if name == "Validate" {
		name = "Validate environment"
	}
	return truncateRunes(name, templateValidationSandboxNameMax)
}

func validationSandboxSlug(templateSlug string, name string) string {
	base := slugFromName(name)
	if base == "" {
		base = "validate-" + slugFromName(templateSlug)
	}
	base = strings.Trim(base, "-")
	if base == "" {
		base = "validation"
	}
	return base + "-" + strconv.FormatInt(time.Now().UTC().UnixNano(), 36)
}

func truncateRunes(value string, maxRunes int) string {
	if maxRunes <= 0 || utf8.RuneCountInString(value) <= maxRunes {
		return value
	}
	runes := []rune(value)
	return string(runes[:maxRunes])
}

func mergeMetadataObjects(data json.RawMessage, values map[string]any) ([]byte, error) {
	object, err := metadataObject(data)
	if err != nil {
		return nil, err
	}
	for key, value := range values {
		object[key] = value
	}
	return json.Marshal(object)
}

func metadataObject(data json.RawMessage) (map[string]any, error) {
	if len(data) == 0 || strings.TrimSpace(string(data)) == "" || strings.TrimSpace(string(data)) == "null" {
		return map[string]any{}, nil
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, err
	}
	if object == nil {
		return map[string]any{}, nil
	}
	return object, nil
}

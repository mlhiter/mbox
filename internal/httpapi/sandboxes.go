package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/mlhiter/mbox/internal/domain"
)

type createSandboxRequest struct {
	ProjectID          uuid.UUID       `json:"projectId"`
	TemplateID         uuid.UUID       `json:"templateId"`
	Name               string          `json:"name"`
	Slug               string          `json:"slug"`
	Namespace          string          `json:"namespace"`
	ServiceAccountName string          `json:"serviceAccountName"`
	Metadata           json.RawMessage `json:"metadata"`
}

const defaultSandboxServiceAccountName = "mbox-sandbox"

type updateSandboxRequest struct {
	Name               *string               `json:"name"`
	Status             *domain.SandboxStatus `json:"status"`
	Namespace          *string               `json:"namespace"`
	ServiceAccountName *string               `json:"serviceAccountName"`
	RuntimeRef         nullableRuntimeRef    `json:"runtimeRef"`
	Ports              *[]domain.SandboxPort `json:"ports"`
	Metadata           *json.RawMessage      `json:"metadata"`
}

func (req *updateSandboxRequest) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if err := rejectUnknownFields(fields, map[string]struct{}{
		"name":               {},
		"status":             {},
		"namespace":          {},
		"serviceAccountName": {},
		"runtimeRef":         {},
		"ports":              {},
		"metadata":           {},
	}); err != nil {
		return err
	}
	if raw, ok := fields["name"]; ok {
		if err := json.Unmarshal(raw, &req.Name); err != nil {
			return err
		}
	}
	if raw, ok := fields["status"]; ok {
		if err := json.Unmarshal(raw, &req.Status); err != nil {
			return err
		}
	}
	if raw, ok := fields["namespace"]; ok {
		if err := json.Unmarshal(raw, &req.Namespace); err != nil {
			return err
		}
	}
	if raw, ok := fields["serviceAccountName"]; ok {
		if err := json.Unmarshal(raw, &req.ServiceAccountName); err != nil {
			return err
		}
	}
	if raw, ok := fields["runtimeRef"]; ok {
		value, err := parseNullableRuntimeRef(raw)
		if err != nil {
			return err
		}
		req.RuntimeRef = value
	}
	if raw, ok := fields["ports"]; ok {
		if err := json.Unmarshal(raw, &req.Ports); err != nil {
			return err
		}
	}
	if raw, ok := fields["metadata"]; ok {
		req.Metadata = &raw
	}
	return nil
}

func (api *API) listSandboxes(w http.ResponseWriter, r *http.Request) {
	projectID, ok := parseOptionalUUIDQuery(r, "projectId")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid projectId query")
		return
	}
	sandboxes, err := api.store.ListSandboxes(r.Context(), projectID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": sandboxes})
}

func (api *API) createSandbox(w http.ResponseWriter, r *http.Request) {
	var req createSandboxRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	slug := slugOrName(req.Slug, req.Name)
	if req.ProjectID == uuid.Nil || !validateRequired(req.Name) || !validateSlug(slug) {
		writeError(w, http.StatusBadRequest, "projectId, name, and valid slug are required")
		return
	}

	project, err := api.store.GetProject(r.Context(), req.ProjectID)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	templateID := req.TemplateID
	if templateID == uuid.Nil {
		if project.DefaultTemplateID == nil {
			writeError(w, http.StatusBadRequest, "templateId is required unless project has defaultTemplateId")
			return
		}
		templateID = *project.DefaultTemplateID
	}
	namespace := req.Namespace
	if !validateRequired(namespace) {
		namespace = project.DefaultNamespace
	}
	serviceAccountName := req.ServiceAccountName
	if !validateRequired(serviceAccountName) {
		serviceAccountName = defaultSandboxServiceAccountName
	}
	if !validateRequired(namespace) || !validateRequired(serviceAccountName) {
		writeError(w, http.StatusBadRequest, "namespace and serviceAccountName are required")
		return
	}
	template, err := api.store.GetTemplate(r.Context(), templateID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if err := api.enforceSandboxLaunchPolicy(r.Context(), project, template, serviceAccountName); err != nil {
		api.recordPolicyDeniedAuditEvent(r.Context(), project.ID, "sandbox.launch", "sandbox", nil, req.Name, err, map[string]any{
			"templateId":         template.ID.String(),
			"templateName":       template.Name,
			"image":              template.Image,
			"serviceAccountName": serviceAccountName,
		})
		if writePolicyError(w, err) {
			return
		}
		writeStoreError(w, err)
		return
	}
	if err := api.enforceSandboxQuotaPolicy(r.Context(), project.ID); err != nil {
		api.recordPolicyDeniedAuditEvent(r.Context(), project.ID, "sandbox.launch", "sandbox", nil, req.Name, err, map[string]any{
			"templateId":         template.ID.String(),
			"templateName":       template.Name,
			"serviceAccountName": serviceAccountName,
		})
		if writePolicyError(w, err) {
			return
		}
		writeStoreError(w, err)
		return
	}
	ports := sandboxPortsFromTemplate(template.ExposedPorts)

	sandbox, err := api.store.CreateSandbox(r.Context(), domain.SandboxCreate{
		ProjectID:          req.ProjectID,
		TemplateID:         templateID,
		Name:               req.Name,
		Slug:               slug,
		Namespace:          namespace,
		ServiceAccountName: serviceAccountName,
		Ports:              ports,
		Metadata:           req.Metadata,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	api.recordAuditEvent(r.Context(), domain.AuditEventCreate{
		ProjectID:    &sandbox.ProjectID,
		Action:       "sandbox.created",
		ResourceType: "sandbox",
		ResourceID:   &sandbox.ID,
		ResourceName: sandbox.Name,
		Metadata: auditMetadata(map[string]any{
			"templateId":           sandbox.TemplateID.String(),
			"namespace":            sandbox.Namespace,
			"serviceAccountName":   sandbox.ServiceAccountName,
			"declaredPreviewPorts": len(sandbox.Ports),
		}),
	})
	writeJSON(w, http.StatusCreated, sandbox)
}

func (api *API) getSandbox(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(r, "sandboxID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid sandbox id")
		return
	}
	sandbox, err := api.store.GetSandbox(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	api.recordAuditEvent(r.Context(), domain.AuditEventCreate{
		ProjectID:    &sandbox.ProjectID,
		Action:       "sandbox.updated",
		ResourceType: "sandbox",
		ResourceID:   &sandbox.ID,
		ResourceName: sandbox.Name,
	})
	writeJSON(w, http.StatusOK, sandbox)
}

func (api *API) updateSandbox(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(r, "sandboxID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid sandbox id")
		return
	}
	var req updateSandboxRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	if req.Name != nil && !validateRequired(*req.Name) {
		writeError(w, http.StatusBadRequest, "name cannot be empty")
		return
	}
	if req.Status != nil && !validSandboxStatus(*req.Status) {
		writeError(w, http.StatusBadRequest, "invalid sandbox status")
		return
	}
	if req.Namespace != nil && !validateRequired(*req.Namespace) {
		writeError(w, http.StatusBadRequest, "namespace cannot be empty")
		return
	}
	if req.ServiceAccountName != nil && !validateRequired(*req.ServiceAccountName) {
		writeError(w, http.StatusBadRequest, "serviceAccountName cannot be empty")
		return
	}
	if req.Ports != nil {
		if err := validateSandboxPorts(*req.Ports); err != "" {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}

	var metadata *[]byte
	if req.Metadata != nil {
		raw := []byte(*req.Metadata)
		metadata = &raw
	}
	var runtimeRef **domain.RuntimeRef
	if req.RuntimeRef.Set {
		runtimeRef = &req.RuntimeRef.Value
	}

	sandbox, err := api.store.UpdateSandbox(r.Context(), id, domain.SandboxUpdate{
		Name:               req.Name,
		Status:             req.Status,
		Namespace:          req.Namespace,
		ServiceAccountName: req.ServiceAccountName,
		RuntimeRef:         runtimeRef,
		Ports:              req.Ports,
		Metadata:           metadata,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sandbox)
}

func (api *API) deleteSandbox(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(r, "sandboxID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid sandbox id")
		return
	}
	sandbox, err := api.store.GetSandbox(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if err := api.store.DeleteSandbox(r.Context(), id); err != nil {
		writeStoreError(w, err)
		return
	}
	api.recordAuditEvent(r.Context(), domain.AuditEventCreate{
		ProjectID:    &sandbox.ProjectID,
		Action:       "sandbox.deleted",
		ResourceType: "sandbox",
		ResourceID:   &id,
		ResourceName: sandbox.Name,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (api *API) stopSandbox(w http.ResponseWriter, r *http.Request) {
	api.setSandboxLifecycleStatus(w, r, domain.SandboxStatusStopped)
}

func (api *API) startSandbox(w http.ResponseWriter, r *http.Request) {
	api.setSandboxLifecycleStatus(w, r, domain.SandboxStatusPending)
}

func (api *API) setSandboxLifecycleStatus(w http.ResponseWriter, r *http.Request, status domain.SandboxStatus) {
	id, ok := parseUUIDParam(r, "sandboxID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid sandbox id")
		return
	}
	sandbox, err := api.store.UpdateSandbox(r.Context(), id, domain.SandboxUpdate{Status: &status})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	action := "sandbox.started"
	if status == domain.SandboxStatusStopped {
		action = "sandbox.stopped"
	}
	api.recordAuditEvent(r.Context(), domain.AuditEventCreate{
		ProjectID:    &sandbox.ProjectID,
		Action:       action,
		ResourceType: "sandbox",
		ResourceID:   &sandbox.ID,
		ResourceName: sandbox.Name,
	})
	writeJSON(w, http.StatusOK, sandbox)
}

func validSandboxStatus(status domain.SandboxStatus) bool {
	switch status {
	case domain.SandboxStatusPending,
		domain.SandboxStatusRunning,
		domain.SandboxStatusStopped,
		domain.SandboxStatusFailed,
		domain.SandboxStatusDeleted:
		return true
	default:
		return false
	}
}

func validateSandboxPorts(ports []domain.SandboxPort) string {
	for _, port := range ports {
		if !validateRequired(port.Name) {
			return "port name is required"
		}
		if port.Port < 1 || port.Port > 65535 {
			return "port must be between 1 and 65535"
		}
		if port.Protocol != "" && port.Protocol != "TCP" && port.Protocol != "UDP" {
			return "port protocol must be TCP or UDP"
		}
	}
	return ""
}

func sandboxPortsFromTemplate(ports []domain.TemplatePort) []domain.SandboxPort {
	out := make([]domain.SandboxPort, 0, len(ports))
	for _, port := range ports {
		protocol := port.Protocol
		if protocol == "" {
			protocol = "TCP"
		}
		out = append(out, domain.SandboxPort{
			Name:     port.Name,
			Port:     port.Port,
			Protocol: protocol,
		})
	}
	return out
}

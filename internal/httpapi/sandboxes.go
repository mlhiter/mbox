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
	if req.ProjectID == uuid.Nil || req.TemplateID == uuid.Nil || !validateRequired(req.Name) || !validateSlug(req.Slug) {
		writeError(w, http.StatusBadRequest, "projectId, templateId, name, and valid slug are required")
		return
	}
	if !validateRequired(req.Namespace) || !validateRequired(req.ServiceAccountName) {
		writeError(w, http.StatusBadRequest, "namespace and serviceAccountName are required")
		return
	}

	sandbox, err := api.store.CreateSandbox(r.Context(), domain.SandboxCreate{
		ProjectID:          req.ProjectID,
		TemplateID:         req.TemplateID,
		Name:               req.Name,
		Slug:               req.Slug,
		Namespace:          req.Namespace,
		ServiceAccountName: req.ServiceAccountName,
		Metadata:           req.Metadata,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
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
	if err := api.store.DeleteSandbox(r.Context(), id); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/mlhiter/mbox/internal/domain"
)

type createTemplateRequest struct {
	ProjectID       *uuid.UUID            `json:"projectId"`
	Name            string                `json:"name"`
	Slug            string                `json:"slug"`
	Image           string                `json:"image"`
	StartupCommand  []string              `json:"startupCommand"`
	WorkingDir      string                `json:"workingDir"`
	CPURequest      string                `json:"cpuRequest"`
	MemoryRequest   string                `json:"memoryRequest"`
	StorageRequest  string                `json:"storageRequest"`
	ExposedPorts    []domain.TemplatePort `json:"exposedPorts"`
	Env             json.RawMessage       `json:"env"`
	SecretRefs      []domain.SecretRef    `json:"secretRefs"`
	NetworkPolicy   string                `json:"networkPolicy"`
	LifecyclePolicy json.RawMessage       `json:"lifecyclePolicy"`
	Metadata        json.RawMessage       `json:"metadata"`
}

type updateTemplateRequest struct {
	Name            *string                `json:"name"`
	Image           *string                `json:"image"`
	StartupCommand  *[]string              `json:"startupCommand"`
	WorkingDir      *string                `json:"workingDir"`
	CPURequest      *string                `json:"cpuRequest"`
	MemoryRequest   *string                `json:"memoryRequest"`
	StorageRequest  *string                `json:"storageRequest"`
	ExposedPorts    *[]domain.TemplatePort `json:"exposedPorts"`
	Env             *json.RawMessage       `json:"env"`
	SecretRefs      *[]domain.SecretRef    `json:"secretRefs"`
	NetworkPolicy   *string                `json:"networkPolicy"`
	LifecyclePolicy *json.RawMessage       `json:"lifecyclePolicy"`
	Metadata        *json.RawMessage       `json:"metadata"`
}

func (api *API) listTemplates(w http.ResponseWriter, r *http.Request) {
	projectID, ok := parseOptionalUUIDQuery(r, "projectId")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid projectId query")
		return
	}
	templates, err := api.store.ListTemplates(r.Context(), projectID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": templates})
}

func (api *API) createTemplate(w http.ResponseWriter, r *http.Request) {
	var req createTemplateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	slug := slugOrName(req.Slug, req.Name)
	if !validateRequired(req.Name) || !validateSlug(slug) || !validateRequired(req.Image) {
		writeError(w, http.StatusBadRequest, "name, valid slug, and image are required")
		return
	}
	if err := validateTemplatePorts(req.ExposedPorts); err != "" {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	template, err := api.store.CreateTemplate(r.Context(), domain.TemplateCreate{
		ProjectID:       req.ProjectID,
		Name:            req.Name,
		Slug:            slug,
		Image:           req.Image,
		StartupCommand:  req.StartupCommand,
		WorkingDir:      req.WorkingDir,
		CPURequest:      req.CPURequest,
		MemoryRequest:   req.MemoryRequest,
		StorageRequest:  req.StorageRequest,
		ExposedPorts:    req.ExposedPorts,
		Env:             req.Env,
		SecretRefs:      req.SecretRefs,
		NetworkPolicy:   req.NetworkPolicy,
		LifecyclePolicy: req.LifecyclePolicy,
		Metadata:        req.Metadata,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, template)
}

func (api *API) getTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(r, "templateID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid template id")
		return
	}
	template, err := api.store.GetTemplate(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, template)
}

func (api *API) updateTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(r, "templateID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid template id")
		return
	}
	var req updateTemplateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	if req.Name != nil && !validateRequired(*req.Name) {
		writeError(w, http.StatusBadRequest, "name cannot be empty")
		return
	}
	if req.Image != nil && !validateRequired(*req.Image) {
		writeError(w, http.StatusBadRequest, "image cannot be empty")
		return
	}
	if req.ExposedPorts != nil {
		if err := validateTemplatePorts(*req.ExposedPorts); err != "" {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}

	var env *[]byte
	if req.Env != nil {
		raw := []byte(*req.Env)
		env = &raw
	}
	var lifecyclePolicy *[]byte
	if req.LifecyclePolicy != nil {
		raw := []byte(*req.LifecyclePolicy)
		lifecyclePolicy = &raw
	}
	var metadata *[]byte
	if req.Metadata != nil {
		raw := []byte(*req.Metadata)
		metadata = &raw
	}

	template, err := api.store.UpdateTemplate(r.Context(), id, domain.TemplateUpdate{
		Name:            req.Name,
		Image:           req.Image,
		StartupCommand:  req.StartupCommand,
		WorkingDir:      req.WorkingDir,
		CPURequest:      req.CPURequest,
		MemoryRequest:   req.MemoryRequest,
		StorageRequest:  req.StorageRequest,
		ExposedPorts:    req.ExposedPorts,
		Env:             env,
		SecretRefs:      req.SecretRefs,
		NetworkPolicy:   req.NetworkPolicy,
		LifecyclePolicy: lifecyclePolicy,
		Metadata:        metadata,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, template)
}

func (api *API) deleteTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(r, "templateID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid template id")
		return
	}
	if err := api.store.DeleteTemplate(r.Context(), id); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validateTemplatePorts(ports []domain.TemplatePort) string {
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

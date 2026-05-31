package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/mlhiter/mbox/internal/domain"
)

type createProjectRequest struct {
	Name             string          `json:"name"`
	Slug             string          `json:"slug"`
	RepositoryURL    string          `json:"repositoryUrl"`
	DefaultNamespace string          `json:"defaultNamespace"`
	Metadata         json.RawMessage `json:"metadata"`
}

type updateProjectRequest struct {
	Name              *string          `json:"name"`
	RepositoryURL     *string          `json:"repositoryUrl"`
	DefaultNamespace  *string          `json:"defaultNamespace"`
	DefaultTemplateID nullableUUID     `json:"defaultTemplateId"`
	Metadata          *json.RawMessage `json:"metadata"`
}

func (req *updateProjectRequest) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if err := rejectUnknownFields(fields, map[string]struct{}{
		"name":              {},
		"repositoryUrl":     {},
		"defaultNamespace":  {},
		"defaultTemplateId": {},
		"metadata":          {},
	}); err != nil {
		return err
	}
	if raw, ok := fields["name"]; ok {
		if err := json.Unmarshal(raw, &req.Name); err != nil {
			return err
		}
	}
	if raw, ok := fields["repositoryUrl"]; ok {
		if err := json.Unmarshal(raw, &req.RepositoryURL); err != nil {
			return err
		}
	}
	if raw, ok := fields["defaultNamespace"]; ok {
		if err := json.Unmarshal(raw, &req.DefaultNamespace); err != nil {
			return err
		}
	}
	if raw, ok := fields["defaultTemplateId"]; ok {
		value, err := parseNullableUUID(raw)
		if err != nil {
			return err
		}
		req.DefaultTemplateID = value
	}
	if raw, ok := fields["metadata"]; ok {
		req.Metadata = &raw
	}
	return nil
}

func (api *API) listProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := api.store.ListProjects(r.Context())
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": projects})
}

func (api *API) createProject(w http.ResponseWriter, r *http.Request) {
	var req createProjectRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	slug := slugOrName(req.Slug, req.Name)
	if !validateRequired(req.Name) || !validateSlug(slug) || !validateRequired(req.DefaultNamespace) {
		writeError(w, http.StatusBadRequest, "name, valid slug, and defaultNamespace are required")
		return
	}

	project, err := api.store.CreateProject(r.Context(), domain.ProjectCreate{
		Name:             req.Name,
		Slug:             slug,
		RepositoryURL:    req.RepositoryURL,
		DefaultNamespace: req.DefaultNamespace,
		Metadata:         req.Metadata,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	api.recordAuditEvent(r.Context(), domain.AuditEventCreate{
		ProjectID:    &project.ID,
		Action:       "project.created",
		ResourceType: "project",
		ResourceID:   &project.ID,
		ResourceName: project.Name,
	})
	writeJSON(w, http.StatusCreated, project)
}

func (api *API) getProject(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(r, "projectID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	project, err := api.store.GetProject(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	api.recordAuditEvent(r.Context(), domain.AuditEventCreate{
		ProjectID:    &project.ID,
		Action:       "project.updated",
		ResourceType: "project",
		ResourceID:   &project.ID,
		ResourceName: project.Name,
	})
	writeJSON(w, http.StatusOK, project)
}

func (api *API) updateProject(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(r, "projectID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	var req updateProjectRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	if req.Name != nil && !validateRequired(*req.Name) {
		writeError(w, http.StatusBadRequest, "name cannot be empty")
		return
	}
	if req.DefaultNamespace != nil && !validateRequired(*req.DefaultNamespace) {
		writeError(w, http.StatusBadRequest, "defaultNamespace cannot be empty")
		return
	}

	var metadata *[]byte
	if req.Metadata != nil {
		raw := []byte(*req.Metadata)
		metadata = &raw
	}
	var defaultTemplateID **uuid.UUID
	if req.DefaultTemplateID.Set {
		defaultTemplateID = &req.DefaultTemplateID.Value
	}

	project, err := api.store.UpdateProject(r.Context(), id, domain.ProjectUpdate{
		Name:              req.Name,
		RepositoryURL:     req.RepositoryURL,
		DefaultNamespace:  req.DefaultNamespace,
		DefaultTemplateID: defaultTemplateID,
		Metadata:          metadata,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, project)
}

func (api *API) deleteProject(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(r, "projectID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	project, err := api.store.GetProject(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if err := api.store.DeleteProject(r.Context(), id); err != nil {
		writeStoreError(w, err)
		return
	}
	api.recordAuditEvent(r.Context(), domain.AuditEventCreate{
		Action:       "project.deleted",
		ResourceType: "project",
		ResourceID:   &id,
		ResourceName: project.Name,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (api *API) getProjectUsage(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(r, "projectID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	usage, err := api.store.GetProjectUsage(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, usage)
}

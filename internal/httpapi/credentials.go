package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/mlhiter/mbox/internal/domain"
)

type projectCredentialRequest struct {
	Name      string                       `json:"name"`
	Slug      string                       `json:"slug"`
	Type      domain.ProjectCredentialType `json:"type"`
	Target    string                       `json:"target"`
	SecretRef domain.SecretRef             `json:"secretRef"`
	Usage     []string                     `json:"usage"`
	Metadata  json.RawMessage              `json:"metadata"`
}

func (api *API) listProjectCredentials(w http.ResponseWriter, r *http.Request) {
	projectID, ok := parseUUIDParam(r, "projectID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	if _, err := api.store.GetProject(r.Context(), projectID); err != nil {
		writeStoreError(w, err)
		return
	}
	credentials, err := api.store.ListProjectCredentials(r.Context(), projectID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": credentials})
}

func (api *API) createProjectCredential(w http.ResponseWriter, r *http.Request) {
	projectID, ok := parseUUIDParam(r, "projectID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	if _, err := api.store.GetProject(r.Context(), projectID); err != nil {
		writeStoreError(w, err)
		return
	}
	var req projectCredentialRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	slug := slugOrName(req.Slug, req.Name)
	if !validateRequired(req.Name) || !validateSlug(slug) || !validProjectCredentialType(req.Type) ||
		!validateRequired(req.SecretRef.Name) {
		writeError(w, http.StatusBadRequest, "name, valid slug, type, and secretRef.name are required")
		return
	}
	credential, err := api.store.CreateProjectCredential(r.Context(), domain.ProjectCredentialCreate{
		ProjectID: projectID,
		Name:      strings.TrimSpace(req.Name),
		Slug:      slug,
		Type:      req.Type,
		Target:    strings.TrimSpace(req.Target),
		SecretRef: domain.SecretRef{
			Name: strings.TrimSpace(req.SecretRef.Name),
			Key:  strings.TrimSpace(req.SecretRef.Key),
		},
		Usage:    normalizeStringList(req.Usage),
		Metadata: req.Metadata,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	api.recordAuditEvent(r.Context(), domain.AuditEventCreate{
		ProjectID:    &credential.ProjectID,
		Action:       "project.credential.created",
		ResourceType: "project-credential",
		ResourceID:   &credential.ID,
		ResourceName: credential.Name,
		Metadata: auditMetadata(map[string]any{
			"type":      credential.Type,
			"target":    credential.Target,
			"secretRef": credential.SecretRef.Name,
		}),
	})
	writeJSON(w, http.StatusCreated, credential)
}

func (api *API) getProjectCredential(w http.ResponseWriter, r *http.Request) {
	credentialID, ok := parseUUIDParam(r, "credentialID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid credential id")
		return
	}
	credential, err := api.store.GetProjectCredential(r.Context(), credentialID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, credential)
}

func (api *API) deleteProjectCredential(w http.ResponseWriter, r *http.Request) {
	credentialID, ok := parseUUIDParam(r, "credentialID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid credential id")
		return
	}
	credential, err := api.store.GetProjectCredential(r.Context(), credentialID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if err := api.store.DeleteProjectCredential(r.Context(), credentialID); err != nil {
		writeStoreError(w, err)
		return
	}
	api.recordAuditEvent(r.Context(), domain.AuditEventCreate{
		ProjectID:    &credential.ProjectID,
		Action:       "project.credential.deleted",
		ResourceType: "project-credential",
		ResourceID:   &credentialID,
		ResourceName: credential.Name,
	})
	w.WriteHeader(http.StatusNoContent)
}

func validProjectCredentialType(value domain.ProjectCredentialType) bool {
	switch value {
	case domain.ProjectCredentialTypeGit,
		domain.ProjectCredentialTypeRegistry,
		domain.ProjectCredentialTypeKubernetes,
		domain.ProjectCredentialTypeSSH,
		domain.ProjectCredentialTypeGeneric:
		return true
	default:
		return false
	}
}

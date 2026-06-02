package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/mlhiter/mbox/internal/domain"
)

type projectMemberRequest struct {
	PrincipalType domain.ProjectMemberPrincipalType `json:"principalType"`
	Principal     string                            `json:"principal"`
	Role          domain.ProjectMemberRole          `json:"role"`
	Metadata      json.RawMessage                   `json:"metadata"`
}

func (api *API) listProjectMembers(w http.ResponseWriter, r *http.Request) {
	projectID, ok := parseUUIDParam(r, "projectID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	if _, err := api.store.GetProject(r.Context(), projectID); err != nil {
		writeStoreError(w, err)
		return
	}
	members, err := api.store.ListProjectMembers(r.Context(), projectID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": members})
}

func (api *API) createProjectMember(w http.ResponseWriter, r *http.Request) {
	projectID, ok := parseUUIDParam(r, "projectID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	if _, err := api.store.GetProject(r.Context(), projectID); err != nil {
		writeStoreError(w, err)
		return
	}
	var req projectMemberRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	principal := strings.TrimSpace(req.Principal)
	if !validProjectMemberPrincipalType(req.PrincipalType) || !validateRequired(principal) || !validProjectMemberRole(req.Role) {
		writeError(w, http.StatusBadRequest, "principalType, principal, and role are required")
		return
	}
	member, err := api.store.CreateProjectMember(r.Context(), domain.ProjectMemberCreate{
		ProjectID:     projectID,
		PrincipalType: req.PrincipalType,
		Principal:     principal,
		Role:          req.Role,
		Metadata:      req.Metadata,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	api.recordAuditEvent(r.Context(), domain.AuditEventCreate{
		ProjectID:    &member.ProjectID,
		Action:       "project.member.created",
		ResourceType: "project-member",
		ResourceID:   &member.ID,
		ResourceName: member.Principal,
		Metadata: auditMetadata(map[string]any{
			"principalType": member.PrincipalType,
			"role":          member.Role,
		}),
	})
	writeJSON(w, http.StatusCreated, member)
}

func (api *API) getProjectMember(w http.ResponseWriter, r *http.Request) {
	memberID, ok := parseUUIDParam(r, "memberID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid member id")
		return
	}
	member, err := api.store.GetProjectMember(r.Context(), memberID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, member)
}

func (api *API) deleteProjectMember(w http.ResponseWriter, r *http.Request) {
	memberID, ok := parseUUIDParam(r, "memberID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid member id")
		return
	}
	member, err := api.store.GetProjectMember(r.Context(), memberID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if err := api.store.DeleteProjectMember(r.Context(), memberID); err != nil {
		writeStoreError(w, err)
		return
	}
	api.recordAuditEvent(r.Context(), domain.AuditEventCreate{
		ProjectID:    &member.ProjectID,
		Action:       "project.member.deleted",
		ResourceType: "project-member",
		ResourceID:   &memberID,
		ResourceName: member.Principal,
		Metadata: auditMetadata(map[string]any{
			"principalType": member.PrincipalType,
			"role":          member.Role,
		}),
	})
	w.WriteHeader(http.StatusNoContent)
}

func validProjectMemberPrincipalType(value domain.ProjectMemberPrincipalType) bool {
	switch value {
	case domain.ProjectMemberPrincipalTypeUser,
		domain.ProjectMemberPrincipalTypeServiceAccount,
		domain.ProjectMemberPrincipalTypeAutomation:
		return true
	default:
		return false
	}
}

func validProjectMemberRole(value domain.ProjectMemberRole) bool {
	switch value {
	case domain.ProjectMemberRoleOwner,
		domain.ProjectMemberRoleOperator,
		domain.ProjectMemberRoleViewer:
		return true
	default:
		return false
	}
}

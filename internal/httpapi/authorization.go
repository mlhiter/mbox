package httpapi

import (
	"net/http"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/mlhiter/mbox/internal/domain"
)

const (
	projectAuthorizationActionProjectView      = "project.view"
	projectAuthorizationActionProjectManage    = "project.manage"
	projectAuthorizationActionSandboxLaunch    = "sandbox.launch"
	projectAuthorizationActionRuntimeOperate   = "runtime.operate"
	projectAuthorizationActionArtifactWrite    = "artifact.write"
	projectAuthorizationActionPolicyManage     = "policy.manage"
	projectAuthorizationActionCredentialManage = "credential.manage"
	projectAuthorizationActionMemberManage     = "member.manage"

	projectAuthorizationEvaluationAllowed        = "allowed"
	projectAuthorizationEvaluationDenied         = "denied"
	projectAuthorizationEvaluationNotEnforceable = "not_enforceable"
)

var projectAuthorizationActions = []string{
	projectAuthorizationActionProjectView,
	projectAuthorizationActionProjectManage,
	projectAuthorizationActionSandboxLaunch,
	projectAuthorizationActionRuntimeOperate,
	projectAuthorizationActionArtifactWrite,
	projectAuthorizationActionPolicyManage,
	projectAuthorizationActionCredentialManage,
	projectAuthorizationActionMemberManage,
}

var starterEnforcedProjectAuthorizationActions = []string{
	projectAuthorizationActionSandboxLaunch,
	projectAuthorizationActionRuntimeOperate,
	projectAuthorizationActionArtifactWrite,
	projectAuthorizationActionPolicyManage,
	projectAuthorizationActionCredentialManage,
	projectAuthorizationActionMemberManage,
}

type ProjectAuthorizationDecision struct {
	ProjectID        uuid.UUID                  `json:"projectId"`
	Action           string                     `json:"action"`
	Allowed          bool                       `json:"allowed"`
	Enforced         bool                       `json:"enforced"`
	Evaluation       string                     `json:"evaluation"`
	RequiredRoles    []domain.ProjectMemberRole `json:"requiredRoles"`
	Caller           CallerInfo                 `json:"caller"`
	MatchedMember    *domain.ProjectMember      `json:"matchedMember,omitempty"`
	MemberCount      int                        `json:"memberCount"`
	AvailableActions []string                   `json:"availableActions"`
	Notes            []string                   `json:"notes"`
}

func (api *API) getProjectAuthorization(w http.ResponseWriter, r *http.Request) {
	projectID, ok := parseUUIDParam(r, "projectID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	if _, err := api.store.GetProject(r.Context(), projectID); err != nil {
		writeStoreError(w, err)
		return
	}
	action := strings.TrimSpace(r.URL.Query().Get("action"))
	if action == "" {
		action = projectAuthorizationActionProjectView
	}
	requiredRoles, ok := requiredRolesForProjectAuthorization(action)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid authorization action")
		return
	}
	members, err := api.store.ListProjectMembers(r.Context(), projectID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, api.projectAuthorizationDecision(r, projectID, action, requiredRoles, members))
}

func (api *API) projectAuthorizationDecision(r *http.Request, projectID uuid.UUID, action string, requiredRoles []domain.ProjectMemberRole, members []domain.ProjectMember) ProjectAuthorizationDecision {
	caller := api.callerInfo(r)
	enforced := api.projectAuthorizationActionEnforced(action)
	decision := ProjectAuthorizationDecision{
		ProjectID:        projectID,
		Action:           action,
		Allowed:          false,
		Enforced:         enforced,
		Evaluation:       projectAuthorizationEvaluationNotEnforceable,
		RequiredRoles:    requiredRoles,
		Caller:           caller,
		MemberCount:      len(members),
		AvailableActions: append([]string{}, projectAuthorizationActions...),
		Notes: []string{
			api.projectAuthorizationEnforcementNote(action),
			"the current caller is not a trusted project identity",
			"project member roles are registry records for future authorization",
		},
	}
	if len(members) == 0 {
		decision.Notes = append(decision.Notes, "project has no member role records")
	}
	if !caller.RBACTrusted {
		if enforced {
			decision.Evaluation = projectAuthorizationEvaluationDenied
			decision.Notes = []string{
				"caller is not a trusted project identity for this enforced action",
				"use an explicitly enabled trusted principal provider with a matching project member role",
			}
		}
		return decision
	}
	callerPrincipalType, ok := projectMemberPrincipalTypeForCaller(caller.PrincipalType)
	if !ok {
		decision.Evaluation = projectAuthorizationEvaluationDenied
		decision.Notes = append(decision.Notes, "caller principal type cannot be matched to project member records")
		return decision
	}
	for _, member := range members {
		if member.PrincipalType != callerPrincipalType || member.Principal != caller.Principal {
			continue
		}
		memberCopy := member
		decision.MatchedMember = &memberCopy
		if slices.Contains(requiredRoles, member.Role) {
			decision.Allowed = true
			decision.Evaluation = projectAuthorizationEvaluationAllowed
			decision.Notes = []string{api.projectAuthorizationAllowedNote(action)}
			return decision
		}
		decision.Evaluation = projectAuthorizationEvaluationDenied
		decision.Notes = []string{"caller matches a project member, but the role is insufficient for this action"}
		return decision
	}
	decision.Evaluation = projectAuthorizationEvaluationDenied
	decision.Notes = append(decision.Notes, "no project member record matches the trusted caller")
	return decision
}

func (api *API) enforceProjectAuthorization(r *http.Request, projectID uuid.UUID, action string) error {
	requiredRoles, ok := requiredRolesForProjectAuthorization(action)
	if !ok {
		return policyDeny("invalid project authorization action")
	}
	if !api.projectAuthorizationActionEnforced(action) {
		return nil
	}
	members, err := api.store.ListProjectMembers(r.Context(), projectID)
	if err != nil {
		return err
	}
	decision := api.projectAuthorizationDecision(r, projectID, action, requiredRoles, members)
	if !decision.Enforced || decision.Allowed {
		return nil
	}
	reason := "project RBAC denied"
	if len(decision.Notes) > 0 && strings.TrimSpace(decision.Notes[0]) != "" {
		reason = "project RBAC denied: " + decision.Notes[0]
	}
	return policyDenyWithMetadata(reason, projectAuthorizationPolicyDeniedMetadata(decision))
}

func projectAuthorizationPolicyDeniedMetadata(decision ProjectAuthorizationDecision) map[string]any {
	if decision.MatchedMember == nil {
		return nil
	}
	return map[string]any{
		"matchedMemberId":            decision.MatchedMember.ID.String(),
		"matchedMemberPrincipalType": decision.MatchedMember.PrincipalType,
		"matchedMemberPrincipal":     decision.MatchedMember.Principal,
		"matchedMemberRole":          decision.MatchedMember.Role,
	}
}

func (api *API) projectAuthorizationActionEnforced(action string) bool {
	return api.projectRBAC.EnforcementEnabled && slices.Contains(starterEnforcedProjectAuthorizationActions, action)
}

func enforcedProjectAuthorizationActions(enabled bool) []string {
	if !enabled {
		return []string{}
	}
	return append([]string{}, starterEnforcedProjectAuthorizationActions...)
}

func (api *API) projectAuthorizationEnforcementNote(action string) string {
	if api.projectAuthorizationActionEnforced(action) {
		return "route-level project RBAC is enforced for this action"
	}
	if api.projectRolesEnforced() {
		return "route-level project RBAC is enabled only for sandbox.launch, runtime.operate, artifact.write, policy.manage, credential.manage, and member.manage"
	}
	return "route-level project RBAC is not enforced yet"
}

func (api *API) projectAuthorizationAllowedNote(action string) string {
	if api.projectAuthorizationActionEnforced(action) {
		return "caller matches a project member role and this action is route-enforced"
	}
	return "caller matches a project member role for this action"
}

func requiredRolesForProjectAuthorization(action string) ([]domain.ProjectMemberRole, bool) {
	switch action {
	case projectAuthorizationActionProjectView:
		return []domain.ProjectMemberRole{
			domain.ProjectMemberRoleOwner,
			domain.ProjectMemberRoleOperator,
			domain.ProjectMemberRoleViewer,
		}, true
	case projectAuthorizationActionSandboxLaunch,
		projectAuthorizationActionRuntimeOperate,
		projectAuthorizationActionArtifactWrite:
		return []domain.ProjectMemberRole{
			domain.ProjectMemberRoleOwner,
			domain.ProjectMemberRoleOperator,
		}, true
	case projectAuthorizationActionProjectManage,
		projectAuthorizationActionPolicyManage,
		projectAuthorizationActionCredentialManage,
		projectAuthorizationActionMemberManage:
		return []domain.ProjectMemberRole{domain.ProjectMemberRoleOwner}, true
	default:
		return nil, false
	}
}

func projectMemberPrincipalTypeForCaller(callerType string) (domain.ProjectMemberPrincipalType, bool) {
	switch callerType {
	case string(domain.ProjectMemberPrincipalTypeUser):
		return domain.ProjectMemberPrincipalTypeUser, true
	case string(domain.ProjectMemberPrincipalTypeServiceAccount):
		return domain.ProjectMemberPrincipalTypeServiceAccount, true
	case string(domain.ProjectMemberPrincipalTypeAutomation):
		return domain.ProjectMemberPrincipalTypeAutomation, true
	default:
		return "", false
	}
}

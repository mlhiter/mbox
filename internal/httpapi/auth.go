package httpapi

import (
	"net/http"
	"strings"

	"github.com/mlhiter/mbox/internal/domain"
)

const (
	callerModeAnonymous       = "anonymous"
	callerModeSharedToken     = "shared_token"
	callerModeTrustedHeader   = "trusted_header"
	callerPrincipalAnonymous  = "anonymous"
	callerPrincipalShared     = "shared-token"
	callerPrincipalTypeAnon   = "anonymous"
	callerPrincipalTypeShared = "shared_token"
)

type TrustedPrincipalHeaderOptions struct {
	Enabled             bool
	PrincipalHeader     string
	PrincipalTypeHeader string
}

type CallerInfo struct {
	Authenticated          bool     `json:"authenticated"`
	AuthenticationRequired bool     `json:"authenticationRequired"`
	Mode                   string   `json:"mode"`
	PrincipalType          string   `json:"principalType"`
	Principal              string   `json:"principal"`
	RBACTrusted            bool     `json:"rbacTrusted"`
	ProjectRolesEnforced   bool     `json:"projectRolesEnforced"`
	Notes                  []string `json:"notes"`
}

func (api *API) getCaller(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, api.callerInfo(r))
}

func (api *API) callerInfo(r *http.Request) CallerInfo {
	if caller, ok := api.trustedHeaderCallerInfo(r); ok {
		return caller
	}
	rolesEnforced := api.projectRolesEnforced()
	if api.apiToken == "" {
		return CallerInfo{
			Authenticated:          false,
			AuthenticationRequired: false,
			Mode:                   callerModeAnonymous,
			PrincipalType:          callerPrincipalTypeAnon,
			Principal:              callerPrincipalAnonymous,
			RBACTrusted:            false,
			ProjectRolesEnforced:   rolesEnforced,
			Notes: []string{
				"local API token authentication is disabled",
				"caller is not a trusted RBAC identity",
				api.projectRoleEnforcementNote(),
			},
		}
	}
	return CallerInfo{
		Authenticated:          true,
		AuthenticationRequired: true,
		Mode:                   callerModeSharedToken,
		PrincipalType:          callerPrincipalTypeShared,
		Principal:              callerPrincipalShared,
		RBACTrusted:            false,
		ProjectRolesEnforced:   rolesEnforced,
		Notes: []string{
			"request matched the configured shared bearer token",
			"shared token is not a user or project RBAC identity",
			api.projectRoleEnforcementNote(),
		},
	}
}

func (api *API) trustedHeaderCallerInfo(r *http.Request) (CallerInfo, bool) {
	if r == nil || !api.principalHeaders.Enabled {
		return CallerInfo{}, false
	}
	principal := strings.TrimSpace(r.Header.Get(api.principalHeaders.PrincipalHeader))
	principalType := strings.TrimSpace(r.Header.Get(api.principalHeaders.PrincipalTypeHeader))
	if principal == "" || principalType == "" || !validCallerPrincipalType(principalType) {
		return CallerInfo{}, false
	}
	return CallerInfo{
		Authenticated:          true,
		AuthenticationRequired: api.apiToken != "",
		Mode:                   callerModeTrustedHeader,
		PrincipalType:          principalType,
		Principal:              principal,
		RBACTrusted:            true,
		ProjectRolesEnforced:   api.projectRolesEnforced(),
		Notes: []string{
			"request included trusted principal headers",
			"caller can be matched to project member records for authorization preflight and enabled project RBAC actions",
			api.projectRoleEnforcementNote(),
		},
	}, true
}

func (api *API) projectRolesEnforced() bool {
	return api.projectRBAC.EnforcementEnabled
}

func (api *API) projectRoleEnforcementNote() string {
	if api.projectRolesEnforced() {
		return "project member roles are enforced for sandbox.launch, runtime.operate, artifact.write, policy.manage, and credential.manage starter routes only"
	}
	return "project member roles are registered but not enforced for route authorization"
}

func normalizeTrustedPrincipalHeaderOptions(options TrustedPrincipalHeaderOptions) TrustedPrincipalHeaderOptions {
	options.PrincipalHeader = strings.TrimSpace(options.PrincipalHeader)
	options.PrincipalTypeHeader = strings.TrimSpace(options.PrincipalTypeHeader)
	if options.PrincipalHeader == "" {
		options.PrincipalHeader = "X-Mbox-Principal"
	}
	if options.PrincipalTypeHeader == "" {
		options.PrincipalTypeHeader = "X-Mbox-Principal-Type"
	}
	return options
}

func validCallerPrincipalType(value string) bool {
	switch domain.ProjectMemberPrincipalType(value) {
	case domain.ProjectMemberPrincipalTypeUser,
		domain.ProjectMemberPrincipalTypeServiceAccount,
		domain.ProjectMemberPrincipalTypeAutomation:
		return true
	default:
		return false
	}
}

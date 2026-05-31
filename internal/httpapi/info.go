package httpapi

import "net/http"

const (
	currentAPIVersion = "v1alpha1"
	defaultVersion    = "0.1.0-dev"
)

type APIInfo struct {
	Name                   string        `json:"name"`
	APIVersion             string        `json:"apiVersion"`
	ServerVersion          string        `json:"serverVersion"`
	RuntimeController      RuntimeInfo   `json:"runtimeController"`
	RuntimeAccess          RuntimeInfo   `json:"runtimeAccess"`
	ArtifactContent        ArtifactInfo  `json:"artifactContent"`
	Capabilities           []string      `json:"capabilities"`
	Compatibility          Compatibility `json:"compatibility"`
	AuthenticationRequired bool          `json:"authenticationRequired"`
}

type RuntimeInfo struct {
	Enabled bool   `json:"enabled"`
	Adapter string `json:"adapter,omitempty"`
}

type ArtifactInfo struct {
	RetainedContentEnabled bool   `json:"retainedContentEnabled"`
	StorageProvider        string `json:"storageProvider"`
	MaxBytes               int64  `json:"maxBytes"`
}

type Compatibility struct {
	MinimumCLIAPIVersion string `json:"minimumCliApiVersion"`
	MinimumSDKAPIVersion string `json:"minimumSdkApiVersion"`
}

type InfoOptions struct {
	ServerVersion            string
	RuntimeControllerEnabled bool
	RuntimeAccessEnabled     bool
	RuntimeAdapter           string
	ArtifactStorageProvider  string
	AuthenticationRequired   bool
}

func (api *API) getInfo(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, api.info)
}

func buildAPIInfo(options InfoOptions) APIInfo {
	version := options.ServerVersion
	if version == "" {
		version = defaultVersion
	}
	runtimeAdapter := options.RuntimeAdapter
	if runtimeAdapter == "" && (options.RuntimeControllerEnabled || options.RuntimeAccessEnabled) {
		runtimeAdapter = "agent-sandbox"
	}
	artifactProvider := options.ArtifactStorageProvider
	if artifactProvider == "" {
		artifactProvider = "postgres"
	}
	return APIInfo{
		Name:          "mbox",
		APIVersion:    currentAPIVersion,
		ServerVersion: version,
		RuntimeController: RuntimeInfo{
			Enabled: options.RuntimeControllerEnabled,
			Adapter: runtimeAdapter,
		},
		RuntimeAccess: RuntimeInfo{
			Enabled: options.RuntimeAccessEnabled,
			Adapter: runtimeAdapter,
		},
		ArtifactContent: ArtifactInfo{
			RetainedContentEnabled: true,
			StorageProvider:        artifactProvider,
			MaxBytes:               maxArtifactContentBytes,
		},
		Capabilities: []string{
			"projects",
			"openapi",
			"project-usage",
			"audit-events",
			"audit-attribution",
			"project-policies",
			"project-quota-policies",
			"project-credential-references",
			"templates",
			"template-validation-runs",
			"boundary-summaries",
			"sandboxes",
			"sandbox-lifecycle",
			"runtime-sessions",
			"execution-tasks",
			"task-events",
			"preview-ports",
			"artifacts",
			"artifact-retained-content",
			"artifact-client-upload",
			"lifecycle-ttl",
			"project-delete-cleanup-guard",
			"runtime-orphan-audit",
			"runtime-orphan-cleanup",
		},
		Compatibility: Compatibility{
			MinimumCLIAPIVersion: currentAPIVersion,
			MinimumSDKAPIVersion: currentAPIVersion,
		},
		AuthenticationRequired: options.AuthenticationRequired,
	}
}

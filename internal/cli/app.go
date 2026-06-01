package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

type Streams struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

type App struct {
	streams Streams
	getenv  func(string) string
	homeDir func() (string, error)
}

func NewApp(streams Streams) *App {
	if streams.Stdin == nil {
		streams.Stdin = os.Stdin
	}
	if streams.Stdout == nil {
		streams.Stdout = os.Stdout
	}
	if streams.Stderr == nil {
		streams.Stderr = os.Stderr
	}
	return &App{
		streams: streams,
		getenv:  os.Getenv,
		homeDir: os.UserHomeDir,
	}
}

func (a *App) Run(ctx context.Context, args []string) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	config := globalConfig{
		APIURL:      a.getenv("MBOX_API_URL"),
		Token:       a.getenv("MBOX_TOKEN"),
		RequestID:   a.getenv("MBOX_REQUEST_ID"),
		AuditActor:  a.getenv("MBOX_AUDIT_ACTOR"),
		AuditSource: a.getenv("MBOX_AUDIT_SOURCE"),
		Context:     a.getenv("MBOX_CONTEXT"),
		ConfigPath:  a.getenv("MBOX_CONFIG"),
	}
	args, err := parseGlobalFlags(args, &config)
	if err != nil {
		return err
	}
	if len(args) == 0 {
		a.usage()
		return flag.ErrHelp
	}
	if args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		a.usage()
		return nil
	}
	if args[0] == "context" || args[0] == "contexts" {
		return a.runContext(ctx, config, args[1:])
	}
	if err := a.applyContext(&config); err != nil {
		return err
	}
	if config.APIURL == "" {
		config.APIURL = defaultAPIURL
	}

	client, err := NewClient(config.APIURL, config.Token)
	if err != nil {
		return err
	}
	client.RequestID = strings.TrimSpace(config.RequestID)
	client.AuditActor = strings.TrimSpace(config.AuditActor)
	client.AuditSource = strings.TrimSpace(config.AuditSource)
	return a.runCommand(ctx, client, config, args)
}

func (a *App) runCommand(ctx context.Context, client *Client, config globalConfig, args []string) error {
	switch args[0] {
	case "health":
		return a.get(ctx, client, "/healthz")
	case "info":
		return a.get(ctx, client, "/v1/info")
	case "compat":
		return a.runCompat(ctx, client, args[1:])
	case "openapi":
		return a.get(ctx, client, "/v1/openapi.json")
	case "runtime":
		return a.runRuntime(ctx, client, args[1:])
	case "audit-events":
		return a.runAuditEvents(ctx, client, args[1:], "")
	case "project", "projects":
		return a.runProject(ctx, client, args[1:])
	case "template", "templates":
		return a.runTemplate(ctx, client, args[1:])
	case "sandbox", "sandboxes":
		return a.runSandbox(ctx, client, args[1:])
	case "session", "sessions":
		return a.runSession(ctx, client, args[1:])
	case "task", "tasks":
		return a.runTask(ctx, client, args[1:])
	case "artifact", "artifacts":
		return a.runArtifact(ctx, client, args[1:])
	case "credential", "credentials":
		return a.runCredential(ctx, client, args[1:])
	case "logs":
		if len(args) != 2 {
			return usageError("usage: mbox logs <sandbox-id>")
		}
		return a.get(ctx, client, "/v1/sandboxes/"+url.PathEscape(args[1])+"/logs")
	case "ports":
		if len(args) != 2 {
			return usageError("usage: mbox ports <sandbox-id>")
		}
		return a.get(ctx, client, "/v1/sandboxes/"+url.PathEscape(args[1])+"/ports")
	case "terminal":
		return a.runTerminal(ctx, client, args[1:])
	default:
		return usageError(fmt.Sprintf("unknown command %q", args[0]))
	}
}

func (a *App) usage() {
	fmt.Fprintln(a.streams.Stderr, `Usage: mbox [--context NAME] [--config PATH] [--api-url URL] [--token TOKEN] [--request-id ID] [--audit-actor ACTOR] [--audit-source SOURCE] <command> [args]

Commands:
  health
  info
  compat [--client-api-version VERSION] [--require-capability CAPABILITY]
  context current|list
  context set NAME --api-url URL [--token TOKEN|--token-env ENV] [--audit-actor ACTOR] [--audit-source SOURCE] [--current]
  context use NAME
  context remove NAME
  openapi
  runtime resources [--namespace NAMESPACE] [--kind KIND] [--summary]
  runtime orphans [--namespace NAMESPACE] [--kind KIND]
  runtime cleanup-orphan --adapter ADAPTER --kind KIND --namespace NAMESPACE --name NAME --reason REASON --confirm delete-orphan-runtime-resource
  audit-events [--project-id PROJECT] [--action ACTION] [--resource-type TYPE] [--resource-id ID] [--actor ACTOR] [--source SOURCE] [--filter-request-id ID] [--operation OPERATION] [--since RFC3339] [--until RFC3339] [--limit N]
  projects list
  projects create --name NAME --namespace NAMESPACE [--slug SLUG]
  projects get <project-id>
  projects usage <project-id>
  projects audit-events <project-id> [--action ACTION] [--resource-type TYPE] [--resource-id ID] [--actor ACTOR] [--source SOURCE] [--filter-request-id ID] [--operation OPERATION] [--since RFC3339] [--until RFC3339] [--limit N]
  projects policy <project-id>
  projects set-policy <project-id> --enforcement disabled|enforced [--allowed-image-prefix PREFIX] [--allowed-service-account NAME] [--allowed-secret-ref NAME]
  projects quota-policy <project-id>
  projects set-quota-policy <project-id> --enforcement disabled|enforced [--max-active-sandboxes N] [--max-retained-artifact-bytes N]
  projects credentials <project-id>
  projects add-credential <project-id> --name NAME --type git|registry|kubernetes|ssh|generic --secret-ref NAME [--secret-key KEY]
  projects delete <project-id>
  templates list [--project-id PROJECT]
  templates get <template-id>
  templates boundary <template-id> [--project-id PROJECT]
  templates validate <template-id> --project-id PROJECT [--name NAME]
  templates decide-validation <template-id> <sandbox-id> --status passed|failed
  sandboxes list [--project-id PROJECT]
  sandboxes create --project-id PROJECT --name NAME [--template-id TEMPLATE]
  sandboxes get <sandbox-id>
  sandboxes boundary <sandbox-id>
  sandboxes start|stop|delete <sandbox-id>
  sessions list <sandbox-id>
  sessions create <sandbox-id> --type TYPE [--client CLIENT]
  sessions get|end <session-id>
  tasks list <sandbox-id>
  tasks create <sandbox-id> --arg sh --arg -lc --arg 'echo ok' [--timeout 60]
  tasks get|cancel|watch|wait <task-id>
  artifacts list <sandbox-id>
  artifacts get|capture|content <artifact-id>
  artifacts upload <artifact-id> (--file PATH|--stdin) [--content-type TYPE]
  credentials get|delete <credential-id>
  logs <sandbox-id>
  ports <sandbox-id>
  terminal <sandbox-id> [--shell sh|bash]

Environment:
  MBOX_API_URL        API base URL, default http://127.0.0.1:18080
  MBOX_TOKEN          optional bearer token for authenticated APIs
  MBOX_REQUEST_ID     optional request correlation id sent as X-Mbox-Request-ID
  MBOX_CONTEXT        optional context name from the CLI config file
  MBOX_CONFIG         optional config file path, default ~/.mbox/config.json
  MBOX_AUDIT_ACTOR    optional client-supplied audit actor label
  MBOX_AUDIT_SOURCE   optional client-supplied audit source label`)
}

func (a *App) runCompat(ctx context.Context, client *Client, args []string) error {
	fs := flag.NewFlagSet("compat", flag.ContinueOnError)
	fs.SetOutput(a.streams.Stderr)
	clientAPIVersion := fs.String("client-api-version", currentClientAPIVersion, "")
	var requiredCapabilities stringListFlag
	fs.Var(&requiredCapabilities, "require-capability", "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return usageError("usage: mbox compat [--client-api-version VERSION] [--require-capability CAPABILITY]")
	}
	var info apiInfo
	if err := client.JSON(ctx, http.MethodGet, "/v1/info", nil, &info); err != nil {
		return err
	}
	result := CheckCLICompatibility(info, strings.TrimSpace(*clientAPIVersion), []string(requiredCapabilities))
	if err := WriteJSON(a.streams.Stdout, result); err != nil {
		return err
	}
	if !result.OK {
		return usageError(result.Message)
	}
	return nil
}

func (a *App) runRuntime(ctx context.Context, client *Client, args []string) error {
	if len(args) == 0 {
		return usageError("usage: mbox runtime resources|orphans|cleanup-orphan")
	}
	switch args[0] {
	case "resource", "resources":
		fs := flag.NewFlagSet("runtime resources", flag.ContinueOnError)
		fs.SetOutput(a.streams.Stderr)
		namespace := fs.String("namespace", "", "")
		kind := fs.String("kind", "", "")
		summaryOnly := fs.Bool("summary", false, "")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return usageError("usage: mbox runtime resources [--namespace NAMESPACE] [--kind KIND] [--summary]")
		}
		path := "/v1/runtime/resources"
		values := url.Values{}
		if strings.TrimSpace(*namespace) != "" {
			values.Set("namespace", strings.TrimSpace(*namespace))
		}
		if strings.TrimSpace(*kind) != "" {
			values.Set("kind", strings.TrimSpace(*kind))
		}
		if encoded := values.Encode(); encoded != "" {
			path += "?" + encoded
		}
		if *summaryOnly {
			var response map[string]any
			if err := client.JSON(ctx, http.MethodGet, path, nil, &response); err != nil {
				return err
			}
			summary, ok := response["summary"]
			if !ok {
				return fmt.Errorf("runtime resources response did not include summary")
			}
			return WriteJSON(a.streams.Stdout, summary)
		}
		return a.get(ctx, client, path)
	case "orphan", "orphans":
		fs := flag.NewFlagSet("runtime orphans", flag.ContinueOnError)
		fs.SetOutput(a.streams.Stderr)
		namespace := fs.String("namespace", "", "")
		kind := fs.String("kind", "", "")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return usageError("usage: mbox runtime orphans [--namespace NAMESPACE] [--kind KIND]")
		}
		path := "/v1/runtime/orphans"
		values := url.Values{}
		if strings.TrimSpace(*namespace) != "" {
			values.Set("namespace", strings.TrimSpace(*namespace))
		}
		if strings.TrimSpace(*kind) != "" {
			values.Set("kind", strings.TrimSpace(*kind))
		}
		if encoded := values.Encode(); encoded != "" {
			path += "?" + encoded
		}
		return a.get(ctx, client, path)
	case "cleanup-orphan":
		fs := flag.NewFlagSet("runtime cleanup-orphan", flag.ContinueOnError)
		fs.SetOutput(a.streams.Stderr)
		adapter := fs.String("adapter", "agent-sandbox", "")
		kind := fs.String("kind", "", "")
		namespace := fs.String("namespace", "", "")
		name := fs.String("name", "", "")
		reason := fs.String("reason", "", "")
		confirm := fs.String("confirm", "", "")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 || strings.TrimSpace(*kind) == "" || strings.TrimSpace(*namespace) == "" ||
			strings.TrimSpace(*name) == "" || strings.TrimSpace(*reason) == "" || strings.TrimSpace(*confirm) == "" {
			return usageError("usage: mbox runtime cleanup-orphan --adapter ADAPTER --kind KIND --namespace NAMESPACE --name NAME --reason REASON --confirm delete-orphan-runtime-resource")
		}
		payload := map[string]any{
			"resource": map[string]string{
				"adapter":   strings.TrimSpace(*adapter),
				"kind":      strings.TrimSpace(*kind),
				"namespace": strings.TrimSpace(*namespace),
				"name":      strings.TrimSpace(*name),
			},
			"reason":       strings.TrimSpace(*reason),
			"deleteOrphan": true,
			"confirm":      strings.TrimSpace(*confirm),
		}
		return a.post(ctx, client, "/v1/runtime/orphans/cleanup", payload)
	default:
		return usageError("usage: mbox runtime resources|orphans|cleanup-orphan")
	}
}

func (a *App) runAuditEvents(ctx context.Context, client *Client, args []string, projectID string) error {
	fs := flag.NewFlagSet("audit-events", flag.ContinueOnError)
	fs.SetOutput(a.streams.Stderr)
	projectIDFlag := fs.String("project-id", projectID, "")
	action := fs.String("action", "", "")
	resourceType := fs.String("resource-type", "", "")
	resourceID := fs.String("resource-id", "", "")
	actor := fs.String("actor", "", "")
	source := fs.String("source", "", "")
	filterRequestID := fs.String("filter-request-id", "", "")
	operation := fs.String("operation", "", "")
	since := fs.String("since", "", "")
	until := fs.String("until", "", "")
	limit := fs.Int("limit", 0, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		if projectID != "" {
			return usageError("usage: mbox projects audit-events <project-id> [--action ACTION] [--resource-type TYPE] [--resource-id ID] [--actor ACTOR] [--source SOURCE] [--filter-request-id ID] [--operation OPERATION] [--since RFC3339] [--until RFC3339] [--limit N]")
		}
		return usageError("usage: mbox audit-events [--project-id PROJECT] [--action ACTION] [--resource-type TYPE] [--resource-id ID] [--actor ACTOR] [--source SOURCE] [--filter-request-id ID] [--operation OPERATION] [--since RFC3339] [--until RFC3339] [--limit N]")
	}
	path := "/v1/audit-events"
	values := url.Values{}
	if strings.TrimSpace(*projectIDFlag) != "" {
		values.Set("projectId", strings.TrimSpace(*projectIDFlag))
	}
	if strings.TrimSpace(*action) != "" {
		values.Set("action", strings.TrimSpace(*action))
	}
	if strings.TrimSpace(*resourceType) != "" {
		values.Set("resourceType", strings.TrimSpace(*resourceType))
	}
	if strings.TrimSpace(*resourceID) != "" {
		values.Set("resourceId", strings.TrimSpace(*resourceID))
	}
	if strings.TrimSpace(*actor) != "" {
		values.Set("actor", strings.TrimSpace(*actor))
	}
	if strings.TrimSpace(*source) != "" {
		values.Set("source", strings.TrimSpace(*source))
	}
	if strings.TrimSpace(*filterRequestID) != "" {
		values.Set("requestId", strings.TrimSpace(*filterRequestID))
	}
	if strings.TrimSpace(*operation) != "" {
		values.Set("operation", strings.TrimSpace(*operation))
	}
	if strings.TrimSpace(*since) != "" {
		values.Set("since", strings.TrimSpace(*since))
	}
	if strings.TrimSpace(*until) != "" {
		values.Set("until", strings.TrimSpace(*until))
	}
	if *limit > 0 {
		values.Set("limit", strconv.Itoa(*limit))
	}
	if projectID != "" {
		path = "/v1/projects/" + url.PathEscape(projectID) + "/audit-events"
		values.Del("projectId")
	}
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return a.get(ctx, client, path)
}

func (a *App) get(ctx context.Context, client *Client, path string) error {
	var out any
	if err := client.JSON(ctx, http.MethodGet, path, nil, &out); err != nil {
		return err
	}
	return WriteJSON(a.streams.Stdout, out)
}

func (a *App) post(ctx context.Context, client *Client, path string, payload any) error {
	var out any
	if err := client.JSON(ctx, http.MethodPost, path, payload, &out); err != nil {
		return err
	}
	return WriteJSON(a.streams.Stdout, out)
}

func (a *App) delete(ctx context.Context, client *Client, path string) error {
	if err := client.JSON(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return err
	}
	_, err := fmt.Fprintln(a.streams.Stdout, "deleted")
	return err
}

func parseGlobalFlags(args []string, config *globalConfig) ([]string, error) {
	out := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--context":
			i++
			if i >= len(args) {
				return nil, usageError("--context requires a value")
			}
			config.Context = args[i]
		case strings.HasPrefix(arg, "--context="):
			config.Context = strings.TrimPrefix(arg, "--context=")
		case arg == "--config":
			i++
			if i >= len(args) {
				return nil, usageError("--config requires a value")
			}
			config.ConfigPath = args[i]
		case strings.HasPrefix(arg, "--config="):
			config.ConfigPath = strings.TrimPrefix(arg, "--config=")
		case arg == "--api-url":
			i++
			if i >= len(args) {
				return nil, usageError("--api-url requires a value")
			}
			config.APIURL = args[i]
			config.apiURLSet = true
		case strings.HasPrefix(arg, "--api-url="):
			config.APIURL = strings.TrimPrefix(arg, "--api-url=")
			config.apiURLSet = true
		case arg == "--token":
			i++
			if i >= len(args) {
				return nil, usageError("--token requires a value")
			}
			config.Token = args[i]
			config.tokenSet = true
		case strings.HasPrefix(arg, "--token="):
			config.Token = strings.TrimPrefix(arg, "--token=")
			config.tokenSet = true
		case arg == "--request-id":
			i++
			if i >= len(args) {
				return nil, usageError("--request-id requires a value")
			}
			config.RequestID = args[i]
		case strings.HasPrefix(arg, "--request-id="):
			config.RequestID = strings.TrimPrefix(arg, "--request-id=")
		case arg == "--audit-actor":
			i++
			if i >= len(args) {
				return nil, usageError("--audit-actor requires a value")
			}
			config.AuditActor = args[i]
			config.auditActorSet = true
		case strings.HasPrefix(arg, "--audit-actor="):
			config.AuditActor = strings.TrimPrefix(arg, "--audit-actor=")
			config.auditActorSet = true
		case arg == "--audit-source":
			i++
			if i >= len(args) {
				return nil, usageError("--audit-source requires a value")
			}
			config.AuditSource = args[i]
			config.auditSourceSet = true
		case strings.HasPrefix(arg, "--audit-source="):
			config.AuditSource = strings.TrimPrefix(arg, "--audit-source=")
			config.auditSourceSet = true
		default:
			out = append(out, arg)
		}
	}
	return out, nil
}

type globalConfig struct {
	APIURL      string
	Token       string
	RequestID   string
	AuditActor  string
	AuditSource string
	Context     string
	ConfigPath  string

	apiURLSet      bool
	tokenSet       bool
	auditActorSet  bool
	auditSourceSet bool
}

type usageError string

func (e usageError) Error() string { return string(e) }

func stringFlag(fs *flag.FlagSet, name string) *string {
	value := fs.String(name, "", "")
	return value
}

func parseMetadataFlag(fs *flag.FlagSet, raw string) (json.RawMessage, error) {
	if raw == "" {
		return nil, nil
	}
	return ReadJSONObject(raw)
}

func commandFromFlag(raw string) []string {
	return ParseStringList(raw)
}

func parsePositiveDuration(raw string, name string) (time.Duration, error) {
	value, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil {
		return 0, usageError(fmt.Sprintf("%s must be a Go duration such as 500ms or 2m", name))
	}
	if value <= 0 {
		return 0, usageError(fmt.Sprintf("%s must be greater than zero", name))
	}
	return value, nil
}

func isTerminalTaskStatus(value any) bool {
	status, ok := value.(string)
	if !ok {
		return false
	}
	switch status {
	case "succeeded", "failed", "canceled", "timed_out":
		return true
	default:
		return false
	}
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type stringListFlag []string

func (f *stringListFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *stringListFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func (a *App) runProject(ctx context.Context, client *Client, args []string) error {
	if len(args) == 0 {
		return usageError("usage: mbox projects list|create|get|usage|audit-events|policy|set-policy|quota-policy|set-quota-policy|credentials|add-credential|delete")
	}
	switch args[0] {
	case "list":
		return a.get(ctx, client, "/v1/projects")
	case "get":
		if len(args) != 2 {
			return usageError("usage: mbox projects get <project-id>")
		}
		return a.get(ctx, client, "/v1/projects/"+url.PathEscape(args[1]))
	case "usage":
		if len(args) != 2 {
			return usageError("usage: mbox projects usage <project-id>")
		}
		return a.get(ctx, client, "/v1/projects/"+url.PathEscape(args[1])+"/usage")
	case "audit-events":
		if len(args) < 2 {
			return usageError("usage: mbox projects audit-events <project-id> [--action ACTION] [--resource-type TYPE] [--resource-id ID] [--actor ACTOR] [--source SOURCE] [--filter-request-id ID] [--operation OPERATION] [--since RFC3339] [--until RFC3339] [--limit N]")
		}
		return a.runAuditEvents(ctx, client, args[2:], args[1])
	case "policy":
		if len(args) != 2 {
			return usageError("usage: mbox projects policy <project-id>")
		}
		return a.get(ctx, client, "/v1/projects/"+url.PathEscape(args[1])+"/policy")
	case "set-policy":
		if len(args) < 2 {
			return usageError("usage: mbox projects set-policy <project-id> --enforcement disabled|enforced")
		}
		projectID := args[1]
		fs := flag.NewFlagSet("projects set-policy", flag.ContinueOnError)
		fs.SetOutput(a.streams.Stderr)
		enforcement := fs.String("enforcement", "", "")
		var allowedImagePrefixes stringListFlag
		var allowedServiceAccounts stringListFlag
		var allowedSecretRefs stringListFlag
		fs.Var(&allowedImagePrefixes, "allowed-image-prefix", "")
		fs.Var(&allowedServiceAccounts, "allowed-service-account", "")
		fs.Var(&allowedSecretRefs, "allowed-secret-ref", "")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		payload := map[string]any{
			"enforcement":            *enforcement,
			"allowedImagePrefixes":   []string(allowedImagePrefixes),
			"allowedServiceAccounts": []string(allowedServiceAccounts),
			"allowedSecretRefs":      []string(allowedSecretRefs),
		}
		var out any
		if err := client.JSON(ctx, http.MethodPut, "/v1/projects/"+url.PathEscape(projectID)+"/policy", payload, &out); err != nil {
			return err
		}
		return WriteJSON(a.streams.Stdout, out)
	case "quota-policy":
		if len(args) != 2 {
			return usageError("usage: mbox projects quota-policy <project-id>")
		}
		return a.get(ctx, client, "/v1/projects/"+url.PathEscape(args[1])+"/quota-policy")
	case "set-quota-policy":
		if len(args) < 2 {
			return usageError("usage: mbox projects set-quota-policy <project-id> --enforcement disabled|enforced")
		}
		projectID := args[1]
		fs := flag.NewFlagSet("projects set-quota-policy", flag.ContinueOnError)
		fs.SetOutput(a.streams.Stderr)
		enforcement := fs.String("enforcement", "", "")
		maxActiveSandboxes := fs.Int("max-active-sandboxes", -1, "")
		maxRetainedArtifactBytes := fs.Int64("max-retained-artifact-bytes", -1, "")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		payload := map[string]any{
			"enforcement": *enforcement,
		}
		if *maxActiveSandboxes >= 0 {
			payload["maxActiveSandboxes"] = *maxActiveSandboxes
		}
		if *maxRetainedArtifactBytes >= 0 {
			payload["maxRetainedArtifactBytes"] = *maxRetainedArtifactBytes
		}
		var out any
		if err := client.JSON(ctx, http.MethodPut, "/v1/projects/"+url.PathEscape(projectID)+"/quota-policy", payload, &out); err != nil {
			return err
		}
		return WriteJSON(a.streams.Stdout, out)
	case "credentials":
		if len(args) != 2 {
			return usageError("usage: mbox projects credentials <project-id>")
		}
		return a.get(ctx, client, "/v1/projects/"+url.PathEscape(args[1])+"/credentials")
	case "add-credential":
		if len(args) < 2 {
			return usageError("usage: mbox projects add-credential <project-id> --name NAME --type TYPE --secret-ref NAME")
		}
		projectID := args[1]
		fs := flag.NewFlagSet("projects add-credential", flag.ContinueOnError)
		fs.SetOutput(a.streams.Stderr)
		name := fs.String("name", "", "")
		slug := fs.String("slug", "", "")
		credentialType := fs.String("type", "", "")
		target := fs.String("target", "", "")
		secretRef := fs.String("secret-ref", "", "")
		secretKey := fs.String("secret-key", "", "")
		metadata := fs.String("metadata", "", "")
		var usage stringListFlag
		fs.Var(&usage, "usage", "")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		rawMetadata, err := parseMetadataFlag(fs, *metadata)
		if err != nil {
			return err
		}
		payload := map[string]any{
			"name": *name,
			"type": *credentialType,
			"secretRef": map[string]any{
				"name": *secretRef,
				"key":  *secretKey,
			},
			"usage": []string(usage),
		}
		SetNonEmpty(payload, "slug", *slug)
		SetNonEmpty(payload, "target", *target)
		SetRaw(payload, "metadata", rawMetadata)
		return a.post(ctx, client, "/v1/projects/"+url.PathEscape(projectID)+"/credentials", payload)
	case "delete":
		if len(args) != 2 {
			return usageError("usage: mbox projects delete <project-id>")
		}
		return a.delete(ctx, client, "/v1/projects/"+url.PathEscape(args[1]))
	case "create":
		fs := flag.NewFlagSet("projects create", flag.ContinueOnError)
		fs.SetOutput(a.streams.Stderr)
		name := stringFlag(fs, "name")
		slug := stringFlag(fs, "slug")
		namespace := fs.String("namespace", "", "")
		repositoryURL := fs.String("repository-url", "", "")
		metadata := fs.String("metadata", "", "")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		rawMetadata, err := parseMetadataFlag(fs, *metadata)
		if err != nil {
			return err
		}
		payload := map[string]any{
			"name":             *name,
			"defaultNamespace": *namespace,
		}
		SetNonEmpty(payload, "slug", *slug)
		SetNonEmpty(payload, "repositoryUrl", *repositoryURL)
		SetRaw(payload, "metadata", rawMetadata)
		return a.post(ctx, client, "/v1/projects", payload)
	default:
		return usageError("usage: mbox projects list|create|get|usage|audit-events|policy|set-policy|quota-policy|set-quota-policy|credentials|add-credential|delete")
	}
}

func (a *App) runTemplate(ctx context.Context, client *Client, args []string) error {
	if len(args) == 0 {
		return usageError("usage: mbox templates list|get|boundary|validate|decide-validation")
	}
	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("templates list", flag.ContinueOnError)
		fs.SetOutput(a.streams.Stderr)
		projectID := fs.String("project-id", "", "")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		path := "/v1/templates"
		if *projectID != "" {
			path += "?projectId=" + url.QueryEscape(*projectID)
		}
		return a.get(ctx, client, path)
	case "get":
		if len(args) != 2 {
			return usageError("usage: mbox templates get <template-id>")
		}
		return a.get(ctx, client, "/v1/templates/"+url.PathEscape(args[1]))
	case "boundary":
		if len(args) < 2 {
			return usageError("usage: mbox templates boundary <template-id> [--project-id PROJECT]")
		}
		templateID := args[1]
		fs := flag.NewFlagSet("templates boundary", flag.ContinueOnError)
		fs.SetOutput(a.streams.Stderr)
		projectID := fs.String("project-id", "", "")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		path := "/v1/templates/" + url.PathEscape(templateID) + "/boundary"
		if *projectID != "" {
			path += "?projectId=" + url.QueryEscape(*projectID)
		}
		return a.get(ctx, client, path)
	case "validate":
		if len(args) < 2 {
			return usageError("usage: mbox templates validate <template-id> --project-id PROJECT [--name NAME]")
		}
		templateID := args[1]
		fs := flag.NewFlagSet("templates validate", flag.ContinueOnError)
		fs.SetOutput(a.streams.Stderr)
		projectID := fs.String("project-id", "", "")
		name := fs.String("name", "", "")
		metadata := fs.String("metadata", "", "")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		rawMetadata, err := parseMetadataFlag(fs, *metadata)
		if err != nil {
			return err
		}
		payload := map[string]any{}
		SetNonEmpty(payload, "projectId", *projectID)
		SetNonEmpty(payload, "name", *name)
		SetRaw(payload, "metadata", rawMetadata)
		return a.post(ctx, client, "/v1/templates/"+url.PathEscape(templateID)+"/validation-runs", payload)
	case "decide-validation":
		if len(args) < 3 {
			return usageError("usage: mbox templates decide-validation <template-id> <sandbox-id> --status passed|failed")
		}
		templateID := args[1]
		sandboxID := args[2]
		fs := flag.NewFlagSet("templates decide-validation", flag.ContinueOnError)
		fs.SetOutput(a.streams.Stderr)
		status := fs.String("status", "", "")
		if err := fs.Parse(args[3:]); err != nil {
			return err
		}
		payload := map[string]any{"status": *status}
		return a.post(
			ctx,
			client,
			"/v1/templates/"+url.PathEscape(templateID)+"/validation-runs/"+url.PathEscape(sandboxID)+"/decision",
			payload,
		)
	default:
		return usageError("usage: mbox templates list|get|boundary|validate|decide-validation")
	}
}

func (a *App) runSandbox(ctx context.Context, client *Client, args []string) error {
	if len(args) == 0 {
		return usageError("usage: mbox sandboxes list|create|get|start|stop|delete")
	}
	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("sandboxes list", flag.ContinueOnError)
		fs.SetOutput(a.streams.Stderr)
		projectID := fs.String("project-id", "", "")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		path := "/v1/sandboxes"
		if *projectID != "" {
			path += "?projectId=" + url.QueryEscape(*projectID)
		}
		return a.get(ctx, client, path)
	case "get":
		if len(args) != 2 {
			return usageError("usage: mbox sandboxes get <sandbox-id>")
		}
		return a.get(ctx, client, "/v1/sandboxes/"+url.PathEscape(args[1]))
	case "boundary":
		if len(args) != 2 {
			return usageError("usage: mbox sandboxes boundary <sandbox-id>")
		}
		return a.get(ctx, client, "/v1/sandboxes/"+url.PathEscape(args[1])+"/boundary")
	case "delete":
		if len(args) != 2 {
			return usageError("usage: mbox sandboxes delete <sandbox-id>")
		}
		return a.delete(ctx, client, "/v1/sandboxes/"+url.PathEscape(args[1]))
	case "start", "stop":
		if len(args) != 2 {
			return usageError("usage: mbox sandboxes start|stop <sandbox-id>")
		}
		return a.post(ctx, client, "/v1/sandboxes/"+url.PathEscape(args[1])+"/"+args[0], nil)
	case "create":
		fs := flag.NewFlagSet("sandboxes create", flag.ContinueOnError)
		fs.SetOutput(a.streams.Stderr)
		projectID := fs.String("project-id", "", "")
		templateID := fs.String("template-id", "", "")
		name := fs.String("name", "", "")
		slug := fs.String("slug", "", "")
		namespace := fs.String("namespace", "", "")
		serviceAccount := fs.String("service-account", "", "")
		metadata := fs.String("metadata", "", "")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		rawMetadata, err := parseMetadataFlag(fs, *metadata)
		if err != nil {
			return err
		}
		payload := map[string]any{
			"projectId": *projectID,
			"name":      *name,
		}
		SetNonEmpty(payload, "templateId", *templateID)
		SetNonEmpty(payload, "slug", *slug)
		SetNonEmpty(payload, "namespace", *namespace)
		SetNonEmpty(payload, "serviceAccountName", *serviceAccount)
		SetRaw(payload, "metadata", rawMetadata)
		return a.post(ctx, client, "/v1/sandboxes", payload)
	default:
		return usageError("usage: mbox sandboxes list|create|get|start|stop|delete")
	}
}

func (a *App) runSession(ctx context.Context, client *Client, args []string) error {
	if len(args) == 0 {
		return usageError("usage: mbox sessions list|create|get|end")
	}
	switch args[0] {
	case "list":
		if len(args) != 2 {
			return usageError("usage: mbox sessions list <sandbox-id>")
		}
		return a.get(ctx, client, "/v1/sandboxes/"+url.PathEscape(args[1])+"/sessions")
	case "get":
		if len(args) != 2 {
			return usageError("usage: mbox sessions get <session-id>")
		}
		return a.get(ctx, client, "/v1/sessions/"+url.PathEscape(args[1]))
	case "end":
		if len(args) != 2 {
			return usageError("usage: mbox sessions end <session-id>")
		}
		return a.post(ctx, client, "/v1/sessions/"+url.PathEscape(args[1])+"/end", nil)
	case "create":
		if len(args) < 2 {
			return usageError("usage: mbox sessions create <sandbox-id> --type TYPE [--client CLIENT]")
		}
		sandboxID := args[1]
		fs := flag.NewFlagSet("sessions create", flag.ContinueOnError)
		fs.SetOutput(a.streams.Stderr)
		sessionType := fs.String("type", "", "")
		clientName := fs.String("client", "mbox-cli", "")
		metadata := fs.String("metadata", "", "")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		rawMetadata, err := parseMetadataFlag(fs, *metadata)
		if err != nil {
			return err
		}
		payload := map[string]any{
			"type":   *sessionType,
			"client": *clientName,
		}
		SetRaw(payload, "metadata", rawMetadata)
		return a.post(ctx, client, "/v1/sandboxes/"+url.PathEscape(sandboxID)+"/sessions", payload)
	default:
		return usageError("usage: mbox sessions list|create|get|end")
	}
}

func (a *App) runTask(ctx context.Context, client *Client, args []string) error {
	if len(args) == 0 {
		return usageError("usage: mbox tasks list|create|get|cancel|watch|wait")
	}
	switch args[0] {
	case "list":
		if len(args) != 2 {
			return usageError("usage: mbox tasks list <sandbox-id>")
		}
		return a.get(ctx, client, "/v1/sandboxes/"+url.PathEscape(args[1])+"/tasks")
	case "get":
		if len(args) != 2 {
			return usageError("usage: mbox tasks get <task-id>")
		}
		return a.get(ctx, client, "/v1/tasks/"+url.PathEscape(args[1]))
	case "cancel":
		if len(args) != 2 {
			return usageError("usage: mbox tasks cancel <task-id>")
		}
		return a.post(ctx, client, "/v1/tasks/"+url.PathEscape(args[1])+"/cancel", nil)
	case "watch":
		if len(args) != 2 {
			return usageError("usage: mbox tasks watch <task-id>")
		}
		return a.stream(ctx, client, "/v1/tasks/"+url.PathEscape(args[1])+"/events")
	case "wait":
		return a.runTaskWait(ctx, client, args[1:])
	case "create":
		if len(args) < 2 {
			return usageError("usage: mbox tasks create <sandbox-id> --command 'sh -lc echo-ok'")
		}
		sandboxID := args[1]
		fs := flag.NewFlagSet("tasks create", flag.ContinueOnError)
		fs.SetOutput(a.streams.Stderr)
		command := fs.String("command", "", "")
		commandJSON := fs.String("command-json", "", "")
		var commandArgs stringListFlag
		fs.Var(&commandArgs, "arg", "")
		timeoutSeconds := fs.Int("timeout", 60, "")
		metadata := fs.String("metadata", "", "")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		rawMetadata, err := parseMetadataFlag(fs, *metadata)
		if err != nil {
			return err
		}
		parsedCommand, err := parseCommandFlags(commandArgs, *command, *commandJSON)
		if err != nil {
			return err
		}
		payload := map[string]any{
			"command":        parsedCommand,
			"timeoutSeconds": *timeoutSeconds,
		}
		SetRaw(payload, "metadata", rawMetadata)
		return a.post(ctx, client, "/v1/sandboxes/"+url.PathEscape(sandboxID)+"/tasks", payload)
	default:
		return usageError("usage: mbox tasks list|create|get|cancel|watch|wait")
	}
}

func (a *App) runTaskWait(ctx context.Context, client *Client, args []string) error {
	fs := flag.NewFlagSet("tasks wait", flag.ContinueOnError)
	fs.SetOutput(a.streams.Stderr)
	intervalRaw := fs.String("interval", "1500ms", "")
	timeoutRaw := fs.String("timeout", "", "")
	taskID := ""
	parseArgs := args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		taskID = strings.TrimSpace(args[0])
		parseArgs = args[1:]
	}
	if err := fs.Parse(parseArgs); err != nil {
		return err
	}
	if taskID == "" && fs.NArg() == 1 {
		taskID = strings.TrimSpace(fs.Arg(0))
	} else if fs.NArg() != 0 {
		return usageError("usage: mbox tasks wait <task-id> [--interval 1500ms] [--timeout 5m]")
	}
	if taskID == "" {
		return usageError("usage: mbox tasks wait <task-id> [--interval 1500ms] [--timeout 5m]")
	}
	interval, err := parsePositiveDuration(*intervalRaw, "interval")
	if err != nil {
		return err
	}
	var timeout time.Duration
	if strings.TrimSpace(*timeoutRaw) != "" {
		timeout, err = parsePositiveDuration(*timeoutRaw, "timeout")
		if err != nil {
			return err
		}
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	for {
		var task map[string]any
		if err := client.JSON(ctx, http.MethodGet, "/v1/tasks/"+url.PathEscape(taskID), nil, &task); err != nil {
			return err
		}
		if isTerminalTaskStatus(task["status"]) {
			return WriteJSON(a.streams.Stdout, task)
		}
		if err := sleepContext(ctx, interval); err != nil {
			return fmt.Errorf("timed out waiting for task %s", taskID)
		}
	}
}

func (a *App) runArtifact(ctx context.Context, client *Client, args []string) error {
	if len(args) == 0 {
		return usageError("usage: mbox artifacts list|get|capture|content|upload")
	}
	switch args[0] {
	case "list":
		if len(args) != 2 {
			return usageError("usage: mbox artifacts list <sandbox-id>")
		}
		return a.get(ctx, client, "/v1/sandboxes/"+url.PathEscape(args[1])+"/artifacts")
	case "get":
		if len(args) != 2 {
			return usageError("usage: mbox artifacts get <artifact-id>")
		}
		return a.get(ctx, client, "/v1/artifacts/"+url.PathEscape(args[1]))
	case "capture":
		if len(args) != 2 {
			return usageError("usage: mbox artifacts capture <artifact-id>")
		}
		return a.post(ctx, client, "/v1/artifacts/"+url.PathEscape(args[1])+"/capture", nil)
	case "content":
		if len(args) != 2 {
			return usageError("usage: mbox artifacts content <artifact-id>")
		}
		response, err := client.Raw(ctx, http.MethodGet, "/v1/artifacts/"+url.PathEscape(args[1])+"/content", nil)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		_, err = io.Copy(a.streams.Stdout, response.Body)
		return err
	case "upload":
		if len(args) < 2 {
			return usageError("usage: mbox artifacts upload <artifact-id> (--file PATH|--stdin) [--content-type TYPE]")
		}
		artifactID := args[1]
		fs := flag.NewFlagSet("artifacts upload", flag.ContinueOnError)
		fs.SetOutput(a.streams.Stderr)
		filePath := fs.String("file", "", "")
		useStdin := fs.Bool("stdin", false, "")
		contentType := fs.String("content-type", "", "")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if (*filePath == "" && !*useStdin) || (*filePath != "" && *useStdin) {
			return usageError("usage: mbox artifacts upload <artifact-id> (--file PATH|--stdin) [--content-type TYPE]")
		}
		var reader io.Reader
		var file *os.File
		if *useStdin {
			reader = a.streams.Stdin
		} else {
			var err error
			file, err = os.Open(*filePath)
			if err != nil {
				return err
			}
			defer file.Close()
			reader = file
		}
		response, err := client.RawBody(ctx, http.MethodPut, "/v1/artifacts/"+url.PathEscape(artifactID)+"/content", reader, *contentType)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		var out any
		if err := json.NewDecoder(response.Body).Decode(&out); err != nil {
			return err
		}
		return WriteJSON(a.streams.Stdout, out)
	default:
		return usageError("usage: mbox artifacts list|get|capture|content|upload")
	}
}

func (a *App) runCredential(ctx context.Context, client *Client, args []string) error {
	if len(args) == 0 {
		return usageError("usage: mbox credentials get|delete <credential-id>")
	}
	switch args[0] {
	case "get":
		if len(args) != 2 {
			return usageError("usage: mbox credentials get <credential-id>")
		}
		return a.get(ctx, client, "/v1/credentials/"+url.PathEscape(args[1]))
	case "delete":
		if len(args) != 2 {
			return usageError("usage: mbox credentials delete <credential-id>")
		}
		return a.delete(ctx, client, "/v1/credentials/"+url.PathEscape(args[1]))
	default:
		return usageError("usage: mbox credentials get|delete <credential-id>")
	}
}

func (a *App) stream(ctx context.Context, client *Client, path string) error {
	response, err := client.Raw(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, err = io.Copy(a.streams.Stdout, response.Body)
	return err
}

func (a *App) runTerminal(ctx context.Context, client *Client, args []string) error {
	if len(args) == 0 {
		return usageError("usage: mbox terminal <sandbox-id> [--shell sh|bash]")
	}
	sandboxID := args[0]
	fs := flag.NewFlagSet("terminal", flag.ContinueOnError)
	fs.SetOutput(a.streams.Stderr)
	shellName := fs.String("shell", "sh", "")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	query := url.Values{}
	query.Set("shell", *shellName)
	query.Set("client", "mbox-cli")
	terminalURL, err := websocketURL(client.BaseURL, "/v1/sandboxes/"+url.PathEscape(sandboxID)+"/terminal?"+query.Encode())
	if err != nil {
		return err
	}
	headers := http.Header{}
	client.setRequestHeaders(headers)
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, terminalURL, headers)
	if err != nil {
		return err
	}
	defer conn.Close()

	errc := make(chan error, 2)
	go func() {
		scanner := bufio.NewScanner(a.streams.Stdin)
		for scanner.Scan() {
			if err := conn.WriteMessage(websocket.TextMessage, append(scanner.Bytes(), '\n')); err != nil {
				errc <- err
				return
			}
		}
		errc <- scanner.Err()
	}()
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				errc <- err
				return
			}
			if _, err := a.streams.Stdout.Write(data); err != nil {
				errc <- err
				return
			}
		}
	}()
	err = <-errc
	if errors.Is(err, io.EOF) || websocket.IsCloseError(err, websocket.CloseNormalClosure) {
		return nil
	}
	return err
}

func websocketURL(baseURL string, path string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	switch parsed.Scheme {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	default:
		return "", fmt.Errorf("API URL must use http or https")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + path
	return parsed.String(), nil
}

func parsePositiveInt(raw string, label string) (int, error) {
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", label)
	}
	return value, nil
}

func parseCommandFlags(args []string, command string, commandJSON string) ([]string, error) {
	set := 0
	if len(args) > 0 {
		set++
	}
	if strings.TrimSpace(command) != "" {
		set++
	}
	if strings.TrimSpace(commandJSON) != "" {
		set++
	}
	if set == 0 {
		return nil, nil
	}
	if set > 1 {
		return nil, usageError("use only one of --arg, --command, or --command-json")
	}
	if len(args) > 0 {
		return []string(args), nil
	}
	if strings.TrimSpace(commandJSON) != "" {
		var values []string
		if err := json.Unmarshal([]byte(commandJSON), &values); err != nil {
			return nil, fmt.Errorf("command-json must be a JSON string array: %w", err)
		}
		return values, nil
	}
	return commandFromFlag(command), nil
}

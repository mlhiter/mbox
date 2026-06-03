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
	"sort"
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
	case "auth":
		return a.runAuth(ctx, client, args[1:])
	case "caller":
		return a.runCaller(ctx, client, args[1:], "mbox caller")
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
	case "member", "members":
		return a.runMember(ctx, client, args[1:])
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
  auth caller [--summary]
  caller [--summary]
  compat [--client-api-version VERSION] [--require-capability CAPABILITY]
  context current|list|check
  context set NAME --api-url URL [--token TOKEN|--token-env ENV] [--audit-actor ACTOR] [--audit-source SOURCE] [--current]
  context use NAME
  context remove NAME
  openapi
  runtime resources [--namespace NAMESPACE] [--project-id PROJECT] [--kind KIND] [--summary|--summary-table] [--resolve-project-names]
  runtime orphans [--namespace NAMESPACE] [--project-id PROJECT] [--kind KIND] [--summary-table]
  runtime cleanup-orphan --adapter ADAPTER --kind KIND --namespace NAMESPACE --name NAME --reason REASON --confirm delete-orphan-runtime-resource
  audit-events [--project-id PROJECT] [--action ACTION] [--resource-type TYPE] [--resource-id ID] [--actor ACTOR] [--source SOURCE] [--filter-request-id ID] [--operation OPERATION] [--reason REASON] [--since RFC3339] [--until RFC3339] [--limit N] [--summary] [--policy-denied-summary]
  projects list
  projects create --name NAME --namespace NAMESPACE [--slug SLUG]
  projects get <project-id>
  projects usage <project-id> [--summary]
  projects authorization <project-id> [--action ACTION] [--summary]
  projects members <project-id> [--summary]
  projects add-member <project-id> --principal PRINCIPAL --role owner|operator|viewer [--principal-type user|service_account|automation]
  projects audit-events <project-id> [--action ACTION] [--resource-type TYPE] [--resource-id ID] [--actor ACTOR] [--source SOURCE] [--filter-request-id ID] [--operation OPERATION] [--reason REASON] [--since RFC3339] [--until RFC3339] [--limit N] [--summary] [--policy-denied-summary]
  projects policy <project-id> [--summary]
  projects set-policy <project-id> --enforcement disabled|enforced [--allowed-image-prefix PREFIX] [--allowed-service-account NAME] [--allowed-secret-ref NAME]
  projects quota-policy <project-id> [--summary]
  projects set-quota-policy <project-id> --enforcement disabled|enforced [--max-active-sandboxes N] [--max-retained-artifact-bytes N]
  projects credentials <project-id> [--summary]
  projects add-credential <project-id> --name NAME --type git|registry|kubernetes|ssh|generic --secret-ref NAME [--secret-key KEY]
  projects delete <project-id>
  templates list [--project-id PROJECT]
  templates create --name NAME --image IMAGE [--project-id PROJECT] [--arg ARG]
  templates get <template-id>
  templates boundary <template-id> [--project-id PROJECT] [--summary]
  templates validate <template-id> --project-id PROJECT [--name NAME]
  templates validate-run <template-id> --project-id PROJECT [--name NAME] [--wait-timeout 5m] [--task-timeout 60] [--require-success] -- sh -lc 'echo ok'
  templates delete <template-id>
  templates decide-validation <template-id> <sandbox-id> --status passed|failed
  sandboxes list [--project-id PROJECT]
  sandboxes create --project-id PROJECT --name NAME [--template-id TEMPLATE]
  sandboxes get <sandbox-id>
  sandboxes boundary <sandbox-id> [--summary]
  sandboxes start|stop|delete <sandbox-id>
  sandboxes wait <sandbox-id> [--status running] [--interval 1500ms] [--timeout 5m] [--require-runtime-ref]
  sessions list <sandbox-id>
  sessions create <sandbox-id> --type TYPE [--client CLIENT]
  sessions get|end <session-id>
  tasks list <sandbox-id>
  tasks create <sandbox-id> (--arg ARG...|--command CMD|--command-json JSON) [--timeout 60]
  tasks run <sandbox-id> [--timeout 60] [--interval 1500ms] [--wait-timeout 5m] [--require-success] -- sh -lc 'echo ok'
  tasks get|cancel|watch <task-id>
  tasks artifacts <task-id>
  tasks wait <task-id> [--interval 1500ms] [--timeout 5m] [--require-success]
  artifacts list <sandbox-id>
  artifacts create <sandbox-id> --kind KIND --name NAME --uri URI
  artifacts get|capture|content <artifact-id>
  artifacts upload <artifact-id> (--file PATH|--stdin) [--content-type TYPE]
  credentials get|delete <credential-id>
  members get|delete <member-id>
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

func (a *App) runAuth(ctx context.Context, client *Client, args []string) error {
	if len(args) == 0 {
		return usageError("usage: mbox auth caller [--summary]")
	}
	switch args[0] {
	case "caller":
		return a.runCaller(ctx, client, args[1:], "mbox auth caller")
	default:
		return usageError("usage: mbox auth caller [--summary]")
	}
}

func (a *App) runCaller(ctx context.Context, client *Client, args []string, command string) error {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(a.streams.Stderr)
	summary := fs.Bool("summary", false, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return usageError("usage: " + command + " [--summary]")
	}
	if *summary {
		var caller callerSummaryInfo
		if err := client.JSON(ctx, http.MethodGet, "/v1/auth/caller", nil, &caller); err != nil {
			return err
		}
		return writeCallerSummary(a.streams.Stdout, caller)
	}
	return a.get(ctx, client, "/v1/auth/caller")
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
		projectID := fs.String("project-id", "", "")
		kind := fs.String("kind", "", "")
		summaryOnly := fs.Bool("summary", false, "")
		summaryTable := fs.Bool("summary-table", false, "")
		resolveProjectNames := fs.Bool("resolve-project-names", false, "")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return usageError("usage: mbox runtime resources [--namespace NAMESPACE] [--project-id PROJECT] [--kind KIND] [--summary|--summary-table] [--resolve-project-names]")
		}
		if *summaryOnly && *summaryTable {
			return usageError("mbox runtime resources accepts only one of --summary or --summary-table")
		}
		if *resolveProjectNames && !*summaryTable {
			return usageError("mbox runtime resources --resolve-project-names requires --summary-table")
		}
		path := "/v1/runtime/resources"
		values := url.Values{}
		if strings.TrimSpace(*namespace) != "" {
			values.Set("namespace", strings.TrimSpace(*namespace))
		}
		if strings.TrimSpace(*projectID) != "" {
			values.Set("projectId", strings.TrimSpace(*projectID))
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
		if *summaryTable {
			var response struct {
				Summary json.RawMessage `json:"summary"`
			}
			if err := client.JSON(ctx, http.MethodGet, path, nil, &response); err != nil {
				return err
			}
			if len(response.Summary) == 0 || strings.TrimSpace(string(response.Summary)) == "null" {
				return fmt.Errorf("runtime resources response did not include summary")
			}
			var summary runtimeResourceSummaryTable
			if err := json.Unmarshal(response.Summary, &summary); err != nil {
				return fmt.Errorf("runtime resources summary was not readable: %w", err)
			}
			projectNames := map[string]string(nil)
			if *resolveProjectNames {
				var err error
				projectNames, err = runtimeResourceProjectNames(ctx, client)
				if err != nil {
					return err
				}
			}
			return writeRuntimeResourceSummaryTable(a.streams.Stdout, summary, projectNames)
		}
		return a.get(ctx, client, path)
	case "orphan", "orphans":
		fs := flag.NewFlagSet("runtime orphans", flag.ContinueOnError)
		fs.SetOutput(a.streams.Stderr)
		namespace := fs.String("namespace", "", "")
		projectID := fs.String("project-id", "", "")
		kind := fs.String("kind", "", "")
		summaryTable := fs.Bool("summary-table", false, "")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return usageError("usage: mbox runtime orphans [--namespace NAMESPACE] [--project-id PROJECT] [--kind KIND] [--summary-table]")
		}
		path := "/v1/runtime/orphans"
		values := url.Values{}
		if strings.TrimSpace(*namespace) != "" {
			values.Set("namespace", strings.TrimSpace(*namespace))
		}
		if strings.TrimSpace(*projectID) != "" {
			values.Set("projectId", strings.TrimSpace(*projectID))
		}
		if strings.TrimSpace(*kind) != "" {
			values.Set("kind", strings.TrimSpace(*kind))
		}
		if encoded := values.Encode(); encoded != "" {
			path += "?" + encoded
		}
		if *summaryTable {
			var audit runtimeOrphanAuditSummaryTable
			if err := client.JSON(ctx, http.MethodGet, path, nil, &audit); err != nil {
				return err
			}
			return writeRuntimeOrphanSummaryTable(a.streams.Stdout, audit)
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

type runtimeResourceSummaryTable struct {
	Total       int                         `json:"total"`
	ByKind      []runtimeResourceCountTable `json:"byKind"`
	ByNamespace []runtimeResourceCountTable `json:"byNamespace"`
	ByOwner     []runtimeResourceCountTable `json:"byOwner"`
	ByProject   []runtimeResourceCountTable `json:"byProject"`
	Workload    runtimeWorkloadSummaryTable `json:"workload"`
}

type runtimeOrphanAuditSummaryTable struct {
	Adapter       string                           `json:"adapter"`
	CheckedAt     string                           `json:"checkedAt"`
	Namespace     string                           `json:"namespace"`
	ResourceCount int                              `json:"resourceCount"`
	OrphanCount   int                              `json:"orphanCount"`
	ExpectedClean bool                             `json:"expectedClean"`
	Items         []runtimeOrphanSummaryTableEntry `json:"items"`
}

type runtimeOrphanSummaryTableEntry struct {
	Reason     string                            `json:"reason"`
	Resource   runtimeOrphanSummaryTableResource `json:"resource"`
	SandboxID  string                            `json:"sandboxId"`
	TemplateID string                            `json:"templateId"`
	ProjectID  string                            `json:"projectId"`
	Status     string                            `json:"status"`
	Message    string                            `json:"message"`
	Evidence   []string                          `json:"evidence"`
}

type runtimeOrphanSummaryTableResource struct {
	Adapter   string `json:"adapter"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type runtimeResourceCountTable struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type runtimeProjectList struct {
	Items []runtimeProjectListItem `json:"items"`
}

type runtimeProjectListItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type runtimeWorkloadSummaryTable struct {
	ObservedResources int                          `json:"observedResources"`
	DesiredPods       int64                        `json:"desiredPods"`
	ObservedPods      int                          `json:"observedPods"`
	RunningPods       int                          `json:"runningPods"`
	ContainersReady   int                          `json:"containersReady"`
	ContainersTotal   int                          `json:"containersTotal"`
	RestartCount      int32                        `json:"restartCount"`
	Requests          map[string]string            `json:"requests"`
	Limits            map[string]string            `json:"limits"`
	StorageCapacity   string                       `json:"storageCapacity"`
	QuantityIssues    []runtimeQuantityIssueTable  `json:"quantityIssues"`
	Storage           []runtimeStorageSummaryTable `json:"storage"`
}

type runtimeQuantityIssueTable struct {
	Resource string `json:"resource"`
	Field    string `json:"field"`
	Value    string `json:"value"`
	Reason   string `json:"reason"`
}

type runtimeStorageSummaryTable struct {
	Phase    string `json:"phase"`
	Count    int    `json:"count"`
	Capacity string `json:"capacity"`
}

func runtimeResourceProjectNames(ctx context.Context, client *Client) (map[string]string, error) {
	var projects runtimeProjectList
	if err := client.JSON(ctx, http.MethodGet, "/v1/projects", nil, &projects); err != nil {
		return nil, fmt.Errorf("runtime resources project names were not readable: %w", err)
	}
	names := make(map[string]string, len(projects.Items))
	for _, project := range projects.Items {
		id := strings.TrimSpace(project.ID)
		name := strings.TrimSpace(project.Name)
		if id != "" && name != "" {
			names[id] = name
		}
	}
	return names, nil
}

func writeRuntimeResourceSummaryTable(w io.Writer, summary runtimeResourceSummaryTable, projectNames map[string]string) error {
	out := bufio.NewWriter(w)
	if _, err := fmt.Fprintln(out, "RUNTIME RESOURCES SUMMARY"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "TOTAL\t%d\n", summary.Total); err != nil {
		return err
	}
	if err := writeRuntimeResourceCountSection(out, "BY KIND", summary.ByKind); err != nil {
		return err
	}
	if err := writeRuntimeResourceCountSection(out, "BY NAMESPACE", summary.ByNamespace); err != nil {
		return err
	}
	if err := writeRuntimeResourceCountSection(out, "BY PROJECT", displayRuntimeProjectCounts(summary.ByProject, projectNames)); err != nil {
		return err
	}
	if err := writeRuntimeResourceCountSection(out, "BY OWNER", summary.ByOwner); err != nil {
		return err
	}
	if err := writeRuntimeWorkloadSummary(out, summary.Workload); err != nil {
		return err
	}
	return out.Flush()
}

func writeRuntimeOrphanSummaryTable(w io.Writer, audit runtimeOrphanAuditSummaryTable) error {
	out := bufio.NewWriter(w)
	if _, err := fmt.Fprintln(out, "RUNTIME ORPHANS SUMMARY"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value any
	}{
		{"adapter", tableValue(audit.Adapter, "unknown")},
		{"checkedAt", tableValue(audit.CheckedAt, "unknown")},
		{"namespace", tableValue(audit.Namespace, "all")},
		{"resources", audit.ResourceCount},
		{"orphans", audit.OrphanCount},
		{"expectedClean", formatBool(audit.ExpectedClean)},
	}
	for _, line := range lines {
		if _, err := fmt.Fprintf(out, "%s\t%v\n", line.label, line.value); err != nil {
			return err
		}
	}
	if err := writeRuntimeResourceCountSection(out, "BY REASON", runtimeOrphanReasonCounts(audit.Items)); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "\nORPHANS"); err != nil {
		return err
	}
	if len(audit.Items) == 0 {
		if _, err := fmt.Fprintln(out, "  (none)"); err != nil {
			return err
		}
		return out.Flush()
	}
	items := append([]runtimeOrphanSummaryTableEntry(nil), audit.Items...)
	sort.Slice(items, func(i, j int) bool {
		left := runtimeOrphanSortKey(items[i])
		right := runtimeOrphanSortKey(items[j])
		if left == right {
			return items[i].Reason < items[j].Reason
		}
		return left < right
	})
	if _, err := fmt.Fprintln(out, "REASON\tRESOURCE\tPROJECT\tSTATUS\tMESSAGE"); err != nil {
		return err
	}
	for _, item := range items {
		if _, err := fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\n",
			tableValue(item.Reason, "unknown"),
			runtimeOrphanResourceLabel(item.Resource),
			tableValue(item.ProjectID, "-"),
			tableValue(item.Status, "-"),
			tableValue(item.Message, "-"),
		); err != nil {
			return err
		}
		if len(item.Evidence) > 0 {
			for _, evidence := range item.Evidence {
				evidence = strings.TrimSpace(evidence)
				if evidence == "" {
					continue
				}
				if _, err := fmt.Fprintf(out, "  evidence\t%s\n", evidence); err != nil {
					return err
				}
			}
		}
	}
	return out.Flush()
}

func runtimeOrphanReasonCounts(items []runtimeOrphanSummaryTableEntry) []runtimeResourceCountTable {
	counts := map[string]int{}
	for _, item := range items {
		reason := strings.TrimSpace(item.Reason)
		if reason == "" {
			reason = "unknown"
		}
		counts[reason]++
	}
	result := make([]runtimeResourceCountTable, 0, len(counts))
	for reason, count := range counts {
		result = append(result, runtimeResourceCountTable{Name: reason, Count: count})
	}
	return result
}

func runtimeOrphanSortKey(item runtimeOrphanSummaryTableEntry) string {
	resource := item.Resource
	return strings.Join([]string{
		strings.TrimSpace(resource.Namespace),
		strings.TrimSpace(resource.Kind),
		strings.TrimSpace(resource.Name),
		strings.TrimSpace(item.Reason),
	}, "/")
}

func runtimeOrphanResourceLabel(resource runtimeOrphanSummaryTableResource) string {
	kind := tableValue(resource.Kind, "Resource")
	namespace := strings.TrimSpace(resource.Namespace)
	name := strings.TrimSpace(resource.Name)
	if namespace == "" && name == "" {
		return kind
	}
	if namespace == "" {
		return kind + " " + name
	}
	if name == "" {
		return kind + " " + namespace
	}
	return kind + " " + namespace + "/" + name
}

func displayRuntimeProjectCounts(counts []runtimeResourceCountTable, projectNames map[string]string) []runtimeResourceCountTable {
	if len(projectNames) == 0 {
		return counts
	}
	items := make([]runtimeResourceCountTable, 0, len(counts))
	for _, item := range counts {
		name := strings.TrimSpace(item.Name)
		if display := strings.TrimSpace(projectNames[name]); display != "" {
			name = fmt.Sprintf("%s (%s)", display, shortRuntimeResourceID(name))
		}
		items = append(items, runtimeResourceCountTable{
			Name:  name,
			Count: item.Count,
		})
	}
	return items
}

func shortRuntimeResourceID(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 12 {
		return value
	}
	return value[:8] + "..." + value[len(value)-4:]
}

func writeRuntimeResourceCountSection(w io.Writer, title string, counts []runtimeResourceCountTable) error {
	if _, err := fmt.Fprintf(w, "\n%s\n", title); err != nil {
		return err
	}
	if len(counts) == 0 {
		_, err := fmt.Fprintln(w, "  (none)")
		return err
	}
	items := append([]runtimeResourceCountTable(nil), counts...)
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = "(empty)"
		}
		if _, err := fmt.Fprintf(w, "  %s\t%d\n", name, item.Count); err != nil {
			return err
		}
	}
	return nil
}

func writeRuntimeWorkloadSummary(w io.Writer, workload runtimeWorkloadSummaryTable) error {
	if _, err := fmt.Fprintln(w, "\nWORKLOAD"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value any
	}{
		{"observedResources", workload.ObservedResources},
		{"desiredPods", workload.DesiredPods},
		{"observedPods", workload.ObservedPods},
		{"runningPods", workload.RunningPods},
		{"containersReady", fmt.Sprintf("%d/%d", workload.ContainersReady, workload.ContainersTotal)},
		{"restartCount", workload.RestartCount},
	}
	for _, line := range lines {
		if _, err := fmt.Fprintf(w, "  %s\t%v\n", line.label, line.value); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "  requests\t%s\n", formatRuntimeSummaryStringMap(workload.Requests)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "  limits\t%s\n", formatRuntimeSummaryStringMap(workload.Limits)); err != nil {
		return err
	}
	if strings.TrimSpace(workload.StorageCapacity) != "" {
		if _, err := fmt.Fprintf(w, "  storageCapacity\t%s\n", strings.TrimSpace(workload.StorageCapacity)); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "  storage\t%s\n", formatRuntimeStorageSummaries(workload.Storage)); err != nil {
		return err
	}
	if len(workload.QuantityIssues) > 0 {
		if _, err := fmt.Fprintf(w, "  quantityIssues\t%d\n", len(workload.QuantityIssues)); err != nil {
			return err
		}
	}
	return nil
}

func formatRuntimeSummaryStringMap(values map[string]string) string {
	if len(values) == 0 {
		return "(none)"
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		if strings.TrimSpace(key) != "" {
			keys = append(keys, strings.TrimSpace(key))
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+strings.TrimSpace(values[key]))
	}
	if len(parts) == 0 {
		return "(none)"
	}
	return strings.Join(parts, " ")
}

func formatRuntimeStorageSummaries(values []runtimeStorageSummaryTable) string {
	if len(values) == 0 {
		return "(none)"
	}
	items := append([]runtimeStorageSummaryTable(nil), values...)
	sort.Slice(items, func(i, j int) bool {
		return items[i].Phase < items[j].Phase
	})
	parts := make([]string, 0, len(items))
	for _, item := range items {
		phase := strings.TrimSpace(item.Phase)
		if phase == "" {
			phase = "Unknown"
		}
		part := fmt.Sprintf("%s=%d", phase, item.Count)
		if strings.TrimSpace(item.Capacity) != "" {
			part += "(" + strings.TrimSpace(item.Capacity) + ")"
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, " ")
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
	reason := fs.String("reason", "", "")
	since := fs.String("since", "", "")
	until := fs.String("until", "", "")
	limit := fs.Int("limit", 0, "")
	summary := fs.Bool("summary", false, "")
	policyDeniedSummary := fs.Bool("policy-denied-summary", false, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		if projectID != "" {
			return usageError("usage: mbox projects audit-events <project-id> [--action ACTION] [--resource-type TYPE] [--resource-id ID] [--actor ACTOR] [--source SOURCE] [--filter-request-id ID] [--operation OPERATION] [--reason REASON] [--since RFC3339] [--until RFC3339] [--limit N] [--summary] [--policy-denied-summary]")
		}
		return usageError("usage: mbox audit-events [--project-id PROJECT] [--action ACTION] [--resource-type TYPE] [--resource-id ID] [--actor ACTOR] [--source SOURCE] [--filter-request-id ID] [--operation OPERATION] [--reason REASON] [--since RFC3339] [--until RFC3339] [--limit N] [--summary] [--policy-denied-summary]")
	}
	if *summary && *policyDeniedSummary {
		return usageError("mbox audit-events accepts only one of --summary or --policy-denied-summary")
	}
	if *policyDeniedSummary && strings.TrimSpace(*action) != "" && strings.TrimSpace(*action) != "policy.denied" {
		return usageError("mbox audit-events --policy-denied-summary requires action policy.denied")
	}
	path := "/v1/audit-events"
	values := url.Values{}
	if strings.TrimSpace(*projectIDFlag) != "" {
		values.Set("projectId", strings.TrimSpace(*projectIDFlag))
	}
	if *policyDeniedSummary {
		values.Set("action", "policy.denied")
	} else if strings.TrimSpace(*action) != "" {
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
	if strings.TrimSpace(*reason) != "" {
		values.Set("reason", strings.TrimSpace(*reason))
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
	if *summary {
		var response auditEventListResponse
		if err := client.JSON(ctx, http.MethodGet, path, nil, &response); err != nil {
			return err
		}
		return writeAuditSummaryTable(a.streams.Stdout, response.Items)
	}
	if *policyDeniedSummary {
		var response auditEventListResponse
		if err := client.JSON(ctx, http.MethodGet, path, nil, &response); err != nil {
			return err
		}
		return writePolicyDeniedSummaryTable(a.streams.Stdout, response.Items)
	}
	return a.get(ctx, client, path)
}

type auditEventListResponse struct {
	Items []auditEventSummaryItem `json:"items"`
}

type auditEventSummaryItem struct {
	Action       string          `json:"action"`
	ResourceType string          `json:"resourceType"`
	ResourceName string          `json:"resourceName"`
	Actor        string          `json:"actor"`
	Source       string          `json:"source"`
	Metadata     json.RawMessage `json:"metadata"`
	CreatedAt    time.Time       `json:"createdAt"`
}

type policyDeniedSummaryRow struct {
	Operation string
	Reason    string
	Count     int
	Latest    time.Time
	Actors    map[string]bool
	Sources   map[string]bool
	Resources map[string]bool
}

type auditSummaryRow struct {
	Action        string
	Count         int
	Latest        time.Time
	ResourceTypes map[string]bool
	Actors        map[string]bool
	Sources       map[string]bool
}

type boundarySummary struct {
	Kind                         string                  `json:"kind"`
	ProjectID                    string                  `json:"projectId"`
	ProjectName                  string                  `json:"projectName"`
	TemplateID                   string                  `json:"templateId"`
	TemplateName                 string                  `json:"templateName"`
	SandboxID                    string                  `json:"sandboxId"`
	SandboxName                  string                  `json:"sandboxName"`
	SandboxStatus                string                  `json:"sandboxStatus"`
	Namespace                    string                  `json:"namespace"`
	ServiceAccountName           string                  `json:"serviceAccountName"`
	ServiceAccountTokenAutomount bool                    `json:"serviceAccountTokenAutomount"`
	RuntimeRef                   *boundaryRuntimeRef     `json:"runtimeRef"`
	Image                        string                  `json:"image"`
	WorkingDir                   string                  `json:"workingDir"`
	ResourceRequests             map[string]string       `json:"resourceRequests"`
	StorageRequest               string                  `json:"storageRequest"`
	PreviewPorts                 []boundaryPortSummary   `json:"previewPorts"`
	EnvVarCount                  int                     `json:"envVarCount"`
	SecretRefs                   []boundarySecretRef     `json:"secretRefs"`
	SecretProjection             string                  `json:"secretProjection"`
	NetworkPolicy                string                  `json:"networkPolicy"`
	NetworkPolicyProjection      string                  `json:"networkPolicyProjection"`
	LifecyclePolicyProjection    string                  `json:"lifecyclePolicyProjection"`
	PolicyEnforcement            string                  `json:"policyEnforcement"`
	AllowedImagePrefixes         []string                `json:"allowedImagePrefixes"`
	AllowedServiceAccounts       []string                `json:"allowedServiceAccounts"`
	AllowedSecretRefs            []string                `json:"allowedSecretRefs"`
	CredentialRefs               []boundaryCredentialRef `json:"credentialRefs"`
	CredentialProjection         string                  `json:"credentialProjection"`
	ControllerPermissions        []string                `json:"controllerPermissions"`
	RuntimeAccess                []string                `json:"runtimeAccess"`
	Cleanup                      []string                `json:"cleanup"`
	Checks                       []boundaryCheckSummary  `json:"checks"`
}

type boundaryRuntimeRef struct {
	Adapter   string `json:"adapter"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type boundaryPortSummary struct {
	Name     string `json:"name"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
}

type boundarySecretRef struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

type boundaryCredentialRef struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Slug      string   `json:"slug"`
	Type      string   `json:"type"`
	Target    string   `json:"target"`
	SecretRef string   `json:"secretRef"`
	Usage     []string `json:"usage"`
}

type boundaryCheckSummary struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Status   string   `json:"status"`
	Message  string   `json:"message"`
	Evidence []string `json:"evidence"`
}

type projectPolicySummary struct {
	ProjectID              string   `json:"projectId"`
	Enforcement            string   `json:"enforcement"`
	AllowedImagePrefixes   []string `json:"allowedImagePrefixes"`
	AllowedServiceAccounts []string `json:"allowedServiceAccounts"`
	AllowedSecretRefs      []string `json:"allowedSecretRefs"`
	CreatedAt              string   `json:"createdAt"`
	UpdatedAt              string   `json:"updatedAt"`
}

type projectQuotaPolicySummary struct {
	ProjectID                string `json:"projectId"`
	Enforcement              string `json:"enforcement"`
	MaxActiveSandboxes       *int   `json:"maxActiveSandboxes"`
	MaxRetainedArtifactBytes *int64 `json:"maxRetainedArtifactBytes"`
	CreatedAt                string `json:"createdAt"`
	UpdatedAt                string `json:"updatedAt"`
}

type projectUsageSummary struct {
	ProjectID       string                        `json:"projectId"`
	GeneratedAt     string                        `json:"generatedAt"`
	Sandboxes       projectSandboxUsageSummary    `json:"sandboxes"`
	RuntimeSessions projectSessionUsageSummary    `json:"runtimeSessions"`
	ExecutionTasks  projectTaskUsageSummary       `json:"executionTasks"`
	Artifacts       projectArtifactUsageSummary   `json:"artifacts"`
	Templates       projectTemplateUsageSummary   `json:"templates"`
	Credentials     projectCredentialUsageSummary `json:"credentials"`
	Notes           []string                      `json:"notes"`
}

type projectSandboxUsageSummary struct {
	Total           int                         `json:"total"`
	Active          int                         `json:"active"`
	Pending         int                         `json:"pending"`
	Running         int                         `json:"running"`
	Stopped         int                         `json:"stopped"`
	Failed          int                         `json:"failed"`
	Deleted         int                         `json:"deleted"`
	CleanupPending  int                         `json:"cleanupPending"`
	ActiveRequests  sandboxResourceRequestUsage `json:"activeRequests"`
	RunningRequests sandboxResourceRequestUsage `json:"runningRequests"`
}

type sandboxResourceRequestUsage struct {
	Count   int                   `json:"count"`
	CPU     resourceQuantityUsage `json:"cpu"`
	Memory  resourceQuantityUsage `json:"memory"`
	Storage resourceQuantityUsage `json:"storage"`
}

type resourceQuantityUsage struct {
	Total    string `json:"total"`
	Declared int    `json:"declared"`
	Missing  int    `json:"missing"`
	Invalid  int    `json:"invalid"`
}

type projectSessionUsageSummary struct {
	Total    int `json:"total"`
	Active   int `json:"active"`
	Ended    int `json:"ended"`
	Failed   int `json:"failed"`
	Terminal int `json:"terminal"`
	IDE      int `json:"ide"`
	Notebook int `json:"notebook"`
	Browser  int `json:"browser"`
	Command  int `json:"command"`
	Custom   int `json:"custom"`
}

type projectTaskUsageSummary struct {
	Total     int `json:"total"`
	Queued    int `json:"queued"`
	Running   int `json:"running"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
	Canceled  int `json:"canceled"`
	TimedOut  int `json:"timedOut"`
}

type projectArtifactUsageSummary struct {
	Total           int   `json:"total"`
	RetainedContent int   `json:"retainedContent"`
	ReferencedBytes int64 `json:"referencedBytes"`
	RetainedBytes   int64 `json:"retainedBytes"`
	File            int   `json:"file"`
	Directory       int   `json:"directory"`
	Log             int   `json:"log"`
	Report          int   `json:"report"`
	Screenshot      int   `json:"screenshot"`
	Image           int   `json:"image"`
	Link            int   `json:"link"`
	Other           int   `json:"other"`
}

type projectTemplateUsageSummary struct {
	ProjectScoped   int                  `json:"projectScoped"`
	GlobalVisible   int                  `json:"globalVisible"`
	CPURequests     []resourceUsageValue `json:"cpuRequests"`
	MemoryRequests  []resourceUsageValue `json:"memoryRequests"`
	StorageRequests []resourceUsageValue `json:"storageRequests"`
}

type resourceUsageValue struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

type projectCredentialUsageSummary struct {
	Total      int `json:"total"`
	Git        int `json:"git"`
	Registry   int `json:"registry"`
	Kubernetes int `json:"kubernetes"`
	SSH        int `json:"ssh"`
	Generic    int `json:"generic"`
}

type projectCredentialListSummary struct {
	Items []projectCredentialSummaryItem `json:"items"`
}

type projectCredentialSummaryItem struct {
	ID        string                  `json:"id"`
	ProjectID string                  `json:"projectId"`
	Name      string                  `json:"name"`
	Slug      string                  `json:"slug"`
	Type      string                  `json:"type"`
	Target    string                  `json:"target"`
	SecretRef projectCredentialSecret `json:"secretRef"`
	Usage     []string                `json:"usage"`
}

type projectCredentialSecret struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

type projectMemberListSummary struct {
	Items []projectMemberSummaryItem `json:"items"`
}

type projectMemberSummaryItem struct {
	ID            string `json:"id"`
	ProjectID     string `json:"projectId"`
	PrincipalType string `json:"principalType"`
	Principal     string `json:"principal"`
	Role          string `json:"role"`
}

type callerSummaryInfo struct {
	Authenticated          bool     `json:"authenticated"`
	AuthenticationRequired bool     `json:"authenticationRequired"`
	Mode                   string   `json:"mode"`
	PrincipalType          string   `json:"principalType"`
	Principal              string   `json:"principal"`
	RBACTrusted            bool     `json:"rbacTrusted"`
	ProjectRolesEnforced   bool     `json:"projectRolesEnforced"`
	Notes                  []string `json:"notes"`
}

type projectAuthorizationSummaryDecision struct {
	ProjectID        string                             `json:"projectId"`
	Action           string                             `json:"action"`
	Allowed          bool                               `json:"allowed"`
	Enforced         bool                               `json:"enforced"`
	Evaluation       string                             `json:"evaluation"`
	RequiredRoles    []string                           `json:"requiredRoles"`
	Caller           projectAuthorizationSummaryCaller  `json:"caller"`
	MatchedMember    *projectAuthorizationSummaryMember `json:"matchedMember"`
	MemberCount      int                                `json:"memberCount"`
	AvailableActions []string                           `json:"availableActions"`
	Notes            []string                           `json:"notes"`
}

type projectAuthorizationSummaryCaller struct {
	Authenticated          bool     `json:"authenticated"`
	AuthenticationRequired bool     `json:"authenticationRequired"`
	Mode                   string   `json:"mode"`
	PrincipalType          string   `json:"principalType"`
	Principal              string   `json:"principal"`
	RBACTrusted            bool     `json:"rbacTrusted"`
	ProjectRolesEnforced   bool     `json:"projectRolesEnforced"`
	Notes                  []string `json:"notes"`
}

type projectAuthorizationSummaryMember struct {
	ID            string `json:"id"`
	PrincipalType string `json:"principalType"`
	Principal     string `json:"principal"`
	Role          string `json:"role"`
}

func writeBoundarySummary(w io.Writer, summary boundarySummary) error {
	out := bufio.NewWriter(w)
	if _, err := fmt.Fprintln(out, "BOUNDARY SUMMARY"); err != nil {
		return err
	}
	rows := [][2]string{
		{"Kind", tableValue(summary.Kind, "unknown")},
		{"Project", boundaryNameIDLabel(summary.ProjectName, summary.ProjectID)},
		{"Template", boundaryNameIDLabel(summary.TemplateName, summary.TemplateID)},
	}
	if strings.TrimSpace(summary.SandboxID) != "" || strings.TrimSpace(summary.SandboxName) != "" {
		rows = append(rows,
			[2]string{"Sandbox", boundaryNameIDLabel(summary.SandboxName, summary.SandboxID)},
			[2]string{"Sandbox status", tableValue(summary.SandboxStatus, "unknown")},
		)
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(out, "%s\t%s\n", row[0], row[1]); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(out, "\nRUNTIME SHAPE"); err != nil {
		return err
	}
	runtimeRows := [][2]string{
		{"Namespace", tableValue(summary.Namespace, "not resolved")},
		{"ServiceAccount", tableValue(summary.ServiceAccountName, "not resolved")},
		{"Token automount", formatBool(summary.ServiceAccountTokenAutomount)},
		{"Runtime ref", boundaryRuntimeRefLabel(summary.RuntimeRef)},
		{"Image", tableValue(summary.Image, "-")},
		{"Working dir", tableValue(summary.WorkingDir, "-")},
		{"Resource requests", formatBoundaryResourceRequests(summary.ResourceRequests)},
		{"Storage request", tableValue(summary.StorageRequest, "-")},
		{"Preview ports", formatBoundaryPorts(summary.PreviewPorts)},
		{"Env vars", strconv.Itoa(summary.EnvVarCount)},
	}
	for _, row := range runtimeRows {
		if _, err := fmt.Fprintf(out, "%s\t%s\n", row[0], row[1]); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(out, "\nPOLICY AND CREDENTIALS"); err != nil {
		return err
	}
	policyRows := [][2]string{
		{"Launch policy", tableValue(summary.PolicyEnforcement, "disabled")},
		{"Allowed image prefixes", formatStringList(cleanStringList(summary.AllowedImagePrefixes))},
		{"Allowed service accounts", formatStringList(cleanStringList(summary.AllowedServiceAccounts))},
		{"Allowed secret refs", formatStringList(cleanStringList(summary.AllowedSecretRefs))},
		{"Template secret refs", formatBoundarySecretRefs(summary.SecretRefs)},
		{"Secret projection", tableValue(summary.SecretProjection, "-")},
		{"Credential refs", formatBoundaryCredentialRefs(summary.CredentialRefs)},
		{"Credential projection", tableValue(summary.CredentialProjection, "-")},
		{"Network policy", tableValue(summary.NetworkPolicy, "default")},
		{"Network projection", tableValue(summary.NetworkPolicyProjection, "-")},
		{"Lifecycle projection", tableValue(summary.LifecyclePolicyProjection, "-")},
	}
	for _, row := range policyRows {
		if _, err := fmt.Fprintf(out, "%s\t%s\n", row[0], row[1]); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(out, "\nCHECKS"); err != nil {
		return err
	}
	if len(summary.Checks) == 0 {
		if _, err := fmt.Fprintln(out, "  (none)"); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintln(out, "STATUS\tCHECK\tMESSAGE\tEVIDENCE"); err != nil {
			return err
		}
		for _, check := range summary.Checks {
			if _, err := fmt.Fprintf(out, "%s\t%s\t%s\t%s\n",
				tableValue(check.Status, "unknown"),
				tableValue(check.Label, tableValue(check.ID, "check")),
				tableValue(check.Message, "-"),
				formatStringList(cleanStringList(check.Evidence)),
			); err != nil {
				return err
			}
		}
	}

	if _, err := fmt.Fprintln(out, "\nBoundary\tread-only runtime safety view; not secret-value access, full RBAC, billing, capacity, or live utilization"); err != nil {
		return err
	}
	return out.Flush()
}

func boundaryNameIDLabel(name string, id string) string {
	name = strings.TrimSpace(name)
	id = strings.TrimSpace(id)
	switch {
	case name != "" && id != "":
		return fmt.Sprintf("%s (%s)", name, shortRuntimeResourceID(id))
	case name != "":
		return name
	case id != "":
		return id
	default:
		return "-"
	}
}

func boundaryRuntimeRefLabel(ref *boundaryRuntimeRef) string {
	if ref == nil {
		return "-"
	}
	parts := []string{}
	if strings.TrimSpace(ref.Adapter) != "" {
		parts = append(parts, "adapter="+strings.TrimSpace(ref.Adapter))
	}
	if strings.TrimSpace(ref.Kind) != "" {
		parts = append(parts, "kind="+strings.TrimSpace(ref.Kind))
	}
	if strings.TrimSpace(ref.Namespace) != "" || strings.TrimSpace(ref.Name) != "" {
		parts = append(parts, "resource="+runtimeOrphanResourceLabel(runtimeOrphanSummaryTableResource{
			Kind:      ref.Kind,
			Namespace: ref.Namespace,
			Name:      ref.Name,
		}))
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " ")
}

func formatBoundaryResourceRequests(values map[string]string) string {
	if len(values) == 0 {
		return "-"
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		if strings.TrimSpace(key) != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	items := make([]string, 0, len(keys))
	for _, key := range keys {
		value := strings.TrimSpace(values[key])
		if value == "" {
			continue
		}
		items = append(items, key+"="+value)
	}
	if len(items) == 0 {
		return "-"
	}
	return strings.Join(items, ",")
}

func formatBoundaryPorts(ports []boundaryPortSummary) string {
	items := make([]string, 0, len(ports))
	for _, port := range ports {
		if port.Port <= 0 {
			continue
		}
		name := strings.TrimSpace(port.Name)
		protocol := strings.TrimSpace(port.Protocol)
		if protocol == "" {
			protocol = "TCP"
		}
		label := strconv.Itoa(port.Port) + "/" + protocol
		if name != "" {
			label = name + ":" + label
		}
		items = append(items, label)
	}
	if len(items) == 0 {
		return "-"
	}
	sort.Strings(items)
	return strings.Join(items, ",")
}

func formatBoundarySecretRefs(refs []boundarySecretRef) string {
	items := make([]string, 0, len(refs))
	for _, ref := range refs {
		items = append(items, boundarySecretRefLabel(ref))
	}
	return formatStringList(cleanStringList(items))
}

func boundarySecretRefLabel(ref boundarySecretRef) string {
	name := strings.TrimSpace(ref.Name)
	key := strings.TrimSpace(ref.Key)
	if name == "" && key == "" {
		return "-"
	}
	if key == "" {
		return name
	}
	if name == "" {
		return "key:" + key
	}
	return name + "/" + key
}

func formatBoundaryCredentialRefs(refs []boundaryCredentialRef) string {
	items := make([]string, 0, len(refs))
	for _, ref := range refs {
		label := strings.TrimSpace(ref.Slug)
		if label == "" {
			label = strings.TrimSpace(ref.Name)
		}
		if label == "" {
			label = strings.TrimSpace(ref.ID)
		}
		parts := []string{tableValue(label, "credential")}
		if strings.TrimSpace(ref.Type) != "" {
			parts = append(parts, "type="+strings.TrimSpace(ref.Type))
		}
		if strings.TrimSpace(ref.SecretRef) != "" {
			parts = append(parts, "secret="+strings.TrimSpace(ref.SecretRef))
		}
		if len(ref.Usage) > 0 {
			parts = append(parts, "usage="+formatStringList(cleanStringList(ref.Usage)))
		}
		items = append(items, strings.Join(parts, " "))
	}
	return formatStringList(cleanStringList(items))
}

func writeProjectPolicySummary(w io.Writer, policy projectPolicySummary) error {
	out := bufio.NewWriter(w)
	if _, err := fmt.Fprintln(out, "PROJECT LAUNCH POLICY SUMMARY"); err != nil {
		return err
	}
	rows := [][2]string{
		{"Project", tableValue(policy.ProjectID, "unknown")},
		{"Enforcement", tableValue(policy.Enforcement, "disabled")},
		{"Allowed image prefixes", formatStringList(cleanStringList(policy.AllowedImagePrefixes))},
		{"Allowed service accounts", formatStringList(cleanStringList(policy.AllowedServiceAccounts))},
		{"Allowed secret refs", formatStringList(cleanStringList(policy.AllowedSecretRefs))},
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(out, "%s\t%s\n", row[0], row[1]); err != nil {
			return err
		}
	}
	if strings.TrimSpace(policy.CreatedAt) != "" || strings.TrimSpace(policy.UpdatedAt) != "" {
		if _, err := fmt.Fprintf(out, "Created at\t%s\nUpdated at\t%s\n", tableValue(policy.CreatedAt, "-"), tableValue(policy.UpdatedAt, "-")); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(out, "Boundary\tlaunch-policy gate for sandbox creation and template validation launches"); err != nil {
		return err
	}
	return out.Flush()
}

func writeProjectQuotaPolicySummary(w io.Writer, policy projectQuotaPolicySummary) error {
	out := bufio.NewWriter(w)
	if _, err := fmt.Fprintln(out, "PROJECT QUOTA POLICY SUMMARY"); err != nil {
		return err
	}
	rows := [][2]string{
		{"Project", tableValue(policy.ProjectID, "unknown")},
		{"Enforcement", tableValue(policy.Enforcement, "disabled")},
		{"Max active sandboxes", formatOptionalInt(policy.MaxActiveSandboxes)},
		{"Max retained artifact bytes", formatOptionalInt64(policy.MaxRetainedArtifactBytes)},
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(out, "%s\t%s\n", row[0], row[1]); err != nil {
			return err
		}
	}
	if strings.TrimSpace(policy.CreatedAt) != "" || strings.TrimSpace(policy.UpdatedAt) != "" {
		if _, err := fmt.Fprintf(out, "Created at\t%s\nUpdated at\t%s\n", tableValue(policy.CreatedAt, "-"), tableValue(policy.UpdatedAt, "-")); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(out, "Boundary\tproduct-record quota gate for active sandbox count and retained artifact bytes"); err != nil {
		return err
	}
	return out.Flush()
}

func formatOptionalInt(value *int) string {
	if value == nil {
		return "unlimited"
	}
	return strconv.Itoa(*value)
}

func formatOptionalInt64(value *int64) string {
	if value == nil {
		return "unlimited"
	}
	return strconv.FormatInt(*value, 10)
}

func writeProjectUsageSummary(w io.Writer, usage projectUsageSummary) error {
	out := bufio.NewWriter(w)
	if _, err := fmt.Fprintln(out, "PROJECT USAGE SUMMARY"); err != nil {
		return err
	}
	rows := [][2]string{
		{"Project", tableValue(usage.ProjectID, "unknown")},
		{"Generated at", tableValue(usage.GeneratedAt, "unknown")},
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(out, "%s\t%s\n", row[0], row[1]); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(out, "\nSANDBOXES"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "total\t%d\nactive\t%d\nrunning\t%d\npending\t%d\nstopped\t%d\nfailed\t%d\ndeleted\t%d\ncleanupPending\t%d\n",
		usage.Sandboxes.Total,
		usage.Sandboxes.Active,
		usage.Sandboxes.Running,
		usage.Sandboxes.Pending,
		usage.Sandboxes.Stopped,
		usage.Sandboxes.Failed,
		usage.Sandboxes.Deleted,
		usage.Sandboxes.CleanupPending,
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "\nDECLARED REQUESTS"); err != nil {
		return err
	}
	if err := writeProjectUsageRequestLine(out, "active", usage.Sandboxes.ActiveRequests); err != nil {
		return err
	}
	if err := writeProjectUsageRequestLine(out, "running", usage.Sandboxes.RunningRequests); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "\nRUNTIME SESSIONS"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "total\t%d\nactive\t%d\nended\t%d\nfailed\t%d\ntypes\t%s\n",
		usage.RuntimeSessions.Total,
		usage.RuntimeSessions.Active,
		usage.RuntimeSessions.Ended,
		usage.RuntimeSessions.Failed,
		formatRuntimeSessionUsageTypes(usage.RuntimeSessions),
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "\nEXECUTION TASKS"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "total\t%d\nqueued\t%d\nrunning\t%d\nsucceeded\t%d\nfailed\t%d\ncanceled\t%d\ntimedOut\t%d\n",
		usage.ExecutionTasks.Total,
		usage.ExecutionTasks.Queued,
		usage.ExecutionTasks.Running,
		usage.ExecutionTasks.Succeeded,
		usage.ExecutionTasks.Failed,
		usage.ExecutionTasks.Canceled,
		usage.ExecutionTasks.TimedOut,
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "\nARTIFACTS"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "total\t%d\nretainedContent\t%d\nreferencedBytes\t%d\nretainedBytes\t%d\nkinds\t%s\n",
		usage.Artifacts.Total,
		usage.Artifacts.RetainedContent,
		usage.Artifacts.ReferencedBytes,
		usage.Artifacts.RetainedBytes,
		formatArtifactUsageKinds(usage.Artifacts),
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "\nTEMPLATES"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "projectScoped\t%d\nglobalVisible\t%d\ncpuRequests\t%s\nmemoryRequests\t%s\nstorageRequests\t%s\n",
		usage.Templates.ProjectScoped,
		usage.Templates.GlobalVisible,
		formatResourceUsageValues(usage.Templates.CPURequests),
		formatResourceUsageValues(usage.Templates.MemoryRequests),
		formatResourceUsageValues(usage.Templates.StorageRequests),
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "\nCREDENTIAL REFERENCES"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "total\t%d\ntypes\t%s\n",
		usage.Credentials.Total,
		formatCredentialUsageTypes(usage.Credentials),
	); err != nil {
		return err
	}
	if len(usage.Notes) > 0 {
		if _, err := fmt.Fprintln(out, "\nNOTES"); err != nil {
			return err
		}
		for _, note := range usage.Notes {
			note = strings.TrimSpace(note)
			if note == "" {
				continue
			}
			if _, err := fmt.Fprintf(out, "- %s\n", note); err != nil {
				return err
			}
		}
	}
	return out.Flush()
}

func writeProjectUsageRequestLine(w io.Writer, label string, requests sandboxResourceRequestUsage) error {
	_, err := fmt.Fprintf(w, "%s\tcount=%d cpu=%s memory=%s storage=%s\n",
		label,
		requests.Count,
		formatResourceQuantityUsage(requests.CPU),
		formatResourceQuantityUsage(requests.Memory),
		formatResourceQuantityUsage(requests.Storage),
	)
	return err
}

func formatResourceQuantityUsage(value resourceQuantityUsage) string {
	total := strings.TrimSpace(value.Total)
	if total == "" {
		total = "-"
	}
	return fmt.Sprintf("%s(declared=%d missing=%d invalid=%d)", total, value.Declared, value.Missing, value.Invalid)
}

func formatRuntimeSessionUsageTypes(usage projectSessionUsageSummary) string {
	return formatNamedCounts([]runtimeResourceCountTable{
		{Name: "terminal", Count: usage.Terminal},
		{Name: "ide", Count: usage.IDE},
		{Name: "notebook", Count: usage.Notebook},
		{Name: "browser", Count: usage.Browser},
		{Name: "command", Count: usage.Command},
		{Name: "custom", Count: usage.Custom},
	})
}

func formatArtifactUsageKinds(usage projectArtifactUsageSummary) string {
	return formatNamedCounts([]runtimeResourceCountTable{
		{Name: "file", Count: usage.File},
		{Name: "directory", Count: usage.Directory},
		{Name: "log", Count: usage.Log},
		{Name: "report", Count: usage.Report},
		{Name: "screenshot", Count: usage.Screenshot},
		{Name: "image", Count: usage.Image},
		{Name: "link", Count: usage.Link},
		{Name: "other", Count: usage.Other},
	})
}

func formatCredentialUsageTypes(usage projectCredentialUsageSummary) string {
	return formatNamedCounts([]runtimeResourceCountTable{
		{Name: "git", Count: usage.Git},
		{Name: "registry", Count: usage.Registry},
		{Name: "kubernetes", Count: usage.Kubernetes},
		{Name: "ssh", Count: usage.SSH},
		{Name: "generic", Count: usage.Generic},
	})
}

func formatNamedCounts(values []runtimeResourceCountTable) string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		if value.Count <= 0 {
			continue
		}
		name := strings.TrimSpace(value.Name)
		if name == "" {
			name = "(empty)"
		}
		items = append(items, fmt.Sprintf("%s=%d", name, value.Count))
	}
	if len(items) == 0 {
		return "(none)"
	}
	sort.Strings(items)
	return strings.Join(items, " ")
}

func formatResourceUsageValues(values []resourceUsageValue) string {
	if len(values) == 0 {
		return "(none)"
	}
	items := append([]resourceUsageValue(nil), values...)
	sort.Slice(items, func(i, j int) bool {
		return strings.TrimSpace(items[i].Value) < strings.TrimSpace(items[j].Value)
	})
	parts := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.TrimSpace(item.Value)
		if value == "" {
			value = "(empty)"
		}
		parts = append(parts, fmt.Sprintf("%s=%d", value, item.Count))
	}
	return strings.Join(parts, " ")
}

func writeProjectCredentialsSummary(w io.Writer, projectID string, credentials projectCredentialListSummary) error {
	out := bufio.NewWriter(w)
	if _, err := fmt.Fprintln(out, "PROJECT CREDENTIAL REFERENCES SUMMARY"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Project\t%s\n", tableValue(projectID, "unknown")); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Total\t%d\n", len(credentials.Items)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Types\t%s\n", formatNamedCounts(projectCredentialTypeCounts(credentials.Items))); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Usage\t%s\n", formatNamedCounts(projectCredentialUsageCounts(credentials.Items))); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Secret refs\t%s\n", formatNamedCounts(projectCredentialSecretRefCounts(credentials.Items))); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "\nCREDENTIAL REFERENCES"); err != nil {
		return err
	}
	if len(credentials.Items) == 0 {
		if _, err := fmt.Fprintln(out, "  (none)"); err != nil {
			return err
		}
		return out.Flush()
	}
	items := append([]projectCredentialSummaryItem(nil), credentials.Items...)
	sort.Slice(items, func(i, j int) bool {
		left := projectCredentialSortKey(items[i])
		right := projectCredentialSortKey(items[j])
		if left == right {
			return strings.TrimSpace(items[i].ID) < strings.TrimSpace(items[j].ID)
		}
		return left < right
	})
	if _, err := fmt.Fprintln(out, "TYPE\tNAME\tTARGET\tSECRET_REF\tUSAGE\tID"); err != nil {
		return err
	}
	for _, item := range items {
		if _, err := fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\t%s\n",
			tableValue(item.Type, "unknown"),
			tableValue(item.Name, tableValue(item.Slug, "unknown")),
			tableValue(item.Target, "-"),
			projectCredentialSecretRefLabel(item.SecretRef),
			formatStringList(cleanStringList(item.Usage)),
			tableValue(item.ID, "-"),
		); err != nil {
			return err
		}
	}
	return out.Flush()
}

func projectCredentialTypeCounts(items []projectCredentialSummaryItem) []runtimeResourceCountTable {
	counts := map[string]int{}
	for _, item := range items {
		credentialType := strings.TrimSpace(item.Type)
		if credentialType == "" {
			credentialType = "unknown"
		}
		counts[credentialType]++
	}
	return projectMemberCounts(counts)
}

func projectCredentialUsageCounts(items []projectCredentialSummaryItem) []runtimeResourceCountTable {
	counts := map[string]int{}
	for _, item := range items {
		for _, usage := range item.Usage {
			usage = strings.TrimSpace(usage)
			if usage == "" {
				continue
			}
			counts[usage]++
		}
	}
	return projectMemberCounts(counts)
}

func projectCredentialSecretRefCounts(items []projectCredentialSummaryItem) []runtimeResourceCountTable {
	counts := map[string]int{}
	for _, item := range items {
		label := projectCredentialSecretRefLabel(item.SecretRef)
		if label == "-" {
			label = "missing"
		}
		counts[label]++
	}
	return projectMemberCounts(counts)
}

func projectCredentialSortKey(item projectCredentialSummaryItem) string {
	return strings.Join([]string{
		strings.TrimSpace(item.Type),
		strings.TrimSpace(item.Name),
		strings.TrimSpace(item.Slug),
	}, "/")
}

func projectCredentialSecretRefLabel(secret projectCredentialSecret) string {
	name := strings.TrimSpace(secret.Name)
	key := strings.TrimSpace(secret.Key)
	if name == "" && key == "" {
		return "-"
	}
	if key == "" {
		return name
	}
	if name == "" {
		return "key:" + key
	}
	return name + "/" + key
}

func cleanStringList(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			cleaned = append(cleaned, value)
		}
	}
	sort.Strings(cleaned)
	return cleaned
}

func writeProjectMembersSummary(w io.Writer, projectID string, members projectMemberListSummary) error {
	out := bufio.NewWriter(w)
	if _, err := fmt.Fprintln(out, "PROJECT MEMBERS SUMMARY"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Project\t%s\n", tableValue(projectID, "unknown")); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Total\t%d\n", len(members.Items)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Roles\t%s\n", formatNamedCounts(projectMemberRoleCounts(members.Items))); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Principal types\t%s\n", formatNamedCounts(projectMemberPrincipalTypeCounts(members.Items))); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "\nMEMBERS"); err != nil {
		return err
	}
	if len(members.Items) == 0 {
		if _, err := fmt.Fprintln(out, "  (none)"); err != nil {
			return err
		}
		return out.Flush()
	}
	items := append([]projectMemberSummaryItem(nil), members.Items...)
	sort.Slice(items, func(i, j int) bool {
		left := projectMemberSortKey(items[i])
		right := projectMemberSortKey(items[j])
		if left == right {
			return strings.TrimSpace(items[i].ID) < strings.TrimSpace(items[j].ID)
		}
		return left < right
	})
	if _, err := fmt.Fprintln(out, "ROLE\tPRINCIPAL\tTYPE\tID"); err != nil {
		return err
	}
	for _, item := range items {
		if _, err := fmt.Fprintf(out, "%s\t%s\t%s\t%s\n",
			tableValue(item.Role, "unknown"),
			tableValue(item.Principal, "unknown"),
			tableValue(item.PrincipalType, "unknown"),
			tableValue(item.ID, "-"),
		); err != nil {
			return err
		}
	}
	return out.Flush()
}

func projectMemberRoleCounts(items []projectMemberSummaryItem) []runtimeResourceCountTable {
	counts := map[string]int{}
	for _, item := range items {
		role := strings.TrimSpace(item.Role)
		if role == "" {
			role = "unknown"
		}
		counts[role]++
	}
	return projectMemberCounts(counts)
}

func projectMemberPrincipalTypeCounts(items []projectMemberSummaryItem) []runtimeResourceCountTable {
	counts := map[string]int{}
	for _, item := range items {
		principalType := strings.TrimSpace(item.PrincipalType)
		if principalType == "" {
			principalType = "unknown"
		}
		counts[principalType]++
	}
	return projectMemberCounts(counts)
}

func projectMemberCounts(counts map[string]int) []runtimeResourceCountTable {
	result := make([]runtimeResourceCountTable, 0, len(counts))
	for name, count := range counts {
		result = append(result, runtimeResourceCountTable{Name: name, Count: count})
	}
	return result
}

func projectMemberSortKey(item projectMemberSummaryItem) string {
	return strings.Join([]string{
		strings.TrimSpace(item.Role),
		strings.TrimSpace(item.PrincipalType),
		strings.TrimSpace(item.Principal),
	}, "/")
}

func writeCallerSummary(w io.Writer, caller callerSummaryInfo) error {
	out := bufio.NewWriter(w)
	if _, err := fmt.Fprintln(out, "CALLER BOUNDARY"); err != nil {
		return err
	}
	rows := [][2]string{
		{"Caller", projectAuthorizationCallerLabel(projectAuthorizationSummaryCaller(caller))},
		{"Mode", tableValue(caller.Mode, "unknown")},
		{"Principal", callerIdentityLabel(caller.PrincipalType, caller.Principal)},
		{"Authenticated", formatBool(caller.Authenticated)},
		{"Authentication required", formatBool(caller.AuthenticationRequired)},
		{"RBAC trusted", formatBool(caller.RBACTrusted)},
		{"Project roles enforced", formatBool(caller.ProjectRolesEnforced)},
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(out, "%s\t%s\n", row[0], row[1]); err != nil {
			return err
		}
	}
	if len(caller.Notes) > 0 {
		if _, err := fmt.Fprintln(out, "Notes"); err != nil {
			return err
		}
		for _, note := range caller.Notes {
			note = strings.TrimSpace(note)
			if note == "" {
				continue
			}
			if _, err := fmt.Fprintf(out, "- %s\n", note); err != nil {
				return err
			}
		}
	}
	return out.Flush()
}

func writeProjectAuthorizationSummary(w io.Writer, decision projectAuthorizationSummaryDecision) error {
	out := bufio.NewWriter(w)
	if _, err := fmt.Fprintln(out, "PROJECT AUTHORIZATION"); err != nil {
		return err
	}
	rows := [][2]string{
		{"Project", tableValue(decision.ProjectID, "unknown")},
		{"Action", tableValue(decision.Action, "project.view")},
		{"Decision", projectAuthorizationDecisionLabel(decision)},
		{"Required roles", formatStringList(decision.RequiredRoles)},
		{"Caller", projectAuthorizationCallerLabel(decision.Caller)},
		{"Matched member", projectAuthorizationMemberLabel(decision.MatchedMember)},
		{"Member records", strconv.Itoa(decision.MemberCount)},
		{"Available actions", formatStringList(decision.AvailableActions)},
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(out, "%s\t%s\n", row[0], row[1]); err != nil {
			return err
		}
	}
	if len(decision.Notes) > 0 {
		if _, err := fmt.Fprintln(out, "Notes"); err != nil {
			return err
		}
		for _, note := range decision.Notes {
			note = strings.TrimSpace(note)
			if note == "" {
				continue
			}
			if _, err := fmt.Fprintf(out, "- %s\n", note); err != nil {
				return err
			}
		}
	}
	return out.Flush()
}

func projectAuthorizationDecisionLabel(decision projectAuthorizationSummaryDecision) string {
	parts := []string{tableValue(decision.Evaluation, "unknown")}
	if decision.Enforced {
		parts = append(parts, "route enforced")
	} else {
		parts = append(parts, "not route enforced")
	}
	return strings.Join(parts, " / ")
}

func projectAuthorizationCallerLabel(caller projectAuthorizationSummaryCaller) string {
	identity := callerIdentityLabel(caller.PrincipalType, caller.Principal)
	mode := tableValue(caller.Mode, "unknown-mode")
	trust := "not trusted"
	if caller.RBACTrusted {
		trust = "rbac trusted"
	}
	enforcement := "roles not enforced"
	if caller.ProjectRolesEnforced {
		enforcement = "roles enforced"
	}
	auth := "unauthenticated"
	if caller.Authenticated {
		auth = "authenticated"
	}
	if caller.AuthenticationRequired {
		auth += ", auth required"
	}
	return fmt.Sprintf("%s (%s, %s, %s, %s)", identity, mode, auth, trust, enforcement)
}

func callerIdentityLabel(principalType string, principal string) string {
	identity := strings.TrimSpace(principal)
	if identity == "" {
		identity = "anonymous"
	}
	principalType = strings.TrimSpace(principalType)
	if principalType != "" {
		identity = principalType + ":" + identity
	}
	return identity
}

func projectAuthorizationMemberLabel(member *projectAuthorizationSummaryMember) string {
	if member == nil {
		return "-"
	}
	principal := tableValue(member.Principal, "unknown")
	if strings.TrimSpace(member.PrincipalType) != "" {
		principal = strings.TrimSpace(member.PrincipalType) + ":" + principal
	}
	role := tableValue(member.Role, "unknown-role")
	if strings.TrimSpace(member.ID) != "" {
		return fmt.Sprintf("%s role=%s id=%s", principal, role, strings.TrimSpace(member.ID))
	}
	return fmt.Sprintf("%s role=%s", principal, role)
}

func formatStringList(values []string) string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			items = append(items, value)
		}
	}
	if len(items) == 0 {
		return "-"
	}
	return strings.Join(items, ",")
}

func formatBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func writeAuditSummaryTable(w io.Writer, events []auditEventSummaryItem) error {
	out := bufio.NewWriter(w)
	if _, err := fmt.Fprintln(out, "AUDIT SUMMARY"); err != nil {
		return err
	}
	rows := auditSummaryRows(events)
	if len(rows) == 0 {
		if _, err := fmt.Fprintln(out, "  (none)"); err != nil {
			return err
		}
		return out.Flush()
	}
	if _, err := fmt.Fprintln(out, "ACTION\tCOUNT\tLATEST\tRESOURCE TYPES\tACTORS\tSOURCES"); err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(
			out,
			"%s\t%d\t%s\t%s\t%s\t%s\n",
			tableValue(row.Action, "unknown"),
			row.Count,
			formatAuditSummaryTime(row.Latest),
			formatAuditSummarySet(row.ResourceTypes),
			formatAuditSummarySet(row.Actors),
			formatAuditSummarySet(row.Sources),
		); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(out, "\nSummary\tread-only over returned best-effort audit events; not a transactional audit log or trusted identity source"); err != nil {
		return err
	}
	return out.Flush()
}

func auditSummaryRows(events []auditEventSummaryItem) []auditSummaryRow {
	groups := map[string]*auditSummaryRow{}
	for _, event := range events {
		action := strings.TrimSpace(event.Action)
		key := action
		row, ok := groups[key]
		if !ok {
			row = &auditSummaryRow{
				Action:        action,
				ResourceTypes: map[string]bool{},
				Actors:        map[string]bool{},
				Sources:       map[string]bool{},
			}
			groups[key] = row
		}
		row.Count++
		if event.CreatedAt.After(row.Latest) {
			row.Latest = event.CreatedAt
		}
		addAuditSummaryValue(row.ResourceTypes, event.ResourceType)
		addAuditSummaryValue(row.Actors, event.Actor)
		addAuditSummaryValue(row.Sources, event.Source)
	}
	rows := make([]auditSummaryRow, 0, len(groups))
	for _, row := range groups {
		rows = append(rows, *row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Count != rows[j].Count {
			return rows[i].Count > rows[j].Count
		}
		if !rows[i].Latest.Equal(rows[j].Latest) {
			return rows[i].Latest.After(rows[j].Latest)
		}
		return rows[i].Action < rows[j].Action
	})
	return rows
}

func writePolicyDeniedSummaryTable(w io.Writer, events []auditEventSummaryItem) error {
	out := bufio.NewWriter(w)
	if _, err := fmt.Fprintln(out, "POLICY DENIED SUMMARY"); err != nil {
		return err
	}
	rows := policyDeniedSummaryRows(events)
	if len(rows) == 0 {
		if _, err := fmt.Fprintln(out, "  (none)"); err != nil {
			return err
		}
		return out.Flush()
	}
	if _, err := fmt.Fprintln(out, "OPERATION\tREASON\tCOUNT\tLATEST\tACTORS\tSOURCES\tRESOURCES"); err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(
			out,
			"%s\t%s\t%d\t%s\t%s\t%s\t%s\n",
			tableValue(row.Operation, "unknown"),
			tableValue(row.Reason, "unspecified"),
			row.Count,
			formatAuditSummaryTime(row.Latest),
			formatAuditSummarySet(row.Actors),
			formatAuditSummarySet(row.Sources),
			formatAuditSummarySet(row.Resources),
		); err != nil {
			return err
		}
	}
	return out.Flush()
}

func policyDeniedSummaryRows(events []auditEventSummaryItem) []policyDeniedSummaryRow {
	groups := map[string]*policyDeniedSummaryRow{}
	for _, event := range events {
		if strings.TrimSpace(event.Action) != "policy.denied" {
			continue
		}
		operation, reason := policyDeniedMetadata(event.Metadata)
		key := operation + "\x00" + reason
		row, ok := groups[key]
		if !ok {
			row = &policyDeniedSummaryRow{
				Operation: operation,
				Reason:    reason,
				Actors:    map[string]bool{},
				Sources:   map[string]bool{},
				Resources: map[string]bool{},
			}
			groups[key] = row
		}
		row.Count++
		if event.CreatedAt.After(row.Latest) {
			row.Latest = event.CreatedAt
		}
		addAuditSummaryValue(row.Actors, event.Actor)
		addAuditSummaryValue(row.Sources, event.Source)
		resource := strings.TrimSpace(event.ResourceName)
		if resource == "" {
			resource = strings.TrimSpace(event.ResourceType)
		}
		addAuditSummaryValue(row.Resources, resource)
	}
	rows := make([]policyDeniedSummaryRow, 0, len(groups))
	for _, row := range groups {
		rows = append(rows, *row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Count != rows[j].Count {
			return rows[i].Count > rows[j].Count
		}
		if !rows[i].Latest.Equal(rows[j].Latest) {
			return rows[i].Latest.After(rows[j].Latest)
		}
		if rows[i].Operation != rows[j].Operation {
			return rows[i].Operation < rows[j].Operation
		}
		return rows[i].Reason < rows[j].Reason
	})
	return rows
}

func policyDeniedMetadata(raw json.RawMessage) (string, string) {
	var metadata map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &metadata) != nil {
		return "", ""
	}
	return auditSummaryMetadataString(metadata, "operation"), auditSummaryMetadataString(metadata, "reason")
}

func auditSummaryMetadataString(metadata map[string]any, key string) string {
	value, ok := metadata[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func addAuditSummaryValue(values map[string]bool, value string) {
	value = strings.TrimSpace(value)
	if value != "" {
		values[value] = true
	}
}

func formatAuditSummarySet(values map[string]bool) string {
	if len(values) == 0 {
		return "-"
	}
	items := make([]string, 0, len(values))
	for value := range values {
		items = append(items, value)
	}
	sort.Strings(items)
	if len(items) > 3 {
		return strings.Join(items[:3], ",") + fmt.Sprintf(",+%d", len(items)-3)
	}
	return strings.Join(items, ",")
}

func formatAuditSummaryTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.UTC().Format(time.RFC3339)
}

func tableValue(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
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

func parseJSONObjectFlag(name string, raw string) (json.RawMessage, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	value, err := ReadJSONObject(raw)
	if err != nil {
		return nil, fmt.Errorf("%s must be a JSON object: %w", name, err)
	}
	return value, nil
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
		return usageError("usage: mbox projects list|create|get|usage|authorization|members|add-member|audit-events|policy|set-policy|quota-policy|set-quota-policy|credentials|add-credential|delete")
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
		if len(args) < 2 {
			return usageError("usage: mbox projects usage <project-id> [--summary]")
		}
		fs := flag.NewFlagSet("projects usage", flag.ContinueOnError)
		fs.SetOutput(a.streams.Stderr)
		summary := fs.Bool("summary", false, "")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return usageError("usage: mbox projects usage <project-id> [--summary]")
		}
		path := "/v1/projects/" + url.PathEscape(args[1]) + "/usage"
		if *summary {
			var usage projectUsageSummary
			if err := client.JSON(ctx, http.MethodGet, path, nil, &usage); err != nil {
				return err
			}
			return writeProjectUsageSummary(a.streams.Stdout, usage)
		}
		return a.get(ctx, client, path)
	case "authorization", "authz":
		if len(args) < 2 {
			return usageError("usage: mbox projects authorization <project-id> [--action ACTION] [--summary]")
		}
		fs := flag.NewFlagSet("projects authorization", flag.ContinueOnError)
		fs.SetOutput(a.streams.Stderr)
		action := fs.String("action", "", "")
		summary := fs.Bool("summary", false, "")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return usageError("usage: mbox projects authorization <project-id> [--action ACTION] [--summary]")
		}
		path := "/v1/projects/" + url.PathEscape(args[1]) + "/authorization"
		if strings.TrimSpace(*action) != "" {
			values := url.Values{}
			values.Set("action", strings.TrimSpace(*action))
			path += "?" + values.Encode()
		}
		if *summary {
			var decision projectAuthorizationSummaryDecision
			if err := client.JSON(ctx, http.MethodGet, path, nil, &decision); err != nil {
				return err
			}
			return writeProjectAuthorizationSummary(a.streams.Stdout, decision)
		}
		return a.get(ctx, client, path)
	case "members":
		if len(args) < 2 {
			return usageError("usage: mbox projects members <project-id> [--summary]")
		}
		fs := flag.NewFlagSet("projects members", flag.ContinueOnError)
		fs.SetOutput(a.streams.Stderr)
		summary := fs.Bool("summary", false, "")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return usageError("usage: mbox projects members <project-id> [--summary]")
		}
		path := "/v1/projects/" + url.PathEscape(args[1]) + "/members"
		if *summary {
			var members projectMemberListSummary
			if err := client.JSON(ctx, http.MethodGet, path, nil, &members); err != nil {
				return err
			}
			return writeProjectMembersSummary(a.streams.Stdout, args[1], members)
		}
		return a.get(ctx, client, path)
	case "add-member":
		if len(args) < 2 {
			return usageError("usage: mbox projects add-member <project-id> --principal PRINCIPAL --role owner|operator|viewer [--principal-type user|service_account|automation]")
		}
		projectID := args[1]
		fs := flag.NewFlagSet("projects add-member", flag.ContinueOnError)
		fs.SetOutput(a.streams.Stderr)
		principalType := fs.String("principal-type", "user", "")
		principal := fs.String("principal", "", "")
		role := fs.String("role", "", "")
		metadata := fs.String("metadata", "", "")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		rawMetadata, err := parseMetadataFlag(fs, *metadata)
		if err != nil {
			return err
		}
		payload := map[string]any{
			"principalType": *principalType,
			"principal":     *principal,
			"role":          *role,
		}
		SetRaw(payload, "metadata", rawMetadata)
		return a.post(ctx, client, "/v1/projects/"+url.PathEscape(projectID)+"/members", payload)
	case "audit-events":
		if len(args) < 2 {
			return usageError("usage: mbox projects audit-events <project-id> [--action ACTION] [--resource-type TYPE] [--resource-id ID] [--actor ACTOR] [--source SOURCE] [--filter-request-id ID] [--operation OPERATION] [--reason REASON] [--since RFC3339] [--until RFC3339] [--limit N] [--summary] [--policy-denied-summary]")
		}
		return a.runAuditEvents(ctx, client, args[2:], args[1])
	case "policy":
		if len(args) < 2 {
			return usageError("usage: mbox projects policy <project-id> [--summary]")
		}
		fs := flag.NewFlagSet("projects policy", flag.ContinueOnError)
		fs.SetOutput(a.streams.Stderr)
		summary := fs.Bool("summary", false, "")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return usageError("usage: mbox projects policy <project-id> [--summary]")
		}
		path := "/v1/projects/" + url.PathEscape(args[1]) + "/policy"
		if *summary {
			var policy projectPolicySummary
			if err := client.JSON(ctx, http.MethodGet, path, nil, &policy); err != nil {
				return err
			}
			return writeProjectPolicySummary(a.streams.Stdout, policy)
		}
		return a.get(ctx, client, path)
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
		if len(args) < 2 {
			return usageError("usage: mbox projects quota-policy <project-id> [--summary]")
		}
		fs := flag.NewFlagSet("projects quota-policy", flag.ContinueOnError)
		fs.SetOutput(a.streams.Stderr)
		summary := fs.Bool("summary", false, "")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return usageError("usage: mbox projects quota-policy <project-id> [--summary]")
		}
		path := "/v1/projects/" + url.PathEscape(args[1]) + "/quota-policy"
		if *summary {
			var policy projectQuotaPolicySummary
			if err := client.JSON(ctx, http.MethodGet, path, nil, &policy); err != nil {
				return err
			}
			return writeProjectQuotaPolicySummary(a.streams.Stdout, policy)
		}
		return a.get(ctx, client, path)
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
		if len(args) < 2 {
			return usageError("usage: mbox projects credentials <project-id> [--summary]")
		}
		fs := flag.NewFlagSet("projects credentials", flag.ContinueOnError)
		fs.SetOutput(a.streams.Stderr)
		summary := fs.Bool("summary", false, "")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return usageError("usage: mbox projects credentials <project-id> [--summary]")
		}
		path := "/v1/projects/" + url.PathEscape(args[1]) + "/credentials"
		if *summary {
			var credentials projectCredentialListSummary
			if err := client.JSON(ctx, http.MethodGet, path, nil, &credentials); err != nil {
				return err
			}
			return writeProjectCredentialsSummary(a.streams.Stdout, args[1], credentials)
		}
		return a.get(ctx, client, path)
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
		return usageError("usage: mbox projects list|create|get|usage|authorization|members|add-member|audit-events|policy|set-policy|quota-policy|set-quota-policy|credentials|add-credential|delete")
	}
}

func (a *App) runTemplate(ctx context.Context, client *Client, args []string) error {
	if len(args) == 0 {
		return usageError("usage: mbox templates list|create|get|boundary|validate|validate-run|decide-validation|delete")
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
	case "create":
		fs := flag.NewFlagSet("templates create", flag.ContinueOnError)
		fs.SetOutput(a.streams.Stderr)
		projectID := fs.String("project-id", "", "")
		name := fs.String("name", "", "")
		slug := fs.String("slug", "", "")
		image := fs.String("image", "", "")
		command := fs.String("command", "", "")
		commandJSON := fs.String("command-json", "", "")
		var commandArgs stringListFlag
		fs.Var(&commandArgs, "arg", "")
		workingDir := fs.String("working-dir", "", "")
		cpuRequest := fs.String("cpu-request", "", "")
		memoryRequest := fs.String("memory-request", "", "")
		storageRequest := fs.String("storage-request", "", "")
		env := fs.String("env", "", "")
		metadata := fs.String("metadata", "", "")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return usageError("usage: mbox templates create --name NAME --image IMAGE [--project-id PROJECT] [--arg ARG]")
		}
		startupCommand, err := parseCommandFlags(commandArgs, *command, *commandJSON)
		if err != nil {
			return err
		}
		rawEnv, err := parseJSONObjectFlag("env", *env)
		if err != nil {
			return err
		}
		rawMetadata, err := parseMetadataFlag(fs, *metadata)
		if err != nil {
			return err
		}
		payload := map[string]any{
			"name":  *name,
			"image": *image,
		}
		SetNonEmpty(payload, "projectId", *projectID)
		SetNonEmpty(payload, "slug", *slug)
		if len(startupCommand) > 0 {
			payload["startupCommand"] = startupCommand
		}
		SetNonEmpty(payload, "workingDir", *workingDir)
		SetNonEmpty(payload, "cpuRequest", *cpuRequest)
		SetNonEmpty(payload, "memoryRequest", *memoryRequest)
		SetNonEmpty(payload, "storageRequest", *storageRequest)
		SetRaw(payload, "env", rawEnv)
		SetRaw(payload, "metadata", rawMetadata)
		return a.post(ctx, client, "/v1/templates", payload)
	case "get":
		if len(args) != 2 {
			return usageError("usage: mbox templates get <template-id>")
		}
		return a.get(ctx, client, "/v1/templates/"+url.PathEscape(args[1]))
	case "delete":
		if len(args) != 2 {
			return usageError("usage: mbox templates delete <template-id>")
		}
		return a.delete(ctx, client, "/v1/templates/"+url.PathEscape(args[1]))
	case "boundary":
		if len(args) < 2 {
			return usageError("usage: mbox templates boundary <template-id> [--project-id PROJECT] [--summary]")
		}
		templateID := args[1]
		fs := flag.NewFlagSet("templates boundary", flag.ContinueOnError)
		fs.SetOutput(a.streams.Stderr)
		projectID := fs.String("project-id", "", "")
		summary := fs.Bool("summary", false, "")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return usageError("usage: mbox templates boundary <template-id> [--project-id PROJECT] [--summary]")
		}
		path := "/v1/templates/" + url.PathEscape(templateID) + "/boundary"
		if *projectID != "" {
			path += "?projectId=" + url.QueryEscape(*projectID)
		}
		if *summary {
			var boundary boundarySummary
			if err := client.JSON(ctx, http.MethodGet, path, nil, &boundary); err != nil {
				return err
			}
			return writeBoundarySummary(a.streams.Stdout, boundary)
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
	case "validate-run":
		return a.runTemplateValidateRun(ctx, client, args[1:])
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
		return usageError("usage: mbox templates list|create|get|boundary|validate|validate-run|decide-validation|delete")
	}
}

func (a *App) runTemplateValidateRun(ctx context.Context, client *Client, args []string) error {
	input, err := a.parseTemplateValidateRun(args)
	if err != nil {
		return err
	}
	validationPayload := map[string]any{}
	SetNonEmpty(validationPayload, "projectId", input.ProjectID)
	SetNonEmpty(validationPayload, "name", input.Name)
	SetRaw(validationPayload, "metadata", input.ValidationMetadata)

	var validation map[string]any
	if err := client.JSON(ctx, http.MethodPost, "/v1/templates/"+url.PathEscape(input.TemplateID)+"/validation-runs", validationPayload, &validation); err != nil {
		return err
	}
	sandboxID, err := validationSandboxID(validation)
	if err != nil {
		return err
	}
	sandbox, err := a.waitForSandboxValue(ctx, client, sandboxID, sandboxWaitOptions{
		Status:            "running",
		Interval:          input.Interval,
		Timeout:           input.WaitTimeout,
		RequireRuntimeRef: true,
	})
	if err != nil {
		return a.failTemplateValidationRun(ctx, client, input.TemplateID, sandboxID, validation, sandbox, nil, err)
	}

	taskPayload := map[string]any{
		"command":        input.Command,
		"timeoutSeconds": input.TaskTimeoutSeconds,
	}
	SetRaw(taskPayload, "metadata", input.TaskMetadata)
	var createdTask map[string]any
	if err := client.JSON(ctx, http.MethodPost, "/v1/sandboxes/"+url.PathEscape(sandboxID)+"/tasks", taskPayload, &createdTask); err != nil {
		return a.failTemplateValidationRun(ctx, client, input.TemplateID, sandboxID, validation, sandbox, nil, err)
	}
	taskID, _ := createdTask["id"].(string)
	if strings.TrimSpace(taskID) == "" {
		return a.failTemplateValidationRun(ctx, client, input.TemplateID, sandboxID, validation, sandbox, nil, fmt.Errorf("task create response missing id"))
	}
	task, err := a.waitForTaskValue(ctx, client, taskID, taskWaitOptions{
		Interval:       input.Interval,
		Timeout:        input.WaitTimeout,
		RequireSuccess: false,
	})
	if err != nil {
		return a.failTemplateValidationRun(ctx, client, input.TemplateID, sandboxID, validation, sandbox, task, err)
	}
	taskStatus, _ := task["status"].(string)
	decisionStatus := "failed"
	if taskStatus == "succeeded" {
		decisionStatus = "passed"
	}
	decision, err := a.decideTemplateValidation(ctx, client, input.TemplateID, sandboxID, decisionStatus)
	if err != nil {
		return err
	}

	result := map[string]any{
		"validation":     validation,
		"sandbox":        sandbox,
		"task":           task,
		"decision":       decision,
		"decisionStatus": decisionStatus,
		"status":         decisionStatus,
	}
	if err := WriteJSON(a.streams.Stdout, result); err != nil {
		return err
	}
	if input.RequireSuccess && taskStatus != "succeeded" {
		return fmt.Errorf("template validation task %s finished with status %s", taskID, taskStatus)
	}
	return nil
}

func (a *App) failTemplateValidationRun(
	ctx context.Context,
	client *Client,
	templateID string,
	sandboxID string,
	validation map[string]any,
	sandbox map[string]any,
	task map[string]any,
	cause error,
) error {
	decision, decisionErr := a.decideTemplateValidation(ctx, client, templateID, sandboxID, "failed")
	if decisionErr != nil {
		return errors.Join(cause, decisionErr)
	}
	result := map[string]any{
		"validation":     validation,
		"sandbox":        sandbox,
		"decision":       decision,
		"decisionStatus": "failed",
		"status":         "failed",
	}
	if task != nil {
		result["task"] = task
	}
	if writeErr := WriteJSON(a.streams.Stdout, result); writeErr != nil {
		return writeErr
	}
	return cause
}

func (a *App) decideTemplateValidation(ctx context.Context, client *Client, templateID string, sandboxID string, status string) (map[string]any, error) {
	var decision map[string]any
	if err := client.JSON(
		ctx,
		http.MethodPost,
		"/v1/templates/"+url.PathEscape(templateID)+"/validation-runs/"+url.PathEscape(sandboxID)+"/decision",
		map[string]any{"status": status},
		&decision,
	); err != nil {
		return nil, err
	}
	return decision, nil
}

type templateValidateRunInput struct {
	TemplateID         string
	ProjectID          string
	Name               string
	Command            []string
	TaskTimeoutSeconds int
	Interval           time.Duration
	WaitTimeout        time.Duration
	RequireSuccess     bool
	ValidationMetadata json.RawMessage
	TaskMetadata       json.RawMessage
}

func (a *App) parseTemplateValidateRun(args []string) (templateValidateRunInput, error) {
	if len(args) < 1 {
		return templateValidateRunInput{}, usageError("usage: mbox templates validate-run <template-id> --project-id PROJECT [--name NAME] [--wait-timeout 5m] [--task-timeout 60] [--require-success] -- sh -lc 'echo ok'")
	}
	input := templateValidateRunInput{TemplateID: args[0]}
	fs := flag.NewFlagSet("templates validate-run", flag.ContinueOnError)
	fs.SetOutput(a.streams.Stderr)
	projectID := fs.String("project-id", "", "")
	name := fs.String("name", "", "")
	command := fs.String("command", "", "")
	commandJSON := fs.String("command-json", "", "")
	var commandArgs stringListFlag
	fs.Var(&commandArgs, "arg", "")
	taskTimeoutSeconds := fs.Int("task-timeout", 60, "")
	intervalRaw := fs.String("interval", "1500ms", "")
	waitTimeoutRaw := fs.String("wait-timeout", "", "")
	requireSuccess := fs.Bool("require-success", false, "")
	validationMetadata := fs.String("metadata", "", "")
	taskMetadata := fs.String("task-metadata", "", "")
	if err := fs.Parse(args[1:]); err != nil {
		return templateValidateRunInput{}, err
	}
	positionalCommand := fs.Args()
	if len(positionalCommand) > 0 && (len(commandArgs) > 0 || strings.TrimSpace(*command) != "" || strings.TrimSpace(*commandJSON) != "") {
		return templateValidateRunInput{}, usageError("use only one of --arg, --command, --command-json, or positional command after --")
	}
	parsedCommand, err := parseCommandFlags(commandArgs, *command, *commandJSON)
	if err != nil {
		return templateValidateRunInput{}, err
	}
	if len(positionalCommand) > 0 {
		parsedCommand = positionalCommand
	}
	if len(parsedCommand) == 0 {
		return templateValidateRunInput{}, usageError("usage: mbox templates validate-run <template-id> --project-id PROJECT [--name NAME] [--wait-timeout 5m] [--task-timeout 60] [--require-success] -- sh -lc 'echo ok'")
	}
	if *taskTimeoutSeconds <= 0 {
		return templateValidateRunInput{}, usageError("task-timeout must be greater than zero")
	}
	interval, err := parsePositiveDuration(*intervalRaw, "interval")
	if err != nil {
		return templateValidateRunInput{}, err
	}
	var waitTimeout time.Duration
	if strings.TrimSpace(*waitTimeoutRaw) != "" {
		waitTimeout, err = parsePositiveDuration(*waitTimeoutRaw, "wait-timeout")
		if err != nil {
			return templateValidateRunInput{}, err
		}
	}
	rawValidationMetadata, err := parseMetadataFlag(fs, *validationMetadata)
	if err != nil {
		return templateValidateRunInput{}, err
	}
	rawTaskMetadata, err := parseMetadataFlag(fs, *taskMetadata)
	if err != nil {
		return templateValidateRunInput{}, err
	}
	input.ProjectID = *projectID
	input.Name = *name
	input.Command = parsedCommand
	input.TaskTimeoutSeconds = *taskTimeoutSeconds
	input.Interval = interval
	input.WaitTimeout = waitTimeout
	input.RequireSuccess = *requireSuccess
	input.ValidationMetadata = rawValidationMetadata
	input.TaskMetadata = rawTaskMetadata
	return input, nil
}

func validationSandboxID(validation map[string]any) (string, error) {
	sandbox, ok := validation["sandbox"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("template validation response missing sandbox")
	}
	sandboxID, _ := sandbox["id"].(string)
	if strings.TrimSpace(sandboxID) == "" {
		return "", fmt.Errorf("template validation response missing sandbox id")
	}
	return sandboxID, nil
}

func (a *App) runSandbox(ctx context.Context, client *Client, args []string) error {
	if len(args) == 0 {
		return usageError("usage: mbox sandboxes list|create|get|boundary|start|stop|wait|delete")
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
		if len(args) < 2 {
			return usageError("usage: mbox sandboxes boundary <sandbox-id> [--summary]")
		}
		fs := flag.NewFlagSet("sandboxes boundary", flag.ContinueOnError)
		fs.SetOutput(a.streams.Stderr)
		summary := fs.Bool("summary", false, "")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return usageError("usage: mbox sandboxes boundary <sandbox-id> [--summary]")
		}
		path := "/v1/sandboxes/" + url.PathEscape(args[1]) + "/boundary"
		if *summary {
			var boundary boundarySummary
			if err := client.JSON(ctx, http.MethodGet, path, nil, &boundary); err != nil {
				return err
			}
			return writeBoundarySummary(a.streams.Stdout, boundary)
		}
		return a.get(ctx, client, path)
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
	case "wait":
		return a.runSandboxWait(ctx, client, args[1:])
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
		return usageError("usage: mbox sandboxes list|create|get|boundary|start|stop|wait|delete")
	}
}

func (a *App) runSandboxWait(ctx context.Context, client *Client, args []string) error {
	fs := flag.NewFlagSet("sandboxes wait", flag.ContinueOnError)
	fs.SetOutput(a.streams.Stderr)
	status := fs.String("status", "running", "")
	intervalRaw := fs.String("interval", "1500ms", "")
	timeoutRaw := fs.String("timeout", "", "")
	requireRuntimeRef := fs.Bool("require-runtime-ref", false, "")
	sandboxID := ""
	parseArgs := args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		sandboxID = strings.TrimSpace(args[0])
		parseArgs = args[1:]
	}
	if err := fs.Parse(parseArgs); err != nil {
		return err
	}
	if sandboxID == "" && fs.NArg() == 1 {
		sandboxID = strings.TrimSpace(fs.Arg(0))
	} else if fs.NArg() != 0 {
		return usageError("usage: mbox sandboxes wait <sandbox-id> [--status running] [--interval 1500ms] [--timeout 5m] [--require-runtime-ref]")
	}
	if sandboxID == "" {
		return usageError("usage: mbox sandboxes wait <sandbox-id> [--status running] [--interval 1500ms] [--timeout 5m] [--require-runtime-ref]")
	}
	if !isSandboxStatus(*status) {
		return usageError("status must be one of pending, running, stopped, failed, or deleted")
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
	return a.waitForSandbox(ctx, client, sandboxID, sandboxWaitOptions{
		Status:            *status,
		Interval:          interval,
		Timeout:           timeout,
		RequireRuntimeRef: *requireRuntimeRef,
	})
}

func (a *App) waitForSandbox(ctx context.Context, client *Client, sandboxID string, options sandboxWaitOptions) error {
	sandbox, err := a.waitForSandboxValue(ctx, client, sandboxID, options)
	if sandbox != nil {
		if writeErr := WriteJSON(a.streams.Stdout, sandbox); writeErr != nil {
			return writeErr
		}
	}
	return err
}

func (a *App) waitForSandboxValue(ctx context.Context, client *Client, sandboxID string, options sandboxWaitOptions) (map[string]any, error) {
	if options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	}
	interval := options.Interval
	if interval <= 0 {
		interval = 1500 * time.Millisecond
	}
	expectedStatus := strings.TrimSpace(options.Status)
	if expectedStatus == "" {
		expectedStatus = "running"
	}
	lastStatus := ""
	lastRuntimeRef := false
	for {
		var sandbox map[string]any
		if err := client.JSON(ctx, http.MethodGet, "/v1/sandboxes/"+url.PathEscape(sandboxID), nil, &sandbox); err != nil {
			return nil, err
		}
		status, _ := sandbox["status"].(string)
		lastStatus = status
		lastRuntimeRef = hasRuntimeRef(sandbox)
		if status == expectedStatus {
			if options.RequireRuntimeRef && !lastRuntimeRef {
				if err := sleepContext(ctx, interval); err != nil {
					return nil, fmt.Errorf("timed out waiting for sandbox %s to reach status %s with runtimeRef; last status=%s runtimeRef=%t", sandboxID, expectedStatus, lastStatus, lastRuntimeRef)
				}
				continue
			}
			return sandbox, nil
		}
		if isTerminalSandboxStatus(status) {
			return sandbox, fmt.Errorf("sandbox %s reached terminal status %s while waiting for %s", sandboxID, status, expectedStatus)
		}
		if err := sleepContext(ctx, interval); err != nil {
			suffix := ""
			if options.RequireRuntimeRef {
				suffix = " with runtimeRef"
			}
			return nil, fmt.Errorf("timed out waiting for sandbox %s to reach status %s%s; last status=%s runtimeRef=%t", sandboxID, expectedStatus, suffix, lastStatus, lastRuntimeRef)
		}
	}
}

type sandboxWaitOptions struct {
	Status            string
	Interval          time.Duration
	Timeout           time.Duration
	RequireRuntimeRef bool
}

func hasRuntimeRef(sandbox map[string]any) bool {
	runtimeRef, ok := sandbox["runtimeRef"].(map[string]any)
	if !ok {
		return false
	}
	name, _ := runtimeRef["name"].(string)
	namespace, _ := runtimeRef["namespace"].(string)
	return strings.TrimSpace(name) != "" && strings.TrimSpace(namespace) != ""
}

func isSandboxStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "pending", "running", "stopped", "failed", "deleted":
		return true
	default:
		return false
	}
}

func isTerminalSandboxStatus(status string) bool {
	switch status {
	case "failed", "deleted":
		return true
	default:
		return false
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
		return usageError(tasksUsage)
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
	case "artifacts":
		if len(args) != 2 {
			return usageError("usage: mbox tasks artifacts <task-id>")
		}
		return a.get(ctx, client, "/v1/tasks/"+url.PathEscape(args[1])+"/artifacts")
	case "wait":
		return a.runTaskWait(ctx, client, args[1:])
	case "run":
		return a.runTaskRun(ctx, client, args[1:])
	case "create":
		sandboxID, payload, err := a.parseTaskCreatePayload(args[1:], "tasks create")
		if err != nil {
			return err
		}
		return a.post(ctx, client, "/v1/sandboxes/"+url.PathEscape(sandboxID)+"/tasks", payload)
	default:
		return usageError(tasksUsage)
	}
}

const tasksUsage = "usage: mbox tasks list|create|run|get|cancel|watch|wait|artifacts"
const taskCreateUsage = "usage: mbox tasks create <sandbox-id> (--arg ARG...|--command CMD|--command-json JSON) [--timeout 60]"

func (a *App) parseTaskCreatePayload(args []string, commandName string) (string, map[string]any, error) {
	if len(args) < 1 {
		return "", nil, usageError(taskCommandUsage(commandName))
	}
	sandboxID := args[0]
	fs := flag.NewFlagSet(commandName, flag.ContinueOnError)
	fs.SetOutput(a.streams.Stderr)
	command := fs.String("command", "", "")
	commandJSON := fs.String("command-json", "", "")
	var commandArgs stringListFlag
	fs.Var(&commandArgs, "arg", "")
	timeoutSeconds := fs.Int("timeout", 60, "")
	metadata := fs.String("metadata", "", "")
	if err := fs.Parse(args[1:]); err != nil {
		return "", nil, err
	}
	positionalCommand := fs.Args()
	rawMetadata, err := parseMetadataFlag(fs, *metadata)
	if err != nil {
		return "", nil, err
	}
	if len(positionalCommand) != 0 {
		return "", nil, usageError(taskCommandUsage(commandName))
	}
	parsedCommand, err := parseCommandFlags(commandArgs, *command, *commandJSON)
	if err != nil {
		return "", nil, err
	}
	payload := map[string]any{
		"command":        parsedCommand,
		"timeoutSeconds": *timeoutSeconds,
	}
	SetRaw(payload, "metadata", rawMetadata)
	return sandboxID, payload, nil
}

func taskCommandUsage(commandName string) string {
	if commandName == "tasks create" {
		return taskCreateUsage
	}
	return "usage: mbox " + commandName + " <sandbox-id> --command 'sh -lc echo-ok'"
}

func (a *App) runTaskWait(ctx context.Context, client *Client, args []string) error {
	fs := flag.NewFlagSet("tasks wait", flag.ContinueOnError)
	fs.SetOutput(a.streams.Stderr)
	intervalRaw := fs.String("interval", "1500ms", "")
	timeoutRaw := fs.String("timeout", "", "")
	requireSuccess := fs.Bool("require-success", false, "")
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
		return usageError("usage: mbox tasks wait <task-id> [--interval 1500ms] [--timeout 5m] [--require-success]")
	}
	if taskID == "" {
		return usageError("usage: mbox tasks wait <task-id> [--interval 1500ms] [--timeout 5m] [--require-success]")
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
	return a.waitForTask(ctx, client, taskID, taskWaitOptions{
		Interval:       interval,
		Timeout:        timeout,
		RequireSuccess: *requireSuccess,
	})
}

func (a *App) waitForTask(ctx context.Context, client *Client, taskID string, options taskWaitOptions) error {
	task, err := a.waitForTaskValue(ctx, client, taskID, options)
	if task != nil {
		if writeErr := WriteJSON(a.streams.Stdout, task); writeErr != nil {
			return writeErr
		}
	}
	return err
}

func (a *App) waitForTaskValue(ctx context.Context, client *Client, taskID string, options taskWaitOptions) (map[string]any, error) {
	if options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	}
	interval := options.Interval
	if interval <= 0 {
		interval = 1500 * time.Millisecond
	}
	for {
		var task map[string]any
		if err := client.JSON(ctx, http.MethodGet, "/v1/tasks/"+url.PathEscape(taskID), nil, &task); err != nil {
			return nil, err
		}
		if isTerminalTaskStatus(task["status"]) {
			status, _ := task["status"].(string)
			if options.RequireSuccess && status != "succeeded" {
				return task, fmt.Errorf("task %s finished with status %s", taskID, status)
			}
			return task, nil
		}
		if err := sleepContext(ctx, interval); err != nil {
			return nil, fmt.Errorf("timed out waiting for task %s", taskID)
		}
	}
}

func (a *App) runTaskRun(ctx context.Context, client *Client, args []string) error {
	sandboxID, payload, waitOptions, err := a.parseTaskRunPayload(args)
	if err != nil {
		return err
	}
	var task map[string]any
	if err := client.JSON(ctx, http.MethodPost, "/v1/sandboxes/"+url.PathEscape(sandboxID)+"/tasks", payload, &task); err != nil {
		return err
	}
	taskID, ok := task["id"].(string)
	if !ok || strings.TrimSpace(taskID) == "" {
		return fmt.Errorf("task create response missing id")
	}
	return a.waitForTask(ctx, client, strings.TrimSpace(taskID), waitOptions)
}

type taskWaitOptions struct {
	Interval       time.Duration
	Timeout        time.Duration
	RequireSuccess bool
}

func (a *App) parseTaskRunPayload(args []string) (string, map[string]any, taskWaitOptions, error) {
	if len(args) < 1 {
		return "", nil, taskWaitOptions{}, usageError("usage: mbox tasks run <sandbox-id> [--timeout 60] [--interval 1500ms] [--wait-timeout 5m] [--require-success] -- sh -lc 'echo ok'")
	}
	sandboxID := args[0]
	fs := flag.NewFlagSet("tasks run", flag.ContinueOnError)
	fs.SetOutput(a.streams.Stderr)
	command := fs.String("command", "", "")
	commandJSON := fs.String("command-json", "", "")
	var commandArgs stringListFlag
	fs.Var(&commandArgs, "arg", "")
	taskTimeoutSeconds := fs.Int("timeout", 60, "")
	metadata := fs.String("metadata", "", "")
	intervalRaw := fs.String("interval", "1500ms", "")
	waitTimeoutRaw := fs.String("wait-timeout", "", "")
	requireSuccess := fs.Bool("require-success", false, "")
	if err := fs.Parse(args[1:]); err != nil {
		return "", nil, taskWaitOptions{}, err
	}
	positionalCommand := fs.Args()
	rawMetadata, err := parseMetadataFlag(fs, *metadata)
	if err != nil {
		return "", nil, taskWaitOptions{}, err
	}
	if len(positionalCommand) > 0 && (len(commandArgs) > 0 || strings.TrimSpace(*command) != "" || strings.TrimSpace(*commandJSON) != "") {
		return "", nil, taskWaitOptions{}, usageError("use only one of --arg, --command, --command-json, or positional command after --")
	}
	parsedCommand, err := parseCommandFlags(commandArgs, *command, *commandJSON)
	if err != nil {
		return "", nil, taskWaitOptions{}, err
	}
	if len(positionalCommand) > 0 {
		parsedCommand = positionalCommand
	}
	payload := map[string]any{
		"command":        parsedCommand,
		"timeoutSeconds": *taskTimeoutSeconds,
	}
	SetRaw(payload, "metadata", rawMetadata)

	interval, err := parsePositiveDuration(*intervalRaw, "interval")
	if err != nil {
		return "", nil, taskWaitOptions{}, err
	}
	var waitTimeout time.Duration
	if strings.TrimSpace(*waitTimeoutRaw) != "" {
		waitTimeout, err = parsePositiveDuration(*waitTimeoutRaw, "wait-timeout")
		if err != nil {
			return "", nil, taskWaitOptions{}, err
		}
	}
	return sandboxID, payload, taskWaitOptions{
		Interval:       interval,
		Timeout:        waitTimeout,
		RequireSuccess: *requireSuccess,
	}, nil
}

func (a *App) runArtifact(ctx context.Context, client *Client, args []string) error {
	if len(args) == 0 {
		return usageError("usage: mbox artifacts list|create|get|capture|content|upload")
	}
	switch args[0] {
	case "list":
		if len(args) != 2 {
			return usageError("usage: mbox artifacts list <sandbox-id>")
		}
		return a.get(ctx, client, "/v1/sandboxes/"+url.PathEscape(args[1])+"/artifacts")
	case "create":
		if len(args) < 2 {
			return usageError("usage: mbox artifacts create <sandbox-id> --kind KIND --name NAME --uri URI")
		}
		sandboxID := args[1]
		fs := flag.NewFlagSet("artifacts create", flag.ContinueOnError)
		fs.SetOutput(a.streams.Stderr)
		taskID := fs.String("task-id", "", "")
		kind := fs.String("kind", "", "")
		name := fs.String("name", "", "")
		uri := fs.String("uri", "", "")
		contentType := fs.String("content-type", "", "")
		sizeBytes := fs.Int64("size-bytes", -1, "")
		metadata := fs.String("metadata", "", "")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return usageError("usage: mbox artifacts create <sandbox-id> --kind KIND --name NAME --uri URI")
		}
		rawMetadata, err := parseMetadataFlag(fs, *metadata)
		if err != nil {
			return err
		}
		payload := map[string]any{
			"kind": *kind,
			"name": *name,
			"uri":  *uri,
		}
		SetNonEmpty(payload, "taskId", *taskID)
		SetNonEmpty(payload, "contentType", *contentType)
		if *sizeBytes >= 0 {
			payload["sizeBytes"] = *sizeBytes
		}
		SetRaw(payload, "metadata", rawMetadata)
		return a.post(ctx, client, "/v1/sandboxes/"+url.PathEscape(sandboxID)+"/artifacts", payload)
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
		return usageError("usage: mbox artifacts list|create|get|capture|content|upload")
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

func (a *App) runMember(ctx context.Context, client *Client, args []string) error {
	if len(args) == 0 {
		return usageError("usage: mbox members get|delete <member-id>")
	}
	switch args[0] {
	case "get":
		if len(args) != 2 {
			return usageError("usage: mbox members get <member-id>")
		}
		return a.get(ctx, client, "/v1/members/"+url.PathEscape(args[1]))
	case "delete":
		if len(args) != 2 {
			return usageError("usage: mbox members delete <member-id>")
		}
		return a.delete(ctx, client, "/v1/members/"+url.PathEscape(args[1]))
	default:
		return usageError("usage: mbox members get|delete <member-id>")
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

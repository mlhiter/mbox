# ROADMAP

## Phase 0: Product and Technical Validation

Goal: confirm the product boundary for an independent Kubernetes execution platform built on `agent-sandbox` for interactive runtimes.

Deliverables:

- Product documents in this repo.
- Runtime integration plan for `agent-sandbox`.
- Kubernetes Job usage boundary for isolated batch tasks that do not need an interactive sandbox.
- Long-term technical surface split: server, web app, CLI, API docs, and SDK package.
- Minimum supported Kubernetes version decision.
- Initial namespace, RBAC, storage, and network policy model.
- Clickable console prototype or low-fidelity UI map.

Exit criteria:

- Clear product boundary around environment templates, sandboxes, runtime sessions, execution tasks, previews, artifacts, and policy.
- Confirmed first runtime adapter boundary around `agent-sandbox`.
- MVP scope small enough to implement without building an agent platform or a full CI/CD platform.

## Phase 1: Core Sandbox MVP

Goal: let a human create, open, inspect, and destroy a Kubernetes-backed sandbox.

Current progress:

- Done: initial Go API server.
- Done: Postgres-backed Project, EnvironmentTemplate, and Sandbox models.
- Done: CRUD routes for `/v1/projects`, `/v1/templates`, and `/v1/sandboxes`.
- Done: opt-in sandbox reconciler that projects Sandbox records to `agent-sandbox` `SandboxTemplate` and `SandboxClaim` resources.
- Done: basic runtime status mapping from `SandboxClaim` Ready condition to mbox sandbox status.
- Done: separate Vite web console with project, template, and sandbox list/create/inspect workflows.
- Done: Notion-adjacent console design system captured in `DESIGN.md`.
- Done: browser terminal for running sandboxes through Kubernetes `pods/exec`.
- Done: runtime target, logs, and Kubernetes events API routes plus main-workspace runtime tabs.
- Done: real `agent-sandbox` cluster smoke verification against `kind-agent-sandbox`.
- Done: declared preview port metadata and API-proxied open links for running sandbox TCP ports.
- Done: PVC-backed workspace projection, runtime storage metadata, and smoke coverage for persistence across Pod replacement.
- Done: simplified sandbox launch UX with generated slugs, namespace defaults, and default sandbox ServiceAccount.
- Done: Node.js workspace template defaults for local terminal and preview testing.
- Done: pending runtime workspace state with polling before terminal/logs/events/preview runtime calls.
- Done: manual preview port declaration from the Preview tab.
- Done: sandbox stop/start lifecycle actions that pause and resume the projected runtime while preserving the product record.
- Done: E2B-style template library surface that treats templates as ready-to-run environments, with Essentials first and raw image/command/policy fields behind Advanced settings.
- Done: sandbox-backed execution tasks with asynchronous command runs, persisted status, stdout/stderr, exit result, timeout/cancellation state, and Runtime Workspace Tasks tab.
- Done: artifact reference records for sandbox and task outputs, with API and Runtime Workspace registration/listing.
- Done: read-only template and sandbox boundary summaries across API, CLI, SDK, smoke tests, and the Runtime Workspace Boundary tab. These expose namespace, ServiceAccount, token automount, secret reference projection, network policy projection, lifecycle policy projection, runtime access, and cleanup behavior without claiming full policy or credential enforcement.
- Done: retained artifact content starter with API/CLI/SDK/Web support for capturing small `workspace://` files, accepting client-uploaded bytes, and serving retained bytes after sandbox cleanup.
- Done: S3-compatible retained artifact content backend starter for remote durability while keeping Postgres as the artifact metadata index.
- Done: first project-scoped launch policy object with API/CLI/SDK/Web visibility and enforcement on sandbox creation and template validation launches for allowed image prefixes, sandbox ServiceAccounts, and template secret reference names.
- Done: project credential-reference starter with API/CLI/SDK/Web visibility and Boundary summary projection for typed Secret references without storing or mounting credential values.
- Done: lifecycle `ttlSeconds` enforcement in the reconciler, using the normal soft-delete and runtime cleanup path.
- Done: project deletion cleanup guard that blocks project hard-delete until sandboxes are deleted and runtime references are cleared.
- Done: runtime orphan audit and explicitly gated one-resource cleanup across API, CLI, SDK, and runtime smoke coverage for detecting and safely removing managed resource drift.
- Done: read-only project usage summaries across API, CLI, SDK, and Web for product-record visibility into sandboxes, sessions, tasks, artifacts, template requests, declared active/running sandbox request totals, credential references, retained bytes, and cleanup-pending rows.
- Done: best-effort product audit-event starter across API, CLI, SDK, and Web project inspector visibility for successful mbox API mutations.
- Done: project quota policy starter across API, CLI, SDK, Web visibility, and smoke coverage, enforcing active sandbox count and retained artifact bytes from product records.
- Done: Web launch preflight visibility for policy and active-sandbox quota blockers, with server-side policy/quota enforcement remaining authoritative.
- Done: `policy.denied` audit-event starter for launch-policy, active-sandbox quota, and retained-artifact-byte quota denials.
- Done: OpenAPI and TypeScript SDK audit contract hardening for known audit actions and typed `policy.denied` metadata without expanding audit into a strong transactional log.
- Done: project member role registry starter across API, Postgres, CLI, SDK, OpenAPI, and Web project inspector visibility for future RBAC, without enforcing authorization from the current shared-token API.
- Done: read-only caller/auth handshake across API, CLI, SDK, OpenAPI, Web rail, docs, and smoke coverage, reporting anonymous or shared-token mode plus explicit RBAC trust/enforcement flags without adding login or route-level authorization.
- Done: project authorization preflight across API, CLI, SDK, OpenAPI, Web project inspector, docs, and smoke coverage, mapping project actions to required roles and later reporting action-level enforcement state.

Scope:

- Project model.
- Environment template model.
- Sandbox model.
- Runtime adapter for `agent-sandbox`.
- Initial Go API server.
- Initial controller/reconciler.
- Basic web console.
- Terminal access.
- Basic port listing and preview endpoint.
- PVC-backed workspace.
- Sandbox lifecycle: create, start, stop, delete.
- Basic logs and events view.
- Minimal API documentation for the implemented sandbox endpoints.
- Asynchronous sandbox-backed execution task API and UI.
- Artifact reference API and UI for output metadata.

Out of scope:

- Full pipeline editor.
- Production deployment target.
- Agent planning or autonomous coding workflow.
- Full SDK package.
- Multi-cluster support.
- Billing.

Exit criteria:

- A user can create a sandbox from a template.
- A user can enter the sandbox from the browser.
- A user can expose and open a preview port.
- A user can see runtime status, logs, and Kubernetes events.
- A user can delete the sandbox and associated resources.

## Phase 2: Template and Policy Management

Goal: make sandboxes reusable and governable.

Scope:

- Template creation and editing UI.
- Template validation by launching a test sandbox.
- Read-only boundary summaries that make the current namespace, identity, secret, network, lifecycle, runtime access, and cleanup contract inspectable before full policy management exists.
- First enforceable project launch policy for ordinary sandbox launches and validation launches.
- Project credential-reference registry for narrow Git, registry, Kubernetes, SSH, or generic Secret references.
- Resource presets for CPU, memory, GPU, and storage.
- Environment variables and secret references.
- Lifecycle policy: TTL, idle timeout, auto cleanup.
- Namespace-scoped RBAC setup.
- Network policy presets.
- Quota display.

Exit criteria:

- Platform users can create a template and make it available to a project.
- Ordinary users can launch only allowed templates.
- Sandboxes respect resource and lifecycle policies.
- Secret references are visible as references, not leaked values.

## Phase 3: Runtime Sessions and Execution Tasks

Goal: make mbox useful as a programmable execution substrate for external agents, developer tools, CI systems, and human operators without making mbox own those upper-layer workflows.

Current progress:

- Done: runtime session records with Postgres persistence, sandbox/session API routes, SDK wrappers, and a Runtime Workspace Sessions tab.
- Done: terminal WebSocket connections automatically create terminal session records and close them as ended or failed.
- Done: sandbox-backed execution task model and Postgres table.
- Done: `POST /v1/sandboxes/{sandboxID}/tasks`, `GET /v1/sandboxes/{sandboxID}/tasks`, `GET /v1/tasks/{taskID}`, and `POST /v1/tasks/{taskID}/cancel`.
- Done: asynchronous command execution path through runtime `pods/exec`, with timeout, cancellation, and output truncation.
- Done: Runtime Workspace Tasks tab for running, polling, canceling, and inspecting command tasks.
- Done: artifact records and Runtime Workspace Artifacts tab for registering files, reports, screenshots, logs, images, links, or other output references.
- Done: `GET /v1/tasks/{taskID}/events` task watch stream with snapshot, status, output, and done events, plus SDK/Web live output support.
- Done: `GET /v1/artifacts/{artifactID}/content` for running-sandbox `workspace://` file artifact reads inside the resolved workspace mount.
- Done: `POST /v1/artifacts/{artifactID}/capture` for retaining small workspace-file artifact bytes with sha256 metadata.
- Done: filesystem retained-content provider starter, with `MBOX_ARTIFACT_CONTENT_BACKEND=filesystem`, provider/key metadata in Postgres, and the same artifact content download route.
- Done: S3-compatible retained-content provider starter, with `MBOX_ARTIFACT_CONTENT_BACKEND=s3`, server-side SigV4 PUT/GET, provider/key metadata in Postgres, and the same artifact content download route.
- Done: lifecycle `ttlSeconds` auto-cleanup through the runtime reconciler.
- Done: `PUT /v1/artifacts/{artifactID}/content` for client-provided retained artifact bytes, with CLI/SDK wrappers and smoke coverage.
- Done: project deletion guard to prevent cascading away sandbox rows while runtime cleanup is still pending.

Scope:

- Extend runtime sessions beyond audit records into richer attachment metadata or protocol-specific session records when needed.
- Extend execution tasks beyond the first sandbox command MVP.
- Cleanup state and richer log streaming for execution tasks.
- Extend artifact records beyond workspace file capture/upload into retention policy, object-store download, and managed remote storage-provider integration.
- Preview records beyond raw sandbox port declarations when useful.
- API and UI for inspecting sessions, tasks, logs, previews, and artifacts.
- Kubernetes Job adapter for isolated batch tasks when a full interactive sandbox is unnecessary.

Execution model:

- Interactive sandboxes remain stateful.
- Sessions attach to sandboxes.
- Tasks can run inside a sandbox when they need shared workspace state.
- Tasks can run as Kubernetes Jobs when they are isolated and repeatable.
- External agents, CI systems, IDEs, and release tools decide why work is running; mbox records and controls where and how it runs.

Exit criteria:

- A client can start a task against a sandbox and watch its status, logs, exit result, and artifacts through the API.
- A user can inspect active sessions and task history from the console.
- A canceled or timed-out task cleans up its runtime resources.
- Preview links and artifacts are attached to the runtime or task that produced them.

## Phase 3.5: CLI and SDK Foundation

Goal: make mbox usable outside the web console by developers, external agents, automation clients, and CI scripts.

Scope:

- CLI authentication and context selection.
- CLI commands for project, policy, credential-reference, template, sandbox, session, task, logs, ports, previews, and artifacts.
- API schema publication for implemented resources.
- API documentation site or generated docs for public endpoints.
- First official SDK package for automation clients.
- Versioning rules across server API, CLI, docs, and SDK.

Exit criteria:

- A user can create, inspect, enter, and delete a sandbox from the CLI.
- An external client can start and watch a task through the CLI or SDK.
- API docs match the implemented server behavior.
- The SDK can authenticate and call the core sandbox, session, task, preview, and artifact APIs.

Current status:

- Started: TypeScript SDK in `sdk/typescript` with typed wrappers for API info/version/capability handshake, caller/auth boundary handshake including trusted-header mode, project authorization preflight, runtime orphan audit/cleanup, health, projects, project policy, project quota policy, project member role records, project credential references, templates, template validation, template/sandbox boundary summaries, sandboxes, runtime target/log/event/port reads, runtime sessions, execution tasks, task polling/watch/cancel, artifact references, retained artifact upload/capture/content, and workspace artifact content fallback.
- Started: Go CLI in `cmd/mbox` as a thin HTTP client for API info/version/capability handshake, caller/auth boundary handshake, project authorization preflight, runtime orphan audit/cleanup, health, projects, project policy, project quota policy, project member role records, project credential references, templates, template validation, template/sandbox boundary summaries, sandboxes, sessions, tasks, artifacts, retained artifact upload/capture/content, logs, ports, and terminal access.
- Done: `GET /v1/info` as a read-only server capability manifest for API version, server version, runtime controller/access state, artifact-content backend, and CLI/SDK compatibility hints.
- Done: `GET /v1/openapi.json` OpenAPI contract starter plus CLI/SDK readers and smoke coverage for implemented public routes and schemas.
- Done: SDK route contract plus OpenAPI path/method alignment check for route-backed TypeScript helpers.
- Done: audit-event list helpers for global and project-scoped product mutation history.
- Done: request correlation starter with `X-Mbox-Request-ID` response headers, server log correlation, CLI/SDK request-id options, and audit metadata request IDs for write events.
- Done: client-supplied audit actor/source attribution and filtering across API, CLI, SDK, Web project inspector, docs, and CLI smoke coverage.
- Done: typed audit-event action and `policy.denied` metadata contracts in OpenAPI and TypeScript SDK.
- Done: audit-event `action` filtering across API, CLI, SDK, Web project inspector, OpenAPI, and docs for narrowing operational feeds without turning audit into a strong transactional log.
- Done: audit-event `requestId` filtering across API, Postgres indexes, CLI, SDK, OpenAPI, docs, and CLI smoke path for retrieving request-correlated audit feed slices.
- Done: audit-event metadata `operation` filtering across API, Postgres indexes, CLI, SDK, OpenAPI, docs, and CLI smoke path for narrowing typed policy-denial feeds.
- Done: audit-event metadata `reason` filtering across API, Postgres indexes, CLI, SDK, Web, OpenAPI, and docs for narrowing policy-denial investigations without treating audit metadata as authorization.
- Done: audit-event inclusive `since` / `until` time-window filtering across API, CLI, SDK, OpenAPI, docs, and tests for bounded operator investigation windows.
- Done: audit-event action query indexes for global and project-scoped feeds ordered by recency.
- Done: Web project inspector policy-denial ergonomics for highlighting `policy.denied` audit rows and surfacing operation, reason, authorization action, caller boundary, and resource hints without treating audit metadata as trusted identity.
- Done: SDK/OpenAPI alignment guard now checks SDK-used query parameters in addition to route paths and methods.
- Done: SDK/OpenAPI alignment guard now checks focused SDK-consumed schema required fields and properties for project usage and audit contracts.
- Done: SDK/OpenAPI alignment guard now checks focused SDK helper response schemas, list item refs, NDJSON task events, binary responses, and no-content delete routes.
- Done: SDK/OpenAPI alignment guard now checks focused SDK helper request body schema refs and binary upload media types.
- Done: SDK/OpenAPI alignment guard now checks reverse published-route coverage, so ordinary OpenAPI operations must have SDK route contract entries while terminal WebSocket and preview proxy pass-through routes stay explicit non-helper exceptions.
- Done: SDK/OpenAPI alignment guard now checks policy/quota denial fields on `PolicyDeniedAuditMetadata`, including `policyKind`, `enforcement`, active-sandbox limits, and retained-artifact byte limits.
- Done: SDK/OpenAPI alignment guard now checks project launch-policy and quota-policy response/upsert schemas, including required `projectId` on policy responses.
- Done: SDK/OpenAPI alignment guard now checks project credential-reference and `SecretRef` schemas without expanding credential records into secret-value storage or runtime mounting.
- Done: SDK/OpenAPI alignment guard now checks template-library schemas for `EnvironmentTemplate`, `TemplateCreate`, `TemplateUpdate`, and `TemplatePort`, including forbidden immutable update fields such as `projectId` and `slug`.
- Done: explicit SDK and CLI compatibility preflight helpers compare client API labels with the server `/v1/info` minimum CLI/SDK API versions before longer automation runs.
- Done: SDK and CLI compatibility preflight can require server capabilities such as `execution-tasks`, `task-events`, and `artifact-client-upload` before clients start a longer run.
- Done: SDK local smoke gate now builds the package and exercises compatibility helpers plus OpenAPI alignment success/failure paths without requiring a live API server.
- Done: SDK package dry-run gate now verifies `npm pack` would include the README, package manifest, compiled JavaScript, and TypeScript declarations while excluding source-only files.
- Done: SDK package consumer smoke now installs a real local `npm pack` tarball into a minimal ESM consumer and verifies exported helpers load from the installed package.
- Done: SDK publish gate now runs typecheck, local smoke, package dry-run, and package consumer smoke via `npm run verify`, with `prepublishOnly` wired to the same gate.
- Done: starter API compatibility policy documented and regression-tested for same-family `vNalphaM`, `vNbetaM`, and stable `vN` ordering, with capabilities kept as separate feature gates.
- Done: starter shared API token model for automation clients: optional `MBOX_API_TOKEN` server gate, public health/info discovery, CLI `MBOX_TOKEN`/`--token`, SDK `token`, and smoke/test coverage.
- Done: OpenAPI auth contract hardening for the starter token model: `bearerAuth` security scheme, explicit public health/info operations, private bearer operations, `401` error responses, and SDK alignment checks for route auth metadata.
- Done: caller/auth handshake: `GET /v1/auth/caller`, CLI `auth caller` / `caller`, SDK `caller()`, and Web rail visibility report anonymous, shared-token, or trusted-header mode plus RBAC trust/enforcement flags.
- Done: project authorization preflight: `GET /v1/projects/{projectID}/authorization?action=...`, CLI `projects authorization`, SDK `getProjectAuthorization()`, OpenAPI, Web project inspector visibility, docs, and smoke coverage map actions to required roles and action-level enforcement state.
- Done: trusted principal header provider starter for explicit reverse-proxy/local-smoke identity, disabled by default and surfaced through `/v1/info`, `/v1/auth/caller`, OpenAPI, SDK, Web rail/project inspector, docs, and smoke coverage.
- Done: disabled-by-default `sandbox.launch` project RBAC enforcement starter across API config, caller/preflight/info surfaces, sandbox creation, template validation launches, OpenAPI, SDK, Web, docs, and tests.
- Done: disabled-by-default `artifact.write` project RBAC enforcement starter across API config, caller/preflight/info surfaces, artifact reference creation, workspace artifact capture, client artifact-content upload, OpenAPI, SDK, docs, and tests.
- Done: disabled-by-default `runtime.operate` project RBAC enforcement starter across API config, caller/preflight/info surfaces, runtime target/log/event/preview/terminal, runtime session create/end, execution task create/cancel/events, workspace artifact fallback reads, SDK request-option propagation, OpenAPI, docs, and tests.
- Done: disabled-by-default `policy.manage` project RBAC enforcement starter across API config, caller/preflight/info surfaces, project launch/quota policy update routes, OpenAPI, SDK, Web audit metadata rendering, docs, and tests; when enabled with trusted principal headers, policy updates require an owner project member match.
- Done: disabled-by-default `credential.manage` project RBAC enforcement starter across API config, caller/preflight/info surfaces, project credential-reference create/delete routes, OpenAPI, SDK, Web preflight visibility, docs, and tests; when enabled with trusted principal headers, credential-reference mutations require an owner project member match while read/list routes stay visible.
- Done: disabled-by-default `member.manage` project RBAC enforcement starter across API config, caller/preflight/info surfaces, project member create/delete routes, OpenAPI, SDK, docs, and tests; when enabled with trusted principal headers, project creation seeds the trusted caller as owner for that project and subsequent member mutations require an owner project member match while list/get routes stay visible.
- Done: starter CLI context selection through `--context`, `MBOX_CONTEXT`, `--config`, `MBOX_CONFIG`, and `~/.mbox/config.json`, with API URL/token/audit-label loading, explicit flag overrides, and local `context current/list` inspection that redacts token values.
- Done: CLI local context management with `context set`, `context use`, and `context remove`, including parent-directory creation, token-env support for reusable configs, redacted JSON output, and CLI smoke coverage against an authenticated API.
- Done: CLI context preflight through `context check`, combining redacted context selection, `/healthz`, `/v1/info`, and capability-aware CLI compatibility checks without implying login, whoami, RBAC, or token validity.
- Done: TypeScript SDK environment factory `createMboxClientFromEnv()` for automation scripts that share the CLI `MBOX_API_URL`, `MBOX_TOKEN`/`MBOX_API_TOKEN`, and audit-label conventions without reading local context files.
- Done: CLI `sandboxes wait` and SDK `waitForSandbox()` polling conveniences for scripts that need a sandbox to reach `running` with a resolved `runtimeRef` before calling runtime routes.
- Done: CLI `templates validate-run` composition for scriptable template validation: create validation sandbox, wait for runtime readiness, run one execution task, and write a passed/failed validation decision without introducing a new server-side workflow primitive.
- Done: TypeScript SDK `runTemplateValidation()` composition over the same existing primitives, with `validationMetadata`, `task.command`, polling `timeoutMs`/`intervalMs`, and `requireSuccess` error handling for automation clients without introducing a server-side workflow or CI engine.
- Done: CLI `tasks wait --require-success` and SDK `waitForTask({ requireSuccess: true })` terminal success gates for automation scripts that need SDK-style task terminal-state waiting without parsing the NDJSON watch stream.
- Done: CLI task-scoped artifact listing through `tasks artifacts <task-id>`, matching the existing task artifact API and SDK helper.
- Done: CLI one-shot execution through `tasks run <sandbox-id> -- ...`, composing task creation with terminal polling while keeping `--timeout` as task execution timeout and `--wait-timeout` as the client wait limit, with runtime smoke coverage for the successful task/artifact path.
- Done: TypeScript SDK live API smoke for disposable local APIs, covering project/template/sandbox/session/artifact upload/content/audit paths without enabling runtime access or Kubernetes reconciliation.
- Done: TypeScript SDK runtime smoke for running sandboxes, covering SDK task creation, `waitForTask({ requireSuccess: true })`, task-scoped artifact listing, workspace capture, and retained content reads against the real `agent-sandbox` runtime.
- Done: read-only runtime managed-resource inventory, namespace/kind filtering, live kind/namespace/owner summary, and structured OpenAPI/SDK orphan-audit contracts across API, CLI, SDK, OpenAPI, docs, and smoke coverage, reusing the runtime auditor without adding automatic cleanup or new write paths.
- Done: Web Runtime inventory view at `#runtime` for read-only operator triage over `/v1/runtime/resources`, including summary, owner, label, and disabled-auditor handling without adding runtime write actions.
- Done: Web project inspector audit-feed ergonomics for request ID, operation, and RFC3339 time-window filters, with trace metadata display when present.
- Done: Web project inspector grouped `policy.denied` quick navigation for recurring denial operation/reason families, using the existing audit feed filters without adding a new audit model.
- Done: Runtime inventory workload observation for resolved SandboxClaim Pod phase, readiness, restart count, summed requests/limits, and PVC state across API, OpenAPI, SDK, Web, and runtime smoke coverage without adding metrics-server utilization or quota semantics.
- Done: Runtime inventory workload rollups for filtered observed resources, desired/observed/running Pods, container readiness, restarts, summed requests/limits, and PVC capacity across API, OpenAPI, SDK, Web, docs, and smoke coverage.
- Done: Web Runtime inventory project filter for label-derived per-project runtime attribution and filtered workload rollups, without treating the view as RBAC, quota, billing, or live capacity.
- Done: Runtime inventory `summary.byProject` rollup across API, OpenAPI, SDK, Web, docs, and tests, deriving project attribution only from runtime owner labels after current filters are applied.
- Done: Web Runtime inventory clickable project attribution strip over `summary.byProject`, reusing the existing read-only `projectId` filter for operator triage without adding quota, billing, RBAC, or capacity semantics.
- Done: CLI runtime summary table can optionally resolve known project IDs to project names through a read-only `/v1/projects` request, preserving JSON output, API contracts, and label-derived attribution semantics.
- Done: CLI runtime orphan audit summary table renders the existing `/v1/runtime/orphans` response as reason counts and actionable resource rows for human operators, preserving default JSON output and the explicitly gated one-resource cleanup model.
- Done: CLI project usage summary renders the existing read-only `/v1/projects/{projectID}/usage` response as project-record counts and declared active/running sandbox request totals for human operators, preserving default JSON output and avoiding live metrics, billing, quota reservation, or capacity semantics.
- Done: CLI project member summary renders the existing read-only `/v1/projects/{projectID}/members` response as role/principal-type counts plus member rows for starter RBAC operator triage, preserving default JSON output and avoiding login/session/identity semantics.
- Done: CLI project credential-reference summary renders the existing read-only `/v1/projects/{projectID}/credentials` response as type, usage, Secret-reference counts, and rows for operator triage, preserving default JSON output and avoiding secret-value storage, exposure, or runtime mounting semantics.
- Done: CLI project policy and quota-policy summaries render the existing read-only launch-policy and quota-policy responses as compact operator text, preserving default JSON output and avoiding broader RBAC, credential mounting, billing, capacity reservation, or live Kubernetes metrics semantics.
- Done: CLI template and sandbox boundary summaries render existing read-only boundary responses as compact operator text, preserving default JSON output and avoiding secret-value access, broader RBAC, credential mounting, custom NetworkPolicy projection, or live utilization semantics.
- Done: CLI audit-event summary renders returned best-effort audit rows by action, resource type, actor, and source for first-pass operator triage while preserving default JSON output and the existing audit model.
- Done: CLI audit help/usage now advertises both generic `--summary` and policy-specific `--policy-denied-summary` modes for global and project-scoped audit feeds, keeping the implemented operator ergonomics discoverable without changing the audit model.
- Done: OpenAPI and SDK schema alignment now publish structured boundary summary/check/credential-reference schemas and guard their required fields, properties, and enum values without expanding boundary summaries into secret access, full RBAC, billing, capacity, or live utilization semantics.
- Done: SDK/OpenAPI reverse route-coverage guard for published operations, with explicit non-helper exceptions for terminal WebSocket and preview proxy pass-through routes.
- Done: SDK/OpenAPI `PolicyDeniedAuditMetadata` schema guard now covers policy/quota denial metadata fields used by operator audit rendering.
- Done: SDK/OpenAPI project policy and quota policy schema guard now covers response/upsert fields for launch and quota policy clients.
- Done: SDK/OpenAPI project credential-reference schema guard now covers `ProjectCredential`, `ProjectCredentialCreate`, and `SecretRef` fields for reference-only credential clients.
- Done: SDK/OpenAPI template-library schema guard now covers template response/create/update/port fields and catches immutable-field drift in update schemas.
- Done: SDK/OpenAPI sandbox and preview schema guard now covers `RuntimeRef`, `SandboxPort`, `Sandbox`, `SandboxCreate`, `SandboxUpdate`, `PreviewPort`, and `PreviewPortsResult`, including immutable sandbox update-field drift.
- Done: SDK/OpenAPI runtime session schema guard now covers runtime session response/create fields for SDK session-history clients.
- Done: SDK/OpenAPI execution task schema guard now covers task response/create/watch-event fields for SDK task clients, including persisted output and timing metadata.
- Done: SDK/OpenAPI artifact schema guard now covers artifact response/create and retained-content metadata fields for SDK artifact clients without expanding artifact storage semantics.
- Done: SDK/OpenAPI project schema guard now covers project response/create/update fields and catches immutable project update-field drift.
- Done: SDK/OpenAPI runtime log schema guard now covers `LogResult` target/log fields for SDK runtime log clients.
- Done: SDK/OpenAPI API info schema guard now covers runtime/artifact capability and compatibility sub-schemas used by CLI/SDK preflight.
- Done: SDK/OpenAPI base response schema guard now covers public health and shared error response fields used by clients.
- Done: SDK/OpenAPI list response guard now checks SDK list helpers publish required `items` arrays, matching the SDK `ListResponse<T>` contract.
- Done: SDK/OpenAPI array item-ref guard now checks selected runtime nested arrays such as `RuntimeTarget.storage`, runtime inventory workload storage/issues, and preview-port result items, keeping runtime client schemas from drifting into loose object arrays.
- Done: Web project inspector audit ergonomics now surfaces member-management denial metadata for requested member principal type, principal, and role as debugging context without treating audit metadata as trusted identity.
- Done: Web project inspector member-management ergonomics now supports registering and removing project member records for starter RBAC, including refreshed authorization/audit visibility and a front-end guard against removing the last visible owner, without adding login, invites, or broader route-level RBAC.
- Remaining: broader route-level user/project RBAC enforcement beyond the current disabled-by-default `sandbox.launch`, `runtime.operate`, `artifact.write`, `policy.manage`, `credential.manage`, and `member.manage` starters, real package publication/release workflow, generated client and full schema alignment, broader CLI ergonomics, and future versioning decisions beyond the current starter policy.

## Phase 4: Upper-layer Workflow Integrations

Goal: prove that the execution platform can support CI, preview deployment, and release automation without making those workflows the base product model.

Scope:

- Optional pipeline definition and pipeline run integration.
- Optional deployment target and deployment run integration.
- Target-scoped credential references.
- Preview namespace or staging namespace execution patterns.
- Build output and image digest artifacts.
- Rollout status and service endpoint artifacts when a deployment workflow is used.
- Approval gate integration for sensitive upper-layer workflows.

Exit criteria:

- A CI system can use mbox to run a test/build flow and collect logs plus artifacts.
- A release tool can use mbox to run a preview deployment workflow with target-scoped permissions.
- Upper-layer workflow records reference sessions, tasks, previews, and artifacts rather than bypassing them.
- Production-like targets require explicit permission and audit.

## Phase 5: Operational Hardening

Goal: make the platform safe enough for shared clusters.

Scope:

- Audit logs.
- Admission checks for allowed images and resources.
- Stronger runtime isolation with RuntimeClass where available.
- Network egress controls.
- Idle detection.
- Orphan resource detection and cleanup.
- Quota enforcement.
- Multi-tenant metrics.
- Backup and restore plan for control-plane state.

Exit criteria:

- Operators can identify who created, accessed, deployed, or deleted each resource.
- Stale sandboxes and volumes are cleaned safely.
- Policy and quota denial reasons are visible in the UI before known-blocked launches and in API error feedback.
- Runtime resource usage is observable at project and user levels.

## Phase 6: Advanced Platform Capabilities

Goal: expand from MVP to mature platform.

Candidate features:

- Warm pools for faster sandbox startup.
- Browser and notebook sandbox templates.
- GPU templates.
- Multi-cluster deployment targets.
- YAML import/export for templates and upper-layer workflow integrations.
- Git provider integrations.
- Registry browsing and image promotion.
- Fine-grained approval workflows.
- Scheduled pipelines.
- API tokens for automation clients.
- Runtime adapter plugin system.

These should be pulled into earlier phases only if they unblock the MVP, not because they are generally useful.

## Near-term First Slice

First slice status:

1. Done: API server with Projects, Templates, and Sandboxes.
2. Done: controller that creates one `agent-sandbox` runtime per Sandbox record when explicitly enabled.
3. Done: Vite web console with project list, template list, sandbox list, create dialogs, resource inspection, and main-area Runtime Workspace.
4. Done: default BusyBox smoke template proving terminal-ready sandbox startup.
5. Done: terminal access plus logs/events in the web console runtime workspace.
6. Done: real cluster smoke verification for create, runtime access, exec, status mapping, and cleanup.
7. Done: preview port entry for declared TCP ports through the API server, including manual port add/remove from the Preview tab.
8. Done: PVC behavior is covered by runtime metadata and smoke verification.
9. Done: launch flow hides machine fields from normal users and relies on generated/defaulted slug, namespace, and ServiceAccount values.
10. Done: stop/start distinguishes pausing runtime compute from deleting the sandbox.
11. Done: template metadata for runtime type, use case, resource preset, and validation status now persists with the product record.
12. Done: first execution task API and Runtime Workspace Tasks tab for asynchronous sandbox commands with cancel.
13. Done: artifact reference registry and Runtime Workspace Artifacts tab.
14. Done: task watch/streaming plus running-sandbox workspace artifact content retrieval.
15. Done: runtime session records across API, terminal attachment, Web, CLI, SDK, and smoke tests.
16. Done: server-backed template validation runs with shared API, Web, CLI, SDK, and smoke coverage.
17. Done: policy and credential boundary starter as read-only API/CLI/SDK/Web summaries, with smoke coverage for the current runtime trust contract.
18. Done: artifact retention starter for small `workspace://` files, including retained bytes, sha256 metadata, API/CLI/SDK/Web support, and runtime smoke coverage.
19. Done: first enforceable project launch policy object across API, CLI, SDK, Web visibility, and smoke coverage.
20. Done: project credential-reference starter across API, CLI, SDK, Web visibility, and Boundary smoke coverage without secret-value storage or runtime mounting.
21. Done: artifact storage-provider starter for filesystem-backed retained bytes.
22. Done: lifecycle `ttlSeconds` enforcement for automatic sandbox cleanup.
23. Done: client-uploaded retained artifact content across API, CLI, SDK, docs, and smoke coverage.
24. Done: project deletion guard that preserves sandbox runtime cleanup state before project cascade deletion.
25. Done: API info/version/capability manifest across API, CLI, SDK, docs, and smoke coverage.
26. Done: runtime orphan audit for mbox-managed `agent-sandbox` resources.
27. Done: explicitly gated orphan cleanup for one currently reported runtime resource at a time.
28. Done: read-only project usage visibility starter for quota and operational-hardening groundwork.
29. Done: best-effort product audit-event starter for successful API mutations across API, CLI, SDK, Web inspector, docs, and smoke coverage.
30. Done: project quota policy starter for active sandbox count and retained artifact bytes across API, CLI, SDK, Web inspector, docs, and smoke coverage.
31. Done: OpenAPI contract starter across API, CLI, SDK, docs, and smoke coverage.
32. Done: Web launch preflight visibility for policy and active-sandbox quota blockers.
33. Done: best-effort `policy.denied` audit events for selected policy/quota denials.
34. Done: OpenAPI/SDK contract hardening for known audit actions and typed `policy.denied` metadata.
35. Done: project usage declared active/running sandbox request totals across API, SDK, Web inspector, docs, and tests.
36. Done: structured OpenAPI schema for project usage, sandbox request totals, and quantity counters.
37. Done: read-only runtime managed-resource inventory as the first live runtime visibility hardening slice.
38. Done: runtime managed-resource inventory summary with namespace/kind filtering and label-derived owner grouping, plus focused OpenAPI/SDK schema contract hardening for inventory and orphan-audit responses.
39. Done: S3-compatible retained artifact content backend starter for remote durability, keeping Postgres as metadata source of truth and preserving the existing artifact content API.
40. Done: request correlation starter for API responses, server logs, CLI/SDK clients, OpenAPI docs, and best-effort audit metadata.
41. Done: request-correlated audit feed filtering with `requestId` query support and JSONB expression indexes.
42. Done: action-specific audit metadata operation filtering with `operation` query support and JSONB expression indexes.
43. Done: audit feed RFC3339 `since` / `until` time-window filtering for bounded operator investigations.
44. Done: Web Runtime inventory route for read-only live runtime triage over the existing runtime auditor.
45. Done: Web project inspector filters for request-correlated and operation-scoped audit feed slices.
46. Done: read-only runtime inventory workload observation for Pod phase/readiness, restart count, resource requests/limits, and PVC state.
47. Done: read-only runtime inventory workload rollups for filtered Pod, readiness, request/limit, restart, and PVC capacity totals.
48. Done: policy-denial reason filtering for audit feeds across API, Postgres, CLI, SDK, Web, OpenAPI, and docs.
49. Done: project member role registry starter across API, Postgres, CLI, SDK, OpenAPI, Web project inspector visibility, docs, and smoke coverage, without route-level authorization enforcement yet.
50. Done: caller/auth handshake across API, CLI, SDK, OpenAPI, Web rail, docs, and smoke coverage, reporting anonymous, shared-token, or trusted-header mode plus RBAC trust/enforcement flags.
51. Done: project authorization preflight across API, CLI, SDK, OpenAPI, Web project inspector, docs, and smoke coverage, mapping `project.view`, `sandbox.launch`, `runtime.operate`, `artifact.write`, policy/member/credential management, and project management to required project roles and action-level enforcement state.
52. Done: trusted principal header provider starter across API config, `/v1/info`, caller handshake, authorization preflight, OpenAPI, SDK, Web rail/project inspector, docs, and smoke coverage; it is disabled by default and intended for trusted reverse-proxy/local-smoke identity.
53. Done: disabled-by-default `sandbox.launch` project RBAC enforcement starter across API config, caller/preflight/info surfaces, sandbox creation, template validation launches, OpenAPI, SDK, Web, docs, and tests; when enabled with trusted principal headers, sandbox launch requires an owner/operator project member match.
54. Done: disabled-by-default `artifact.write` project RBAC enforcement starter across API config, caller/preflight/info surfaces, artifact reference creation, workspace artifact capture, client artifact-content upload, OpenAPI, SDK, docs, and tests; when enabled with trusted principal headers, artifact writes require an owner/operator project member match.
55. Done: disabled-by-default `runtime.operate` project RBAC enforcement starter across API config, caller/preflight/info surfaces, runtime target/log/event/preview/terminal, runtime session create/end, execution task create/cancel/events, workspace artifact fallback reads, SDK request-option propagation, OpenAPI, docs, and tests; when enabled with trusted principal headers, live runtime operations require an owner/operator project member match.
56. Done: Web project inspector policy-denial/RBAC audit ergonomics highlight `policy.denied` events and expose typed denial metadata for operation, reason, authorization action, caller boundary, and resource hints without expanding the audit log or identity model.
57. Done: Web Runtime inventory per-project filter over existing `projectId` runtime-resource attribution, preserving filtered workload rollups as read-only operator triage rather than quota, billing, RBAC, or capacity semantics.
58. Done: disabled-by-default `policy.manage` project RBAC enforcement starter for project launch/quota policy update routes, with owner-only trusted-header authorization, `policy.denied` audit metadata, OpenAPI/SDK/Web/docs sync, and tests.
59. Done: Web project inspector grouped `policy.denied` quick navigation for recurring denial operation/reason families, using existing action/operation/reason filters and preserving audit metadata as operator debugging rather than trusted identity.
60. Done: SDK/OpenAPI published-route coverage guard requires ordinary OpenAPI operations to be represented in `SDK_ROUTE_CONTRACT`, while keeping terminal WebSocket and preview proxy pass-through routes as explicit non-helper exceptions.
61. Done: disabled-by-default `credential.manage` RBAC gate for project credential-reference create/delete routes, with owner-only trusted-header authorization, `policy.denied` metadata, OpenAPI/SDK/Web/docs sync, and tests.
62. Done: disabled-by-default `member.manage` RBAC gate for project member create/delete routes, with trusted-caller owner bootstrap on project creation, owner-only trusted-header authorization, `policy.denied` metadata, OpenAPI/SDK/docs sync, and tests.
63. Done: Runtime inventory `summary.byProject` rollup for deeper per-project runtime attribution, preserving the view as read-only label-derived operator triage rather than RBAC, quota, billing, metrics utilization, or capacity reservation.
64. Done: CLI `runtime resources --summary-table` for a human-readable runtime inventory summary over the existing filtered `summary` object, preserving JSON output for scripts and keeping runtime inventory read-only.
65. Done: SDK/OpenAPI `PolicyDeniedAuditMetadata` schema guard now covers typed policy/quota denial fields such as `policyKind`, `enforcement`, active-sandbox limits, and retained-artifact byte limits.
66. Done: SDK/OpenAPI project launch-policy and quota-policy schema guard covers response/upsert fields and required `projectId` on policy responses.
67. Done: SDK/OpenAPI project credential-reference schema guard covers reference-only credential fields and `SecretRef` without implying secret-value storage or runtime mounting.
68. Done: SDK/OpenAPI template-library schema guard covers template response/create/update/port fields and rejects immutable `projectId`/`slug` drift on update schemas.
69. Done: SDK/OpenAPI sandbox and preview schema guard covers sandbox response/create/update, runtime reference, declared sandbox ports, and preview port result fields, including immutable `projectId`/`templateId`/`slug` drift on sandbox update schemas.
70. Done: SDK/OpenAPI runtime session schema guard covers session response/create fields such as `type`, `status`, `startedAt`, client labels, runtime reference, and metadata.
71. Done: SDK/OpenAPI execution task schema guard covers task response/create/watch-event fields such as command, status, timeout, stdout/stderr, truncation marker, runtime reference, timing metadata, and event stream output fields.
72. Done: SDK/OpenAPI artifact schema guard covers artifact response/create and retained-content metadata fields such as artifact identity, task linkage, kind/name/URI, size/content metadata, retained-content hash/source/provider/key, and capture time.
73. Done: SDK/OpenAPI project schema guard covers project response/create/update fields such as name, slug, repository URL, default namespace/template, metadata, and timestamp fields while rejecting immutable `id`/`slug` drift on update schemas.
74. Done: Web project inspector member-management denial ergonomics show requested member principal type, principal, and role in `policy.denied` rows as operator debugging context, preserving audit metadata as non-identity evidence.
75. Done: Web project inspector member-management ergonomics can register and remove project member records for the starter RBAC model, refresh member/preflight/audit state after mutations, and keep at least one visible owner from being removed in the UI without adding login, invite, or broader route-level RBAC primitives.
76. Done: Web Runtime inventory attribution ergonomics render `summary.byProject` as clickable project distribution chips, preserving label-derived read-only runtime triage without turning the view into quota, billing, RBAC, metrics utilization, or capacity reservation.
77. Done: CLI Runtime inventory attribution ergonomics add `runtime resources --summary-table --resolve-project-names`, resolving known project IDs through `/v1/projects` only for human-readable output while preserving raw JSON and runtime-resource contracts for scripts.
78. Done: CLI audit ergonomics add `audit-events --policy-denied-summary` and `projects audit-events --policy-denied-summary`, grouping returned `policy.denied` rows by operation/reason over the existing read-only audit feed filters while preserving JSON output for scripts.
79. Done: CLI RBAC ergonomics add `projects authorization --summary`, rendering the existing project authorization preflight response as a compact human-readable decision view while preserving default JSON output and starter RBAC boundaries.
80. Done: CLI auth ergonomics add `auth caller --summary` and `caller --summary`, rendering the existing caller/auth boundary handshake as compact text while preserving JSON output and avoiding login/session semantics.
81. Done: SDK/OpenAPI schema alignment guard now checks selected auth and authorization enum values, including `CallerInfo` mode/principal-type values and project authorization action/evaluation values, so starter RBAC client contracts fail fast on enum drift.
82. Done: SDK/OpenAPI schema alignment guard now checks starter RBAC/policy/credential enum values, including project policy/quota `enforcement`, project member `principalType`/`role`, and project credential `type`, so published SDK union types fail fast on OpenAPI enum drift.
83. Done: SDK/OpenAPI schema alignment guard now checks runtime-facing enum values for template validation decisions, sandbox statuses, runtime session types/statuses, execution task statuses/events/streams, artifact kinds, and retained-content storage providers, preserving the SDK as a thin typed client over existing execution primitives.
84. Done: SDK/OpenAPI schema alignment guard now checks runtime orphan audit and gated cleanup enum/literal values, including orphan reasons, orphan sandbox statuses, cleanup request/result reasons, and the `delete-orphan-runtime-resource` confirmation string, without adding automatic cleanup or new runtime write paths.
85. Done: SDK/OpenAPI schema alignment guard now checks `RuntimeResourceOwner.kind` values (`sandbox` and `template`) so read-only runtime inventory attribution cannot silently drift away from the SDK owner model.
86. Done: CLI runtime orphan audit ergonomics add `runtime orphans --summary-table`, grouping returned orphan rows by reason and rendering resource/project/status/message/evidence columns over the existing read-only audit response while preserving default JSON output and explicit cleanup confirmation.
87. Done: CLI project usage ergonomics add `projects usage <project-id> --summary`, rendering sandbox/session/task/artifact/template/credential product-record counts plus declared active/running sandbox request totals over the existing read-only usage response while preserving default JSON output and avoiding live metrics or billing semantics.
88. Done: CLI project member ergonomics add `projects members <project-id> --summary`, rendering the starter RBAC role registry as role/principal-type counts and member rows over the existing read-only members response while preserving default JSON output and avoiding login/session/identity semantics.
89. Done: CLI project credential-reference ergonomics add `projects credentials <project-id> --summary`, rendering reference records as type/usage/Secret-reference counts and rows over the existing read-only credential response while preserving default JSON output and avoiding secret-value storage, exposure, or runtime mounting semantics.
90. Done: CLI project policy/quota ergonomics add `projects policy <project-id> --summary` and `projects quota-policy <project-id> --summary`, rendering existing launch-policy and quota-policy boundaries as compact text while preserving default JSON output and existing enforcement semantics.
91. Done: CLI boundary ergonomics add `templates boundary <template-id> --summary` and `sandboxes boundary <sandbox-id> --summary`, rendering namespace, runtime identity, policy, credential, projection, runtime-ref, and check status fields as compact text while preserving JSON output for scripts.
92. Done: SDK/OpenAPI boundary schema alignment structures `BoundarySummary`, `BoundaryCheck`, `BoundaryPort`, and `BoundaryCredentialRef` schemas and adds guard coverage for boundary kind, check status, policy enforcement, sandbox status, and credential type values.
93. Done: SDK/OpenAPI runtime log schema alignment guards `LogResult` required target/log fields, so SDK runtime log readers fail fast on published response drift without changing runtime behavior.
94. Done: SDK/OpenAPI API info schema alignment guards `RuntimeInfo`, `ArtifactInfo`, and `Compatibility` fields used by CLI/SDK capability and compatibility preflight.
95. Done: SDK/OpenAPI base response schema alignment guards `/healthz` status and shared `Error.error` fields used by SDK health and API error handling.
96. Done: SDK/OpenAPI list response alignment requires route-backed list helpers to publish an `items` array, matching the SDK `ListResponse<T>` type for project, audit, template, sandbox, runtime event, session, task, and artifact lists.
97. Done: CLI audit ergonomics add `audit-events --summary` and `projects audit-events --summary`, grouping returned rows by action with resource-type, actor, source, count, and latest-time columns over the existing read-only audit feed.
98. Done: SDK/OpenAPI array item-ref alignment guards selected runtime nested arrays, including runtime target storage, runtime inventory workload storage/issues, and preview port result items, so SDK runtime clients fail fast on published nested-array schema drift.
99. Done: CLI audit help/usage alignment advertises `--summary` alongside `--policy-denied-summary` for both global and project-scoped audit-event commands, so the new read-only summary mode is discoverable from help and early usage errors.
100. Done: SDK/OpenAPI property-ref alignment guards selected runtime inventory, log/task event, and runtime orphan/cleanup nested schema references, including `$ref`-backed orphan reason enums, so generated-client preparation catches object-ref drift without changing runtime audit or cleanup behavior.
101. Done: SDK/OpenAPI project usage schema-ref alignment guards `ProjectUsage` usage buckets, sandbox active/running request totals, and CPU/memory/storage quantity references, keeping per-project product-record usage contracts generated-client ready without adding live metrics, billing, quota reservation, or capacity semantics.
102. Done: CLI tasks usage alignment now lists the implemented `tasks run` subcommand in task-group usage errors, keeping one-shot execution discoverable outside the top-level help without changing task execution behavior.
103. Done: CLI `tasks create` usage alignment now advertises repeated `--arg`, comma-split `--command`, and JSON-array `--command-json` input modes, keeping existing task creation ergonomics discoverable without changing task execution semantics.
104. Done: CLI `templates validate-run` usage alignment now advertises repeated `--arg`, comma-split `--command`, JSON-array `--command-json`, and positional `-- COMMAND...` input modes, keeping template validation command entrypoints discoverable without changing task execution or validation decision semantics.
105. Done: CLI `tasks run` usage alignment now advertises repeated `--arg`, comma-split `--command`, JSON-array `--command-json`, and positional `-- COMMAND...` input modes, keeping one-shot execution discoverable without changing task creation or wait semantics.
106. Done: SDK/OpenAPI project template-usage array item-ref alignment guards `ProjectTemplateUsage` CPU, memory, and storage request distribution arrays as `ResourceUsageValue[]`, keeping generated-client project usage schemas precise without adding live metrics, billing, quota reservation, or capacity semantics.
107. Done: SDK/OpenAPI API info property-ref alignment guards `APIInfo` runtime controller/access, artifact content, trusted principal header, project RBAC, and compatibility sub-schemas, keeping capability handshakes generated-client ready without changing authentication, RBAC, or runtime behavior.
108. Done: SDK/OpenAPI boundary summary ref alignment guards `BoundarySummary` runtime reference plus preview port, secret reference, credential reference, and check arrays, keeping boundary clients generated-client ready without adding secret-value access, broader RBAC, custom network projection, or live utilization semantics.
109. Done: SDK/OpenAPI project authorization decision ref alignment guards `ProjectAuthorizationDecision` caller and matched-member schema refs, keeping starter RBAC preflight generated-client ready without adding login, invite, trusted identity, or broader route-level RBAC behavior.
110. Done: SDK/OpenAPI project credential-reference ref alignment guards `ProjectCredential` and `ProjectCredentialCreate` `secretRef` schema refs, keeping credential reference clients generated-client ready without adding secret-value storage, credential issuance, runtime mounting, or broader RBAC behavior.
111. Done: SDK/OpenAPI template validation-run ref alignment guards `TemplateValidationRun` template and sandbox schema refs, keeping validation clients generated-client ready without changing validation execution, task execution, sandbox launch, or decision semantics.
112. Done: SDK/OpenAPI template library array item-ref alignment guards `EnvironmentTemplate`, `TemplateCreate`, and `TemplateUpdate` `exposedPorts[]` and `secretRefs[]` schema refs, keeping template clients generated-client ready without changing runtime projection, secret mounting, or policy enforcement behavior.
113. Done: SDK/OpenAPI sandbox ref alignment guards `Sandbox` and `SandboxUpdate` `runtimeRef` plus `ports[]` schema refs, keeping sandbox clients generated-client ready without changing lifecycle, runtime reconciliation, or preview-port behavior.
114. Next: deeper generated-client/schema alignment, broader CLI ergonomics, or the next narrow RBAC/audit ergonomics slice that does not add login, invite flows, an agent brain, CI/CD platform, or deployment release manager primitives.

This slice proves the core runtime loop before upper-layer CI or deployment integrations expand the surface area.

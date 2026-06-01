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

- Started: TypeScript SDK in `sdk/typescript` with typed wrappers for API info/version/capability handshake, runtime orphan audit/cleanup, health, projects, project policy, project quota policy, project credential references, templates, template validation, template/sandbox boundary summaries, sandboxes, runtime target/log/event/port reads, runtime sessions, execution tasks, task polling/watch/cancel, artifact references, retained artifact upload/capture/content, and workspace artifact content fallback.
- Started: Go CLI in `cmd/mbox` as a thin HTTP client for API info/version/capability handshake, runtime orphan audit/cleanup, health, projects, project policy, project quota policy, project credential references, templates, template validation, template/sandbox boundary summaries, sandboxes, sessions, tasks, artifacts, retained artifact upload/capture/content, logs, ports, and terminal access.
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
- Done: audit-event inclusive `since` / `until` time-window filtering across API, CLI, SDK, OpenAPI, docs, and tests for bounded operator investigation windows.
- Done: audit-event action query indexes for global and project-scoped feeds ordered by recency.
- Done: SDK/OpenAPI alignment guard now checks SDK-used query parameters in addition to route paths and methods.
- Done: SDK/OpenAPI alignment guard now checks focused SDK-consumed schema required fields and properties for project usage and audit contracts.
- Done: SDK/OpenAPI alignment guard now checks focused SDK helper response schemas, list item refs, NDJSON task events, binary responses, and no-content delete routes.
- Done: SDK/OpenAPI alignment guard now checks focused SDK helper request body schema refs and binary upload media types.
- Done: explicit SDK and CLI compatibility preflight helpers compare client API labels with the server `/v1/info` minimum CLI/SDK API versions before longer automation runs.
- Done: SDK and CLI compatibility preflight can require server capabilities such as `execution-tasks`, `task-events`, and `artifact-client-upload` before clients start a longer run.
- Done: SDK local smoke gate now builds the package and exercises compatibility helpers plus OpenAPI alignment success/failure paths without requiring a live API server.
- Done: SDK package dry-run gate now verifies `npm pack` would include the README, package manifest, compiled JavaScript, and TypeScript declarations while excluding source-only files.
- Done: SDK package consumer smoke now installs a real local `npm pack` tarball into a minimal ESM consumer and verifies exported helpers load from the installed package.
- Done: SDK publish gate now runs typecheck, local smoke, package dry-run, and package consumer smoke via `npm run verify`, with `prepublishOnly` wired to the same gate.
- Done: starter API compatibility policy documented and regression-tested for same-family `vNalphaM`, `vNbetaM`, and stable `vN` ordering, with capabilities kept as separate feature gates.
- Done: starter shared API token model for automation clients: optional `MBOX_API_TOKEN` server gate, public health/info discovery, CLI `MBOX_TOKEN`/`--token`, SDK `token`, and smoke/test coverage.
- Done: OpenAPI auth contract hardening for the starter token model: `bearerAuth` security scheme, explicit public health/info operations, private bearer operations, `401` error responses, and SDK alignment checks for route auth metadata.
- Done: starter CLI context selection through `--context`, `MBOX_CONTEXT`, `--config`, `MBOX_CONFIG`, and `~/.mbox/config.json`, with API URL/token/audit-label loading, explicit flag overrides, and local `context current/list` inspection that redacts token values.
- Done: CLI local context management with `context set`, `context use`, and `context remove`, including parent-directory creation, token-env support for reusable configs, redacted JSON output, and CLI smoke coverage against an authenticated API.
- Done: TypeScript SDK environment factory `createMboxClientFromEnv()` for automation scripts that share the CLI `MBOX_API_URL`, `MBOX_TOKEN`/`MBOX_API_TOKEN`, and audit-label conventions without reading local context files.
- Done: CLI `tasks wait` polling helper for automation scripts that need SDK-style task terminal-state waiting without parsing the NDJSON watch stream.
- Done: read-only runtime managed-resource inventory, namespace/kind filtering, live kind/namespace/owner summary, and structured OpenAPI/SDK orphan-audit contracts across API, CLI, SDK, OpenAPI, docs, and smoke coverage, reusing the runtime auditor without adding automatic cleanup or new write paths.
- Done: Web Runtime inventory view at `#runtime` for read-only operator triage over `/v1/runtime/resources`, including summary, owner, label, and disabled-auditor handling without adding runtime write actions.
- Done: Web project inspector audit-feed ergonomics for request ID, operation, and RFC3339 time-window filters, with trace metadata display when present.
- Done: Runtime inventory workload observation for resolved SandboxClaim Pod phase, readiness, restart count, summed requests/limits, and PVC state across API, OpenAPI, SDK, Web, and runtime smoke coverage without adding metrics-server utilization or quota semantics.
- Remaining: user/project RBAC beyond the shared automation token, real package publication/release workflow, generated client and full schema alignment, broader CLI ergonomics, and future versioning decisions beyond the current starter policy.

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
47. Next: operational hardening around remaining live runtime usage rollups, audit-feed ergonomics, or user/project RBAC starter.

This slice proves the core runtime loop before upper-layer CI or deployment integrations expand the surface area.

# ARCHITECTURE

## Architecture Goal

mbox should provide a product-level Kubernetes execution platform for programmable sandboxes, runtime sessions, controlled tasks, previews, artifacts, and policy boundaries while keeping the product contract decoupled from runtime CRDs.

The selected interactive sandbox runtime is `kubernetes-sigs/agent-sandbox`. mbox still owns the product API, permissions, UI, runtime lifecycle, and execution records. It should not become an agent platform or a CI/CD platform; agents, CI systems, IDEs, and release tools are clients of the execution platform.

Current implementation status:

- Go server entrypoint: `cmd/mbox-server`.
- Web console: separate Vite React app under `web/`.
- Product state: Postgres through `pgx`.
- Implemented product resources: projects, environment templates, sandboxes, and sandbox-backed execution tasks.
- Implemented API surface: `GET /healthz`, CRUD routes under `/v1/projects`, `/v1/templates`, and `/v1/sandboxes`, plus runtime target with storage metadata, logs, events, preview ports, preview proxy, terminal routes, and asynchronous task execution for sandboxes.
- Runtime projection: opt-in `agent-sandbox` adapter and sandbox reconciler.
- Runtime access: separately opt-in browser terminal, execution tasks, logs, events, preview proxy, and runtime target resolution through the API server's Kubernetes client.

See `docs/server-api.md` for the current concrete API and configuration contract, and `docs/web-console.md` for the current frontend structure.

## High-level Layers

```text
Client Surfaces
  Web console, CLI, API docs, SDK packages, external agents, IDEs, CI systems

Control Plane
  Go API server, Auth, RBAC, state, scheduling decisions, audit, lifecycle management

Execution Control
  Runtime sessions, command/task records, logs, cancellation, previews, artifacts

Runtime Adapter
  agent-sandbox for interactive sandboxes, Kubernetes Job or future adapters for isolated batch tasks

Kubernetes
  agent-sandbox CRDs, Namespaces, Pods, PVCs, Services, Ingress/Gateway, NetworkPolicy, RBAC
```

## Long-term Technical Surfaces

mbox should grow into several coordinated technical surfaces rather than a single web application.

### Server

The server side includes:

- Go API server for product APIs.
- Controllers/reconcilers for product records and Kubernetes runtime resources.
- `agent-sandbox` integration for interactive sandbox runtime.
- Kubernetes resources, RBAC, NetworkPolicy, PVCs, Services, Gateway or Ingress, logs, and events.

The server is the source of truth for projects, templates, sandboxes, runtime sessions, execution tasks, previews, artifacts, policies, credentials, audit, and runtime state mapping.

### Web App

The web app is the human-facing console for daily operation:

- project, sandbox, template, runtime session, task, preview, artifact, policy, and credential management
- terminal, IDE, notebook, browser, command, preview port, logs, events, and status views
- dense operational workflows for repeated use

The web app should consume the same product APIs as the CLI and SDK. It should not depend on private controller behavior or raw Kubernetes objects as its main contract.

The current web app is a Vite React app in `web/`. In local development it runs separately from the Go API server and proxies `/healthz` plus `/v1/*` to the configured API target. The default local split is API on `127.0.0.1:18080` and Vite on `127.0.0.1:5174`.

### CLI

The CLI should provide scriptable access to the same core workflows:

- project and template inspection
- sandbox create, enter, list, stop, delete
- session, port, log, event, and artifact access
- command/task run, watch, cancel, and inspect
- upper-layer workflow integration points without owning CI or deployment logic

The CLI should be suitable for local developer use, agent/tool integration, CI scripts, and operational debugging. It should be a first-class API client, not a separate control path.

### API Docs

The API documentation surface should publish the product API contract for humans and automation clients:

- authentication model
- resource schemas
- request and response examples
- error codes and policy denial reasons
- streaming/log/watch semantics
- versioning and compatibility rules

The API docs should track the server API version and SDK generation boundary.

The current implementation publishes a starter OpenAPI 3.1 document at `GET /v1/openapi.json`. It is the machine-readable contract for implemented public routes and schemas, and is meant to anchor future generated docs or client generation without changing the product model.

### SDK Package

mbox should provide at least one official SDK/package for automation clients. The first package can be Node.js or Go, depending on the first integration audience.

The SDK should wrap the public product API for common runtime workflows while keeping raw API access possible for advanced clients. It should share API schemas with the server and API docs where practical.

The current repository includes a first TypeScript SDK package in `sdk/typescript`. It is deliberately a thin HTTP client over product primitives: projects, project launch policies, project quota policies, project credential references, templates, template validation, boundary summaries, sandboxes, runtime access, runtime sessions, execution tasks, preview ports, and artifacts. Convenience methods such as `waitForTask` and `watchExecutionTask` belong in the client because they help external agents and scripts compose the primitive API without turning the mbox server into an agent planner or CI workflow engine.

## Runtime Boundary

mbox uses `agent-sandbox` as the interactive sandbox runtime. It should not treat `agent-sandbox` as the whole product.

The product should depend on a stable internal runtime contract:

```text
CreateRuntime
StartRuntime
StopRuntime
PauseRuntime
ResumeRuntime
RunCommand
StreamLogs
ExposePort
AttachVolume
DeleteRuntime
GetRuntimeStatus
```

For interactive sandboxes, the adapter creates `SandboxClaim` resources and maps them to mbox `Sandbox` records.

For isolated batch tasks, mbox can use sandbox-backed execution when the run needs an interactive or stateful workspace. Short, isolated, repeatable commands can use ordinary Kubernetes Jobs or a future batch adapter.

## Core Components

### API Server

Owns product APIs:

- projects
- templates
- sandboxes
- runtime sessions
- execution tasks
- previews
- artifacts
- policies
- credentials and secret references
- audit records

The API server should persist desired state and user intent. It should not rely on the runtime Pod as the only source of truth.

### Web Console

Human-facing console for:

- creating and entering sandboxes
- editing environment templates
- inspecting runtime sessions, tasks, previews, artifacts, logs, and events
- managing resource and security policies

The UI should be operational and dense enough for repeated use. Avoid landing-page style composition in the app surface.

Current implemented console scope is intentionally narrower than the long-term product: list and create projects, templates, and sandboxes; inspect selected resource IDs and runtime state; show project launch policy, quota policy, and credential-reference visibility in the project inspector; launch and decide template validation sandboxes; stop, start, and delete sandboxes from compact row actions; open a main-workspace browser terminal for ready sandboxes; show resolved workspace PVC storage metadata; show the read-only Boundary tab for namespace, ServiceAccount, secret, credential-reference, network, lifecycle, runtime access, and cleanup visibility; show declared preview ports through the API server's Pod proxy path; run, poll, cancel, and inspect asynchronous command tasks inside a ready sandbox; register and list artifact references for sandbox and task outputs; show runtime session history; show lightweight runtime logs and Kubernetes events in runtime tabs; show API health and request errors. Full credential injection and full policy management screens are still roadmap work. Pipeline and deployment products may be built later as clients of these primitives rather than as the base architecture.

### Controller / Reconciler

Reconciles mbox product records to Kubernetes resources:

- project namespace
- service account and RBAC
- runtime resources
- PVCs
- Services
- Gateway or Ingress routes
- NetworkPolicy
- cleanup jobs

The reconciler must be idempotent and safe under retries.

### Execution Controller

Owns runtime work state:

- queued
- running
- succeeded
- failed
- canceled
- timed out
- cleanup pending

Each execution task should have explicit status, timing, command or workload metadata, logs, retry or restart count when applicable, runtime reference, artifacts, cancellation state, and failure reason.

The execution controller is deliberately lower-level than CI. A CI pipeline, coding agent, IDE action, notebook runner, or release workflow can create tasks through the public API, but mbox should not need to understand their business-level plan to provide useful execution records.

### Runtime Adapter

The runtime adapter translates product intent into concrete execution resources.

Selected runtime adapters:

- `agent-sandbox` for interactive, stateful, singleton sandbox environments.
- Kubernetes Job for isolated, repeatable batch tasks when a full sandbox is unnecessary.
- Future custom runner for specialized build or deployment execution if required.

## Kubernetes Model

### Namespace Strategy

Default to namespace-scoped isolation.

Recommended starting model:

- One project namespace for long-lived project resources.
- Optional per-run or per-sandbox namespace when stronger isolation is needed.
- Separate system namespace for mbox controllers and API services.

### RBAC Strategy

Use scoped service accounts:

- user sandbox service account
- execution task service account
- controller service account

Do not mount broad kubeconfigs into user sandboxes or task runtimes. Upper-layer deployment tools must use explicit, narrow credential references if they call mbox for deployment-related execution.

### Storage Strategy

Support both:

- persistent workspace volume for interactive sandboxes
- ephemeral volumes for isolated batch tasks

Long-lived volumes need cleanup rules, quota visibility, and ownership metadata.

### Network Strategy

NetworkPolicy should default to restricted ingress and controlled egress.

Common policies:

- allow web console or gateway to reach sandbox exposed ports
- allow package registry and Git endpoints
- optionally block cluster-internal network access
- optionally allow project namespace services

### Credential Strategy

Credentials should be injected narrowly:

- Git credentials are repo-scoped.
- Registry credentials are project- or task-scoped.
- Kubernetes credentials are explicit references for specific upper-layer workflows.
- Production credentials require explicit permission and audit.

Prefer short-lived tokens and secret references over copying long-lived credentials into runtime filesystems.

## Data Model Draft

The implemented data model currently covers the sandbox control-plane slice plus runtime session records, sandbox-backed execution task records, artifact reference records, retained content for small captured workspace-file artifacts, a first project-scoped launch policy, a project quota policy, project credential-reference records, and best-effort product audit events for successful API mutations. Credential value storage/injection, strong transactional audit, auth identity attribution, full RBAC, and deeper observability records below are still roadmap items.

### Project

- id
- name
- slug
- repository URL
- default namespace
- default template id
- metadata
- created at
- updated at

### EnvironmentTemplate

- id
- project id, optional for global templates
- name
- slug
- image
- startup command
- working directory
- CPU, memory, and storage requests
- exposed ports
- env
- secret references
- network policy
- lifecycle policy
- metadata
- created at
- updated at

Template metadata is product-facing library metadata, not runtime configuration. The current web console stores `runtimeType`, `useCase`, `resourcePreset`, and `validationStatus` there so users can choose templates by environment intent. The runtime adapter must continue to project the concrete template fields into Kubernetes resources.

### Sandbox

- id
- project id
- template id
- name
- slug
- status: `pending`, `running`, `stopped`, `failed`, or `deleted`
- namespace
- service account name
- runtime reference
- ports
- metadata
- created at
- updated at
- deleted at

Implemented Postgres constraints:

- UUID primary keys are generated by Postgres.
- Active sandbox slugs are unique per project while deleted sandbox slugs can be reused.
- Global template slugs are unique globally.
- Project template slugs are unique per project.
- Sandbox deletion is soft deletion through `deleted_at`.
- Execution tasks currently belong to one project and one sandbox.
- `updated_at` is maintained by database triggers.

### RuntimeSession

- id
- sandbox id
- project id
- kind: terminal, IDE, notebook, browser, command, or custom
- status: active, ended, or failed
- client label and user agent
- runtime reference
- started at
- ended at
- metadata

Runtime sessions are attachment/audit records for clients entering an existing sandbox. They intentionally do not model an internal agent brain or own upper-layer workflow semantics.

### ExecutionTask

- id
- project id
- sandbox id
- status: `queued`, `running`, `succeeded`, `failed`, `canceled`, or `timed_out`
- command
- timeout seconds
- runtime reference
- exit code
- stdout
- stderr
- output truncated flag
- failure reason or error
- started at
- finished at
- metadata
- created at
- updated at

The current implementation is asynchronous and sandbox-backed. `POST /v1/sandboxes/{sandboxID}/tasks` creates a queued task and returns immediately, the API server runs the command through runtime access in the background, clients can poll task records or stream `GET /v1/tasks/{taskID}/events`, and `POST /v1/tasks/{taskID}/cancel` cancels tasks currently running on this API server. Batch-only runtimes, log references, durable cleanup state, and restart-safe task control remain future work.

### Artifact

- id
- project id
- sandbox id
- task id, optional
- kind: `file`, `directory`, `log`, `report`, `screenshot`, `image`, `link`, or `other`
- name
- URI or storage reference
- content type
- size bytes
- retained content metadata: content type, size, sha256, source URI, and captured time
- metadata
- created at
- updated at

The current artifact implementation is a metadata registry with a narrow workspace-file retention path. Clients can register references such as `workspace:///workspace/reports/test.json`, an HTTPS URL, an object-store URI, a log reference, or a generated screenshot path. `POST /v1/artifacts/{artifactID}/capture` reads a supported `workspace://` file from a running sandbox workspace, stores a small retained byte copy server-side, and records sha256 plus storage-provider metadata. The default provider stores bytes in Postgres for compatibility; the filesystem provider writes bytes under a configured local artifact directory and keeps Postgres as the metadata index. `GET /v1/artifacts/{artifactID}/content` returns retained bytes first, then falls back to reading the running workspace when no retained copy exists. Later storage integration can add client uploads, object-store download, retention policies, and access control without changing the fact that artifacts belong to sandboxes and can optionally point at the task that produced them.

### ProjectPolicy

- project id
- enforcement: `disabled` or `enforced`
- allowed image prefixes
- allowed service account names
- allowed secret reference names
- created at
- updated at

The first policy object is a project-scoped sandbox launch gate. When absent or disabled, sandbox launch behavior stays unchanged except for the base cross-project template check. When enforced, `POST /v1/sandboxes` and template validation launches check the selected template image prefix, requested sandbox ServiceAccount, and declared template `secretRefs` by name before creating the sandbox record. This is not full RBAC, credential mounting, custom network projection, or lifecycle policy management; it is the first enforceable product boundary that prevents out-of-scope runtime shapes from launching.

### ProjectQuotaPolicy

- project id
- enforcement: `disabled` or `enforced`
- max active sandboxes
- max retained artifact bytes
- created at
- updated at

The first quota policy object is a project-record guard, not billing, capacity reservation, live Kubernetes metrics, or a scheduler contract. When absent or disabled, sandbox creation and artifact retention behave normally. When enforced, sandbox creation is denied once the product-record active sandbox count is at or above `maxActiveSandboxes`, and artifact capture/upload is denied when current retained artifact bytes plus incoming bytes would exceed `maxRetainedArtifactBytes`.

### ProjectCredential

- id
- project id
- name and slug
- type: `git`, `registry`, `kubernetes`, `ssh`, or `generic`
- target, such as repository URL, registry host, cluster name, or service endpoint
- Kubernetes Secret reference by name/key
- usage labels
- metadata
- created at
- updated at

Project credentials are reference records, not secret-value records. They catalog which project-scoped Secret may be used for which target and purpose so future task/session/upper-layer integrations can request narrow credentials explicitly. The current runtime adapter does not mount these credentials into sandbox Pods and mbox does not copy or store secret values.

### UpperLayerWorkflow

This is intentionally not a required base table. CI pipelines, deployment flows, and agent task plans may be represented by future integrations once the lower-level runtime primitives are stable. If mbox stores them later, they should reference sandboxes, sessions, tasks, previews, and artifacts rather than replacing those primitives.

Potential future workflow records:

- pipeline definition
- pipeline run
- deployment target
- deployment run
- approval gate

### Policy

- id
- scope
- resource limits
- allowed images
- allowed registries
- allowed network destinations
- lifecycle rules
- credential access rules

## Security Requirements

- Default namespace isolation.
- No cluster-admin credentials in ordinary runtime environments.
- Separate human, external client, execution task, and controller permissions.
- Explicit audit for secret use, runtime access, task execution, and cleanup actions.
- NetworkPolicy enabled for sandbox namespaces.
- RuntimeClass support for stronger isolation when available.
- Artifact provenance for generated outputs.
- Cleanup controller for stale sandboxes and volumes.

## Observability Requirements

The product should expose:

- sandbox phase and pod status
- container logs
- runtime session status
- execution task logs
- Kubernetes events
- resource usage
- preview status
- artifact metadata
- failed scheduling reasons
- image pull errors
- quota and policy denial reasons

## Runtime Implementation Notes

`agent-sandbox` is the selected interactive sandbox runtime because it provides Kubernetes-native CRDs for stateful singleton workloads and claims/templates/warm pools.

Do not couple product APIs to `SandboxClaim` directly. Store mbox product records and map them to runtime resources through an adapter. This keeps the mbox product model stable and prevents the UI/API from becoming a thin wrapper around runtime CRDs.

The implemented runtime controller is intentionally opt-in through `MBOX_RUNTIME_CONTROLLER_ENABLED=true`. With the controller disabled, the server only writes mbox product records in Postgres. With it enabled, the reconciler projects eligible sandbox records into Kubernetes by creating or updating a namespace, scoped ServiceAccount, `SandboxTemplate`, and `SandboxClaim`.

Stop/start is represented as product lifecycle state on the mbox sandbox record rather than direct UI access to runtime CRDs. `POST /v1/sandboxes/{id}/stop` marks the sandbox `stopped`; the reconciler keeps the `runtimeRef`, skips runtime status polling, resolves `SandboxClaim.status.sandbox.name`, and scales the resolved `agent-sandbox` `Sandbox.spec.replicas` to `0`. `POST /v1/sandboxes/{id}/start` marks the sandbox `pending`; when the sandbox already has a `runtimeRef`, the reconciler scales the same runtime `Sandbox` back to `1` replica before status mapping. Delete remains a separate soft-delete path that removes the `SandboxClaim` and clears `runtimeRef`.

The generated sandbox ServiceAccount and pod template both set token automount to false. Runtime credentials should be introduced later as narrow, explicit capabilities rather than inherited cluster access.

The boundary summary routes are read-only views of this runtime contract. `GET /v1/templates/{templateID}/boundary` can resolve a global template against a `projectId` query to show the namespace, default runtime identity, launch policy state, and project credential references that would apply. `GET /v1/sandboxes/{sandboxID}/boundary` shows the sandbox's resolved namespace, ServiceAccount, runtime reference, secret reference projection, credential-reference projection, network policy projection, lifecycle policy projection, project launch policy state, runtime access paths, and cleanup behavior. These routes do not mount secrets, mount project credentials, or project custom network policy yet; they make the current safety contract explicit so later policy and credential work has a stable baseline. Template `lifecyclePolicy.ttlSeconds` is enforced by the reconciler as automatic sandbox cleanup; idle timeout and richer cleanup policies remain future work.

When an environment template includes `storageRequest`, the adapter projects a `workspace` `volumeClaimTemplates` entry into the generated `SandboxTemplate` and mounts it at the template `workingDir`, defaulting to `/workspace`. This is the current persistence contract for interactive sandboxes: workspace data should survive runtime Pod replacement and stop/start while the sandbox exists. Files written outside the persistent workspace are container-local and can be lost when stop/start removes and recreates the Pod.

Runtime access is intentionally gated separately from reconciliation through `MBOX_RUNTIME_ACCESS_ENABLED=true`. Enabling reconciliation alone may create or delete Kubernetes runtime resources, but it does not expose terminal, execution task, logs, events, or runtime target APIs. When runtime access is enabled, the server resolves a running mbox sandbox through `Sandbox.runtimeRef`, `SandboxClaim.status.sandbox.name`, `Sandbox.status.selector`, and the matching Pod, preferring the `workspace` container when present.

Runtime target responses include PVC-backed storage metadata resolved from the selected container's volume mounts and PersistentVolumeClaims. This keeps the web console and automation clients on the mbox API surface while still exposing the workspace mount path, claim name, bound phase, capacity, and storage class when Kubernetes reports them.

The browser terminal is a WebSocket proxy to Kubernetes `pods/exec`. The HTTP layer rejects non-running sandboxes before upgrading the connection and only permits `sh` or `bash` as shell selectors. Local Vite development depends on the `/v1/*` proxy forwarding WebSocket upgrades.

Sandbox execution tasks use the same runtime target resolution and Kubernetes `pods/exec` boundary without TTY. `POST /v1/sandboxes/{sandboxID}/tasks` requires runtime access, a running sandbox, and a ready runtime reference. It records command metadata, timing, stdout, stderr, exit code when Kubernetes reports one, timeout/cancellation status, output truncation, and the runtime reference used for execution. `GET /v1/tasks/{taskID}/events` streams process-local newline-delimited JSON events for snapshot, status, stdout/stderr chunks, and done; finished tasks still return persisted snapshot/done events. Task cancellation is process-local in this slice because the cancel function lives in the API server currently running the exec; persisted task status remains in Postgres. Shell behavior is explicit: API clients should pass `["sh", "-lc", "..."]` when they need shell parsing.

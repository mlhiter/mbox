# Architecture Guide

This guide is the short external handoff for the implemented mbox slice. The fuller product architecture remains in `ARCHITECTURE.md`.

mbox is a Kubernetes execution platform, not an agent product and not primarily a CI/CD product. External agents, IDEs, CI systems, release tools, and humans call mbox to create sandboxes, connect runtime sessions, run controlled work, inspect previews/artifacts, and clean up resources.

## Current Shape

mbox is split into three active layers:

- Go API server in `cmd/mbox-server`
- Postgres product store through `internal/postgres`
- Vite React console in `web/`

The API server owns product records. Kubernetes resources are runtime projections, not the source of truth.

## Implemented Product Records

- `Project`: codebase or product scope with a default namespace and optional default template.
- `EnvironmentTemplate`: reusable sandbox launch shape with image, command, working directory, resource requests, storage request, exposed ports, env, secret refs, network/lifecycle policy, and product-facing metadata.
- `Sandbox`: product-level runtime request with namespace, service account, status, runtime reference, and declared preview ports.
- `ExecutionTask`: sandbox-backed command run with command metadata, status, timing, stdout, stderr, exit result, timeout/cancellation state, and runtime reference.

Postgres migrations live in `internal/postgres/migrations`. Startup requires `DATABASE_URL` and runs embedded migrations before serving HTTP.

Create routes can derive slugs from names. The normal sandbox launch path is product-oriented: users provide project, template, and name; mbox fills namespace from the project and uses the default sandbox ServiceAccount unless an API client deliberately overrides those fields.

Template metadata exists to support the environment-library product surface. The current metadata keys are `runtimeType`, `useCase`, `resourcePreset`, and `validationStatus`. They describe how users choose and trust a template; they do not replace the concrete runtime fields that the adapter projects into `agent-sandbox`.

## Runtime Projection

Runtime reconciliation is disabled by default. When `MBOX_RUNTIME_CONTROLLER_ENABLED=true`, the sandbox reconciler:

1. Lists sandboxes that need runtime reconciliation.
2. Creates or updates the target namespace.
3. Creates or updates the sandbox ServiceAccount with token automount disabled.
4. Creates or updates an `agent-sandbox` `SandboxTemplate`.
5. Creates a `SandboxClaim`.
6. Stores a `runtimeRef` on the mbox sandbox record.
7. Maps runtime status and runtime-reported ports back into Postgres.
8. Scales the resolved runtime `Sandbox` to zero replicas when the mbox sandbox is `stopped`.
9. Scales the existing runtime `Sandbox` back to one replica when a stopped sandbox is started and returns to `pending`.
10. Deletes the runtime claim when the sandbox is soft-deleted.

The generated pod template also disables service account token automount. When a template has `storageRequest`, the generated `SandboxTemplate` includes a `workspace` PVC template mounted at the template `workingDir`, defaulting to `/workspace`. Stop/start preserves the mbox record and runtime reference, but only PVC-backed workspace data is expected to survive the Pod removal caused by scaling to zero.

A `pending` mbox sandbox can exist before a `runtimeRef` is written and before the Pod is ready. Frontend runtime surfaces should treat that as a starting state, not a failed terminal/runtime request.

A `pending` mbox sandbox can also mean a previously stopped sandbox is starting again. In that case the reconciler does not create a second runtime; it uses the stored `runtimeRef`, scales the existing runtime `Sandbox` back to one replica, and then resumes status mapping.

## Runtime Access

Runtime access is separately gated by `MBOX_RUNTIME_ACCESS_ENABLED=true`. This enables:

- runtime target resolution
- resolved workspace PVC storage metadata
- logs
- Kubernetes events
- declared preview port listing
- API-proxied preview port access
- browser terminal
- asynchronous sandbox command tasks

The runtime target resolution path is:

1. mbox `Sandbox.runtimeRef`
2. `SandboxClaim.status.sandbox.name`
3. runtime `Sandbox.status.selector`
4. matching Pod, preferring the `workspace` container

The terminal is a WebSocket bridge to Kubernetes `pods/exec`. It only opens for mbox sandboxes with `running` status and only accepts `sh` or `bash`.

Preview ports are product declarations stored on the mbox sandbox record. Template `exposedPorts` seed the list at creation time, and the Preview tab can patch the sandbox list when a user starts an additional service in the terminal. Preview links are allowed only when the port is declared, the sandbox is running, and the protocol is TCP. The first implementation proxies through the mbox API server to the resolved Kubernetes Pod proxy path.

Runtime target responses include PVC-backed storage metadata by inspecting the resolved Pod's `workspace` container volume mounts and matching PersistentVolumeClaims. The Storage tab uses this to show mount path, claim name, bound phase, capacity, and storage class without exposing raw Kubernetes access to the browser.

Execution tasks reuse the same runtime access boundary as terminal, but they run non-interactive `pods/exec` commands and persist the result in Postgres. The current task MVP is asynchronous and sandbox-backed: it requires a running sandbox with a runtime reference, creates a queued task record, runs the command in the API server background, captures stdout and stderr with output caps, records exit code when available, and marks timeout or explicit cancellation. It is deliberately lower-level than CI or agent planning.

Artifacts are product metadata records for outputs produced by sandboxes or tasks. They store kind, name, URI, optional task linkage, content type, size, and metadata. The first implementation does not move bytes; it gives external agents, SDK clients, CI systems, and humans a stable place to register output references such as workspace paths, HTTP URLs, reports, screenshots, images, and logs.

## Platform Primitives

The implemented slice covers projects, templates, sandboxes, sandbox-backed execution tasks, and artifact references. The next product layer should stay below agent and CI semantics:

- SDK: typed external-client access to sandbox, task, preview, and artifact primitives without making mbox itself an agent runtime.
- Runtime sessions: terminal, IDE, notebook, browser, command, and custom client attachments to a sandbox.
- Execution tasks: extend the current sandbox command MVP toward watch/streaming semantics, cleanup handling, artifact automation, and optional batch runtimes.
- Previews: inspectable runtime endpoints or rendered outputs.
- Artifacts: extend the current reference registry with retrieval, retention, integrity, and storage-provider integration.
- Policies and credential references: explicit boundaries for resources, network, secrets, lifecycle, and output retention.

Pipeline, deployment, release, and autonomous agent workflows can be built on these primitives later. They should reference sessions, tasks, previews, and artifacts rather than becoming the base runtime model.

## Web Console

The console is a separate Vite app. It consumes `/healthz` and `/v1/*` through the Vite proxy in development.

The current UI is a single operational console with:

- projects table and create dialog
- templates table plus create/edit dialog for ready-to-run environments
- sandboxes table and launch dialog
- compact sandbox row actions for Workspace, start/stop, and delete
- right metadata detail pane
- main-area Runtime Workspace for the selected sandbox

The Runtime Workspace has tabs for Terminal, Storage, Preview, Tasks, Artifacts, Logs, and Events. Terminal is intentionally treated as a primary workspace surface, not as right-sidebar metadata.

The template flow follows the same product direction as E2B-style sandboxes: users pick a ready environment by runtime, use case, entrypoints, resource preset, and validation status. mbox keeps Kubernetes-specific details available under Advanced settings because those fields still drive runtime projection. The default template is a Node.js web app workspace (`node:22-bookworm-slim`, `/workspace`, `web:3000`) so a fresh local sandbox can immediately prove terminal plus preview behavior without users hand-writing the low-level template shape.

## Safety Boundaries

- Normal local API startup writes only to Postgres.
- Kubernetes writes happen only when the runtime controller is explicitly enabled.
- Runtime access needs a Kubernetes client and is explicitly enabled separately.
- Ordinary sandbox Pods do not receive broad Kubernetes tokens by default.
- Do not run Postgres integration tests or Kubernetes smoke tests against external targets unless that target is explicitly intended for test writes.

# Architecture Guide

This guide is the short external handoff for the implemented mbox slice. The fuller product architecture remains in `ARCHITECTURE.md`.

## Current Shape

mbox is split into three active layers:

- Go API server in `cmd/mbox-server`
- Postgres product store through `internal/postgres`
- Vite React console in `web/`

The API server owns product records. Kubernetes resources are runtime projections, not the source of truth.

## Implemented Product Records

- `Project`: codebase or product scope with a default namespace and optional default template.
- `EnvironmentTemplate`: reusable sandbox launch shape with image, command, working directory, resource requests, storage request, and exposed ports.
- `Sandbox`: product-level runtime request with namespace, service account, status, runtime reference, and declared preview ports.

Postgres migrations live in `internal/postgres/migrations`. Startup requires `DATABASE_URL` and runs embedded migrations before serving HTTP.

## Runtime Projection

Runtime reconciliation is disabled by default. When `MBOX_RUNTIME_CONTROLLER_ENABLED=true`, the sandbox reconciler:

1. Lists sandboxes that need runtime reconciliation.
2. Creates or updates the target namespace.
3. Creates or updates the sandbox ServiceAccount with token automount disabled.
4. Creates or updates an `agent-sandbox` `SandboxTemplate`.
5. Creates a `SandboxClaim`.
6. Stores a `runtimeRef` on the mbox sandbox record.
7. Maps runtime status and runtime-reported ports back into Postgres.
8. Deletes the runtime claim when the sandbox is soft-deleted.

The generated pod template also disables service account token automount. When a template has `storageRequest`, the generated `SandboxTemplate` includes a `workspace` PVC template mounted at the template `workingDir`, defaulting to `/workspace`.

## Runtime Access

Runtime access is separately gated by `MBOX_RUNTIME_ACCESS_ENABLED=true`. This enables:

- runtime target resolution
- resolved workspace PVC storage metadata
- logs
- Kubernetes events
- declared preview port listing
- API-proxied preview port access
- browser terminal

The runtime target resolution path is:

1. mbox `Sandbox.runtimeRef`
2. `SandboxClaim.status.sandbox.name`
3. runtime `Sandbox.status.selector`
4. matching Pod, preferring the `workspace` container

The terminal is a WebSocket bridge to Kubernetes `pods/exec`. It only opens for mbox sandboxes with `running` status and only accepts `sh` or `bash`.

Preview ports are allowed only when the port is declared in the mbox sandbox record, the sandbox is running, and the protocol is TCP. The first implementation proxies through the mbox API server to the resolved Kubernetes Pod proxy path.

Runtime target responses include PVC-backed storage metadata by inspecting the resolved Pod's `workspace` container volume mounts and matching PersistentVolumeClaims. The Storage tab uses this to show mount path, claim name, bound phase, capacity, and storage class without exposing raw Kubernetes access to the browser.

## Web Console

The console is a separate Vite app. It consumes `/healthz` and `/v1/*` through the Vite proxy in development.

The current UI is a single operational console with:

- projects table and create dialog
- templates table and create dialog
- sandboxes table and launch dialog
- right metadata detail pane
- main-area Runtime Workspace for the selected sandbox

The Runtime Workspace has tabs for Terminal, Storage, Preview, Logs, and Events. Terminal is intentionally treated as a primary workspace surface, not as right-sidebar metadata.

## Safety Boundaries

- Normal local API startup writes only to Postgres.
- Kubernetes writes happen only when the runtime controller is explicitly enabled.
- Runtime access needs a Kubernetes client and is explicitly enabled separately.
- Ordinary sandbox Pods do not receive broad Kubernetes tokens by default.
- Do not run Postgres integration tests or Kubernetes smoke tests against external targets unless that target is explicitly intended for test writes.

# Server API

This document describes the currently implemented mbox server slice. It is intentionally narrower than the long-term product model in `PRODUCT.md` and `ARCHITECTURE.md`.

Long-term, the API should expose lower-level execution-platform primitives: sandboxes, runtime sessions, execution tasks, previews, artifacts, policies, and credential references. Agent products, CI systems, and deployment tools should call those APIs rather than being built into this server slice as the base model.

## Current Scope

The server is a Go HTTP API backed by Postgres. It stores mbox product records for:

- projects
- environment templates
- sandboxes

`DATABASE_URL` is required. Startup connects to Postgres, runs embedded migrations from `internal/postgres/migrations`, and then serves the API.

The runtime controller is disabled by default. When explicitly enabled, it reconciles mbox `Sandbox` records into `agent-sandbox` runtime resources.

The web console is a separate Vite app under `web/`. In development, Vite proxies `/healthz` and `/v1/*` to the Go API server. See `docs/web-console.md` for frontend structure and verification.

## Configuration

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `DATABASE_URL` | yes | none | Postgres connection string for product state. |
| `MBOX_LISTEN_ADDR` | no | `127.0.0.1:18080` | HTTP listen address. |
| `MBOX_RUNTIME_CONTROLLER_ENABLED` | no | `false` | Enables Kubernetes runtime reconciliation when set to true. |
| `MBOX_RUNTIME_ACCESS_ENABLED` | no | `false` | Enables runtime access routes for terminal, logs, events, preview ports, and runtime target resolution. |
| `MBOX_RUNTIME_RECONCILE_INTERVAL` | no | `5s` | Sandbox reconciler polling interval. |
| `MBOX_KUBECONFIG` | no | in-cluster or default client behavior | Kubeconfig path used by the runtime controller. |
| `MBOX_KUBE_CONTEXT` | no | current context | Kubeconfig context used by the runtime controller. |
| `MBOX_AGENT_SANDBOX_WARM_POOL` | no | empty | Optional `agent-sandbox` warm pool policy value placed on `SandboxClaim.spec.warmpool`. |

Postgres integration tests are opt-in through `MBOX_TEST_DATABASE_URL`.

Frontend development variables live in `web/vite.config.ts`:

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `MBOX_API_PROXY_TARGET` | no | `http://127.0.0.1:18080` | API target used by Vite dev proxy. |
| `MBOX_WEB_PORT` | no | `5174` | Vite dev server port. |

## Routes

All responses are JSON unless the route returns `204 No Content`.

| Method | Path | Notes |
| --- | --- | --- |
| `GET` | `/healthz` | Returns `{"status":"ok"}`. |
| `GET` | `/v1/projects` | Lists projects. |
| `POST` | `/v1/projects` | Creates a project. |
| `GET` | `/v1/projects/{projectID}` | Gets one project. |
| `PATCH` | `/v1/projects/{projectID}` | Updates mutable project fields. |
| `DELETE` | `/v1/projects/{projectID}` | Deletes a project. |
| `GET` | `/v1/templates` | Lists templates. Optional `projectId` query filters project-scoped templates. |
| `POST` | `/v1/templates` | Creates a global or project-scoped template. |
| `GET` | `/v1/templates/{templateID}` | Gets one template. |
| `PATCH` | `/v1/templates/{templateID}` | Updates mutable template fields. |
| `DELETE` | `/v1/templates/{templateID}` | Deletes a template. |
| `GET` | `/v1/sandboxes` | Lists non-deleted sandboxes. Optional `projectId` query filters by project. |
| `POST` | `/v1/sandboxes` | Creates a sandbox product record with `pending` status. |
| `GET` | `/v1/sandboxes/{sandboxID}` | Gets one non-deleted sandbox. |
| `PATCH` | `/v1/sandboxes/{sandboxID}` | Updates mutable sandbox fields. |
| `DELETE` | `/v1/sandboxes/{sandboxID}` | Soft-deletes a sandbox. |
| `POST` | `/v1/sandboxes/{sandboxID}/stop` | Marks the sandbox `stopped`; the controller pauses the runtime when it reconciles. |
| `POST` | `/v1/sandboxes/{sandboxID}/start` | Marks the sandbox `pending`; the controller resumes or creates runtime resources when it reconciles. |
| `GET` | `/v1/sandboxes/{sandboxID}/runtime` | Resolves the runtime Pod target for a ready sandbox. Requires runtime access. |
| `GET` | `/v1/sandboxes/{sandboxID}/logs` | Returns recent logs from the runtime Pod. Optional `tailLines`, default `200`. |
| `GET` | `/v1/sandboxes/{sandboxID}/events` | Returns Kubernetes events for the runtime Pod. |
| `GET` | `/v1/sandboxes/{sandboxID}/ports` | Returns declared sandbox preview ports plus API proxy URLs when available. |
| `GET` | `/v1/sandboxes/{sandboxID}/ports/{port}/proxy/*` | Proxies a declared TCP port on a running sandbox Pod through the API server. |
| `GET` | `/v1/sandboxes/{sandboxID}/terminal` | WebSocket terminal proxy to the runtime Pod shell. |

Errors use:

```json
{"error":"message"}
```

Store errors currently map to:

- `404` for missing resources
- `409` for uniqueness or constraint conflicts
- `500` for other store failures

## Request Notes

Slugs must match:

```text
^[a-z0-9]([a-z0-9-]*[a-z0-9])?$
```

`POST /v1/projects`, `POST /v1/templates`, and `POST /v1/sandboxes` accept an omitted or empty `slug`. In that case the server derives a slug from `name` before validation.

`PATCH /v1/projects/{projectID}` accepts:

- `name`
- `repositoryUrl`
- `defaultNamespace`
- `defaultTemplateId`
- `metadata`

`defaultTemplateId` is nullable. If the field is absent, the existing value is kept. If it is `null`, the default template reference is cleared.

`POST /v1/templates` and `PATCH /v1/templates/{templateID}` accept these template fields:

- `name`
- `image`
- `startupCommand`
- `workingDir`
- `cpuRequest`
- `memoryRequest`
- `storageRequest`
- `exposedPorts`
- `env`
- `secretRefs`
- `networkPolicy`
- `lifecyclePolicy`
- `metadata`

Template `metadata` currently stores the product-facing template library fields: `runtimeType`, `useCase`, `resourcePreset`, and `validationStatus`. Runtime projection still uses the concrete fields such as image, command, resources, ports, storage, env, secrets, network policy, and lifecycle policy.

`validationStatus` is informational for the current UI. Saving a template through the web console resets it to `not_tested` because any edit can invalidate a previous launch validation.

`PATCH /v1/sandboxes/{sandboxID}` accepts:

- `name`
- `status`
- `namespace`
- `serviceAccountName`
- `runtimeRef`
- `ports`
- `metadata`

`runtimeRef` has the same nullable PATCH semantics: absent keeps the existing value, and `null` clears it.

`ports` is the sandbox's declared preview-port list. Templates seed this list from `exposedPorts`, and the web Preview tab updates it through this PATCH route when a user adds or removes a manual preview port.

`POST /v1/sandboxes` accepts `namespace` and `serviceAccountName`, but they are optional on the normal create path:

- If `namespace` is omitted or empty, the project `defaultNamespace` is used.
- If `serviceAccountName` is omitted or empty, `mbox-sandbox` is used.
- If `templateId` is omitted, the project must have `defaultTemplateId` set.
- Sandbox `ports` are initialized from the selected template's `exposedPorts`.

The intended user-facing launch path only needs `projectId`, `name`, and either `templateId` or a project `defaultTemplateId`. Slug, namespace, and ServiceAccount are machine defaults unless a lower-level API client intentionally overrides them.

Valid sandbox statuses are:

- `pending`
- `running`
- `stopped`
- `failed`
- `deleted`

## Data Model

The first migration creates:

- `projects`
- `environment_templates`
- `sandboxes`
- `schema_migrations`

Important constraints:

- UUID primary keys use `pgcrypto` `gen_random_uuid()`.
- `projects.slug` is globally unique.
- Global templates have unique `slug` where `project_id IS NULL`.
- Project templates have unique `(project_id, slug)` where `project_id IS NOT NULL`.
- Active sandboxes have unique `(project_id, slug)` where `deleted_at IS NULL`.
- `sandboxes` are soft-deleted by setting `status = 'deleted'` and `deleted_at = now()`.
- `updated_at` is maintained by Postgres triggers.
- `environment_templates.metadata` is `JSONB NOT NULL DEFAULT '{}'::jsonb`; existing databases receive it through `002_template_metadata.sql`.

Product records stay separate from Kubernetes runtime resources. Postgres remains the product source of truth.

## Runtime Projection

The runtime controller only runs when `MBOX_RUNTIME_CONTROLLER_ENABLED=true`.

For each reconciled sandbox:

1. If the sandbox is active and has no `runtimeRef`, mbox loads the template and creates runtime resources.
2. The `agent-sandbox` adapter ensures the namespace exists.
3. It creates or updates the configured sandbox ServiceAccount with token automount disabled.
4. It creates or updates a namespaced `SandboxTemplate`.
5. It creates a namespaced `SandboxClaim`.
6. It stores a `runtimeRef` pointing at the `SandboxClaim`.
7. It maps the `SandboxClaim` Ready condition to mbox sandbox status.
8. If the mbox sandbox is stopped, it resolves the runtime `Sandbox` from the `SandboxClaim` and scales it to zero replicas.
9. If a stopped sandbox is started, it marks the record `pending` and scales the existing runtime `Sandbox` back to one replica.
10. If the mbox sandbox is soft-deleted, it deletes the `SandboxClaim` and clears the `runtimeRef`.

When `EnvironmentTemplate.storageRequest` is set, the adapter adds a `workspace` `volumeClaimTemplates` entry and mounts it into the workspace container at the template `workingDir`, defaulting to `/workspace`. This is the Phase 1 persistence contract: workspace data should survive runtime Pod replacement and sandbox stop/start while the sandbox exists. Files written outside persistent workspace storage are container-local and can be lost when a stopped sandbox's Pod is removed. PVC deletion behavior after sandbox deletion is owned by the runtime controller and must be checked in smoke tests for the target cluster.

Runtime reference shape:

```json
{
  "adapter": "agent-sandbox",
  "kind": "SandboxClaim",
  "namespace": "mbox-demo",
  "name": "demo-sandbox-12345678"
}
```

The generated pod template sets `serviceAccountName` and `automountServiceAccountToken: false`. This keeps ordinary sandbox pods from receiving broad Kubernetes credentials by default.

## Runtime Access

Runtime access routes are available only when `MBOX_RUNTIME_ACCESS_ENABLED=true`, because the server needs explicit permission to proxy terminal, logs, events, and runtime target reads through its Kubernetes client.

The terminal route upgrades to WebSocket and proxies browser input/output to Kubernetes `pods/exec` for the resolved sandbox Pod. The server resolves the runtime target through:

1. mbox `Sandbox.runtimeRef`
2. `SandboxClaim.status.sandbox.name`
3. `Sandbox.status.selector`
4. the matching Pod and `workspace` container when present

The runtime target response includes persistent storage metadata when the resolved container mounts PVC-backed volumes:

```json
{
  "namespace": "mbox-demo",
  "podName": "demo-pod",
  "container": "workspace",
  "phase": "Running",
  "selector": "agents.x-k8s.io/sandbox=demo",
  "storage": [
    {
      "name": "workspace",
      "mountPath": "/workspace",
      "claimName": "workspace-demo",
      "phase": "Bound",
      "capacity": "1Gi",
      "storageClassName": "standard"
    }
  ]
}
```

The terminal route only opens for sandboxes whose mbox status is `running`. The default shell command is `/bin/sh`. Passing `?shell=bash` requests `/bin/bash`; other shell values are rejected.

The preview port route exposes only sandbox ports declared in the mbox sandbox record and only for TCP ports while the sandbox is `running`. Declaring a port is separate from proving a process is listening on that port: users can start a service inside the terminal, add the TCP port to the Preview tab, and then open the generated API proxy URL after the sandbox is running. The first implementation proxies through the Kubernetes Pod proxy behind the mbox API server:

```text
/v1/sandboxes/{sandboxID}/ports/{port}/proxy/
```

This keeps the browser using the mbox API surface instead of direct Kubernetes access. Gateway, Ingress, and public preview URLs remain future exposure mechanisms.

## Verification

Default verification:

```sh
go test ./...
cd web && npm run build
```

Runtime smoke verification against a cluster with `agent-sandbox` installed:

```sh
export MBOX_API_URL=http://127.0.0.1:18080
export MBOX_KUBECONFIG="$HOME/.kube/config"
export MBOX_KUBE_CONTEXT=kind-agent-sandbox
./scripts/smoke-agent-sandbox.sh
```

The smoke script verifies runtime projection, terminal-ready Pod startup, ServiceAccount token automount disabled, workspace PVC projection, file persistence across Pod replacement, runtime storage metadata, preview-port metadata, logs, events, and `SandboxClaim` cleanup.

Optional Postgres integration verification:

```sh
export MBOX_TEST_DATABASE_URL='postgres://mbox:mbox@127.0.0.1:5432/mbox_test?sslmode=disable'
go test ./internal/postgres
```

Do not run the Postgres integration test against an external database unless that database is explicitly intended for test writes.

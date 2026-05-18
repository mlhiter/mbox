# Server API

This document describes the currently implemented mbox server slice. It is intentionally narrower than the long-term product model in `PRODUCT.md` and `ARCHITECTURE.md`.

## Current Scope

The server is a Go HTTP API backed by Postgres. It stores mbox product records for:

- projects
- environment templates
- sandboxes

`DATABASE_URL` is required. Startup connects to Postgres, runs embedded migrations from `internal/postgres/migrations`, and then serves the API.

The runtime controller is disabled by default. When explicitly enabled, it reconciles mbox `Sandbox` records into `agent-sandbox` runtime resources.

## Configuration

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `DATABASE_URL` | yes | none | Postgres connection string for product state. |
| `MBOX_LISTEN_ADDR` | no | `127.0.0.1:8080` | HTTP listen address. |
| `MBOX_RUNTIME_CONTROLLER_ENABLED` | no | `false` | Enables Kubernetes runtime reconciliation when set to true. |
| `MBOX_RUNTIME_RECONCILE_INTERVAL` | no | `5s` | Sandbox reconciler polling interval. |
| `MBOX_KUBECONFIG` | no | in-cluster or default client behavior | Kubeconfig path used by the runtime controller. |
| `MBOX_KUBE_CONTEXT` | no | current context | Kubeconfig context used by the runtime controller. |
| `MBOX_AGENT_SANDBOX_WARM_POOL` | no | empty | Optional `agent-sandbox` warm pool policy value placed on `SandboxClaim.spec.warmpool`. |

Postgres integration tests are opt-in through `MBOX_TEST_DATABASE_URL`.

## Routes

All responses are JSON unless the route returns `204 No Content`.

| Method | Path | Notes |
| --- | --- | --- |
| `GET` | `/` | Serves the embedded web console. |
| `GET` | `/console` | Serves the embedded web console. |
| `GET` | `/console/app.css` | Serves console styles. |
| `GET` | `/console/app.js` | Serves console JavaScript. |
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

`PATCH /v1/projects/{projectID}` accepts:

- `name`
- `repositoryUrl`
- `defaultNamespace`
- `defaultTemplateId`
- `metadata`

`defaultTemplateId` is nullable. If the field is absent, the existing value is kept. If it is `null`, the default template reference is cleared.

`PATCH /v1/sandboxes/{sandboxID}` accepts:

- `name`
- `status`
- `namespace`
- `serviceAccountName`
- `runtimeRef`
- `ports`
- `metadata`

`runtimeRef` has the same nullable PATCH semantics: absent keeps the existing value, and `null` clears it.

`POST /v1/sandboxes` accepts `namespace` and `serviceAccountName`, but they are optional on the normal create path:

- If `namespace` is omitted or empty, the project `defaultNamespace` is used.
- If `serviceAccountName` is omitted or empty, `mbox-sandbox` is used.
- If `templateId` is omitted, the project must have `defaultTemplateId` set.

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
8. If the mbox sandbox is soft-deleted, it deletes the `SandboxClaim` and clears the `runtimeRef`.

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

## Verification

Default verification:

```sh
go test ./...
```

Runtime smoke verification against a cluster with `agent-sandbox` installed:

```sh
export MBOX_API_URL=http://127.0.0.1:8080
export MBOX_KUBECONFIG="$HOME/.kube/config"
export MBOX_KUBE_CONTEXT=kind-agent-sandbox
./scripts/smoke-agent-sandbox.sh
```

Optional Postgres integration verification:

```sh
export MBOX_TEST_DATABASE_URL='postgres://mbox:mbox@127.0.0.1:5432/mbox_test?sslmode=disable'
go test ./internal/postgres
```

Do not run the Postgres integration test against an external database unless that database is explicitly intended for test writes.

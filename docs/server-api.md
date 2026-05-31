# Server API

This document describes the currently implemented mbox server slice. It is intentionally narrower than the long-term product model in `PRODUCT.md` and `ARCHITECTURE.md`.

Long-term, the API should expose lower-level execution-platform primitives: sandboxes, runtime sessions, execution tasks, previews, artifacts, policies, and credential references. Agent products, CI systems, and deployment tools should call those APIs rather than being built into this server slice as the base model.

## Current Scope

The server is a Go HTTP API backed by Postgres. It stores mbox product records for:

- projects
- project-scoped audit events
- project policies
- project quota policies
- environment templates
- sandboxes
- execution tasks
- artifacts

`DATABASE_URL` is required. Startup connects to Postgres, runs embedded migrations from `internal/postgres/migrations`, and then serves the API.

The runtime controller is disabled by default. When explicitly enabled, it reconciles mbox `Sandbox` records into `agent-sandbox` runtime resources.

The web console is a separate Vite app under `web/`. In development, Vite proxies `/healthz` and `/v1/*` to the Go API server. See `docs/web-console.md` for frontend structure and verification.

The first SDK surface is a TypeScript package under `sdk/typescript`. It is a thin client over these same HTTP routes for external agents, IDE integrations, CI scripts, release tools, and other automation clients. The first CLI surface lives under `cmd/mbox` and follows the same rule: it maps user commands to public HTTP routes without direct database or Kubernetes access. These client surfaces do not add agent or workflow semantics to the server; they help callers compose the lower-level primitives.

The machine-readable API contract starts at `GET /v1/openapi.json`. It is published from the API server so CLI, SDK, documentation tooling, and future schema-generation work can inspect the implemented route surface without reading Go internals.

## Configuration

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `DATABASE_URL` | yes | none | Postgres connection string for product state. |
| `MBOX_LISTEN_ADDR` | no | `127.0.0.1:18080` | HTTP listen address. |
| `MBOX_API_TOKEN` | no | empty | Optional shared bearer token for CLI, SDK, and automation clients. When set, private API routes require `Authorization: Bearer <token>`. |
| `MBOX_RUNTIME_CONTROLLER_ENABLED` | no | `false` | Enables Kubernetes runtime reconciliation when set to true. |
| `MBOX_RUNTIME_ACCESS_ENABLED` | no | `false` | Enables runtime access routes for terminal, execution tasks, logs, events, preview ports, and runtime target resolution. |
| `MBOX_RUNTIME_RECONCILE_INTERVAL` | no | `5s` | Sandbox reconciler polling interval. |
| `MBOX_ARTIFACT_CONTENT_BACKEND` | no | `postgres` | Retained artifact content backend. Supported values: `postgres`, `filesystem`, `s3`. |
| `MBOX_ARTIFACT_CONTENT_DIR` | no | `.mbox/artifacts` when filesystem is enabled | Local directory for retained bytes when `MBOX_ARTIFACT_CONTENT_BACKEND=filesystem`. |
| `MBOX_ARTIFACT_CONTENT_S3_ENDPOINT` | when backend is `s3` | none | S3-compatible endpoint for retained artifact bytes. |
| `MBOX_ARTIFACT_CONTENT_S3_REGION` | no | `us-east-1` | Region used for S3 SigV4 signing. |
| `MBOX_ARTIFACT_CONTENT_S3_BUCKET` | when backend is `s3` | none | Bucket for retained artifact bytes. |
| `MBOX_ARTIFACT_CONTENT_S3_PREFIX` | no | empty | Optional object key prefix for retained artifact bytes. |
| `MBOX_ARTIFACT_CONTENT_S3_ACCESS_KEY_ID` | when backend is `s3` | none | Access key used only by the server-side retained-content backend. |
| `MBOX_ARTIFACT_CONTENT_S3_SECRET_ACCESS_KEY` | when backend is `s3` | none | Secret key used only by the server-side retained-content backend. |
| `MBOX_ARTIFACT_CONTENT_S3_FORCE_PATH_STYLE` | no | `true` | Uses path-style bucket URLs for S3-compatible services such as local MinIO or private object-store gateways. |
| `MBOX_KUBECONFIG` | no | in-cluster or default client behavior | Kubeconfig path used by the runtime controller. |
| `MBOX_KUBE_CONTEXT` | no | current context | Kubeconfig context used by the runtime controller. |
| `MBOX_AGENT_SANDBOX_WARM_POOL` | no | empty | Optional `agent-sandbox` warm pool policy value placed on `SandboxClaim.spec.warmpool`. |

Postgres integration tests are opt-in through `MBOX_TEST_DATABASE_URL`.

Frontend development variables live in `web/vite.config.ts`:

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `MBOX_API_PROXY_TARGET` | no | `http://127.0.0.1:18080` | API target used by Vite dev proxy. |
| `MBOX_TOKEN` / `MBOX_API_TOKEN` | no | empty | Optional token used by the Vite dev proxy to add `Authorization: Bearer <token>` when proxying to an authenticated local API. |
| `MBOX_WEB_PORT` | no | `5174` | Vite dev server port. |

## Routes

All responses include `X-Mbox-Request-ID`. If the client sends `X-Mbox-Request-ID`, the server trims and echoes that value; otherwise it generates one. Responses are JSON unless the route returns `204 No Content`.

| Method | Path | Notes |
| --- | --- | --- |
| `GET` | `/healthz` | Returns `{"status":"ok"}`. |
| `GET` | `/v1/info` | Returns API version, server version, enabled runtime/artifact capabilities, and compatibility hints for CLI/SDK clients. |
| `GET` | `/v1/openapi.json` | Returns the current OpenAPI 3.1 contract starter for implemented routes, schemas, and bearer-auth security metadata. |
| `GET` | `/v1/runtime/resources` | Lists the current mbox-managed runtime resources reported by the runtime auditor. Optional `namespace` and `kind` queries scope the inventory. Requires a configured runtime auditor. |
| `GET` | `/v1/runtime/orphans` | Read-only operational audit for mbox-managed runtime resources whose Kubernetes labels no longer line up cleanly with product records. Optional `namespace` and `kind` queries scope the report. Requires a configured runtime auditor. |
| `GET` | `/v1/audit-events` | Lists recent product audit events. Optional `projectId`, `action`, `resourceType`, `resourceId`, `actor`, `source`, `requestId`, `operation`, `since`, `until`, and `limit` query filters. |
| `GET` | `/v1/projects` | Lists projects. |
| `POST` | `/v1/projects` | Creates a project. |
| `GET` | `/v1/projects/{projectID}` | Gets one project. |
| `PATCH` | `/v1/projects/{projectID}` | Updates mutable project fields. |
| `DELETE` | `/v1/projects/{projectID}` | Deletes a project. |
| `GET` | `/v1/projects/{projectID}/policy` | Gets the effective project launch policy; missing policies return disabled defaults. |
| `PUT` | `/v1/projects/{projectID}/policy` | Upserts the project launch policy. |
| `GET` | `/v1/projects/{projectID}/quota-policy` | Gets the effective project quota policy; missing policies return disabled defaults. |
| `PUT` | `/v1/projects/{projectID}/quota-policy` | Upserts the project quota policy. |
| `GET` | `/v1/projects/{projectID}/credentials` | Lists project credential-reference records. |
| `POST` | `/v1/projects/{projectID}/credentials` | Creates a project credential-reference record. |
| `GET` | `/v1/projects/{projectID}/usage` | Returns a read-only product-record usage summary for project sandboxes, sessions, tasks, artifacts, templates, and credential references. |
| `GET` | `/v1/projects/{projectID}/audit-events` | Lists recent product audit events for one project. Optional `action`, `resourceType`, `resourceId`, `actor`, `source`, `requestId`, `operation`, `since`, `until`, and `limit` query filters. |
| `GET` | `/v1/credentials/{credentialID}` | Gets one project credential-reference record. |
| `DELETE` | `/v1/credentials/{credentialID}` | Deletes one project credential-reference record. |
| `GET` | `/v1/templates` | Lists templates. Optional `projectId` query filters project-scoped templates. |
| `POST` | `/v1/templates` | Creates a global or project-scoped template. |
| `GET` | `/v1/templates/{templateID}` | Gets one template. |
| `PATCH` | `/v1/templates/{templateID}` | Updates mutable template fields. |
| `DELETE` | `/v1/templates/{templateID}` | Deletes a template. |
| `GET` | `/v1/templates/{templateID}/boundary` | Returns the template's namespace, identity, secret, network, lifecycle, and cleanup boundary summary. Global templates can accept `projectId`. |
| `POST` | `/v1/templates/{templateID}/validation-runs` | Launches a validation sandbox and marks the template validation status as `testing`. |
| `POST` | `/v1/templates/{templateID}/validation-runs/{sandboxID}/decision` | Records a validation sandbox decision as `passed` or `failed`. |
| `GET` | `/v1/sandboxes` | Lists non-deleted sandboxes. Optional `projectId` query filters by project. |
| `POST` | `/v1/sandboxes` | Creates a sandbox product record with `pending` status. |
| `GET` | `/v1/sandboxes/{sandboxID}` | Gets one non-deleted sandbox. |
| `PATCH` | `/v1/sandboxes/{sandboxID}` | Updates mutable sandbox fields. |
| `DELETE` | `/v1/sandboxes/{sandboxID}` | Soft-deletes a sandbox. |
| `GET` | `/v1/sandboxes/{sandboxID}/boundary` | Returns the resolved runtime boundary summary for one sandbox. |
| `POST` | `/v1/sandboxes/{sandboxID}/stop` | Marks the sandbox `stopped`; the controller pauses the runtime when it reconciles. |
| `POST` | `/v1/sandboxes/{sandboxID}/start` | Marks the sandbox `pending`; the controller resumes or creates runtime resources when it reconciles. |
| `GET` | `/v1/sandboxes/{sandboxID}/runtime` | Resolves the runtime Pod target for a ready sandbox. Requires runtime access. |
| `GET` | `/v1/sandboxes/{sandboxID}/logs` | Returns recent logs from the runtime Pod. Optional `tailLines`, default `200`. |
| `GET` | `/v1/sandboxes/{sandboxID}/events` | Returns Kubernetes events for the runtime Pod. |
| `GET` | `/v1/sandboxes/{sandboxID}/ports` | Returns declared sandbox preview ports plus API proxy URLs when available. |
| `GET` | `/v1/sandboxes/{sandboxID}/ports/{port}/proxy/*` | Proxies a declared TCP port on a running sandbox Pod through the API server. |
| `GET` | `/v1/sandboxes/{sandboxID}/terminal` | WebSocket terminal proxy to the runtime Pod shell; creates a terminal runtime session record. |
| `GET` | `/v1/sandboxes/{sandboxID}/sessions` | Lists runtime session records for a sandbox. |
| `POST` | `/v1/sandboxes/{sandboxID}/sessions` | Creates a runtime session audit record for a terminal, IDE, notebook, browser, command, or custom client. |
| `GET` | `/v1/sandboxes/{sandboxID}/tasks` | Lists execution tasks recorded for a sandbox. |
| `POST` | `/v1/sandboxes/{sandboxID}/tasks` | Creates one asynchronous command task in a running sandbox and records output. Requires runtime access. |
| `GET` | `/v1/sandboxes/{sandboxID}/artifacts` | Lists artifact references recorded for a sandbox. |
| `POST` | `/v1/sandboxes/{sandboxID}/artifacts` | Creates an artifact reference for a sandbox, optionally linked to a task. |
| `GET` | `/v1/sessions/{sessionID}` | Gets one runtime session record. |
| `POST` | `/v1/sessions/{sessionID}/end` | Marks a runtime session ended with `endedAt`. |
| `GET` | `/v1/tasks/{taskID}` | Gets one execution task. |
| `GET` | `/v1/tasks/{taskID}/events` | Streams newline-delimited JSON task events: snapshot, status, output, and done. |
| `POST` | `/v1/tasks/{taskID}/cancel` | Cancels a queued or running execution task on the current API server. |
| `GET` | `/v1/tasks/{taskID}/artifacts` | Lists artifact references linked to one execution task. |
| `GET` | `/v1/artifacts/{artifactID}` | Gets one artifact reference. |
| `POST` | `/v1/artifacts/{artifactID}/capture` | Captures retained bytes for a `workspace://` file artifact from a running sandbox workspace. Requires runtime access. |
| `PUT` | `/v1/artifacts/{artifactID}/content` | Uploads client-provided bytes into the retained-content store for one non-directory artifact. |
| `GET` | `/v1/artifacts/{artifactID}/content` | Reads retained artifact bytes when present; otherwise reads `workspace://` file content from the running sandbox workspace. |

Errors use:

```json
{"error":"message"}
```

Store errors currently map to:

- `404` for missing resources
- `409` for uniqueness, constraint conflicts, or cleanup guards
- `503` for runtime operational routes that require a runtime auditor or access adapter that is not configured
- `500` for other store failures

## API Info

`GET /v1/info` is the client handshake route. It is more specific than `/healthz`: health only proves the server can answer, while info tells CLI, SDK, web console, and external integrations which execution-platform primitives and runtime options are available on this server process.

The route is read-only and does not touch Kubernetes or mutate Postgres. Its response includes:

- `apiVersion`: current public API compatibility label. This slice reports `v1alpha1`.
- `serverVersion`: server build/runtime version. Local development defaults to `0.1.0-dev`; deployments can set `MBOX_SERVER_VERSION`.
- `runtimeController`: whether this server process can reconcile mbox sandboxes into runtime resources.
- `runtimeAccess`: whether this server process exposes runtime target, terminal, logs, events, task execution, preview proxy, and workspace artifact reads.
- `artifactContent`: retained-content support, storage provider, and maximum retained byte size.
- `capabilities`: stable feature flags for implemented product primitives, such as `sandboxes`, `openapi`, `project-usage`, `project-quota-policies`, `audit-events`, `execution-tasks`, `task-events`, `artifact-client-upload`, `project-delete-cleanup-guard`, `runtime-orphan-audit`, and `runtime-orphan-cleanup`.
- `compatibility`: minimum CLI and SDK API compatibility labels expected by this server.
- `authenticationRequired`: `true` when `MBOX_API_TOKEN` is configured, otherwise `false`; clients should use this discovery bit instead of inferring auth from failures.

### Authentication

By default, local development remains unauthenticated. Set `MBOX_API_TOKEN` to enable the starter API token model. When configured, `GET /healthz` and `GET /v1/info` remain public so clients can discover health, API version, capability flags, and `authenticationRequired`; every other route, including `GET /v1/openapi.json`, requires `Authorization: Bearer <token>`.

The OpenAPI contract publishes `components.securitySchemes.bearerAuth` with HTTP bearer auth. Public operations such as `/healthz` and `/v1/info` explicitly publish `security: []`; private operations publish `security: [{"bearerAuth":[]}]` and a `401` response using the shared `Error` schema.

The CLI reads `MBOX_TOKEN` or `--token`, and the TypeScript SDK accepts `new MboxClient({ token })`. This is a process-level shared secret for automation clients, not a user identity model, RBAC system, or project permission model. Audit attribution headers are still client-supplied labels and are not proof of identity.

### API Compatibility Policy

The `apiVersion` and compatibility fields are public API compatibility labels, not server binary versions or npm package versions. The current label family is `v1alpha1`.

Clients and servers currently accept labels in the form `vNalphaM`, `vNbetaM`, or `vN`. Compatibility is intentionally conservative:

- The client label must parse successfully.
- The server `apiVersion` and the relevant minimum client label must parse successfully.
- The client, server, and minimum labels must be in the same major family, such as `v1`.
- Within one family, ordering is `alpha` < `beta` < stable, and higher numeric prerelease labels satisfy lower minimum labels.
- A client is compatible when its label is greater than or equal to the server's relevant minimum label for that client kind.

For example, a CLI labeled `v1beta1` can satisfy a server minimum of `v1alpha2`, and a stable `v1` client can satisfy `v1beta1`. A `v1` client does not satisfy a `v2alpha1` server minimum, even if the HTTP route still exists.

Capabilities are separate feature gates. A client that needs task streaming should require `task-events`; a client that uploads retained artifact bytes should require `artifact-client-upload`. This lets external agents, IDE integrations, CI scripts, and release tools fail fast before a longer run without treating every optional primitive as a new API version. Capability checks are not authentication or authorization.

## Runtime Inventory And Orphan Audit

`GET /v1/runtime/resources` is a read-only operational route. It lists the current mbox-managed `agent-sandbox` runtime resources reported by the runtime adapter, including kind, namespace, name, label-derived owner, raw labels, and creation time when available. It also returns a small summary with total resources plus counts by kind, namespace, and owner. This is live runtime inventory visibility, not product-record usage, quota, billing, or live cluster capacity management. It does not compare against Postgres product records and does not delete or patch Kubernetes resources.

Use `?namespace=<name>` to scope the response to one namespace, and `?kind=SandboxClaim` or `?kind=SandboxTemplate` to inspect one managed resource kind. Filters can be combined, which is useful for per-smoke or per-project checks on clusters that may already contain older mbox-managed resources.

`GET /v1/runtime/orphans` uses the same runtime inventory, then compares labels with the Postgres product records to find drift. It also stays read-only. The OpenAPI contract publishes structured schemas for the inventory, orphan audit, orphan entries, and the gated cleanup request/result so CLI and SDK clients can validate the fields they render.

The inventory response shape is:

```json
{
  "adapter": "agent-sandbox",
  "checkedAt": "2026-05-29T00:00:00Z",
  "summary": {
    "total": 2,
    "byKind": [
      {
        "name": "SandboxClaim",
        "count": 1
      },
      {
        "name": "SandboxTemplate",
        "count": 1
      }
    ],
    "byNamespace": [
      {
        "name": "mbox-smoke-20260529",
        "count": 2
      }
    ],
    "byOwner": [
      {
        "name": "project/.../sandbox/...",
        "count": 1
      },
      {
        "name": "template/...",
        "count": 1
      }
    ]
  },
  "items": [
    {
      "adapter": "agent-sandbox",
      "kind": "SandboxClaim",
      "namespace": "mbox-smoke-20260529",
      "name": "demo",
      "owner": {
        "kind": "sandbox",
        "projectId": "...",
        "sandboxId": "..."
      },
      "labels": {
        "mbox.dev/project-id": "...",
        "mbox.dev/sandbox-id": "..."
      }
    }
  ]
}
```

The orphan-audit response shape is:

```json
{
  "adapter": "agent-sandbox",
  "checkedAt": "2026-05-29T00:00:00Z",
  "namespace": "mbox-smoke-20260529",
  "resourceCount": 2,
  "orphanCount": 0,
  "expectedClean": true,
  "items": []
}
```

When `items` is non-empty, each entry includes:

- `reason`: one of `missing-sandbox-record`, `cleanup-pending`, `runtime-ref-mismatch`, `missing-template-record`, or `unlabeled-owner`.
- `resource`: the managed runtime resource identity, kind, namespace, name, labels, and creation time when available.
- optional product references such as `sandboxId`, `templateId`, `projectId`, `runtimeRef`, `status`, and `deletedAt`.
- `message` and `evidence` for operator-readable diagnosis.

The inventory and orphan routes return `503` when no runtime auditor is configured. In local development that means neither runtime controller nor runtime access was enabled for the server process.

`POST /v1/runtime/orphans/cleanup` deletes one currently reported orphan runtime resource. It is intentionally gated and does not run automatic cleanup. The request must include the exact resource identity, the current audit reason, `deleteOrphan: true`, and the confirmation string:

```json
{
  "resource": {
    "adapter": "agent-sandbox",
    "kind": "SandboxClaim",
    "namespace": "mbox-old",
    "name": "old-claim"
  },
  "reason": "missing-sandbox-record",
  "deleteOrphan": true,
  "confirm": "delete-orphan-runtime-resource"
}
```

The server re-runs the orphan audit before deleting. It returns `409` if the resource is no longer an orphan or if the reason changed, and `503` when no runtime cleaner is configured. The first adapter implementation only deletes mbox-managed `agent-sandbox` `SandboxClaim` and `SandboxTemplate` resources.

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

`DELETE /v1/projects/{projectID}` is guarded against losing runtime cleanup state. It returns `409` while the project still has any non-deleted sandbox, or any soft-deleted sandbox whose `runtimeRef` has not been cleared by the reconciler yet. Delete sandboxes first and wait for runtime cleanup before deleting the project. This prevents a hard project delete from cascading away product rows while Kubernetes `SandboxClaim` cleanup is still pending.

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

Template `metadata` currently stores the product-facing template library fields: `runtimeType`, `useCase`, `resourcePreset`, `validationStatus`, `validationSandboxId`, `validationStartedAt`, and `validationDecidedAt`. Runtime projection still uses the concrete fields such as image, command, resources, ports, storage, env, secrets, network policy, and lifecycle policy.

`POST /v1/templates/{templateID}/validation-runs` creates a sandbox from the template and records validation metadata on both resources. Project-scoped templates derive the project from the template; global templates require `projectId` in the request body. The response shape is:

```json
{
  "template": {"id": "template-id"},
  "sandbox": {"id": "validation-sandbox-id"}
}
```

`POST /v1/templates/{templateID}/validation-runs/{sandboxID}/decision` accepts `{"status":"passed"}` or `{"status":"failed"}` and updates the template plus validation sandbox metadata. Saving a template through the web console still resets `validationStatus` to `not_tested` because any edit can invalidate a previous launch validation.

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

Sandbox launch rejects a project-scoped template that belongs to a different project. `GET /v1/projects/{projectID}/policy` returns the effective launch policy:

```json
{
  "projectId": "project-id",
  "enforcement": "disabled",
  "allowedImagePrefixes": [],
  "allowedServiceAccounts": [],
  "allowedSecretRefs": []
}
```

`PUT /v1/projects/{projectID}/policy` accepts:

- `enforcement`: required. `disabled` or `enforced`.
- `allowedImagePrefixes`: optional string array. When non-empty and enforced, the template image must start with one of these prefixes.
- `allowedServiceAccounts`: optional string array. When non-empty and enforced, the sandbox ServiceAccount must be listed.
- `allowedSecretRefs`: optional string array. When enforced, every declared template `secretRefs[].name` must be listed; an empty list means templates with secret references are denied.

When enforcement is `enforced`, `POST /v1/sandboxes` and `POST /v1/templates/{templateID}/validation-runs` can return `403` with a `policy denied: ...` error. This is a launch gate, not full RBAC, credential mounting, or custom NetworkPolicy projection. Lifecycle policy enforcement is separate and currently covers only template `lifecyclePolicy.ttlSeconds`.

`GET /v1/projects/{projectID}/quota-policy` returns the effective project quota policy:

```json
{
  "projectId": "project-id",
  "enforcement": "disabled",
  "maxActiveSandboxes": 5,
  "maxRetainedArtifactBytes": 1048576
}
```

`PUT /v1/projects/{projectID}/quota-policy` accepts:

- `enforcement`: required. `disabled` or `enforced`.
- `maxActiveSandboxes`: optional non-negative integer. When enforced, `POST /v1/sandboxes` is denied once active sandbox product records are at or above the limit.
- `maxRetainedArtifactBytes`: optional non-negative integer. When enforced, artifact capture/upload is denied if current retained artifact bytes plus incoming bytes would exceed the limit.

This is a product-record guard, not live cluster capacity management, billing, reservation, or real-time Kubernetes metrics. The checks use the same project usage aggregation as the read-only usage summary.

`POST /v1/projects/{projectID}/credentials` accepts:

- `name`: required display name.
- `slug`: optional stable key; derived from name when omitted.
- `type`: required. One of `git`, `registry`, `kubernetes`, `ssh`, or `generic`.
- `target`: optional repository URL, registry host, cluster name, or service endpoint.
- `secretRef`: required object with `name` and optional `key`.
- `usage`: optional labels such as `clone`, `fetch`, `push`, `pull`, or `deploy`.
- `metadata`: optional JSON object.

Project credential records are references only. mbox stores the Secret name/key and metadata, but not secret values, and the current runtime adapter does not mount these credentials into sandbox Pods.

`GET /v1/projects/{projectID}/usage` is a read-only operational summary over mbox product records. It reports sandbox status counts, cleanup-pending soft-deleted sandboxes, runtime session counts, execution task status counts, artifact counts and retained bytes, visible template resource-request strings, active/running sandbox declared resource-request totals, and credential-reference counts. Sandbox request totals are derived by joining active sandbox product records to their saved templates and summing parseable Kubernetes quantity strings for CPU, memory, and storage; missing or invalid request strings are counted but do not make the usage route fail. The OpenAPI contract publishes this shape through `ProjectUsage`, `ProjectSandboxUsage`, `SandboxResourceRequestUsage`, and `ResourceQuantityUsage`. It does not read live Kubernetes metrics. Project quota policies use this product-record aggregation for the currently implemented sandbox-count and retained-byte enforcement points.

`GET /v1/audit-events` and `GET /v1/projects/{projectID}/audit-events` list recent product audit events recorded after successful API write operations and selected policy/quota denials. Query filters are `projectId` for the global route, `action`, `resourceType`, `resourceId`, `actor`, `source`, `requestId`, `operation`, `since`, `until`, and `limit` from 1 to 200. `since` and `until` are inclusive RFC3339 timestamps applied to `createdAt`; the server returns `400` if either timestamp is invalid or `since` is after `until`. `requestId` filters against `metadata.requestId`, which is present only on events written through an HTTP request carrying or receiving an mbox request ID. `operation` filters against `metadata.operation`, which is currently useful for typed `policy.denied` events such as `sandbox.launch`, `template.validation`, `artifact.content.capture`, and `artifact.content.upload`; events without that metadata key do not match an operation filter. Events include `action`, `resourceType`, optional project/resource IDs, resource name, actor, source, metadata, and creation time.

Clients can send `X-Mbox-Request-ID` to correlate an API response with server logs and any audit event written during that request. When a best-effort audit event is written, the same value is recorded in `metadata.requestId`; audit feeds can then be narrowed with `?requestId=...`. CLI and SDK clients expose request ID headers through `--request-id`, `MBOX_REQUEST_ID`, and SDK `requestId`, and expose audit-feed filtering through `--filter-request-id`, `--operation`, and SDK audit list `requestId` / `operation`. Request IDs improve traceability; they are not authentication, authorization, idempotency keys, or a guarantee that every request writes an audit event.

Write requests can set client-supplied attribution with `X-Mbox-Audit-Actor` and `X-Mbox-Audit-Source`; the server trims and bounds those labels, and defaults source to `http-api` when no source is supplied. CLI and SDK clients expose this through `--audit-actor` / `--audit-source`, `MBOX_AUDIT_ACTOR` / `MBOX_AUDIT_SOURCE`, and SDK `auditActor` / `auditSource` options. This attribution improves operator visibility but is not authentication, authorization, or a trusted identity proof.

This is a best-effort product-record audit starter: it currently records successful mbox API mutations such as project/template/sandbox writes, launch policy changes, quota policy changes, credential-reference changes, validation decisions, runtime session lifecycle, task creation/cancel requests, artifact creation/content retention, and gated runtime orphan cleanup. It also records `policy.denied` events for project launch policy denials, active sandbox quota denials, and retained artifact byte quota denials. The OpenAPI contract publishes `AuditEventAction` and `PolicyDeniedAuditMetadata`; for `policy.denied`, metadata always includes `operation` and `reason` and may include `requestId` for request/log correlation. Current `operation` values are `sandbox.launch`, `template.validation`, `artifact.content.capture`, and `artifact.content.upload`. It is not yet a strong transactional audit log, auth identity model, or general failure-event stream.

`GET /v1/templates/{templateID}/boundary` and `GET /v1/sandboxes/{sandboxID}/boundary` are read-only policy-boundary summaries. They answer the current runtime safety questions from the existing project, project launch policy, project credential references, template, sandbox, and runtime projection contract:

- namespace and ServiceAccount identity
- ServiceAccount token automount expectation
- project launch policy state and checks
- visible secret reference names and whether they are projected
- project credential reference names and whether they are projected
- network policy field and current projection state
- lifecycle policy field and current enforcement state
- controller operations, runtime access paths, and cleanup behavior
- machine-readable `checks` with `pass`, `warn`, or `fail`

The current adapter keeps sandbox ServiceAccount token automount disabled and does not mount template `secretRefs` or project credential references. Project launch policy can deny launches that reference unapproved template secret names, but it still does not project secret values. The `networkPolicy` field is recorded on the template while `agent-sandbox` manages the baseline runtime NetworkPolicy; custom egress policy projection remains future policy work. Lifecycle policy `ttlSeconds` is enforced by the reconciler as sandbox auto-cleanup; idle timeout and richer cleanup policies remain future work.

Valid sandbox statuses are:

- `pending`
- `running`
- `stopped`
- `failed`
- `deleted`

`POST /v1/sandboxes/{sandboxID}/sessions` accepts:

- `type`: required. One of `terminal`, `ide`, `notebook`, `browser`, `command`, or `custom`.
- `client`: optional short client label such as `web-terminal`, `sdk`, or an external tool name.
- `metadata`: optional JSON object for client metadata.

Runtime sessions are audit records for attachments to a sandbox. They record project, sandbox, type, status, client label, user agent, runtime reference, `startedAt`, optional `endedAt`, and metadata. They are not an internal agent identity model. The browser terminal route automatically creates a `terminal` session when the WebSocket is accepted and marks it `ended` or `failed` when the stream closes.

Valid runtime session statuses are:

- `active`
- `ended`
- `failed`

`POST /v1/sandboxes/{sandboxID}/tasks` accepts:

- `command`: required string array. Shell features require an explicit shell, for example `["sh", "-lc", "npm test"]`.
- `timeoutSeconds`: optional, defaults to `60`, maximum `600`.
- `metadata`: optional JSON object for client metadata.

The route creates a queued task and returns immediately. The API server then runs the command asynchronously through runtime access. Clients can poll `GET /v1/tasks/{taskID}` or `GET /v1/sandboxes/{sandboxID}/tasks` until the task reaches a terminal status, or stream `GET /v1/tasks/{taskID}/events` for newline-delimited JSON task events. The route only runs for sandboxes whose mbox status is `running` and whose `runtimeRef` is ready. It captures stdout and stderr separately with a per-stream size cap and returns an `outputTruncated` marker when output was clipped.

Task event stream event types:

- `snapshot`: the current task record when the stream opens.
- `status`: a task status or timing update.
- `output`: an incremental stdout or stderr chunk with `stream`, `data`, and `offset`.
- `done`: the terminal task record.

The stream is process-local for live output. If a client connects after a task is already finished, it receives a snapshot and done event from the persisted task record.

`POST /v1/tasks/{taskID}/cancel` cancels a queued or running task when that task is currently running on the same API server process. Finished tasks return `409`. If a task is still marked active in Postgres but the current server has no in-memory cancel handle for it, the route also returns `409`; a later execution controller should make this restart-safe.

Valid execution task statuses are:

- `queued`
- `running`
- `succeeded`
- `failed`
- `canceled`
- `timed_out`

`POST /v1/sandboxes/{sandboxID}/artifacts` accepts:

- `kind`: required. One of `file`, `directory`, `log`, `report`, `screenshot`, `image`, `link`, or `other`.
- `name`: required display name.
- `uri`: required output reference, for example `workspace:///workspace/reports/test.json`, an HTTPS URL, or an object-store URI.
- `taskId`: optional execution task ID. When present, the task must belong to the same sandbox.
- `contentType`: optional media type.
- `sizeBytes`: optional non-negative byte size.
- `metadata`: optional JSON object for client metadata.

Artifacts are product metadata records with an optional retained-content record. `POST /v1/artifacts/{artifactID}/capture` reads a `workspace://` file from the running sandbox workspace, stores bytes server-side up to the current 8 MiB limit, records content type, size, sha256, source URI, captured time, storage provider, and storage key, and returns the artifact with `retainedContent` metadata. `PUT /v1/artifacts/{artifactID}/content` supports the same retained-content metadata path for client-provided bytes, so an external agent, IDE, or CI client can attach an already-produced report without first writing it into the sandbox. The upload route rejects directory artifacts and content over the 8 MiB limit; it uses the request `Content-Type` header unless the artifact already has `contentType`, and accepts optional `X-Mbox-Artifact-Source-URI` for a client-side source reference.

`GET /v1/artifacts/{artifactID}/content` returns retained bytes first, so content can still be downloaded after the sandbox stops or is deleted. If no retained bytes exist, the route falls back to the running-workspace read path and still rejects directory artifacts, non-workspace references, stopped sandboxes, and paths outside workspace storage. The default retained-content provider is `postgres`; `MBOX_ARTIFACT_CONTENT_BACKEND=filesystem` writes retained bytes under `MBOX_ARTIFACT_CONTENT_DIR`, while `MBOX_ARTIFACT_CONTENT_BACKEND=s3` writes retained bytes to the configured S3-compatible bucket. In both non-Postgres modes, Postgres stores only metadata plus a provider-specific storage key. The API still does not proxy arbitrary external HTTPS or object-store artifact references.

## SDK Client

The TypeScript SDK in `sdk/typescript` wraps the current HTTP API with exported resource types and a `MboxClient`.

The package currently includes:

- API info/version/capability handshake helper
- OpenAPI contract helper
- client-supplied request ID and audit attribution headers
- SDK route contract and OpenAPI alignment helpers
- health, project, template, and sandbox CRUD helpers
- audit event list helpers
- typed `policy.denied` audit metadata and an `isPolicyDeniedAuditEvent()` type guard
- project policy get/set helpers
- project quota policy get/set helpers
- project credential-reference list/create/get/delete helpers
- sandbox lifecycle helpers for start and stop
- runtime target, log, event, and preview-port readers
- runtime session create/list/get/end helpers
- execution task create/list/get/cancel helpers
- `watchExecutionTask(taskId)` for newline-delimited task events
- `waitForTask(taskId)` polling convenience for external clients
- runtime orphan audit helper
- sandbox artifact create/list helpers
- task artifact list, artifact get, retained-content capture/upload, and artifact content helpers
- `MboxAPIError` with HTTP status and response body details

Example:

```ts
import { MboxClient } from "@mbox/sdk"

const mbox = new MboxClient({ baseUrl: "http://127.0.0.1:18080" })

const task = await mbox.createExecutionTask("<sandbox-id>", {
  command: ["sh", "-lc", "pwd && echo ok"],
  timeoutSeconds: 60,
})

const finished = await mbox.waitForTask(task.id)
```

Use `watchExecutionTask(task.id, { onEvent })` when a client needs live stdout/stderr chunks instead of polling final task output.

The SDK exports `SDK_ROUTE_CONTRACT`, `SDK_SCHEMA_CONTRACT`, `checkOpenAPIAlignment`, `assertOpenAPIAlignment`, and `fetchAndAssertOpenAPIAlignment` as a starter route-alignment guard. The guard verifies SDK route-backed helpers against the published OpenAPI path, method, SDK-used query parameter set, route auth metadata, focused request bodies, and focused response shapes. Auth checks cover the bearer security scheme, explicit public operations, private bearer operations, and `401` responses. Request checks cover JSON schema refs and binary upload media types. Response checks cover direct schema refs, list item refs, NDJSON task-event streams, binary responses, and no-content delete routes. It then checks a focused set of SDK-consumed schema required fields and properties. It is not yet a generated client or full request/response schema validator. Usage and audit contracts are no longer entirely loose objects: the SDK and OpenAPI both expose project usage request-total types, known audit action string types, the current `PolicyDeniedAuditMetadata` shape, and `isPolicyDeniedAuditEvent()` so clients can safely render selected denial events without treating all audit metadata as stable. `createMboxClientFromEnv()` mirrors the CLI environment convention for `MBOX_API_URL`, `MBOX_TOKEN`/`MBOX_API_TOKEN`, `MBOX_REQUEST_ID`, and audit labels, but it does not read CLI context files.

The SDK also exposes `checkCompatibility()` and `assertCompatibility()` on `MboxClient`, plus standalone `checkSDKCompatibility()` and `checkClientCompatibility()` helpers. These compare the client API compatibility label with the server's `/v1/info` minimum SDK or CLI API version and can require specific server capabilities, so external agents and scripts can fail fast before a longer run. The helpers implement the API compatibility policy above: same major family, ordered alpha/beta/stable labels, and separate required capability checks. They do not authenticate the caller.

Run `npm run smoke` in `sdk/typescript` before packaging or changing SDK contract helpers. The smoke check builds the package and exercises the exported compatibility helpers, `MboxClient.assertCompatibility()`, and OpenAPI alignment success/failure paths without requiring a live API server.

Run `npm run check:pack` in `sdk/typescript` before publishing a package. It builds the SDK, runs `npm pack --dry-run --json`, and verifies the tarball includes the README, package manifest, compiled JavaScript, and TypeScript declaration files while excluding source-only files. It does not publish anything.

Run `npm run check:pack:consumer` in `sdk/typescript` when changing package exports or publish files. It builds the SDK, creates a real local `npm pack` tarball in a temporary directory, installs it into a minimal private ESM consumer project with `--ignore-scripts`, and verifies that `@mbox/sdk` imports from the installed tarball. It does not publish anything or contact the public npm registry.

Run `npm run verify` as the SDK publish gate. It runs typecheck, the local SDK smoke check, the package dry run, and the package consumer smoke. The SDK package also maps `prepublishOnly` to `npm run verify`, so `npm publish` executes these checks before publishing.

SDK methods map directly to public API resources. Upper-layer agent, CI, deploy, or IDE workflows should live in their own clients and store their own workflow semantics while referencing mbox sandboxes, sessions, tasks, previews, and artifacts.

## CLI Client

The Go CLI in `cmd/mbox` is the first scriptable command surface for the implemented API. It uses `MBOX_API_URL` or `--api-url` to select the API server and accepts `MBOX_TOKEN` or `--token` for the `Authorization: Bearer` header when the server is started with `MBOX_API_TOKEN`. `MBOX_REQUEST_ID` or `--request-id` sends `X-Mbox-Request-ID` so scripts can correlate command output with server logs and audit metadata. It also supports client-side contexts with `--context`, `MBOX_CONTEXT`, `--config`, `MBOX_CONFIG`, and a default `~/.mbox/config.json` file when present. `context set`, `context use`, and `context remove` manage that local JSON file; `context current` and `context list` inspect it without printing token values. Contexts are a local CLI convenience; they do not create server-side projects, identities, or permissions.

Example CLI config:

```json
{
  "currentContext": "local",
  "contexts": {
    "local": {
      "apiUrl": "http://127.0.0.1:18080",
      "tokenEnv": "MBOX_TOKEN",
      "auditActor": "local-operator",
      "auditSource": "mbox-cli"
    }
  }
}
```

Explicit flags such as `--api-url`, `--token`, `--request-id`, `--audit-actor`, and `--audit-source` override values loaded from the selected context or environment.

Current command groups:

- `info` for API version, enabled runtime/artifact capabilities, and CLI/SDK compatibility hints.
- `compat` for an explicit CLI/server API compatibility and capability preflight using `/v1/info`. Use repeated `--require-capability` flags for features a script depends on before it starts creating sandboxes, sessions, tasks, or artifacts.
- `context set|use|remove|current|list` for local CLI context management and inspection. Token values are never printed; outputs only include `hasToken`.
- `openapi` for the machine-readable OpenAPI contract.
- `audit-events` for recent product audit events.
- `runtime resources` for the read-only managed runtime resource inventory.
- `runtime orphans` for the read-only runtime orphan audit.
- `projects`: list, create, get, usage, audit-events, policy, set-policy, quota-policy, set-quota-policy, credentials, add-credential, delete.
- `credentials`: get, delete.
- `templates`: list, get.
- `sandboxes`: list, create, get, start, stop, delete.
- `sessions`: list, create, get, end.
- `tasks`: list, create, get, wait, cancel, watch.
- `artifacts`: list, get, capture, upload, content.
- `logs`, `ports`, and `terminal` for sandbox runtime access.

The CLI should remain API-bound. Do not teach it to bypass mbox product records by writing Postgres directly or operating Kubernetes resources directly.

## Data Model

The first migration creates:

- `projects`
- `environment_templates`
- `sandboxes`
- `schema_migrations`

The third migration adds:

- `execution_tasks`

The fourth migration adds:

- `artifacts`

The fifth migration adds:

- `runtime_sessions`

The sixth migration adds:

- `artifact_contents`

The seventh migration adds:

- `project_policies`

The eighth migration adds:

- `project_credentials`

The ninth migration extends:

- `artifact_contents`: retained artifact content metadata plus provider-specific byte references; the `postgres` provider stores bytes in `content`, while the `filesystem` and `s3` providers store a relative/provider object `storage_key`.

The tenth migration adds:

- `audit_events`: best-effort product audit events for successful API write operations.

The eleventh migration adds:

- `project_quota_policies`: project-level product-record quota guards for active sandbox count and retained artifact bytes.

The twelfth and thirteenth migrations add:

- audit-event attribution and action indexes for the current list filters.

Important constraints:

- UUID primary keys use `pgcrypto` `gen_random_uuid()`.
- `projects.slug` is globally unique.
- Global templates have unique `slug` where `project_id IS NULL`.
- Project templates have unique `(project_id, slug)` where `project_id IS NOT NULL`.
- Active sandboxes have unique `(project_id, slug)` where `deleted_at IS NULL`.
- `sandboxes` are soft-deleted by setting `status = 'deleted'` and `deleted_at = now()`.
- `runtime_sessions` belong to one project and one sandbox, constrain type/status values, and require `ended_at >= started_at` when ended.
- `execution_tasks` belong to one project and one sandbox in the current sandbox-backed task MVP.
- `artifacts` belong to one project and one sandbox, and may reference one execution task from that sandbox.
- `project_policies` are one-to-one with projects and cascade on project deletion.
- `project_quota_policies` are one-to-one with projects, cascade on project deletion, and constrain limits to non-negative values.
- `project_credentials` belong to one project, have unique `(project_id, slug)`, and store only a Kubernetes Secret reference plus target/usage metadata.
- `audit_events` may belong to a project, keep optional resource IDs, and use `ON DELETE SET NULL` for deleted projects so global operators can still inspect recent deletion activity.
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
10. If the selected template has `lifecyclePolicy.ttlSeconds` and the sandbox age exceeds it, mbox soft-deletes the sandbox.
11. If the mbox sandbox is soft-deleted, it deletes the `SandboxClaim` and clears the `runtimeRef`.

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

Runtime access routes are available only when `MBOX_RUNTIME_ACCESS_ENABLED=true`, because the server needs explicit permission to proxy terminal, execution tasks, logs, events, and runtime target reads through its Kubernetes client.

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

Execution tasks reuse the same runtime access path as terminal, but without TTY. The server calls Kubernetes `pods/exec` through the runtime adapter, records command metadata, status, timing, stdout, stderr, exit code when Kubernetes reports one, timeout or cancellation state, and the runtime reference used for the run. This is an execution-platform primitive, not a CI pipeline model: agents, CLI tools, IDEs, and CI systems decide why to run the command.

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

The smoke script verifies runtime projection, terminal-ready Pod startup, ServiceAccount token automount disabled, workspace PVC projection, file persistence across Pod replacement, runtime storage metadata, preview-port metadata, logs, events, runtime session records, task watch events, workspace artifact content, and `SandboxClaim` cleanup.

Optional Postgres integration verification:

```sh
export MBOX_TEST_DATABASE_URL='postgres://mbox:mbox@127.0.0.1:5432/mbox_test?sslmode=disable'
go test ./internal/postgres
```

Do not run the Postgres integration test against an external database unless that database is explicitly intended for test writes.

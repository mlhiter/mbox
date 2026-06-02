# mbox

mbox is a Kubernetes-native execution platform for programmable sandboxes, runtime sessions, previews, artifacts, and policy boundaries.

The product provides a web console and API for creating runnable development sandboxes, configuring environment templates, connecting runtime sessions, exposing previews, collecting execution outputs, and managing the policies that make those workflows safe in a shared Kubernetes cluster.

The project is independent at the product layer. Its core language is environment template, sandbox, runtime session, execution task, preview, artifact, policy, and credential boundary. External agents, IDEs, CI systems, release tools, and human operators are clients of the platform; mbox does not include an agent brain and is not primarily a CI/CD product.

Long term, mbox should have several coordinated technical surfaces:

- server side: Go API server, controllers, `agent-sandbox` integration, and Kubernetes resources
- web app: human-facing operational console
- CLI: scriptable operation for developers, CI, and platform users
- API docs: published product API contract
- SDK package: Node.js or Go package for automation clients

## Product Shape

- People can create and enter sandboxes through terminal, IDE, notebook, browser, or preview endpoints.
- Platform users can define templates for language stacks, tools, startup commands, resources, storage, network access, and lifecycle rules.
- External clients can run controlled sessions and tasks inside Kubernetes execution environments.
- Previews and artifacts make runtime outputs inspectable without exposing direct Kubernetes access.
- CI and deployment systems can be built as upper-layer clients after the lower-level runtime primitives are stable.
- Operators can enforce quota, RBAC, network policy, credential boundaries, and cleanup rules.

## Core Documents

- [PRODUCT.md](PRODUCT.md): product direction, users, scope, and product limits.
- [ARCHITECTURE.md](ARCHITECTURE.md): system layers, runtime design, security boundaries, and Kubernetes integration.
- [ROADMAP.md](ROADMAP.md): staged execution plan from prototype to production platform.
- [AGENTS.md](AGENTS.md): instructions for future coding agents working in this repo.
- [docs/architecture.md](docs/architecture.md): short implementation architecture handoff for the current slice.
- [docs/server-api.md](docs/server-api.md): currently implemented server routes, config, data model, and runtime projection.
- [docs/web-console.md](docs/web-console.md): Vite console structure, local proxy behavior, UI scope, and verification.
- [docs/runbook.md](docs/runbook.md): local startup, verification, runtime smoke, and troubleshooting commands.
- [docs/ia.md](docs/ia.md): current web-console information architecture.
- [docs/references.md](docs/references.md): runtime, Kubernetes, and frontend reference notes.
- [docs/research-agent-sandbox.md](docs/research-agent-sandbox.md): notes about using `kubernetes-sigs/agent-sandbox` as the interactive sandbox runtime substrate.

## Current Status

This repository now contains the first vertical slice: a Go API server backed by Postgres for mbox product records plus a separate Vite web console.

Implemented resources:

- `Project`
- `EnvironmentTemplate`
- `Sandbox`
- TypeScript SDK package under `sdk/typescript` for external automation clients
- Go CLI entrypoint under `cmd/mbox` for scriptable access to the public HTTP API
- Vite console views for listing projects, creating/editing templates, and launching sandboxes
- template library for ready-to-run environments, with user-facing runtime type, use case, entrypoints, resource preset, server-backed validation runs, and advanced image/command/policy fields
- SDK and CLI template-validation conveniences that compose validation-run creation, sandbox readiness waiting, one execution task, and validation decision over existing API primitives without adding a server-side workflow or CI engine
- project-scoped launch policy that can gate sandbox launches by image prefix, runtime identity, and template secret reference names
- project member role registry for user, service-account, and automation principals as RBAC groundwork
- caller/auth handshake that reports anonymous, shared-token, or explicitly trusted reverse-proxy header mode plus whether any project-role route gate is enabled
- disabled-by-default trusted principal header provider for reverse-proxy/test environments, used only by caller visibility and authorization preflight
- project authorization preflight that maps actions to required project roles and can match trusted header principals to project member records
- disabled-by-default project RBAC enforcement starter for `sandbox.launch`, `runtime.operate`, `artifact.write`, `policy.manage`, `credential.manage`, and `member.manage`
- project credential-reference registry for narrow Git, registry, Kubernetes, SSH, or generic secret references without storing credential values
- read-only project usage summaries for product-record counts and declared active/running sandbox request totals across sandboxes, sessions, tasks, artifacts, templates, and credential references
- read-only boundary summaries for templates and sandboxes, covering namespace, runtime identity, launch policy, secret references, project credential references, network policy projection, lifecycle policy projection, runtime access paths, and cleanup behavior
- lifecycle `ttlSeconds` enforcement that automatically soft-deletes expired sandboxes and triggers the normal runtime cleanup path
- runtime orphan audit plus explicitly confirmed one-resource cleanup for mbox-managed runtime resources whose labels no longer match product records
- simplified sandbox launch that asks for project, template, and name while deriving slug, namespace, and ServiceAccount defaults
- sandbox stop/start actions that pause and resume the projected runtime without deleting the product record
- browser terminal, workspace storage, manually declared preview ports, asynchronous execution tasks with cancellation, artifact references with retained workspace-file or client-uploaded content, and lightweight logs/events for ready sandbox runtimes

This slice persists mbox product state in Postgres. When the runtime controller is explicitly enabled, it reconciles `Sandbox` records into `agent-sandbox` `SandboxTemplate` and `SandboxClaim` resources.

## Local Development

Start the full local development stack:

```sh
./scripts/dev.sh
```

This starts local Postgres, the Go API server, and the Vite web console. Open `http://127.0.0.1:5174`.

Runtime access is disabled by default so ordinary local startup does not write Kubernetes resources. To enable the `agent-sandbox` controller, terminal, execution tasks, logs, events, and preview proxy:

```sh
MBOX_KUBE_CONTEXT=kind-agent-sandbox ./scripts/dev.sh --runtime
```

If you already have Postgres running, skip Docker Postgres:

```sh
DATABASE_URL='postgres://mbox:mbox@127.0.0.1:5432/mbox?sslmode=disable' ./scripts/dev.sh --no-docker
```

Start a local Postgres:

```sh
docker run --name mbox-postgres \
  -e POSTGRES_USER=mbox \
  -e POSTGRES_PASSWORD=mbox \
  -e POSTGRES_DB=mbox \
  -p 5432:5432 \
  -d postgres:17
```

Run the API server:

```sh
DATABASE_URL='postgres://mbox:mbox@127.0.0.1:5432/mbox?sslmode=disable' go run ./cmd/mbox-server
```

The server listens on `127.0.0.1:18080` by default. Override it with `MBOX_LISTEN_ADDR`.

Run the Vite web console in a second shell:

```sh
cd web
npm install
npm run dev
```

Open `http://127.0.0.1:5174`. During local development, Vite proxies `/healthz` and `/v1/*` to the API server at `127.0.0.1:18080`. If you override `MBOX_LISTEN_ADDR`, set `MBOX_API_PROXY_TARGET` before starting Vite:

```sh
MBOX_API_PROXY_TARGET=http://127.0.0.1:19080 npm run dev
```

If the API server uses `MBOX_API_TOKEN`, start Vite with the same token in `MBOX_TOKEN` or `MBOX_API_TOKEN`; the dev proxy adds the bearer header server-side and does not expose it to frontend code.

The web dev port defaults to `5174` to avoid colliding with other local Vite projects. Override it with `MBOX_WEB_PORT`.

The runtime controller is disabled by default so local API development does not write to a Kubernetes cluster. Enable it explicitly when you want mbox to reconcile `Sandbox` records into `agent-sandbox` resources:

```sh
export MBOX_RUNTIME_CONTROLLER_ENABLED=true
export MBOX_RUNTIME_ACCESS_ENABLED=true
export MBOX_KUBECONFIG="$HOME/.kube/config"
export MBOX_KUBE_CONTEXT="<context-name>"
go run ./cmd/mbox-server
```

Optional runtime settings:

- `MBOX_RUNTIME_RECONCILE_INTERVAL`: reconcile loop interval, for example `5s`.
- `MBOX_RUNTIME_ACCESS_ENABLED`: enables terminal, task execution, logs, events, and runtime target routes when set to `true`.
- `MBOX_AGENT_SANDBOX_WARM_POOL`: `agent-sandbox` warm pool policy, for example `none` or `default`.

When enabled, mbox ensures the sandbox namespace exists, creates a scoped sandbox ServiceAccount with token automount disabled, creates or updates a `SandboxTemplate`, and creates a `SandboxClaim` in that namespace. The generated pod template uses the configured sandbox ServiceAccount and also disables token automount. If the template has `storageRequest`, mbox projects a `workspace` PVC template and mounts it at the template working directory, defaulting to `/workspace`. The mbox Postgres record remains the product source of truth; Kubernetes resources are the runtime projection.

Stopping a sandbox pauses the projected runtime by scaling the resolved `agent-sandbox` `Sandbox` to zero replicas. This releases the Pod and running processes while keeping the mbox record and runtime reference. Workspace data is preserved only when it lives on the persistent workspace PVC; files written to container-local paths can be lost during stop/start. Starting a stopped sandbox marks it `pending` and scales the runtime back to one replica.

`MBOX_RUNTIME_ACCESS_ENABLED=true` enables `/v1/sandboxes/{id}/terminal`, `/tasks`, `/runtime`, `/logs`, `/events`, and `/ports`. The terminal route is a WebSocket proxy from the browser to Kubernetes `pods/exec`; execution tasks enqueue non-interactive `pods/exec` commands, persist output records, and can be canceled while running on the current API server; declared TCP preview ports are proxied through the mbox API server to the resolved runtime Pod. Artifact reference routes and client content uploads do not require direct runtime access, while workspace content read/capture routes do. Artifact content is retained server-side for small `workspace://` file captures or direct client uploads; external URLs and object-store bytes are still references only. Project credential records store only Secret references and metadata, not secret values, and ordinary sandbox pods still do not receive broad Kubernetes credentials.

Retained artifact bytes default to the `postgres` backend for local compatibility. Set `MBOX_ARTIFACT_CONTENT_BACKEND=filesystem` and optionally `MBOX_ARTIFACT_CONTENT_DIR=/path/to/artifacts` to keep captured bytes out of Postgres while leaving artifact metadata and provider keys in the product database. For remote durability, `MBOX_ARTIFACT_CONTENT_BACKEND=s3` writes retained bytes to an explicitly configured S3-compatible endpoint and bucket while Postgres remains the metadata index.

Project deletion is guarded while sandbox cleanup is pending. Delete sandboxes first, let the reconciler clear their `runtimeRef` after deleting the projected `SandboxClaim`, then delete the project. This keeps Postgres from cascading away the product rows that the controller needs for runtime cleanup.

The runtime orphan audit is read-only and intended for operators:

```sh
go run ./cmd/mbox --api-url http://127.0.0.1:18080 runtime resources
go run ./cmd/mbox --api-url http://127.0.0.1:18080 runtime resources --summary
go run ./cmd/mbox --api-url http://127.0.0.1:18080 runtime resources --summary-table
go run ./cmd/mbox --api-url http://127.0.0.1:18080 runtime resources --summary-table --resolve-project-names
go run ./cmd/mbox --api-url http://127.0.0.1:18080 runtime resources --namespace mbox-smoke-20260529
go run ./cmd/mbox --api-url http://127.0.0.1:18080 runtime resources --project-id <project-id> --kind SandboxClaim
go run ./cmd/mbox --api-url http://127.0.0.1:18080 runtime resources --kind SandboxClaim
go run ./cmd/mbox --api-url http://127.0.0.1:18080 runtime orphans
go run ./cmd/mbox --api-url http://127.0.0.1:18080 runtime orphans --namespace mbox-smoke-20260529
go run ./cmd/mbox --api-url http://127.0.0.1:18080 runtime orphans --project-id <project-id>
go run ./cmd/mbox --api-url http://127.0.0.1:18080 runtime orphans --kind SandboxTemplate
go run ./cmd/mbox --api-url http://127.0.0.1:18080 runtime orphans --summary-table --project-id <project-id>
```

The resource inventory lists mbox-managed `SandboxClaim` and `SandboxTemplate` objects reported by the runtime auditor, with a live summary by kind, namespace, label-derived owner, label-derived project, and observed workload shape. Use `runtime resources --summary` when a script needs only that filtered JSON summary object, or `runtime resources --summary-table` when an operator wants a human-readable total, kind/namespace/project/owner distribution, and workload rollup without the full resource rows. Add `--resolve-project-names` to the summary table when the CLI should make a read-only `/v1/projects` request and render known project IDs as `name (short-id)`; JSON output and runtime API responses stay unchanged. The Web Runtime view also renders `summary.byProject` as clickable project attribution so operators can jump from all resources into the same `projectId` filter. For `SandboxClaim` rows, inventory also includes a best-effort runtime observation: resolved runtime sandbox, selected Pod phase, Pod counts, container readiness, restart count, summed Pod requests/limits, and workspace PVC state when available. The inventory and orphan audit can be scoped by namespace, project owner label, kind, or a combination of those filters. The `--project-id` filter matches mbox-managed runtime resources that carry `mbox.dev/project-id`; it is mainly useful for project-owned `SandboxClaim` rows and should not be read as a full RBAC or quota boundary. `summary.byProject` is computed after the same filters and only counts resources with a non-empty runtime owner project label. The orphan audit compares that inventory with product records and reports resources whose labels no longer line up cleanly, including cleanup-pending soft-deleted sandboxes. Use `runtime orphans --summary-table` when an operator wants the existing read-only orphan audit rendered as reason counts plus resource/project/status/message/evidence rows; default `runtime orphans` output remains JSON for scripts. The workload rollup is not metrics-server utilization, quota, billing, or product-record usage; it is only the current runtime auditor view.

Cleanup is deliberately gated and one-resource-at-a-time:

```sh
go run ./cmd/mbox --api-url http://127.0.0.1:18080 runtime cleanup-orphan \
  --adapter agent-sandbox \
  --kind SandboxClaim \
  --namespace mbox-old \
  --name old-claim \
  --reason missing-sandbox-record \
  --confirm delete-orphan-runtime-resource
```

The server re-runs the audit first and deletes only when the resource is still reported as an orphan with the requested reason.

Run tests:

```sh
go test ./...
go run ./cmd/mbox --help
./scripts/smoke-cli.sh
cd web && npm run build
cd sdk/typescript && npm install && npm run build
```

Postgres integration tests are opt-in because they write to the configured test database:

```sh
export MBOX_TEST_DATABASE_URL='postgres://mbox:mbox@127.0.0.1:5432/mbox_test?sslmode=disable'
go test ./internal/postgres
```

## API Smoke Test

Create a project:

```sh
curl -sS -X POST http://127.0.0.1:18080/v1/projects \
  -H 'content-type: application/json' \
  -d '{
    "name": "Demo Project",
    "repositoryUrl": "https://github.com/example/demo",
    "defaultNamespace": "mbox-demo"
  }'
```

Create a Node.js workspace template:

```sh
curl -sS -X POST http://127.0.0.1:18080/v1/templates \
  -H 'content-type: application/json' \
  -d '{
    "name": "Node.js Workspace",
    "image": "node:22-bookworm-slim",
    "startupCommand": ["sh", "-c", "mkdir -p /workspace && cd /workspace && echo mbox node sandbox ready && tail -f /dev/null"],
    "workingDir": "/workspace",
    "cpuRequest": "250m",
    "memoryRequest": "512Mi",
    "storageRequest": "2Gi",
    "exposedPorts": [
      {"name": "web", "port": 3000, "protocol": "TCP"}
    ],
    "metadata": {
      "runtimeType": "Node.js",
      "useCase": "Web app preview",
      "resourcePreset": "Small",
      "validationStatus": "not_tested"
    }
  }'
```

Create a sandbox by using the returned project and template IDs:

```sh
curl -sS -X POST http://127.0.0.1:18080/v1/sandboxes \
  -H 'content-type: application/json' \
  -d '{
    "projectId": "<project-id>",
    "templateId": "<template-id>",
    "name": "Demo Sandbox"
  }'
```

If a project has `defaultTemplateId`, sandbox creation can use project defaults:

```sh
curl -sS -X PATCH http://127.0.0.1:18080/v1/projects/<project-id> \
  -H 'content-type: application/json' \
  -d '{"defaultTemplateId":"<template-id>"}'

curl -sS -X POST http://127.0.0.1:18080/v1/sandboxes \
  -H 'content-type: application/json' \
  -d '{
    "projectId": "<project-id>",
    "name": "Demo Sandbox"
  }'
```

In these paths, mbox derives the slug from the name when `slug` is omitted, uses the project `defaultNamespace`, and defaults the sandbox ServiceAccount to `mbox-sandbox`.

List resources:

```sh
curl -sS http://127.0.0.1:18080/v1/projects
curl -sS http://127.0.0.1:18080/v1/templates
curl -sS http://127.0.0.1:18080/v1/sandboxes
```

## CLI

The first CLI lives at `cmd/mbox`. It is a thin HTTP client over the same public API used by the web console and SDK. It does not talk directly to Postgres or Kubernetes, and it does not add agent or CI workflow semantics.

Run it locally:

```sh
go run ./cmd/mbox --api-url http://127.0.0.1:18080 health
go run ./cmd/mbox --api-url http://127.0.0.1:18080 info
go run ./cmd/mbox context current
go run ./cmd/mbox projects list
go run ./cmd/mbox projects usage <project-id>
go run ./cmd/mbox sandboxes create --project-id <project-id> --template-id <template-id> --name "Demo Sandbox"
go run ./cmd/mbox tasks create <sandbox-id> --arg sh --arg -lc --arg 'pwd && echo ok'
go run ./cmd/mbox tasks run <sandbox-id> --require-success -- sh -lc 'pwd && echo ok'
go run ./cmd/mbox tasks wait <task-id> --timeout 2m
go run ./cmd/mbox tasks wait <task-id> --timeout 2m --require-success
go run ./cmd/mbox tasks watch <task-id>
go run ./cmd/mbox tasks artifacts <task-id>
go run ./cmd/mbox sessions list <sandbox-id>
go run ./cmd/mbox artifacts upload <artifact-id> --file report.txt --content-type text/plain
```

For a local CLI/API smoke test:

```sh
./scripts/smoke-cli.sh
```

Configuration:

- `MBOX_API_URL`: API base URL, defaults to `http://127.0.0.1:18080`.
- `MBOX_API_TOKEN`: optional server-side shared API token. When set on the API server, private routes require bearer authentication while `/healthz` and `/v1/info` stay public.
- `MBOX_TOKEN`: optional CLI bearer token sent as `Authorization: Bearer <token>`.
- `MBOX_TRUSTED_PRINCIPAL_HEADERS_ENABLED`: optional server-side trusted principal header provider for reverse-proxy or local test environments. Defaults to `false`.
- `MBOX_TRUSTED_PRINCIPAL_HEADER`: principal header name, defaulting to `X-Mbox-Principal` when trusted principal headers are enabled.
- `MBOX_TRUSTED_PRINCIPAL_TYPE_HEADER`: principal type header name, defaulting to `X-Mbox-Principal-Type`; accepted values are `user`, `service_account`, and `automation`.
- `MBOX_PROJECT_RBAC_ENFORCEMENT_ENABLED`: optional route-level project RBAC starter. Defaults to `false`; when `true`, trusted-header callers must match an `owner` or `operator` project member to create sandboxes, launch template validation sandboxes, operate live runtime surfaces, create artifact references, capture workspace artifact content, or upload retained artifact bytes. Project launch/quota policy updates, project credential-reference create/delete routes, and project member create/delete routes require an `owner` project member. In this mode, project creation seeds the trusted caller as owner for the new project. Credential-reference and member list/get routes remain read visibility, and credential reads do not expose secret values.
- `MBOX_REQUEST_ID`: optional request correlation id sent as `X-Mbox-Request-ID`; the server echoes it and records it in audit metadata for write events.
- `MBOX_CONTEXT`: optional context name selected from the CLI config file.
- `MBOX_CONFIG`: optional CLI config file path, defaulting to `~/.mbox/config.json` when that file exists.

CLI contexts are client-side only. They select API URL, token, and audit labels for scripts without changing server state:

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

Create and use a context with:

```sh
go run ./cmd/mbox context set local \
  --api-url http://127.0.0.1:18080 \
  --token-env MBOX_TOKEN \
  --audit-actor local-operator \
  --audit-source mbox-cli \
  --current

go run ./cmd/mbox --context local projects list
go run ./cmd/mbox --context local context current
go run ./cmd/mbox --context local context check --require-capability execution-tasks
go run ./cmd/mbox context list
```

Use `context check` as a script preflight for health, `/v1/info`, and CLI/server compatibility before a longer run. It prints redacted diagnostic JSON and can require capabilities such as `execution-tasks`; it is not a login, whoami, RBAC, or token-validity proof. Use `auth caller` when a script needs the server's current caller boundary: it reports anonymous local mode or accepted shared-token mode plus RBAC trust/enforcement flags. Add `--summary` to `auth caller` or the `caller` shorthand when an operator wants the same read-only boundary as compact text instead of JSON. Use `context use <name>` to switch the default context and `context remove <name>` to delete a local context entry. Prefer `--token-env` for reusable configs so token values stay outside the JSON file.

Use `--request-id <id>` when a script needs to line up a command with API logs and `audit_events.metadata.requestId`. Use `--filter-request-id <id>` on `audit-events` reads to retrieve the matching product audit events, `--operation <operation>` and `--reason <reason>` to narrow typed metadata such as `policy.denied` operation and denial reason values, and RFC3339 `--since` / `--until` windows to inspect a known run interval. Add `--policy-denied-summary` to `audit-events` or `projects audit-events` when an operator wants a grouped, human-readable summary of recurring `policy.denied` operation/reason families over the same existing filters. Request IDs and audit labels are correlation/debugging context, not auth claims or idempotency keys.

Run a controlled task after the sandbox reaches `running` with runtime access enabled:

```sh
curl -sS -X POST http://127.0.0.1:18080/v1/sandboxes/<sandbox-id>/tasks \
  -H 'content-type: application/json' \
  -d '{
    "command": ["sh", "-lc", "pwd && ls -la"],
    "timeoutSeconds": 60
  }'

curl -sS http://127.0.0.1:18080/v1/sandboxes/<sandbox-id>/tasks

curl -sS -X POST http://127.0.0.1:18080/v1/tasks/<task-id>/cancel

curl -sS -X POST http://127.0.0.1:18080/v1/sandboxes/<sandbox-id>/artifacts \
  -H 'content-type: application/json' \
  -d '{
    "taskId": "<task-id>",
    "kind": "report",
    "name": "Test report",
    "uri": "workspace:///workspace/reports/test.json",
    "contentType": "application/json"
  }'
```

## TypeScript SDK

The first SDK package lives in `sdk/typescript`. It is a thin client for the public HTTP API, aimed at agents, IDE integrations, CI scripts, release tools, and other external callers. It does not run an agent loop inside mbox; it only makes the platform primitives easier to call.

Build it locally:

```sh
cd sdk/typescript
npm install
npm run build
npm run smoke
```

Run a task and register a referenced artifact:

```ts
import { MboxClient, createMboxClientFromEnv } from "@mbox/sdk"

const mbox = new MboxClient({ baseUrl: "http://127.0.0.1:18080" })
const envMbox = createMboxClientFromEnv()

const info = await mbox.info()
console.log(info.apiVersion, info.capabilities)

const caller = await mbox.caller()
console.log(caller.mode, caller.rbacTrusted, caller.projectRolesEnforced)

await mbox.assertCompatibility({
  requiredCapabilities: ["execution-tasks", "task-events", "artifact-client-upload"],
})

const task = await mbox.createExecutionTask("<sandbox-id>", {
  command: ["sh", "-lc", "npm test -- --reporter=json > /workspace/reports/test.json"],
  timeoutSeconds: 300,
  metadata: { caller: "external-agent" },
})

const finished = await mbox.waitForTask(task.id, { requireSuccess: true })

const validation = await mbox.runTemplateValidation("<template-id>", {
  projectId: "<project-id>",
  validationMetadata: { caller: "external-agent" },
  task: {
    command: ["sh", "-lc", "pwd && echo template-ok"],
    timeoutSeconds: 60,
  },
  timeoutMs: 300_000,
  requireSuccess: true,
})
console.log(validation.status, validation.task?.status)

await mbox.createArtifact("<sandbox-id>", {
  taskId: finished.id,
  kind: "report",
  name: "Test report",
  uri: "workspace:///workspace/reports/test.json",
  contentType: "application/json",
})

await mbox.captureArtifactContent("<artifact-id>")
await mbox.uploadArtifactContent("<artifact-id>", new Blob(["client report"], { type: "text/plain" }), {
  headers: { "content-type": "text/plain" },
})
```

When the API server is started with `MBOX_API_TOKEN`, pass the matching client token:

```ts
const mbox = new MboxClient({
  baseUrl: "http://127.0.0.1:18080",
  token: process.env.MBOX_TOKEN,
})
```

For scripts that follow the CLI environment convention, `createMboxClientFromEnv()` reads `MBOX_API_URL`, `MBOX_TOKEN` or `MBOX_API_TOKEN`, `MBOX_REQUEST_ID`, `MBOX_AUDIT_ACTOR`, and `MBOX_AUDIT_SOURCE`. It does not read CLI context files.

Retained artifact metadata includes `retainedContent.storageProvider`, currently `postgres`, `filesystem`, or `s3`, so clients can tell where retained bytes are backed without changing the content download route. `captureArtifactContent` reads from a running sandbox workspace, while `uploadArtifactContent` attaches client-provided bytes directly to the artifact.

For a disposable local API database, `npm run smoke:api` exercises the SDK against a live server without enabling runtime/Kubernetes access. It creates and cleans up a temporary project, template, sandbox, session record, and client-uploaded artifact.

For a running runtime-enabled API and sandbox, `npm run smoke:runtime` exercises SDK task execution, successful task waiting, task-scoped artifact listing, workspace content capture, and retained content reads. `scripts/smoke-agent-sandbox.sh` calls this SDK runtime smoke after it launches a sandbox, and uses `mbox sandboxes wait --require-runtime-ref` before runtime actions, so the cluster smoke covers API, CLI, and SDK runtime paths without making the SDK create or clean up Kubernetes resources directly.

For scriptable template checks, `mbox templates validate-run <template-id> --project-id <project-id> -- sh -lc '...'` creates a validation sandbox, waits for runtime readiness, runs one task, and writes a passed/failed validation decision. It is a CLI convenience over existing mbox primitives, not a server-side pipeline engine.

The SDK also exports `caller()` for the same read-only caller/auth handshake as `GET /v1/auth/caller`. It reports anonymous, shared-token, or explicitly enabled `trusted_header` mode, whether authentication is required, and explicit `rbacTrusted` / `projectRolesEnforced` booleans. The CLI `auth caller --summary` and `caller --summary` commands render that same boundary as compact text for operator checks. `getProjectAuthorization(projectId, { action })` returns the current project authorization preflight for actions such as `sandbox.launch`, `runtime.operate`, `artifact.write`, `policy.manage`, `credential.manage`, and `member.manage`, including required roles, caller boundary, member count, matched member when a trusted header principal maps to a project member record, and whether that action is route-enforced. The CLI equivalent is `mbox projects authorization <project-id> --action <action>`; add `--summary` when an operator wants the same read-only preflight rendered as a compact decision summary instead of JSON. Trusted headers and project RBAC enforcement are disabled by default; when both are explicitly enabled, `sandbox.launch` gates sandbox creation and template validation launches, `runtime.operate` gates active runtime target/log/event/preview/terminal/session/task/workspace-content paths, `artifact.write` gates artifact reference creation, workspace artifact capture, and client artifact-content upload on an owner/operator project member match, while `policy.manage`, `credential.manage`, and `member.manage` gate project launch/quota policy updates, credential-reference create/delete routes, and member create/delete routes on an owner project member match. In that mode, project creation seeds the trusted caller as owner for the new project. Ordinary product-record reads such as sandbox/session/task/artifact/credential/member list/get routes remain outside this starter gate. The SDK also exports project member helpers for registering user, service-account, or automation principals with `owner`, `operator`, or `viewer` roles. These records are starter RBAC records for project authorization enforcement, but the current shared-token API does not turn tokens or audit labels into user identity.

The SDK also exports the current audit-event action types, `PolicyDeniedAuditMetadata`, and `isPolicyDeniedAuditEvent()` for safely rendering selected policy/quota denial audit events. Audit metadata includes `requestId` when a request writes an event, plus typed policy/quota hints such as `policyKind`, `enforcement`, `maxActiveSandboxes`, and `maxRetainedArtifactBytes` when the denial path provides them. Audit list helpers accept `requestId`, `operation`, `reason`, `since`, and `until` to filter feeds for one request, script run, typed denial operation/reason, or known time window. The Web project inspector highlights `policy.denied` rows and shows available denial operation, reason, authorization action, caller boundary, and resource hints for operator debugging. The Web Members group can also register or remove project member records for starter RBAC while guarding against removing the last visible owner; those records are not login sessions or trusted identities by themselves. Audit events are still best-effort product records, not authentication claims or a strong transactional audit log.

## Runtime Smoke Test

With the server running and both `MBOX_RUNTIME_CONTROLLER_ENABLED=true` and `MBOX_RUNTIME_ACCESS_ENABLED=true`, run the cluster smoke test against a kubeconfig context that already has `agent-sandbox` installed:

```sh
export MBOX_API_URL=http://127.0.0.1:18080
export MBOX_KUBECONFIG="$HOME/.kube/config"
export MBOX_KUBE_CONTEXT=kind-agent-sandbox
./scripts/smoke-agent-sandbox.sh
```

The smoke test creates a project, a BusyBox terminal template, and a sandbox through the mbox API. It then verifies the generated `SandboxClaim`, resolved `Sandbox`, ready Pod, disabled ServiceAccount token automount, pod logs, workspace exec, API status mapping, task output capture, CLI `tasks run --require-success`, retained artifact content, delete cleanup path, and a clean runtime orphan audit after cleanup.

## Node Preview Smoke

With runtime mode enabled, launch a sandbox from the Node.js workspace template and wait for it to reach `running`. The Runtime Workspace shows a starting panel while the `SandboxClaim` and Pod are still pending, then enables Terminal, Logs, Events, Storage, Preview, and Tasks.

In the terminal tab, start a service in the background:

```sh
cat > server.js <<'EOF'
const http = require('http')
http.createServer((req, res) => {
  res.end('hello from mbox node preview')
}).listen(3000, '0.0.0.0')
EOF

node server.js > server.log 2>&1 &
```

Open the Preview tab, make sure port `3000` is declared as `web`, and use `Open` after the sandbox is running. If the template did not declare the port, add it in the Preview tab; this saves the sandbox `ports` field through `PATCH /v1/sandboxes/{id}`.

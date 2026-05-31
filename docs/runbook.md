# Runbook

This runbook covers local development, verification, and common failures for the current mbox slice.

## One-Command Local Stack

Start Postgres in Docker, the Go API server, and the Vite console:

```sh
./scripts/dev.sh
```

Default endpoints:

- API: `http://127.0.0.1:18080`
- Web: `http://127.0.0.1:5174`
- DB: `postgres://mbox:mbox@127.0.0.1:5432/mbox?sslmode=disable`

`scripts/dev.sh` leaves the Postgres container running when stopped. It stops only the API and web dev server.

## Runtime Mode

Use runtime mode when you want the full local flow through `agent-sandbox`:

```sh
MBOX_KUBE_CONTEXT=kind-agent-sandbox ./scripts/dev.sh --runtime
```

Runtime mode sets:

- `MBOX_RUNTIME_CONTROLLER_ENABLED=true`
- `MBOX_RUNTIME_ACCESS_ENABLED=true`
- `MBOX_KUBECONFIG=${MBOX_KUBECONFIG:-$HOME/.kube/config}`
- `MBOX_KUBE_CONTEXT=${MBOX_KUBE_CONTEXT:-kind-agent-sandbox}`

Only use runtime mode against a cluster where `agent-sandbox` is installed and test resource creation is acceptable.

## Reusing an Existing Postgres

Skip Docker Postgres and use an explicit database URL:

```sh
DATABASE_URL='postgres://mbox:mbox@127.0.0.1:5432/mbox?sslmode=disable' ./scripts/dev.sh --no-docker
```

If local port `5432` is already used by another container, either point `DATABASE_URL` at the existing mbox database or choose another host port:

```sh
MBOX_POSTGRES_PORT=15432 ./scripts/dev.sh
```

When reusing a previous Docker Postgres container, do not assume its host port stayed at `5432`. Recover the live port before using `--no-docker`:

```sh
docker start mbox-postgres-dev
docker port mbox-postgres-dev 5432/tcp
pg_isready -h 127.0.0.1 -p <host-port> -U mbox -d mbox
DATABASE_URL='postgres://mbox:mbox@127.0.0.1:<host-port>/mbox?sslmode=disable' ./scripts/dev.sh --runtime --no-docker
```

To verify retained artifact bytes outside Postgres during runtime smoke, start the same stack with the filesystem content backend:

```sh
DATABASE_URL='postgres://mbox:mbox@127.0.0.1:<host-port>/mbox?sslmode=disable' \
MBOX_ARTIFACT_CONTENT_BACKEND=filesystem \
MBOX_ARTIFACT_CONTENT_DIR=/tmp/mbox-artifacts \
./scripts/dev.sh --runtime --no-docker
```

For S3-compatible retained bytes, configure the server-side backend explicitly. This keeps Postgres as the artifact metadata index while object bytes are written to the configured bucket:

```sh
DATABASE_URL='postgres://mbox:mbox@127.0.0.1:<host-port>/mbox?sslmode=disable' \
MBOX_ARTIFACT_CONTENT_BACKEND=s3 \
MBOX_ARTIFACT_CONTENT_S3_ENDPOINT=http://127.0.0.1:9000 \
MBOX_ARTIFACT_CONTENT_S3_BUCKET=mbox-artifacts \
MBOX_ARTIFACT_CONTENT_S3_ACCESS_KEY_ID=minioadmin \
MBOX_ARTIFACT_CONTENT_S3_SECRET_ACCESS_KEY=minioadmin \
./scripts/dev.sh --runtime --no-docker
```

If startup fails with `dial tcp 127.0.0.1:<port>: connect: connection refused`, the API never reached migrations; the configured `DATABASE_URL` points at a port with no live Postgres listener.

## Manual Startup

API only:

```sh
DATABASE_URL='postgres://mbox:mbox@127.0.0.1:5432/mbox?sslmode=disable' go run ./cmd/mbox-server
```

Web only:

```sh
cd web
npm install
npm run dev
```

If the API runs on a non-default port:

```sh
cd web
MBOX_API_PROXY_TARGET=http://127.0.0.1:19080 npm run dev
```

If the API is started with `MBOX_API_TOKEN`, start Vite with `MBOX_TOKEN` or the same `MBOX_API_TOKEN` so the dev proxy can attach the bearer header server-side:

```sh
cd web
MBOX_TOKEN=local-token npm run dev
```

If the default web port is busy:

```sh
cd web
MBOX_WEB_PORT=5175 npm run dev
```

## Verification

Default verification:

```sh
go test ./...
go run ./cmd/mbox --help
./scripts/smoke-cli.sh
cd web && npm run build
cd sdk/typescript && npm run build
git diff --check
```

For frontend template-library changes, also run a browser check against the Templates view. At minimum verify:

- the table shows Environment, Use case, Entrypoints, Preset, and Status
- creating a template defaults to the Node.js web-app environment
- invalid entrypoint text such as `web:abc` blocks save
- editing CPU or memory to a non-preset value saves `metadata.resourcePreset` as `Custom`

Postgres integration tests are opt-in and write to the configured test database:

```sh
export MBOX_TEST_DATABASE_URL='postgres://mbox:mbox@127.0.0.1:5432/mbox_test?sslmode=disable'
go test ./internal/postgres
```

Runtime smoke test against the dedicated local target:

```sh
export MBOX_API_URL=http://127.0.0.1:18080
export MBOX_KUBECONFIG="$HOME/.kube/config"
export MBOX_KUBE_CONTEXT=kind-agent-sandbox
./scripts/smoke-agent-sandbox.sh
```

The smoke test creates and deletes Kubernetes runtime resources. It verifies the API info capability manifest, project usage summary, product audit-event feed through CLI smoke, runtime `SandboxClaim`, Pod readiness, ServiceAccount token automount, workspace PVC mount, file persistence across Pod replacement, runtime storage metadata, template/sandbox boundary summaries, preview-port metadata, logs, events, runtime session records, task watch events, workspace artifact content, runtime cleanup, and the runtime orphan audit returning clean after cleanup. Do not run it against a cluster unless that context is explicitly intended for mbox runtime testing.

Project deletion has a cleanup guard. If `DELETE /v1/projects/{projectID}` returns `409`, list the project's sandboxes, delete them first, and wait until the runtime reconciler has cleared their `runtimeRef`. The runtime smoke script covers this path by waiting for `SandboxClaim` deletion and API-side runtime cleanup before deleting the project.

Use the orphan audit when runtime labels and product records may have drifted:

```sh
go run ./cmd/mbox runtime resources
go run ./cmd/mbox runtime resources --namespace mbox-smoke-20260529
go run ./cmd/mbox runtime resources --namespace mbox-smoke-20260529 --kind SandboxClaim
go run ./cmd/mbox runtime orphans
go run ./cmd/mbox runtime orphans --namespace mbox-smoke-20260529
go run ./cmd/mbox runtime orphans --kind SandboxTemplate
curl -fsS http://127.0.0.1:18080/v1/runtime/orphans | jq
```

The runtime inventory reports the current auditor view plus `summary.total`, `summary.byKind`, `summary.byNamespace`, and `summary.byOwner` for quick operator triage. Owner entries are derived from existing `mbox.dev/project-id`, `mbox.dev/sandbox-id`, and `mbox.dev/template-id` labels; they are not live metrics or capacity accounting. The orphan audit reports `missing-sandbox-record`, `cleanup-pending`, `runtime-ref-mismatch`, `missing-template-record`, and `unlabeled-owner`. Use `--namespace` / `?namespace=` and `--kind` / `?kind=` when a shared test cluster has older mbox-managed resources from prior runs.

If an operator decides to remove a reported orphan, use the explicitly gated cleanup command. It deletes only one currently reported orphan runtime resource and requires the current reason plus the confirmation string:

```sh
go run ./cmd/mbox runtime cleanup-orphan \
  --adapter agent-sandbox \
  --kind SandboxClaim \
  --namespace mbox-old \
  --name old-claim \
  --reason missing-sandbox-record \
  --confirm delete-orphan-runtime-resource
```

Do not use orphan cleanup as a substitute for the normal reconciler path. For active or soft-deleted product records, delete the mbox sandbox and let the reconciler clear `runtimeRef`.

## CLI Check

Use this after changing public API routes or the CLI surface:

```sh
go run ./cmd/mbox --help
./scripts/smoke-cli.sh
go run ./cmd/mbox --api-url http://127.0.0.1:18080 health
go run ./cmd/mbox --api-url http://127.0.0.1:18080 openapi | jq '.openapi, .info.title'
go run ./cmd/mbox projects list
go run ./cmd/mbox projects usage <project-id>
go run ./cmd/mbox projects audit-events <project-id> --action policy.denied --operation sandbox.launch --actor cli-smoke --source mbox-cli --filter-request-id cli-smoke-request --since 2026-05-30T00:00:00Z --until 2026-05-30T01:00:00Z
go run ./cmd/mbox projects quota-policy <project-id>
go run ./cmd/mbox projects set-quota-policy <project-id> --enforcement enforced --max-active-sandboxes 5 --max-retained-artifact-bytes 1048576
go run ./cmd/mbox audit-events --project-id <project-id> --action sandbox.created --actor cli-smoke --source mbox-cli --filter-request-id cli-smoke-request --limit 20
go run ./cmd/mbox sandboxes list
```

Every API response includes `X-Mbox-Request-ID`. Pass `--request-id <id>` or set `MBOX_REQUEST_ID` when a script needs to correlate command output with server logs and `audit_events.metadata.requestId`. Use `--filter-request-id <id>` on `audit-events` or `projects audit-events` when reading the feed back for one script or agent run. Use `--operation <operation>` when narrowing typed metadata, especially `policy.denied` operations such as `sandbox.launch` and `artifact.content.upload`. Use inclusive RFC3339 `--since` / `--until` windows when operators need to inspect a known run interval.

For the starter API token model, start the API with `MBOX_API_TOKEN` and pass the same value as `MBOX_TOKEN` or `--token`:

```sh
MBOX_API_TOKEN=local-token DATABASE_URL="$DATABASE_URL" go run ./cmd/mbox-server
go run ./cmd/mbox --api-url http://127.0.0.1:18080 info
MBOX_TOKEN=local-token go run ./cmd/mbox --api-url http://127.0.0.1:18080 projects list
```

`/healthz` and `/v1/info` remain public for discovery. Other routes return `401` without a matching bearer token. This is a shared automation token, not a user identity or RBAC model.

For repeated CLI use, create a client-side context file:

```sh
go run ./cmd/mbox context set local \
  --api-url http://127.0.0.1:18080 \
  --token-env MBOX_TOKEN \
  --audit-actor local-operator \
  --audit-source mbox-cli \
  --current

MBOX_TOKEN=local-token go run ./cmd/mbox --context local projects list
MBOX_TOKEN=local-token go run ./cmd/mbox --context local context current
go run ./cmd/mbox context list
go run ./cmd/mbox context use local
```

Use `--config <path>` or `MBOX_CONFIG` for a repo-local or CI-provided config file. `context set` creates parent directories and writes `0600` JSON. Prefer `--token-env` for reusable configs so token values stay outside the file; `context current` and `context list` only print `hasToken`. Explicit flags override selected context values.

Use `--audit-actor` and `--audit-source`, or `MBOX_AUDIT_ACTOR` and `MBOX_AUDIT_SOURCE`, when a script wants successful write events and selected `policy.denied` events to carry client-supplied attribution:

```sh
go run ./cmd/mbox \
  --api-url http://127.0.0.1:18080 \
  --request-id agent-run-20260530-001 \
  --audit-actor local-operator \
  --audit-source mbox-cli \
  sandboxes create --project-id <project-id> --template-id <template-id> --name audit-demo
```

These labels are visible in audit events but are not authentication or authorization claims. Request IDs are correlation labels only; they are not idempotency keys or trusted identity. Use `--action` when narrowing feeds to a known event type such as `sandbox.created` or `policy.denied`, `--operation` when narrowing action-specific metadata, `--filter-request-id` when narrowing to one request or scripted run, and `--since` / `--until` when narrowing to a time window. Current denial audit coverage is intentionally narrow: project launch-policy denials, active sandbox quota denials, and retained artifact byte quota denials record `action: "policy.denied"` with `operation`, `reason`, and request correlation metadata when available. The published OpenAPI schema and TypeScript SDK expose this metadata shape for the current operations: `sandbox.launch`, `template.validation`, `artifact.content.capture`, and `artifact.content.upload`.

For runtime-enabled sandboxes, the CLI maps to the same lower-level primitives as the API and SDK:

```sh
go run ./cmd/mbox runtime resources --namespace <namespace>
go run ./cmd/mbox templates validate <template-id> --project-id <project-id>
go run ./cmd/mbox templates decide-validation <template-id> <validation-sandbox-id> --status passed
go run ./cmd/mbox sessions list <sandbox-id>
go run ./cmd/mbox tasks create <sandbox-id> --arg sh --arg -lc --arg 'pwd && echo task-ok'
go run ./cmd/mbox tasks wait <task-id> --timeout 2m
go run ./cmd/mbox tasks watch <task-id>
go run ./cmd/mbox artifacts content <artifact-id>
```

The CLI should remain a thin HTTP client. It must not write to Postgres directly or operate Kubernetes resources directly.

`scripts/smoke-cli.sh` starts a local API server when needed, exercises the OpenAPI contract route, runtime-auditor disabled errors, project, launch policy, quota policy, project credential-reference, audit-event reads, policy denial, quota denial, template validation, sandbox, artifact upload, and session commands through the CLI, then deletes the created records. It expects a reachable Postgres from `DATABASE_URL`; on this development machine the reusable default is `postgres://mbox:mbox@127.0.0.1:32768/mbox?sslmode=disable`.

## Node.js Preview Smoke

Use this when checking the console flow that users expect from a fresh sandbox.

1. Start the stack in runtime mode:

```sh
MBOX_KUBE_CONTEXT=kind-agent-sandbox ./scripts/dev.sh --runtime
```

2. In the web console, create or select a project, create a template using the default `Node.js Workspace` values, and launch a sandbox with only Project, Template, and Name.

3. The Runtime Workspace may show `Starting runtime` while the sandbox is `pending`. Wait for the sandbox to become `running`; the workspace polls the sandbox record while it is starting.

4. In the Terminal tab, start a Node service in the background:

```sh
cat > server.js <<'EOF'
const http = require('http')
http.createServer((req, res) => {
  res.end('hello from mbox node preview')
}).listen(3000, '0.0.0.0')
EOF

node server.js > server.log 2>&1 &
```

5. In the Preview tab, add `web` port `3000` if it is not already declared. The Preview tab saves sandbox ports through `PATCH /v1/sandboxes/{id}`. Use `Open` after the sandbox is running.

## Execution Task Check

Use this after changing task APIs, runtime access, or the Runtime Workspace Tasks tab.

1. Start runtime mode and wait for a sandbox to reach `running`.

```sh
MBOX_KUBE_CONTEXT=kind-agent-sandbox ./scripts/dev.sh --runtime
```

2. Run a simple task through the API:

```sh
SANDBOX_ID='<sandbox-id>'
curl -fsS -X POST "http://127.0.0.1:18080/v1/sandboxes/$SANDBOX_ID/tasks" \
  -H 'content-type: application/json' \
  -d '{"command":["sh","-lc","pwd && echo task-ok"],"timeoutSeconds":60}'
```

Expected result:

- API returns `201 Created`.
- Task status initially returns as `queued`.
- Polling eventually shows `succeeded`, or the watch endpoint streams a terminal `done` event.
- `stdout` includes the workspace path and `task-ok`.
- `exitCode` is `0`.

The CLI polling helper mirrors the SDK `waitForTask()` convenience:

```sh
go run ./cmd/mbox tasks wait "$TASK_ID" --interval 500ms --timeout 2m
```

It prints the final task JSON when status reaches `succeeded`, `failed`, `canceled`, or `timed_out`.

3. Verify the task history:

```sh
curl -fsS "http://127.0.0.1:18080/v1/sandboxes/$SANDBOX_ID/tasks"
```

Stream live task events:

```sh
TASK_ID='<task-id>'
curl -fsS "http://127.0.0.1:18080/v1/tasks/$TASK_ID/events"
```

The stream is newline-delimited JSON. Expect a `snapshot` event first, optional `status` and `output` events while the task runs, and a final `done` event.

For a failure path, run `{"command":["sh","-lc","exit 7"]}` and expect polling to show `status` as `failed` with exit code `7` when Kubernetes reports it. For timeout behavior, run a command that exceeds `timeoutSeconds` and expect `timed_out`. For cancellation, start a long command, capture the returned task ID, then run:

```sh
curl -fsS -X POST "http://127.0.0.1:18080/v1/tasks/$TASK_ID/cancel"
```

Poll task history, or use `tasks wait`, until it reports `canceled`.

4. Register an artifact reference for a task output:

```sh
TASK_ID='<task-id>'
curl -fsS -X POST "http://127.0.0.1:18080/v1/sandboxes/$SANDBOX_ID/artifacts" \
  -H 'content-type: application/json' \
  -d '{"taskId":"'"$TASK_ID"'","kind":"report","name":"Task report","uri":"workspace:///workspace/reports/task.json","contentType":"application/json"}'
```

Verify the sandbox and task artifact lists:

```sh
curl -fsS "http://127.0.0.1:18080/v1/sandboxes/$SANDBOX_ID/artifacts"
curl -fsS "http://127.0.0.1:18080/v1/tasks/$TASK_ID/artifacts"
```

Capture and read content for a supported workspace file artifact while the sandbox is running:

```sh
ARTIFACT_ID='<artifact-id>'
curl -fsS -X POST "http://127.0.0.1:18080/v1/artifacts/$ARTIFACT_ID/capture"
curl -fsS "http://127.0.0.1:18080/v1/artifacts/$ARTIFACT_ID/content"
```

Upload and read client-provided retained bytes for an artifact that is not backed by a workspace file:

```sh
curl -fsS -X PUT "http://127.0.0.1:18080/v1/artifacts/$ARTIFACT_ID/content" \
  -H 'content-type: text/plain' \
  --data-binary @report.txt

go run ./cmd/mbox artifacts upload "$ARTIFACT_ID" --file report.txt --content-type text/plain
go run ./cmd/mbox artifacts content "$ARTIFACT_ID"
```

The capture API reads `workspace://` file references only from the resolved workspace mount and retains bytes server-side up to the current 8 MiB limit. The upload API stores client-provided bytes through the same retained-content backend and rejects directory artifacts or content over the same limit. Content reads return retained bytes first, then fall back to the running workspace path. Retained responses include `retainedContent.storageProvider`, which is `postgres` by default, `filesystem` when `MBOX_ARTIFACT_CONTENT_BACKEND=filesystem` is active, and `s3` when the S3-compatible backend is active. The API does not proxy arbitrary HTTPS/object-store references.

## Boundary Summary Check

Use this after changing template security fields, sandbox launch defaults, runtime projection, or the Boundary tab.

```sh
TEMPLATE_ID='<template-id>'
PROJECT_ID='<project-id>'
SANDBOX_ID='<sandbox-id>'

curl -fsS "http://127.0.0.1:18080/v1/templates/$TEMPLATE_ID/boundary?projectId=$PROJECT_ID"
curl -fsS "http://127.0.0.1:18080/v1/sandboxes/$SANDBOX_ID/boundary"

go run ./cmd/mbox templates boundary "$TEMPLATE_ID" --project-id "$PROJECT_ID"
go run ./cmd/mbox sandboxes boundary "$SANDBOX_ID"
go run ./cmd/mbox projects add-credential "$PROJECT_ID" --name "GitHub App" --type git --target "https://github.com/mlhiter/mbox" --secret-ref github-app-token --secret-key token --usage clone
go run ./cmd/mbox projects credentials "$PROJECT_ID"
```

Expected result:

- `serviceAccountTokenAutomount` is `false`.
- The template boundary resolves the project namespace when `projectId` is provided.
- The sandbox boundary includes the sandbox ID, namespace, ServiceAccount, and runtime reference when the sandbox has been projected.
- Project launch policy state appears in template and sandbox boundary summaries. Enforced policy gates sandbox creation and template validation launches by image prefix, ServiceAccount, and declared secret reference names.
- Project quota policy is visible through API/CLI/SDK/Web project surfaces. Enforced quota currently gates active sandbox creation and retained artifact-byte capture/upload from product records; it is not live Kubernetes capacity, billing, or reservation.
- Project credential references appear in project inspectors and boundary summaries by type, target, usage, and Kubernetes Secret name/key only. They are not secret values and are not mounted into runtime Pods yet.
- Secret references are reported as references, not values.
- Custom network policy is reported as recorded but not yet custom-projected.
- Template `lifecyclePolicy.ttlSeconds` is reported as `ttl-enforced`; idle cleanup policy is not implemented yet.

## TypeScript SDK Check

Use this after changing public API shapes or SDK wrappers.

```sh
cd sdk/typescript
npm install
npm run build
npm run check:openapi -- http://127.0.0.1:18080
```

`check:openapi` builds the SDK, fetches `/v1/openapi.json` from the API base URL, and verifies every route-backed SDK helper in `SDK_ROUTE_CONTRACT` has a matching OpenAPI path, method, route auth metadata, focused request shape, and focused response shape. It sends `MBOX_TOKEN` or `MBOX_API_TOKEN` as a bearer token when either is set, and it can also accept a saved OpenAPI JSON file path. When usage, audit, request correlation, or auth contracts change, also check that OpenAPI schemas such as `ProjectUsage`, `SandboxResourceRequestUsage`, `AuditEventAction`, `PolicyDeniedAuditMetadata`, and the `bearerAuth` security scheme still match the exported SDK contract.

The SDK is a thin external-client package. It should stay aligned with the server routes in `docs/server-api.md` and should not introduce mbox-internal agent or CI workflow semantics. A basic external caller shape is:

```ts
import { MboxClient } from "@mbox/sdk"

const mbox = new MboxClient({
  baseUrl: "http://127.0.0.1:18080",
  auditActor: "agent-runner",
  auditSource: "sdk",
})
const boundary = await mbox.getSandboxBoundary("<sandbox-id>")
const task = await mbox.createExecutionTask("<sandbox-id>", {
  command: ["sh", "-lc", "pwd && echo task-ok"],
})
const finished = await mbox.waitForTask(task.id)
```

## Sandbox Stop/Start Check

Use this after changing lifecycle routes, controller reconciliation, or the sandbox row actions.

1. Start runtime mode and launch a sandbox that reaches `running`.

```sh
MBOX_KUBE_CONTEXT=kind-agent-sandbox ./scripts/dev.sh --runtime
```

2. Record the sandbox ID and stop it through the API:

```sh
SANDBOX_ID='<sandbox-id>'
curl -fsS -X POST "http://127.0.0.1:18080/v1/sandboxes/$SANDBOX_ID/stop"
```

Expected result:

- API returns the sandbox with `status` set to `stopped`.
- The mbox record keeps its `runtimeRef`.
- The controller scales the resolved `agent-sandbox` `Sandbox.spec.replicas` to `0`.
- The web row shows a start action instead of a stop action.

3. Start it again:

```sh
curl -fsS -X POST "http://127.0.0.1:18080/v1/sandboxes/$SANDBOX_ID/start"
```

Expected result:

- API returns the sandbox with `status` set to `pending`.
- The controller scales the existing runtime `Sandbox.spec.replicas` back to `1`.
- The sandbox eventually returns to `running`.

Stop/start preserves only data that is on the persistent workspace PVC. Processes and files written to container-local paths can disappear when the Pod is removed and recreated.

## Troubleshooting

### Docker Postgres Fails on Port 5432

Symptom:

```text
Bind for 127.0.0.1:5432 failed: port is already allocated
```

Check the owner:

```sh
docker ps --format 'table {{.Names}}\t{{.Ports}}'
lsof -nP -iTCP:5432 -sTCP:LISTEN
```

Use a different mbox Postgres port:

```sh
MBOX_POSTGRES_PORT=15432 ./scripts/dev.sh
```

Or point `DATABASE_URL` at an existing mbox database and use `--no-docker`.

### Web Terminal Does Not Connect

Check:

- sandbox status is `running`
- sandbox has a `runtimeRef`
- API was started with `MBOX_RUNTIME_ACCESS_ENABLED=true`
- Vite proxy has `ws: true` for `/v1`
- the server can resolve the runtime Pod through the configured kube context

For a newly launched sandbox, `pending` is expected. The console should show `Starting runtime` and poll status instead of trying to connect the terminal before the `runtimeRef` exists.

### Preview Port Does Not Open

Check:

- the sandbox declares the port in its `ports` field
- if the template did not declare the port in `exposedPorts`, add the TCP port in the Preview tab
- protocol is TCP
- sandbox status is `running`
- runtime access is enabled
- a process is actually listening on the target port inside the sandbox

If the terminal command looks concatenated, for example `node server.jsls`, the shell did not receive a newline or the service was started in the foreground. Start the service with a background command such as `node server.js > server.log 2>&1 &`, then run `ls`, `cat server.log`, or `curl 127.0.0.1:3000` as separate commands.

### Sandbox Stop/Start Returns 404

Check that the running API server binary includes the lifecycle routes:

```sh
curl -i -X POST http://127.0.0.1:18080/v1/sandboxes/not-a-uuid/stop
curl -i -X POST http://127.0.0.1:18080/v1/sandboxes/not-a-uuid/start
```

Expected response is `400 Bad Request` with `invalid sandbox id`. A `404 Not Found` usually means Vite is proxying to a stale API process that was started before the route was added. Restart the Go API server or restart `./scripts/dev.sh`.

### Workspace Storage Does Not Persist

Check:

- the template has `storageRequest` set
- the generated `SandboxTemplate` has `spec.volumeClaimTemplates`
- the resolved runtime Pod mounts the `workspace` PVC at the template `workingDir`
- the PVC is `Bound`
- the replacement Pod reuses the same PVC after Pod deletion

### API Is Unavailable in the Web Console

Check:

```sh
curl -fsS http://127.0.0.1:18080/healthz
```

If the API uses another address, restart Vite with `MBOX_API_PROXY_TARGET`.

### Template Preset Looks Wrong

Template table presets are derived from metadata when present, otherwise from concrete CPU and memory requests. If the user edits CPU or memory manually, the expected saved preset is `Custom` unless the pair exactly matches:

- Small: `250m` and `512Mi`
- Medium: `500m` and `1Gi`
- Large: `1000m` and `2Gi`

If the table says Small while the API payload has custom resource values, inspect the `PATCH /v1/templates/{templateID}` request body and confirm `metadata.resourcePreset` matches the final CPU and memory fields.

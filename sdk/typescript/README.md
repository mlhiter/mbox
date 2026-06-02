# @mbox/sdk

TypeScript SDK for the mbox HTTP API. The SDK is a thin client over product primitives: projects, project policies, project quota policies, credential references, templates, sandboxes, runtime access, runtime sessions, execution tasks, preview ports, and artifact references.

It is intended for external agents, IDE tools, CI systems, release tools, and scripts that call mbox as a lower-level execution platform. It does not include an agent brain or workflow engine.

`createMboxClientFromEnv()` follows the CLI environment convention by reading `MBOX_API_URL`, `MBOX_TOKEN` or `MBOX_API_TOKEN`, `MBOX_REQUEST_ID`, `MBOX_AUDIT_ACTOR`, and `MBOX_AUDIT_SOURCE`. It is a process-env convenience for automation scripts and does not read CLI context files.

## Usage

```ts
import { assertOpenAPIAlignment, createMboxClientFromEnv, isPolicyDeniedAuditEvent, MboxClient } from "@mbox/sdk"

const mbox = new MboxClient({
  baseUrl: "http://127.0.0.1:18080",
  token: process.env.MBOX_TOKEN,
  requestId: process.env.MBOX_REQUEST_ID,
  auditActor: "agent-runner",
  auditSource: "sdk",
})

const envMbox = createMboxClientFromEnv()

const info = await mbox.info()
console.log(info.apiVersion, info.capabilities)

const caller = await mbox.caller()
console.log(caller.mode, caller.rbacTrusted, caller.projectRolesEnforced)

const authorization = await mbox.getProjectAuthorization("project-id", { action: "sandbox.launch" })
console.log(authorization.evaluation, authorization.requiredRoles)

await mbox.assertCompatibility({
  requiredCapabilities: ["sandboxes", "execution-tasks", "task-events", "artifact-client-upload"],
})

const contract = await mbox.openAPI()
console.log(contract.openapi, Object.keys(contract.paths).length)
assertOpenAPIAlignment(contract)

const runtimeResources = await mbox.listRuntimeResources()
const claimResources = await mbox.listRuntimeResources({ kind: "SandboxClaim" })
const projectClaims = await mbox.listRuntimeResources({ projectId: "project-id", kind: "SandboxClaim" })
const orphanAudit = await mbox.listRuntimeOrphans()
const namespaceAudit = await mbox.listRuntimeOrphans("mbox-smoke-20260529")
const projectAudit = await mbox.listRuntimeOrphans({ projectId: "project-id" })
const templateAudit = await mbox.listRuntimeOrphans({ kind: "SandboxTemplate" })
console.log(runtimeResources.items.length)
console.log(claimResources.summary.total)
console.log(claimResources.summary.workload.observedPods, claimResources.summary.workload.requests?.cpu)
console.log(claimResources.items[0]?.observation?.podPhase, claimResources.items[0]?.observation?.requests?.cpu)
console.log(orphanAudit.expectedClean, orphanAudit.orphanCount)
console.log(namespaceAudit.namespace, projectAudit.expectedClean, namespaceAudit.expectedClean)
console.log(templateAudit.resourceCount)

const orphan = namespaceAudit.items.find((item) => item.reason === "missing-sandbox-record")
if (orphan) {
  await mbox.cleanupRuntimeOrphan({
    resource: {
      adapter: orphan.resource.adapter,
      kind: orphan.resource.kind,
      namespace: orphan.resource.namespace!,
      name: orphan.resource.name,
    },
    reason: orphan.reason,
    deleteOrphan: true,
    confirm: "delete-orphan-runtime-resource",
  })
}

const sandbox = await mbox.getSandbox("sandbox-id")
const usage = await mbox.getProjectUsage(sandbox.projectId)
console.log(usage.sandboxes.running, usage.executionTasks.total, usage.artifacts.retainedBytes)

const member = await mbox.createProjectMember(sandbox.projectId, {
  principalType: "automation",
  principal: "agent-runner",
  role: "operator",
})
console.log((await mbox.listProjectMembers(sandbox.projectId)).items.length, member.role)
const launchAuthorization = await mbox.getProjectAuthorization(sandbox.projectId, { action: "sandbox.launch" })
console.log(launchAuthorization.evaluation, launchAuthorization.caller.rbacTrusted)

const events = await mbox.listProjectAuditEvents(sandbox.projectId, {
  action: "policy.denied",
  operation: "sandbox.launch",
  reason: "active sandbox quota exceeded",
  actor: "agent-runner",
  source: "sdk",
  requestId: process.env.MBOX_REQUEST_ID,
  since: "2026-05-30T00:00:00Z",
  until: "2026-05-30T01:00:00Z",
  limit: 10,
})
console.log(events.items.map((event) => event.action))

const denied = events.items.find(isPolicyDeniedAuditEvent)
if (denied) {
  console.log(denied.metadata.operation, denied.metadata.reason, denied.metadata.requestId)
}

const boundary = await mbox.getSandboxBoundary(sandbox.id)
console.log(boundary.checks.map((check) => `${check.id}:${check.status}`))

await mbox.setProjectPolicy(sandbox.projectId, {
  enforcement: "enforced",
  allowedImagePrefixes: ["node:", "busybox:"],
  allowedServiceAccounts: ["mbox-sandbox"],
})

await mbox.setProjectQuotaPolicy(sandbox.projectId, {
  enforcement: "enforced",
  maxActiveSandboxes: 5,
  maxRetainedArtifactBytes: 1_048_576,
})

const quotaPolicy = await mbox.getProjectQuotaPolicy(sandbox.projectId)
console.log(quotaPolicy.enforcement, quotaPolicy.maxActiveSandboxes)

await mbox.createProjectCredential(sandbox.projectId, {
  name: "GitHub App",
  type: "git",
  target: "https://github.com/mlhiter/mbox",
  secretRef: { name: "github-app-token", key: "token" },
  usage: ["clone", "fetch"],
})

const validation = await mbox.createTemplateValidationRun("template-id", {
  projectId: sandbox.projectId,
  metadata: { caller: "agent-runner" },
})

await mbox.decideTemplateValidationRun("template-id", validation.sandbox.id, {
  status: "passed",
})

const validationResult = await mbox.runTemplateValidation("template-id", {
  projectId: sandbox.projectId,
  validationMetadata: { caller: "agent-runner" },
  task: {
    command: ["sh", "-lc", "pwd && echo template-ok"],
    timeoutSeconds: 60,
    metadata: { caller: "agent-runner" },
  },
  intervalMs: 1500,
  timeoutMs: 300_000,
  requireSuccess: true,
})
console.log(validationResult.status, validationResult.task?.status)

const session = await mbox.createRuntimeSession(sandbox.id, {
  type: "custom",
  client: "agent-runner",
  metadata: { purpose: "task orchestration" },
})

const task = await mbox.createExecutionTask(sandbox.id, {
  command: ["sh", "-lc", "npm test -- --reporter=json > /workspace/reports/test.json"],
  timeoutSeconds: 300,
  metadata: { caller: "agent" },
})

await mbox.watchExecutionTask(task.id, {
  onEvent(event) {
    if (event.type === "output") {
      process.stdout.write(event.data)
    }
  },
})

const finished = await mbox.waitForTask(task.id, {
  intervalMs: 1500,
  timeoutMs: 360_000,
  requireSuccess: true,
})

await mbox.createArtifact(sandbox.id, {
  taskId: finished.id,
  kind: "report",
  name: "test report",
  uri: "workspace:///workspace/reports/test.json",
  contentType: "application/json",
})

const response = await mbox.getArtifactContent("artifact-id")
const report = await response.text()

await mbox.captureArtifactContent("artifact-id")
await mbox.uploadArtifactContent("artifact-id", new Blob(["client report"], { type: "text/plain" }), {
  headers: { "content-type": "text/plain" },
})

await mbox.endRuntimeSession(session.id)
```

Boundary summaries expose namespace, ServiceAccount, token automount, secret reference projection, project credential-reference projection, network policy projection, lifecycle policy projection, launch policy state, runtime access paths, and cleanup behavior. Project usage summarizes mbox product records for sandboxes, sessions, tasks, artifacts, templates, and credential references; active/running sandbox request totals are summed from saved template request strings and are not live Kubernetes metrics. Project quota policy currently gates active sandbox creation and retained artifact-byte capture/upload from product records; it is not billing, reservation, or live cluster capacity management. `caller()` reports the current caller/auth boundary as anonymous, shared-token, or explicitly enabled `trusted_header` mode. Trusted-header callers can set `rbacTrusted=true` for preflight matching and explicitly enabled project RBAC actions; `projectRolesEnforced=true` currently means only the starter `sandbox.launch`, `runtime.operate`, `artifact.write`, `policy.manage`, and `credential.manage` gates are active. `getProjectAuthorization(projectId, { action })` maps project actions to required roles and explains the current caller/member state; anonymous/shared-token callers are untrusted when enforcement is active, while trusted-header callers can return allowed or denied decisions. Project member helpers manage `user`, `service_account`, and `automation` principal role records; with trusted headers and project RBAC enforcement explicitly enabled, owner/operator members can authorize sandbox creation, template validation launches, active runtime operations, and artifact writes, while owner members can update project launch/quota policies and create/delete project credential-reference records. Product audit events list recent successful API mutations for operator visibility, but they are not yet a strong transactional audit log or auth identity model. `policy.denied` events expose typed metadata for the current denial operations including `sandbox.launch`, `template.validation`, `project.policy.update`, `project.quota_policy.update`, `project.credential.create`, `project.credential.delete`, runtime operations, execution task operations, `artifact.write`, `artifact.content.workspace.read`, `artifact.content.capture`, and `artifact.content.upload`, including starter RBAC denial fields when applicable. Project launch policy currently gates sandbox launches by image prefix, ServiceAccount name, and template secret reference names; lifecycle `ttlSeconds` is enforced by the reconciler; project credential records store only Kubernetes Secret references and metadata. Credential list/get routes remain read visibility and do not expose secret values. Runtime resource inventory lists mbox-managed runtime resources as reported by the runtime auditor and includes a live summary by kind, namespace, label-derived owner, label-derived project, and best-effort SandboxClaim Pod/PVC observation for operator triage; inventory and orphan audit helpers can be scoped by namespace, runtime owner project label, kind, or a combination of those filters. Summary workload fields roll up observed Pods, container readiness, restarts, requests, limits, and storage capacity for the filtered inventory. `summary.byProject` is computed after the same filters and only counts resources with runtime owner project labels. Observation fields expose current Pod phase, readiness, restart count, summed requests/limits, and PVC state. Neither layer is metrics-server utilization, quota, billing, RBAC, or capacity reservation. Runtime orphan audit compares that inventory with product records, and cleanup deletes only one currently reported orphan after the caller supplies the matching reason and confirmation string. This slice does not mount credentials, replace full RBAC, or add automatic orphan cleanup.

`runTemplateValidation(templateId, options)` is an SDK convenience over existing routes, not a server-side workflow or CI engine. It calls `createTemplateValidationRun()`, waits for the validation sandbox to reach `running` with a `runtimeRef`, creates one execution task from `task.command`, waits for its terminal task record, then writes a template validation decision as `passed` only when the task succeeds. Put validation-run metadata in `validationMetadata` and task metadata in `task.metadata`; use `timeoutMs` and `intervalMs` for SDK-side polling and `task.timeoutSeconds` for the server-side task runtime limit. The result contains `validation`, `sandbox`, optional `task`, `decision`, `decisionStatus`, and `status`. If sandbox readiness or task execution fails, the helper best-effort records a failed decision and throws `MboxTemplateValidationRunError` with the partial result on `error.result`. If the task reaches an unsuccessful terminal status, `requireSuccess: true` makes the helper throw after recording the failed decision; without it, the failed result is returned for caller-controlled handling.
Audit list helpers accept `reason` as a `metadata.reason` filter alongside `requestId`, `operation`, `since`, and `until`, mainly for narrowing `policy.denied` events during operator investigation.
Task commands are array-form commands. Use an explicit shell such as `["sh", "-lc", "..."]` when shell parsing is required. By default, `waitForTask()` returns any terminal task status; pass `requireSuccess: true` to throw `MboxTaskStatusError` for `failed`, `canceled`, or `timed_out` while keeping the final task on `error.task`.
Task watch streams newline-delimited JSON events from the API and resolves after the terminal `done` event. Workspace artifact content reads require a running sandbox and a `workspace://` file reference unless the artifact has retained content. `captureArtifactContent` retains small workspace-file bytes server-side, while `uploadArtifactContent` stores client-provided bytes through the same retained-content backend. Both paths return size, sha256, source URI, and storage-provider metadata.
Use `waitForSandbox(sandbox.id, { status: "running", requireRuntimeRef: true })` before calling runtime routes from scripts that just launched or started a sandbox. It polls the product sandbox record, returns the final sandbox on success, throws `MboxSandboxStatusError` if the sandbox reaches `failed` or `deleted` while waiting for another status, and throws `MboxSandboxRuntimeRefError` if the requested status is reached but `runtimeRef` is still absent when the wait times out.
Runtime sessions are audit records for terminal, IDE, notebook, browser, command, or custom clients that attach to a sandbox; they do not imply an internal agent identity model. `auditActor` and `auditSource` set client-supplied audit attribution headers on SDK requests. They improve operator visibility in audit events but are not authentication or authorization claims.

When the API server is started with `MBOX_API_TOKEN`, pass the matching client token through `new MboxClient({ token })`. The SDK sends it as `Authorization: Bearer <token>`. This is a shared automation token, not a user identity or RBAC model.

Use `checkCompatibility()` or `assertCompatibility()` as an explicit preflight before a longer automation run. The check compares this SDK's API compatibility label with the server's `/v1/info` minimum SDK API version and can also require server capabilities such as `execution-tasks`, `task-events`, or `artifact-client-upload`. It returns or throws before the caller starts creating sandboxes or tasks. It is a compatibility and capability gate, not authentication.

API compatibility labels are not npm package versions. The SDK currently uses labels shaped as `vNalphaM`, `vNbetaM`, or `vN`. Client, server, and minimum labels must stay in the same major family, and ordering inside a family is `alpha` before `beta` before stable. For example, a `v1beta1` SDK can satisfy a server minimum of `v1alpha2`, while a `v1` SDK cannot satisfy a `v2alpha1` minimum. Capabilities stay separate from version labels so callers can require optional primitives before starting a longer task.

## Build

```sh
npm run build
```

## Smoke Check

```sh
npm run smoke
```

The smoke check builds the package, then runs a local Node-based guard over the exported compatibility helpers, `MboxClient.assertCompatibility()`, and the OpenAPI alignment success and failure paths. It does not require a running API server.

## Live API Smoke

```sh
MBOX_API_URL=http://127.0.0.1:18080 npm run smoke:api
```

The live API smoke builds the package, connects to an already running mbox API server, creates a temporary project, project-scoped template, sandbox, runtime session record, and client-uploaded artifact, verifies retained artifact content and request-correlated audit metadata, then deletes the sandbox and project. It does not enable runtime access or Kubernetes reconciliation, and it is intentionally not part of `npm run verify` because it writes to the selected development API database.

## Runtime Smoke

```sh
MBOX_API_URL=http://127.0.0.1:18080 \
MBOX_SDK_RUNTIME_SANDBOX_ID='<running-sandbox-id>' \
npm run smoke:runtime
```

The runtime smoke builds the package, connects to an already running runtime-enabled mbox API server, requires a running sandbox with a runtime reference, starts an execution task through the SDK, waits for success, registers a task artifact, captures retained workspace content, and reads the retained bytes back. It is intentionally not part of `npm run verify` because it requires Kubernetes runtime access and writes task/artifact records to the selected development API.

## Package Dry Run

```sh
npm run check:pack
```

The package check builds the SDK, runs `npm pack --dry-run --json`, and verifies the publishable tarball would contain the README, package manifest, compiled JavaScript, and TypeScript declarations while excluding source-only files. It does not publish the package.

## Package Consumer Smoke

```sh
npm run check:pack:consumer
```

The consumer smoke builds the SDK, creates a real local `npm pack` tarball in a temporary directory, installs it into a minimal private ESM consumer project with `--ignore-scripts`, and imports `@mbox/sdk`. It verifies the package `exports` entry can load the compiled SDK and that the compatibility helper works from the installed tarball. It does not publish the package or contact the public npm registry.

## Publish Gate

```sh
npm run verify
```

The verify command runs typecheck, the local smoke check, the package dry run, and the package consumer smoke. The package also wires `prepublishOnly` to `npm run verify`, so `npm publish` runs these gates before publishing.

## OpenAPI Alignment Check

Use this when SDK wrappers or public routes change:

```sh
npm run check:openapi -- http://127.0.0.1:18080
```

The command fetches `/v1/openapi.json` from the API base URL and verifies every route-backed SDK helper in `SDK_ROUTE_CONTRACT` has a matching OpenAPI path, method, SDK-used query parameter declarations, route auth metadata, focused request body shape, and focused response shape. It also checks reverse route coverage: ordinary published OpenAPI operations must have SDK route contract entries. The intentional non-helper exceptions are the terminal WebSocket upgrade route and preview proxy pass-through route, which are not ordinary JSON SDK helpers. Auth checks cover the bearer security scheme, explicit public operations, private bearer operations, and `401` responses. Request checks cover JSON schema refs and binary upload media types. Response checks cover direct schema refs, list item refs, NDJSON task-event streams, binary responses, and no-content delete routes. It also checks the starter `SDK_SCHEMA_CONTRACT` for required fields, properties, selected forbidden properties, and selected enum values that the SDK types already rely on, including `CallerInfo` mode/principal-type values and project authorization action/evaluation values. It can also read a saved OpenAPI JSON file.

When the target API requires `MBOX_API_TOKEN`, set `MBOX_TOKEN` or `MBOX_API_TOKEN` for this command so it can fetch the private OpenAPI route.

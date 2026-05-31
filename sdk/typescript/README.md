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

await mbox.assertCompatibility({
  requiredCapabilities: ["sandboxes", "execution-tasks", "task-events", "artifact-client-upload"],
})

const contract = await mbox.openAPI()
console.log(contract.openapi, Object.keys(contract.paths).length)
assertOpenAPIAlignment(contract)

const runtimeResources = await mbox.listRuntimeResources()
const claimResources = await mbox.listRuntimeResources({ kind: "SandboxClaim" })
const orphanAudit = await mbox.listRuntimeOrphans()
const namespaceAudit = await mbox.listRuntimeOrphans("mbox-smoke-20260529")
const templateAudit = await mbox.listRuntimeOrphans({ kind: "SandboxTemplate" })
console.log(runtimeResources.items.length)
console.log(claimResources.summary.total)
console.log(orphanAudit.expectedClean, orphanAudit.orphanCount)
console.log(namespaceAudit.namespace, namespaceAudit.expectedClean)
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

const events = await mbox.listProjectAuditEvents(sandbox.projectId, {
  action: "policy.denied",
  operation: "sandbox.launch",
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

Boundary summaries expose namespace, ServiceAccount, token automount, secret reference projection, project credential-reference projection, network policy projection, lifecycle policy projection, launch policy state, runtime access paths, and cleanup behavior. Project usage summarizes mbox product records for sandboxes, sessions, tasks, artifacts, templates, and credential references; active/running sandbox request totals are summed from saved template request strings and are not live Kubernetes metrics. Project quota policy currently gates active sandbox creation and retained artifact-byte capture/upload from product records; it is not billing, reservation, or live cluster capacity management. Product audit events list recent successful API mutations for operator visibility, but they are not yet a strong transactional audit log or auth identity model. `policy.denied` events expose typed metadata for the current denial operations: `sandbox.launch`, `template.validation`, `artifact.content.capture`, and `artifact.content.upload`. Project launch policy currently gates sandbox launches by image prefix, ServiceAccount name, and template secret reference names; lifecycle `ttlSeconds` is enforced by the reconciler; project credential records store only Kubernetes Secret references and metadata. Runtime resource inventory lists mbox-managed runtime resources as reported by the runtime auditor and includes a live summary by kind, namespace, and label-derived owner for operator triage; inventory and orphan audit helpers can be scoped by namespace, kind, or both. Runtime orphan audit compares that inventory with product records, and cleanup deletes only one currently reported orphan after the caller supplies the matching reason and confirmation string. This slice does not mount credentials, replace full RBAC, or add automatic orphan cleanup.

Task commands are array-form commands. Use an explicit shell such as `["sh", "-lc", "..."]` when shell parsing is required.
Task watch streams newline-delimited JSON events from the API and resolves after the terminal `done` event. Workspace artifact content reads require a running sandbox and a `workspace://` file reference unless the artifact has retained content. `captureArtifactContent` retains small workspace-file bytes server-side, while `uploadArtifactContent` stores client-provided bytes through the same retained-content backend. Both paths return size, sha256, source URI, and storage-provider metadata.
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

The command fetches `/v1/openapi.json` from the API base URL and verifies every route-backed SDK helper in `SDK_ROUTE_CONTRACT` has a matching OpenAPI path, method, SDK-used query parameter declarations, route auth metadata, focused request body shape, and focused response shape. Auth checks cover the bearer security scheme, explicit public operations, private bearer operations, and `401` responses. Request checks cover JSON schema refs and binary upload media types. Response checks cover direct schema refs, list item refs, NDJSON task-event streams, binary responses, and no-content delete routes. It also checks the starter `SDK_SCHEMA_CONTRACT` for required fields and properties that the SDK types already rely on. It can also read a saved OpenAPI JSON file.

When the target API requires `MBOX_API_TOKEN`, set `MBOX_TOKEN` or `MBOX_API_TOKEN` for this command so it can fetch the private OpenAPI route.

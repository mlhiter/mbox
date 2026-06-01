#!/usr/bin/env node
import assert from "node:assert/strict"
import { createMboxClientFromEnv, MboxAPIError } from "../dist/index.js"

const baseUrl = process.env.MBOX_API_URL ?? "http://127.0.0.1:18080"
const sandboxId =
  process.env.MBOX_SDK_RUNTIME_SANDBOX_ID ??
  process.env.MBOX_SANDBOX_ID ??
  process.argv[2]
const expectedBackend =
  process.env.MBOX_EXPECTED_ARTIFACT_CONTENT_BACKEND ??
  process.env.MBOX_ARTIFACT_CONTENT_BACKEND
const waitTimeoutMs =
  parsePositiveInteger(
    process.env.MBOX_SDK_RUNTIME_WAIT_TIMEOUT_SECONDS ??
      process.env.MBOX_SMOKE_TIMEOUT_SECONDS,
    120,
  ) * 1000
const taskTimeoutSeconds = parsePositiveInteger(
  process.env.MBOX_SDK_RUNTIME_TASK_TIMEOUT_SECONDS,
  30,
)
const requestId = process.env.MBOX_REQUEST_ID ?? `sdk-runtime-smoke-${Date.now()}`
const auditActor = process.env.MBOX_AUDIT_ACTOR ?? "sdk-runtime-smoke"
const auditSource = process.env.MBOX_AUDIT_SOURCE ?? "mbox-sdk"
const marker = `sdk-runtime-ok:${Date.now()}`

if (!sandboxId) {
  console.error(
    "missing sandbox id; set MBOX_SDK_RUNTIME_SANDBOX_ID or pass one argument",
  )
  process.exit(2)
}

const client = createMboxClientFromEnv(process.env, {
  baseUrl,
  requestId,
  auditActor,
  auditSource,
})

try {
  const health = await client.health()
  assert.equal(health.status, "ok")

  const info = await client.info()
  assert.equal(info.runtimeAccess.enabled, true)
  assert.equal(info.runtimeAccess.adapter, "agent-sandbox")
  assert.equal(info.artifactContent.retainedContentEnabled, true)
  if (expectedBackend) {
    assert.equal(info.artifactContent.storageProvider, expectedBackend)
  }
  await client.assertCompatibility({
    requiredCapabilities: ["execution-tasks", "task-events", "artifact-client-upload"],
  })

  const sandbox = await client.getSandbox(sandboxId)
  assert.equal(sandbox.id, sandboxId)
  assert.equal(sandbox.status, "running")
  assert.ok(sandbox.runtimeRef?.name, "sandbox must have a runtimeRef")

  const runtimeTarget = await client.getRuntimeTarget(sandboxId)
  assert.equal(runtimeTarget.namespace, sandbox.runtimeRef.namespace)
  assert.ok(runtimeTarget.podName, "runtime target must resolve a pod")
  assert.ok(runtimeTarget.container, "runtime target must resolve a container")

  const task = await client.createExecutionTask(sandboxId, {
    command: ["sh", "-lc", `printf '${marker}' | tee /workspace/sdk-runtime.txt`],
    timeoutSeconds: taskTimeoutSeconds,
    metadata: { smoke: "sdk-runtime" },
  })
  const finished = await client.waitForTask(task.id, {
    intervalMs: 500,
    timeoutMs: waitTimeoutMs,
    requireSuccess: true,
  })
  assert.equal(finished.status, "succeeded")
  assert.ok(finished.stdout?.includes(marker))

  const artifact = await client.createArtifact(sandboxId, {
    taskId: finished.id,
    kind: "report",
    name: "sdk-runtime.txt",
    uri: "workspace:///workspace/sdk-runtime.txt",
    contentType: "text/plain",
    metadata: { smoke: "sdk-runtime" },
  })
  assert.equal(artifact.taskId, finished.id)

  const taskArtifacts = await client.listTaskArtifacts(finished.id)
  assert.ok((taskArtifacts.items ?? []).some((item) => item.id === artifact.id))

  const captured = await client.captureArtifactContent(artifact.id)
  assert.equal(captured.retainedContent?.storageProvider, info.artifactContent.storageProvider)
  assert.ok((captured.retainedContent?.sizeBytes ?? 0) > 0)
  assert.equal(captured.retainedContent?.sha256.length, 64)

  const contentResponse = await client.getArtifactContent(artifact.id)
  assert.equal(contentResponse.headers.get("x-mbox-artifact-retained"), "true")
  assert.equal(await contentResponse.text(), marker)

  console.log(`SDK runtime smoke passed against ${baseUrl} for sandbox ${sandboxId}`)
} catch (error) {
  console.error(error instanceof Error ? error.message : String(error))
  if (error instanceof MboxAPIError && error.body) {
    console.error(JSON.stringify(error.body))
  }
  process.exitCode = 1
}

function parsePositiveInteger(value, fallback) {
  if (value === undefined || value === "") {
    return fallback
  }
  const parsed = Number.parseInt(value, 10)
  if (!Number.isFinite(parsed) || parsed <= 0) {
    throw new Error(`expected positive integer, got ${value}`)
  }
  return parsed
}

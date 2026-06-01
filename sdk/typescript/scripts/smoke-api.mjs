#!/usr/bin/env node
import assert from "node:assert/strict"
import { createMboxClientFromEnv, fetchAndAssertOpenAPIAlignment, MboxAPIError } from "../dist/index.js"

const baseUrl = process.env.MBOX_API_URL ?? "http://127.0.0.1:18080"
const requestId = process.env.MBOX_REQUEST_ID ?? `sdk-smoke-${Date.now()}`
const auditActor = process.env.MBOX_AUDIT_ACTOR ?? "sdk-smoke"
const auditSource = process.env.MBOX_AUDIT_SOURCE ?? "mbox-sdk"
const suffix = process.env.MBOX_SDK_SMOKE_SUFFIX ?? timestampSuffix()
const name = `sdk-smoke-${suffix}`

const client = createMboxClientFromEnv(process.env, {
  baseUrl,
  requestId,
  auditActor,
  auditSource,
})

let project
let template
let sandbox

try {
  await client.health()

  const info = await client.info()
  assert.equal(info.name, "mbox")
  await client.assertCompatibility({
    requiredCapabilities: ["sandboxes", "artifact-client-upload"],
  })
  await fetchAndAssertOpenAPIAlignment(client)

  project = await client.createProject({
    name,
    slug: name,
    defaultNamespace: name,
    metadata: { smoke: "sdk-live-api" },
  })
  assert.equal(project.slug, name)

  template = await client.createTemplate({
    projectId: project.id,
    name: `${name}-template`,
    slug: `${name}-template`,
    image: "busybox:1.36",
    startupCommand: ["sh", "-c", "echo mbox sdk smoke ready && tail -f /dev/null"],
    workingDir: "/workspace",
    cpuRequest: "100m",
    memoryRequest: "128Mi",
    storageRequest: "1Gi",
    exposedPorts: [{ name: "web", port: 8080, protocol: "TCP" }],
    metadata: { smoke: "sdk-live-api" },
  })
  assert.equal(template.projectId, project.id)

  await client.getTemplate(template.id)
  const templates = await client.listTemplates(project.id)
  assert.ok((templates.items ?? []).some((item) => item.id === template.id))

  sandbox = await client.createSandbox({
    projectId: project.id,
    templateId: template.id,
    name: `${name}-sandbox`,
    slug: `${name}-sandbox`,
    metadata: { smoke: "sdk-live-api" },
  })
  assert.equal(sandbox.projectId, project.id)
  assert.equal(sandbox.templateId, template.id)
  assert.equal(sandbox.status, "pending")

  const projectUsage = await client.getProjectUsage(project.id)
  assert.equal(projectUsage.sandboxes.active, 1)
  assert.equal(projectUsage.templates.projectScoped, 1)

  const sandboxBoundary = await client.getSandboxBoundary(sandbox.id)
  assert.equal(sandboxBoundary.kind, "sandbox")
  assert.equal(sandboxBoundary.sandboxId, sandbox.id)
  assert.equal(sandboxBoundary.serviceAccountTokenAutomount, false)

  const session = await client.createRuntimeSession(sandbox.id, {
    type: "custom",
    client: "sdk-smoke",
    metadata: { purpose: "live-api-smoke" },
  })
  assert.equal(session.status, "active")

  const sessions = await client.listRuntimeSessions(sandbox.id)
  assert.ok((sessions.items ?? []).some((item) => item.id === session.id))

  const endedSession = await client.endRuntimeSession(session.id)
  assert.equal(endedSession.status, "ended")
  assert.ok(endedSession.endedAt)

  const artifact = await client.createArtifact(sandbox.id, {
    kind: "report",
    name: "sdk-smoke-report.txt",
    uri: "client://sdk-smoke/report.txt",
    contentType: "text/plain",
    metadata: { source: "sdk-live-api-smoke" },
  })
  assert.equal(artifact.kind, "report")

  const reportBody = `sdk-smoke-ok:${suffix}`
  const uploaded = await client.uploadArtifactContent(artifact.id, new Blob([reportBody], { type: "text/plain" }), {
    headers: {
      "content-type": "text/plain",
      "x-mbox-artifact-source-uri": "client://sdk-smoke/report.txt",
    },
  })
  assert.equal(uploaded.retainedContent?.sizeBytes, reportBody.length)
  assert.equal(uploaded.retainedContent?.storageProvider, info.artifactContent.storageProvider)

  const contentResponse = await client.getArtifactContent(artifact.id)
  assert.equal(await contentResponse.text(), reportBody)
  assert.equal(contentResponse.headers.get("x-mbox-artifact-retained"), "true")

  const artifacts = await client.listArtifacts(sandbox.id)
  assert.ok((artifacts.items ?? []).some((item) => item.id === artifact.id))

  const usageAfterArtifact = await client.getProjectUsage(project.id)
  assert.equal(usageAfterArtifact.runtimeSessions.total, 1)
  assert.equal(usageAfterArtifact.runtimeSessions.ended, 1)
  assert.equal(usageAfterArtifact.artifacts.total, 1)
  assert.equal(usageAfterArtifact.artifacts.retainedContent, 1)
  assert.equal(usageAfterArtifact.artifacts.retainedBytes, reportBody.length)

  const sandboxEvents = await client.listProjectAuditEvents(project.id, {
    action: "sandbox.created",
    resourceType: "sandbox",
    actor: auditActor,
    source: auditSource,
    requestId,
    limit: 20,
  })
  assert.ok(
    (sandboxEvents.items ?? []).some(
      (event) =>
        event.action === "sandbox.created" &&
        event.resourceId === sandbox.id &&
        event.metadata?.requestId === requestId,
    ),
  )

  await cleanup()
  console.log(`SDK API smoke passed against ${baseUrl}`)
} catch (error) {
  await cleanup()
  console.error(error instanceof Error ? error.message : String(error))
  if (error instanceof MboxAPIError && error.body) {
    console.error(JSON.stringify(error.body))
  }
  process.exitCode = 1
}

async function cleanup() {
  if (sandbox) {
    await ignoreNotFound(() => client.deleteSandbox(sandbox.id))
    sandbox = undefined
  }
  if (project) {
    await ignoreNotFound(() => client.deleteProject(project.id))
    project = undefined
  }
  template = undefined
}

async function ignoreNotFound(action) {
  try {
    await action()
  } catch (error) {
    if (error instanceof MboxAPIError && error.status === 404) {
      return
    }
    throw error
  }
}

function timestampSuffix() {
  return new Date()
    .toISOString()
    .replace(/[-:TZ.]/g, "")
    .slice(0, 14)
    .toLowerCase()
}

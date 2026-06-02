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
let projectMember
let template
let sandbox
let rbacRequestOptions
let ownerRequestOptions

try {
  await client.health()

  const info = await client.info()
  assert.equal(info.name, "mbox")
  assert.ok(info.capabilities.includes("caller-info"))
  assert.ok(info.capabilities.includes("trusted-principal-headers"))
  assert.ok(info.capabilities.includes("project-authorization-preflight"))
  assert.ok(info.capabilities.includes("project-rbac-enforcement"))
  assert.equal(typeof info.trustedPrincipalHeaders.enabled, "boolean")
  assert.equal(typeof info.projectRbac.enforcementEnabled, "boolean")
  assert.ok(Array.isArray(info.projectRbac.enforcedActions))
  if (info.projectRbac.enforcementEnabled) {
    assert.equal(info.trustedPrincipalHeaders.enabled, true)
    assert.ok(info.projectRbac.enforcedActions.includes("sandbox.launch"))
    assert.ok(info.projectRbac.enforcedActions.includes("runtime.operate"))
    assert.ok(info.projectRbac.enforcedActions.includes("artifact.write"))
    assert.ok(info.projectRbac.enforcedActions.includes("policy.manage"))
    assert.ok(info.projectRbac.enforcedActions.includes("credential.manage"))
    assert.ok(info.projectRbac.enforcedActions.includes("member.manage"))
  } else {
    assert.deepEqual(info.projectRbac.enforcedActions, [])
  }
  rbacRequestOptions = info.projectRbac.enforcementEnabled
    ? trustedPrincipalRequestOptions(info, "sdk-smoke-bot")
    : undefined
  ownerRequestOptions = info.projectRbac.enforcementEnabled
    ? trustedPrincipalRequestOptions(info, "sdk-smoke-owner")
    : undefined
  const caller = await client.caller(rbacRequestOptions)
  assert.equal(caller.authenticationRequired, info.authenticationRequired)
  if (caller.mode === "trusted_header") {
    assert.equal(caller.mode, "trusted_header")
    assert.equal(caller.rbacTrusted, true)
  } else {
    assert.equal(caller.rbacTrusted, false)
  }
  assert.equal(caller.projectRolesEnforced, info.projectRbac.enforcementEnabled)
  await client.assertCompatibility({
    requiredCapabilities: ["sandboxes", "artifact-client-upload"],
  })
  await fetchAndAssertOpenAPIAlignment(client)

  project = await client.createProject(
    {
      name,
      slug: name,
      defaultNamespace: name,
      metadata: { smoke: "sdk-live-api" },
    },
    ownerRequestOptions,
  )
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

  if (info.projectRbac.enforcementEnabled) {
    const deniedAuthorization = await client.getProjectAuthorization(project.id, { action: "sandbox.launch" })
    assert.equal(deniedAuthorization.enforced, true)
    assert.equal(deniedAuthorization.allowed, false)
    assert.equal(deniedAuthorization.evaluation, "denied")
    assert.equal(deniedAuthorization.caller.rbacTrusted, false)
    await expectLaunchDenied(() =>
      client.createSandbox({
        projectId: project.id,
        templateId: template.id,
        name: `${name}-denied-sandbox`,
        slug: `${name}-denied-sandbox`,
        metadata: { smoke: "sdk-live-api-denied" },
      }),
    )
    await expectForbidden(
      () =>
        client.createProjectMember(
          project.id,
          {
            principalType: "automation",
            principal: "sdk-smoke-viewer",
            role: "viewer",
            metadata: { smoke: "sdk-live-api-rbac-denied" },
          },
          rbacRequestOptions,
        ),
      "expected operator member create to be denied",
    )
    projectMember = await client.createProjectMember(
      project.id,
      {
        principalType: "automation",
        principal: "sdk-smoke-bot",
        role: "operator",
        metadata: { smoke: "sdk-live-api-rbac" },
      },
      ownerRequestOptions,
    )
    assert.equal(projectMember.projectId, project.id)
    assert.equal(projectMember.role, "operator")
  }

  await client.getTemplate(template.id)
  const templates = await client.listTemplates(project.id)
  assert.ok((templates.items ?? []).some((item) => item.id === template.id))

  sandbox = await client.createSandbox(
    {
      projectId: project.id,
      templateId: template.id,
      name: `${name}-sandbox`,
      slug: `${name}-sandbox`,
      metadata: { smoke: "sdk-live-api" },
    },
    rbacRequestOptions,
  )
  assert.equal(sandbox.projectId, project.id)
  assert.equal(sandbox.templateId, template.id)
  assert.equal(sandbox.status, "pending")

  const authorization = await client.getProjectAuthorization(project.id, {
    action: "sandbox.launch",
    ...rbacRequestOptions,
  })
  assert.equal(authorization.projectId, project.id)
  assert.equal(authorization.action, "sandbox.launch")
  assert.equal(authorization.enforced, info.projectRbac.enforcementEnabled)
  assert.deepEqual(authorization.requiredRoles, ["owner", "operator"])
  assert.equal(authorization.caller.projectRolesEnforced, info.projectRbac.enforcementEnabled)
  if (info.projectRbac.enforcementEnabled) {
    assert.equal(authorization.allowed, true)
    assert.equal(authorization.evaluation, "allowed")
    assert.equal(authorization.caller.rbacTrusted, true)
    assert.equal(authorization.matchedMember?.id, projectMember.id)
  } else if (authorization.caller.rbacTrusted) {
    assert.ok(["allowed", "denied"].includes(authorization.evaluation))
  } else {
    assert.equal(authorization.allowed, false)
    assert.equal(authorization.evaluation, "not_enforceable")
    assert.equal(authorization.caller.rbacTrusted, false)
  }

  const artifactAuthorization = await client.getProjectAuthorization(project.id, {
    action: "artifact.write",
    ...rbacRequestOptions,
  })
  assert.equal(artifactAuthorization.projectId, project.id)
  assert.equal(artifactAuthorization.action, "artifact.write")
  assert.equal(artifactAuthorization.enforced, info.projectRbac.enforcementEnabled)
  assert.deepEqual(artifactAuthorization.requiredRoles, ["owner", "operator"])
  if (info.projectRbac.enforcementEnabled) {
    assert.equal(artifactAuthorization.allowed, true)
    assert.equal(artifactAuthorization.evaluation, "allowed")
    assert.equal(artifactAuthorization.matchedMember?.id, projectMember.id)
  }

  const runtimeAuthorization = await client.getProjectAuthorization(project.id, {
    action: "runtime.operate",
    ...rbacRequestOptions,
  })
  assert.equal(runtimeAuthorization.projectId, project.id)
  assert.equal(runtimeAuthorization.action, "runtime.operate")
  assert.equal(runtimeAuthorization.enforced, info.projectRbac.enforcementEnabled)
  assert.deepEqual(runtimeAuthorization.requiredRoles, ["owner", "operator"])
  if (info.projectRbac.enforcementEnabled) {
    assert.equal(runtimeAuthorization.allowed, true)
    assert.equal(runtimeAuthorization.evaluation, "allowed")
    assert.equal(runtimeAuthorization.matchedMember?.id, projectMember.id)
  }

  const policyAuthorization = await client.getProjectAuthorization(project.id, {
    action: "policy.manage",
    ...rbacRequestOptions,
  })
  assert.equal(policyAuthorization.projectId, project.id)
  assert.equal(policyAuthorization.action, "policy.manage")
  assert.equal(policyAuthorization.enforced, info.projectRbac.enforcementEnabled)
  assert.deepEqual(policyAuthorization.requiredRoles, ["owner"])
  if (info.projectRbac.enforcementEnabled) {
    assert.equal(policyAuthorization.allowed, false)
    assert.equal(policyAuthorization.evaluation, "denied")
    assert.equal(policyAuthorization.matchedMember?.id, projectMember.id)
    await expectForbidden(
      () => client.setProjectPolicy(project.id, { enforcement: "enforced", allowedImagePrefixes: ["busybox:"] }, rbacRequestOptions),
      "expected operator policy update to be denied",
    )
    await expectForbidden(
      () => client.setProjectQuotaPolicy(project.id, { enforcement: "enforced", maxActiveSandboxes: 3 }, rbacRequestOptions),
      "expected operator quota policy update to be denied",
    )
    const memberAuthorization = await client.getProjectAuthorization(project.id, {
      action: "member.manage",
      ...rbacRequestOptions,
    })
    assert.equal(memberAuthorization.projectId, project.id)
    assert.equal(memberAuthorization.action, "member.manage")
    assert.equal(memberAuthorization.enforced, true)
    assert.deepEqual(memberAuthorization.requiredRoles, ["owner"])
    assert.equal(memberAuthorization.allowed, false)
    assert.equal(memberAuthorization.evaluation, "denied")
    assert.equal(memberAuthorization.matchedMember?.id, projectMember.id)
  } else {
    const updatedPolicy = await client.setProjectPolicy(project.id, { enforcement: "enforced", allowedImagePrefixes: ["busybox:"] }, rbacRequestOptions)
    assert.equal(updatedPolicy.enforcement, "enforced")
    const updatedQuotaPolicy = await client.setProjectQuotaPolicy(project.id, { enforcement: "enforced", maxActiveSandboxes: 3 }, rbacRequestOptions)
    assert.equal(updatedQuotaPolicy.enforcement, "enforced")
  }

  const projectUsage = await client.getProjectUsage(project.id)
  assert.equal(projectUsage.sandboxes.active, 1)
  assert.equal(projectUsage.templates.projectScoped, 1)

  const sandboxBoundary = await client.getSandboxBoundary(sandbox.id)
  assert.equal(sandboxBoundary.kind, "sandbox")
  assert.equal(sandboxBoundary.sandboxId, sandbox.id)
  assert.equal(sandboxBoundary.serviceAccountTokenAutomount, false)

  if (info.projectRbac.enforcementEnabled) {
    const deniedRuntimeAuthorization = await client.getProjectAuthorization(project.id, { action: "runtime.operate" })
    assert.equal(deniedRuntimeAuthorization.enforced, true)
    assert.equal(deniedRuntimeAuthorization.allowed, false)
    assert.equal(deniedRuntimeAuthorization.evaluation, "denied")
    await expectForbidden(() =>
      client.createRuntimeSession(sandbox.id, {
        type: "custom",
        client: "sdk-smoke-denied",
        metadata: { purpose: "sdk-live-api-denied-runtime" },
      }),
      "expected runtime operation to be denied",
    )
  }

  const session = await client.createRuntimeSession(
    sandbox.id,
    {
      type: "custom",
      client: "sdk-smoke",
      metadata: { purpose: "live-api-smoke" },
    },
    rbacRequestOptions,
  )
  assert.equal(session.status, "active")

  const sessions = await client.listRuntimeSessions(sandbox.id)
  assert.ok((sessions.items ?? []).some((item) => item.id === session.id))

  const endedSession = await client.endRuntimeSession(session.id, rbacRequestOptions)
  assert.equal(endedSession.status, "ended")
  assert.ok(endedSession.endedAt)

  if (info.projectRbac.enforcementEnabled) {
    const deniedArtifactAuthorization = await client.getProjectAuthorization(project.id, { action: "artifact.write" })
    assert.equal(deniedArtifactAuthorization.enforced, true)
    assert.equal(deniedArtifactAuthorization.allowed, false)
    assert.equal(deniedArtifactAuthorization.evaluation, "denied")
    await expectForbidden(() =>
      client.createArtifact(sandbox.id, {
        kind: "report",
        name: "sdk-smoke-denied-report.txt",
        uri: "client://sdk-smoke/denied-report.txt",
        contentType: "text/plain",
        metadata: { source: "sdk-live-api-denied-artifact" },
      }),
      "expected artifact write to be denied",
    )
  }

  const artifact = await client.createArtifact(
    sandbox.id,
    {
      kind: "report",
      name: "sdk-smoke-report.txt",
      uri: "client://sdk-smoke/report.txt",
      contentType: "text/plain",
      metadata: { source: "sdk-live-api-smoke" },
    },
    rbacRequestOptions,
  )
  assert.equal(artifact.kind, "report")

  if (info.projectRbac.enforcementEnabled) {
    await expectForbidden(
      () => client.uploadArtifactContent(artifact.id, new Blob(["denied"], { type: "text/plain" })),
      "expected artifact content upload to be denied",
    )
  }

  const reportBody = `sdk-smoke-ok:${suffix}`
  const uploaded = await client.uploadArtifactContent(
    artifact.id,
    new Blob([reportBody], { type: "text/plain" }),
    {
      ...rbacRequestOptions,
      headers: {
        ...rbacRequestOptions?.headers,
        "content-type": "text/plain",
        "x-mbox-artifact-source-uri": "client://sdk-smoke/report.txt",
      },
    },
  )
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
    await ignoreNotFound(() => client.deleteSandbox(sandbox.id, rbacRequestOptions))
    sandbox = undefined
  }
  if (projectMember) {
    await ignoreNotFound(() => client.deleteProjectMember(projectMember.id, ownerRequestOptions))
    projectMember = undefined
  }
  if (project) {
    await ignoreNotFound(() => client.deleteProject(project.id, ownerRequestOptions))
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

function trustedPrincipalRequestOptions(info, principal) {
  const principalHeader = info.trustedPrincipalHeaders.principalHeader ?? "X-Mbox-Principal"
  const principalTypeHeader = info.trustedPrincipalHeaders.principalTypeHeader ?? "X-Mbox-Principal-Type"
  return {
    headers: {
      [principalTypeHeader]: "automation",
      [principalHeader]: principal,
    },
  }
}

async function expectLaunchDenied(action) {
  await expectForbidden(action, "expected sandbox launch to be denied")
}

async function expectForbidden(action, message) {
  try {
    await action()
  } catch (error) {
    if (error instanceof MboxAPIError && error.status === 403) {
      return
    }
    throw error
  }
  throw new Error(message)
}

function timestampSuffix() {
  return new Date()
    .toISOString()
    .replace(/[-:TZ.]/g, "")
    .slice(0, 14)
    .toLowerCase()
}

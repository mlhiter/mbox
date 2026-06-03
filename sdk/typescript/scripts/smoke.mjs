#!/usr/bin/env node
import assert from "node:assert/strict"
import {
  MboxClient,
  MboxCompatibilityError,
  MboxSandboxRuntimeRefError,
  MboxSandboxStatusError,
  MboxTemplateValidationRunError,
  MboxTaskStatusError,
  OpenAPIAlignmentError,
  assertOpenAPIAlignment,
  checkClientCompatibility,
  checkSDKCompatibility,
  createMboxClientFromEnv,
} from "../dist/index.js"

const compatibleInfo = {
  name: "mbox",
  apiVersion: "v1alpha1",
  serverVersion: "test",
  runtimeController: { enabled: false },
  runtimeAccess: { enabled: false },
  artifactContent: {
    retainedContentEnabled: true,
    storageProvider: "postgres",
    maxBytes: 8388608,
  },
  trustedPrincipalHeaders: {
    enabled: false,
    principalHeader: "X-Mbox-Principal",
    principalTypeHeader: "X-Mbox-Principal-Type",
  },
  projectRbac: {
    enforcementEnabled: false,
    enforcedActions: [],
  },
  capabilities: [
    "sandboxes",
    "caller-info",
    "trusted-principal-headers",
    "project-authorization-preflight",
    "project-rbac-enforcement",
    "execution-tasks",
    "task-events",
    "artifact-client-upload",
  ],
  compatibility: {
    minimumCliApiVersion: "v1alpha1",
    minimumSdkApiVersion: "v1alpha1",
  },
  authenticationRequired: false,
}

const s3Info = {
  ...compatibleInfo,
  artifactContent: {
    ...compatibleInfo.artifactContent,
    storageProvider: "s3",
  },
}

const futureInfo = {
  ...compatibleInfo,
  apiVersion: "v1alpha2",
  compatibility: {
    minimumCliApiVersion: "v1alpha2",
    minimumSdkApiVersion: "v1alpha2",
  },
}

assert.equal(checkSDKCompatibility(compatibleInfo).ok, true)
assert.equal(checkSDKCompatibility(s3Info).ok, true)
assert.equal(checkClientCompatibility(compatibleInfo, "cli", "v1alpha1").ok, true)
assert.equal(checkSDKCompatibility(futureInfo, "v1alpha1").ok, false)
assert.equal(
  checkSDKCompatibility(
    {
      ...compatibleInfo,
      apiVersion: "v1beta1",
      compatibility: {
        minimumCliApiVersion: "v1alpha2",
        minimumSdkApiVersion: "v1alpha2",
      },
    },
    "v1beta1",
  ).ok,
  true,
)
assert.equal(
  checkSDKCompatibility(
    {
      ...compatibleInfo,
      apiVersion: "v1",
      compatibility: {
        minimumCliApiVersion: "v1beta1",
        minimumSdkApiVersion: "v1beta1",
      },
    },
    "v1",
  ).ok,
  true,
)
assert.equal(
  checkSDKCompatibility(
    {
      ...compatibleInfo,
      apiVersion: "v2alpha1",
      compatibility: {
        minimumCliApiVersion: "v2alpha1",
        minimumSdkApiVersion: "v2alpha1",
      },
    },
    "v1",
  ).ok,
  false,
)
assert.equal(checkSDKCompatibility(compatibleInfo, "dev").ok, false)
assert.deepEqual(
  checkSDKCompatibility(compatibleInfo, "v1alpha1", ["execution-tasks", "missing-capability"])
    .missingCapabilities,
  ["missing-capability"],
)

const compatibleClient = new MboxClient({
  baseUrl: "http://example.test",
  fetch: jsonFetch(compatibleInfo),
})
assert.equal(
  (
    await compatibleClient.assertCompatibility({
      requiredCapabilities: ["execution-tasks", "task-events"],
    })
  ).ok,
  true,
)

const incompatibleClient = new MboxClient({
  baseUrl: "http://example.test",
  apiVersion: "v1alpha1",
  fetch: jsonFetch(futureInfo),
})
await assert.rejects(() => incompatibleClient.assertCompatibility(), MboxCompatibilityError)
await assert.rejects(
  () =>
    compatibleClient.assertCompatibility({
      requiredCapabilities: ["runtime-access-that-does-not-exist"],
    }),
  MboxCompatibilityError,
)

const authenticatedClient = new MboxClient({
  baseUrl: "http://example.test",
  token: "secret",
  fetch: async (url, init) => {
    assert.equal(new URL(url).pathname, "/v1/info")
    assert.equal(new Headers(init?.headers).get("authorization"), "Bearer secret")
    return new Response(JSON.stringify(compatibleInfo), {
      status: 200,
      headers: { "content-type": "application/json" },
    })
  },
})
assert.equal((await authenticatedClient.info()).authenticationRequired, false)

const callerClient = new MboxClient({
  baseUrl: "http://caller.example.test",
  token: "secret",
  fetch: async (url, init) => {
    assert.equal(new URL(url).pathname, "/v1/auth/caller")
    assert.equal(new Headers(init?.headers).get("authorization"), "Bearer secret")
    return jsonResponse({
      authenticated: true,
      authenticationRequired: true,
      mode: "shared_token",
      principalType: "shared_token",
      principal: "shared-token",
      rbacTrusted: false,
      projectRolesEnforced: false,
      notes: ["shared token accepted"],
    })
  },
})
const callerInfo = await callerClient.caller()
assert.equal(callerInfo.mode, "shared_token")
assert.equal(callerInfo.rbacTrusted, false)
assert.equal(callerInfo.projectRolesEnforced, false)

const trustedCallerClient = new MboxClient({
  baseUrl: "http://trusted-caller.example.test",
  fetch: async (url) => {
    assert.equal(new URL(url).pathname, "/v1/auth/caller")
    return jsonResponse({
      authenticated: true,
      authenticationRequired: false,
      mode: "trusted_header",
      principalType: "automation",
      principal: "ci-bot",
      rbacTrusted: true,
      projectRolesEnforced: false,
      notes: ["trusted principal headers accepted"],
    })
  },
})
const trustedCaller = await trustedCallerClient.caller()
assert.equal(trustedCaller.mode, "trusted_header")
assert.equal(trustedCaller.principalType, "automation")
assert.equal(trustedCaller.rbacTrusted, true)
assert.equal(trustedCaller.projectRolesEnforced, false)

const envClient = createMboxClientFromEnv(
  {
    MBOX_API_URL: "http://env.example.test/",
    MBOX_TOKEN: "env-token",
    MBOX_API_TOKEN: "fallback-token",
    MBOX_REQUEST_ID: "env-request",
    MBOX_AUDIT_ACTOR: "env-actor",
    MBOX_AUDIT_SOURCE: "env-source",
  },
  {
    fetch: async (url, init) => {
      assert.equal(url, "http://env.example.test/v1/info")
      const headers = new Headers(init?.headers)
      assert.equal(headers.get("authorization"), "Bearer env-token")
      assert.equal(headers.get("x-mbox-request-id"), "env-request")
      assert.equal(headers.get("x-mbox-audit-actor"), "env-actor")
      assert.equal(headers.get("x-mbox-audit-source"), "env-source")
      return new Response(JSON.stringify(compatibleInfo), {
        status: 200,
        headers: { "content-type": "application/json" },
      })
    },
  },
)
assert.equal((await envClient.info()).name, "mbox")

const overrideEnvClient = createMboxClientFromEnv(
  {
    MBOX_API_URL: "http://env.example.test",
    MBOX_TOKEN: "env-token",
    MBOX_AUDIT_ACTOR: "env-actor",
  },
  {
    baseUrl: "http://override.example.test",
    token: "override-token",
    requestId: "override-request",
    auditActor: "override-actor",
    fetch: async (url, init) => {
      assert.equal(url, "http://override.example.test/v1/info")
      const headers = new Headers(init?.headers)
      assert.equal(headers.get("authorization"), "Bearer override-token")
      assert.equal(headers.get("x-mbox-request-id"), "override-request")
      assert.equal(headers.get("x-mbox-audit-actor"), "override-actor")
      return new Response(JSON.stringify(compatibleInfo), {
        status: 200,
        headers: { "content-type": "application/json" },
      })
    },
  },
)
assert.equal((await overrideEnvClient.info()).apiVersion, "v1alpha1")

const runtimeFilterCalls = []
const runtimeFilterClient = new MboxClient({
  baseUrl: "http://runtime.example.test",
  fetch: async (url) => {
    runtimeFilterCalls.push(new URL(url))
    if (runtimeFilterCalls.at(-1)?.pathname === "/v1/runtime/resources") {
      return jsonResponse({
        adapter: "agent-sandbox",
        checkedAt: "2026-01-01T00:00:00Z",
        summary: { total: 0, byKind: [], byNamespace: [], byOwner: [], byProject: [], workload: emptyWorkloadSummary() },
        items: [],
      })
    }
    return jsonResponse({
      adapter: "agent-sandbox",
      checkedAt: "2026-01-01T00:00:00Z",
      resourceCount: 0,
      orphanCount: 0,
      expectedClean: true,
      items: [],
    })
  },
})
await runtimeFilterClient.listRuntimeResources({
  namespace: "mbox-smoke",
  projectId: "project-1",
  kind: "SandboxClaim",
})
await runtimeFilterClient.listRuntimeOrphans({
  namespace: "mbox-smoke",
  projectId: "project-1",
  kind: "SandboxClaim",
})
assert.equal(runtimeFilterCalls[0].pathname, "/v1/runtime/resources")
assert.equal(runtimeFilterCalls[0].searchParams.get("namespace"), "mbox-smoke")
assert.equal(runtimeFilterCalls[0].searchParams.get("projectId"), "project-1")
assert.equal(runtimeFilterCalls[0].searchParams.get("kind"), "SandboxClaim")
assert.equal(runtimeFilterCalls[1].pathname, "/v1/runtime/orphans")
assert.equal(runtimeFilterCalls[1].searchParams.get("namespace"), "mbox-smoke")
assert.equal(runtimeFilterCalls[1].searchParams.get("projectId"), "project-1")
assert.equal(runtimeFilterCalls[1].searchParams.get("kind"), "SandboxClaim")

const memberCalls = []
const memberClient = new MboxClient({
  baseUrl: "http://members.example.test",
  fetch: async (url, init) => {
    const parsed = new URL(url)
    memberCalls.push(`${init?.method ?? "GET"} ${parsed.pathname}`)
    const body = init?.body ? JSON.parse(String(init.body)) : undefined
    switch (`${init?.method ?? "GET"} ${parsed.pathname}`) {
      case "GET /v1/projects/project-1/members":
        return jsonResponse({ items: [{ id: "member-1", projectId: "project-1", principalType: "user", principal: "alice@example.com", role: "operator" }] })
      case "GET /v1/projects/project-1/authorization":
        assert.ok(["sandbox.launch", "policy.manage", "member.manage"].includes(parsed.searchParams.get("action")))
        if (parsed.searchParams.get("action") === "member.manage") {
          return jsonResponse({
            projectId: "project-1",
            action: "member.manage",
            allowed: false,
            enforced: true,
            evaluation: "denied",
            requiredRoles: ["owner"],
            caller: {
              authenticated: true,
              authenticationRequired: false,
              mode: "trusted_header",
              principalType: "automation",
              principal: "sdk-bot",
              rbacTrusted: true,
              projectRolesEnforced: true,
              notes: [],
            },
            memberCount: 1,
            availableActions: ["project.view", "sandbox.launch", "policy.manage", "member.manage"],
            notes: ["caller matches a project member, but the role is insufficient for this action"],
          })
        }
        if (parsed.searchParams.get("action") === "policy.manage") {
          return jsonResponse({
            projectId: "project-1",
            action: "policy.manage",
            allowed: false,
            enforced: true,
            evaluation: "denied",
            requiredRoles: ["owner"],
            caller: {
              authenticated: true,
              authenticationRequired: false,
              mode: "trusted_header",
              principalType: "automation",
              principal: "sdk-bot",
              rbacTrusted: true,
              projectRolesEnforced: true,
              notes: [],
            },
            memberCount: 1,
            availableActions: ["project.view", "sandbox.launch", "policy.manage"],
            notes: ["caller matches a project member, but the role is insufficient for this action"],
          })
        }
        return jsonResponse({
          projectId: "project-1",
          action: "sandbox.launch",
          allowed: false,
          enforced: false,
          evaluation: "not_enforceable",
          requiredRoles: ["owner", "operator"],
          caller: {
            authenticated: false,
            authenticationRequired: false,
            mode: "anonymous",
            principalType: "anonymous",
            principal: "anonymous",
            rbacTrusted: false,
            projectRolesEnforced: false,
            notes: [],
          },
          memberCount: 1,
          availableActions: ["project.view", "sandbox.launch"],
          notes: ["route-level project RBAC is not enforced yet"],
        })
      case "POST /v1/projects/project-1/members":
        assert.equal(body.principalType, "automation")
        assert.equal(body.principal, "sdk-bot")
        assert.equal(body.role, "viewer")
        return jsonResponse({ id: "member-2", projectId: "project-1", ...body }, 201)
      case "GET /v1/members/member-1":
        return jsonResponse({ id: "member-1", projectId: "project-1", principalType: "user", principal: "alice@example.com", role: "operator" })
      case "DELETE /v1/members/member-1":
        return new Response(null, { status: 204 })
      default:
        throw new Error(`unexpected member request ${init?.method ?? "GET"} ${parsed.pathname}`)
    }
  },
})
assert.equal((await memberClient.listProjectMembers("project-1")).items[0].principal, "alice@example.com")
assert.equal((await memberClient.getProjectAuthorization("project-1", { action: "sandbox.launch" })).evaluation, "not_enforceable")
assert.equal((await memberClient.getProjectAuthorization("project-1", { action: "policy.manage" })).enforced, true)
assert.equal((await memberClient.getProjectAuthorization("project-1", { action: "member.manage" })).enforced, true)
assert.equal((await memberClient.createProjectMember("project-1", { principalType: "automation", principal: "sdk-bot", role: "viewer" })).role, "viewer")
assert.equal((await memberClient.getProjectMember("member-1")).role, "operator")
await memberClient.deleteProjectMember("member-1")
assert.deepEqual(memberCalls, [
  "GET /v1/projects/project-1/members",
  "GET /v1/projects/project-1/authorization",
  "GET /v1/projects/project-1/authorization",
  "GET /v1/projects/project-1/authorization",
  "POST /v1/projects/project-1/members",
  "GET /v1/members/member-1",
  "DELETE /v1/members/member-1",
])

const failedTask = {
  id: "task-failed",
  projectId: "project-1",
  sandboxId: "sandbox-1",
  status: "failed",
  command: ["sh", "-lc", "exit 2"],
  timeoutSeconds: 60,
  stdout: "",
  stderr: "failed",
  outputTruncated: false,
  exitCode: 2,
}
const failedTaskClient = new MboxClient({
  baseUrl: "http://tasks.example.test",
  fetch: async (url) => {
    assert.equal(new URL(url).pathname, "/v1/tasks/task-failed")
    return jsonResponse(failedTask)
  },
})
assert.equal((await failedTaskClient.waitForTask("task-failed")).status, "failed")
await assert.rejects(
  () => failedTaskClient.waitForTask("task-failed", { requireSuccess: true }),
  (error) =>
    error instanceof MboxTaskStatusError &&
    error.task.id === "task-failed" &&
    error.task.status === "failed" &&
    error.message === "task task-failed finished with status failed",
)

for (const status of ["canceled", "timed_out"]) {
  const terminalTaskClient = new MboxClient({
    baseUrl: "http://tasks.example.test",
    fetch: async () =>
      jsonResponse({
        ...failedTask,
        id: `task-${status}`,
        status,
      }),
  })
  await assert.rejects(
    () => terminalTaskClient.waitForTask(`task-${status}`, { requireSuccess: true }),
    (error) => error instanceof MboxTaskStatusError && error.task.status === status,
  )
}

const succeededTaskClient = new MboxClient({
  baseUrl: "http://tasks.example.test",
  fetch: async () =>
    jsonResponse({
      ...failedTask,
      id: "task-succeeded",
      status: "succeeded",
      stderr: "",
      exitCode: 0,
    }),
})
assert.equal(
  (await succeededTaskClient.waitForTask("task-succeeded", { requireSuccess: true })).status,
  "succeeded",
)

let sandboxPolls = 0
const sandboxWaitClient = new MboxClient({
  baseUrl: "http://sandboxes.example.test",
  fetch: async (url) => {
    assert.equal(new URL(url).pathname, "/v1/sandboxes/sandbox-1")
    sandboxPolls += 1
    if (sandboxPolls === 1) {
      return jsonResponse({
        id: "sandbox-1",
        projectId: "project-1",
        name: "Smoke",
        slug: "smoke",
        namespace: "mbox-smoke",
        serviceAccountName: "mbox-sandbox",
        status: "pending",
      })
    }
    return jsonResponse({
      id: "sandbox-1",
      projectId: "project-1",
      name: "Smoke",
      slug: "smoke",
      namespace: "mbox-smoke",
      serviceAccountName: "mbox-sandbox",
      status: "running",
      runtimeRef: {
        adapter: "agent-sandbox",
        kind: "SandboxClaim",
        namespace: "mbox-smoke",
        name: "claim-1",
      },
    })
  },
})
assert.equal(
  (await sandboxWaitClient.waitForSandbox("sandbox-1", {
    intervalMs: 1,
    timeoutMs: 1000,
    requireRuntimeRef: true,
  })).runtimeRef.name,
  "claim-1",
)
assert.equal(sandboxPolls, 2)

const failedSandboxClient = new MboxClient({
  baseUrl: "http://sandboxes.example.test",
  fetch: async () =>
    jsonResponse({
      id: "sandbox-failed",
      projectId: "project-1",
      name: "Failed",
      slug: "failed",
      namespace: "mbox-smoke",
      serviceAccountName: "mbox-sandbox",
      status: "failed",
    }),
})
await assert.rejects(
  () => failedSandboxClient.waitForSandbox("sandbox-failed", { status: "running" }),
  (error) =>
    error instanceof MboxSandboxStatusError &&
    error.sandbox.id === "sandbox-failed" &&
    error.expectedStatus === "running",
)

const missingRuntimeRefClient = new MboxClient({
  baseUrl: "http://sandboxes.example.test",
  fetch: async () =>
    jsonResponse({
      id: "sandbox-running",
      projectId: "project-1",
      name: "Running",
      slug: "running",
      namespace: "mbox-smoke",
      serviceAccountName: "mbox-sandbox",
      status: "running",
    }),
})
await assert.rejects(
  () =>
    missingRuntimeRefClient.waitForSandbox("sandbox-running", {
      requireRuntimeRef: true,
      intervalMs: 1,
      timeoutMs: 1,
    }),
  (error) =>
    error instanceof MboxSandboxRuntimeRefError &&
    error.sandbox.id === "sandbox-running",
)

let validationTaskPolls = 0
const validationCalls = []
const templateValidationClient = new MboxClient({
  baseUrl: "http://validation.example.test",
  fetch: async (url, init) => {
    const parsed = new URL(url)
    validationCalls.push(`${init?.method ?? "GET"} ${parsed.pathname}`)
    const body = init?.body ? JSON.parse(String(init.body)) : undefined
    switch (`${init?.method ?? "GET"} ${parsed.pathname}`) {
      case "POST /v1/templates/template-1/validation-runs":
        assert.equal(new Headers(init?.headers).get("x-mbox-principal"), "validation-bot")
        assert.equal(body.projectId, "project-1")
        assert.equal(body.metadata.source, "sdk-smoke")
        return jsonResponse({
          template: { id: "template-1", name: "Template" },
          sandbox: { id: "sandbox-1", status: "pending" },
        }, 201)
      case "GET /v1/sandboxes/sandbox-1":
        return jsonResponse({
          id: "sandbox-1",
          projectId: "project-1",
          name: "Validation",
          slug: "validation",
          namespace: "mbox-smoke",
          serviceAccountName: "mbox-sandbox",
          status: "running",
          runtimeRef: {
            adapter: "agent-sandbox",
            kind: "SandboxClaim",
            namespace: "mbox-smoke",
            name: "claim-1",
          },
        })
      case "POST /v1/sandboxes/sandbox-1/tasks":
        assert.equal(new Headers(init?.headers).get("x-mbox-principal"), "validation-bot")
        assert.deepEqual(body.command, ["sh", "-lc", "echo ok"])
        assert.equal(body.timeoutSeconds, 30)
        assert.equal(body.metadata.source, "sdk-smoke")
        return jsonResponse({ id: "task-1", status: "queued" }, 201)
      case "GET /v1/tasks/task-1":
        validationTaskPolls += 1
        if (validationTaskPolls === 1) {
          return jsonResponse({ id: "task-1", status: "running" })
        }
        return jsonResponse({
          id: "task-1",
          projectId: "project-1",
          sandboxId: "sandbox-1",
          command: ["sh", "-lc", "echo ok"],
          timeoutSeconds: 30,
          status: "succeeded",
          stdout: "ok",
          stderr: "",
          outputTruncated: false,
        })
      case "POST /v1/templates/template-1/validation-runs/sandbox-1/decision":
        assert.equal(new Headers(init?.headers).get("x-mbox-principal"), "validation-bot")
        assert.equal(body.status, "passed")
        return jsonResponse({
          template: { id: "template-1", metadata: { validationStatus: "passed" } },
          sandbox: { id: "sandbox-1", metadata: { validationResult: "passed" } },
        })
      default:
        throw new Error(`unexpected validation request ${init?.method ?? "GET"} ${parsed.pathname}`)
    }
  },
})
const validationResult = await templateValidationClient.runTemplateValidation("template-1", {
  projectId: "project-1",
  validationMetadata: { source: "sdk-smoke" },
  task: {
    command: ["sh", "-lc", "echo ok"],
    timeoutSeconds: 30,
    metadata: { source: "sdk-smoke" },
  },
  intervalMs: 1,
  timeoutMs: 1000,
  requireSuccess: true,
  headers: {
    "X-Mbox-Principal": "validation-bot",
    "X-Mbox-Principal-Type": "automation",
  },
})
assert.equal(validationResult.status, "passed")
assert.equal(validationResult.decisionStatus, "passed")
assert.equal(validationResult.task.status, "succeeded")
assert.deepEqual(validationCalls, [
  "POST /v1/templates/template-1/validation-runs",
  "GET /v1/sandboxes/sandbox-1",
  "POST /v1/sandboxes/sandbox-1/tasks",
  "GET /v1/tasks/task-1",
  "GET /v1/tasks/task-1",
  "POST /v1/templates/template-1/validation-runs/sandbox-1/decision",
])

const failedValidationResultClient = new MboxClient({
  baseUrl: "http://validation.example.test",
  fetch: async (url, init) => {
    const parsed = new URL(url)
    const body = init?.body ? JSON.parse(String(init.body)) : undefined
    switch (`${init?.method ?? "GET"} ${parsed.pathname}`) {
      case "POST /v1/templates/template-1/validation-runs":
        return jsonResponse({
          template: { id: "template-1", name: "Template" },
          sandbox: { id: "sandbox-1", status: "pending" },
        }, 201)
      case "GET /v1/sandboxes/sandbox-1":
        return jsonResponse({
          id: "sandbox-1",
          projectId: "project-1",
          name: "Validation",
          slug: "validation",
          namespace: "mbox-smoke",
          serviceAccountName: "mbox-sandbox",
          status: "running",
          runtimeRef: {
            adapter: "agent-sandbox",
            kind: "SandboxClaim",
            namespace: "mbox-smoke",
            name: "claim-1",
          },
        })
      case "POST /v1/sandboxes/sandbox-1/tasks":
        return jsonResponse({ id: "task-1", status: "queued" }, 201)
      case "GET /v1/tasks/task-1":
        return jsonResponse({
          id: "task-1",
          projectId: "project-1",
          sandboxId: "sandbox-1",
          command: ["sh", "-lc", "exit 7"],
          timeoutSeconds: 30,
          status: "failed",
          stdout: "",
          stderr: "failed",
          outputTruncated: false,
          exitCode: 7,
        })
      case "POST /v1/templates/template-1/validation-runs/sandbox-1/decision":
        assert.equal(body.status, "failed")
        return jsonResponse({
          template: { id: "template-1", metadata: { validationStatus: "failed" } },
          sandbox: { id: "sandbox-1", metadata: { validationResult: "failed" } },
        })
      default:
        throw new Error(`unexpected failed validation result request ${init?.method ?? "GET"} ${parsed.pathname}`)
    }
  },
})
const failedValidationResult = await failedValidationResultClient.runTemplateValidation("template-1", {
  task: { command: ["sh", "-lc", "exit 7"], timeoutSeconds: 30 },
  intervalMs: 1,
  timeoutMs: 1000,
})
assert.equal(failedValidationResult.status, "failed")
assert.equal(failedValidationResult.task.status, "failed")

const failedValidationClient = new MboxClient({
  baseUrl: "http://validation.example.test",
  fetch: async (url, init) => {
    const parsed = new URL(url)
    const body = init?.body ? JSON.parse(String(init.body)) : undefined
    switch (`${init?.method ?? "GET"} ${parsed.pathname}`) {
      case "POST /v1/templates/template-1/validation-runs":
        return jsonResponse({
          template: { id: "template-1", name: "Template" },
          sandbox: { id: "sandbox-1", status: "pending" },
        }, 201)
      case "GET /v1/sandboxes/sandbox-1":
        return jsonResponse({
          id: "sandbox-1",
          projectId: "project-1",
          name: "Validation",
          slug: "validation",
          namespace: "mbox-smoke",
          serviceAccountName: "mbox-sandbox",
          status: "running",
          runtimeRef: {
            adapter: "agent-sandbox",
            kind: "SandboxClaim",
            namespace: "mbox-smoke",
            name: "claim-1",
          },
        })
      case "POST /v1/sandboxes/sandbox-1/tasks":
        return jsonResponse({ id: "task-1", status: "queued" }, 201)
      case "GET /v1/tasks/task-1":
        return jsonResponse({
          id: "task-1",
          projectId: "project-1",
          sandboxId: "sandbox-1",
          command: ["sh", "-lc", "exit 7"],
          timeoutSeconds: 30,
          status: "failed",
          stdout: "",
          stderr: "failed",
          outputTruncated: false,
          exitCode: 7,
        })
      case "POST /v1/templates/template-1/validation-runs/sandbox-1/decision":
        assert.equal(body.status, "failed")
        return jsonResponse({
          template: { id: "template-1", metadata: { validationStatus: "failed" } },
          sandbox: { id: "sandbox-1", metadata: { validationResult: "failed" } },
        })
      default:
        throw new Error(`unexpected failed validation request ${init?.method ?? "GET"} ${parsed.pathname}`)
    }
  },
})
await assert.rejects(
  () =>
    failedValidationClient.runTemplateValidation("template-1", {
      task: { command: ["sh", "-lc", "exit 7"], timeoutSeconds: 30 },
      intervalMs: 1,
      timeoutMs: 1000,
      requireSuccess: true,
    }),
  (error) =>
    error instanceof MboxTemplateValidationRunError &&
    error.result.status === "failed" &&
    error.result.task.status === "failed" &&
    error.cause instanceof MboxTaskStatusError,
)

assert.equal(assertOpenAPIAlignment(buildOpenAPI()).ok, true)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    delete broken.paths["/v1/projects"].post.responses["201"].content["application/json"].schema.$ref
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some((issue) => issue.reason === "response-schema-mismatch"),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.paths["/v1/projects/{projectID}/published-only"] = {
      get: op(jsonRef("Project")),
    }
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-sdk-route-coverage" &&
        issue.method === "GET" &&
        issue.path === "/v1/projects/{projectID}/published-only",
      ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.paths["/v1/projects"].get.responses["200"].content["application/json"].schema.required = []
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-response-required" &&
        issue.sdk === "listProjects" &&
        issue.property === "items",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    delete broken.components.schemas.PolicyDeniedAuditMetadata.properties.policyKind
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-property" &&
        issue.schema === "PolicyDeniedAuditMetadata" &&
        issue.property === "policyKind",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    delete broken.components.schemas.PolicyDeniedAuditMetadata.properties.role
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-property" &&
        issue.schema === "PolicyDeniedAuditMetadata" &&
        issue.property === "role",
      ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.PolicyDeniedAuditMetadata.properties.operation.enum =
      broken.components.schemas.PolicyDeniedAuditMetadata.properties.operation.enum.filter((value) => value !== "sandbox.launch")
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-enum-value" &&
        issue.schema === "PolicyDeniedAuditMetadata" &&
        issue.property === "operation" &&
        issue.enumValue === "sandbox.launch",
      ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.PolicyDeniedAuditMetadata.properties.policyKind.enum =
      broken.components.schemas.PolicyDeniedAuditMetadata.properties.policyKind.enum.filter((value) => value !== "quota")
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-enum-value" &&
        issue.schema === "PolicyDeniedAuditMetadata" &&
        issue.property === "policyKind" &&
        issue.enumValue === "quota",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.PolicyDeniedAuditMetadata.properties.enforcement.enum =
      broken.components.schemas.PolicyDeniedAuditMetadata.properties.enforcement.enum.filter((value) => value !== "enforced")
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-enum-value" &&
        issue.schema === "PolicyDeniedAuditMetadata" &&
        issue.property === "enforcement" &&
        issue.enumValue === "enforced",
      ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.PolicyDeniedAuditMetadata.properties.incomingBytes.type = "string"
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "schema-property-type-mismatch" &&
        issue.schema === "PolicyDeniedAuditMetadata" &&
        issue.property === "incomingBytes" &&
        issue.expectedType === "integer" &&
        issue.actualType === "string",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.ProjectQuotaPolicy.required = ["enforcement"]
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-required" &&
        issue.schema === "ProjectQuotaPolicy" &&
        issue.property === "projectId",
      ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.ProjectQuotaPolicyUpsert.properties.maxRetainedArtifactBytes.type = "string"
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "schema-property-type-mismatch" &&
        issue.schema === "ProjectQuotaPolicyUpsert" &&
        issue.property === "maxRetainedArtifactBytes" &&
        issue.expectedType === "integer" &&
        issue.actualType === "string",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    delete broken.components.schemas.ProjectCredential.properties.secretRef
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-property" &&
        issue.schema === "ProjectCredential" &&
        issue.property === "secretRef",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.TemplateCreate.required = ["name"]
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-required" &&
        issue.schema === "TemplateCreate" &&
        issue.property === "image",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.TemplateUpdate.properties.projectId = { type: "string" }
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "unexpected-schema-property" &&
        issue.schema === "TemplateUpdate" &&
        issue.property === "projectId",
      ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.PolicyDeniedAuditMetadata.properties.authorizationAction.enum =
      broken.components.schemas.PolicyDeniedAuditMetadata.properties.authorizationAction.enum.filter((value) => value !== "member.manage")
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-enum-value" &&
        issue.schema === "PolicyDeniedAuditMetadata" &&
        issue.property === "authorizationAction" &&
        issue.enumValue === "member.manage",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.PolicyDeniedAuditMetadata.properties.callerMode.enum = ["anonymous", "shared_token"]
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-enum-value" &&
        issue.schema === "PolicyDeniedAuditMetadata" &&
        issue.property === "callerMode" &&
        issue.enumValue === "trusted_header",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.PolicyDeniedAuditMetadata.properties.callerPrincipalType.enum = [
      "anonymous",
      "shared_token",
      "user",
      "service_account",
    ]
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-enum-value" &&
        issue.schema === "PolicyDeniedAuditMetadata" &&
        issue.property === "callerPrincipalType" &&
        issue.enumValue === "automation",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.PolicyDeniedAuditMetadata.properties.principalType.enum = ["user", "service_account"]
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-enum-value" &&
        issue.schema === "PolicyDeniedAuditMetadata" &&
        issue.property === "principalType" &&
        issue.enumValue === "automation",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.PolicyDeniedAuditMetadata.properties.role.enum = ["owner", "operator"]
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-enum-value" &&
        issue.schema === "PolicyDeniedAuditMetadata" &&
        issue.property === "role" &&
        issue.enumValue === "viewer",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.Sandbox.required = broken.components.schemas.Sandbox.required.filter((name) => name !== "status")
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-required" &&
        issue.schema === "Sandbox" &&
        issue.property === "status",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.SandboxUpdate.properties.slug = { type: "string" }
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "unexpected-schema-property" &&
        issue.schema === "SandboxUpdate" &&
        issue.property === "slug",
      ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.RuntimeSession.required = broken.components.schemas.RuntimeSession.required.filter(
      (name) => name !== "startedAt",
    )
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-required" &&
        issue.schema === "RuntimeSession" &&
        issue.property === "startedAt",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.RuntimeSessionCreate.required = []
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-required" &&
        issue.schema === "RuntimeSessionCreate" &&
        issue.property === "type",
      ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.LogResult.required = broken.components.schemas.LogResult.required.filter(
      (name) => name !== "logs",
    )
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-required" &&
        issue.schema === "LogResult" &&
        issue.property === "logs",
      ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.RuntimeTarget.properties.storage.items = { type: "object" }
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "schema-array-item-ref-mismatch" &&
        issue.schema === "RuntimeTarget" &&
        issue.property === "storage" &&
        issue.expectedSchema === "RuntimeStorage",
      ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.RuntimeOrphan.properties.resource = { type: "object" }
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "schema-property-ref-mismatch" &&
        issue.schema === "RuntimeOrphan" &&
        issue.property === "resource" &&
        issue.expectedSchema === "RuntimeResource",
      ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.ProjectUsage.properties.sandboxes = { type: "object" }
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "schema-property-ref-mismatch" &&
        issue.schema === "ProjectUsage" &&
        issue.property === "sandboxes" &&
        issue.expectedSchema === "ProjectSandboxUsage",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.ProjectSandboxUsage.properties.runningRequests = { type: "object" }
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "schema-property-ref-mismatch" &&
        issue.schema === "ProjectSandboxUsage" &&
        issue.property === "runningRequests" &&
        issue.expectedSchema === "SandboxResourceRequestUsage",
      ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.ProjectSessionUsage.properties.active.type = "string"
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "schema-property-type-mismatch" &&
        issue.schema === "ProjectSessionUsage" &&
        issue.property === "active" &&
        issue.expectedType === "integer" &&
        issue.actualType === "string",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.SandboxResourceRequestUsage.properties.storage = { type: "object" }
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "schema-property-ref-mismatch" &&
        issue.schema === "SandboxResourceRequestUsage" &&
        issue.property === "storage" &&
        issue.expectedSchema === "ResourceQuantityUsage",
      ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.ProjectTemplateUsage.properties.cpuRequests.items = { type: "object" }
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "schema-array-item-ref-mismatch" &&
        issue.schema === "ProjectTemplateUsage" &&
        issue.property === "cpuRequests" &&
        issue.expectedSchema === "ResourceUsageValue",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.RuntimeResourceSummary.properties.byProject.items = { type: "object" }
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "schema-array-item-ref-mismatch" &&
        issue.schema === "RuntimeResourceSummary" &&
        issue.property === "byProject" &&
        issue.expectedSchema === "RuntimeResourceCount",
      ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.RuntimeResourceCount.properties.count.type = "string"
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "schema-property-type-mismatch" &&
        issue.schema === "RuntimeResourceCount" &&
        issue.property === "count" &&
        issue.expectedType === "integer" &&
        issue.actualType === "string",
      ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.RuntimeWorkloadSummary.properties.runningPods.type = "string"
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "schema-property-type-mismatch" &&
        issue.schema === "RuntimeWorkloadSummary" &&
        issue.property === "runningPods" &&
        issue.expectedType === "integer" &&
        issue.actualType === "string",
      ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.RuntimeResourceObservation.properties.podCount.type = "string"
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "schema-property-type-mismatch" &&
        issue.schema === "RuntimeResourceObservation" &&
        issue.property === "podCount" &&
        issue.expectedType === "integer" &&
        issue.actualType === "string",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.RuntimeOrphanReason.enum = [
      "missing-sandbox-record",
      "cleanup-pending",
      "missing-template-record",
      "unlabeled-owner",
    ]
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-enum-value" &&
        issue.schema === "RuntimeOrphan" &&
        issue.property === "reason" &&
        issue.enumValue === "runtime-ref-mismatch",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.ExecutionTask.required = broken.components.schemas.ExecutionTask.required.filter(
      (name) => name !== "outputTruncated",
    )
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-required" &&
        issue.schema === "ExecutionTask" &&
        issue.property === "outputTruncated",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.ExecutionTaskCreate.required = []
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-required" &&
        issue.schema === "ExecutionTaskCreate" &&
        issue.property === "command",
      ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.Artifact.required = broken.components.schemas.Artifact.required.filter((name) => name !== "uri")
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-required" &&
        issue.schema === "Artifact" &&
        issue.property === "uri",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.ArtifactContent.required = broken.components.schemas.ArtifactContent.required.filter(
      (name) => name !== "storageProvider",
    )
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-required" &&
        issue.schema === "ArtifactContent" &&
        issue.property === "storageProvider",
      ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.ArtifactContent.properties.sizeBytes.type = "string"
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "schema-property-type-mismatch" &&
        issue.schema === "ArtifactContent" &&
        issue.property === "sizeBytes" &&
        issue.expectedType === "number" &&
        issue.actualType === "string",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.Project.required = broken.components.schemas.Project.required.filter(
      (name) => name !== "defaultNamespace",
    )
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-required" &&
        issue.schema === "Project" &&
        issue.property === "defaultNamespace",
      ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.Compatibility.required = broken.components.schemas.Compatibility.required.filter(
      (name) => name !== "minimumSdkApiVersion",
    )
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-required" &&
        issue.schema === "Compatibility" &&
        issue.property === "minimumSdkApiVersion",
      ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.APIInfo.properties.runtimeAccess = { type: "object" }
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "schema-property-ref-mismatch" &&
        issue.schema === "APIInfo" &&
        issue.property === "runtimeAccess" &&
        issue.expectedSchema === "RuntimeInfo",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.CallerInfo.properties.mode.enum = ["anonymous", "shared_token"]
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-enum-value" &&
        issue.schema === "CallerInfo" &&
        issue.property === "mode" &&
        issue.enumValue === "trusted_header",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.ProjectAuthorizationDecision.properties.caller = { type: "object" }
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "schema-property-ref-mismatch" &&
        issue.schema === "ProjectAuthorizationDecision" &&
        issue.property === "caller" &&
        issue.expectedSchema === "CallerInfo",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.ProjectAuthorizationDecision.properties.action.enum = [
      "project.view",
      "project.manage",
      "sandbox.launch",
      "runtime.operate",
      "artifact.write",
      "policy.manage",
      "credential.manage",
    ]
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-enum-value" &&
        issue.schema === "ProjectAuthorizationDecision" &&
        issue.property === "action" &&
        issue.enumValue === "member.manage",
      ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.BoundarySummary.properties.checks.items = { type: "object" }
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "schema-array-item-ref-mismatch" &&
        issue.schema === "BoundarySummary" &&
        issue.property === "checks" &&
        issue.expectedSchema === "BoundaryCheck",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.ProjectPolicy.properties.enforcement.enum = ["disabled"]
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-enum-value" &&
        issue.schema === "ProjectPolicy" &&
        issue.property === "enforcement" &&
        issue.enumValue === "enforced",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.ProjectMemberCreate.properties.principalType.enum = ["user", "service_account"]
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-enum-value" &&
        issue.schema === "ProjectMemberCreate" &&
        issue.property === "principalType" &&
        issue.enumValue === "automation",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.ProjectMember.properties.role.enum = ["owner", "operator"]
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-enum-value" &&
        issue.schema === "ProjectMember" &&
        issue.property === "role" &&
        issue.enumValue === "viewer",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.ProjectCredential.properties.secretRef = { type: "object" }
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "schema-property-ref-mismatch" &&
        issue.schema === "ProjectCredential" &&
        issue.property === "secretRef" &&
        issue.expectedSchema === "SecretRef",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.ProjectCredentialCreate.properties.type.enum = ["git", "registry", "ssh", "generic"]
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-enum-value" &&
        issue.schema === "ProjectCredentialCreate" &&
        issue.property === "type" &&
        issue.enumValue === "kubernetes",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.TemplateCreate.properties.exposedPorts.items = { type: "object" }
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "schema-array-item-ref-mismatch" &&
        issue.schema === "TemplateCreate" &&
        issue.property === "exposedPorts" &&
        issue.expectedSchema === "TemplatePort",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.TemplateValidationRun.properties.template = { type: "object" }
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "schema-property-ref-mismatch" &&
        issue.schema === "TemplateValidationRun" &&
        issue.property === "template" &&
        issue.expectedSchema === "EnvironmentTemplate",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.TemplateValidationRunDecision.properties.status.enum = ["passed"]
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-enum-value" &&
        issue.schema === "TemplateValidationRunDecision" &&
        issue.property === "status" &&
        issue.enumValue === "failed",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.Sandbox.properties.runtimeRef = { type: "object" }
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "schema-property-ref-mismatch" &&
        issue.schema === "Sandbox" &&
        issue.property === "runtimeRef" &&
        issue.expectedSchema === "RuntimeRef",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.SandboxUpdate.properties.ports.items = { type: "object" }
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "schema-array-item-ref-mismatch" &&
        issue.schema === "SandboxUpdate" &&
        issue.property === "ports" &&
        issue.expectedSchema === "SandboxPort",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.Sandbox.properties.status.enum = ["pending", "running", "failed", "deleted"]
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-enum-value" &&
        issue.schema === "Sandbox" &&
        issue.property === "status" &&
        issue.enumValue === "stopped",
      ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.PreviewPortsResult.properties.target = { type: "object" }
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "schema-property-ref-mismatch" &&
        issue.schema === "PreviewPortsResult" &&
        issue.property === "target" &&
        issue.expectedSchema === "RuntimeTarget",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.RuntimeSessionCreate.properties.type.enum = ["terminal", "ide", "notebook", "browser", "command"]
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-enum-value" &&
        issue.schema === "RuntimeSessionCreate" &&
        issue.property === "type" &&
        issue.enumValue === "custom",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.RuntimeSession.properties.runtimeRef = { type: "object" }
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "schema-property-ref-mismatch" &&
        issue.schema === "RuntimeSession" &&
        issue.property === "runtimeRef" &&
        issue.expectedSchema === "RuntimeRef",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.ExecutionTask.properties.runtimeRef = { type: "object" }
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "schema-property-ref-mismatch" &&
        issue.schema === "ExecutionTask" &&
        issue.property === "runtimeRef" &&
        issue.expectedSchema === "RuntimeRef",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.ExecutionTask.properties.status.enum = ["queued", "running", "succeeded", "failed", "canceled"]
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-enum-value" &&
        issue.schema === "ExecutionTask" &&
        issue.property === "status" &&
        issue.enumValue === "timed_out",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.ExecutionTaskEvent.properties.stream.enum = ["stdout"]
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-enum-value" &&
        issue.schema === "ExecutionTaskEvent" &&
        issue.property === "stream" &&
        issue.enumValue === "stderr",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.Artifact.properties.retainedContent = { type: "object" }
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "schema-property-ref-mismatch" &&
        issue.schema === "Artifact" &&
        issue.property === "retainedContent" &&
        issue.expectedSchema === "ArtifactContent",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.ArtifactCreate.properties.kind.enum = ["file", "directory", "log", "report", "image", "link", "other"]
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-enum-value" &&
        issue.schema === "ArtifactCreate" &&
        issue.property === "kind" &&
        issue.enumValue === "screenshot",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.ArtifactContent.properties.storageProvider.enum = ["postgres", "filesystem"]
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-enum-value" &&
        issue.schema === "ArtifactContent" &&
        issue.property === "storageProvider" &&
        issue.enumValue === "s3",
      ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.AuditEvent.properties.metadata = { type: "object" }
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "schema-property-ref-mismatch" &&
        issue.schema === "AuditEvent" &&
        issue.property === "metadata" &&
        issue.expectedSchema === "AuditEventMetadata",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.AuditEvent.properties.action = { type: "string" }
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "schema-property-ref-mismatch" &&
        issue.schema === "AuditEvent" &&
        issue.property === "action" &&
        issue.expectedSchema === "AuditEventAction",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.RuntimeOrphanReason.enum = [
      "missing-sandbox-record",
      "cleanup-pending",
      "missing-template-record",
      "unlabeled-owner",
    ]
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-enum-value" &&
        issue.schema === "RuntimeOrphan" &&
        issue.property === "reason" &&
        issue.enumValue === "runtime-ref-mismatch",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.RuntimeResourceOwner.properties.kind.enum = ["sandbox"]
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-enum-value" &&
        issue.schema === "RuntimeResourceOwner" &&
        issue.property === "kind" &&
        issue.enumValue === "template",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.RuntimeOrphanCleanupRequest.properties.confirm.enum = ["delete-runtime-resource"]
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-enum-value" &&
        issue.schema === "RuntimeOrphanCleanupRequest" &&
        issue.property === "confirm" &&
        issue.enumValue === "delete-orphan-runtime-resource",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.RuntimeOrphan.properties.status.enum = ["pending", "running", "failed", "deleted"]
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-enum-value" &&
        issue.schema === "RuntimeOrphan" &&
        issue.property === "status" &&
        issue.enumValue === "stopped",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.ProjectUpdate.properties.slug = { type: "string" }
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "unexpected-schema-property" &&
        issue.schema === "ProjectUpdate" &&
        issue.property === "slug",
    ),
)
assert.throws(
  () => {
    const broken = buildOpenAPI()
    broken.components.schemas.Error.required = []
    assertOpenAPIAlignment(broken)
  },
  (error) =>
    error instanceof OpenAPIAlignmentError &&
    error.result.missing.some(
      (issue) =>
        issue.reason === "missing-schema-required" &&
        issue.schema === "Error" &&
        issue.property === "error",
    ),
)
assert.equal(assertOpenAPIAlignment(buildOpenAPIWithIntentionalSDKExceptions()).ok, true)

console.log("SDK smoke passed")

function jsonFetch(payload) {
  return async (url) => {
    assert.equal(new URL(url).pathname, "/v1/info")
    return jsonResponse(payload)
  }
}

function jsonResponse(payload, status = 200) {
  return new Response(JSON.stringify(payload), {
    status,
    headers: { "content-type": "application/json" },
  })
}

function emptyWorkloadSummary() {
  return {
    observedResources: 0,
    desiredPods: 0,
    observedPods: 0,
    runningPods: 0,
    containersReady: 0,
    containersTotal: 0,
    restartCount: 0,
  }
}

function buildOpenAPI() {
  const paths = {
    "/healthz": { get: op(jsonRef("Health"), { auth: "none" }) },
    "/v1/info": { get: op(jsonRef("APIInfo"), { auth: "none" }) },
    "/v1/auth/caller": { get: op(jsonRef("CallerInfo")) },
    "/v1/openapi.json": { get: op({ type: "object" }) },
    "/v1/runtime/resources": {
      get: op(jsonRef("RuntimeResourceList"), { parameters: [queryParam("namespace"), queryParam("projectId"), queryParam("kind")] }),
    },
    "/v1/runtime/orphans": {
      get: op(jsonRef("RuntimeOrphanAudit"), { parameters: [queryParam("namespace"), queryParam("projectId"), queryParam("kind")] }),
    },
    "/v1/runtime/orphans/cleanup": {
      post: op(jsonRef("RuntimeOrphanCleanupResult"), {
        request: jsonRef("RuntimeOrphanCleanupRequest"),
      }),
    },
    "/v1/audit-events": {
      get: op(listSchema("AuditEvent"), {
        parameters: auditParams(true),
      }),
    },
    "/v1/projects": {
      get: op(listSchema("Project")),
      post: op(jsonRef("Project"), { status: "201", request: jsonRef("ProjectCreate") }),
    },
    "/v1/projects/{projectID}": {
      get: op(jsonRef("Project")),
      patch: op(jsonRef("Project"), { request: jsonRef("ProjectUpdate") }),
      delete: noContentOp(),
    },
    "/v1/projects/{projectID}/policy": {
      get: op(jsonRef("ProjectPolicy")),
      put: op(jsonRef("ProjectPolicy"), { request: jsonRef("ProjectPolicyUpsert") }),
    },
    "/v1/projects/{projectID}/quota-policy": {
      get: op(jsonRef("ProjectQuotaPolicy")),
      put: op(jsonRef("ProjectQuotaPolicy"), { request: jsonRef("ProjectQuotaPolicyUpsert") }),
    },
    "/v1/projects/{projectID}/authorization": {
      get: op(jsonRef("ProjectAuthorizationDecision"), { parameters: [queryParam("action")] }),
    },
    "/v1/projects/{projectID}/members": {
      get: op(listSchema("ProjectMember")),
      post: op(jsonRef("ProjectMember"), { status: "201", request: jsonRef("ProjectMemberCreate") }),
    },
    "/v1/projects/{projectID}/credentials": {
      get: op(listSchema("ProjectCredential")),
      post: op(jsonRef("ProjectCredential"), { status: "201", request: jsonRef("ProjectCredentialCreate") }),
    },
    "/v1/projects/{projectID}/usage": { get: op(jsonRef("ProjectUsage")) },
    "/v1/projects/{projectID}/audit-events": {
      get: op(listSchema("AuditEvent"), {
        parameters: auditParams(false),
      }),
    },
    "/v1/members/{memberID}": {
      get: op(jsonRef("ProjectMember")),
      delete: noContentOp(),
    },
    "/v1/credentials/{credentialID}": {
      get: op(jsonRef("ProjectCredential")),
      delete: noContentOp(),
    },
    "/v1/templates": {
      get: op(listSchema("EnvironmentTemplate"), { parameters: [queryParam("projectId")] }),
      post: op(jsonRef("EnvironmentTemplate"), { status: "201", request: jsonRef("TemplateCreate") }),
    },
    "/v1/templates/{templateID}": {
      get: op(jsonRef("EnvironmentTemplate")),
      patch: op(jsonRef("EnvironmentTemplate"), { request: jsonRef("TemplateUpdate") }),
      delete: noContentOp(),
    },
    "/v1/templates/{templateID}/boundary": {
      get: op(jsonRef("BoundarySummary"), { parameters: [queryParam("projectId")] }),
    },
    "/v1/templates/{templateID}/validation-runs": {
      post: op(jsonRef("TemplateValidationRun"), {
        status: "201",
        request: jsonRef("TemplateValidationRunCreate"),
      }),
    },
    "/v1/templates/{templateID}/validation-runs/{sandboxID}/decision": {
      post: op(jsonRef("TemplateValidationRun"), {
        request: jsonRef("TemplateValidationRunDecision"),
      }),
    },
    "/v1/sandboxes": {
      get: op(listSchema("Sandbox"), { parameters: [queryParam("projectId")] }),
      post: op(jsonRef("Sandbox"), { status: "201", request: jsonRef("SandboxCreate") }),
    },
    "/v1/sandboxes/{sandboxID}": {
      get: op(jsonRef("Sandbox")),
      patch: op(jsonRef("Sandbox"), { request: jsonRef("SandboxUpdate") }),
      delete: noContentOp(),
    },
    "/v1/sandboxes/{sandboxID}/boundary": { get: op(jsonRef("BoundarySummary")) },
    "/v1/sandboxes/{sandboxID}/start": { post: op(jsonRef("Sandbox")) },
    "/v1/sandboxes/{sandboxID}/stop": { post: op(jsonRef("Sandbox")) },
    "/v1/sandboxes/{sandboxID}/runtime": { get: op(jsonRef("RuntimeTarget")) },
    "/v1/sandboxes/{sandboxID}/logs": {
      get: op(jsonRef("LogResult"), { parameters: [queryParam("tailLines")] }),
    },
    "/v1/sandboxes/{sandboxID}/events": { get: op(listSchema("RuntimeEvent")) },
    "/v1/sandboxes/{sandboxID}/ports": { get: op(jsonRef("PreviewPortsResult")) },
    "/v1/sandboxes/{sandboxID}/sessions": {
      get: op(listSchema("RuntimeSession")),
      post: op(jsonRef("RuntimeSession"), { status: "201", request: jsonRef("RuntimeSessionCreate") }),
    },
    "/v1/sandboxes/{sandboxID}/tasks": {
      get: op(listSchema("ExecutionTask")),
      post: op(jsonRef("ExecutionTask"), { status: "201", request: jsonRef("ExecutionTaskCreate") }),
    },
    "/v1/sandboxes/{sandboxID}/artifacts": {
      get: op(listSchema("Artifact")),
      post: op(jsonRef("Artifact"), { status: "201", request: jsonRef("ArtifactCreate") }),
    },
    "/v1/sessions/{sessionID}": { get: op(jsonRef("RuntimeSession")) },
    "/v1/sessions/{sessionID}/end": { post: op(jsonRef("RuntimeSession")) },
    "/v1/tasks/{taskID}": { get: op(jsonRef("ExecutionTask")) },
    "/v1/tasks/{taskID}/events": { get: op(jsonRef("ExecutionTaskEvent"), { mediaType: "application/x-ndjson" }) },
    "/v1/tasks/{taskID}/cancel": { post: op(jsonRef("ExecutionTask")) },
    "/v1/tasks/{taskID}/artifacts": { get: op(listSchema("Artifact")) },
    "/v1/artifacts/{artifactID}": { get: op(jsonRef("Artifact")) },
    "/v1/artifacts/{artifactID}/capture": { post: op(jsonRef("Artifact")) },
    "/v1/artifacts/{artifactID}/content": {
      get: op(binarySchema()),
      put: op(jsonRef("Artifact"), { request: binaryRequest() }),
    },
  }
  return {
    openapi: "3.1.0",
    info: { title: "mbox API", version: "v1alpha1" },
    paths,
    components: {
      securitySchemes: {
        bearerAuth: {
          type: "http",
          scheme: "bearer",
        },
      },
      schemas: schemaComponents(),
    },
  }
}

function buildOpenAPIWithIntentionalSDKExceptions() {
  const document = buildOpenAPI()
  document.paths["/v1/sandboxes/{sandboxID}/ports/{port}/proxy/"] = {
    get: op(binarySchema()),
  }
  document.paths["/v1/sandboxes/{sandboxID}/terminal"] = {
    get: {
      security: [{ bearerAuth: [] }],
      responses: {
        101: { description: "WebSocket upgrade" },
        401: unauthorizedResponse(),
      },
    },
  }
  return document
}

function op(responseSchema, options = {}) {
  const status = options.status ?? "200"
  const mediaType = options.mediaType ?? "application/json"
  const operation = {
    security: options.auth === "none" ? [] : [{ bearerAuth: [] }],
    responses: {
      [status]: {
        description: "OK",
        content: { [mediaType]: { schema: responseSchema } },
      },
    },
  }
  if (options.auth !== "none") {
    operation.responses["401"] = unauthorizedResponse()
  }
  if (options.parameters) {
    operation.parameters = options.parameters
  }
  if (options.request) {
    operation.requestBody = { required: true, content: requestContent(options.request) }
  }
  return operation
}

function noContentOp() {
  return {
    security: [{ bearerAuth: [] }],
    responses: {
      204: { description: "Deleted" },
      401: unauthorizedResponse(),
    },
  }
}

function unauthorizedResponse() {
  return {
    description: "Unauthorized",
    content: { "application/json": { schema: jsonRef("Error") } },
  }
}

function jsonRef(name) {
  return { $ref: `#/components/schemas/${name}` }
}

function listSchema(name) {
  return {
    type: "object",
    properties: {
      items: { type: "array", items: jsonRef(name) },
    },
    required: ["items"],
  }
}

function binarySchema() {
  return { type: "string", format: "binary" }
}

function binaryRequest() {
  return {
    "application/octet-stream": { schema: binarySchema() },
    "text/plain": { schema: binarySchema() },
  }
}

function requestContent(schemaOrContent) {
  if (schemaOrContent["application/octet-stream"] || schemaOrContent["text/plain"]) {
    return schemaOrContent
  }
  return { "application/json": { schema: schemaOrContent } }
}

function queryParam(name) {
  return { name, in: "query", schema: { type: "string" } }
}

function auditParams(includeProject) {
  return [
    ...(includeProject ? [queryParam("projectId")] : []),
    "action",
    "resourceType",
    "resourceId",
    "actor",
    "source",
    "requestId",
    "operation",
    "reason",
    "since",
    "until",
    "limit",
  ].map((item) => (typeof item === "string" ? queryParam(item) : item))
}

function schemaComponents() {
  const schemas = {}
  for (const name of [
    "Health",
    "Error",
    "APIInfo",
    "TrustedPrincipalHeaderInfo",
    "ProjectRBACInfo",
    "CallerInfo",
    "RuntimeResourceList",
    "RuntimeResourceSummary",
    "RuntimeResourceCount",
    "RuntimeWorkloadSummary",
    "RuntimeQuantityIssue",
    "RuntimeStorageSummary",
    "RuntimeResource",
    "RuntimeResourceOwner",
    "RuntimeResourceObservation",
    "RuntimeStorage",
    "RuntimeOrphanAudit",
    "RuntimeOrphan",
    "RuntimeOrphanReason",
    "ManagedResourceRef",
    "RuntimeOrphanCleanupRequest",
    "RuntimeOrphanCleanupResult",
    "AuditEvent",
    "Project",
    "ProjectCreate",
    "ProjectUpdate",
    "ProjectPolicy",
    "ProjectPolicyUpsert",
    "ProjectQuotaPolicy",
    "ProjectQuotaPolicyUpsert",
    "ResourceUsageValue",
    "ProjectMember",
    "ProjectMemberCreate",
    "ProjectCredential",
    "ProjectCredentialCreate",
    "EnvironmentTemplate",
    "TemplateCreate",
    "TemplateUpdate",
    "BoundarySummary",
    "BoundaryCheck",
    "BoundaryPort",
    "BoundaryCredentialRef",
    "TemplateValidationRun",
    "TemplateValidationRunCreate",
    "TemplateValidationRunDecision",
    "Sandbox",
    "SandboxCreate",
    "SandboxUpdate",
    "RuntimeTarget",
    "LogResult",
    "RuntimeEvent",
    "PreviewPortsResult",
    "RuntimeSession",
    "RuntimeSessionCreate",
    "ExecutionTask",
    "ExecutionTaskCreate",
    "ExecutionTaskEvent",
    "Artifact",
    "ArtifactCreate",
    "ArtifactContent",
  ]) {
    schemas[name] = { type: "object", properties: {}, required: [] }
  }
  Object.assign(schemas, {
    Health: objectSchema(["status"]),
    Error: objectSchema(["error"]),
    RuntimeResourceList: objectSchema(["adapter", "checkedAt", "summary", "items"]),
    RuntimeResourceSummary: objectSchema(["total", "byKind", "byNamespace", "byOwner", "byProject", "workload"]),
    RuntimeResourceCount: objectSchema(["name", "count"]),
    RuntimeWorkloadSummary: objectSchema([
      "observedResources",
      "desiredPods",
      "observedPods",
      "runningPods",
      "containersReady",
      "containersTotal",
      "restartCount",
    ], [
      "requests",
      "limits",
      "storageCapacity",
      "quantityIssues",
      "storage",
    ]),
    RuntimeQuantityIssue: objectSchema(["resource", "field", "reason"], ["value"]),
    RuntimeStorageSummary: objectSchema(["phase", "count"], ["capacity"]),
    RuntimeResource: objectSchema(["adapter", "kind", "name"], ["namespace", "owner", "observation", "labels", "createdAt"]),
    RuntimeResourceOwner: objectSchema(["kind"], ["projectId", "sandboxId", "templateId"]),
    RuntimeResourceObservation: objectSchema([], [
      "runtimeName",
      "selector",
      "replicas",
      "podCount",
      "runningPodCount",
      "podName",
      "podPhase",
      "containersReady",
      "containersTotal",
      "restartCount",
      "requests",
      "limits",
      "storage",
      "readyCondition",
      "message",
    ]),
    RuntimeStorage: objectSchema(["name", "mountPath"], [
      "claimName",
      "phase",
      "capacity",
      "storageClassName",
      "message",
    ]),
    RuntimeTarget: objectSchema(["namespace", "podName", "container", "phase", "selector"], [
      "commands",
      "storage",
    ]),
    LogResult: objectSchema(["target", "logs"]),
    RuntimeEvent: objectSchema([], [
      "type",
      "reason",
      "message",
      "count",
      "firstTimestamp",
      "lastTimestamp",
    ]),
    ExecutionTaskEvent: objectSchema(["type", "createdAt"], [
      "task",
      "stream",
      "data",
      "offset",
    ]),
    ExecutionTask: objectSchema([
      "id",
      "projectId",
      "sandboxId",
      "status",
      "command",
      "timeoutSeconds",
      "stdout",
      "stderr",
      "outputTruncated",
      "createdAt",
      "updatedAt",
    ], [
      "exitCode",
      "error",
      "runtimeRef",
      "metadata",
      "startedAt",
      "finishedAt",
    ]),
    ExecutionTaskCreate: objectSchema(["command"], [
      "timeoutSeconds",
      "metadata",
    ]),
    RuntimeOrphanAudit: objectSchema([
      "adapter",
      "checkedAt",
      "resourceCount",
      "orphanCount",
      "expectedClean",
      "items",
    ], ["namespace"]),
    RuntimeOrphan: objectSchema(["reason", "resource", "message"], [
      "sandboxId",
      "templateId",
      "projectId",
      "runtimeRef",
      "status",
      "deletedAt",
      "evidence",
    ]),
    ManagedResourceRef: objectSchema(["adapter", "kind", "namespace", "name"]),
    RuntimeOrphanCleanupRequest: objectSchema(["resource", "reason", "confirm", "deleteOrphan"]),
    RuntimeOrphanCleanupResult: objectSchema(["deleted", "resource", "reason", "message"]),
    Project: objectSchema(["id", "name", "slug", "defaultNamespace"], [
      "repositoryUrl",
      "defaultTemplateId",
      "metadata",
      "createdAt",
      "updatedAt",
    ]),
    ProjectCreate: objectSchema(["name", "defaultNamespace"], [
      "slug",
      "repositoryUrl",
      "metadata",
    ]),
    ProjectUpdate: objectSchema([], [
      "name",
      "repositoryUrl",
      "defaultNamespace",
      "defaultTemplateId",
      "metadata",
    ]),
    ProjectUsage: objectSchema([
      "projectId",
      "generatedAt",
      "sandboxes",
      "runtimeSessions",
      "executionTasks",
      "artifacts",
      "templates",
      "credentials",
    ], ["notes"]),
    ProjectSandboxUsage: objectSchema([
      "total",
      "active",
      "pending",
      "running",
      "stopped",
      "failed",
      "deleted",
      "cleanupPending",
      "activeRequests",
      "runningRequests",
    ]),
    SandboxResourceRequestUsage: objectSchema(["count", "cpu", "memory", "storage"]),
    ResourceQuantityUsage: objectSchema(["declared", "missing", "invalid"], ["total"]),
    ProjectSessionUsage: objectSchema([
      "total",
      "active",
      "ended",
      "failed",
      "terminal",
      "ide",
      "notebook",
      "browser",
      "command",
      "custom",
    ]),
    ProjectTaskUsage: objectSchema(["total", "queued", "running", "succeeded", "failed", "canceled", "timedOut"]),
    ProjectArtifactUsage: objectSchema([
      "total",
      "retainedContent",
      "referencedBytes",
      "retainedBytes",
      "file",
      "directory",
      "log",
      "report",
      "screenshot",
      "image",
      "link",
      "other",
    ]),
    ProjectTemplateUsage: objectSchema(["projectScoped", "globalVisible"], [
      "cpuRequests",
      "memoryRequests",
      "storageRequests",
    ]),
    ResourceUsageValue: objectSchema(["value", "count"]),
    ProjectCredentialUsage: objectSchema(["total", "git", "registry", "kubernetes", "ssh", "generic"]),
    BoundarySummary: objectSchema([
      "kind",
      "templateId",
      "templateName",
      "serviceAccountTokenAutomount",
      "image",
      "workingDir",
      "envVarCount",
      "secretProjection",
      "networkPolicy",
      "networkPolicyProjection",
      "lifecyclePolicyProjection",
      "policyEnforcement",
      "credentialProjection",
      "controllerPermissions",
      "runtimeAccess",
      "cleanup",
      "checks",
    ], [
      "projectId",
      "projectName",
      "sandboxId",
      "sandboxName",
      "sandboxStatus",
      "namespace",
      "serviceAccountName",
      "runtimeRef",
      "resourceRequests",
      "storageRequest",
      "previewPorts",
      "secretRefs",
      "lifecyclePolicy",
      "allowedImagePrefixes",
      "allowedServiceAccounts",
      "allowedSecretRefs",
      "credentialRefs",
    ]),
    BoundaryCheck: objectSchema(["id", "label", "status", "message"], ["evidence"]),
    BoundaryPort: objectSchema(["name", "port", "protocol"]),
    BoundaryCredentialRef: objectSchema(["id", "name", "slug", "type", "secretRef"], [
      "target",
      "usage",
    ]),
    SecretRef: objectSchema(["name"], ["key"]),
    ProjectCredential: objectSchema(["id", "projectId", "name", "slug", "type", "secretRef"], [
      "target",
      "usage",
      "metadata",
      "createdAt",
      "updatedAt",
    ]),
    ProjectCredentialCreate: objectSchema(["name", "type", "secretRef"], [
      "slug",
      "target",
      "usage",
      "metadata",
    ]),
    TemplatePort: objectSchema(["name", "port", "protocol"]),
    EnvironmentTemplate: objectSchema(["id", "name", "slug", "image"], [
      "projectId",
      "startupCommand",
      "workingDir",
      "cpuRequest",
      "memoryRequest",
      "storageRequest",
      "exposedPorts",
      "env",
      "secretRefs",
      "networkPolicy",
      "lifecyclePolicy",
      "metadata",
      "createdAt",
      "updatedAt",
    ]),
    TemplateCreate: objectSchema(["name", "image"], [
      "projectId",
      "slug",
      "startupCommand",
      "workingDir",
      "cpuRequest",
      "memoryRequest",
      "storageRequest",
      "exposedPorts",
      "env",
      "secretRefs",
      "networkPolicy",
      "lifecyclePolicy",
      "metadata",
    ]),
    TemplateUpdate: objectSchema([], [
      "name",
      "image",
      "startupCommand",
      "workingDir",
      "cpuRequest",
      "memoryRequest",
      "storageRequest",
      "exposedPorts",
      "env",
      "secretRefs",
      "networkPolicy",
      "lifecyclePolicy",
      "metadata",
    ]),
    TemplateValidationRunCreate: objectSchema([], [
      "projectId",
      "name",
      "metadata",
    ]),
    TemplateValidationRunDecision: objectSchema(["status"]),
    TemplateValidationRun: objectSchema(["template", "sandbox"]),
    RuntimeRef: objectSchema(["kind", "namespace", "name"], ["adapter"]),
    SandboxPort: objectSchema(["name", "port", "protocol"], ["previewUrl"]),
    Sandbox: objectSchema([
      "id",
      "projectId",
      "name",
      "slug",
      "status",
      "namespace",
      "serviceAccountName",
    ], [
      "templateId",
      "runtimeRef",
      "ports",
      "metadata",
      "createdAt",
      "updatedAt",
      "deletedAt",
    ]),
    SandboxCreate: objectSchema(["projectId", "name"], [
      "templateId",
      "slug",
      "namespace",
      "serviceAccountName",
      "metadata",
    ]),
    SandboxUpdate: objectSchema([], [
      "name",
      "status",
      "namespace",
      "serviceAccountName",
      "runtimeRef",
      "ports",
      "metadata",
    ]),
    PreviewPort: objectSchema(["name", "port", "protocol", "available"], [
      "previewUrl",
      "message",
    ]),
    PreviewPortsResult: objectSchema(["target", "items"]),
    RuntimeSession: objectSchema([
      "id",
      "projectId",
      "sandboxId",
      "type",
      "status",
      "startedAt",
    ], [
      "client",
      "userAgent",
      "runtimeRef",
      "metadata",
      "endedAt",
      "createdAt",
      "updatedAt",
    ]),
    RuntimeSessionCreate: objectSchema(["type"], [
      "client",
      "metadata",
    ]),
    Artifact: objectSchema(["id", "projectId", "sandboxId", "kind", "name", "uri"], [
      "taskId",
      "contentType",
      "sizeBytes",
      "metadata",
      "retainedContent",
      "createdAt",
      "updatedAt",
    ]),
    ArtifactCreate: objectSchema(["kind", "name", "uri"], [
      "taskId",
      "contentType",
      "sizeBytes",
      "metadata",
    ]),
    ArtifactContent: objectSchema(["artifactId", "sizeBytes", "sha256", "sourceUri", "storageProvider", "capturedAt"], [
      "contentType",
      "storageKey",
    ]),
    APIInfo: objectSchema([
      "name",
      "apiVersion",
      "serverVersion",
      "runtimeController",
      "runtimeAccess",
      "artifactContent",
      "trustedPrincipalHeaders",
      "projectRbac",
      "capabilities",
      "compatibility",
      "authenticationRequired",
    ]),
    TrustedPrincipalHeaderInfo: objectSchema(["enabled"], ["principalHeader", "principalTypeHeader"]),
    ProjectRBACInfo: objectSchema(["enforcementEnabled", "enforcedActions"]),
    RuntimeInfo: objectSchema(["enabled"], ["adapter"]),
    ArtifactInfo: objectSchema(["retainedContentEnabled", "storageProvider", "maxBytes"]),
    Compatibility: objectSchema(["minimumCliApiVersion", "minimumSdkApiVersion"]),
    ProjectPolicy: objectSchema(["projectId", "enforcement"], [
      "allowedImagePrefixes",
      "allowedServiceAccounts",
      "allowedSecretRefs",
      "createdAt",
      "updatedAt",
    ]),
    ProjectPolicyUpsert: objectSchema(["enforcement"], [
      "allowedImagePrefixes",
      "allowedServiceAccounts",
      "allowedSecretRefs",
    ]),
    ProjectQuotaPolicy: objectSchema(["projectId", "enforcement"], [
      "maxActiveSandboxes",
      "maxRetainedArtifactBytes",
      "createdAt",
      "updatedAt",
    ]),
    ProjectQuotaPolicyUpsert: objectSchema(["enforcement"], [
      "maxActiveSandboxes",
      "maxRetainedArtifactBytes",
    ]),
    ProjectMember: objectSchema(["id", "projectId", "principalType", "principal", "role"], [
      "metadata",
      "createdAt",
      "updatedAt",
    ]),
    ProjectMemberCreate: objectSchema(["principalType", "principal", "role"], ["metadata"]),
    ProjectAuthorizationDecision: objectSchema(
      [
        "projectId",
        "action",
        "allowed",
        "enforced",
        "evaluation",
        "requiredRoles",
        "caller",
        "memberCount",
        "availableActions",
        "notes",
      ],
      ["matchedMember"],
    ),
    CallerInfo: objectSchema([
      "authenticated",
      "authenticationRequired",
      "mode",
      "principalType",
      "principal",
      "rbacTrusted",
      "projectRolesEnforced",
      "notes",
    ]),
    PolicyDeniedAuditMetadata: objectSchema(["operation", "reason"], [
      "requestId",
      "templateId",
      "templateName",
      "image",
      "serviceAccountName",
      "sandboxId",
      "authorizationAction",
      "callerMode",
      "callerPrincipalType",
      "callerPrincipal",
      "artifactKind",
      "incomingBytes",
      "policyKind",
      "enforcement",
      "maxActiveSandboxes",
      "maxRetainedArtifactBytes",
      "type",
      "target",
      "secretRef",
      "principalType",
      "principal",
      "role",
    ]),
  })
  schemas.AuditEvent = objectSchema(["id", "action", "resourceType", "createdAt"], [
    "projectId",
    "resourceId",
    "resourceName",
    "actor",
    "source",
    "metadata",
  ])
  schemas.AuditEvent.properties.action = jsonRef("AuditEventAction")
  schemas.AuditEvent.properties.metadata = jsonRef("AuditEventMetadata")
  schemas.PolicyDeniedAuditMetadata.properties.incomingBytes.type = "integer"
  schemas.PolicyDeniedAuditMetadata.properties.maxActiveSandboxes.type = "integer"
  schemas.PolicyDeniedAuditMetadata.properties.maxRetainedArtifactBytes.type = "integer"
  schemas.ProjectQuotaPolicy.properties.maxActiveSandboxes.type = "integer"
  schemas.ProjectQuotaPolicy.properties.maxRetainedArtifactBytes.type = "integer"
  schemas.ProjectQuotaPolicyUpsert.properties.maxActiveSandboxes.type = "integer"
  schemas.ProjectQuotaPolicyUpsert.properties.maxRetainedArtifactBytes.type = "integer"
  schemas.APIInfo.properties.runtimeController = jsonRef("RuntimeInfo")
  schemas.APIInfo.properties.runtimeAccess = jsonRef("RuntimeInfo")
  schemas.APIInfo.properties.artifactContent = jsonRef("ArtifactInfo")
  schemas.APIInfo.properties.trustedPrincipalHeaders = jsonRef("TrustedPrincipalHeaderInfo")
  schemas.APIInfo.properties.projectRbac = jsonRef("ProjectRBACInfo")
  schemas.APIInfo.properties.compatibility = jsonRef("Compatibility")
  schemas.ProjectUsage.properties.sandboxes = jsonRef("ProjectSandboxUsage")
  schemas.ProjectUsage.properties.runtimeSessions = jsonRef("ProjectSessionUsage")
  schemas.ProjectUsage.properties.executionTasks = jsonRef("ProjectTaskUsage")
  schemas.ProjectUsage.properties.artifacts = jsonRef("ProjectArtifactUsage")
  schemas.ProjectUsage.properties.templates = jsonRef("ProjectTemplateUsage")
  schemas.ProjectUsage.properties.credentials = jsonRef("ProjectCredentialUsage")
  schemas.ProjectSandboxUsage.properties.activeRequests = jsonRef("SandboxResourceRequestUsage")
  schemas.ProjectSandboxUsage.properties.runningRequests = jsonRef("SandboxResourceRequestUsage")
  schemas.SandboxResourceRequestUsage.properties.cpu = jsonRef("ResourceQuantityUsage")
  schemas.SandboxResourceRequestUsage.properties.memory = jsonRef("ResourceQuantityUsage")
  schemas.SandboxResourceRequestUsage.properties.storage = jsonRef("ResourceQuantityUsage")
  for (const [schemaName, properties] of Object.entries({
    ProjectSandboxUsage: [
      "total",
      "active",
      "pending",
      "running",
      "stopped",
      "failed",
      "deleted",
      "cleanupPending",
    ],
    SandboxResourceRequestUsage: ["count"],
    ResourceQuantityUsage: ["declared", "missing", "invalid"],
    ProjectSessionUsage: ["total", "active", "ended", "failed", "terminal", "ide", "notebook", "browser", "command", "custom"],
    ProjectTaskUsage: ["total", "queued", "running", "succeeded", "failed", "canceled", "timedOut"],
    ProjectTemplateUsage: ["projectScoped", "globalVisible"],
    ProjectCredentialUsage: ["total", "git", "registry", "kubernetes", "ssh", "generic"],
    ResourceUsageValue: ["count"],
  })) {
    for (const property of properties) {
      schemas[schemaName].properties[property].type = "integer"
    }
  }
  schemas.ProjectTemplateUsage.properties.cpuRequests = arrayRef("ResourceUsageValue")
  schemas.ProjectTemplateUsage.properties.memoryRequests = arrayRef("ResourceUsageValue")
  schemas.ProjectTemplateUsage.properties.storageRequests = arrayRef("ResourceUsageValue")
  schemas.ProjectAuthorizationDecision.properties.caller = jsonRef("CallerInfo")
  schemas.ProjectAuthorizationDecision.properties.matchedMember = jsonRef("ProjectMember")
  schemas.ProjectAuthorizationDecision.properties.action.enum = [
    "project.view",
    "project.manage",
    "sandbox.launch",
    "runtime.operate",
    "artifact.write",
    "policy.manage",
    "credential.manage",
    "member.manage",
  ]
  schemas.ProjectAuthorizationDecision.properties.evaluation.enum = ["allowed", "denied", "not_enforceable"]
  schemas.RuntimeResourceList.properties.summary = jsonRef("RuntimeResourceSummary")
  schemas.RuntimeResourceList.properties.items = arrayRef("RuntimeResource")
  schemas.RuntimeResourceSummary.properties.workload = jsonRef("RuntimeWorkloadSummary")
  schemas.RuntimeResourceSummary.properties.total.type = "integer"
  schemas.RuntimeResourceCount.properties.count.type = "integer"
  for (const property of [
    "observedResources",
    "desiredPods",
    "observedPods",
    "runningPods",
    "containersReady",
    "containersTotal",
    "restartCount",
  ]) {
    schemas.RuntimeWorkloadSummary.properties[property].type = "integer"
  }
  schemas.RuntimeStorageSummary.properties.count.type = "integer"
  for (const property of [
    "replicas",
    "podCount",
    "runningPodCount",
    "containersReady",
    "containersTotal",
    "restartCount",
  ]) {
    schemas.RuntimeResourceObservation.properties[property].type = "integer"
  }
  schemas.RuntimeResourceSummary.properties.byKind = arrayRef("RuntimeResourceCount")
  schemas.RuntimeResourceSummary.properties.byNamespace = arrayRef("RuntimeResourceCount")
  schemas.RuntimeResourceSummary.properties.byOwner = arrayRef("RuntimeResourceCount")
  schemas.RuntimeResourceSummary.properties.byProject = arrayRef("RuntimeResourceCount")
  schemas.RuntimeWorkloadSummary.properties.quantityIssues = arrayRef("RuntimeQuantityIssue")
  schemas.RuntimeWorkloadSummary.properties.storage = arrayRef("RuntimeStorageSummary")
  schemas.RuntimeResource.properties.owner = jsonRef("RuntimeResourceOwner")
  schemas.RuntimeResource.properties.observation = jsonRef("RuntimeResourceObservation")
  schemas.RuntimeResourceObservation.properties.storage = arrayRef("RuntimeStorage")
  schemas.RuntimeTarget.properties.storage = arrayRef("RuntimeStorage")
  schemas.LogResult.properties.target = jsonRef("RuntimeTarget")
  schemas.PreviewPortsResult.properties.target = jsonRef("RuntimeTarget")
  schemas.PreviewPortsResult.properties.items = arrayRef("PreviewPort")
  schemas.ExecutionTaskEvent.properties.task = jsonRef("ExecutionTask")
  schemas.CallerInfo.properties.mode.enum = ["anonymous", "shared_token", "trusted_header"]
  schemas.CallerInfo.properties.principalType.enum = [
    "anonymous",
    "shared_token",
    "user",
    "service_account",
    "automation",
  ]
  schemas.ProjectPolicy.properties.enforcement.enum = ["disabled", "enforced"]
  schemas.ProjectPolicyUpsert.properties.enforcement.enum = ["disabled", "enforced"]
  schemas.ProjectQuotaPolicy.properties.enforcement.enum = ["disabled", "enforced"]
  schemas.ProjectQuotaPolicyUpsert.properties.enforcement.enum = ["disabled", "enforced"]
  schemas.ProjectMember.properties.principalType.enum = ["user", "service_account", "automation"]
  schemas.ProjectMember.properties.role.enum = ["owner", "operator", "viewer"]
  schemas.ProjectMemberCreate.properties.principalType.enum = ["user", "service_account", "automation"]
  schemas.ProjectMemberCreate.properties.role.enum = ["owner", "operator", "viewer"]
  schemas.ProjectCredential.properties.secretRef = jsonRef("SecretRef")
  schemas.ProjectCredentialCreate.properties.secretRef = jsonRef("SecretRef")
  schemas.ProjectCredential.properties.type.enum = ["git", "registry", "kubernetes", "ssh", "generic"]
  schemas.ProjectCredentialCreate.properties.type.enum = ["git", "registry", "kubernetes", "ssh", "generic"]
  for (const schemaName of ["EnvironmentTemplate", "TemplateCreate", "TemplateUpdate"]) {
    schemas[schemaName].properties.exposedPorts = arrayRef("TemplatePort")
    schemas[schemaName].properties.secretRefs = arrayRef("SecretRef")
  }
  schemas.Sandbox.properties.runtimeRef = jsonRef("RuntimeRef")
  schemas.Sandbox.properties.ports = arrayRef("SandboxPort")
  schemas.SandboxUpdate.properties.runtimeRef = jsonRef("RuntimeRef")
  schemas.SandboxUpdate.properties.ports = arrayRef("SandboxPort")
  schemas.TemplateValidationRun.properties.template = jsonRef("EnvironmentTemplate")
  schemas.TemplateValidationRun.properties.sandbox = jsonRef("Sandbox")
  schemas.BoundarySummary.properties.kind.enum = ["template", "sandbox"]
  schemas.BoundarySummary.properties.sandboxStatus.enum = ["pending", "running", "stopped", "failed", "deleted"]
  schemas.BoundarySummary.properties.policyEnforcement.enum = ["disabled", "enforced"]
  schemas.BoundarySummary.properties.runtimeRef = jsonRef("RuntimeRef")
  schemas.BoundarySummary.properties.previewPorts = arrayRef("BoundaryPort")
  schemas.BoundarySummary.properties.secretRefs = arrayRef("SecretRef")
  schemas.BoundarySummary.properties.credentialRefs = arrayRef("BoundaryCredentialRef")
  schemas.BoundarySummary.properties.checks = arrayRef("BoundaryCheck")
  schemas.BoundaryCheck.properties.status.enum = ["pass", "warn", "fail"]
  schemas.BoundaryCredentialRef.properties.type.enum = ["git", "registry", "kubernetes", "ssh", "generic"]
  schemas.TemplateValidationRunDecision.properties.status.enum = ["passed", "failed"]
  schemas.Sandbox.properties.status.enum = ["pending", "running", "stopped", "failed", "deleted"]
  schemas.SandboxUpdate.properties.status.enum = ["pending", "running", "stopped", "failed", "deleted"]
  schemas.RuntimeSession.properties.type.enum = ["terminal", "ide", "notebook", "browser", "command", "custom"]
  schemas.RuntimeSession.properties.status.enum = ["active", "ended", "failed"]
  schemas.RuntimeSession.properties.runtimeRef = jsonRef("RuntimeRef")
  schemas.RuntimeSessionCreate.properties.type.enum = ["terminal", "ide", "notebook", "browser", "command", "custom"]
  schemas.ExecutionTask.properties.status.enum = ["queued", "running", "succeeded", "failed", "canceled", "timed_out"]
  schemas.ExecutionTask.properties.runtimeRef = jsonRef("RuntimeRef")
  schemas.ExecutionTaskEvent.properties.type.enum = ["snapshot", "status", "output", "done"]
  schemas.ExecutionTaskEvent.properties.stream.enum = ["stdout", "stderr"]
  schemas.Artifact.properties.retainedContent = jsonRef("ArtifactContent")
  schemas.Artifact.properties.sizeBytes.type = "number"
  schemas.ArtifactCreate.properties.sizeBytes.type = "number"
  schemas.ArtifactContent.properties.sizeBytes.type = "number"
  schemas.Artifact.properties.kind.enum = ["file", "directory", "log", "report", "screenshot", "image", "link", "other"]
  schemas.ArtifactCreate.properties.kind.enum = ["file", "directory", "log", "report", "screenshot", "image", "link", "other"]
  schemas.ArtifactContent.properties.storageProvider.enum = ["postgres", "filesystem", "s3"]
  schemas.PolicyDeniedAuditMetadata.properties.operation.enum = [
    "sandbox.launch",
    "template.validation",
    "project.policy.update",
    "project.quota_policy.update",
    "runtime.resolve",
    "runtime.logs",
    "runtime.events",
    "runtime.ports",
    "runtime.preview.proxy",
    "runtime.terminal",
    "runtime.session.create",
    "runtime.session.end",
    "execution.task.create",
    "execution.task.cancel",
    "execution.task.events",
    "artifact.write",
    "artifact.content.workspace.read",
    "artifact.content.capture",
    "artifact.content.upload",
    "project.credential.create",
    "project.credential.delete",
    "project.member.create",
    "project.member.delete",
  ]
  schemas.PolicyDeniedAuditMetadata.properties.authorizationAction.enum = [
    "project.view",
    "project.manage",
    "sandbox.launch",
    "runtime.operate",
    "artifact.write",
    "policy.manage",
    "credential.manage",
    "member.manage",
  ]
  schemas.PolicyDeniedAuditMetadata.properties.callerMode.enum = ["anonymous", "shared_token", "trusted_header"]
  schemas.PolicyDeniedAuditMetadata.properties.callerPrincipalType.enum = [
    "anonymous",
    "shared_token",
    "user",
    "service_account",
    "automation",
  ]
  schemas.PolicyDeniedAuditMetadata.properties.policyKind.enum = ["launch", "quota"]
  schemas.PolicyDeniedAuditMetadata.properties.enforcement.enum = ["disabled", "enforced"]
  schemas.PolicyDeniedAuditMetadata.properties.principalType.enum = ["user", "service_account", "automation"]
  schemas.PolicyDeniedAuditMetadata.properties.role.enum = ["owner", "operator", "viewer"]
  schemas.RuntimeResourceOwner.properties.kind.enum = ["sandbox", "template"]
  schemas.RuntimeOrphanReason = {
    type: "string",
    enum: [
      "missing-sandbox-record",
      "cleanup-pending",
      "runtime-ref-mismatch",
      "missing-template-record",
      "unlabeled-owner",
    ],
  }
  schemas.RuntimeOrphan.properties.reason = jsonRef("RuntimeOrphanReason")
  schemas.RuntimeOrphan.properties.resource = jsonRef("RuntimeResource")
  schemas.RuntimeOrphan.properties.runtimeRef = jsonRef("RuntimeRef")
  schemas.RuntimeOrphan.properties.evidence = { type: "array", items: { type: "string" } }
  schemas.RuntimeOrphanAudit.properties.items = arrayRef("RuntimeOrphan")
  schemas.RuntimeOrphanCleanupRequest.properties.resource = jsonRef("ManagedResourceRef")
  schemas.RuntimeOrphanCleanupRequest.properties.reason = jsonRef("RuntimeOrphanReason")
  schemas.RuntimeOrphanCleanupResult.properties.resource = jsonRef("ManagedResourceRef")
  schemas.RuntimeOrphanCleanupResult.properties.reason = jsonRef("RuntimeOrphanReason")
  schemas.RuntimeOrphanCleanupRequest.properties.confirm.enum = ["delete-orphan-runtime-resource"]
  schemas.RuntimeOrphan.properties.status.enum = ["pending", "running", "stopped", "failed", "deleted"]
  return schemas
}

function objectSchema(required, optional = []) {
  const properties = {}
  for (const name of [...required, ...optional]) {
    properties[name] = { type: "string" }
  }
  return { type: "object", required, properties }
}

function arrayRef(name) {
  return { type: "array", items: jsonRef(name) }
}

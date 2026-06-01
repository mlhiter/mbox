#!/usr/bin/env node
import assert from "node:assert/strict"
import {
  MboxClient,
  MboxCompatibilityError,
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
  capabilities: ["sandboxes", "execution-tasks", "task-events", "artifact-client-upload"],
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

console.log("SDK smoke passed")

function jsonFetch(payload) {
  return async (url) => {
    assert.equal(new URL(url).pathname, "/v1/info")
    return new Response(JSON.stringify(payload), {
      status: 200,
      headers: { "content-type": "application/json" },
    })
  }
}

function buildOpenAPI() {
  const paths = {
    "/healthz": { get: op(jsonRef("Health"), { auth: "none" }) },
    "/v1/info": { get: op(jsonRef("APIInfo"), { auth: "none" }) },
    "/v1/openapi.json": { get: op({ type: "object" }) },
    "/v1/runtime/resources": {
      get: op(jsonRef("RuntimeResourceList"), { parameters: [queryParam("namespace"), queryParam("kind")] }),
    },
    "/v1/runtime/orphans": {
      get: op(jsonRef("RuntimeOrphanAudit"), { parameters: [queryParam("namespace"), queryParam("kind")] }),
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
    "RuntimeResourceList",
    "RuntimeOrphanAudit",
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
    "ProjectCredential",
    "ProjectCredentialCreate",
    "EnvironmentTemplate",
    "TemplateCreate",
    "TemplateUpdate",
    "BoundarySummary",
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
  ]) {
    schemas[name] = { type: "object", properties: {}, required: [] }
  }
  Object.assign(schemas, {
    RuntimeResourceList: objectSchema(["adapter", "checkedAt", "summary", "items"]),
    RuntimeResourceSummary: objectSchema(["total", "byKind", "byNamespace", "byOwner"]),
    RuntimeResourceCount: objectSchema(["name", "count"]),
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
    ProjectCredentialUsage: objectSchema(["total", "git", "registry", "kubernetes", "ssh", "generic"]),
    PolicyDeniedAuditMetadata: objectSchema(["operation", "reason"], [
      "requestId",
      "templateId",
      "templateName",
      "image",
      "serviceAccountName",
      "sandboxId",
      "artifactKind",
      "incomingBytes",
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
  return schemas
}

function objectSchema(required, optional = []) {
  const properties = {}
  for (const name of [...required, ...optional]) {
    properties[name] = { type: "string" }
  }
  return { type: "object", required, properties }
}

import type { MboxClient, OpenAPIDocument } from "./index.js"

export type SDKRouteMethod = "GET" | "POST" | "PUT" | "PATCH" | "DELETE"

export type SDKRouteContractEntry = {
  sdk: keyof MboxClient
  method: SDKRouteMethod
  path: string
  auth?: SDKRouteAuth
  query?: readonly string[]
  request?: SDKRouteRequestContract
  response?: SDKRouteResponseContract
}

export type SDKRouteAuth = "bearer" | "none"

export type SDKMediaType = "application/json" | "application/x-ndjson" | "application/octet-stream" | "text/plain"

export type SDKRouteRequestContract = {
  mediaType?: SDKMediaType
  mediaTypes?: readonly SDKMediaType[]
  schema?: string
  binary?: true
}

export type SDKRouteResponseContract = {
  status?: string
  mediaType?: SDKMediaType
  schema?: string
  listItem?: string
  noContent?: true
  binary?: true
}

export type SDKSchemaContractEntry = {
  schema: string
  required?: readonly string[]
  properties?: readonly string[]
  absentProperties?: readonly string[]
  enumProperties?: readonly SDKSchemaEnumPropertyContract[]
  propertyRefs?: readonly SDKSchemaPropertyRefContract[]
  arrayItemRefs?: readonly SDKSchemaArrayItemRefContract[]
}

export type SDKSchemaEnumPropertyContract = {
  property: string
  values: readonly string[]
}

export type SDKSchemaPropertyRefContract = {
  property: string
  ref: string
}

export type SDKSchemaArrayItemRefContract = {
  property: string
  ref: string
}

export type SDKOpenAPIAlignmentIssue = {
  reason:
    | "missing-path"
    | "missing-method"
    | "missing-query-param"
    | "missing-request-body"
    | "missing-request-content"
    | "missing-request-schema"
    | "request-schema-mismatch"
    | "request-binary-mismatch"
    | "missing-security-scheme"
    | "security-mismatch"
    | "missing-unauthorized-response"
    | "missing-response"
    | "missing-response-content"
    | "missing-response-schema"
    | "missing-response-required"
    | "response-schema-mismatch"
    | "response-list-item-mismatch"
    | "response-binary-mismatch"
    | "missing-sdk-route-coverage"
    | "missing-schema"
    | "missing-schema-required"
    | "missing-schema-property"
    | "unexpected-schema-property"
    | "missing-schema-enum-value"
    | "schema-property-ref-mismatch"
    | "schema-array-item-ref-mismatch"
  sdk?: keyof MboxClient
  method?: SDKRouteMethod
  path?: string
  query?: readonly string[]
  parameter?: string
  responseStatus?: string
  mediaType?: string
  expectedAuth?: SDKRouteAuth
  actualAuth?: string
  expectedSchema?: string
  actualSchema?: string
  schema?: string
  required?: readonly string[]
  properties?: readonly string[]
  property?: string
  enumValue?: string
}

export type SDKOpenAPIAlignmentResult = {
  ok: boolean
  checked: number
  checkedQueryParams: number
  checkedAuth: number
  checkedRequests: number
  checkedResponses: number
  checkedPublishedOperations: number
  ignoredPublishedOperations: number
  checkedSchemas: number
  checkedSchemaRequired: number
  checkedSchemaProperties: number
  checkedSchemaAbsentProperties: number
  checkedSchemaEnumValues: number
  checkedSchemaPropertyRefs: number
  checkedSchemaArrayItemRefs: number
  missing: SDKOpenAPIAlignmentIssue[]
}

const OPENAPI_METHODS = ["GET", "POST", "PUT", "PATCH", "DELETE"] as const satisfies readonly SDKRouteMethod[]

const SDK_ROUTE_COVERAGE_EXCEPTIONS = [
  {
    method: "GET",
    path: "/v1/sandboxes/{sandboxID}/ports/{port}/proxy/",
    reason: "preview proxy is a browser/streaming pass-through route, not an ordinary SDK helper",
  },
  {
    method: "GET",
    path: "/v1/sandboxes/{sandboxID}/terminal",
    reason: "terminal is a WebSocket upgrade route consumed by browser/CLI terminal clients",
  },
] as const satisfies ReadonlyArray<{ method: SDKRouteMethod; path: string; reason: string }>

const noContentResponse = { status: "204", noContent: true } as const satisfies SDKRouteResponseContract

const projectPolicyEnforcementValues = ["disabled", "enforced"] as const
const projectMemberPrincipalTypeValues = ["user", "service_account", "automation"] as const
const projectMemberRoleValues = ["owner", "operator", "viewer"] as const
const callerAuthModeValues = ["anonymous", "shared_token", "trusted_header"] as const
const callerPrincipalTypeValues = ["anonymous", "shared_token", "user", "service_account", "automation"] as const
const projectAuthorizationActionValues = [
  "project.view",
  "project.manage",
  "sandbox.launch",
  "runtime.operate",
  "artifact.write",
  "policy.manage",
  "credential.manage",
  "member.manage",
] as const
const projectCredentialTypeValues = ["git", "registry", "kubernetes", "ssh", "generic"] as const
const sandboxStatusValues = ["pending", "running", "stopped", "failed", "deleted"] as const
const executionTaskStatusValues = ["queued", "running", "succeeded", "failed", "canceled", "timed_out"] as const
const executionTaskEventTypeValues = ["snapshot", "status", "output", "done"] as const
const executionTaskEventStreamValues = ["stdout", "stderr"] as const
const runtimeSessionTypeValues = ["terminal", "ide", "notebook", "browser", "command", "custom"] as const
const runtimeSessionStatusValues = ["active", "ended", "failed"] as const
const artifactKindValues = ["file", "directory", "log", "report", "screenshot", "image", "link", "other"] as const
const artifactStorageProviderValues = ["postgres", "filesystem", "s3"] as const
const templateValidationDecisionStatusValues = ["passed", "failed"] as const
const boundaryKindValues = ["template", "sandbox"] as const
const boundaryCheckStatusValues = ["pass", "warn", "fail"] as const
const runtimeOrphanReasonValues = [
  "missing-sandbox-record",
  "cleanup-pending",
  "runtime-ref-mismatch",
  "missing-template-record",
  "unlabeled-owner",
] as const
const runtimeOrphanCleanupConfirmValues = ["delete-orphan-runtime-resource"] as const
const runtimeResourceOwnerKindValues = ["sandbox", "template"] as const
const policyDeniedPolicyKindValues = ["launch", "quota"] as const
const policyDeniedOperationValues = [
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
] as const

function jsonResponse(schema: string, status = "200"): SDKRouteResponseContract {
  return { status, schema }
}

function createdResponse(schema: string): SDKRouteResponseContract {
  return jsonResponse(schema, "201")
}

function listResponse(listItem: string): SDKRouteResponseContract {
  return { listItem }
}

function ndjsonResponse(schema: string): SDKRouteResponseContract {
  return { mediaType: "application/x-ndjson", schema }
}

function binaryResponse(): SDKRouteResponseContract {
  return { binary: true }
}

function jsonRequest(schema: string): SDKRouteRequestContract {
  return { schema }
}

function binaryRequest(): SDKRouteRequestContract {
  return { mediaTypes: ["application/octet-stream", "text/plain"], binary: true }
}

export const SDK_ROUTE_CONTRACT = [
  { sdk: "health", method: "GET", path: "/healthz", auth: "none", response: jsonResponse("Health") },
  { sdk: "info", method: "GET", path: "/v1/info", auth: "none", response: jsonResponse("APIInfo") },
  { sdk: "caller", method: "GET", path: "/v1/auth/caller", response: jsonResponse("CallerInfo") },
  { sdk: "openAPI", method: "GET", path: "/v1/openapi.json" },
  {
    sdk: "listRuntimeResources",
    method: "GET",
    path: "/v1/runtime/resources",
    query: ["namespace", "projectId", "kind"],
    response: jsonResponse("RuntimeResourceList"),
  },
  {
    sdk: "listRuntimeOrphans",
    method: "GET",
    path: "/v1/runtime/orphans",
    query: ["namespace", "projectId", "kind"],
    response: jsonResponse("RuntimeOrphanAudit"),
  },
  {
    sdk: "cleanupRuntimeOrphan",
    method: "POST",
    path: "/v1/runtime/orphans/cleanup",
    request: jsonRequest("RuntimeOrphanCleanupRequest"),
    response: jsonResponse("RuntimeOrphanCleanupResult"),
  },
  {
    sdk: "listAuditEvents",
    method: "GET",
    path: "/v1/audit-events",
    query: ["projectId", "action", "resourceType", "resourceId", "actor", "source", "requestId", "operation", "reason", "since", "until", "limit"],
    response: listResponse("AuditEvent"),
  },
  { sdk: "listProjects", method: "GET", path: "/v1/projects", response: listResponse("Project") },
  {
    sdk: "createProject",
    method: "POST",
    path: "/v1/projects",
    request: jsonRequest("ProjectCreate"),
    response: createdResponse("Project"),
  },
  { sdk: "getProject", method: "GET", path: "/v1/projects/{projectID}", response: jsonResponse("Project") },
  {
    sdk: "updateProject",
    method: "PATCH",
    path: "/v1/projects/{projectID}",
    request: jsonRequest("ProjectUpdate"),
    response: jsonResponse("Project"),
  },
  { sdk: "deleteProject", method: "DELETE", path: "/v1/projects/{projectID}", response: noContentResponse },
  {
    sdk: "getProjectAuthorization",
    method: "GET",
    path: "/v1/projects/{projectID}/authorization",
    query: ["action"],
    response: jsonResponse("ProjectAuthorizationDecision"),
  },
  {
    sdk: "getProjectPolicy",
    method: "GET",
    path: "/v1/projects/{projectID}/policy",
    response: jsonResponse("ProjectPolicy"),
  },
  {
    sdk: "setProjectPolicy",
    method: "PUT",
    path: "/v1/projects/{projectID}/policy",
    request: jsonRequest("ProjectPolicyUpsert"),
    response: jsonResponse("ProjectPolicy"),
  },
  {
    sdk: "getProjectQuotaPolicy",
    method: "GET",
    path: "/v1/projects/{projectID}/quota-policy",
    response: jsonResponse("ProjectQuotaPolicy"),
  },
  {
    sdk: "setProjectQuotaPolicy",
    method: "PUT",
    path: "/v1/projects/{projectID}/quota-policy",
    request: jsonRequest("ProjectQuotaPolicyUpsert"),
    response: jsonResponse("ProjectQuotaPolicy"),
  },
  {
    sdk: "listProjectMembers",
    method: "GET",
    path: "/v1/projects/{projectID}/members",
    response: listResponse("ProjectMember"),
  },
  {
    sdk: "createProjectMember",
    method: "POST",
    path: "/v1/projects/{projectID}/members",
    request: jsonRequest("ProjectMemberCreate"),
    response: createdResponse("ProjectMember"),
  },
  {
    sdk: "getProjectMember",
    method: "GET",
    path: "/v1/members/{memberID}",
    response: jsonResponse("ProjectMember"),
  },
  {
    sdk: "deleteProjectMember",
    method: "DELETE",
    path: "/v1/members/{memberID}",
    response: noContentResponse,
  },
  {
    sdk: "listProjectCredentials",
    method: "GET",
    path: "/v1/projects/{projectID}/credentials",
    response: listResponse("ProjectCredential"),
  },
  {
    sdk: "createProjectCredential",
    method: "POST",
    path: "/v1/projects/{projectID}/credentials",
    request: jsonRequest("ProjectCredentialCreate"),
    response: createdResponse("ProjectCredential"),
  },
  {
    sdk: "getProjectUsage",
    method: "GET",
    path: "/v1/projects/{projectID}/usage",
    response: jsonResponse("ProjectUsage"),
  },
  {
    sdk: "listProjectAuditEvents",
    method: "GET",
    path: "/v1/projects/{projectID}/audit-events",
    query: ["action", "resourceType", "resourceId", "actor", "source", "requestId", "operation", "reason", "since", "until", "limit"],
    response: listResponse("AuditEvent"),
  },
  {
    sdk: "getProjectCredential",
    method: "GET",
    path: "/v1/credentials/{credentialID}",
    response: jsonResponse("ProjectCredential"),
  },
  {
    sdk: "deleteProjectCredential",
    method: "DELETE",
    path: "/v1/credentials/{credentialID}",
    response: noContentResponse,
  },
  {
    sdk: "listTemplates",
    method: "GET",
    path: "/v1/templates",
    query: ["projectId"],
    response: listResponse("EnvironmentTemplate"),
  },
  {
    sdk: "createTemplate",
    method: "POST",
    path: "/v1/templates",
    request: jsonRequest("TemplateCreate"),
    response: createdResponse("EnvironmentTemplate"),
  },
  {
    sdk: "getTemplate",
    method: "GET",
    path: "/v1/templates/{templateID}",
    response: jsonResponse("EnvironmentTemplate"),
  },
  {
    sdk: "updateTemplate",
    method: "PATCH",
    path: "/v1/templates/{templateID}",
    request: jsonRequest("TemplateUpdate"),
    response: jsonResponse("EnvironmentTemplate"),
  },
  { sdk: "deleteTemplate", method: "DELETE", path: "/v1/templates/{templateID}", response: noContentResponse },
  {
    sdk: "getTemplateBoundary",
    method: "GET",
    path: "/v1/templates/{templateID}/boundary",
    query: ["projectId"],
    response: jsonResponse("BoundarySummary"),
  },
  {
    sdk: "createTemplateValidationRun",
    method: "POST",
    path: "/v1/templates/{templateID}/validation-runs",
    request: jsonRequest("TemplateValidationRunCreate"),
    response: createdResponse("TemplateValidationRun"),
  },
  {
    sdk: "decideTemplateValidationRun",
    method: "POST",
    path: "/v1/templates/{templateID}/validation-runs/{sandboxID}/decision",
    request: jsonRequest("TemplateValidationRunDecision"),
    response: jsonResponse("TemplateValidationRun"),
  },
  {
    sdk: "listSandboxes",
    method: "GET",
    path: "/v1/sandboxes",
    query: ["projectId"],
    response: listResponse("Sandbox"),
  },
  {
    sdk: "createSandbox",
    method: "POST",
    path: "/v1/sandboxes",
    request: jsonRequest("SandboxCreate"),
    response: createdResponse("Sandbox"),
  },
  { sdk: "getSandbox", method: "GET", path: "/v1/sandboxes/{sandboxID}", response: jsonResponse("Sandbox") },
  {
    sdk: "updateSandbox",
    method: "PATCH",
    path: "/v1/sandboxes/{sandboxID}",
    request: jsonRequest("SandboxUpdate"),
    response: jsonResponse("Sandbox"),
  },
  { sdk: "deleteSandbox", method: "DELETE", path: "/v1/sandboxes/{sandboxID}", response: noContentResponse },
  {
    sdk: "getSandboxBoundary",
    method: "GET",
    path: "/v1/sandboxes/{sandboxID}/boundary",
    response: jsonResponse("BoundarySummary"),
  },
  {
    sdk: "startSandbox",
    method: "POST",
    path: "/v1/sandboxes/{sandboxID}/start",
    response: jsonResponse("Sandbox"),
  },
  {
    sdk: "stopSandbox",
    method: "POST",
    path: "/v1/sandboxes/{sandboxID}/stop",
    response: jsonResponse("Sandbox"),
  },
  {
    sdk: "getRuntimeTarget",
    method: "GET",
    path: "/v1/sandboxes/{sandboxID}/runtime",
    response: jsonResponse("RuntimeTarget"),
  },
  {
    sdk: "getRuntimeLogs",
    method: "GET",
    path: "/v1/sandboxes/{sandboxID}/logs",
    query: ["tailLines"],
    response: jsonResponse("LogResult"),
  },
  {
    sdk: "getRuntimeEvents",
    method: "GET",
    path: "/v1/sandboxes/{sandboxID}/events",
    response: listResponse("RuntimeEvent"),
  },
  {
    sdk: "getPreviewPorts",
    method: "GET",
    path: "/v1/sandboxes/{sandboxID}/ports",
    response: jsonResponse("PreviewPortsResult"),
  },
  {
    sdk: "listRuntimeSessions",
    method: "GET",
    path: "/v1/sandboxes/{sandboxID}/sessions",
    response: listResponse("RuntimeSession"),
  },
  {
    sdk: "createRuntimeSession",
    method: "POST",
    path: "/v1/sandboxes/{sandboxID}/sessions",
    request: jsonRequest("RuntimeSessionCreate"),
    response: createdResponse("RuntimeSession"),
  },
  {
    sdk: "listExecutionTasks",
    method: "GET",
    path: "/v1/sandboxes/{sandboxID}/tasks",
    response: listResponse("ExecutionTask"),
  },
  {
    sdk: "createExecutionTask",
    method: "POST",
    path: "/v1/sandboxes/{sandboxID}/tasks",
    request: jsonRequest("ExecutionTaskCreate"),
    response: createdResponse("ExecutionTask"),
  },
  {
    sdk: "listArtifacts",
    method: "GET",
    path: "/v1/sandboxes/{sandboxID}/artifacts",
    response: listResponse("Artifact"),
  },
  {
    sdk: "createArtifact",
    method: "POST",
    path: "/v1/sandboxes/{sandboxID}/artifacts",
    request: jsonRequest("ArtifactCreate"),
    response: createdResponse("Artifact"),
  },
  {
    sdk: "getRuntimeSession",
    method: "GET",
    path: "/v1/sessions/{sessionID}",
    response: jsonResponse("RuntimeSession"),
  },
  {
    sdk: "endRuntimeSession",
    method: "POST",
    path: "/v1/sessions/{sessionID}/end",
    response: jsonResponse("RuntimeSession"),
  },
  {
    sdk: "getExecutionTask",
    method: "GET",
    path: "/v1/tasks/{taskID}",
    response: jsonResponse("ExecutionTask"),
  },
  { sdk: "waitForTask", method: "GET", path: "/v1/tasks/{taskID}", response: jsonResponse("ExecutionTask") },
  {
    sdk: "watchExecutionTask",
    method: "GET",
    path: "/v1/tasks/{taskID}/events",
    response: ndjsonResponse("ExecutionTaskEvent"),
  },
  {
    sdk: "cancelExecutionTask",
    method: "POST",
    path: "/v1/tasks/{taskID}/cancel",
    response: jsonResponse("ExecutionTask"),
  },
  {
    sdk: "listTaskArtifacts",
    method: "GET",
    path: "/v1/tasks/{taskID}/artifacts",
    response: listResponse("Artifact"),
  },
  { sdk: "getArtifact", method: "GET", path: "/v1/artifacts/{artifactID}", response: jsonResponse("Artifact") },
  {
    sdk: "captureArtifactContent",
    method: "POST",
    path: "/v1/artifacts/{artifactID}/capture",
    response: jsonResponse("Artifact"),
  },
  {
    sdk: "getArtifactContent",
    method: "GET",
    path: "/v1/artifacts/{artifactID}/content",
    response: binaryResponse(),
  },
  {
    sdk: "uploadArtifactContent",
    method: "PUT",
    path: "/v1/artifacts/{artifactID}/content",
    request: binaryRequest(),
    response: jsonResponse("Artifact"),
  },
] as const satisfies readonly SDKRouteContractEntry[]

export const SDK_SCHEMA_CONTRACT = [
  {
    schema: "Health",
    required: ["status"],
    properties: ["status"],
  },
  {
    schema: "Error",
    required: ["error"],
    properties: ["error"],
  },
  {
    schema: "RuntimeResourceList",
    required: ["adapter", "checkedAt", "summary", "items"],
    properties: ["adapter", "checkedAt", "summary", "items"],
    propertyRefs: [{ property: "summary", ref: "RuntimeResourceSummary" }],
    arrayItemRefs: [{ property: "items", ref: "RuntimeResource" }],
  },
  {
    schema: "RuntimeResourceSummary",
    required: ["total", "byKind", "byNamespace", "byOwner", "byProject", "workload"],
    properties: ["total", "byKind", "byNamespace", "byOwner", "byProject", "workload"],
    propertyRefs: [{ property: "workload", ref: "RuntimeWorkloadSummary" }],
    arrayItemRefs: [
      { property: "byKind", ref: "RuntimeResourceCount" },
      { property: "byNamespace", ref: "RuntimeResourceCount" },
      { property: "byOwner", ref: "RuntimeResourceCount" },
      { property: "byProject", ref: "RuntimeResourceCount" },
    ],
  },
  {
    schema: "RuntimeResourceCount",
    required: ["name", "count"],
    properties: ["name", "count"],
  },
  {
    schema: "RuntimeWorkloadSummary",
    required: [
      "observedResources",
      "desiredPods",
      "observedPods",
      "runningPods",
      "containersReady",
      "containersTotal",
      "restartCount",
    ],
    properties: [
      "observedResources",
      "desiredPods",
      "observedPods",
      "runningPods",
      "containersReady",
      "containersTotal",
      "restartCount",
      "requests",
      "limits",
      "storageCapacity",
      "quantityIssues",
      "storage",
    ],
    arrayItemRefs: [
      { property: "quantityIssues", ref: "RuntimeQuantityIssue" },
      { property: "storage", ref: "RuntimeStorageSummary" },
    ],
  },
  {
    schema: "RuntimeQuantityIssue",
    required: ["resource", "field", "reason"],
    properties: ["resource", "field", "value", "reason"],
  },
  {
    schema: "RuntimeStorageSummary",
    required: ["phase", "count"],
    properties: ["phase", "count", "capacity"],
  },
  {
    schema: "RuntimeResource",
    required: ["adapter", "kind", "name"],
    properties: ["adapter", "kind", "namespace", "name", "owner", "observation", "labels", "createdAt"],
    propertyRefs: [
      { property: "owner", ref: "RuntimeResourceOwner" },
      { property: "observation", ref: "RuntimeResourceObservation" },
    ],
  },
  {
    schema: "RuntimeResourceOwner",
    required: ["kind"],
    properties: ["kind", "projectId", "sandboxId", "templateId"],
    enumProperties: [{ property: "kind", values: runtimeResourceOwnerKindValues }],
  },
  {
    schema: "RuntimeResourceObservation",
    properties: [
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
    ],
    arrayItemRefs: [{ property: "storage", ref: "RuntimeStorage" }],
  },
  {
    schema: "RuntimeStorage",
    required: ["name", "mountPath"],
    properties: ["name", "mountPath", "claimName", "phase", "capacity", "storageClassName", "message"],
  },
  {
    schema: "RuntimeTarget",
    required: ["namespace", "podName", "container", "phase", "selector"],
    properties: ["namespace", "podName", "container", "phase", "selector", "commands", "storage"],
    arrayItemRefs: [{ property: "storage", ref: "RuntimeStorage" }],
  },
  {
    schema: "LogResult",
    required: ["target", "logs"],
    properties: ["target", "logs"],
    propertyRefs: [{ property: "target", ref: "RuntimeTarget" }],
  },
  {
    schema: "RuntimeEvent",
    properties: ["type", "reason", "message", "count", "firstTimestamp", "lastTimestamp"],
  },
  {
    schema: "ExecutionTaskEvent",
    required: ["type", "createdAt"],
    properties: ["type", "task", "stream", "data", "offset", "createdAt"],
    propertyRefs: [{ property: "task", ref: "ExecutionTask" }],
    enumProperties: [
      { property: "type", values: executionTaskEventTypeValues },
      { property: "stream", values: executionTaskEventStreamValues },
    ],
  },
  {
    schema: "ExecutionTask",
    required: [
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
    ],
    properties: [
      "id",
      "projectId",
      "sandboxId",
      "status",
      "command",
      "timeoutSeconds",
      "exitCode",
      "stdout",
      "stderr",
      "outputTruncated",
      "error",
      "runtimeRef",
      "metadata",
      "startedAt",
      "finishedAt",
      "createdAt",
      "updatedAt",
    ],
    enumProperties: [{ property: "status", values: executionTaskStatusValues }],
    propertyRefs: [{ property: "runtimeRef", ref: "RuntimeRef" }],
  },
  {
    schema: "ExecutionTaskCreate",
    required: ["command"],
    properties: ["command", "timeoutSeconds", "metadata"],
  },
  {
    schema: "RuntimeOrphanAudit",
    required: ["adapter", "checkedAt", "resourceCount", "orphanCount", "expectedClean", "items"],
    properties: ["adapter", "checkedAt", "namespace", "resourceCount", "orphanCount", "expectedClean", "items"],
    arrayItemRefs: [{ property: "items", ref: "RuntimeOrphan" }],
  },
  {
    schema: "RuntimeOrphan",
    required: ["reason", "resource", "message"],
    properties: [
      "reason",
      "resource",
      "sandboxId",
      "templateId",
      "projectId",
      "runtimeRef",
      "status",
      "deletedAt",
      "message",
      "evidence",
    ],
    propertyRefs: [
      { property: "reason", ref: "RuntimeOrphanReason" },
      { property: "resource", ref: "RuntimeResource" },
      { property: "runtimeRef", ref: "RuntimeRef" },
    ],
    enumProperties: [
      { property: "reason", values: runtimeOrphanReasonValues },
      { property: "status", values: sandboxStatusValues },
    ],
  },
  {
    schema: "ManagedResourceRef",
    required: ["adapter", "kind", "namespace", "name"],
    properties: ["adapter", "kind", "namespace", "name"],
  },
  {
    schema: "RuntimeOrphanCleanupRequest",
    required: ["resource", "reason", "confirm", "deleteOrphan"],
    properties: ["resource", "reason", "confirm", "deleteOrphan"],
    propertyRefs: [
      { property: "resource", ref: "ManagedResourceRef" },
      { property: "reason", ref: "RuntimeOrphanReason" },
    ],
    enumProperties: [
      { property: "reason", values: runtimeOrphanReasonValues },
      { property: "confirm", values: runtimeOrphanCleanupConfirmValues },
    ],
  },
  {
    schema: "RuntimeOrphanCleanupResult",
    required: ["deleted", "resource", "reason", "message"],
    properties: ["deleted", "resource", "reason", "message"],
    propertyRefs: [
      { property: "resource", ref: "ManagedResourceRef" },
      { property: "reason", ref: "RuntimeOrphanReason" },
    ],
    enumProperties: [{ property: "reason", values: runtimeOrphanReasonValues }],
  },
  {
    schema: "ProjectUsage",
    required: [
      "projectId",
      "generatedAt",
      "sandboxes",
      "runtimeSessions",
      "executionTasks",
      "artifacts",
      "templates",
      "credentials",
    ],
    properties: [
      "projectId",
      "generatedAt",
      "sandboxes",
      "runtimeSessions",
      "executionTasks",
      "artifacts",
      "templates",
      "credentials",
      "notes",
    ],
    propertyRefs: [
      { property: "sandboxes", ref: "ProjectSandboxUsage" },
      { property: "runtimeSessions", ref: "ProjectSessionUsage" },
      { property: "executionTasks", ref: "ProjectTaskUsage" },
      { property: "artifacts", ref: "ProjectArtifactUsage" },
      { property: "templates", ref: "ProjectTemplateUsage" },
      { property: "credentials", ref: "ProjectCredentialUsage" },
    ],
  },
  {
    schema: "BoundarySummary",
    required: [
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
    ],
    properties: [
      "kind",
      "projectId",
      "projectName",
      "templateId",
      "templateName",
      "sandboxId",
      "sandboxName",
      "sandboxStatus",
      "namespace",
      "serviceAccountName",
      "serviceAccountTokenAutomount",
      "runtimeRef",
      "image",
      "workingDir",
      "resourceRequests",
      "storageRequest",
      "previewPorts",
      "envVarCount",
      "secretRefs",
      "secretProjection",
      "networkPolicy",
      "networkPolicyProjection",
      "lifecyclePolicy",
      "lifecyclePolicyProjection",
      "policyEnforcement",
      "allowedImagePrefixes",
      "allowedServiceAccounts",
      "allowedSecretRefs",
      "credentialRefs",
      "credentialProjection",
      "controllerPermissions",
      "runtimeAccess",
      "cleanup",
      "checks",
    ],
    enumProperties: [
      { property: "kind", values: boundaryKindValues },
      { property: "sandboxStatus", values: sandboxStatusValues },
      { property: "policyEnforcement", values: projectPolicyEnforcementValues },
    ],
    propertyRefs: [{ property: "runtimeRef", ref: "RuntimeRef" }],
    arrayItemRefs: [
      { property: "previewPorts", ref: "BoundaryPort" },
      { property: "secretRefs", ref: "SecretRef" },
      { property: "credentialRefs", ref: "BoundaryCredentialRef" },
      { property: "checks", ref: "BoundaryCheck" },
    ],
  },
  {
    schema: "BoundaryCheck",
    required: ["id", "label", "status", "message"],
    properties: ["id", "label", "status", "message", "evidence"],
    enumProperties: [{ property: "status", values: boundaryCheckStatusValues }],
  },
  {
    schema: "BoundaryPort",
    required: ["name", "port", "protocol"],
    properties: ["name", "port", "protocol"],
  },
  {
    schema: "BoundaryCredentialRef",
    required: ["id", "name", "slug", "type", "secretRef"],
    properties: ["id", "name", "slug", "type", "target", "secretRef", "usage"],
    enumProperties: [{ property: "type", values: projectCredentialTypeValues }],
  },
  {
    schema: "Project",
    required: ["id", "name", "slug", "defaultNamespace"],
    properties: [
      "id",
      "name",
      "slug",
      "repositoryUrl",
      "defaultNamespace",
      "defaultTemplateId",
      "metadata",
      "createdAt",
      "updatedAt",
    ],
  },
  {
    schema: "ProjectCreate",
    required: ["name", "defaultNamespace"],
    properties: ["name", "slug", "repositoryUrl", "defaultNamespace", "metadata"],
  },
  {
    schema: "ProjectUpdate",
    properties: ["name", "repositoryUrl", "defaultNamespace", "defaultTemplateId", "metadata"],
    absentProperties: ["id", "slug"],
  },
  {
    schema: "ProjectPolicy",
    required: ["projectId", "enforcement"],
    properties: [
      "projectId",
      "enforcement",
      "allowedImagePrefixes",
      "allowedServiceAccounts",
      "allowedSecretRefs",
      "createdAt",
      "updatedAt",
    ],
    enumProperties: [{ property: "enforcement", values: projectPolicyEnforcementValues }],
  },
  {
    schema: "ProjectPolicyUpsert",
    required: ["enforcement"],
    properties: ["enforcement", "allowedImagePrefixes", "allowedServiceAccounts", "allowedSecretRefs"],
    enumProperties: [{ property: "enforcement", values: projectPolicyEnforcementValues }],
  },
  {
    schema: "ProjectQuotaPolicy",
    required: ["projectId", "enforcement"],
    properties: ["projectId", "enforcement", "maxActiveSandboxes", "maxRetainedArtifactBytes", "createdAt", "updatedAt"],
    enumProperties: [{ property: "enforcement", values: projectPolicyEnforcementValues }],
  },
  {
    schema: "ProjectQuotaPolicyUpsert",
    required: ["enforcement"],
    properties: ["enforcement", "maxActiveSandboxes", "maxRetainedArtifactBytes"],
    enumProperties: [{ property: "enforcement", values: projectPolicyEnforcementValues }],
  },
  {
    schema: "ProjectMember",
    required: ["id", "projectId", "principalType", "principal", "role"],
    properties: ["id", "projectId", "principalType", "principal", "role", "metadata", "createdAt", "updatedAt"],
    enumProperties: [
      { property: "principalType", values: projectMemberPrincipalTypeValues },
      { property: "role", values: projectMemberRoleValues },
    ],
  },
  {
    schema: "ProjectMemberCreate",
    required: ["principalType", "principal", "role"],
    properties: ["principalType", "principal", "role", "metadata"],
    enumProperties: [
      { property: "principalType", values: projectMemberPrincipalTypeValues },
      { property: "role", values: projectMemberRoleValues },
    ],
  },
  {
    schema: "APIInfo",
    required: [
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
    ],
    properties: [
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
    ],
    propertyRefs: [
      { property: "runtimeController", ref: "RuntimeInfo" },
      { property: "runtimeAccess", ref: "RuntimeInfo" },
      { property: "artifactContent", ref: "ArtifactInfo" },
      { property: "trustedPrincipalHeaders", ref: "TrustedPrincipalHeaderInfo" },
      { property: "projectRbac", ref: "ProjectRBACInfo" },
      { property: "compatibility", ref: "Compatibility" },
    ],
  },
  {
    schema: "RuntimeInfo",
    required: ["enabled"],
    properties: ["enabled", "adapter"],
  },
  {
    schema: "ArtifactInfo",
    required: ["retainedContentEnabled", "storageProvider", "maxBytes"],
    properties: ["retainedContentEnabled", "storageProvider", "maxBytes"],
  },
  {
    schema: "Compatibility",
    required: ["minimumCliApiVersion", "minimumSdkApiVersion"],
    properties: ["minimumCliApiVersion", "minimumSdkApiVersion"],
  },
  {
    schema: "TrustedPrincipalHeaderInfo",
    required: ["enabled"],
    properties: ["enabled", "principalHeader", "principalTypeHeader"],
  },
  {
    schema: "ProjectRBACInfo",
    required: ["enforcementEnabled", "enforcedActions"],
    properties: ["enforcementEnabled", "enforcedActions"],
  },
  {
    schema: "ProjectAuthorizationDecision",
    required: [
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
    properties: [
      "projectId",
      "action",
      "allowed",
      "enforced",
      "evaluation",
      "requiredRoles",
      "caller",
      "matchedMember",
      "memberCount",
      "availableActions",
      "notes",
    ],
    enumProperties: [
      { property: "action", values: projectAuthorizationActionValues },
      { property: "evaluation", values: ["allowed", "denied", "not_enforceable"] },
    ],
    propertyRefs: [
      { property: "caller", ref: "CallerInfo" },
      { property: "matchedMember", ref: "ProjectMember" },
    ],
  },
  {
    schema: "CallerInfo",
    required: [
      "authenticated",
      "authenticationRequired",
      "mode",
      "principalType",
      "principal",
      "rbacTrusted",
      "projectRolesEnforced",
      "notes",
    ],
    properties: [
      "authenticated",
      "authenticationRequired",
      "mode",
      "principalType",
      "principal",
      "rbacTrusted",
      "projectRolesEnforced",
      "notes",
    ],
    enumProperties: [
      { property: "mode", values: callerAuthModeValues },
      { property: "principalType", values: callerPrincipalTypeValues },
    ],
  },
  {
    schema: "ProjectSandboxUsage",
    required: [
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
    ],
    propertyRefs: [
      { property: "activeRequests", ref: "SandboxResourceRequestUsage" },
      { property: "runningRequests", ref: "SandboxResourceRequestUsage" },
    ],
  },
  {
    schema: "SandboxResourceRequestUsage",
    required: ["count", "cpu", "memory", "storage"],
    propertyRefs: [
      { property: "cpu", ref: "ResourceQuantityUsage" },
      { property: "memory", ref: "ResourceQuantityUsage" },
      { property: "storage", ref: "ResourceQuantityUsage" },
    ],
  },
  {
    schema: "ResourceQuantityUsage",
    required: ["declared", "missing", "invalid"],
    properties: ["total", "declared", "missing", "invalid"],
  },
  {
    schema: "ProjectSessionUsage",
    required: ["total", "active", "ended", "failed", "terminal", "ide", "notebook", "browser", "command", "custom"],
  },
  {
    schema: "ProjectTaskUsage",
    required: ["total", "queued", "running", "succeeded", "failed", "canceled", "timedOut"],
  },
  {
    schema: "ProjectArtifactUsage",
    required: [
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
    ],
  },
  {
    schema: "ProjectTemplateUsage",
    required: ["projectScoped", "globalVisible"],
    properties: ["projectScoped", "globalVisible", "cpuRequests", "memoryRequests", "storageRequests"],
    arrayItemRefs: [
      { property: "cpuRequests", ref: "ResourceUsageValue" },
      { property: "memoryRequests", ref: "ResourceUsageValue" },
      { property: "storageRequests", ref: "ResourceUsageValue" },
    ],
  },
  {
    schema: "ProjectCredentialUsage",
    required: ["total", "git", "registry", "kubernetes", "ssh", "generic"],
  },
  {
    schema: "SecretRef",
    required: ["name"],
    properties: ["name", "key"],
  },
  {
    schema: "ProjectCredential",
    required: ["id", "projectId", "name", "slug", "type", "secretRef"],
    properties: [
      "id",
      "projectId",
      "name",
      "slug",
      "type",
      "target",
      "secretRef",
      "usage",
      "metadata",
      "createdAt",
      "updatedAt",
    ],
    enumProperties: [{ property: "type", values: projectCredentialTypeValues }],
    propertyRefs: [{ property: "secretRef", ref: "SecretRef" }],
  },
  {
    schema: "ProjectCredentialCreate",
    required: ["name", "type", "secretRef"],
    properties: ["name", "slug", "type", "target", "secretRef", "usage", "metadata"],
    enumProperties: [{ property: "type", values: projectCredentialTypeValues }],
    propertyRefs: [{ property: "secretRef", ref: "SecretRef" }],
  },
  {
    schema: "TemplatePort",
    required: ["name", "port", "protocol"],
    properties: ["name", "port", "protocol"],
  },
  {
    schema: "EnvironmentTemplate",
    required: ["id", "name", "slug", "image"],
    properties: [
      "id",
      "projectId",
      "name",
      "slug",
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
      "createdAt",
      "updatedAt",
    ],
    arrayItemRefs: [
      { property: "exposedPorts", ref: "TemplatePort" },
      { property: "secretRefs", ref: "SecretRef" },
    ],
  },
  {
    schema: "TemplateCreate",
    required: ["name", "image"],
    properties: [
      "projectId",
      "name",
      "slug",
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
    ],
    arrayItemRefs: [
      { property: "exposedPorts", ref: "TemplatePort" },
      { property: "secretRefs", ref: "SecretRef" },
    ],
  },
  {
    schema: "TemplateUpdate",
    properties: [
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
    ],
    absentProperties: ["projectId", "slug"],
    arrayItemRefs: [
      { property: "exposedPorts", ref: "TemplatePort" },
      { property: "secretRefs", ref: "SecretRef" },
    ],
  },
  {
    schema: "TemplateValidationRunCreate",
    properties: ["projectId", "name", "metadata"],
  },
  {
    schema: "TemplateValidationRunDecision",
    required: ["status"],
    properties: ["status"],
    enumProperties: [{ property: "status", values: templateValidationDecisionStatusValues }],
  },
  {
    schema: "TemplateValidationRun",
    required: ["template", "sandbox"],
    properties: ["template", "sandbox"],
    propertyRefs: [
      { property: "template", ref: "EnvironmentTemplate" },
      { property: "sandbox", ref: "Sandbox" },
    ],
  },
  {
    schema: "RuntimeRef",
    required: ["kind", "namespace", "name"],
    properties: ["adapter", "kind", "namespace", "name"],
  },
  {
    schema: "SandboxPort",
    required: ["name", "port", "protocol"],
    properties: ["name", "port", "protocol", "previewUrl"],
  },
  {
    schema: "Sandbox",
    required: ["id", "projectId", "name", "slug", "status", "namespace", "serviceAccountName"],
    properties: [
      "id",
      "projectId",
      "templateId",
      "name",
      "slug",
      "status",
      "namespace",
      "serviceAccountName",
      "runtimeRef",
      "ports",
      "metadata",
      "createdAt",
      "updatedAt",
      "deletedAt",
    ],
    enumProperties: [{ property: "status", values: sandboxStatusValues }],
    propertyRefs: [{ property: "runtimeRef", ref: "RuntimeRef" }],
    arrayItemRefs: [{ property: "ports", ref: "SandboxPort" }],
  },
  {
    schema: "SandboxCreate",
    required: ["projectId", "name"],
    properties: ["projectId", "templateId", "name", "slug", "namespace", "serviceAccountName", "metadata"],
  },
  {
    schema: "SandboxUpdate",
    properties: ["name", "status", "namespace", "serviceAccountName", "runtimeRef", "ports", "metadata"],
    absentProperties: ["projectId", "templateId", "slug"],
    enumProperties: [{ property: "status", values: sandboxStatusValues }],
    propertyRefs: [{ property: "runtimeRef", ref: "RuntimeRef" }],
    arrayItemRefs: [{ property: "ports", ref: "SandboxPort" }],
  },
  {
    schema: "PreviewPort",
    required: ["name", "port", "protocol", "available"],
    properties: ["name", "port", "protocol", "previewUrl", "available", "message"],
  },
  {
    schema: "PreviewPortsResult",
    required: ["target", "items"],
    properties: ["target", "items"],
    propertyRefs: [{ property: "target", ref: "RuntimeTarget" }],
    arrayItemRefs: [{ property: "items", ref: "PreviewPort" }],
  },
  {
    schema: "RuntimeSession",
    required: ["id", "projectId", "sandboxId", "type", "status", "startedAt"],
    properties: [
      "id",
      "projectId",
      "sandboxId",
      "type",
      "status",
      "client",
      "userAgent",
      "runtimeRef",
      "metadata",
      "startedAt",
      "endedAt",
      "createdAt",
      "updatedAt",
    ],
    enumProperties: [
      { property: "type", values: runtimeSessionTypeValues },
      { property: "status", values: runtimeSessionStatusValues },
    ],
    propertyRefs: [{ property: "runtimeRef", ref: "RuntimeRef" }],
  },
  {
    schema: "RuntimeSessionCreate",
    required: ["type"],
    properties: ["type", "client", "metadata"],
    enumProperties: [{ property: "type", values: runtimeSessionTypeValues }],
  },
  {
    schema: "Artifact",
    required: ["id", "projectId", "sandboxId", "kind", "name", "uri"],
    properties: [
      "id",
      "projectId",
      "sandboxId",
      "taskId",
      "kind",
      "name",
      "uri",
      "contentType",
      "sizeBytes",
      "metadata",
      "retainedContent",
      "createdAt",
      "updatedAt",
    ],
    enumProperties: [{ property: "kind", values: artifactKindValues }],
    propertyRefs: [{ property: "retainedContent", ref: "ArtifactContent" }],
  },
  {
    schema: "ArtifactCreate",
    required: ["kind", "name", "uri"],
    properties: ["taskId", "kind", "name", "uri", "contentType", "sizeBytes", "metadata"],
    enumProperties: [{ property: "kind", values: artifactKindValues }],
  },
  {
    schema: "ArtifactContent",
    required: ["artifactId", "sizeBytes", "sha256", "sourceUri", "storageProvider", "capturedAt"],
    properties: ["artifactId", "contentType", "sizeBytes", "sha256", "sourceUri", "storageProvider", "storageKey", "capturedAt"],
    enumProperties: [{ property: "storageProvider", values: artifactStorageProviderValues }],
  },
  {
    schema: "AuditEvent",
    required: ["id", "action", "resourceType", "createdAt"],
    properties: [
      "id",
      "projectId",
      "action",
      "resourceType",
      "resourceId",
      "resourceName",
      "actor",
      "source",
      "metadata",
      "createdAt",
    ],
    propertyRefs: [
      { property: "action", ref: "AuditEventAction" },
      { property: "metadata", ref: "AuditEventMetadata" },
    ],
  },
  {
    schema: "PolicyDeniedAuditMetadata",
    required: ["operation", "reason"],
    properties: [
      "operation",
      "reason",
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
    ],
    enumProperties: [
      { property: "operation", values: policyDeniedOperationValues },
      { property: "authorizationAction", values: projectAuthorizationActionValues },
      { property: "callerMode", values: callerAuthModeValues },
      { property: "callerPrincipalType", values: callerPrincipalTypeValues },
      { property: "policyKind", values: policyDeniedPolicyKindValues },
      { property: "enforcement", values: projectPolicyEnforcementValues },
      { property: "principalType", values: projectMemberPrincipalTypeValues },
      { property: "role", values: projectMemberRoleValues },
    ],
  },
] as const satisfies readonly SDKSchemaContractEntry[]

export class OpenAPIAlignmentError extends Error {
  readonly result: SDKOpenAPIAlignmentResult

  constructor(result: SDKOpenAPIAlignmentResult) {
    super(openAPIAlignmentMessage(result))
    this.name = "OpenAPIAlignmentError"
    this.result = result
  }
}

export function checkOpenAPIAlignment(
  document: OpenAPIDocument,
  routes: readonly SDKRouteContractEntry[] = SDK_ROUTE_CONTRACT,
  schemas: readonly SDKSchemaContractEntry[] = SDK_SCHEMA_CONTRACT,
): SDKOpenAPIAlignmentResult {
  const paths = isRecord(document.paths) ? document.paths : {}
  const componentSchemas = isRecord(document.components?.schemas) ? document.components.schemas : {}
  const missing: SDKOpenAPIAlignmentIssue[] = []
  let checkedQueryParams = 0
  let checkedAuth = 0
  let checkedRequests = 0
  let checkedResponses = 0
  let checkedPublishedOperations = 0
  let ignoredPublishedOperations = 0
  let checkedSchemaRequired = 0
  let checkedSchemaProperties = 0
  let checkedSchemaAbsentProperties = 0
  let checkedSchemaEnumValues = 0
  let checkedSchemaPropertyRefs = 0
  let checkedSchemaArrayItemRefs = 0
  const routeCoverage = sdkRouteCoverage(routes)

  for (const operation of openAPIOperations(paths)) {
    if (isSDKRouteCoverageException(operation)) {
      ignoredPublishedOperations += 1
      continue
    }
    checkedPublishedOperations += 1
    if (!routeCoverage.has(routeKey(operation))) {
      missing.push({ reason: "missing-sdk-route-coverage", method: operation.method, path: operation.path })
    }
  }

  for (const route of routes) {
    const pathItem = paths[route.path]
    if (!isRecord(pathItem)) {
      missing.push({ ...route, reason: "missing-path" })
      continue
    }
    const operation = pathItem[route.method.toLowerCase()]
    if (!isRecord(operation)) {
      missing.push({ ...route, reason: "missing-method" })
      continue
    }
    checkedAuth += 1
    checkRouteAuth(route, operation, document, missing)
    const expectedQuery = route.query ?? []
    checkedQueryParams += expectedQuery.length
    if (expectedQuery.length > 0) {
      const queryParams = queryParameterNames(operation)
      for (const parameter of expectedQuery) {
        if (!queryParams.has(parameter)) {
          missing.push({ ...route, reason: "missing-query-param", parameter })
        }
      }
    }
    if (route.request) {
      checkedRequests += 1
      checkRouteRequest(route, operation, missing)
    }
    if (route.response) {
      checkedResponses += 1
      checkRouteResponse(route, operation, missing)
    }
  }

  for (const schemaContract of schemas) {
    const schema = componentSchemas[schemaContract.schema]
    if (!isRecord(schema)) {
      missing.push({ ...schemaContract, reason: "missing-schema" })
      continue
    }
    const required = stringSet(schema.required)
    for (const property of schemaContract.required ?? []) {
      checkedSchemaRequired += 1
      if (!required.has(property)) {
        missing.push({ ...schemaContract, reason: "missing-schema-required", property })
      }
    }
    const properties = propertyNames(schema)
    for (const property of schemaContract.properties ?? []) {
      checkedSchemaProperties += 1
      if (!properties.has(property)) {
        missing.push({ ...schemaContract, reason: "missing-schema-property", property })
      }
    }
    for (const property of schemaContract.absentProperties ?? []) {
      checkedSchemaAbsentProperties += 1
      if (properties.has(property)) {
        missing.push({ ...schemaContract, reason: "unexpected-schema-property", property })
      }
    }
    for (const enumProperty of schemaContract.enumProperties ?? []) {
      const values = schemaPropertyEnumValues(schema, enumProperty.property, componentSchemas)
      for (const value of enumProperty.values) {
        checkedSchemaEnumValues += 1
        if (!values.has(value)) {
          missing.push({
            ...schemaContract,
            reason: "missing-schema-enum-value",
            property: enumProperty.property,
            enumValue: value,
          })
        }
      }
    }
    for (const propertyRef of schemaContract.propertyRefs ?? []) {
      checkedSchemaPropertyRefs += 1
      const actualRef = schemaPropertyRefName(schema, propertyRef.property)
      if (actualRef !== propertyRef.ref) {
        missing.push({
          ...schemaContract,
          reason: "schema-property-ref-mismatch",
          property: propertyRef.property,
          expectedSchema: propertyRef.ref,
          actualSchema: actualRef,
        })
      }
    }
    for (const itemRef of schemaContract.arrayItemRefs ?? []) {
      checkedSchemaArrayItemRefs += 1
      const actualRef = schemaArrayItemRefName(schema, itemRef.property)
      if (actualRef !== itemRef.ref) {
        missing.push({
          ...schemaContract,
          reason: "schema-array-item-ref-mismatch",
          property: itemRef.property,
          expectedSchema: itemRef.ref,
          actualSchema: actualRef,
        })
      }
    }
  }

  return {
    ok: missing.length === 0,
    checked: routes.length,
    checkedQueryParams,
    checkedAuth,
    checkedRequests,
    checkedResponses,
    checkedPublishedOperations,
    ignoredPublishedOperations,
    checkedSchemas: schemas.length,
    checkedSchemaRequired,
    checkedSchemaProperties,
    checkedSchemaAbsentProperties,
    checkedSchemaEnumValues,
    checkedSchemaPropertyRefs,
    checkedSchemaArrayItemRefs,
    missing,
  }
}

export function assertOpenAPIAlignment(
  document: OpenAPIDocument,
  routes: readonly SDKRouteContractEntry[] = SDK_ROUTE_CONTRACT,
  schemas: readonly SDKSchemaContractEntry[] = SDK_SCHEMA_CONTRACT,
): SDKOpenAPIAlignmentResult {
  const result = checkOpenAPIAlignment(document, routes, schemas)
  if (!result.ok) {
    throw new OpenAPIAlignmentError(result)
  }
  return result
}

export async function fetchAndAssertOpenAPIAlignment(
  client: Pick<MboxClient, "openAPI">,
  routes: readonly SDKRouteContractEntry[] = SDK_ROUTE_CONTRACT,
  schemas: readonly SDKSchemaContractEntry[] = SDK_SCHEMA_CONTRACT,
) {
  return assertOpenAPIAlignment(await client.openAPI(), routes, schemas)
}

function openAPIAlignmentMessage(result: SDKOpenAPIAlignmentResult) {
  if (result.ok) {
    return `OpenAPI covers ${result.checked} SDK route entries, ${result.checkedPublishedOperations} published route operations, ${result.checkedQueryParams} SDK-used query parameters, ${result.checkedAuth} SDK route auth contracts, ${result.checkedRequests} SDK helper request contracts, ${result.checkedResponses} SDK helper response contracts, ${result.checkedSchemas} SDK schema contracts, ${result.checkedSchemaEnumValues} SDK schema enum values, ${result.checkedSchemaPropertyRefs} SDK schema property refs, and ${result.checkedSchemaArrayItemRefs} SDK schema array item refs`
  }
  const preview = result.missing
    .slice(0, 10)
    .map((issue) => {
      if (issue.schema) {
        const property = issue.property ? `:${issue.property}` : ""
        const enumValue = issue.enumValue ? `=${issue.enumValue}` : ""
        return `${issue.schema} (${issue.reason}${property}${enumValue})`
      }
      if (issue.reason === "missing-sdk-route-coverage") {
        return `OpenAPI operation ${issue.method} ${issue.path} (${issue.reason})`
      }
      if (issue.responseStatus || issue.mediaType || issue.expectedSchema) {
        const parts = [
          issue.responseStatus ? `status=${issue.responseStatus}` : undefined,
          issue.mediaType ? `media=${issue.mediaType}` : undefined,
          issue.expectedSchema ? `expected=${issue.expectedSchema}` : undefined,
          issue.actualSchema ? `actual=${issue.actualSchema}` : undefined,
          issue.property ? `property=${issue.property}` : undefined,
        ].filter(Boolean)
        return `${String(issue.sdk)}: ${issue.method} ${issue.path} (${issue.reason}${
          parts.length ? ` ${parts.join(",")}` : ""
        })`
      }
      if (issue.expectedAuth || issue.actualAuth) {
        const parts = [
          issue.expectedAuth ? `expected=${issue.expectedAuth}` : undefined,
          issue.actualAuth ? `actual=${issue.actualAuth}` : undefined,
        ].filter(Boolean)
        return `${String(issue.sdk)}: ${issue.method} ${issue.path} (${issue.reason}${
          parts.length ? ` ${parts.join(",")}` : ""
        })`
      }
      const parameter = issue.parameter ? `:${issue.parameter}` : ""
      return `${String(issue.sdk)}: ${issue.method} ${issue.path} (${issue.reason}${parameter})`
    })
    .join("; ")
  const suffix = result.missing.length > 10 ? `; +${result.missing.length - 10} more` : ""
  return `OpenAPI/SDK alignment has ${result.missing.length} issue(s) across ${result.checked} SDK route entries and ${result.checkedPublishedOperations} published route operations: ${preview}${suffix}`
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value)
}

function queryParameterNames(operation: Record<string, unknown>) {
  const names = new Set<string>()
  const parameters = Array.isArray(operation.parameters) ? operation.parameters : []
  for (const parameter of parameters) {
    if (!isRecord(parameter) || parameter.in !== "query" || typeof parameter.name !== "string") {
      continue
    }
    names.add(parameter.name)
  }
  return names
}

function sdkRouteCoverage(routes: readonly SDKRouteContractEntry[]) {
  return new Set(routes.map((route) => routeKey(route)))
}

function openAPIOperations(paths: Record<string, unknown>) {
  const operations: Array<{ method: SDKRouteMethod; path: string }> = []
  for (const [path, pathItem] of Object.entries(paths)) {
    if (!isRecord(pathItem)) {
      continue
    }
    for (const method of OPENAPI_METHODS) {
      if (isRecord(pathItem[method.toLowerCase()])) {
        operations.push({ method, path })
      }
    }
  }
  return operations
}

function routeKey(route: { method: SDKRouteMethod; path: string }) {
  return `${route.method} ${route.path}`
}

function isSDKRouteCoverageException(route: { method: SDKRouteMethod; path: string }) {
  return SDK_ROUTE_COVERAGE_EXCEPTIONS.some((exception) => exception.method === route.method && exception.path === route.path)
}

function checkRouteAuth(
  route: SDKRouteContractEntry,
  operation: Record<string, unknown>,
  document: OpenAPIDocument,
  missing: SDKOpenAPIAlignmentIssue[],
) {
  const expectedAuth = route.auth ?? "bearer"
  if (expectedAuth === "none") {
    if (!isPublicSecurity(operation.security)) {
      missing.push({ ...route, reason: "security-mismatch", expectedAuth, actualAuth: securitySummary(operation.security) })
    }
    return
  }

  if (!hasBearerSecurityScheme(document)) {
    missing.push({ ...route, reason: "missing-security-scheme", expectedAuth })
  }
  if (!requiresBearerSecurity(operation.security)) {
    missing.push({ ...route, reason: "security-mismatch", expectedAuth, actualAuth: securitySummary(operation.security) })
  }
  if (!hasUnauthorizedResponse(operation)) {
    missing.push({ ...route, reason: "missing-unauthorized-response", responseStatus: "401" })
  }
}

function checkRouteRequest(
  route: SDKRouteContractEntry,
  operation: Record<string, unknown>,
  missing: SDKOpenAPIAlignmentIssue[],
) {
  const expected = route.request
  if (!expected) {
    return
  }
  const requestBody = operation.requestBody
  if (!isRecord(requestBody)) {
    missing.push({ ...route, reason: "missing-request-body" })
    return
  }
  const content = isRecord(requestBody.content) ? requestBody.content : undefined
  for (const mediaType of expected.mediaTypes ?? [expected.mediaType ?? "application/json"]) {
    const media = content?.[mediaType]
    if (!isRecord(media)) {
      missing.push({ ...route, reason: "missing-request-content", mediaType })
      continue
    }
    const schema = media.schema
    if (!isRecord(schema)) {
      missing.push({ ...route, reason: "missing-request-schema", mediaType })
      continue
    }
    if (expected.binary) {
      if (!isBinarySchema(schema)) {
        missing.push({ ...route, reason: "request-binary-mismatch", mediaType })
      }
      continue
    }
    if (expected.schema) {
      const actualSchema = schemaRefName(schema)
      if (actualSchema !== expected.schema) {
        missing.push({
          ...route,
          reason: "request-schema-mismatch",
          mediaType,
          expectedSchema: expected.schema,
          actualSchema,
        })
      }
    }
  }
}

function checkRouteResponse(
  route: SDKRouteContractEntry,
  operation: Record<string, unknown>,
  missing: SDKOpenAPIAlignmentIssue[],
) {
  const expected = route.response
  if (!expected) {
    return
  }
  const status = expected.status ?? (expected.noContent ? "204" : "200")
  const responses = isRecord(operation.responses) ? operation.responses : undefined
  const response = responses?.[status]
  if (!isRecord(response)) {
    missing.push({ ...route, reason: "missing-response", responseStatus: status })
    return
  }
  if (expected.noContent) {
    return
  }
  const mediaType = expected.mediaType ?? "application/json"
  const content = isRecord(response.content) ? response.content : undefined
  const media = content?.[mediaType]
  if (!isRecord(media)) {
    missing.push({ ...route, reason: "missing-response-content", responseStatus: status, mediaType })
    return
  }
  const schema = media.schema
  if (!isRecord(schema)) {
    missing.push({ ...route, reason: "missing-response-schema", responseStatus: status, mediaType })
    return
  }
  if (expected.binary) {
    if (!isBinarySchema(schema)) {
      missing.push({ ...route, reason: "response-binary-mismatch", responseStatus: status, mediaType })
    }
    return
  }
  if (expected.schema) {
    const actualSchema = schemaRefName(schema)
    if (actualSchema !== expected.schema) {
      missing.push({
        ...route,
        reason: "response-schema-mismatch",
        responseStatus: status,
        mediaType,
        expectedSchema: expected.schema,
        actualSchema,
      })
    }
  }
  if (expected.listItem) {
    const required = stringSet(schema.required)
    if (!required.has("items")) {
      missing.push({
        ...route,
        reason: "missing-response-required",
        responseStatus: status,
        mediaType,
        property: "items",
      })
    }
    const actualSchema = listItemRefName(schema)
    if (actualSchema !== expected.listItem) {
      missing.push({
        ...route,
        reason: "response-list-item-mismatch",
        responseStatus: status,
        mediaType,
        expectedSchema: expected.listItem,
        actualSchema,
      })
    }
  }
}

function isBinarySchema(schema: Record<string, unknown>) {
  return schema.type === "string" && schema.format === "binary"
}

function schemaRefName(schema: Record<string, unknown>) {
  if (typeof schema.$ref !== "string") {
    return undefined
  }
  const prefix = "#/components/schemas/"
  return schema.$ref.startsWith(prefix) ? schema.$ref.slice(prefix.length) : schema.$ref
}

function listItemRefName(schema: Record<string, unknown>) {
  if (!isRecord(schema.properties)) {
    return undefined
  }
  const itemsProperty = schema.properties.items
  if (!isRecord(itemsProperty) || !isRecord(itemsProperty.items)) {
    return undefined
  }
  return schemaRefName(itemsProperty.items)
}

function propertyNames(schema: Record<string, unknown>) {
  const names = new Set<string>()
  if (!isRecord(schema.properties)) {
    return names
  }
  for (const name of Object.keys(schema.properties)) {
    names.add(name)
  }
  return names
}

function schemaPropertyEnumValues(
  schema: Record<string, unknown>,
  property: string,
  componentSchemas?: Record<string, unknown>,
) {
  if (!isRecord(schema.properties)) {
    return new Set<string>()
  }
  const propertySchema = schema.properties[property]
  if (!isRecord(propertySchema)) {
    return new Set<string>()
  }
  const refName = schemaRefName(propertySchema)
  const resolvedSchema =
    refName && componentSchemas && isRecord(componentSchemas[refName]) ? componentSchemas[refName] : propertySchema
  return stringSet(resolvedSchema.enum)
}

function schemaPropertyRefName(schema: Record<string, unknown>, property: string) {
  if (!isRecord(schema.properties)) {
    return undefined
  }
  const propertySchema = schema.properties[property]
  if (!isRecord(propertySchema)) {
    return undefined
  }
  return schemaRefName(propertySchema)
}

function schemaArrayItemRefName(schema: Record<string, unknown>, property: string) {
  if (!isRecord(schema.properties)) {
    return undefined
  }
  const propertySchema = schema.properties[property]
  if (!isRecord(propertySchema) || propertySchema.type !== "array" || !isRecord(propertySchema.items)) {
    return undefined
  }
  return schemaRefName(propertySchema.items)
}

function stringSet(value: unknown) {
  const names = new Set<string>()
  if (!Array.isArray(value)) {
    return names
  }
  for (const item of value) {
    if (typeof item === "string") {
      names.add(item)
    }
  }
  return names
}

function hasBearerSecurityScheme(document: OpenAPIDocument) {
  const securitySchemes = isRecord(document.components?.securitySchemes) ? document.components.securitySchemes : {}
  const bearerAuth = securitySchemes.bearerAuth
  return isRecord(bearerAuth) && bearerAuth.type === "http" && bearerAuth.scheme === "bearer"
}

function isPublicSecurity(value: unknown) {
  return Array.isArray(value) && value.length === 0
}

function requiresBearerSecurity(value: unknown) {
  if (!Array.isArray(value)) {
    return false
  }
  return value.some((item) => isRecord(item) && Array.isArray(item.bearerAuth))
}

function hasUnauthorizedResponse(operation: Record<string, unknown>) {
  const responses = isRecord(operation.responses) ? operation.responses : undefined
  return isRecord(responses?.["401"])
}

function securitySummary(value: unknown) {
  if (value === undefined) {
    return "missing"
  }
  if (isPublicSecurity(value)) {
    return "none"
  }
  if (requiresBearerSecurity(value)) {
    return "bearer"
  }
  return "other"
}

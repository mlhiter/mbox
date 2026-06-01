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
    | "response-schema-mismatch"
    | "response-list-item-mismatch"
    | "response-binary-mismatch"
    | "missing-schema"
    | "missing-schema-required"
    | "missing-schema-property"
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
}

export type SDKOpenAPIAlignmentResult = {
  ok: boolean
  checked: number
  checkedQueryParams: number
  checkedAuth: number
  checkedRequests: number
  checkedResponses: number
  checkedSchemas: number
  checkedSchemaRequired: number
  checkedSchemaProperties: number
  missing: SDKOpenAPIAlignmentIssue[]
}

const noContentResponse = { status: "204", noContent: true } as const satisfies SDKRouteResponseContract

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
  { sdk: "openAPI", method: "GET", path: "/v1/openapi.json" },
  {
    sdk: "listRuntimeResources",
    method: "GET",
    path: "/v1/runtime/resources",
    query: ["namespace", "kind"],
    response: jsonResponse("RuntimeResourceList"),
  },
  {
    sdk: "listRuntimeOrphans",
    method: "GET",
    path: "/v1/runtime/orphans",
    query: ["namespace", "kind"],
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
    query: ["projectId", "action", "resourceType", "resourceId", "actor", "source", "requestId", "operation", "since", "until", "limit"],
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
    query: ["action", "resourceType", "resourceId", "actor", "source", "requestId", "operation", "since", "until", "limit"],
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
    schema: "RuntimeResourceList",
    required: ["adapter", "checkedAt", "summary", "items"],
    properties: ["adapter", "checkedAt", "summary", "items"],
  },
  {
    schema: "RuntimeResourceSummary",
    required: ["total", "byKind", "byNamespace", "byOwner", "workload"],
    properties: ["total", "byKind", "byNamespace", "byOwner", "workload"],
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
  },
  {
    schema: "RuntimeResourceOwner",
    required: ["kind"],
    properties: ["kind", "projectId", "sandboxId", "templateId"],
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
  },
  {
    schema: "RuntimeStorage",
    required: ["name", "mountPath"],
    properties: ["name", "mountPath", "claimName", "phase", "capacity", "storageClassName", "message"],
  },
  {
    schema: "RuntimeOrphanAudit",
    required: ["adapter", "checkedAt", "resourceCount", "orphanCount", "expectedClean", "items"],
    properties: ["adapter", "checkedAt", "namespace", "resourceCount", "orphanCount", "expectedClean", "items"],
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
  },
  {
    schema: "RuntimeOrphanCleanupResult",
    required: ["deleted", "resource", "reason", "message"],
    properties: ["deleted", "resource", "reason", "message"],
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
  },
  {
    schema: "SandboxResourceRequestUsage",
    required: ["count", "cpu", "memory", "storage"],
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
  },
  {
    schema: "ProjectCredentialUsage",
    required: ["total", "git", "registry", "kubernetes", "ssh", "generic"],
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
      "artifactKind",
      "incomingBytes",
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
  let checkedSchemaRequired = 0
  let checkedSchemaProperties = 0

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
  }

  return {
    ok: missing.length === 0,
    checked: routes.length,
    checkedQueryParams,
    checkedAuth,
    checkedRequests,
    checkedResponses,
    checkedSchemas: schemas.length,
    checkedSchemaRequired,
    checkedSchemaProperties,
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
    return `OpenAPI covers ${result.checked} SDK route entries, ${result.checkedQueryParams} SDK-used query parameters, ${result.checkedAuth} SDK route auth contracts, ${result.checkedRequests} SDK helper request contracts, ${result.checkedResponses} SDK helper response contracts, and ${result.checkedSchemas} SDK schema contracts`
  }
  const preview = result.missing
    .slice(0, 10)
    .map((issue) => {
      if (issue.schema) {
        const property = issue.property ? `:${issue.property}` : ""
        return `${issue.schema} (${issue.reason}${property})`
      }
      if (issue.responseStatus || issue.mediaType || issue.expectedSchema) {
        const parts = [
          issue.responseStatus ? `status=${issue.responseStatus}` : undefined,
          issue.mediaType ? `media=${issue.mediaType}` : undefined,
          issue.expectedSchema ? `expected=${issue.expectedSchema}` : undefined,
          issue.actualSchema ? `actual=${issue.actualSchema}` : undefined,
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
  return `OpenAPI is missing ${result.missing.length} of ${result.checked} SDK route entries: ${preview}${suffix}`
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

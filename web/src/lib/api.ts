import type {
  Artifact,
  ArtifactKind,
  AuditEvent,
  APIInfo,
  BoundarySummary,
  ExecutionTask,
  ExecutionTaskEvent,
  ListResponse,
  LogResult,
  PreviewPortsResult,
  Project,
  ProjectCredential,
  ProjectPolicy,
  ProjectQuotaPolicy,
  ProjectUsage,
  RuntimeEvent,
  RuntimeOrphanAudit,
  RuntimeOrphanCleanupRequest,
  RuntimeOrphanCleanupResult,
  RuntimeResourceList,
  RuntimeSession,
  RuntimeTarget,
  SandboxPort,
  Sandbox,
  Template,
  TemplateValidationRun,
} from "@/types"

export async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch(path, {
    headers: { "content-type": "application/json" },
    ...options,
  })
  if (!response.ok) {
    let message = `${response.status} ${response.statusText}`
    try {
      const body = (await response.json()) as { error?: string }
      message = body.error || message
    } catch {
      // Keep the HTTP status message.
    }
    throw new Error(message)
  }
  if (response.status === 204) {
    return undefined as T
  }
  return response.json() as Promise<T>
}

export async function rawRequest(path: string, options: RequestInit = {}) {
  const response = await fetch(path, options)
  if (!response.ok) {
    let message = `${response.status} ${response.statusText}`
    try {
      const body = (await response.json()) as { error?: string }
      message = body.error || message
    } catch {
      // Keep the HTTP status message.
    }
    throw new Error(message)
  }
  return response
}

export function getHealth() {
  return request<{ status?: string }>("/healthz")
}

export function getInfo() {
  return request<APIInfo>("/v1/info")
}

export function getRuntimeOrphans(namespace?: string) {
  const query = namespace ? `?namespace=${encodeURIComponent(namespace)}` : ""
  return request<RuntimeOrphanAudit>(`/v1/runtime/orphans${query}`)
}

export function getRuntimeResources(options: { namespace?: string; kind?: string } = {}) {
  const query = new URLSearchParams()
  if (options.namespace?.trim()) {
    query.set("namespace", options.namespace.trim())
  }
  if (options.kind?.trim()) {
    query.set("kind", options.kind.trim())
  }
  const suffix = query.toString() ? `?${query.toString()}` : ""
  return request<RuntimeResourceList>(`/v1/runtime/resources${suffix}`)
}

export function cleanupRuntimeOrphan(payload: RuntimeOrphanCleanupRequest) {
  return request<RuntimeOrphanCleanupResult>("/v1/runtime/orphans/cleanup", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function listProjects() {
  return request<ListResponse<Project>>("/v1/projects")
}

export function getProjectPolicy(projectID: string) {
  return request<ProjectPolicy>(`/v1/projects/${projectID}/policy`)
}

export function getProjectQuotaPolicy(projectID: string) {
  return request<ProjectQuotaPolicy>(`/v1/projects/${projectID}/quota-policy`)
}

export function listProjectCredentials(projectID: string) {
  return request<ListResponse<ProjectCredential>>(`/v1/projects/${projectID}/credentials`)
}

export function getProjectUsage(projectID: string) {
  return request<ProjectUsage>(`/v1/projects/${projectID}/usage`)
}

export type AuditEventListOptions = {
  limit?: number
  action?: string
  actor?: string
  source?: string
  resourceType?: string
  resourceId?: string
  requestId?: string
  operation?: string
  since?: string
  until?: string
}

export function listProjectAuditEvents(projectID: string, options: AuditEventListOptions = {}) {
  const query = new URLSearchParams()
  query.set("limit", String(options.limit ?? 8))
  if (options.action?.trim()) {
    query.set("action", options.action.trim())
  }
  if (options.actor?.trim()) {
    query.set("actor", options.actor.trim())
  }
  if (options.source?.trim()) {
    query.set("source", options.source.trim())
  }
  if (options.resourceType?.trim()) {
    query.set("resourceType", options.resourceType.trim())
  }
  if (options.resourceId?.trim()) {
    query.set("resourceId", options.resourceId.trim())
  }
  if (options.requestId?.trim()) {
    query.set("requestId", options.requestId.trim())
  }
  if (options.operation?.trim()) {
    query.set("operation", options.operation.trim())
  }
  if (options.since?.trim()) {
    query.set("since", options.since.trim())
  }
  if (options.until?.trim()) {
    query.set("until", options.until.trim())
  }
  return request<ListResponse<AuditEvent>>(`/v1/projects/${projectID}/audit-events?${query.toString()}`)
}

export function listTemplates() {
  return request<ListResponse<Template>>("/v1/templates")
}

export function listSandboxes() {
  return request<ListResponse<Sandbox>>("/v1/sandboxes")
}

export function createProject(payload: Partial<Project>) {
  return request<Project>("/v1/projects", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function createTemplate(payload: Partial<Template>) {
  return request<Template>("/v1/templates", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function updateTemplate(id: string, payload: Partial<Template>) {
  return request<Template>(`/v1/templates/${id}`, {
    method: "PATCH",
    body: JSON.stringify(payload),
  })
}

export function createTemplateValidationRun(
  templateID: string,
  payload: { projectId?: string; name?: string; metadata?: Record<string, unknown> } = {},
) {
  return request<TemplateValidationRun>(`/v1/templates/${templateID}/validation-runs`, {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function decideTemplateValidationRun(templateID: string, sandboxID: string, status: "passed" | "failed") {
  return request<TemplateValidationRun>(`/v1/templates/${templateID}/validation-runs/${sandboxID}/decision`, {
    method: "POST",
    body: JSON.stringify({ status }),
  })
}

export function getTemplateBoundary(templateID: string, projectID?: string) {
  const query = projectID ? `?projectId=${encodeURIComponent(projectID)}` : ""
  return request<BoundarySummary>(`/v1/templates/${templateID}/boundary${query}`)
}

export function updateProject(id: string, payload: Partial<Project>) {
  return request<Project>(`/v1/projects/${id}`, {
    method: "PATCH",
    body: JSON.stringify(payload),
  })
}

export function createSandbox(payload: Partial<Sandbox>) {
  return request<Sandbox>("/v1/sandboxes", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function getSandbox(id: string) {
  return request<Sandbox>(`/v1/sandboxes/${id}`)
}

export function getSandboxBoundary(id: string) {
  return request<BoundarySummary>(`/v1/sandboxes/${id}/boundary`)
}

export function updateSandboxPorts(id: string, ports: SandboxPort[]) {
  return request<Sandbox>(`/v1/sandboxes/${id}`, {
    method: "PATCH",
    body: JSON.stringify({ ports }),
  })
}

export function updateSandbox(id: string, payload: Partial<Sandbox>) {
  return request<Sandbox>(`/v1/sandboxes/${id}`, {
    method: "PATCH",
    body: JSON.stringify(payload),
  })
}

export function deleteSandbox(id: string) {
  return request<void>(`/v1/sandboxes/${id}`, { method: "DELETE" })
}

export function startSandbox(id: string) {
  return request<Sandbox>(`/v1/sandboxes/${id}/start`, { method: "POST" })
}

export function stopSandbox(id: string) {
  return request<Sandbox>(`/v1/sandboxes/${id}/stop`, { method: "POST" })
}

export function getRuntimeTarget(sandboxID: string) {
  return request<RuntimeTarget>(`/v1/sandboxes/${sandboxID}/runtime`)
}

export function getRuntimeLogs(sandboxID: string) {
  return request<LogResult>(`/v1/sandboxes/${sandboxID}/logs?tailLines=120`)
}

export function getRuntimeEvents(sandboxID: string) {
  return request<ListResponse<RuntimeEvent>>(`/v1/sandboxes/${sandboxID}/events`)
}

export function getPreviewPorts(sandboxID: string) {
  return request<PreviewPortsResult>(`/v1/sandboxes/${sandboxID}/ports`)
}

export function listExecutionTasks(sandboxID: string) {
  return request<ListResponse<ExecutionTask>>(`/v1/sandboxes/${sandboxID}/tasks`)
}

export function listRuntimeSessions(sandboxID: string) {
  return request<ListResponse<RuntimeSession>>(`/v1/sandboxes/${sandboxID}/sessions`)
}

export function listArtifacts(sandboxID: string) {
  return request<ListResponse<Artifact>>(`/v1/sandboxes/${sandboxID}/artifacts`)
}

export function createArtifact(
  sandboxID: string,
  payload: {
    kind: ArtifactKind
    name: string
    uri: string
    taskId?: string
    contentType?: string
    sizeBytes?: number
    metadata?: Record<string, unknown>
  },
) {
  return request<Artifact>(`/v1/sandboxes/${sandboxID}/artifacts`, {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function captureArtifactContent(artifactID: string) {
  return request<Artifact>(`/v1/artifacts/${artifactID}/capture`, {
    method: "POST",
  })
}

export function createExecutionTask(
  sandboxID: string,
  payload: { command: string[]; timeoutSeconds?: number; metadata?: Record<string, unknown> },
) {
  return request<ExecutionTask>(`/v1/sandboxes/${sandboxID}/tasks`, {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function cancelExecutionTask(taskID: string) {
  return request<ExecutionTask>(`/v1/tasks/${taskID}/cancel`, {
    method: "POST",
  })
}

export async function watchExecutionTask(
  taskID: string,
  options: {
    signal?: AbortSignal
    onEvent: (event: ExecutionTaskEvent) => void
  },
) {
  const response = await rawRequest(`/v1/tasks/${taskID}/events`, { signal: options.signal })
  if (!response.body) {
    return
  }
  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ""
  try {
    for (;;) {
      const { done, value } = await reader.read()
      if (done) {
        break
      }
      buffer += decoder.decode(value, { stream: true })
      let newline = buffer.indexOf("\n")
      while (newline >= 0) {
        const line = buffer.slice(0, newline).trim()
        buffer = buffer.slice(newline + 1)
        if (line) {
          options.onEvent(JSON.parse(line) as ExecutionTaskEvent)
        }
        newline = buffer.indexOf("\n")
      }
    }
    buffer += decoder.decode()
    const line = buffer.trim()
    if (line) {
      options.onEvent(JSON.parse(line) as ExecutionTaskEvent)
    }
  } finally {
    reader.releaseLock()
  }
}

export function artifactContentURL(artifactID: string) {
  return `/v1/artifacts/${artifactID}/content`
}

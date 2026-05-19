import type {
  ListResponse,
  LogResult,
  PreviewPortsResult,
  Project,
  RuntimeEvent,
  RuntimeTarget,
  SandboxPort,
  Sandbox,
  Template,
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

export function getHealth() {
  return request<{ status?: string }>("/healthz")
}

export function listProjects() {
  return request<ListResponse<Project>>("/v1/projects")
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

export function updateSandboxPorts(id: string, ports: SandboxPort[]) {
  return request<Sandbox>(`/v1/sandboxes/${id}`, {
    method: "PATCH",
    body: JSON.stringify({ ports }),
  })
}

export function deleteSandbox(id: string) {
  return request<void>(`/v1/sandboxes/${id}`, { method: "DELETE" })
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

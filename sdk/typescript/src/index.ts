export type JSONValue =
  | string
  | number
  | boolean
  | null
  | JSONValue[]
  | { [key: string]: JSONValue }

export type JSONObject = { [key: string]: JSONValue }

export type ListResponse<T> = {
  items?: T[]
}

export type RuntimeRef = {
  adapter?: string
  kind: string
  namespace: string
  name: string
}

export type RuntimeStorage = {
  name: string
  mountPath: string
  claimName?: string
  phase?: string
  capacity?: string
  storageClassName?: string
  message?: string
}

export type RuntimeTarget = {
  namespace: string
  podName: string
  container: string
  phase: string
  selector: string
  commands?: string[]
  storage?: RuntimeStorage[]
}

export type LogResult = {
  target: RuntimeTarget
  logs: string
}

export type RuntimeEvent = {
  type?: string
  reason?: string
  message?: string
  count?: number
  firstTimestamp?: string
  lastTimestamp?: string
}

export type TemplatePort = {
  name: string
  port: number
  protocol: string
}

export type SandboxPort = {
  name: string
  port: number
  protocol: string
  previewUrl?: string
}

export type PreviewPort = {
  name: string
  port: number
  protocol: string
  previewUrl?: string
  available: boolean
  message?: string
}

export type PreviewPortsResult = {
  target: RuntimeTarget
  items: PreviewPort[]
}

export type SecretRef = {
  name: string
  key?: string
}

export type Project = {
  id: string
  name: string
  slug: string
  repositoryUrl?: string
  defaultNamespace: string
  defaultTemplateId?: string
  metadata?: JSONObject
  createdAt?: string
  updatedAt?: string
}

export type ProjectCreate = {
  name: string
  slug?: string
  repositoryUrl?: string
  defaultNamespace: string
  metadata?: JSONObject
}

export type ProjectUpdate = Partial<Pick<ProjectCreate, "name" | "repositoryUrl" | "defaultNamespace" | "metadata">> & {
  defaultTemplateId?: string | null
}

export type EnvironmentTemplate = {
  id: string
  projectId?: string
  name: string
  slug: string
  image: string
  startupCommand?: string[]
  workingDir?: string
  cpuRequest?: string
  memoryRequest?: string
  storageRequest?: string
  exposedPorts?: TemplatePort[]
  env?: Record<string, string>
  secretRefs?: SecretRef[]
  networkPolicy?: string
  lifecyclePolicy?: JSONObject
  metadata?: JSONObject
  createdAt?: string
  updatedAt?: string
}

export type TemplateCreate = {
  projectId?: string
  name: string
  slug?: string
  image: string
  startupCommand?: string[]
  workingDir?: string
  cpuRequest?: string
  memoryRequest?: string
  storageRequest?: string
  exposedPorts?: TemplatePort[]
  env?: Record<string, string>
  secretRefs?: SecretRef[]
  networkPolicy?: string
  lifecyclePolicy?: JSONObject
  metadata?: JSONObject
}

export type TemplateUpdate = Partial<Omit<TemplateCreate, "projectId" | "slug">>

export type SandboxStatus = "pending" | "running" | "stopped" | "failed" | "deleted"

export type Sandbox = {
  id: string
  projectId: string
  templateId?: string
  name: string
  slug: string
  status: SandboxStatus
  namespace: string
  serviceAccountName: string
  runtimeRef?: RuntimeRef
  ports?: SandboxPort[]
  metadata?: JSONObject
  createdAt?: string
  updatedAt?: string
  deletedAt?: string
}

export type SandboxCreate = {
  projectId: string
  templateId?: string
  name: string
  slug?: string
  namespace?: string
  serviceAccountName?: string
  metadata?: JSONObject
}

export type SandboxUpdate = Partial<Pick<SandboxCreate, "name" | "namespace" | "serviceAccountName" | "metadata">> & {
  status?: SandboxStatus
  runtimeRef?: RuntimeRef | null
  ports?: SandboxPort[]
}

export type ExecutionTaskStatus =
  | "queued"
  | "running"
  | "succeeded"
  | "failed"
  | "canceled"
  | "timed_out"

export type ExecutionTask = {
  id: string
  projectId: string
  sandboxId: string
  status: ExecutionTaskStatus
  command: string[]
  timeoutSeconds: number
  exitCode?: number
  stdout: string
  stderr: string
  outputTruncated: boolean
  error?: string
  runtimeRef?: RuntimeRef
  metadata?: JSONObject
  startedAt?: string
  finishedAt?: string
  createdAt?: string
  updatedAt?: string
}

export type ExecutionTaskCreate = {
  command: string[]
  timeoutSeconds?: number
  metadata?: JSONObject
}

export type ArtifactKind =
  | "file"
  | "directory"
  | "log"
  | "report"
  | "screenshot"
  | "image"
  | "link"
  | "other"

export type Artifact = {
  id: string
  projectId: string
  sandboxId: string
  taskId?: string
  kind: ArtifactKind
  name: string
  uri: string
  contentType?: string
  sizeBytes?: number
  metadata?: JSONObject
  createdAt?: string
  updatedAt?: string
}

export type ArtifactCreate = {
  taskId?: string
  kind: ArtifactKind
  name: string
  uri: string
  contentType?: string
  sizeBytes?: number
  metadata?: JSONObject
}

export type MboxClientOptions = {
  baseUrl: string
  headers?: HeadersInit
  fetch?: typeof fetch
}

export type RequestOptions = {
  signal?: AbortSignal
  headers?: HeadersInit
}

export type WaitForTaskOptions = {
  intervalMs?: number
  timeoutMs?: number
  signal?: AbortSignal
}

export class MboxAPIError extends Error {
  readonly status: number
  readonly statusText: string
  readonly body: unknown

  constructor(message: string, response: Response, body: unknown) {
    super(message)
    this.name = "MboxAPIError"
    this.status = response.status
    this.statusText = response.statusText
    this.body = body
  }
}

const TERMINAL_TASK_STATUSES = new Set<ExecutionTaskStatus>([
  "succeeded",
  "failed",
  "canceled",
  "timed_out",
])

export class MboxClient {
  private readonly baseUrl: string
  private readonly headers?: HeadersInit
  private readonly fetchFn: typeof fetch

  constructor(options: MboxClientOptions) {
    if (!options.baseUrl) {
      throw new Error("baseUrl is required")
    }
    this.baseUrl = options.baseUrl.replace(/\/+$/, "")
    this.headers = options.headers
    this.fetchFn = options.fetch ?? globalThis.fetch
    if (!this.fetchFn) {
      throw new Error("fetch is required; use Node.js 20+ or pass options.fetch")
    }
  }

  health(options?: RequestOptions) {
    return this.request<{ status?: string }>("/healthz", options)
  }

  listProjects(options?: RequestOptions) {
    return this.request<ListResponse<Project>>("/v1/projects", options)
  }

  createProject(payload: ProjectCreate, options?: RequestOptions) {
    return this.request<Project>("/v1/projects", { ...options, method: "POST", body: payload })
  }

  getProject(projectId: string, options?: RequestOptions) {
    return this.request<Project>(`/v1/projects/${encodeURIComponent(projectId)}`, options)
  }

  updateProject(projectId: string, payload: ProjectUpdate, options?: RequestOptions) {
    return this.request<Project>(`/v1/projects/${encodeURIComponent(projectId)}`, {
      ...options,
      method: "PATCH",
      body: payload,
    })
  }

  deleteProject(projectId: string, options?: RequestOptions) {
    return this.request<void>(`/v1/projects/${encodeURIComponent(projectId)}`, {
      ...options,
      method: "DELETE",
    })
  }

  listTemplates(projectId?: string, options?: RequestOptions) {
    const query = projectId ? `?projectId=${encodeURIComponent(projectId)}` : ""
    return this.request<ListResponse<EnvironmentTemplate>>(`/v1/templates${query}`, options)
  }

  createTemplate(payload: TemplateCreate, options?: RequestOptions) {
    return this.request<EnvironmentTemplate>("/v1/templates", {
      ...options,
      method: "POST",
      body: payload,
    })
  }

  getTemplate(templateId: string, options?: RequestOptions) {
    return this.request<EnvironmentTemplate>(`/v1/templates/${encodeURIComponent(templateId)}`, options)
  }

  updateTemplate(templateId: string, payload: TemplateUpdate, options?: RequestOptions) {
    return this.request<EnvironmentTemplate>(`/v1/templates/${encodeURIComponent(templateId)}`, {
      ...options,
      method: "PATCH",
      body: payload,
    })
  }

  deleteTemplate(templateId: string, options?: RequestOptions) {
    return this.request<void>(`/v1/templates/${encodeURIComponent(templateId)}`, {
      ...options,
      method: "DELETE",
    })
  }

  listSandboxes(projectId?: string, options?: RequestOptions) {
    const query = projectId ? `?projectId=${encodeURIComponent(projectId)}` : ""
    return this.request<ListResponse<Sandbox>>(`/v1/sandboxes${query}`, options)
  }

  createSandbox(payload: SandboxCreate, options?: RequestOptions) {
    return this.request<Sandbox>("/v1/sandboxes", { ...options, method: "POST", body: payload })
  }

  getSandbox(sandboxId: string, options?: RequestOptions) {
    return this.request<Sandbox>(`/v1/sandboxes/${encodeURIComponent(sandboxId)}`, options)
  }

  updateSandbox(sandboxId: string, payload: SandboxUpdate, options?: RequestOptions) {
    return this.request<Sandbox>(`/v1/sandboxes/${encodeURIComponent(sandboxId)}`, {
      ...options,
      method: "PATCH",
      body: payload,
    })
  }

  deleteSandbox(sandboxId: string, options?: RequestOptions) {
    return this.request<void>(`/v1/sandboxes/${encodeURIComponent(sandboxId)}`, {
      ...options,
      method: "DELETE",
    })
  }

  startSandbox(sandboxId: string, options?: RequestOptions) {
    return this.request<Sandbox>(`/v1/sandboxes/${encodeURIComponent(sandboxId)}/start`, {
      ...options,
      method: "POST",
    })
  }

  stopSandbox(sandboxId: string, options?: RequestOptions) {
    return this.request<Sandbox>(`/v1/sandboxes/${encodeURIComponent(sandboxId)}/stop`, {
      ...options,
      method: "POST",
    })
  }

  getRuntimeTarget(sandboxId: string, options?: RequestOptions) {
    return this.request<RuntimeTarget>(`/v1/sandboxes/${encodeURIComponent(sandboxId)}/runtime`, options)
  }

  getRuntimeLogs(sandboxId: string, tailLines?: number, options?: RequestOptions) {
    const query = tailLines ? `?tailLines=${encodeURIComponent(String(tailLines))}` : ""
    return this.request<LogResult>(`/v1/sandboxes/${encodeURIComponent(sandboxId)}/logs${query}`, options)
  }

  getRuntimeEvents(sandboxId: string, options?: RequestOptions) {
    return this.request<ListResponse<RuntimeEvent>>(
      `/v1/sandboxes/${encodeURIComponent(sandboxId)}/events`,
      options,
    )
  }

  getPreviewPorts(sandboxId: string, options?: RequestOptions) {
    return this.request<PreviewPortsResult>(`/v1/sandboxes/${encodeURIComponent(sandboxId)}/ports`, options)
  }

  listExecutionTasks(sandboxId: string, options?: RequestOptions) {
    return this.request<ListResponse<ExecutionTask>>(
      `/v1/sandboxes/${encodeURIComponent(sandboxId)}/tasks`,
      options,
    )
  }

  createExecutionTask(sandboxId: string, payload: ExecutionTaskCreate, options?: RequestOptions) {
    return this.request<ExecutionTask>(`/v1/sandboxes/${encodeURIComponent(sandboxId)}/tasks`, {
      ...options,
      method: "POST",
      body: payload,
    })
  }

  getExecutionTask(taskId: string, options?: RequestOptions) {
    return this.request<ExecutionTask>(`/v1/tasks/${encodeURIComponent(taskId)}`, options)
  }

  cancelExecutionTask(taskId: string, options?: RequestOptions) {
    return this.request<ExecutionTask>(`/v1/tasks/${encodeURIComponent(taskId)}/cancel`, {
      ...options,
      method: "POST",
    })
  }

  async waitForTask(taskId: string, options: WaitForTaskOptions = {}) {
    const intervalMs = options.intervalMs ?? 1500
    const started = Date.now()
    for (;;) {
      const task = await this.getExecutionTask(taskId, { signal: options.signal })
      if (TERMINAL_TASK_STATUSES.has(task.status)) {
        return task
      }
      if (options.timeoutMs && Date.now() - started >= options.timeoutMs) {
        throw new Error(`timed out waiting for task ${taskId}`)
      }
      await sleep(intervalMs, options.signal)
    }
  }

  listArtifacts(sandboxId: string, options?: RequestOptions) {
    return this.request<ListResponse<Artifact>>(
      `/v1/sandboxes/${encodeURIComponent(sandboxId)}/artifacts`,
      options,
    )
  }

  createArtifact(sandboxId: string, payload: ArtifactCreate, options?: RequestOptions) {
    return this.request<Artifact>(`/v1/sandboxes/${encodeURIComponent(sandboxId)}/artifacts`, {
      ...options,
      method: "POST",
      body: payload,
    })
  }

  listTaskArtifacts(taskId: string, options?: RequestOptions) {
    return this.request<ListResponse<Artifact>>(`/v1/tasks/${encodeURIComponent(taskId)}/artifacts`, options)
  }

  getArtifact(artifactId: string, options?: RequestOptions) {
    return this.request<Artifact>(`/v1/artifacts/${encodeURIComponent(artifactId)}`, options)
  }

  async request<T>(
    path: string,
    options: RequestOptions & { method?: string; body?: unknown } = {},
  ): Promise<T> {
    const headers = new Headers(this.headers)
    mergeHeaders(headers, options.headers)
    const hasBody = options.body !== undefined
    if (hasBody && !headers.has("content-type")) {
      headers.set("content-type", "application/json")
    }

    const response = await this.fetchFn(`${this.baseUrl}${path}`, {
      method: options.method ?? "GET",
      headers,
      signal: options.signal,
      body: hasBody ? JSON.stringify(options.body) : undefined,
    })

    if (!response.ok) {
      const body = await readResponseBody(response)
      const message =
        typeof body === "object" && body && "error" in body && typeof body.error === "string"
          ? body.error
          : `${response.status} ${response.statusText}`
      throw new MboxAPIError(message, response, body)
    }

    if (response.status === 204) {
      return undefined as T
    }
    return response.json() as Promise<T>
  }
}

function mergeHeaders(target: Headers, headers?: HeadersInit) {
  if (!headers) {
    return
  }
  new Headers(headers).forEach((value, key) => target.set(key, value))
}

async function readResponseBody(response: Response) {
  const contentType = response.headers.get("content-type") || ""
  if (contentType.includes("application/json")) {
    try {
      return await response.json()
    } catch {
      return undefined
    }
  }
  try {
    return await response.text()
  } catch {
    return undefined
  }
}

function sleep(ms: number, signal?: AbortSignal) {
  if (signal?.aborted) {
    return Promise.reject(signal.reason)
  }
  return new Promise<void>((resolve, reject) => {
    const timeout = setTimeout(resolve, ms)
    signal?.addEventListener(
      "abort",
      () => {
        clearTimeout(timeout)
        reject(signal.reason)
      },
      { once: true },
    )
  })
}

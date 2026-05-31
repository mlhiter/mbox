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

export type APIInfo = {
  name: string
  apiVersion: string
  serverVersion: string
  runtimeController: RuntimeCapability
  runtimeAccess: RuntimeCapability
  artifactContent: ArtifactContentCapability
  capabilities: string[]
  compatibility: {
    minimumCliApiVersion: string
    minimumSdkApiVersion: string
  }
  authenticationRequired: boolean
}

export type ClientKind = "cli" | "sdk"

export type CompatibilityCheckResult = {
  ok: boolean
  client: ClientKind
  clientApiVersion: string
  serverApiVersion: string
  minimumApiVersion: string
  requiredCapabilities: string[]
  missingCapabilities: string[]
  message: string
}

export type OpenAPIDocument = {
  openapi: string
  info: {
    title: string
    version: string
    description?: string
  }
  servers?: Array<{
    url: string
    description?: string
  }>
  tags?: Array<{
    name: string
    description?: string
  }>
  paths: Record<string, unknown>
  components?: {
    schemas?: Record<string, unknown>
    [key: string]: unknown
  }
  [key: string]: unknown
}

export type RuntimeCapability = {
  enabled: boolean
  adapter?: string
}

export type ArtifactContentCapability = {
  retainedContentEnabled: boolean
  storageProvider: "postgres" | "filesystem" | string
  maxBytes: number
}

export type RuntimeRef = {
  adapter?: string
  kind: string
  namespace: string
  name: string
}

export type ManagedResource = {
  adapter: string
  kind: string
  namespace?: string
  name: string
  owner?: ManagedResourceOwner
  labels?: Record<string, string>
  createdAt?: string
}

export type ManagedResourceCount = {
  name: string
  count: number
}

export type ManagedResourceOwner = {
  kind: "sandbox" | "template"
  projectId?: string
  sandboxId?: string
  templateId?: string
}

export type ManagedResourceSummary = {
  total: number
  byKind: ManagedResourceCount[]
  byNamespace: ManagedResourceCount[]
  byOwner: ManagedResourceCount[]
}

export type ManagedResourceRef = {
  adapter: string
  kind: string
  namespace: string
  name: string
}

export type RuntimeResourceListOptions = RequestOptions & {
  namespace?: string
  kind?: string
}

export type RuntimeOrphanReason =
  | "missing-sandbox-record"
  | "cleanup-pending"
  | "runtime-ref-mismatch"
  | "missing-template-record"
  | "unlabeled-owner"

export type RuntimeOrphan = {
  reason: RuntimeOrphanReason
  resource: ManagedResource
  sandboxId?: string
  templateId?: string
  projectId?: string
  runtimeRef?: RuntimeRef
  status?: SandboxStatus
  deletedAt?: string
  message: string
  evidence?: string[]
}

export type RuntimeOrphanAudit = {
  adapter: string
  checkedAt: string
  namespace?: string
  resourceCount: number
  orphanCount: number
  expectedClean: boolean
  items: RuntimeOrphan[]
}

export type RuntimeOrphanCleanupRequest = {
  resource: ManagedResourceRef
  reason: RuntimeOrphanReason
  deleteOrphan: true
  confirm: "delete-orphan-runtime-resource"
}

export type RuntimeOrphanCleanupResult = {
  deleted: boolean
  resource: ManagedResourceRef
  reason: RuntimeOrphanReason
  message: string
}

export type RuntimeResourceList = {
  adapter: string
  checkedAt: string
  summary: ManagedResourceSummary
  items: ManagedResource[]
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

export type ProjectPolicyEnforcement = "disabled" | "enforced"

export type ProjectPolicy = {
  projectId: string
  enforcement: ProjectPolicyEnforcement
  allowedImagePrefixes?: string[]
  allowedServiceAccounts?: string[]
  allowedSecretRefs?: string[]
  createdAt?: string
  updatedAt?: string
}

export type ProjectPolicyUpsert = {
  enforcement: ProjectPolicyEnforcement
  allowedImagePrefixes?: string[]
  allowedServiceAccounts?: string[]
  allowedSecretRefs?: string[]
}

export type ProjectQuotaPolicyEnforcement = "disabled" | "enforced"

export type ProjectQuotaPolicy = {
  projectId: string
  enforcement: ProjectQuotaPolicyEnforcement
  maxActiveSandboxes?: number
  maxRetainedArtifactBytes?: number
  createdAt?: string
  updatedAt?: string
}

export type ProjectQuotaPolicyUpsert = {
  enforcement: ProjectQuotaPolicyEnforcement
  maxActiveSandboxes?: number
  maxRetainedArtifactBytes?: number
}

export type ProjectCredentialType = "git" | "registry" | "kubernetes" | "ssh" | "generic"

export type ProjectCredential = {
  id: string
  projectId: string
  name: string
  slug: string
  type: ProjectCredentialType
  target?: string
  secretRef: SecretRef
  usage?: string[]
  metadata?: JSONObject
  createdAt?: string
  updatedAt?: string
}

export type ProjectCredentialCreate = {
  name: string
  slug?: string
  type: ProjectCredentialType
  target?: string
  secretRef: SecretRef
  usage?: string[]
  metadata?: JSONObject
}

export type ResourceUsageValue = {
  value: string
  count: number
}

export type ResourceQuantityUsage = {
  total?: string
  declared: number
  missing: number
  invalid: number
}

export type SandboxResourceRequestUsage = {
  count: number
  cpu: ResourceQuantityUsage
  memory: ResourceQuantityUsage
  storage: ResourceQuantityUsage
}

export type ProjectUsage = {
  projectId: string
  generatedAt: string
  sandboxes: {
    total: number
    active: number
    pending: number
    running: number
    stopped: number
    failed: number
    deleted: number
    cleanupPending: number
    activeRequests: SandboxResourceRequestUsage
    runningRequests: SandboxResourceRequestUsage
  }
  runtimeSessions: {
    total: number
    active: number
    ended: number
    failed: number
    terminal: number
    ide: number
    notebook: number
    browser: number
    command: number
    custom: number
  }
  executionTasks: {
    total: number
    queued: number
    running: number
    succeeded: number
    failed: number
    canceled: number
    timedOut: number
  }
  artifacts: {
    total: number
    retainedContent: number
    referencedBytes: number
    retainedBytes: number
    file: number
    directory: number
    log: number
    report: number
    screenshot: number
    image: number
    link: number
    other: number
  }
  templates: {
    projectScoped: number
    globalVisible: number
    cpuRequests?: ResourceUsageValue[]
    memoryRequests?: ResourceUsageValue[]
    storageRequests?: ResourceUsageValue[]
  }
  credentials: {
    total: number
    git: number
    registry: number
    kubernetes: number
    ssh: number
    generic: number
  }
  notes?: string[]
}

export type KnownAuditEventAction =
  | "project.created"
  | "project.updated"
  | "project.deleted"
  | "project.policy.updated"
  | "project.quota_policy.updated"
  | "project.credential.created"
  | "project.credential.deleted"
  | "template.created"
  | "template.updated"
  | "template.deleted"
  | "template.validation.started"
  | "template.validation.decided"
  | "sandbox.created"
  | "sandbox.updated"
  | "sandbox.deleted"
  | "sandbox.stopped"
  | "sandbox.started"
  | "runtime.session.created"
  | "runtime.session.ended"
  | "execution.task.created"
  | "execution.task.cancel.requested"
  | "artifact.created"
  | "artifact.content.captured"
  | "artifact.content.uploaded"
  | "runtime.orphan.deleted"
  | "policy.denied"

export type PolicyDeniedOperation =
  | "sandbox.launch"
  | "template.validation"
  | "artifact.content.capture"
  | "artifact.content.upload"

export type PolicyDeniedAuditMetadata = JSONObject & {
  operation: PolicyDeniedOperation
  reason: string
  requestId?: string
  templateId?: string
  templateName?: string
  image?: string
  serviceAccountName?: string
  sandboxId?: string
  artifactKind?: string
  incomingBytes?: number
}

export type AuditEvent = {
  id: string
  projectId?: string
  action: KnownAuditEventAction | (string & {})
  resourceType: string
  resourceId?: string
  resourceName?: string
  actor?: string
  source?: string
  metadata?: JSONObject | PolicyDeniedAuditMetadata
  createdAt: string
}

export type AuditEventListOptions = RequestOptions & {
  projectId?: string
  action?: KnownAuditEventAction | (string & {})
  resourceType?: string
  resourceId?: string
  actor?: string
  source?: string
  requestId?: string
  operation?: PolicyDeniedOperation | (string & {})
  since?: string
  until?: string
  limit?: number
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

export type TemplateValidationRunCreate = {
  projectId?: string
  name?: string
  metadata?: JSONObject
}

export type TemplateValidationRunDecision = {
  status: "passed" | "failed"
}

export type TemplateValidationRun = {
  template: EnvironmentTemplate
  sandbox: Sandbox
}

export type BoundaryCheck = {
  id: string
  label: string
  status: "pass" | "warn" | "fail"
  message: string
  evidence?: string[]
}

export type BoundaryPort = {
  name: string
  port: number
  protocol: string
}

export type BoundarySummary = {
  kind: "template" | "sandbox"
  projectId?: string
  projectName?: string
  templateId: string
  templateName: string
  sandboxId?: string
  sandboxName?: string
  sandboxStatus?: string
  namespace?: string
  serviceAccountName?: string
  serviceAccountTokenAutomount: boolean
  runtimeRef?: RuntimeRef
  image: string
  workingDir: string
  resourceRequests?: Record<string, string>
  storageRequest?: string
  previewPorts?: BoundaryPort[]
  envVarCount: number
  secretRefs?: SecretRef[]
  secretProjection: string
  networkPolicy: string
  networkPolicyProjection: string
  lifecyclePolicy?: JSONObject
  lifecyclePolicyProjection: string
  policyEnforcement: ProjectPolicyEnforcement
  allowedImagePrefixes?: string[]
  allowedServiceAccounts?: string[]
  allowedSecretRefs?: string[]
  credentialRefs?: Array<{
    id: string
    name: string
    slug: string
    type: ProjectCredentialType
    target?: string
    secretRef: string
    usage?: string[]
  }>
  credentialProjection: string
  controllerPermissions: string[]
  runtimeAccess: string[]
  cleanup: string[]
  checks: BoundaryCheck[]
}

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

export type RuntimeSessionType =
  | "terminal"
  | "ide"
  | "notebook"
  | "browser"
  | "command"
  | "custom"

export type RuntimeSessionStatus = "active" | "ended" | "failed"

export type RuntimeSession = {
  id: string
  projectId: string
  sandboxId: string
  type: RuntimeSessionType
  status: RuntimeSessionStatus
  client?: string
  userAgent?: string
  runtimeRef?: RuntimeRef
  metadata?: JSONObject
  startedAt: string
  endedAt?: string
  createdAt?: string
  updatedAt?: string
}

export type RuntimeSessionCreate = {
  type: RuntimeSessionType
  client?: string
  metadata?: JSONObject
}

export type ExecutionTaskEvent =
  | {
      type: "snapshot" | "status" | "done"
      task: ExecutionTask
      createdAt: string
    }
  | {
      type: "output"
      stream: "stdout" | "stderr"
      data: string
      offset?: number
      createdAt: string
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
  retainedContent?: ArtifactContent
  createdAt?: string
  updatedAt?: string
}

export type ArtifactContent = {
  artifactId: string
  contentType?: string
  sizeBytes: number
  sha256: string
  sourceUri: string
  storageProvider: "postgres" | "filesystem" | "s3"
  storageKey?: string
  capturedAt: string
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
  apiVersion?: string
  token?: string
  headers?: HeadersInit
  requestId?: string
  auditActor?: string
  auditSource?: string
  fetch?: typeof fetch
}

export type MboxEnvironment = Record<string, string | undefined>

export type MboxClientFromEnvOptions = Omit<MboxClientOptions, "baseUrl"> & {
  baseUrl?: string
}

export type RequestOptions = {
  signal?: AbortSignal
  headers?: HeadersInit
}

export type CompatibilityCheckOptions = RequestOptions & {
  requiredCapabilities?: readonly string[]
}

export type RawBody = BodyInit

export type WaitForTaskOptions = {
  intervalMs?: number
  timeoutMs?: number
  signal?: AbortSignal
}

export type WatchExecutionTaskOptions = RequestOptions & {
  onEvent?: (event: ExecutionTaskEvent) => void | Promise<void>
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

export class MboxCompatibilityError extends Error {
  readonly result: CompatibilityCheckResult

  constructor(result: CompatibilityCheckResult) {
    super(result.message)
    this.name = "MboxCompatibilityError"
    this.result = result
  }
}

const TERMINAL_TASK_STATUSES = new Set<ExecutionTaskStatus>([
  "succeeded",
  "failed",
  "canceled",
  "timed_out",
])

export function createMboxClientFromEnv(
  env: MboxEnvironment = processEnv(),
  options: MboxClientFromEnvOptions = {},
) {
  return new MboxClient({
    baseUrl: options.baseUrl ?? env.MBOX_API_URL ?? "http://127.0.0.1:18080",
    apiVersion: options.apiVersion,
    token: options.token ?? env.MBOX_TOKEN ?? env.MBOX_API_TOKEN,
    headers: options.headers,
    requestId: options.requestId ?? env.MBOX_REQUEST_ID,
    auditActor: options.auditActor ?? env.MBOX_AUDIT_ACTOR,
    auditSource: options.auditSource ?? env.MBOX_AUDIT_SOURCE,
    fetch: options.fetch,
  })
}

export class MboxClient {
  private readonly baseUrl: string
  private readonly apiVersion: string
  private readonly headers?: HeadersInit
  private readonly fetchFn: typeof fetch

  constructor(options: MboxClientOptions) {
    if (!options.baseUrl) {
      throw new Error("baseUrl is required")
    }
    this.baseUrl = options.baseUrl.replace(/\/+$/, "")
    this.apiVersion = options.apiVersion ?? "v1alpha1"
    this.headers = clientHeaders(options)
    this.fetchFn = options.fetch ?? globalThis.fetch
    if (!this.fetchFn) {
      throw new Error("fetch is required; use Node.js 20+ or pass options.fetch")
    }
  }

  health(options?: RequestOptions) {
    return this.request<{ status?: string }>("/healthz", options)
  }

  info(options?: RequestOptions) {
    return this.request<APIInfo>("/v1/info", options)
  }

  async checkCompatibility(options: CompatibilityCheckOptions = {}) {
    const { requiredCapabilities, ...requestOptions } = options
    return checkSDKCompatibility(await this.info(requestOptions), this.apiVersion, requiredCapabilities)
  }

  async assertCompatibility(options?: CompatibilityCheckOptions) {
    const result = await this.checkCompatibility(options)
    if (!result.ok) {
      throw new MboxCompatibilityError(result)
    }
    return result
  }

  openAPI(options?: RequestOptions) {
    return this.request<OpenAPIDocument>("/v1/openapi.json", options)
  }

  listRuntimeResources(namespaceOrOptions?: string | RuntimeResourceListOptions, options?: RequestOptions) {
    const filters = runtimeResourceListFilters(namespaceOrOptions)
    const requestOptions = typeof namespaceOrOptions === "string" ? options : namespaceOrOptions
    const query = queryString(filters)
    return this.request<RuntimeResourceList>(`/v1/runtime/resources${query}`, requestOptions)
  }

  listRuntimeOrphans(namespaceOrOptions?: string | RuntimeResourceListOptions, options?: RequestOptions) {
    const filters = runtimeResourceListFilters(namespaceOrOptions)
    const requestOptions = typeof namespaceOrOptions === "string" ? options : namespaceOrOptions
    const query = queryString(filters)
    return this.request<RuntimeOrphanAudit>(`/v1/runtime/orphans${query}`, requestOptions)
  }

  cleanupRuntimeOrphan(payload: RuntimeOrphanCleanupRequest, options?: RequestOptions) {
    return this.request<RuntimeOrphanCleanupResult>("/v1/runtime/orphans/cleanup", {
      ...options,
      method: "POST",
      body: payload,
    })
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

  getProjectUsage(projectId: string, options?: RequestOptions) {
    return this.request<ProjectUsage>(`/v1/projects/${encodeURIComponent(projectId)}/usage`, options)
  }

  listAuditEvents(options: AuditEventListOptions = {}) {
    const { projectId, action, resourceType, resourceId, actor, source, requestId, operation, since, until, limit, ...requestOptions } = options
    const query = auditEventQuery({ projectId, action, resourceType, resourceId, actor, source, requestId, operation, since, until, limit })
    return this.request<ListResponse<AuditEvent>>(`/v1/audit-events${query}`, requestOptions)
  }

  listProjectAuditEvents(projectId: string, options: Omit<AuditEventListOptions, "projectId"> = {}) {
    const { action, resourceType, resourceId, actor, source, requestId, operation, since, until, limit, ...requestOptions } = options
    const query = auditEventQuery({ action, resourceType, resourceId, actor, source, requestId, operation, since, until, limit })
    return this.request<ListResponse<AuditEvent>>(
      `/v1/projects/${encodeURIComponent(projectId)}/audit-events${query}`,
      requestOptions,
    )
  }

  getProjectPolicy(projectId: string, options?: RequestOptions) {
    return this.request<ProjectPolicy>(`/v1/projects/${encodeURIComponent(projectId)}/policy`, options)
  }

  setProjectPolicy(projectId: string, payload: ProjectPolicyUpsert, options?: RequestOptions) {
    return this.request<ProjectPolicy>(`/v1/projects/${encodeURIComponent(projectId)}/policy`, {
      ...options,
      method: "PUT",
      body: payload,
    })
  }

  getProjectQuotaPolicy(projectId: string, options?: RequestOptions) {
    return this.request<ProjectQuotaPolicy>(`/v1/projects/${encodeURIComponent(projectId)}/quota-policy`, options)
  }

  setProjectQuotaPolicy(projectId: string, payload: ProjectQuotaPolicyUpsert, options?: RequestOptions) {
    return this.request<ProjectQuotaPolicy>(`/v1/projects/${encodeURIComponent(projectId)}/quota-policy`, {
      ...options,
      method: "PUT",
      body: payload,
    })
  }

  listProjectCredentials(projectId: string, options?: RequestOptions) {
    return this.request<ListResponse<ProjectCredential>>(
      `/v1/projects/${encodeURIComponent(projectId)}/credentials`,
      options,
    )
  }

  createProjectCredential(projectId: string, payload: ProjectCredentialCreate, options?: RequestOptions) {
    return this.request<ProjectCredential>(`/v1/projects/${encodeURIComponent(projectId)}/credentials`, {
      ...options,
      method: "POST",
      body: payload,
    })
  }

  getProjectCredential(credentialId: string, options?: RequestOptions) {
    return this.request<ProjectCredential>(`/v1/credentials/${encodeURIComponent(credentialId)}`, options)
  }

  deleteProjectCredential(credentialId: string, options?: RequestOptions) {
    return this.request<void>(`/v1/credentials/${encodeURIComponent(credentialId)}`, {
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

  getTemplateBoundary(templateId: string, projectId?: string, options?: RequestOptions) {
    const query = projectId ? `?projectId=${encodeURIComponent(projectId)}` : ""
    return this.request<BoundarySummary>(
      `/v1/templates/${encodeURIComponent(templateId)}/boundary${query}`,
      options,
    )
  }

  createTemplateValidationRun(
    templateId: string,
    payload: TemplateValidationRunCreate = {},
    options?: RequestOptions,
  ) {
    return this.request<TemplateValidationRun>(
      `/v1/templates/${encodeURIComponent(templateId)}/validation-runs`,
      {
        ...options,
        method: "POST",
        body: payload,
      },
    )
  }

  decideTemplateValidationRun(
    templateId: string,
    sandboxId: string,
    payload: TemplateValidationRunDecision,
    options?: RequestOptions,
  ) {
    return this.request<TemplateValidationRun>(
      `/v1/templates/${encodeURIComponent(templateId)}/validation-runs/${encodeURIComponent(sandboxId)}/decision`,
      {
        ...options,
        method: "POST",
        body: payload,
      },
    )
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

  getSandboxBoundary(sandboxId: string, options?: RequestOptions) {
    return this.request<BoundarySummary>(`/v1/sandboxes/${encodeURIComponent(sandboxId)}/boundary`, options)
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

  listRuntimeSessions(sandboxId: string, options?: RequestOptions) {
    return this.request<ListResponse<RuntimeSession>>(
      `/v1/sandboxes/${encodeURIComponent(sandboxId)}/sessions`,
      options,
    )
  }

  createRuntimeSession(sandboxId: string, payload: RuntimeSessionCreate, options?: RequestOptions) {
    return this.request<RuntimeSession>(`/v1/sandboxes/${encodeURIComponent(sandboxId)}/sessions`, {
      ...options,
      method: "POST",
      body: payload,
    })
  }

  getRuntimeSession(sessionId: string, options?: RequestOptions) {
    return this.request<RuntimeSession>(`/v1/sessions/${encodeURIComponent(sessionId)}`, options)
  }

  endRuntimeSession(sessionId: string, options?: RequestOptions) {
    return this.request<RuntimeSession>(`/v1/sessions/${encodeURIComponent(sessionId)}/end`, {
      ...options,
      method: "POST",
    })
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

  async watchExecutionTask(taskId: string, options: WatchExecutionTaskOptions = {}) {
    const response = await this.rawRequest(`/v1/tasks/${encodeURIComponent(taskId)}/events`, options)
    const events: ExecutionTaskEvent[] = []
    for await (const event of readNDJSON<ExecutionTaskEvent>(response, options.signal)) {
      events.push(event)
      await options.onEvent?.(event)
    }
    return events
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

  captureArtifactContent(artifactId: string, options?: RequestOptions) {
    return this.request<Artifact>(`/v1/artifacts/${encodeURIComponent(artifactId)}/capture`, {
      ...options,
      method: "POST",
    })
  }

  uploadArtifactContent(artifactId: string, body: RawBody, options?: RequestOptions) {
    return this.rawBodyRequest<Artifact>(`/v1/artifacts/${encodeURIComponent(artifactId)}/content`, body, {
      ...options,
      method: "PUT",
    })
  }

  async getArtifactContent(artifactId: string, options?: RequestOptions) {
    return this.rawRequest(`/v1/artifacts/${encodeURIComponent(artifactId)}/content`, options)
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

    const response = await this.rawFetch(path, {
      method: options.method,
      headers,
      signal: options.signal,
      body: hasBody ? options.body : undefined,
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

  async rawRequest(path: string, options: RequestOptions & { method?: string; body?: unknown } = {}) {
    const headers = new Headers(this.headers)
    mergeHeaders(headers, options.headers)
    const hasBody = options.body !== undefined
    if (hasBody && !headers.has("content-type")) {
      headers.set("content-type", "application/json")
    }
    return this.rawFetch(path, {
      method: options.method,
      headers,
      signal: options.signal,
      body: hasBody ? options.body : undefined,
    })
  }

  async rawBodyRequest<T>(
    path: string,
    body: RawBody,
    options: RequestOptions & { method?: string } = {},
  ): Promise<T> {
    const headers = new Headers(this.headers)
    mergeHeaders(headers, options.headers)

    const response = await this.rawFetch(path, {
      method: options.method,
      headers,
      signal: options.signal,
      rawBody: body,
    })

    if (response.status === 204) {
      return undefined as T
    }
    return response.json() as Promise<T>
  }

  private async rawFetch(
    path: string,
    options: RequestOptions & { method?: string; body?: unknown; rawBody?: RawBody } = {},
  ) {
    const response = await this.fetchFn(`${this.baseUrl}${path}`, {
      method: options.method ?? "GET",
      headers: options.headers,
      signal: options.signal,
      body: options.rawBody ?? (options.body === undefined ? undefined : JSON.stringify(options.body)),
    })

    if (!response.ok) {
      const body = await readResponseBody(response)
      const message =
        typeof body === "object" && body && "error" in body && typeof body.error === "string"
          ? body.error
          : `${response.status} ${response.statusText}`
      throw new MboxAPIError(message, response, body)
    }
    return response
  }
}

function mergeHeaders(target: Headers, headers?: HeadersInit) {
  if (!headers) {
    return
  }
  new Headers(headers).forEach((value, key) => target.set(key, value))
}

function clientHeaders(options: MboxClientOptions) {
  const headers = new Headers(options.headers)
  const token = options.token?.trim()
  if (token && !headers.has("authorization")) {
    headers.set("authorization", `Bearer ${token}`)
  }
  const requestId = options.requestId?.trim()
  if (requestId) {
    headers.set("x-mbox-request-id", requestId)
  }
  const auditActor = options.auditActor?.trim()
  if (auditActor) {
    headers.set("x-mbox-audit-actor", auditActor)
  }
  const auditSource = options.auditSource?.trim()
  if (auditSource) {
    headers.set("x-mbox-audit-source", auditSource)
  }
  return headers
}

function processEnv(): MboxEnvironment {
  const maybeProcess = globalThis as typeof globalThis & {
    process?: { env?: MboxEnvironment }
  }
  return maybeProcess.process?.env ?? {}
}

function auditEventQuery(options: {
  projectId?: string
  action?: string
  resourceType?: string
  resourceId?: string
  actor?: string
  source?: string
  requestId?: string
  operation?: string
  since?: string
  until?: string
  limit?: number
}) {
  const params = new URLSearchParams()
  if (options.projectId) {
    params.set("projectId", options.projectId)
  }
  if (options.action) {
    params.set("action", options.action)
  }
  if (options.resourceType) {
    params.set("resourceType", options.resourceType)
  }
  if (options.resourceId) {
    params.set("resourceId", options.resourceId)
  }
  if (options.actor) {
    params.set("actor", options.actor)
  }
  if (options.source) {
    params.set("source", options.source)
  }
  if (options.requestId) {
    params.set("requestId", options.requestId)
  }
  if (options.operation) {
    params.set("operation", options.operation)
  }
  if (options.since) {
    params.set("since", options.since)
  }
  if (options.until) {
    params.set("until", options.until)
  }
  if (options.limit && options.limit > 0) {
    params.set("limit", String(options.limit))
  }
  const query = params.toString()
  return query ? `?${query}` : ""
}

function runtimeResourceListFilters(namespaceOrOptions?: string | RuntimeResourceListOptions) {
  if (typeof namespaceOrOptions === "string") {
    return { namespace: namespaceOrOptions }
  }
  return {
    namespace: namespaceOrOptions?.namespace,
    kind: namespaceOrOptions?.kind,
  }
}

function queryString(filters: Record<string, string | undefined>) {
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(filters)) {
    if (value) {
      params.set(key, value)
    }
  }
  const query = params.toString()
  return query ? `?${query}` : ""
}

export function isPolicyDeniedAuditEvent(
  event: AuditEvent,
): event is AuditEvent & { action: "policy.denied"; metadata: PolicyDeniedAuditMetadata } {
  if (event.action !== "policy.denied" || !isJSONObject(event.metadata)) {
    return false
  }
  return isPolicyDeniedOperation(event.metadata.operation) && typeof event.metadata.reason === "string"
}

export function checkSDKCompatibility(
  info: APIInfo,
  clientApiVersion = "v1alpha1",
  requiredCapabilities: readonly string[] = [],
): CompatibilityCheckResult {
  return checkClientCompatibility(info, "sdk", clientApiVersion, requiredCapabilities)
}

export function checkClientCompatibility(
  info: APIInfo,
  client: ClientKind,
  clientApiVersion = "v1alpha1",
  requiredCapabilities: readonly string[] = [],
): CompatibilityCheckResult {
  const minimumApiVersion =
    client === "cli"
      ? info.compatibility?.minimumCliApiVersion
      : info.compatibility?.minimumSdkApiVersion
  const normalizedRequiredCapabilities = normalizeCapabilities(requiredCapabilities)
  const availableCapabilities = new Set(normalizeCapabilities(info.capabilities))
  const missingCapabilities = normalizedRequiredCapabilities.filter((capability) => !availableCapabilities.has(capability))
  const normalizedClient = normalizeAPIVersion(clientApiVersion)
  const normalizedMinimum = normalizeAPIVersion(minimumApiVersion)
  const normalizedServer = normalizeAPIVersion(info.apiVersion)
  const versionIncompatible =
    normalizedClient === undefined ||
    normalizedMinimum === undefined ||
    normalizedServer === undefined ||
    normalizedClient.family !== normalizedMinimum.family ||
    normalizedServer.family !== normalizedMinimum.family ||
    normalizedClient.number < normalizedMinimum.number
  if (versionIncompatible) {
    return {
      ok: false,
      client,
      clientApiVersion,
      serverApiVersion: info.apiVersion,
      minimumApiVersion: minimumApiVersion ?? "",
      requiredCapabilities: normalizedRequiredCapabilities,
      missingCapabilities,
      message: `mbox ${client} API ${clientApiVersion} is not compatible with server ${info.apiVersion}; server requires ${minimumApiVersion ?? "an unknown minimum API version"}`,
    }
  }
  if (missingCapabilities.length > 0) {
    return {
      ok: false,
      client,
      clientApiVersion,
      serverApiVersion: info.apiVersion,
      minimumApiVersion,
      requiredCapabilities: normalizedRequiredCapabilities,
      missingCapabilities,
      message: `mbox server ${info.apiVersion} is missing required capabilities: ${missingCapabilities.join(", ")}`,
    }
  }
  return {
    ok: true,
    client,
    clientApiVersion,
    serverApiVersion: info.apiVersion,
    minimumApiVersion,
    requiredCapabilities: normalizedRequiredCapabilities,
    missingCapabilities,
    message: `mbox ${client} API ${clientApiVersion} is compatible with server ${info.apiVersion}`,
  }
}

function normalizeCapabilities(capabilities: readonly string[] | undefined) {
  const out: string[] = []
  const seen = new Set<string>()
  for (const capability of capabilities ?? []) {
    const clean = capability.trim()
    if (clean && !seen.has(clean)) {
      seen.add(clean)
      out.push(clean)
    }
  }
  return out
}

function normalizeAPIVersion(version: string | undefined) {
  const match = version?.trim().match(/^v(\d+)(?:(alpha|beta)(\d+))?$/)
  if (!match) {
    return undefined
  }
  const major = Number(match[1])
  const stage = match[2] ?? "stable"
  const stageNumber = match[3] ? Number(match[3]) : 0
  const stageRank = stage === "alpha" ? 0 : stage === "beta" ? 1 : 2
  return {
    family: `v${major}`,
    number: major * 1_000_000 + stageRank * 1_000 + stageNumber,
  }
}

function isPolicyDeniedOperation(value: unknown): value is PolicyDeniedOperation {
  return (
    value === "sandbox.launch" ||
    value === "template.validation" ||
    value === "artifact.content.capture" ||
    value === "artifact.content.upload"
  )
}

function isJSONObject(value: unknown): value is JSONObject {
  return typeof value === "object" && value !== null && !Array.isArray(value)
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

async function* readNDJSON<T>(response: Response, signal?: AbortSignal): AsyncGenerator<T> {
  if (!response.body) {
    return
  }
  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ""
  try {
    for (;;) {
      if (signal?.aborted) {
        throw signal.reason
      }
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
          yield JSON.parse(line) as T
        }
        newline = buffer.indexOf("\n")
      }
    }
    buffer += decoder.decode()
    const line = buffer.trim()
    if (line) {
      yield JSON.parse(line) as T
    }
  } finally {
    reader.releaseLock()
  }
}

export * from "./contract.js"

import type { ReactNode } from "react"

export type APIState = "checking" | "ok" | "bad"

export type APIStatus = {
  state: APIState
  label: string
}

export type APIInfo = {
  name: string
  apiVersion: string
  serverVersion: string
  runtimeController: {
    enabled: boolean
    adapter?: string
  }
  runtimeAccess: {
    enabled: boolean
    adapter?: string
  }
  artifactContent: {
    retainedContentEnabled: boolean
    storageProvider: string
    maxBytes: number
  }
  capabilities: string[]
  compatibility: {
    minimumCliApiVersion: string
    minimumSdkApiVersion: string
  }
  authenticationRequired: boolean
}

export type ResourceKind = "project" | "template" | "sandbox"

export type WorkspaceView = "projects" | "templates" | "sandboxes" | "sandbox-detail" | "runtime"

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

export type RuntimeResourceList = {
  adapter: string
  checkedAt: string
  summary: ManagedResourceSummary
  items: ManagedResource[]
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
  status?: string
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

export type RuntimeTarget = {
  namespace: string
  podName: string
  container: string
  phase: string
  selector: string
  commands?: string[]
  storage?: RuntimeStorage[]
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

export type PreviewPort = {
  name: string
  port: number
  protocol: string
  previewUrl?: string
  available: boolean
  message?: string
}

export type SandboxPort = {
  name: string
  port: number
  protocol: string
  previewUrl?: string
}

export type PreviewPortsResult = {
  target: RuntimeTarget
  items: PreviewPort[]
}

export type Project = {
  id: string
  name: string
  slug: string
  repositoryUrl?: string
  defaultNamespace: string
  defaultTemplateId?: string
  createdAt?: string
  updatedAt?: string
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

export type ProjectQuotaPolicyEnforcement = "disabled" | "enforced"

export type ProjectQuotaPolicy = {
  projectId: string
  enforcement: ProjectQuotaPolicyEnforcement
  maxActiveSandboxes?: number
  maxRetainedArtifactBytes?: number
  createdAt?: string
  updatedAt?: string
}

export type ProjectCredentialType = "git" | "registry" | "kubernetes" | "ssh" | "generic"

export type ProjectCredential = {
  id: string
  projectId: string
  name: string
  slug: string
  type: ProjectCredentialType
  target?: string
  secretRef: { name: string; key?: string }
  usage?: string[]
  metadata?: Record<string, unknown>
  createdAt?: string
  updatedAt?: string
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
  generatedAt?: string
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

export type AuditEvent = {
  id: string
  projectId?: string
  action: string
  resourceType: string
  resourceId?: string
  resourceName?: string
  actor?: string
  source?: string
  metadata?: Record<string, unknown>
  createdAt?: string
}

export type Template = {
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
  exposedPorts?: Array<{ name: string; port: number; protocol: string }>
  env?: Record<string, string>
  secretRefs?: Array<{ name: string; key?: string }>
  networkPolicy?: string
  lifecyclePolicy?: Record<string, unknown>
  metadata?: {
    runtimeType?: string
    useCase?: string
    resourcePreset?: string
    validationStatus?: string
    [key: string]: unknown
  }
  createdAt?: string
  updatedAt?: string
}

export type Sandbox = {
  id: string
  projectId: string
  templateId?: string
  name: string
  slug: string
  namespace: string
  serviceAccountName: string
  status: string
  runtimeRef?: RuntimeRef
  ports?: SandboxPort[]
  metadata?: Record<string, unknown>
  createdAt?: string
  updatedAt?: string
}

export type TemplateValidationRun = {
  template: Template
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
  secretRefs?: Array<{ name: string; key?: string }>
  secretProjection: string
  networkPolicy: string
  networkPolicyProjection: string
  lifecyclePolicy?: Record<string, unknown>
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

export type Selection = {
  kind: ResourceKind
  id: string
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
  metadata?: Record<string, unknown>
  startedAt?: string
  finishedAt?: string
  createdAt?: string
  updatedAt?: string
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
  metadata?: Record<string, unknown>
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

export type RuntimeSessionType = "terminal" | "ide" | "notebook" | "browser" | "command" | "custom"

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
	metadata?: Record<string, unknown>
	startedAt: string
	endedAt?: string
	createdAt?: string
	updatedAt?: string
}

export type RuntimeTab = "terminal" | "sessions" | "boundary" | "preview" | "tasks" | "artifacts" | "logs" | "events"

export type ListResponse<T> = {
  items?: T[]
}

export type FormRecord = Record<string, FormDataEntryValue>

export type ConsolePanelProps = {
  id: string
  eyebrow: string
  title: string
  action: ReactNode
  wide?: boolean
  children: ReactNode
}

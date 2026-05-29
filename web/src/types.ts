import type { ReactNode } from "react"

export type APIState = "checking" | "ok" | "bad"

export type APIStatus = {
  state: APIState
  label: string
}

export type ResourceKind = "project" | "template" | "sandbox"

export type WorkspaceView = "projects" | "templates" | "sandboxes" | "sandbox-detail"

export type RuntimeRef = {
  adapter?: string
  kind: string
  namespace: string
  name: string
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
  createdAt?: string
  updatedAt?: string
}

export type RuntimeTab = "terminal" | "preview" | "tasks" | "artifacts" | "logs" | "events"

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

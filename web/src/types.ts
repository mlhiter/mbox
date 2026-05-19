import type { ReactNode } from "react"

export type APIState = "checking" | "ok" | "bad"

export type APIStatus = {
  state: APIState
  label: string
}

export type ResourceKind = "project" | "template" | "sandbox"

export type WorkspaceView = "projects" | "templates" | "sandboxes"

export type RuntimeRef = {
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
}

export type Selection = {
  kind: ResourceKind
  id: string
}

export type RuntimeTab = "terminal" | "storage" | "preview" | "logs" | "events"

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

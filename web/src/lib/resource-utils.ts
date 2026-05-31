import type {
  ArtifactKind,
  ExecutionTask,
  ExecutionTaskStatus,
  FormRecord,
  Project,
  ProjectPolicy,
  ProjectQuotaPolicy,
  ResourceKind,
  RuntimeRef,
  RuntimeStorage,
  Sandbox,
  Template,
} from "@/types"

export function collectionFor(
  kind: ResourceKind,
  projects: Project[],
  templates: Template[],
  sandboxes: Sandbox[],
) {
  if (kind === "project") {
    return projects
  }
  if (kind === "template") {
    return templates
  }
  return sandboxes
}

export function detailRows(
  kind: ResourceKind,
  item: Project | Template | Sandbox,
  projects: Project[],
  templates: Template[],
): Array<[string, string]> {
  const rows: Array<[string, string]> = [
    ["ID", item.id],
    ["Slug", item.slug],
  ]
  if (kind === "project") {
    const project = item as Project
    rows.push(["Namespace", project.defaultNamespace])
    rows.push(["Repository", project.repositoryUrl || ""])
    rows.push(["Default environment", templateName(project.defaultTemplateId, templates)])
  }
  if (kind === "template") {
    const template = item as Template
    rows.push(["Runtime", templateRuntimeType(template)])
    rows.push(["Use case", templateUseCase(template)])
    rows.push(["Preview ports", templateEntrypoints(template)])
    rows.push(["Size", templateResourcePreset(template)])
    rows.push(["Workspace", templatePersistence(template)])
    rows.push(["Validation", templateValidationText(template)])
    rows.push(["Image", template.image])
    rows.push(["Working directory", template.workingDir || ""])
    rows.push(["Project", template.projectId ? projectName(template.projectId, projects) : "Global"])
  }
  if (kind === "sandbox") {
    const sandbox = item as Sandbox
    rows.push(["Status", sandbox.status])
    rows.push(["Project", projectName(sandbox.projectId, projects)])
    rows.push(["Environment", templateName(sandbox.templateId, templates)])
    rows.push(["Namespace", sandbox.namespace])
    rows.push(["Runtime identity", sandbox.serviceAccountName])
    rows.push(["Runtime", runtimeText(sandbox.runtimeRef)])
  }
  return rows
}

export function projectName(id: string | undefined, projects: Project[]) {
  return projects.find((project) => project.id === id)?.name || shortID(id)
}

export function projectPolicyText(policy: ProjectPolicy | undefined) {
  if (policy?.enforcement === "enforced") {
    return "Policy enforced"
  }
  return "Policy disabled"
}

export function projectQuotaPolicyText(policy: ProjectQuotaPolicy | undefined) {
  if (policy?.enforcement === "enforced") {
    return "Quota enforced"
  }
  return "Quota disabled"
}

export function sandboxLaunchPreflight(props: {
  project?: Project
  template?: Template
  policy?: ProjectPolicy
  quotaPolicy?: ProjectQuotaPolicy
  usage?: { sandboxes?: { active?: number } }
  serviceAccountName?: string
}) {
  const blockers: string[] = []
  const warnings: string[] = []
  const serviceAccountName = props.serviceAccountName || "mbox-sandbox"

  if (!props.project) {
    blockers.push("Select a project before launching.")
  }
  if (!props.template) {
    blockers.push("Select an environment before launching.")
  }
  if (!props.project || !props.template) {
    return { blockers, warnings }
  }

  if (props.template.projectId && props.template.projectId !== props.project.id) {
    blockers.push("This environment belongs to a different project.")
  }

  if (props.policy?.enforcement === "enforced") {
    const imagePrefixes = props.policy.allowedImagePrefixes || []
    if (imagePrefixes.length > 0 && !imagePrefixes.some((prefix) => props.template?.image.startsWith(prefix))) {
      blockers.push(`Image ${props.template.image} is outside this project's allowed prefixes.`)
    }

    const allowedServiceAccounts = props.policy.allowedServiceAccounts || []
    if (allowedServiceAccounts.length > 0 && !allowedServiceAccounts.includes(serviceAccountName)) {
      blockers.push(`Runtime identity ${serviceAccountName} is not allowed for this project.`)
    }

    const secretRefs = props.template.secretRefs || []
    if (secretRefs.length > 0) {
      const allowedSecretRefs = new Set(props.policy.allowedSecretRefs || [])
      const deniedSecretRefs = secretRefs
        .map((secretRef) => secretRef.name)
        .filter((name) => !allowedSecretRefs.has(name))
      if (deniedSecretRefs.length > 0) {
        blockers.push(`Secret refs not allowed: ${deniedSecretRefs.join(", ")}.`)
      }
    }
  }

  if (props.quotaPolicy?.enforcement === "enforced" && props.quotaPolicy.maxActiveSandboxes !== undefined) {
    const active = props.usage?.sandboxes?.active ?? 0
    if (active >= props.quotaPolicy.maxActiveSandboxes) {
      blockers.push(`Active sandbox quota reached: ${active} / ${props.quotaPolicy.maxActiveSandboxes}.`)
    } else {
      warnings.push(`Active sandbox quota after launch: ${active + 1} / ${props.quotaPolicy.maxActiveSandboxes}.`)
    }
  }

  return { blockers, warnings }
}

export function templateName(id: string | undefined, templates: Template[]) {
  if (!id) {
    return "-"
  }
  return templates.find((template) => template.id === id)?.name || shortID(id)
}

export function templateForSandbox(sandbox: Sandbox, templates: Template[]) {
  if (!sandbox.templateId) {
    return undefined
  }
  return templates.find((template) => template.id === sandbox.templateId)
}

export function sandboxValidationRun(sandbox: Sandbox): {
  isValidationRun: boolean
  templateId: string | undefined
  result: "passed" | "failed" | undefined
  decidedAt: string | undefined
} {
  const metadata = sandbox.metadata || {}
  const result =
    metadata.validationResult === "passed" || metadata.validationResult === "failed"
      ? metadata.validationResult
      : undefined
  return {
    isValidationRun: metadata.purpose === "environment-validation",
    templateId: typeof metadata.templateId === "string" ? metadata.templateId : sandbox.templateId,
    result,
    decidedAt: typeof metadata.validationDecidedAt === "string" ? metadata.validationDecidedAt : undefined,
  }
}

export function templateValidationRun(template: Template, sandboxes: Sandbox[]) {
  const validationSandboxId =
    typeof template.metadata?.validationSandboxId === "string"
      ? template.metadata.validationSandboxId
      : undefined
  const sandbox = validationSandboxId
    ? sandboxes.find((item) => item.id === validationSandboxId)
    : sandboxes
        .filter((item) => {
          const run = sandboxValidationRun(item)
          return run.isValidationRun && run.templateId === template.id
        })
        .sort((left, right) => Date.parse(right.createdAt || "") - Date.parse(left.createdAt || ""))[0]
  const decidedAt =
    typeof template.metadata?.validationDecidedAt === "string"
      ? template.metadata.validationDecidedAt
      : sandboxValidationRun(sandbox || ({} as Sandbox)).decidedAt

  return {
    sandbox,
    decidedAt,
    status: templateValidationStatus(template),
  }
}

export function templateRuntimeType(template: Template) {
  const metadataType = template.metadata?.runtimeType
  if (typeof metadataType === "string" && metadataType) {
    return metadataType
  }
  const image = template.image.toLowerCase()
  if (image.includes("node") || image.includes("bun") || image.includes("deno")) {
    return "Node.js"
  }
  if (image.includes("python") || image.includes("jupyter")) {
    return "Python"
  }
  if (image.includes("golang") || image.includes("go:")) {
    return "Go"
  }
  if (image.includes("browser") || image.includes("playwright") || image.includes("chrome")) {
    return "Browser"
  }
  return "Custom"
}

export function templateUseCase(template: Template) {
  const metadataUseCase = template.metadata?.useCase
  if (typeof metadataUseCase === "string" && metadataUseCase) {
    return metadataUseCase
  }
  const ports = template.exposedPorts || []
  if (ports.some((port) => port.port === 8888 || port.name.toLowerCase().includes("notebook"))) {
    return "Notebook workspace"
  }
  if (ports.some((port) => port.port === 3000 || port.name.toLowerCase().includes("web"))) {
    return "Web app preview"
  }
  if (ports.some((port) => port.port === 8080 || port.name.toLowerCase().includes("api"))) {
    return "API service"
  }
  if (templateRuntimeType(template) === "Browser") {
    return "Agent browser runtime"
  }
  return "General sandbox"
}

export function templateEntrypoints(template: Template) {
  const ports = template.exposedPorts || []
  const entries = ["Terminal", ...ports.map((port) => `${port.name || "port"} :${port.port}`)]
  return entries.join(", ")
}

export function templateResourcePreset(template: Template) {
  const metadataPreset = template.metadata?.resourcePreset
  if (typeof metadataPreset === "string" && metadataPreset) {
    return metadataPreset
  }
  return presetForResources(template.cpuRequest || "", template.memoryRequest || "")
}

export function templatePersistence(template: Template) {
  return template.storageRequest ? `Workspace ${template.storageRequest}` : "Ephemeral workspace"
}

export function templateValidationText(template: Template) {
  const status = templateValidationStatus(template)
  if (status === "passed") {
    return "Validated"
  }
  if (status === "failed") {
    return "Failed"
  }
  if (status === "testing") {
    return "Testing"
  }
  return "Not tested"
}

export function templateValidationStatus(template: Template) {
  const status = template.metadata?.validationStatus
  if (status === "passed" || status === "failed" || status === "testing") {
    return status
  }
  return "not_tested"
}

export function templateValidationTone(template: Template) {
  const status = templateValidationStatus(template)
  if (status === "passed") {
    return "success"
  }
  if (status === "failed") {
    return "danger"
  }
  if (status === "testing") {
    return "warning"
  }
  return "neutral"
}

export function templateValidationHint(template: Template) {
  const status = templateValidationStatus(template)
  if (status === "passed") {
    return "Ready for repeated launch"
  }
  if (status === "failed") {
    return "Needs correction before default use"
  }
  if (status === "testing") {
    return "Validation sandbox in progress"
  }
  return "No validation sandbox yet"
}

export function storageSummary(storage: RuntimeStorage[] | undefined) {
  const workspace = storage?.find((item) => item.mountPath === "/workspace") || storage?.[0]
  if (!workspace) {
    return "No PVC"
  }
  return [workspace.phase || "PVC", workspace.capacity, workspace.claimName].filter(Boolean).join(" · ")
}

export function sandboxPreviewPortsText(sandbox: Sandbox) {
  const ports = sandbox.ports || []
  return ports.length ? formatPorts(ports) : "No preview ports"
}

export function sandboxPreviewPortsHint(sandbox: Sandbox) {
  const ports = sandbox.ports || []
  if (!ports.length) {
    return "No declared TCP ports"
  }
  return `${ports.length} declared TCP ${ports.length === 1 ? "port" : "ports"}`
}

export function parsePorts(value: string) {
  const ports = []
  for (const item of value.split(",").map((entry) => entry.trim()).filter(Boolean)) {
    const parts = item.split(":")
    if (parts.length > 2) {
      throw new Error("Preview ports must use name:port or port")
    }
    const hasName = parts.length === 2
    const rawName = hasName ? parts[0].trim() : ""
    const rawPort = hasName ? parts[1].trim() : parts[0].trim()
    const port = Number(rawPort)
    if ((hasName && !rawName) || !Number.isInteger(port) || port < 1 || port > 65535) {
      throw new Error("Preview ports must be integers between 1 and 65535")
    }
    ports.push({
      name: hasName ? rawName : `port-${port}`,
      port,
      protocol: "TCP",
    })
  }
  return ports
}

export function formatPorts(ports: Array<{ name: string; port: number; protocol?: string }>) {
  return ports.map((port) => `${port.name}:${port.port}`).join(", ")
}

export function formatCommand(command: string[] | undefined) {
  if (!command?.length) {
    return ""
  }
  if (command.length === 3 && command[0] === "sh" && command[1] === "-c") {
    if (command[2].includes("'")) {
      return JSON.stringify(command)
    }
    return `sh -c '${command[2]}'`
  }
  return JSON.stringify(command)
}

export function parseKeyValueLines(value: string) {
  const record: Record<string, string> = {}
  for (const line of value.split(/\r?\n/)) {
    const trimmed = line.trim()
    if (!trimmed) {
      continue
    }
    const separator = trimmed.indexOf("=")
    if (separator < 1) {
      continue
    }
    const key = trimmed.slice(0, separator).trim()
    const entry = trimmed.slice(separator + 1).trim()
    if (key) {
      record[key] = entry
    }
  }
  return record
}

export function formatKeyValueLines(value: Record<string, string> | undefined) {
  return Object.entries(value || {})
    .map(([key, entry]) => `${key}=${entry}`)
    .join("\n")
}

export function parseSecretRefs(value: string) {
  return value
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean)
    .map((item) => {
      const [name, key] = item.split(":")
      return { name: name.trim(), key: key?.trim() || undefined }
    })
    .filter((item) => item.name)
}

export function formatSecretRefs(refs: Array<{ name: string; key?: string }> | undefined) {
  return (refs || []).map((ref) => (ref.key ? `${ref.name}:${ref.key}` : ref.name)).join(", ")
}

export function parseJSONField(value: string, label: string) {
  const trimmed = value.trim()
  if (!trimmed) {
    return {}
  }
  try {
    return JSON.parse(trimmed) as Record<string, unknown>
  } catch {
    throw new Error(`${label} must be valid JSON`)
  }
}

export function formatJSONField(value: Record<string, unknown> | undefined) {
  if (!value || Object.keys(value).length === 0) {
    return ""
  }
  return JSON.stringify(value, null, 2)
}

export function templatePayloadFromForm(data: FormRecord) {
  const runtimeType = stringValue(data.runtimeType) || "Custom"
  const selectedResourcePreset = stringValue(data.resourcePreset) || "Small"
  const presetResources = resourcesForPreset(selectedResourcePreset)
  const image = stringValue(data.image) || defaultImageForRuntime(runtimeType)
  const cpuRequest = stringValue(data.cpuRequest) || presetResources.cpuRequest
  const memoryRequest = stringValue(data.memoryRequest) || presetResources.memoryRequest
  const resourcePreset = presetForResources(cpuRequest, memoryRequest)
  return {
    projectId: stringValue(data.projectId),
    name: stringValue(data.name),
    slug: stringValue(data.slug) || generatedSlug(stringValue(data.name), "environment"),
    image,
    startupCommand: parseCommand(stringValue(data.startupCommand)),
    workingDir: stringValue(data.workingDir) || "/workspace",
    cpuRequest,
    memoryRequest,
    storageRequest: stringValue(data.storageRequest),
    exposedPorts: parsePorts(stringValue(data.exposedPorts)),
    env: parseKeyValueLines(stringValue(data.env)),
    secretRefs: parseSecretRefs(stringValue(data.secretRefs)),
    networkPolicy: stringValue(data.networkPolicy),
    lifecyclePolicy: parseJSONField(stringValue(data.lifecyclePolicy), "Cleanup policy"),
    metadata: {
      runtimeType,
      useCase: stringValue(data.useCase) || defaultUseCaseForRuntime(runtimeType),
      resourcePreset,
      validationStatus: "not_tested",
    },
  }
}

export function defaultImageForRuntime(runtimeType: string) {
  if (runtimeType === "Python") {
    return "python:3.12-bookworm"
  }
  if (runtimeType === "Go") {
    return "golang:1.24-bookworm"
  }
  if (runtimeType === "Browser") {
    return "mcr.microsoft.com/playwright:v1.57.0-noble"
  }
  if (runtimeType === "Notebook") {
    return "python:3.12-bookworm"
  }
  return "node:22-bookworm-slim"
}

export function defaultStartupCommandForRuntime(runtimeType: string) {
  if (runtimeType === "Python") {
    return "sh -c 'mkdir -p /workspace && cd /workspace && echo mbox python sandbox ready && tail -f /dev/null'"
  }
  if (runtimeType === "Go") {
    return "sh -c 'mkdir -p /workspace && cd /workspace && echo mbox go sandbox ready && tail -f /dev/null'"
  }
  if (runtimeType === "Browser") {
    return "sh -c 'mkdir -p /workspace && cd /workspace && echo mbox browser sandbox ready && tail -f /dev/null'"
  }
  if (runtimeType === "Notebook") {
    return "sh -c 'mkdir -p /workspace && cd /workspace && echo mbox notebook sandbox ready && tail -f /dev/null'"
  }
  return "sh -c 'mkdir -p /workspace && cd /workspace && echo mbox node sandbox ready && tail -f /dev/null'"
}

export function defaultUseCaseForRuntime(runtimeType: string) {
  if (runtimeType === "Python") {
    return "Data analysis workspace"
  }
  if (runtimeType === "Go") {
    return "API service"
  }
  if (runtimeType === "Browser") {
    return "Agent browser runtime"
  }
  if (runtimeType === "Notebook") {
    return "Notebook workspace"
  }
  return "Web app preview"
}

export function defaultPortsForRuntime(runtimeType: string) {
  if (runtimeType === "Go") {
    return "api:8080"
  }
  if (runtimeType === "Notebook") {
    return "notebook:8888"
  }
  if (runtimeType === "Browser") {
    return ""
  }
  return "web:3000"
}

export function resourcesForPreset(preset: string) {
  if (preset === "Large") {
    return { cpuRequest: "1000m", memoryRequest: "2Gi" }
  }
  if (preset === "Medium") {
    return { cpuRequest: "500m", memoryRequest: "1Gi" }
  }
  if (preset === "Small") {
    return { cpuRequest: "250m", memoryRequest: "512Mi" }
  }
  return { cpuRequest: "", memoryRequest: "" }
}

export function presetForResources(cpuRequest: string, memoryRequest: string) {
  if (cpuRequest === "1000m" && memoryRequest === "2Gi") {
    return "Large"
  }
  if (cpuRequest === "500m" && memoryRequest === "1Gi") {
    return "Medium"
  }
  if (cpuRequest === "250m" && memoryRequest === "512Mi") {
    return "Small"
  }
  return "Custom"
}

export function runtimeText(ref: RuntimeRef | undefined) {
  if (!ref) {
    return "-"
  }
  return `${ref.kind} ${ref.namespace}/${ref.name}`
}

export function formatDateTime(value: string | undefined) {
  if (!value) {
    return "-"
  }
  return new Intl.DateTimeFormat(undefined, {
    month: "short",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(value))
}

export function formatClockTime(value: string | undefined) {
  if (!value) {
    return "-"
  }
  return new Intl.DateTimeFormat(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).format(new Date(value))
}

export function formatDuration(start: string | undefined, end: string | undefined) {
  if (!start) {
    return "-"
  }
  const startMs = Date.parse(start)
  const endMs = end ? Date.parse(end) : Date.now()
  if (!Number.isFinite(startMs) || !Number.isFinite(endMs) || endMs < startMs) {
    return "-"
  }
  const totalSeconds = Math.floor((endMs - startMs) / 1000)
  if (totalSeconds < 60) {
    return `${totalSeconds}s`
  }
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  if (minutes < 60) {
    return `${minutes}m ${seconds}s`
  }
  const hours = Math.floor(minutes / 60)
  const remainingMinutes = minutes % 60
  return `${hours}h ${remainingMinutes}m`
}

export function formatBytes(value: number | undefined) {
  if (typeof value !== "number") {
    return "-"
  }
  if (value < 1024) {
    return `${value} B`
  }
  if (value < 1024 * 1024) {
    return `${(value / 1024).toFixed(1)} KiB`
  }
  if (value < 1024 * 1024 * 1024) {
    return `${(value / (1024 * 1024)).toFixed(1)} MiB`
  }
  return `${(value / (1024 * 1024 * 1024)).toFixed(1)} GiB`
}

export function formatTaskCommand(command: string[] | undefined) {
  if (!command?.length) {
    return "-"
  }
  if (command.length === 3 && command[0] === "sh" && command[1] === "-lc") {
    return command[2]
  }
  if (command.length === 3 && command[0] === "sh" && command[1] === "-c") {
    return command[2]
  }
  return command.join(" ")
}

export function isActiveTask(task: ExecutionTask) {
  return task.status === "queued" || task.status === "running"
}

export function taskStatusLabel(status: ExecutionTaskStatus) {
  if (status === "timed_out") {
    return "timed out"
  }
  return status
}

export function taskStatusTone(status: ExecutionTaskStatus) {
  if (status === "succeeded") {
    return "success"
  }
  if (status === "failed" || status === "timed_out" || status === "canceled") {
    return "danger"
  }
  if (status === "queued" || status === "running") {
    return "warning"
  }
  return "neutral"
}

export function artifactKindLabel(kind: ArtifactKind) {
  return kind
    .split("_")
    .map((part) => part.slice(0, 1).toUpperCase() + part.slice(1))
    .join(" ")
}

export function terminalURL(sandboxID: string) {
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:"
  return `${protocol}//${window.location.host}/v1/sandboxes/${sandboxID}/terminal`
}

export function shortID(id: string | undefined) {
  return id ? `${id.slice(0, 8)}...` : "-"
}

export function parseCommand(value: string) {
  const trimmed = value.trim()
  if (!trimmed) {
    return []
  }
  if (trimmed.startsWith("[") && trimmed.endsWith("]")) {
    try {
      const parsed = JSON.parse(trimmed)
      return Array.isArray(parsed) ? parsed.map(String) : [trimmed]
    } catch {
      return [trimmed]
    }
  }
  if (trimmed.startsWith("sh -c ")) {
    return ["sh", "-c", trimmed.slice(6).replace(/^['"]|['"]$/g, "")]
  }
  return trimmed.split(/\s+/)
}

export function slugFromName(value: string) {
  return value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
}

export function generatedSlug(value: string, fallbackPrefix: string) {
  return slugFromName(value) || `${fallbackPrefix}-${Date.now().toString(36)}`
}

export function stringValue(value: FormDataEntryValue | undefined) {
  return typeof value === "string" ? value.trim() : ""
}

export function compactObject<T extends Record<string, unknown>>(value: T) {
  return Object.fromEntries(
    Object.entries(value).filter(([, entry]) => {
      if (entry === undefined || entry === null || entry === "") {
        return false
      }
      if (Array.isArray(entry) && entry.length === 0) {
        return false
      }
      if (entry === "global" || entry === "default") {
        return false
      }
      return true
    }),
  )
}

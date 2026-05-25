import type {
  FormRecord,
  Project,
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
    rows.push(["Default template", templateName(project.defaultTemplateId, templates)])
  }
  if (kind === "template") {
    const template = item as Template
    rows.push(["Runtime", templateRuntimeType(template)])
    rows.push(["Use case", templateUseCase(template)])
    rows.push(["Entrypoints", templateEntrypoints(template)])
    rows.push(["Preset", templateResourcePreset(template)])
    rows.push(["Storage", templatePersistence(template)])
    rows.push(["Validation", templateValidationText(template)])
    rows.push(["Image", template.image])
    rows.push(["Working dir", template.workingDir || ""])
    rows.push(["Project", template.projectId ? projectName(template.projectId, projects) : "Global"])
  }
  if (kind === "sandbox") {
    const sandbox = item as Sandbox
    rows.push(["Status", sandbox.status])
    rows.push(["Project", projectName(sandbox.projectId, projects)])
    rows.push(["Template", templateName(sandbox.templateId, templates)])
    rows.push(["Namespace", sandbox.namespace])
    rows.push(["ServiceAccount", sandbox.serviceAccountName])
    rows.push(["Runtime", runtimeText(sandbox.runtimeRef)])
  }
  return rows
}

export function projectName(id: string | undefined, projects: Project[]) {
  return projects.find((project) => project.id === id)?.name || shortID(id)
}

export function templateName(id: string | undefined, templates: Template[]) {
  if (!id) {
    return "-"
  }
  return templates.find((template) => template.id === id)?.name || shortID(id)
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
  const status = template.metadata?.validationStatus
  if (status === "passed") {
    return "Validated"
  }
  if (status === "failed") {
    return "Failed"
  }
  return "Not tested"
}

export function storageSummary(storage: RuntimeStorage[] | undefined) {
  const workspace = storage?.find((item) => item.mountPath === "/workspace") || storage?.[0]
  if (!workspace) {
    return "No PVC"
  }
  return [workspace.phase || "PVC", workspace.capacity, workspace.claimName].filter(Boolean).join(" · ")
}

export function parsePorts(value: string) {
  const ports = []
  for (const item of value.split(",").map((entry) => entry.trim()).filter(Boolean)) {
    const parts = item.split(":")
    if (parts.length > 2) {
      throw new Error("Entrypoints must use name:port or port")
    }
    const hasName = parts.length === 2
    const rawName = hasName ? parts[0].trim() : ""
    const rawPort = hasName ? parts[1].trim() : parts[0].trim()
    const port = Number(rawPort)
    if ((hasName && !rawName) || !Number.isInteger(port) || port < 1 || port > 65535) {
      throw new Error("Entrypoint ports must be integers between 1 and 65535")
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
    slug: stringValue(data.slug),
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
    lifecyclePolicy: parseJSONField(stringValue(data.lifecyclePolicy), "Lifecycle policy"),
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

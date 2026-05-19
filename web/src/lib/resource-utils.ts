import type {
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
    rows.push(["Image", template.image])
    rows.push(["Working dir", template.workingDir || ""])
    rows.push(["Resources", resourceText(template)])
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

export function resourceText(template: Template) {
  return [template.cpuRequest, template.memoryRequest, template.storageRequest].filter(Boolean).join(" / ") || "-"
}

export function storageSummary(storage: RuntimeStorage[] | undefined) {
  const workspace = storage?.find((item) => item.mountPath === "/workspace") || storage?.[0]
  if (!workspace) {
    return "No PVC"
  }
  return [workspace.phase || "PVC", workspace.capacity, workspace.claimName].filter(Boolean).join(" · ")
}

export function parsePorts(value: string) {
  return value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean)
    .map((item) => {
      const [nameOrPort, rawPort] = item.split(":")
      const port = Number(rawPort || nameOrPort)
      return {
        name: rawPort ? nameOrPort.trim() : `port-${port}`,
        port,
        protocol: "TCP",
      }
    })
    .filter((item) => item.port >= 1 && item.port <= 65535)
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

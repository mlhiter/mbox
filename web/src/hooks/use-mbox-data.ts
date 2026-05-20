import { useCallback, useMemo, useState } from "react"
import { toast } from "sonner"
import {
  createProject as createProjectRequest,
  createSandbox as createSandboxRequest,
  createTemplate as createTemplateRequest,
  deleteSandbox as deleteSandboxRequest,
  getHealth,
  getSandbox,
  listProjects,
  listSandboxes,
  listTemplates,
  startSandbox as startSandboxRequest,
  stopSandbox as stopSandboxRequest,
  updateProject,
} from "@/lib/api"
import {
  compactObject,
  parseCommand,
  parsePorts,
  slugFromName,
  stringValue,
} from "@/lib/resource-utils"
import type {
  APIStatus,
  FormRecord,
  Project,
  Sandbox,
  Selection,
  Template,
} from "@/types"

const initialAPIStatus: APIStatus = {
  state: "checking",
  label: "Checking API",
}

export function useMboxData() {
  const [projects, setProjects] = useState<Project[]>([])
  const [templates, setTemplates] = useState<Template[]>([])
  const [sandboxes, setSandboxes] = useState<Sandbox[]>([])
  const [selection, setSelection] = useState<Selection | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [apiState, setAPIState] = useState<APIStatus>(initialAPIStatus)

  const counts = useMemo(
    () => ({
      projects: projects.length,
      templates: templates.length,
      sandboxes: sandboxes.length,
      running: sandboxes.filter((sandbox) => sandbox.status === "running").length,
    }),
    [projects, sandboxes, templates],
  )

  const selectedSandbox = useMemo(() => {
    if (selection?.kind !== "sandbox") {
      return null
    }
    return sandboxes.find((sandbox) => sandbox.id === selection.id) || null
  }, [sandboxes, selection])

  const loadAll = useCallback(async () => {
    setLoading(true)
    setError(null)
    setAPIState(initialAPIStatus)
    try {
      const [health, projectList, templateList, sandboxList] = await Promise.all([
        getHealth(),
        listProjects(),
        listTemplates(),
        listSandboxes(),
      ])
      setProjects(projectList.items || [])
      setTemplates(templateList.items || [])
      setSandboxes(sandboxList.items || [])
      setAPIState({
        state: health.status === "ok" ? "ok" : "bad",
        label: health.status || "Unknown",
      })
    } catch (requestError) {
      const message = requestError instanceof Error ? requestError.message : "Request failed"
      setError(message)
      setAPIState({ state: "bad", label: "API unavailable" })
      toast.error(message)
    } finally {
      setLoading(false)
    }
  }, [])

  const createProject = useCallback(
    async (data: FormRecord) => {
      await createProjectRequest({
        name: stringValue(data.name),
        slug: stringValue(data.slug),
        repositoryUrl: stringValue(data.repositoryUrl),
        defaultNamespace: stringValue(data.defaultNamespace),
      })
      await loadAll()
      toast.success("Project created")
    },
    [loadAll],
  )

  const createTemplate = useCallback(
    async (data: FormRecord) => {
      const projectId = stringValue(data.projectId)
      const payload = compactObject({
        projectId,
        name: stringValue(data.name),
        slug: stringValue(data.slug),
        image: stringValue(data.image),
        startupCommand: parseCommand(stringValue(data.startupCommand)),
        workingDir: stringValue(data.workingDir),
        cpuRequest: stringValue(data.cpuRequest),
        memoryRequest: stringValue(data.memoryRequest),
        storageRequest: stringValue(data.storageRequest),
        exposedPorts: parsePorts(stringValue(data.exposedPorts)),
      })
      const template = await createTemplateRequest(payload)
      if (data.setDefault === "on" && projectId && projectId !== "global") {
        await updateProject(projectId, { defaultTemplateId: template.id })
      }
      await loadAll()
      toast.success("Template created")
    },
    [loadAll],
  )

  const createSandbox = useCallback(
    async (data: FormRecord) => {
      const name = stringValue(data.name)
      const payload = compactObject({
        projectId: stringValue(data.projectId),
        templateId: stringValue(data.templateId),
        name,
        slug: slugFromName(name),
      })
      const sandbox = await createSandboxRequest(payload)
      await loadAll()
      setSelection({ kind: "sandbox", id: sandbox.id })
      toast.success("Sandbox launched")
    },
    [loadAll],
  )

  const deleteSandbox = useCallback(
    async (id: string) => {
      try {
        await deleteSandboxRequest(id)
        if (selection?.kind === "sandbox" && selection.id === id) {
          setSelection(null)
        }
        await loadAll()
        toast.success("Sandbox deleted")
      } catch (deleteError) {
        const message = deleteError instanceof Error ? deleteError.message : "Delete failed"
        toast.error(message)
        throw deleteError
      }
    },
    [loadAll, selection],
  )

  const stopSandbox = useCallback(
    async (id: string) => {
      try {
        const sandbox = await stopSandboxRequest(id)
        setSandboxes((current) => current.map((item) => (item.id === id ? sandbox : item)))
        toast.success("Sandbox stopped")
        return sandbox
      } catch (stopError) {
        const message = stopError instanceof Error ? stopError.message : "Stop failed"
        toast.error(message)
        throw stopError
      }
    },
    [],
  )

  const startSandbox = useCallback(
    async (id: string) => {
      try {
        const sandbox = await startSandboxRequest(id)
        setSandboxes((current) => current.map((item) => (item.id === id ? sandbox : item)))
        toast.success("Sandbox starting")
        return sandbox
      } catch (startError) {
        const message = startError instanceof Error ? startError.message : "Start failed"
        toast.error(message)
        throw startError
      }
    },
    [],
  )

  const refreshSandbox = useCallback(async (id: string) => {
    const sandbox = await getSandbox(id)
    setSandboxes((current) => current.map((item) => (item.id === id ? sandbox : item)))
    return sandbox
  }, [])

  return {
    apiState,
    counts,
    createProject,
    createSandbox,
    createTemplate,
    deleteSandbox,
    error,
    loadAll,
    loading,
    projects,
    refreshSandbox,
    sandboxes,
    selectedSandbox,
    selection,
    setSelection,
    startSandbox,
    stopSandbox,
    templates,
  }
}

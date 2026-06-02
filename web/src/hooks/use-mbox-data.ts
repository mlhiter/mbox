import { useCallback, useMemo, useState } from "react"
import { toast } from "sonner"
import {
  createProject as createProjectRequest,
  createProjectMember as createProjectMemberRequest,
  createSandbox as createSandboxRequest,
  createTemplate as createTemplateRequest,
  createTemplateValidationRun,
  decideTemplateValidationRun as decideTemplateValidationRunRequest,
  deleteProjectMember as deleteProjectMemberRequest,
  deleteSandbox as deleteSandboxRequest,
  getCaller,
  getHealth,
  getProjectAuthorization,
  getProjectPolicy,
  getProjectQuotaPolicy,
  getProjectUsage,
  getRuntimeResources,
  getSandbox,
  listProjectAuditEvents,
  listProjectCredentials,
  listProjectMembers,
  listProjects,
  listSandboxes,
  listTemplates,
  startSandbox as startSandboxRequest,
  stopSandbox as stopSandboxRequest,
  updateProject,
  updateTemplate as updateTemplateRequest,
} from "@/lib/api"
import {
  compactObject,
  generatedSlug,
  stringValue,
  templatePayloadFromForm,
} from "@/lib/resource-utils"
import type {
  APIStatus,
  AuditEvent,
  CallerInfo,
  FormRecord,
  Project,
  ProjectAuthorizationDecision,
  ProjectCredential,
  ProjectMember,
  ProjectMemberPrincipalType,
  ProjectMemberRole,
  ProjectPolicy,
  ProjectQuotaPolicy,
  ProjectUsage,
  RuntimeResourceList,
  Sandbox,
  Selection,
  Template,
} from "@/types"

const initialAPIStatus: APIStatus = {
  state: "checking",
  label: "Checking API",
}

type RuntimeResourceFilters = {
  namespace?: string
  projectId?: string
  kind?: string
}

type ProjectMemberInput = {
  principalType: ProjectMemberPrincipalType
  principal: string
  role: ProjectMemberRole
}

export function useMboxData() {
  const [projects, setProjects] = useState<Project[]>([])
  const [projectPolicies, setProjectPolicies] = useState<Record<string, ProjectPolicy>>({})
  const [projectAuthorizations, setProjectAuthorizations] = useState<Record<string, ProjectAuthorizationDecision[]>>({})
  const [projectQuotaPolicies, setProjectQuotaPolicies] = useState<Record<string, ProjectQuotaPolicy>>({})
  const [projectCredentials, setProjectCredentials] = useState<Record<string, ProjectCredential[]>>({})
  const [projectMembers, setProjectMembers] = useState<Record<string, ProjectMember[]>>({})
  const [projectUsage, setProjectUsage] = useState<Record<string, ProjectUsage>>({})
  const [projectAuditEvents, setProjectAuditEvents] = useState<Record<string, AuditEvent[]>>({})
  const [runtimeResources, setRuntimeResources] = useState<RuntimeResourceList | null>(null)
  const [runtimeResourcesError, setRuntimeResourcesError] = useState<string | null>(null)
  const [callerInfo, setCallerInfo] = useState<CallerInfo | null>(null)
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

  const loadAll = useCallback(async (runtimeFilters: RuntimeResourceFilters = {}) => {
    setLoading(true)
    setError(null)
    setAPIState(initialAPIStatus)
    try {
      const [health, caller, projectList, templateList, sandboxList] = await Promise.all([
        getHealth(),
        getCaller(),
        listProjects(),
        listTemplates(),
        listSandboxes(),
      ])
      setCallerInfo(caller)
      const nextProjects = projectList.items || []
      setProjects(nextProjects)
      setTemplates(templateList.items || [])
      setSandboxes(sandboxList.items || [])
      const policies = await Promise.all(
        nextProjects.map(async (project) => {
          try {
            return await getProjectPolicy(project.id)
          } catch {
            return {
              projectId: project.id,
              enforcement: "disabled" as const,
            }
          }
        }),
      )
      setProjectPolicies(Object.fromEntries(policies.map((policy) => [policy.projectId, policy])))
      const authorizations = await Promise.all(
        nextProjects.map(async (project) => {
          try {
            const decisions: ProjectAuthorizationDecision[] = await Promise.all([
              getProjectAuthorization(project.id, "sandbox.launch"),
              getProjectAuthorization(project.id, "runtime.operate"),
              getProjectAuthorization(project.id, "artifact.write"),
              getProjectAuthorization(project.id, "policy.manage"),
              getProjectAuthorization(project.id, "credential.manage"),
              getProjectAuthorization(project.id, "member.manage"),
            ])
            return [project.id, decisions] as const
          } catch {
            return [project.id, undefined] as const
          }
        }),
      )
      setProjectAuthorizations(
        Object.fromEntries(
          authorizations.filter((entry): entry is readonly [string, ProjectAuthorizationDecision[]] => Boolean(entry[1])),
        ),
      )
      const quotaPolicies = await Promise.all(
        nextProjects.map(async (project) => {
          try {
            return await getProjectQuotaPolicy(project.id)
          } catch {
            return {
              projectId: project.id,
              enforcement: "disabled" as const,
            }
          }
        }),
      )
      setProjectQuotaPolicies(Object.fromEntries(quotaPolicies.map((policy) => [policy.projectId, policy])))
      const credentials = await Promise.all(
        nextProjects.map(async (project) => {
          try {
            const result = await listProjectCredentials(project.id)
            return [project.id, result.items || []] as const
          } catch {
            return [project.id, []] as const
          }
        }),
      )
      setProjectCredentials(Object.fromEntries(credentials))
      const members = await Promise.all(
        nextProjects.map(async (project) => {
          try {
            const result = await listProjectMembers(project.id)
            return [project.id, result.items || []] as const
          } catch {
            return [project.id, []] as const
          }
        }),
      )
      setProjectMembers(Object.fromEntries(members))
      const usage = await Promise.all(
        nextProjects.map(async (project) => {
          try {
            return [project.id, await getProjectUsage(project.id)] as const
          } catch {
            return [project.id, undefined] as const
          }
        }),
      )
      setProjectUsage(Object.fromEntries(usage.filter((entry): entry is readonly [string, ProjectUsage] => Boolean(entry[1]))))
      const auditEvents = await Promise.all(
        nextProjects.map(async (project) => {
          try {
            const result = await listProjectAuditEvents(project.id)
            return [project.id, result.items || []] as const
          } catch {
            return [project.id, []] as const
          }
        }),
      )
      setProjectAuditEvents(Object.fromEntries(auditEvents))
      try {
        const inventory = await getRuntimeResources(runtimeFilters)
        setRuntimeResources(inventory)
        setRuntimeResourcesError(null)
      } catch (runtimeError) {
        const message = runtimeError instanceof Error ? runtimeError.message : "Runtime inventory unavailable"
        setRuntimeResources(null)
        setRuntimeResourcesError(message)
      }
      setAPIState({
        state: health.status === "ok" ? "ok" : "bad",
        label: health.status || "Unknown",
      })
    } catch (requestError) {
      const message = requestError instanceof Error ? requestError.message : "Request failed"
      setError(message)
      setCallerInfo(null)
      setProjectAuthorizations({})
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
        slug: stringValue(data.slug) || generatedSlug(stringValue(data.name), "project"),
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
      const parsed = templatePayloadFromForm(data)
      const projectId = parsed.projectId
      const payload = compactObject(parsed)
      const template = await createTemplateRequest(payload)
      if (data.setDefault === "on" && projectId && projectId !== "global") {
        await updateProject(projectId, { defaultTemplateId: template.id })
      }
      await loadAll()
      toast.success("Environment created")
    },
    [loadAll],
  )

  const updateTemplate = useCallback(
    async (id: string, data: FormRecord) => {
      const parsed = templatePayloadFromForm(data)
      const { projectId: _projectId, slug: _slug, ...payload } = parsed
      await updateTemplateRequest(id, payload)
      await loadAll()
      setSelection({ kind: "template", id })
      toast.success("Environment updated")
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
        slug: generatedSlug(name, "sandbox"),
      })
      const sandbox = await createSandboxRequest(payload)
      await loadAll()
      setSelection({ kind: "sandbox", id: sandbox.id })
      toast.success("Sandbox launched")
      return sandbox
    },
    [loadAll],
  )

  const validateTemplate = useCallback(
    async (id: string) => {
      try {
        const template = templates.find((item) => item.id === id)
        if (!template) {
          throw new Error("Environment not found")
        }
        const project = template.projectId
          ? projects.find((item) => item.id === template.projectId)
          : projects[0]
        if (!project) {
          throw new Error("Create a project before validating an environment.")
        }
        const validationName = `Validate ${template.name}`.slice(0, 58)
        const run = await createTemplateValidationRun(template.id, {
          projectId: project.id,
          name: validationName,
        })
        await loadAll()
        setSelection({ kind: "sandbox", id: run.sandbox.id })
        toast.success("Validation sandbox launched")
        return run.sandbox
      } catch (validationError) {
        const message = validationError instanceof Error ? validationError.message : "Validation launch failed"
        toast.error(message)
        throw validationError
      }
    },
    [loadAll, projects, templates],
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

  const decideTemplateValidation = useCallback(
    async (sandboxID: string, status: "passed" | "failed") => {
      try {
        const sandbox = sandboxes.find((item) => item.id === sandboxID)
        if (!sandbox) {
          throw new Error("Sandbox not found")
        }
        const templateID = typeof sandbox.metadata?.templateId === "string" ? sandbox.metadata.templateId : sandbox.templateId
        const template = templates.find((item) => item.id === templateID)
        if (!template || !templateID) {
          throw new Error("Environment not found for this validation run")
        }
        await decideTemplateValidationRunRequest(templateID, sandbox.id, status)
        await loadAll()
        setSelection({ kind: "sandbox", id: sandbox.id })
        toast.success(status === "passed" ? "Environment marked validated" : "Environment marked failed")
      } catch (validationError) {
        const message = validationError instanceof Error ? validationError.message : "Could not update validation result"
        toast.error(message)
        throw validationError
      }
    },
    [loadAll, sandboxes, templates],
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

  const refreshProjectAuditEvents = useCallback(
    async (
      projectID: string,
      filters: {
        action?: string
        actor?: string
        source?: string
        requestId?: string
        operation?: string
        reason?: string
        since?: string
        until?: string
      } = {},
    ) => {
      const result = await listProjectAuditEvents(projectID, {
        limit: 20,
        action: filters.action,
        actor: filters.actor,
        source: filters.source,
        requestId: filters.requestId,
        operation: filters.operation,
        reason: filters.reason,
        since: filters.since,
        until: filters.until,
      })
      const events = result.items || []
      setProjectAuditEvents((current) => ({ ...current, [projectID]: events }))
      return events
    },
    [],
  )

  const refreshProjectMembers = useCallback(async (projectID: string) => {
    const result = await listProjectMembers(projectID)
    const members = result.items || []
    setProjectMembers((current) => ({ ...current, [projectID]: members }))
    return members
  }, [])

  const refreshProjectAuthorizations = useCallback(async (projectID: string) => {
    const decisions: ProjectAuthorizationDecision[] = await Promise.all([
      getProjectAuthorization(projectID, "sandbox.launch"),
      getProjectAuthorization(projectID, "runtime.operate"),
      getProjectAuthorization(projectID, "artifact.write"),
      getProjectAuthorization(projectID, "policy.manage"),
      getProjectAuthorization(projectID, "credential.manage"),
      getProjectAuthorization(projectID, "member.manage"),
    ])
    setProjectAuthorizations((current) => ({ ...current, [projectID]: decisions }))
    return decisions
  }, [])

  const createProjectMember = useCallback(
    async (projectID: string, input: ProjectMemberInput) => {
      try {
        const principal = input.principal.trim()
        if (!principal) {
          throw new Error("Principal is required")
        }
        const member = await createProjectMemberRequest(projectID, {
          principalType: input.principalType,
          principal,
          role: input.role,
        })
        await Promise.all([
          refreshProjectMembers(projectID),
          refreshProjectAuthorizations(projectID).catch(() => undefined),
          refreshProjectAuditEvents(projectID).catch(() => []),
        ])
        toast.success("Member registered")
        return member
      } catch (memberError) {
        const message = memberError instanceof Error ? memberError.message : "Could not register member"
        toast.error(message)
        throw memberError
      }
    },
    [refreshProjectAuditEvents, refreshProjectAuthorizations, refreshProjectMembers],
  )

  const deleteProjectMember = useCallback(
    async (member: ProjectMember) => {
      try {
        await deleteProjectMemberRequest(member.id)
        await Promise.all([
          refreshProjectMembers(member.projectId),
          refreshProjectAuthorizations(member.projectId).catch(() => undefined),
          refreshProjectAuditEvents(member.projectId).catch(() => []),
        ])
        toast.success("Member removed")
      } catch (memberError) {
        const message = memberError instanceof Error ? memberError.message : "Could not remove member"
        toast.error(message)
        throw memberError
      }
    },
    [refreshProjectAuditEvents, refreshProjectAuthorizations, refreshProjectMembers],
  )

  const refreshRuntimeResources = useCallback(async (filters: RuntimeResourceFilters = {}) => {
    try {
      const inventory = await getRuntimeResources(filters)
      setRuntimeResources(inventory)
      setRuntimeResourcesError(null)
      return inventory
    } catch (runtimeError) {
      const message = runtimeError instanceof Error ? runtimeError.message : "Runtime inventory unavailable"
      setRuntimeResources(null)
      setRuntimeResourcesError(message)
      throw runtimeError
    }
  }, [])

  return {
    apiState,
    callerInfo,
    counts,
    createProject,
    createProjectMember,
    createSandbox,
    createTemplate,
    decideTemplateValidation,
    deleteProjectMember,
    deleteSandbox,
    error,
    loadAll,
    loading,
    projectPolicies,
    projectQuotaPolicies,
    projectCredentials,
    projectAuthorizations,
    projectMembers,
    projectAuditEvents,
    projectUsage,
    projects,
    refreshProjectAuditEvents,
    refreshRuntimeResources,
    refreshSandbox,
    runtimeResources,
    runtimeResourcesError,
    sandboxes,
    selectedSandbox,
    selection,
    setSelection,
    startSandbox,
    stopSandbox,
    templates,
    updateTemplate,
    validateTemplate,
  }
}

import { useEffect, useMemo, useState } from "react"
import { RefreshCw } from "lucide-react"
import { Button } from "@/components/ui/button"
import { AppShell } from "@/components/console/app-shell"
import { SummaryStrip } from "@/components/console/summary-strip"
import { ProjectTable } from "@/features/resources/project-table"
import { SandboxTable } from "@/features/resources/sandbox-table"
import { TemplateTable } from "@/features/resources/template-table"
import { SandboxDetailFallback, SandboxDetailPage } from "@/features/runtime/sandbox-detail-page"
import { useMboxData } from "@/hooks/use-mbox-data"
import { collectionFor } from "@/lib/resource-utils"
import type { WorkspaceView } from "@/types"

const workspaceCopy: Record<WorkspaceView, { eyebrow: string; title: string; note: string }> = {
  projects: {
    eyebrow: "Scope",
    title: "Projects",
    note: "Bind repositories, namespaces, and default environments before launching runtime work.",
  },
  templates: {
    eyebrow: "Launch shape",
    title: "Environments",
    note: "Manage ready-to-run environments that users launch into sandboxes.",
  },
  sandboxes: {
    eyebrow: "Execution",
    title: "Sandboxes",
    note: "Launch controlled runtimes, open the workspace, and inspect runtime state from one place.",
  },
  "sandbox-detail": {
    eyebrow: "Execution",
    title: "Sandbox",
    note: "Operate one runtime workspace, inspect boundaries, and review outputs.",
  },
}

type RouteState = {
  view: WorkspaceView
  sandboxId?: string
}

function safeDecode(value: string) {
  try {
    return decodeURIComponent(value)
  } catch {
    return value
  }
}

function routeFromHash(hash: string): RouteState {
  const parts = hash
    .replace(/^#\/?/, "")
    .split("/")
    .map((part) => safeDecode(part.trim()))
    .filter(Boolean)
  const [section, id] = parts
  if (section === "projects") {
    return { view: "projects" }
  }
  if (section === "templates" || section === "environments") {
    return { view: "templates" }
  }
  if (section === "sandboxes" && id) {
    return { view: "sandbox-detail", sandboxId: id }
  }
  return { view: "sandboxes" }
}

function currentRoute() {
  return routeFromHash(window.location.hash)
}

function hashForRoute(route: RouteState) {
  if (route.view === "projects") {
    return "#projects"
  }
  if (route.view === "templates") {
    return "#environments"
  }
  if (route.view === "sandbox-detail" && route.sandboxId) {
    return `#sandboxes/${encodeURIComponent(route.sandboxId)}`
  }
  return "#sandboxes"
}

function writeHash(route: RouteState) {
  const hash = hashForRoute(route)
  if (window.location.hash === hash) {
    return
  }
  window.history.pushState(null, "", `${window.location.pathname}${window.location.search}${hash}`)
}

export function App() {
  const initialRoute = currentRoute()
  const [activeView, setActiveView] = useState<WorkspaceView>(initialRoute.view)
  const [routeSandboxId, setRouteSandboxId] = useState<string | null>(initialRoute.sandboxId || null)
  const {
    apiState,
    counts,
    createProject,
    createSandbox,
    createTemplate,
    decideTemplateValidation,
    deleteSandbox,
    error,
    loadAll,
    loading,
    projectCredentials,
    projectAuditEvents,
    projectPolicies,
    projectQuotaPolicies,
    projectUsage,
    projects,
    refreshSandbox,
    refreshProjectAuditEvents,
    sandboxes,
    selectedSandbox,
    selection,
    setSelection,
    startSandbox,
    stopSandbox,
    templates,
    updateTemplate,
    validateTemplate,
  } = useMboxData()

  const activeCopy = workspaceCopy[activeView]
  const activeCount = useMemo(() => {
    if (activeView === "projects") {
      return projects.length
    }
    if (activeView === "templates") {
      return templates.length
    }
    return sandboxes.length
  }, [activeView, projects.length, sandboxes.length, templates.length])

  useEffect(() => {
    void loadAll()
  }, [loadAll])

  useEffect(() => {
    if (initialRoute.view === "sandbox-detail" && initialRoute.sandboxId) {
      setSelection({ kind: "sandbox", id: initialRoute.sandboxId })
    }
  }, [initialRoute.sandboxId, initialRoute.view, setSelection])

  useEffect(() => {
    function applyCurrentRoute() {
      const route = currentRoute()
      setActiveView(route.view)
      setRouteSandboxId(route.sandboxId || null)
      if (route.view === "sandbox-detail" && route.sandboxId) {
        setSelection({ kind: "sandbox", id: route.sandboxId })
      }
    }
    window.addEventListener("hashchange", applyCurrentRoute)
    window.addEventListener("popstate", applyCurrentRoute)
    return () => {
      window.removeEventListener("hashchange", applyCurrentRoute)
      window.removeEventListener("popstate", applyCurrentRoute)
    }
  }, [setSelection])

  useEffect(() => {
    if (!selection) {
      return
    }
    const selectionView: WorkspaceView =
      selection.kind === "project" ? "projects" : selection.kind === "template" ? "templates" : "sandboxes"
    const detailSandbox = activeView === "sandbox-detail" && selection.kind === "sandbox"
    if (selectionView !== activeView && !detailSandbox) {
      setSelection(null)
      return
    }
    if (loading) {
      return
    }
    const exists = collectionFor(selection.kind, projects, templates, sandboxes).some(
      (item) => item.id === selection.id,
    )
    if (!exists && !detailSandbox) {
      setSelection(null)
    }
  }, [activeView, loading, projects, sandboxes, selection, setSelection, templates])

  useEffect(() => {
    if (activeView === "sandbox-detail") {
      window.scrollTo({ top: 0, behavior: "auto" })
    }
  }, [activeView, selectedSandbox?.id])

  function navigateToView(view: WorkspaceView) {
    const route: RouteState = { view: view === "sandbox-detail" ? "sandboxes" : view }
    setActiveView(route.view)
    setRouteSandboxId(null)
    writeHash(route)
  }

  function openSandboxWorkspace(id: string) {
    const route: RouteState = { view: "sandbox-detail", sandboxId: id }
    setSelection({ kind: "sandbox", id })
    setActiveView(route.view)
    setRouteSandboxId(id)
    writeHash(route)
  }

  async function createSandboxAndOpen(data: Parameters<typeof createSandbox>[0]) {
    const sandbox = await createSandbox(data)
    openSandboxWorkspace(sandbox.id)
  }

  async function validateTemplateAndOpen(id: string) {
    const sandbox = await validateTemplate(id)
    openSandboxWorkspace(sandbox.id)
  }

  const detailSandboxId =
    activeView === "sandbox-detail"
      ? selectedSandbox?.id || (selection?.kind === "sandbox" ? selection.id : routeSandboxId || undefined)
      : undefined

  return (
    <AppShell
      activeView={activeView}
      apiState={apiState}
      projects={projects}
      templates={templates}
      sandboxes={sandboxes}
      selection={selection}
      showDetailPane={activeView !== "sandbox-detail"}
      onViewChange={navigateToView}
      onValidateTemplate={validateTemplateAndOpen}
      onOpenSandboxWorkspace={openSandboxWorkspace}
      onClearSelection={() => setSelection(null)}
      projectAuditEvents={projectAuditEvents}
      projectPolicies={projectPolicies}
      projectQuotaPolicies={projectQuotaPolicies}
      projectCredentials={projectCredentials}
      projectUsage={projectUsage}
      onRefreshProjectAuditEvents={refreshProjectAuditEvents}
    >
      {activeView === "sandbox-detail" ? (
        selectedSandbox ? (
          <SandboxDetailPage
            sandbox={selectedSandbox}
            projects={projects}
            templates={templates}
            onBack={() => navigateToView("sandboxes")}
            onRefresh={refreshSandbox}
            onStart={startSandbox}
            onStop={stopSandbox}
            onDecideValidation={decideTemplateValidation}
            onDelete={async (id) => {
              await deleteSandbox(id)
              navigateToView("sandboxes")
            }}
          />
        ) : (
          <SandboxDetailFallback
            sandboxId={detailSandboxId}
            loading={loading}
            error={error}
            onBack={() => navigateToView("sandboxes")}
            onRefresh={loadAll}
          />
        )
      ) : (
        <>
          <header className="topbar">
            <div>
              <p className="eyebrow">{activeCopy.eyebrow}</p>
              <h1 className="page-title">{activeCopy.title}</h1>
              <p className="page-note">{activeCopy.note}</p>
            </div>
            <div className="topbar-actions">
              <span>{activeCount} records</span>
              <Button onClick={() => void loadAll()}>
                <RefreshCw data-icon="inline-start" />
                Refresh
              </Button>
            </div>
          </header>

          <SummaryStrip counts={counts} />

          <div className="resource-grid">
            {activeView === "projects" ? (
              <ProjectTable
                projects={projects}
                projectPolicies={projectPolicies}
                projectQuotaPolicies={projectQuotaPolicies}
                projectCredentials={projectCredentials}
                projectUsage={projectUsage}
                templates={templates}
                sandboxes={sandboxes}
                loading={loading}
                error={error}
                selection={selection}
                onSelect={(id) => setSelection({ kind: "project", id })}
                onCreate={createProject}
              />
            ) : null}
            {activeView === "templates" ? (
              <TemplateTable
                projects={projects}
                templates={templates}
                loading={loading}
                error={error}
                selection={selection}
                onSelect={(id) => setSelection({ kind: "template", id })}
                onCreate={createTemplate}
                onUpdate={updateTemplate}
                onValidate={validateTemplateAndOpen}
              />
            ) : null}
            {activeView === "sandboxes" ? (
              <SandboxTable
                projects={projects}
                projectPolicies={projectPolicies}
                projectQuotaPolicies={projectQuotaPolicies}
                projectUsage={projectUsage}
                templates={templates}
                sandboxes={sandboxes}
                loading={loading}
                error={error}
                selection={selection}
                onSelect={(id) => setSelection({ kind: "sandbox", id })}
                onOpenWorkspace={openSandboxWorkspace}
                onCreate={createSandboxAndOpen}
                onDelete={deleteSandbox}
                onStart={startSandbox}
                onStop={stopSandbox}
              />
            ) : null}
          </div>
        </>
      )}
    </AppShell>
  )
}

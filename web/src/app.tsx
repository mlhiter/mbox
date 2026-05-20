import { useEffect, useMemo, useState } from "react"
import { RefreshCw } from "lucide-react"
import { Button } from "@/components/ui/button"
import { AppShell } from "@/components/console/app-shell"
import { SummaryStrip } from "@/components/console/summary-strip"
import { ProjectTable } from "@/features/resources/project-table"
import { SandboxTable } from "@/features/resources/sandbox-table"
import { TemplateTable } from "@/features/resources/template-table"
import { RuntimeWorkspace } from "@/features/runtime/runtime-workspace"
import { useMboxData } from "@/hooks/use-mbox-data"
import { collectionFor } from "@/lib/resource-utils"
import type { WorkspaceView } from "@/types"

const workspaceCopy: Record<WorkspaceView, { eyebrow: string; title: string; note: string }> = {
  projects: {
    eyebrow: "Scope",
    title: "Projects",
    note: "Bind repositories, namespaces, and default templates before launching runtime work.",
  },
  templates: {
    eyebrow: "Launch shape",
    title: "Templates",
    note: "Define reusable images, commands, resources, storage, and exposed ports for sandboxes.",
  },
  sandboxes: {
    eyebrow: "Execution",
    title: "Sandboxes",
    note: "Launch controlled runtimes, open the workspace, and inspect runtime state from one place.",
  },
}

export function App() {
  const [activeView, setActiveView] = useState<WorkspaceView>("sandboxes")
  const {
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
    if (!selection) {
      return
    }
    const selectionView: WorkspaceView =
      selection.kind === "project" ? "projects" : selection.kind === "template" ? "templates" : "sandboxes"
    if (selectionView !== activeView) {
      setSelection(null)
      return
    }
    const exists = collectionFor(selection.kind, projects, templates, sandboxes).some(
      (item) => item.id === selection.id,
    )
    if (!exists) {
      setSelection(null)
    }
  }, [activeView, projects, sandboxes, selection, setSelection, templates])

  useEffect(() => {
    if (selectedSandbox && activeView === "sandboxes") {
      const prefersReducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches
      document
        .getElementById("runtime-workspace")
        ?.scrollIntoView({ block: "start", behavior: prefersReducedMotion ? "auto" : "smooth" })
    }
  }, [activeView, selectedSandbox?.id])

  return (
    <AppShell
      activeView={activeView}
      apiState={apiState}
      projects={projects}
      templates={templates}
      sandboxes={sandboxes}
      selection={selection}
      onViewChange={setActiveView}
      onClearSelection={() => setSelection(null)}
    >
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

      {activeView === "sandboxes" && selectedSandbox ? (
        <RuntimeWorkspace sandbox={selectedSandbox} onSandboxChange={refreshSandbox} />
      ) : null}

      <div className="resource-grid">
        {activeView === "projects" ? (
          <ProjectTable
            projects={projects}
            templates={templates}
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
          />
        ) : null}
        {activeView === "sandboxes" ? (
          <SandboxTable
            projects={projects}
            templates={templates}
            sandboxes={sandboxes}
            loading={loading}
            error={error}
            selection={selection}
            onSelect={(id) => setSelection({ kind: "sandbox", id })}
            onCreate={createSandbox}
            onDelete={deleteSandbox}
            onStart={startSandbox}
            onStop={stopSandbox}
          />
        ) : null}
      </div>
    </AppShell>
  )
}

import type { ReactNode } from "react"
import { Toaster } from "@/components/ui/sonner"
import { DetailPane } from "@/components/console/detail-pane"
import { Rail } from "@/components/console/rail"
import type { APIStatus, Project, Sandbox, Selection, Template, WorkspaceView } from "@/types"

export function AppShell({
  activeView,
  apiState,
  children,
  onViewChange,
  projects,
  sandboxes,
  selection,
  templates,
  onClearSelection,
}: {
  activeView: WorkspaceView
  apiState: APIStatus
  children: ReactNode
  onViewChange: (view: WorkspaceView) => void
  projects: Project[]
  sandboxes: Sandbox[]
  selection: Selection | null
  templates: Template[]
  onClearSelection: () => void
}) {
  return (
    <>
      <div className="shell">
        <Rail activeView={activeView} apiState={apiState} onViewChange={onViewChange} />
        <main className="workspace">{children}</main>
        <DetailPane
          selection={selection}
          projects={projects}
          templates={templates}
          sandboxes={sandboxes}
          onClear={onClearSelection}
        />
      </div>
      <Toaster position="bottom-right" />
    </>
  )
}

import type { ReactNode } from "react"
import { Toaster } from "@/components/ui/sonner"
import { DetailPane } from "@/components/console/detail-pane"
import { Rail } from "@/components/console/rail"
import { cn } from "@/lib/utils"
import type { APIStatus, Project, Sandbox, Selection, Template, WorkspaceView } from "@/types"

export function AppShell({
  activeView,
  apiState,
  children,
  showDetailPane = true,
  onViewChange,
  onValidateTemplate,
  onOpenSandboxWorkspace,
  projects,
  sandboxes,
  selection,
  templates,
  onClearSelection,
}: {
  activeView: WorkspaceView
  apiState: APIStatus
  children: ReactNode
  showDetailPane?: boolean
  onViewChange: (view: WorkspaceView) => void
  onValidateTemplate?: (id: string) => Promise<void>
  onOpenSandboxWorkspace?: (id: string) => void
  projects: Project[]
  sandboxes: Sandbox[]
  selection: Selection | null
  templates: Template[]
  onClearSelection: () => void
}) {
  return (
    <>
      <div className={cn("shell", !showDetailPane && "shell-no-detail")}>
        <Rail activeView={activeView} apiState={apiState} onViewChange={onViewChange} />
        <main className="workspace">{children}</main>
        {showDetailPane ? (
          <DetailPane
            selection={selection}
            projects={projects}
            templates={templates}
            sandboxes={sandboxes}
            onValidateTemplate={onValidateTemplate}
            onOpenSandboxWorkspace={onOpenSandboxWorkspace}
            onClear={onClearSelection}
          />
        ) : null}
      </div>
      <Toaster position="bottom-right" />
    </>
  )
}

import type { ReactNode } from "react"
import { Toaster } from "@/components/ui/sonner"
import { DetailPane } from "@/components/console/detail-pane"
import { Rail } from "@/components/console/rail"
import { cn } from "@/lib/utils"
import type {
  APIStatus,
  AuditEvent,
  Project,
  ProjectCredential,
  ProjectPolicy,
  ProjectQuotaPolicy,
  ProjectUsage,
  Sandbox,
  Selection,
  Template,
  WorkspaceView,
} from "@/types"

export function AppShell({
  activeView,
  apiState,
  children,
  showDetailPane = true,
  onViewChange,
  onValidateTemplate,
  onOpenSandboxWorkspace,
  onRefreshProjectAuditEvents,
  projects,
  projectAuditEvents,
  projectCredentials,
  projectPolicies,
  projectQuotaPolicies,
  projectUsage,
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
  onRefreshProjectAuditEvents?: (
    projectID: string,
    filters?: {
      action?: string
      actor?: string
      source?: string
      requestId?: string
      operation?: string
      since?: string
      until?: string
    },
  ) => Promise<AuditEvent[]>
  projects: Project[]
  projectAuditEvents: Record<string, AuditEvent[]>
  projectCredentials: Record<string, ProjectCredential[]>
  projectPolicies: Record<string, ProjectPolicy>
  projectQuotaPolicies: Record<string, ProjectQuotaPolicy>
  projectUsage: Record<string, ProjectUsage>
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
            projectAuditEvents={projectAuditEvents}
            projectCredentials={projectCredentials}
            projectPolicies={projectPolicies}
            projectQuotaPolicies={projectQuotaPolicies}
            projectUsage={projectUsage}
            templates={templates}
            sandboxes={sandboxes}
            onValidateTemplate={onValidateTemplate}
            onOpenSandboxWorkspace={onOpenSandboxWorkspace}
            onRefreshProjectAuditEvents={onRefreshProjectAuditEvents}
            onClear={onClearSelection}
          />
        ) : null}
      </div>
      <Toaster position="bottom-right" />
    </>
  )
}

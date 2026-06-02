import { Boxes, FolderKanban, Layers3, ServerCog } from "lucide-react"
import { cn } from "@/lib/utils"
import type { APIStatus, CallerInfo, WorkspaceView } from "@/types"

const navItems: Array<{
  id: WorkspaceView
  label: string
  icon: typeof FolderKanban
}> = [
  { id: "projects", label: "Projects", icon: FolderKanban },
  { id: "templates", label: "Environments", icon: Layers3 },
  { id: "sandboxes", label: "Sandboxes", icon: Boxes },
  { id: "runtime", label: "Runtime", icon: ServerCog },
]

export function Rail({
  activeView,
  apiState,
  callerInfo,
  onViewChange,
}: {
  activeView: WorkspaceView
  apiState: APIStatus
  callerInfo: CallerInfo | null
  onViewChange: (view: WorkspaceView) => void
}) {
  return (
    <aside className="rail" aria-label="Main navigation">
      <div className="brand">
        <span className="brand-mark" aria-hidden="true">
          <span className="brand-mark-grid" />
        </span>
        <div className="brand-copy">
          <strong>mbox</strong>
          <span>control plane</span>
        </div>
      </div>
      <nav className="nav">
        {navItems.map((item) => {
          const Icon = item.icon
          const selected = activeView === item.id || (activeView === "sandbox-detail" && item.id === "sandboxes")
          return (
            <button
              key={item.id}
              type="button"
              className={cn(selected && "is-active")}
              aria-current={selected ? "page" : undefined}
              onClick={() => onViewChange(item.id)}
            >
              <Icon />
              {item.label}
            </button>
          )
        })}
      </nav>
      <div className="rail-status">
        <span
          className={cn(
            "status-dot",
            apiState.state === "ok" && "ok",
            apiState.state === "bad" && "bad",
          )}
        />
        <span>{apiState.label}</span>
      </div>
      <div className="rail-auth" aria-label="Caller boundary">
        <span>Caller</span>
        <strong>{callerLabel(callerInfo)}</strong>
        <small>{callerTrustLabel(callerInfo)}</small>
      </div>
    </aside>
  )
}

function callerLabel(callerInfo: CallerInfo | null) {
  if (!callerInfo) {
    return "Unknown"
  }
  if (callerInfo.mode === "trusted_header") {
    return callerInfo.principal
  }
  if (callerInfo.mode === "shared_token") {
    return "Shared token"
  }
  return "Anonymous"
}

function callerTrustLabel(callerInfo: CallerInfo | null) {
  if (!callerInfo) {
    return "No caller handshake"
  }
  if (callerInfo.rbacTrusted && callerInfo.projectRolesEnforced) {
    return "Project RBAC enforced"
  }
  if (callerInfo.projectRolesEnforced) {
    return "Enforced, caller untrusted"
  }
  if (callerInfo.rbacTrusted) {
    return "Trusted preflight only"
  }
  return "RBAC not enforced"
}

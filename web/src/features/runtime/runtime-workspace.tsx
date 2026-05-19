import { useEffect, useState } from "react"
import { RefreshCw } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  getPreviewPorts,
  getRuntimeEvents,
  getRuntimeLogs,
  getRuntimeTarget,
} from "@/lib/api"
import { storageSummary } from "@/lib/resource-utils"
import { cn } from "@/lib/utils"
import { PreviewPorts } from "@/features/runtime/preview-ports"
import { RuntimeEvents, RuntimeLogs } from "@/features/runtime/runtime-observe"
import { RuntimeStoragePanel } from "@/features/runtime/runtime-storage-panel"
import { TerminalPane } from "@/features/runtime/terminal-pane"
import type {
  PreviewPort,
  RuntimeEvent,
  RuntimeTab,
  RuntimeTarget,
  Sandbox,
} from "@/types"

const runtimeTabs: Array<{ id: RuntimeTab; label: string }> = [
  { id: "terminal", label: "Terminal" },
  { id: "storage", label: "Storage" },
  { id: "preview", label: "Preview" },
  { id: "logs", label: "Logs" },
  { id: "events", label: "Events" },
]

export function RuntimeWorkspace({ sandbox }: { sandbox: Sandbox }) {
  const [activeTab, setActiveTab] = useState<RuntimeTab>("terminal")
  const [target, setTarget] = useState<RuntimeTarget | null>(null)
  const [logs, setLogs] = useState("")
  const [events, setEvents] = useState<RuntimeEvent[]>([])
  const [ports, setPorts] = useState<PreviewPort[]>([])
  const [runtimeError, setRuntimeError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const terminalDisabledReason = !sandbox.runtimeRef
    ? "Runtime has not been projected yet"
    : sandbox.status !== "running"
      ? `Sandbox is ${sandbox.status || "not running"}`
      : undefined

  async function loadRuntime() {
    if (!sandbox.runtimeRef) {
      setRuntimeError("Runtime is not ready")
      setTarget(null)
      setLogs("")
      setEvents([])
      setPorts([])
      return
    }
    setLoading(true)
    setRuntimeError(null)
    try {
      const [runtimeTarget, logResult, eventResult, portResult] = await Promise.all([
        getRuntimeTarget(sandbox.id),
        getRuntimeLogs(sandbox.id),
        getRuntimeEvents(sandbox.id),
        getPreviewPorts(sandbox.id),
      ])
      setTarget(runtimeTarget)
      setLogs(logResult.logs)
      setEvents(eventResult.items || [])
      setPorts(portResult.items || [])
    } catch (requestError) {
      const message = requestError instanceof Error ? requestError.message : "Runtime request failed"
      setRuntimeError(message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    setActiveTab("terminal")
    void loadRuntime()
  }, [sandbox.id, sandbox.runtimeRef?.name])

  return (
    <section id="runtime-workspace" className="runtime-workspace" aria-label="Sandbox runtime workspace">
      <div className="runtime-workspace-head">
        <div>
          <p className="eyebrow">Runtime workspace</p>
          <h2>{sandbox.name}</h2>
        </div>
        <Button variant="outline" size="sm" onClick={() => void loadRuntime()} disabled={loading}>
          <RefreshCw data-icon="inline-start" />
          {loading ? "Loading..." : "Refresh"}
        </Button>
      </div>
      {runtimeError ? <p className="runtime-error">{runtimeError}</p> : null}
      {terminalDisabledReason ? (
        <p className="runtime-notice" role="status">
          Terminal will be available when the runtime target is running. Current blocker: {terminalDisabledReason}.
        </p>
      ) : null}
      <div className="runtime-target-strip">
        <div>
          <span>Runtime</span>
          <strong>{sandbox.runtimeRef ? `${sandbox.runtimeRef.kind} ${sandbox.runtimeRef.namespace}/${sandbox.runtimeRef.name}` : "Pending"}</strong>
        </div>
        <div>
          <span>Pod</span>
          <strong>{target ? `${target.namespace}/${target.podName}` : "Pending"}</strong>
        </div>
        <div>
          <span>Container</span>
          <strong>{target ? `${target.container} · ${target.phase || "unknown"}` : "No target"}</strong>
        </div>
        <div>
          <span>Workspace</span>
          <strong>{storageSummary(target?.storage)}</strong>
        </div>
      </div>
      <div className="runtime-tabs" role="tablist" aria-label="Runtime views">
        {runtimeTabs.map((tab) => (
          <button
            key={tab.id}
            type="button"
            role="tab"
            aria-selected={activeTab === tab.id}
            className={cn(activeTab === tab.id && "is-active")}
            onClick={() => setActiveTab(tab.id)}
          >
            {tab.label}
          </button>
        ))}
      </div>
      <div className="runtime-tab-panel">
        {activeTab === "terminal" ? (
          <TerminalPane sandbox={sandbox} disabled={Boolean(terminalDisabledReason)} disabledReason={terminalDisabledReason} />
        ) : null}
        {activeTab === "storage" ? <RuntimeStoragePanel storage={target?.storage || []} /> : null}
        {activeTab === "preview" ? <PreviewPorts ports={ports} /> : null}
        {activeTab === "logs" ? <RuntimeLogs logs={logs} /> : null}
        {activeTab === "events" ? <RuntimeEvents events={events} /> : null}
      </div>
    </section>
  )
}

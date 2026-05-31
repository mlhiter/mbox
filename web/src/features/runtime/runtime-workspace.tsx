import { useEffect, useState } from "react"
import { Box, Cable, CheckCircle2, Clock3, RefreshCw, XCircle } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  cancelExecutionTask,
  getPreviewPorts,
  getRuntimeEvents,
  getRuntimeLogs,
  getSandboxBoundary,
  getRuntimeTarget,
  listArtifacts,
  listExecutionTasks,
  listRuntimeSessions,
  updateSandboxPorts,
  watchExecutionTask,
} from "@/lib/api"
import { storageSummary } from "@/lib/resource-utils"
import { cn } from "@/lib/utils"
import { PreviewPorts } from "@/features/runtime/preview-ports"
import { RuntimeBoundary } from "@/features/runtime/runtime-boundary"
import { RuntimeArtifacts } from "@/features/runtime/runtime-artifacts"
import { RuntimeEvents, RuntimeLogs } from "@/features/runtime/runtime-observe"
import { RuntimeSessions } from "@/features/runtime/runtime-sessions"
import { RuntimeStoragePanel } from "@/features/runtime/runtime-storage-panel"
import { RuntimeTasks } from "@/features/runtime/runtime-tasks"
import { TerminalPane } from "@/features/runtime/terminal-pane"
import type {
  Artifact,
  BoundarySummary,
  ExecutionTask,
  PreviewPort,
  RuntimeEvent,
  RuntimeSession,
  RuntimeTab,
  RuntimeTarget,
  Sandbox,
} from "@/types"

const runtimeTabs: Array<{ id: RuntimeTab; label: string }> = [
  { id: "terminal", label: "Terminal" },
  { id: "sessions", label: "Sessions" },
  { id: "boundary", label: "Boundary" },
  { id: "preview", label: "Preview" },
  { id: "tasks", label: "Tasks" },
  { id: "artifacts", label: "Artifacts" },
  { id: "logs", label: "Logs" },
  { id: "events", label: "Events" },
]

function previewPortsFromSandbox(sandbox: Sandbox, message: string): PreviewPort[] {
  return (sandbox.ports || []).map((port) => ({
    name: port.name,
    port: port.port,
    protocol: port.protocol || "TCP",
    available: false,
    message,
  }))
}

function RuntimePendingPanel({
  sandbox,
  reason,
}: {
  sandbox: Sandbox
  reason?: string
}) {
  return (
    <div className="runtime-pending-panel">
      <strong>{sandbox.status === "running" && sandbox.runtimeRef ? "Runtime access unavailable" : "Starting runtime"}</strong>
      <span>{reason || "Runtime is starting"}</span>
      <small>
        Current state: {sandbox.status || "pending"}
        {sandbox.runtimeRef ? ` · ${sandbox.runtimeRef.namespace}/${sandbox.runtimeRef.name}` : ""}
      </small>
    </div>
  )
}

function mergeTask(current: ExecutionTask[], task: ExecutionTask) {
  return [task, ...current.filter((item) => item.id !== task.id)]
}

function applyTaskOutput(current: ExecutionTask[], taskID: string, stream: "stdout" | "stderr", data: string) {
  return current.map((task) => {
    if (task.id !== taskID) {
      return task
    }
    if (stream === "stdout") {
      return { ...task, stdout: `${task.stdout || ""}${data}` }
    }
    return { ...task, stderr: `${task.stderr || ""}${data}` }
  })
}

function applyTaskOutputEvent(
  current: ExecutionTask[],
  taskID: string,
  stream: "stdout" | "stderr",
  data: string,
  offset?: number,
) {
  return current.map((task) => {
    if (task.id !== taskID) {
      return task
    }
    const currentOutput = stream === "stdout" ? task.stdout || "" : task.stderr || ""
    if (typeof offset === "number" && offset < currentOutput.length) {
      const missing = data.slice(currentOutput.length - offset)
      if (!missing) {
        return task
      }
      return applyTaskOutput([task], taskID, stream, missing)[0]
    }
    return applyTaskOutput([task], taskID, stream, data)[0]
  })
}

export function RuntimeWorkspace({
  sandbox,
  onSandboxChange,
}: {
  sandbox: Sandbox
  onSandboxChange: (id: string) => Promise<Sandbox>
}) {
  const [activeTab, setActiveTab] = useState<RuntimeTab>("terminal")
  const [target, setTarget] = useState<RuntimeTarget | null>(null)
  const [logs, setLogs] = useState("")
  const [events, setEvents] = useState<RuntimeEvent[]>([])
  const [ports, setPorts] = useState<PreviewPort[]>([])
  const [sessions, setSessions] = useState<RuntimeSession[]>([])
  const [boundary, setBoundary] = useState<BoundarySummary | null>(null)
  const [tasks, setTasks] = useState<ExecutionTask[]>([])
  const [artifacts, setArtifacts] = useState<Artifact[]>([])
  const [runtimeError, setRuntimeError] = useState<string | null>(null)
  const [boundaryError, setBoundaryError] = useState<string | null>(null)
  const [runtimeAccessReady, setRuntimeAccessReady] = useState(false)
  const [loading, setLoading] = useState(false)
  const runtimeReady = Boolean(sandbox.runtimeRef && sandbox.status === "running")
  const runtimeStarting = sandbox.status === "pending" || !sandbox.runtimeRef
  const previewPortCount = ports.length || sandbox.ports?.length || 0
  const terminalDisabledReason = runtimeStarting
    ? "Runtime is starting"
    : runtimeError && !runtimeAccessReady
      ? runtimeError
      : sandbox.status !== "running"
      ? `Sandbox is ${sandbox.status || "not running"}`
      : undefined
  const runtimeNotice = runtimeStarting
    ? "Runtime workspace is starting. Terminal, logs, events, and preview links become active after the sandbox is running."
    : terminalDisabledReason
      ? "Runtime access is unavailable. Terminal, logs, events, and preview links become active after runtime access is configured."
      : undefined

  async function loadRuntime() {
    async function loadTasks() {
      try {
        const taskResult = await listExecutionTasks(sandbox.id)
        setTasks(taskResult.items || [])
      } catch (requestError) {
        const message = requestError instanceof Error ? requestError.message : "Task history request failed"
        setRuntimeError((current) => current || message)
      }
    }
    async function loadSessions() {
      try {
        const sessionResult = await listRuntimeSessions(sandbox.id)
        setSessions(sessionResult.items || [])
      } catch (requestError) {
        const message = requestError instanceof Error ? requestError.message : "Session history request failed"
        setRuntimeError((current) => current || message)
      }
    }
    async function loadArtifacts() {
      try {
        const artifactResult = await listArtifacts(sandbox.id)
        setArtifacts(artifactResult.items || [])
      } catch (requestError) {
        const message = requestError instanceof Error ? requestError.message : "Artifact request failed"
        setRuntimeError((current) => current || message)
      }
    }
    async function loadBoundary() {
      try {
        const boundaryResult = await getSandboxBoundary(sandbox.id)
        setBoundary(boundaryResult)
        setBoundaryError(null)
      } catch (requestError) {
        const message = requestError instanceof Error ? requestError.message : "Boundary request failed"
        setBoundaryError(message)
      }
    }

    if (!runtimeReady) {
      setLoading(false)
      setRuntimeError(null)
      setTarget(null)
      setLogs("")
      setEvents([])
      setRuntimeAccessReady(false)
      setPorts(previewPortsFromSandbox(sandbox, runtimeStarting ? "sandbox is starting" : "sandbox must be running before preview is available"))
      await loadSessions()
      await loadBoundary()
      await loadTasks()
      await loadArtifacts()
      return
    }
    setLoading(true)
    setRuntimeError(null)
    setRuntimeAccessReady(false)
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
      setRuntimeAccessReady(true)
    } catch (requestError) {
      const message = requestError instanceof Error ? requestError.message : "Runtime request failed"
      setRuntimeAccessReady(false)
      setRuntimeError(message)
      setPorts(previewPortsFromSandbox(sandbox, message))
    }
    await loadSessions()
    await loadBoundary()
    await loadTasks()
    await loadArtifacts()
    setLoading(false)
  }

  async function savePreviewPorts(ports: Sandbox["ports"]) {
    await updateSandboxPorts(sandbox.id, ports || [])
    const updated = await onSandboxChange(sandbox.id)
    if (updated.runtimeRef && updated.status === "running") {
      const portResult = await getPreviewPorts(sandbox.id)
      setPorts(portResult.items || [])
    } else {
      setPorts(previewPortsFromSandbox(updated, "sandbox must be running before preview is available"))
    }
    return updated
  }

  function addTask(task: ExecutionTask) {
    setTasks((current) => mergeTask(current, task))
  }

  function addArtifact(artifact: Artifact) {
    setArtifacts((current) => [artifact, ...current.filter((item) => item.id !== artifact.id)])
  }

  async function cancelTask(taskID: string) {
    const task = await cancelExecutionTask(taskID)
    addTask(task)
    return task
  }

  useEffect(() => {
    setActiveTab("terminal")
    void loadRuntime()
  }, [sandbox.id, sandbox.runtimeRef?.name, sandbox.status])

  useEffect(() => {
    if (runtimeReady || sandbox.status === "failed" || sandbox.status === "deleted" || sandbox.status === "stopped") {
      return
    }
    let cancelled = false
    const timer = window.setInterval(() => {
      void onSandboxChange(sandbox.id).catch(() => {
        if (!cancelled) {
          setRuntimeError("Could not refresh sandbox status")
        }
      })
    }, 2500)
    return () => {
      cancelled = true
      window.clearInterval(timer)
    }
  }, [onSandboxChange, runtimeReady, sandbox.id, sandbox.status])

  useEffect(() => {
    if (!tasks.some((task) => task.status === "queued" || task.status === "running")) {
      return
    }
    const timer = window.setInterval(() => {
      void listExecutionTasks(sandbox.id)
        .then((taskResult) => setTasks(taskResult.items || []))
        .catch((requestError) => {
          const message = requestError instanceof Error ? requestError.message : "Task history request failed"
          setRuntimeError((current) => current || message)
        })
    }, 1500)
    return () => {
      window.clearInterval(timer)
    }
  }, [sandbox.id, tasks])

  useEffect(() => {
    const activeTasks = tasks.filter((task) => task.status === "queued" || task.status === "running")
    if (!activeTasks.length) {
      return
    }
    const controllers = activeTasks.map((task) => {
      const controller = new AbortController()
      void watchExecutionTask(task.id, {
        signal: controller.signal,
        onEvent: (event) => {
          if (event.type === "output") {
            setTasks((current) => applyTaskOutputEvent(current, task.id, event.stream, event.data, event.offset))
            return
          }
          if (event.task) {
            setTasks((current) => mergeTask(current, event.task))
          }
        },
      }).catch((requestError) => {
        if (!controller.signal.aborted) {
          const message = requestError instanceof Error ? requestError.message : "Task watch request failed"
          setRuntimeError((current) => current || message)
        }
      })
      return controller
    })
    return () => {
      controllers.forEach((controller) => controller.abort())
    }
  }, [tasks.map((task) => `${task.id}:${task.status}`).join("|")])

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
      {runtimeNotice ? (
        <p className="runtime-notice" role="status">
          {runtimeNotice}
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
      <div className="runtime-output-summary" aria-label="Runtime output summary">
        <div>
          <Cable aria-hidden="true" />
          <span>Preview ports</span>
          <strong>{previewPortCount}</strong>
        </div>
        <div>
          <Clock3 aria-hidden="true" />
          <span>Active tasks</span>
          <strong>{tasks.filter((task) => task.status === "queued" || task.status === "running").length}</strong>
        </div>
        <div>
          <CheckCircle2 aria-hidden="true" />
          <span>Succeeded</span>
          <strong>{tasks.filter((task) => task.status === "succeeded").length}</strong>
        </div>
        <div>
          <XCircle aria-hidden="true" />
          <span>Needs review</span>
          <strong>{tasks.filter((task) => task.status === "failed" || task.status === "timed_out" || task.status === "canceled").length}</strong>
        </div>
        <div>
          <Box aria-hidden="true" />
          <span>Artifacts</span>
          <strong>{artifacts.length}</strong>
        </div>
      </div>
      <RuntimeStoragePanel storage={target?.storage || []} compact />
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
          runtimeReady && runtimeAccessReady ? (
            <TerminalPane sandbox={sandbox} disabled={false} onSessionChange={loadRuntime} />
          ) : (
            <RuntimePendingPanel sandbox={sandbox} reason={terminalDisabledReason} />
          )
        ) : null}
        {activeTab === "sessions" ? (
          <RuntimeSessions sessions={sessions} loading={loading} onRefresh={loadRuntime} />
        ) : null}
        {activeTab === "boundary" ? (
          <RuntimeBoundary boundary={boundary} loading={loading} error={boundaryError} onRefresh={loadRuntime} />
        ) : null}
        {activeTab === "preview" ? (
          <PreviewPorts
            ports={ports}
            sandboxPorts={sandbox.ports || []}
            sandboxStatus={sandbox.status}
            onSave={savePreviewPorts}
          />
        ) : null}
        {activeTab === "tasks" ? (
          <RuntimeTasks
            sandbox={sandbox}
            tasks={tasks}
            runtimeReady={runtimeReady && runtimeAccessReady}
            loading={loading}
            onRefresh={loadRuntime}
            onTaskCreated={addTask}
            onTaskCancel={cancelTask}
          />
        ) : null}
        {activeTab === "artifacts" ? (
          <RuntimeArtifacts
            sandbox={sandbox}
            artifacts={artifacts}
            tasks={tasks}
            loading={loading}
            onRefresh={loadRuntime}
            onArtifactCreated={addArtifact}
          />
        ) : null}
        {activeTab === "logs" ? <RuntimeLogs logs={logs} /> : null}
        {activeTab === "events" ? <RuntimeEvents events={events} /> : null}
      </div>
    </section>
  )
}

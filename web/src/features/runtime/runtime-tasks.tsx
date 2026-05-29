import { FormEvent, useMemo, useState } from "react"
import { OctagonX, Play, RefreshCw } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { createExecutionTask } from "@/lib/api"
import {
  formatClockTime,
  formatDuration,
  formatTaskCommand,
  isActiveTask,
  shortID,
  taskStatusLabel,
  taskStatusTone,
} from "@/lib/resource-utils"
import { RuntimeSectionHead } from "@/features/runtime/runtime-section-head"
import type { ExecutionTask, Sandbox } from "@/types"

type RuntimeTasksProps = {
  sandbox: Sandbox
  tasks: ExecutionTask[]
  runtimeReady: boolean
  loading: boolean
  onRefresh: () => Promise<void>
  onTaskCreated: (task: ExecutionTask) => void
  onTaskCancel: (taskID: string) => Promise<ExecutionTask>
}

export function RuntimeTasks({
  sandbox,
  tasks,
  runtimeReady,
  loading,
  onRefresh,
  onTaskCreated,
  onTaskCancel,
}: RuntimeTasksProps) {
  const [command, setCommand] = useState("pwd")
  const [commandMode, setCommandMode] = useState<"shell" | "array">("shell")
  const [timeoutSeconds, setTimeoutSeconds] = useState("60")
  const [submitting, setSubmitting] = useState(false)
  const [cancelingTaskID, setCancelingTaskID] = useState<string | null>(null)
  const [formError, setFormError] = useState<string | null>(null)
  const sortedTasks = useMemo(
    () => [...tasks].sort((left, right) => (right.createdAt || "").localeCompare(left.createdAt || "")),
    [tasks],
  )
  const taskSummary = useMemo(
    () => ({
      total: tasks.length,
      active: tasks.filter(isActiveTask).length,
      succeeded: tasks.filter((task) => task.status === "succeeded").length,
      needsReview: tasks.filter((task) => task.status === "failed" || task.status === "timed_out" || task.status === "canceled").length,
    }),
    [tasks],
  )

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const timeout = Number(timeoutSeconds)
    const parsedCommand = parseCommand(command, commandMode)
    if (!parsedCommand.ok) {
      setFormError(parsedCommand.error)
      return
    }
    if (!Number.isInteger(timeout) || timeout < 1 || timeout > 600) {
      setFormError("Timeout must be between 1 and 600 seconds.")
      return
    }
    setSubmitting(true)
    setFormError(null)
    try {
      const task = await createExecutionTask(sandbox.id, {
        command: parsedCommand.command,
        timeoutSeconds: timeout,
      })
      onTaskCreated(task)
    } catch (error) {
      setFormError(error instanceof Error ? error.message : "Could not run task.")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="runtime-tasks">
      <RuntimeSectionHead eyebrow="Execution" title="Tasks" />
      <div className="runtime-ledger-head runtime-task-ledger-head" aria-label="Task summary">
        <div>
          <span>Total</span>
          <strong>{taskSummary.total}</strong>
        </div>
        <div>
          <span>Active</span>
          <strong>{taskSummary.active}</strong>
        </div>
        <div>
          <span>Succeeded</span>
          <strong>{taskSummary.succeeded}</strong>
        </div>
        <div>
          <span>Needs review</span>
          <strong>{taskSummary.needsReview}</strong>
        </div>
      </div>
      <form className="runtime-task-form" onSubmit={handleSubmit}>
        <div>
          <Label htmlFor="runtime-task-command">Command</Label>
          <Input
            id="runtime-task-command"
            value={command}
            placeholder={commandMode === "shell" ? "npm test" : "[\"npm\", \"test\"]"}
            onChange={(event) => setCommand(event.target.value)}
            disabled={submitting || !runtimeReady}
          />
        </div>
        <div>
          <Label htmlFor="runtime-task-mode">Mode</Label>
          <select
            id="runtime-task-mode"
            value={commandMode}
            onChange={(event) => setCommandMode(event.target.value as "shell" | "array")}
            disabled={submitting || !runtimeReady}
          >
            <option value="shell">Shell</option>
            <option value="array">Array</option>
          </select>
        </div>
        <div>
          <Label htmlFor="runtime-task-timeout">Timeout</Label>
          <Input
            id="runtime-task-timeout"
            inputMode="numeric"
            pattern="[0-9]*"
            value={timeoutSeconds}
            onChange={(event) => setTimeoutSeconds(event.target.value)}
            disabled={submitting || !runtimeReady}
          />
        </div>
        <Button type="submit" disabled={submitting || !runtimeReady}>
          <Play data-icon="inline-start" />
          {submitting ? "Queued..." : "Run"}
        </Button>
        <Button type="button" variant="outline" disabled={loading} onClick={() => void onRefresh()}>
          <RefreshCw data-icon="inline-start" />
          Refresh
        </Button>
      </form>
      {!runtimeReady ? <p className="runtime-task-note">Tasks run after the sandbox runtime and runtime access are ready.</p> : null}
      {formError ? <p className="runtime-task-error">{formError}</p> : null}
      {sortedTasks.length === 0 ? (
        <p>No tasks have run in this sandbox.</p>
      ) : (
        <ul>
          {sortedTasks.map((task) => (
            <li key={task.id}>
              <div className="runtime-task-row-head">
                <div>
                  <code className="runtime-task-command">{formatTaskCommand(task.command)}</code>
                  <span className="runtime-task-meta">
                    {shortID(task.id)} · {formatClockTime(task.createdAt)} · {formatDuration(task.startedAt || task.createdAt, task.finishedAt)}
                    {typeof task.exitCode === "number" ? ` · exit ${task.exitCode}` : ""}
                    {task.outputTruncated ? " · truncated" : ""}
                  </span>
                </div>
                <div className="runtime-task-row-actions">
                  <span className={`runtime-status-badge tone-${taskStatusTone(task.status)}`}>
                    {taskStatusLabel(task.status)}
                  </span>
                  {isActiveTask(task) ? (
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      disabled={cancelingTaskID === task.id}
                      onClick={() => void handleCancel(task.id)}
                    >
                      <OctagonX data-icon="inline-start" />
                      {cancelingTaskID === task.id ? "Canceling" : "Cancel"}
                    </Button>
                  ) : null}
                  <time>{formatClockTime(task.finishedAt || task.updatedAt || task.createdAt)}</time>
                </div>
              </div>
              {task.error ? <small>{task.error}</small> : null}
              <div className="runtime-task-output-grid">
                <OutputBlock label="stdout" value={task.stdout} />
                <OutputBlock label="stderr" value={task.stderr} tone="danger" />
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  )

  async function handleCancel(taskID: string) {
    setCancelingTaskID(taskID)
    setFormError(null)
    try {
      await onTaskCancel(taskID)
      await onRefresh()
    } catch (error) {
      setFormError(error instanceof Error ? error.message : "Could not cancel task.")
    } finally {
      setCancelingTaskID(null)
    }
  }
}

function OutputBlock({ label, value, tone }: { label: string; value?: string; tone?: "danger" }) {
  return (
    <div className={`runtime-output-block ${tone === "danger" ? "tone-danger" : ""}`}>
      <span>{label}</span>
      {value ? <pre>{value}</pre> : <small>No output</small>}
    </div>
  )
}

function parseCommand(command: string, mode: "shell" | "array"): { ok: true; command: string[] } | { ok: false; error: string } {
  const trimmed = command.trim()
  if (!trimmed) {
    return { ok: false, error: "Command is required." }
  }
  if (mode === "shell") {
    return { ok: true, command: ["sh", "-lc", trimmed] }
  }
  try {
    const parsed = JSON.parse(trimmed)
    if (!Array.isArray(parsed) || !parsed.every((item) => typeof item === "string") || parsed.length === 0) {
      return { ok: false, error: "Array mode expects a JSON string array." }
    }
    return { ok: true, command: parsed }
  } catch {
    return { ok: false, error: "Array mode expects valid JSON, for example [\"npm\", \"test\"]." }
  }
}

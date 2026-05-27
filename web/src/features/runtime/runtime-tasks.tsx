import { FormEvent, useMemo, useState } from "react"
import { OctagonX, Play, RefreshCw } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { createExecutionTask } from "@/lib/api"
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
  const [timeoutSeconds, setTimeoutSeconds] = useState("60")
  const [submitting, setSubmitting] = useState(false)
  const [cancelingTaskID, setCancelingTaskID] = useState<string | null>(null)
  const [formError, setFormError] = useState<string | null>(null)
  const sortedTasks = useMemo(
    () => [...tasks].sort((left, right) => (right.createdAt || "").localeCompare(left.createdAt || "")),
    [tasks],
  )

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const parsedCommand = parseCommand(command)
    const timeout = Number(timeoutSeconds)
    if (!parsedCommand.length) {
      setFormError("Command is required.")
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
        command: parsedCommand,
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
      <form className="runtime-task-form" onSubmit={handleSubmit}>
        <div>
          <Label htmlFor="runtime-task-command">Command</Label>
          <Input
            id="runtime-task-command"
            value={command}
            placeholder="npm test"
            onChange={(event) => setCommand(event.target.value)}
            disabled={submitting || !runtimeReady}
          />
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
                  <strong>{task.command.join(" ")}</strong>
                  <span>
                    {task.status}
                    {typeof task.exitCode === "number" ? ` · exit ${task.exitCode}` : ""}
                    {task.outputTruncated ? " · truncated" : ""}
                  </span>
                </div>
                <div className="runtime-task-row-actions">
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
                  <time>{formatTaskTime(task.finishedAt || task.createdAt)}</time>
                </div>
              </div>
              {task.error ? <small>{task.error}</small> : null}
              {task.stdout || task.stderr ? (
                <pre>
                  {[task.stdout, task.stderr].filter(Boolean).join("\n")}
                </pre>
              ) : null}
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

function parseCommand(command: string) {
  return command.trim().split(/\s+/).filter(Boolean)
}

function formatTaskTime(value?: string) {
  if (!value) {
    return "-"
  }
  return new Intl.DateTimeFormat(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).format(new Date(value))
}

function isActiveTask(task: ExecutionTask) {
  return task.status === "queued" || task.status === "running"
}

import { FormEvent, useMemo, useState } from "react"
import { ExternalLink, Plus, RefreshCw } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { createArtifact } from "@/lib/api"
import { RuntimeSectionHead } from "@/features/runtime/runtime-section-head"
import type { Artifact, ArtifactKind, ExecutionTask, Sandbox } from "@/types"

const artifactKinds: ArtifactKind[] = ["file", "directory", "log", "report", "screenshot", "image", "link", "other"]

type RuntimeArtifactsProps = {
  sandbox: Sandbox
  artifacts: Artifact[]
  tasks: ExecutionTask[]
  loading: boolean
  onRefresh: () => Promise<void>
  onArtifactCreated: (artifact: Artifact) => void
}

export function RuntimeArtifacts({
  sandbox,
  artifacts,
  tasks,
  loading,
  onRefresh,
  onArtifactCreated,
}: RuntimeArtifactsProps) {
  const [kind, setKind] = useState<ArtifactKind>("file")
  const [name, setName] = useState("")
  const [uri, setURI] = useState("")
  const [taskID, setTaskID] = useState("")
  const [contentType, setContentType] = useState("")
  const [sizeBytes, setSizeBytes] = useState("")
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)
  const sortedArtifacts = useMemo(
    () => [...artifacts].sort((left, right) => (right.createdAt || "").localeCompare(left.createdAt || "")),
    [artifacts],
  )
  const taskOptions = useMemo(
    () => [...tasks].sort((left, right) => (right.createdAt || "").localeCompare(left.createdAt || "")),
    [tasks],
  )

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const cleanName = name.trim()
    const cleanURI = uri.trim()
    const parsedSize = sizeBytes.trim() === "" ? undefined : Number(sizeBytes)
    if (!cleanName) {
      setFormError("Name is required.")
      return
    }
    if (!cleanURI) {
      setFormError("URI is required.")
      return
    }
    if (parsedSize !== undefined && (!Number.isInteger(parsedSize) || parsedSize < 0)) {
      setFormError("Size must be a non-negative integer.")
      return
    }
    setSubmitting(true)
    setFormError(null)
    try {
      const artifact = await createArtifact(sandbox.id, {
        kind,
        name: cleanName,
        uri: cleanURI,
        taskId: taskID || undefined,
        contentType: contentType.trim() || undefined,
        sizeBytes: parsedSize,
      })
      onArtifactCreated(artifact)
      setName("")
      setURI("")
      setContentType("")
      setSizeBytes("")
    } catch (error) {
      setFormError(error instanceof Error ? error.message : "Could not register artifact.")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="runtime-artifacts">
      <RuntimeSectionHead eyebrow="Outputs" title="Artifacts" />
      <form className="runtime-artifact-form" onSubmit={handleSubmit}>
        <div>
          <Label htmlFor="runtime-artifact-kind">Kind</Label>
          <select
            id="runtime-artifact-kind"
            value={kind}
            onChange={(event) => setKind(event.target.value as ArtifactKind)}
            disabled={submitting}
          >
            {artifactKinds.map((item) => (
              <option key={item} value={item}>
                {item}
              </option>
            ))}
          </select>
        </div>
        <div>
          <Label htmlFor="runtime-artifact-name">Name</Label>
          <Input
            id="runtime-artifact-name"
            value={name}
            placeholder="Test report"
            onChange={(event) => setName(event.target.value)}
            disabled={submitting}
          />
        </div>
        <div>
          <Label htmlFor="runtime-artifact-uri">URI</Label>
          <Input
            id="runtime-artifact-uri"
            value={uri}
            placeholder="workspace:///workspace/reports/test.json"
            onChange={(event) => setURI(event.target.value)}
            disabled={submitting}
          />
        </div>
        <div>
          <Label htmlFor="runtime-artifact-task">Task</Label>
          <select id="runtime-artifact-task" value={taskID} onChange={(event) => setTaskID(event.target.value)} disabled={submitting}>
            <option value="">None</option>
            {taskOptions.map((task) => (
              <option key={task.id} value={task.id}>
                {task.command.join(" ").slice(0, 80) || task.id}
              </option>
            ))}
          </select>
        </div>
        <div>
          <Label htmlFor="runtime-artifact-content-type">Content type</Label>
          <Input
            id="runtime-artifact-content-type"
            value={contentType}
            placeholder="application/json"
            onChange={(event) => setContentType(event.target.value)}
            disabled={submitting}
          />
        </div>
        <div>
          <Label htmlFor="runtime-artifact-size">Size</Label>
          <Input
            id="runtime-artifact-size"
            inputMode="numeric"
            pattern="[0-9]*"
            value={sizeBytes}
            placeholder="bytes"
            onChange={(event) => setSizeBytes(event.target.value)}
            disabled={submitting}
          />
        </div>
        <Button type="submit" disabled={submitting}>
          <Plus data-icon="inline-start" />
          {submitting ? "Adding..." : "Add"}
        </Button>
        <Button type="button" variant="outline" disabled={loading} onClick={() => void onRefresh()}>
          <RefreshCw data-icon="inline-start" />
          Refresh
        </Button>
      </form>
      {formError ? <p className="runtime-artifact-error">{formError}</p> : null}
      {sortedArtifacts.length === 0 ? (
        <p>No artifacts have been registered for this sandbox.</p>
      ) : (
        <ul>
          {sortedArtifacts.map((artifact) => (
            <li key={artifact.id}>
              <div>
                <strong>{artifact.name}</strong>
                <span>
                  {artifact.kind}
                  {artifact.contentType ? ` · ${artifact.contentType}` : ""}
                  {typeof artifact.sizeBytes === "number" ? ` · ${formatBytes(artifact.sizeBytes)}` : ""}
                </span>
                <small>{artifact.uri}</small>
                {artifact.taskId ? <small>task {artifact.taskId}</small> : null}
              </div>
              {artifact.uri.startsWith("http://") || artifact.uri.startsWith("https://") ? (
                <a href={artifact.uri} target="_blank" rel="noreferrer" aria-label={`Open ${artifact.name}`}>
                  <ExternalLink aria-hidden="true" />
                </a>
              ) : null}
              <time>{formatArtifactTime(artifact.createdAt)}</time>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

function formatArtifactTime(value?: string) {
  if (!value) {
    return "-"
  }
  return new Intl.DateTimeFormat(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).format(new Date(value))
}

function formatBytes(value: number) {
  if (value < 1024) {
    return `${value} B`
  }
  if (value < 1024 * 1024) {
    return `${(value / 1024).toFixed(1)} KiB`
  }
  return `${(value / (1024 * 1024)).toFixed(1)} MiB`
}

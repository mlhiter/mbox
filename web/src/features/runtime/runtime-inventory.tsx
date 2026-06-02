import { RefreshCw } from "lucide-react"
import type { CSSProperties } from "react"
import { Button } from "@/components/ui/button"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { ConsolePanel } from "@/components/console/console-panel"
import { EmptyRow, SkeletonRows } from "@/components/console/table-state"
import type {
  ManagedResource,
  ManagedResourceOwner,
  Project,
  RuntimeResourceList,
} from "@/types"

const visibleLabelKeys = [
  "mbox.dev/project-id",
  "mbox.dev/sandbox-id",
  "mbox.dev/template-id",
  "app.kubernetes.io/managed-by",
]

export function RuntimeInventory({
  error,
  inventory,
  loading,
  projects,
  selectedProjectId,
  onProjectChange,
  onRefresh,
}: {
  error: string | null
  inventory: RuntimeResourceList | null
  loading: boolean
  projects: Project[]
  selectedProjectId: string
  onProjectChange: (projectId: string) => void
  onRefresh: () => Promise<RuntimeResourceList>
}) {
  const items = inventory?.items || []
  const summary = inventory?.summary
  const workload = summary?.workload
  const checkedAt = inventory?.checkedAt ? formatTimestamp(inventory.checkedAt) : "Not checked"
  const adapter = inventory?.adapter || "runtime auditor"
  const projectRollup = projectRollupLabel(summary?.byProject, projects)
  const projectAttribution = projectAttributionRows(summary?.byProject, projects)
  const selectedProject = selectedProjectId ? projects.find((project) => project.id === selectedProjectId) : undefined
  const scopeValue = selectedProjectId ? selectedProject?.name || "Unknown project" : "All projects"
  const scopeDetail = selectedProjectId ? `project/${shortID(selectedProjectId)}` : "all owner labels"
  return (
    <ConsolePanel
      id="runtime"
      eyebrow="Live runtime"
      title="Runtime inventory"
      wide
      action={
        <Button onClick={() => void onRefresh()}>
          <RefreshCw data-icon="inline-start" />
          Refresh
        </Button>
      }
    >
      <div className="runtime-inventory-filters" aria-label="Runtime inventory filters">
        <div>
          <label htmlFor="runtime-project-filter">Project</label>
          <select
            id="runtime-project-filter"
            value={selectedProjectId}
            onChange={(event) => onProjectChange(event.target.value)}
            disabled={loading}
          >
            <option value="">All projects</option>
            {projects.map((project) => (
              <option key={project.id} value={project.id}>
                {project.name}
              </option>
            ))}
          </select>
        </div>
        <span>{items.length} matched resources</span>
      </div>
      <div className="runtime-inventory-summary" aria-label="Runtime inventory summary">
        <SummaryCell label="Managed" value={String(summary?.total ?? 0)} detail="runtime resources" />
        <SummaryCell label="Scope" value={scopeValue} detail={scopeDetail} />
        <SummaryCell label="Projects" value={projectRollup.value} detail={projectRollup.detail} />
        <SummaryCell label="Adapter" value={adapter} detail="auditor source" mono />
        <SummaryCell label="Pods" value={podSummaryValue(workload)} detail={podSummaryDetail(workload)} />
        <SummaryCell label="Requests" value={requestSummaryValue(workload)} detail={storageSummaryDetail(workload)} mono />
        <SummaryCell label="Checked" value={checkedAt} detail="latest inventory" mono />
      </div>
      <div className="runtime-project-attribution" aria-label="Runtime project attribution">
        <div className="runtime-project-attribution-head">
          <div>
            <span>Project attribution</span>
            <strong>{projectRollup.value} labeled projects</strong>
          </div>
          <button
            type="button"
            className={!selectedProjectId ? "active" : undefined}
            onClick={() => onProjectChange("")}
            disabled={loading || !selectedProjectId}
          >
            All
          </button>
        </div>
        {projectAttribution.length === 0 ? (
          <p>No runtime owner project labels in this result.</p>
        ) : (
          <div className="runtime-project-attribution-list">
            {projectAttribution.map((project) => (
              <button
                type="button"
                key={project.id}
                className={selectedProjectId === project.id ? "active" : undefined}
                onClick={() => onProjectChange(project.id)}
                disabled={loading}
              >
                <span>
                  <strong>{project.name}</strong>
                  <code>{project.shortID}</code>
                </span>
                <em>{project.count}</em>
                <i style={{ "--runtime-project-share": `${project.share}%` } as CSSProperties} />
              </button>
            ))}
          </div>
        )}
      </div>
      <Table className="resource-table runtime-inventory-table">
        <TableHeader>
          <TableRow>
            <TableHead>Resource</TableHead>
            <TableHead>Namespace</TableHead>
            <TableHead>Owner</TableHead>
            <TableHead>Runtime</TableHead>
            <TableHead>Requests</TableHead>
            <TableHead>Labels</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {loading ? (
            <SkeletonRows columns={6} />
          ) : error ? (
            <EmptyRow columns={6} title="Runtime inventory unavailable" detail={error} />
          ) : items.length === 0 ? (
            <EmptyRow columns={6} title="No managed runtime resources" detail={selectedProjectId ? "No mbox-managed Kubernetes resources matched this project owner label." : "The auditor did not report mbox-managed Kubernetes resources."} />
          ) : (
            items.map((resource) => (
              <TableRow key={`${resource.adapter}:${resource.kind}:${resource.namespace}:${resource.name}`}>
                <TableCell>
                  <div className="runtime-inventory-resource">
                    <strong>{resource.kind}</strong>
                    <code>{resource.name}</code>
                    <small>{formatTimestamp(resource.createdAt)}</small>
                  </div>
                </TableCell>
                <TableCell className="mono">{resource.namespace || "-"}</TableCell>
                <TableCell>
                  <OwnerCell owner={resource.owner} />
                </TableCell>
                <TableCell>
                  <ObservationCell resource={resource} />
                </TableCell>
                <TableCell>
                  <RequestCell resource={resource} />
                </TableCell>
                <TableCell>
                  <LabelCell resource={resource} />
                </TableCell>
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>
    </ConsolePanel>
  )
}

function ObservationCell({ resource }: { resource: ManagedResource }) {
  const observation = resource.observation
  if (!observation) {
    return <span className="runtime-inventory-muted">No live observation</span>
  }
  const ready =
    observation.containersTotal && observation.containersTotal > 0
      ? `${observation.containersReady ?? 0}/${observation.containersTotal} ready`
      : ""
  const pods =
    observation.podCount && observation.podCount > 0
      ? `${observation.runningPodCount ?? 0}/${observation.podCount} running`
      : ""
  const details = [
    observation.podName ? `pod/${shortID(observation.podName)}` : "",
    ready,
    pods,
    observation.restartCount ? `${observation.restartCount} restarts` : "",
  ].filter(Boolean)
  return (
    <div className="runtime-inventory-observation">
      <strong>{observation.podPhase || observation.readyCondition || "Observed"}</strong>
      {observation.runtimeName ? <code>{observation.runtimeName}</code> : null}
      {details.length > 0 ? <span>{details.join(" · ")}</span> : null}
      {observation.message ? <small>{observation.message}</small> : null}
    </div>
  )
}

function RequestCell({ resource }: { resource: ManagedResource }) {
  const observation = resource.observation
  const values = [
    ...resourcePairs("req", observation?.requests),
    ...resourcePairs("lim", observation?.limits),
    ...storagePairs(observation?.storage),
  ]
  if (values.length === 0) {
    return <span className="runtime-inventory-muted">No observed requests</span>
  }
  return (
    <div className="runtime-inventory-labels">
      {values.map((value) => (
        <code key={value}>{value}</code>
      ))}
    </div>
  )
}

function SummaryCell({
  detail,
  label,
  mono = false,
  value,
}: {
  detail: string
  label: string
  mono?: boolean
  value: string
}) {
  return (
    <div>
      <span>{label}</span>
      <strong className={mono ? "mono" : undefined}>{value}</strong>
      <small>{detail}</small>
    </div>
  )
}

function OwnerCell({ owner }: { owner?: ManagedResourceOwner }) {
  const label = ownerLabel(owner)
  return (
    <div className="runtime-inventory-owner">
      <strong>{label.primary}</strong>
      {label.detail ? <code>{label.detail}</code> : null}
    </div>
  )
}

function LabelCell({ resource }: { resource: ManagedResource }) {
  const labels = visibleLabelKeys
    .map((key) => {
      const value = resource.labels?.[key]
      return value ? `${key}=${shortID(value)}` : ""
    })
    .filter(Boolean)
  if (labels.length === 0) {
    return <span className="runtime-inventory-muted">No mbox owner labels</span>
  }
  return (
    <div className="runtime-inventory-labels">
      {labels.map((label) => (
        <code key={label}>{label}</code>
      ))}
    </div>
  )
}

function ownerLabel(owner?: ManagedResourceOwner) {
  if (!owner) {
    return { primary: "Unlabeled", detail: "" }
  }
  if (owner.kind === "template") {
    return {
      primary: "Template",
      detail: owner.templateId ? shortID(owner.templateId) : "",
    }
  }
  if (owner.kind === "sandbox") {
    const project = owner.projectId ? `project/${shortID(owner.projectId)}` : ""
    const sandbox = owner.sandboxId ? `sandbox/${shortID(owner.sandboxId)}` : "sandbox"
    return {
      primary: "Sandbox",
      detail: project ? `${project}/${sandbox}` : sandbox,
    }
  }
  return { primary: "Unknown", detail: "" }
}

function projectRollupLabel(
  byProject: Array<{ name: string; count: number }> | undefined,
  projects: Project[],
) {
  const items = byProject || []
  if (items.length === 0) {
    return { value: "0", detail: "labeled projects" }
  }
  const total = items.reduce((sum, item) => sum + item.count, 0)
  const [top] = items
  const project = projects.find((item) => item.id === top.name)
  return {
    value: `${items.length}`,
    detail: `${total} resources · top ${project?.name || shortID(top.name)} (${top.count})`,
  }
}

function projectAttributionRows(
  byProject: Array<{ name: string; count: number }> | undefined,
  projects: Project[],
) {
  const items = byProject || []
  const total = items.reduce((sum, item) => sum + item.count, 0)
  return [...items]
    .sort((left, right) => right.count - left.count || left.name.localeCompare(right.name))
    .map((item) => {
      const project = projects.find((candidate) => candidate.id === item.name)
      const share = total > 0 ? Math.max(3, Math.round((item.count / total) * 100)) : 0
      return {
        id: item.name,
        name: project?.name || `Project ${shortID(item.name)}`,
        shortID: shortID(item.name),
        count: item.count,
        share,
      }
    })
}

function shortID(value: string) {
  if (value.length <= 12) {
    return value
  }
  return `${value.slice(0, 8)}...${value.slice(-4)}`
}

function resourcePairs(prefix: string, values?: Record<string, string>) {
  if (!values) {
    return []
  }
  return Object.entries(values)
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([key, value]) => `${prefix} ${key}=${value}`)
}

function storagePairs(storage?: Array<{ phase?: string; capacity?: string; claimName?: string }>) {
  if (!storage || storage.length === 0) {
    return []
  }
  return storage.map((item) => {
    const state = [item.phase, item.capacity].filter(Boolean).join(" ")
    return `pvc ${state || shortID(item.claimName || "workspace")}`
  })
}

function podSummaryValue(workload?: { observedPods?: number; runningPods?: number }) {
  const observedPods = workload?.observedPods ?? 0
  if (observedPods === 0) {
    return "0"
  }
  return `${workload?.runningPods ?? 0}/${observedPods}`
}

function podSummaryDetail(workload?: { observedPods?: number; observedResources?: number }) {
  if (!workload || workload.observedPods === 0) {
    return workload?.observedResources ? `${workload.observedResources} resources observed` : "no live pod observations"
  }
  return "running / observed pods"
}

function requestSummaryValue(workload?: { requests?: Record<string, string> }) {
  const requests = workload?.requests
  if (!requests) {
    return "none"
  }
  return [requests.cpu ? `cpu ${requests.cpu}` : "", requests.memory ? `mem ${requests.memory}` : ""]
    .filter(Boolean)
    .join(" · ") || "custom"
}

function storageSummaryDetail(workload?: { storageCapacity?: string; restartCount?: number }) {
  const bits = [
    workload?.storageCapacity ? `storage ${workload.storageCapacity}` : "",
    workload?.restartCount ? `${workload.restartCount} restarts` : "",
  ].filter(Boolean)
  return bits.length > 0 ? bits.join(" · ") : "observed workload shape"
}

function formatTimestamp(value?: string) {
  if (!value) {
    return "-"
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return date.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  })
}

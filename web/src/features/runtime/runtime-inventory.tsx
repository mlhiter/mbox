import { RefreshCw } from "lucide-react"
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
  onRefresh,
}: {
  error: string | null
  inventory: RuntimeResourceList | null
  loading: boolean
  onRefresh: () => Promise<RuntimeResourceList>
}) {
  const items = inventory?.items || []
  const summary = inventory?.summary
  const workload = summary?.workload
  const checkedAt = inventory?.checkedAt ? formatTimestamp(inventory.checkedAt) : "Not checked"
  const adapter = inventory?.adapter || "runtime auditor"
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
      <div className="runtime-inventory-summary" aria-label="Runtime inventory summary">
        <SummaryCell label="Managed" value={String(summary?.total ?? 0)} detail="runtime resources" />
        <SummaryCell label="Adapter" value={adapter} detail="auditor source" mono />
        <SummaryCell label="Pods" value={podSummaryValue(workload)} detail={podSummaryDetail(workload)} />
        <SummaryCell label="Requests" value={requestSummaryValue(workload)} detail={storageSummaryDetail(workload)} mono />
        <SummaryCell label="Checked" value={checkedAt} detail="latest inventory" mono />
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
            <EmptyRow columns={6} title="No managed runtime resources" detail="The auditor did not report mbox-managed Kubernetes resources." />
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

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
  ManagedResourceCount,
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
        <SummaryCell label="Namespaces" value={formatCounts(summary?.byNamespace)} detail="live scope" />
        <SummaryCell label="Checked" value={checkedAt} detail="latest inventory" mono />
      </div>
      <Table className="resource-table runtime-inventory-table">
        <TableHeader>
          <TableRow>
            <TableHead>Resource</TableHead>
            <TableHead>Namespace</TableHead>
            <TableHead>Owner</TableHead>
            <TableHead>Labels</TableHead>
            <TableHead>Created</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {loading ? (
            <SkeletonRows columns={5} />
          ) : error ? (
            <EmptyRow columns={5} title="Runtime inventory unavailable" detail={error} />
          ) : items.length === 0 ? (
            <EmptyRow columns={5} title="No managed runtime resources" detail="The auditor did not report mbox-managed Kubernetes resources." />
          ) : (
            items.map((resource) => (
              <TableRow key={`${resource.adapter}:${resource.kind}:${resource.namespace}:${resource.name}`}>
                <TableCell>
                  <div className="runtime-inventory-resource">
                    <strong>{resource.kind}</strong>
                    <code>{resource.name}</code>
                  </div>
                </TableCell>
                <TableCell className="mono">{resource.namespace || "-"}</TableCell>
                <TableCell>
                  <OwnerCell owner={resource.owner} />
                </TableCell>
                <TableCell>
                  <LabelCell resource={resource} />
                </TableCell>
                <TableCell className="mono">{formatTimestamp(resource.createdAt)}</TableCell>
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>
    </ConsolePanel>
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

function formatCounts(counts?: ManagedResourceCount[]) {
  if (!counts || counts.length === 0) {
    return "0"
  }
  return counts
    .slice(0, 2)
    .map((item) => `${item.name} ${item.count}`)
    .join(" · ")
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

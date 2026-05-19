import { SquareTerminal } from "lucide-react"
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
import { ResourceTitleCell, RuntimeCell } from "@/components/console/resource-cells"
import { StatusBadge } from "@/components/console/status-badge"
import { EmptyRow, SkeletonRows } from "@/components/console/table-state"
import { DeleteSandboxDialog } from "@/features/resources/delete-sandbox-dialog"
import { SandboxDialog } from "@/features/resources/sandbox-dialog"
import { projectName } from "@/lib/resource-utils"
import { cn } from "@/lib/utils"
import type { FormRecord, Project, Sandbox, Selection, Template } from "@/types"

export function SandboxTable(props: {
  projects: Project[]
  templates: Template[]
  sandboxes: Sandbox[]
  loading: boolean
  error: string | null
  selection: Selection | null
  onSelect: (id: string) => void
  onCreate: (data: FormRecord) => Promise<void>
  onDelete: (id: string) => Promise<void>
}) {
  const canLaunch = props.projects.length > 0 && props.templates.length > 0

  return (
    <ConsolePanel
      id="sandboxes"
      eyebrow="Execution"
      title="Sandboxes"
      wide
      action={<SandboxDialog projects={props.projects} templates={props.templates} onSubmit={props.onCreate} />}
    >
      {!canLaunch && !props.loading ? (
        <div className="prereq-strip" role="status">
          <strong>Sandbox launch needs setup</strong>
          <span>
            {props.projects.length === 0 ? "Create a project. " : ""}
            {props.templates.length === 0 ? "Create a template. " : ""}
            Sandboxes need both before mbox can create a runtime record.
          </span>
        </div>
      ) : null}
      <Table className="sandbox-table">
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Project</TableHead>
            <TableHead>Namespace</TableHead>
            <TableHead>Runtime</TableHead>
            <TableHead />
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.loading ? (
            <SkeletonRows columns={6} />
          ) : props.error ? (
            <EmptyRow columns={6} title="Could not load sandboxes" detail="Check the API server and refresh." />
          ) : props.sandboxes.length === 0 ? (
            <EmptyRow columns={6} title="No sandboxes yet" detail="Launch one from a project and template." />
          ) : (
            props.sandboxes.map((sandbox) => (
              <TableRow
                key={sandbox.id}
                className={cn(props.selection?.kind === "sandbox" && props.selection.id === sandbox.id && "is-selected")}
                data-state={props.selection?.kind === "sandbox" && props.selection.id === sandbox.id ? "selected" : undefined}
              >
                <TableCell>
                  <ResourceTitleCell name={sandbox.name} slug={sandbox.slug} />
                </TableCell>
                <TableCell>
                  <StatusBadge status={sandbox.status} />
                </TableCell>
                <TableCell>{projectName(sandbox.projectId, props.projects)}</TableCell>
                <TableCell className="mono">{sandbox.namespace}</TableCell>
                <TableCell>
                  <RuntimeCell refValue={sandbox.runtimeRef} />
                </TableCell>
                <TableCell>
                  <div className="flex justify-end gap-2">
                    <Button variant="outline" size="sm" onClick={() => props.onSelect(sandbox.id)}>
                      <SquareTerminal data-icon="inline-start" />
                      Open workspace
                    </Button>
                    <DeleteSandboxDialog sandbox={sandbox} onDelete={props.onDelete} />
                  </div>
                </TableCell>
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>
    </ConsolePanel>
  )
}

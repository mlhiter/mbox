import { Play, Power, SquareTerminal } from "lucide-react"
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
  onStart: (id: string) => Promise<Sandbox>
  onStop: (id: string) => Promise<Sandbox>
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
                  <div className="row-actions" aria-label={`Actions for ${sandbox.name}`}>
                    <div className="row-action-group">
                      <Button
                        variant="ghost"
                        size="sm"
                        className="row-action-workspace"
                        onClick={() => props.onSelect(sandbox.id)}
                      >
                        <SquareTerminal data-icon="inline-start" />
                        Workspace
                      </Button>
                    </div>
                    <div className="row-action-group row-action-icon-group">
                      {sandbox.status === "stopped" ? (
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          className="row-action-button row-action-lifecycle"
                          aria-label={`Start ${sandbox.name}`}
                          title="Start sandbox"
                          onClick={() => void props.onStart(sandbox.id)}
                        >
                          <Play />
                        </Button>
                      ) : (
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          className="row-action-button row-action-lifecycle"
                          aria-label={`Stop ${sandbox.name}`}
                          title="Stop sandbox"
                          disabled={sandbox.status === "deleted"}
                          onClick={() => void props.onStop(sandbox.id)}
                        >
                          <Power />
                        </Button>
                      )}
                      <DeleteSandboxDialog
                        sandbox={sandbox}
                        onDelete={props.onDelete}
                        className="row-action-button row-action-danger"
                      />
                    </div>
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

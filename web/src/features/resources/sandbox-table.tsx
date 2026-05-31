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
import {
  projectName,
  sandboxPreviewPortsHint,
  sandboxPreviewPortsText,
  sandboxValidationRun,
  templateForSandbox,
  templateName,
  templateRuntimeType,
  templateUseCase,
} from "@/lib/resource-utils"
import { cn } from "@/lib/utils"
import type { FormRecord, Project, ProjectPolicy, ProjectQuotaPolicy, ProjectUsage, Sandbox, Selection, Template } from "@/types"

export function SandboxTable(props: {
  projects: Project[]
  projectPolicies: Record<string, ProjectPolicy>
  projectQuotaPolicies: Record<string, ProjectQuotaPolicy>
  projectUsage: Record<string, ProjectUsage>
  templates: Template[]
  sandboxes: Sandbox[]
  loading: boolean
  error: string | null
  selection: Selection | null
  onSelect: (id: string) => void
  onOpenWorkspace: (id: string) => void
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
      action={
        <SandboxDialog
          projects={props.projects}
          projectPolicies={props.projectPolicies}
          projectQuotaPolicies={props.projectQuotaPolicies}
          projectUsage={props.projectUsage}
          templates={props.templates}
          onSubmit={props.onCreate}
        />
      }
    >
      {!canLaunch && !props.loading ? (
        <div className="prereq-strip" role="status">
          <strong>Sandbox launch needs setup</strong>
          <span>
            {props.projects.length === 0 ? "Create a project. " : ""}
            {props.templates.length === 0 ? "Create an environment. " : ""}
            Sandboxes need both before mbox can create a runtime record.
          </span>
        </div>
      ) : null}
      <Table className="sandbox-table">
        <TableHeader>
          <TableRow>
            <TableHead>Sandbox</TableHead>
            <TableHead>State</TableHead>
            <TableHead>Environment</TableHead>
            <TableHead>Preview</TableHead>
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
            <EmptyRow columns={6} title="No sandboxes yet" detail="Launch one from a project and environment." />
          ) : (
            props.sandboxes.map((sandbox) => {
              const template = templateForSandbox(sandbox, props.templates)
              const validationRun = sandboxValidationRun(sandbox)
              return (
                <TableRow
                  key={sandbox.id}
                  className={cn(props.selection?.kind === "sandbox" && props.selection.id === sandbox.id && "is-selected")}
                  data-state={props.selection?.kind === "sandbox" && props.selection.id === sandbox.id ? "selected" : undefined}
                >
                  <TableCell>
                    <div className="sandbox-title-cell">
                      <ResourceTitleCell name={sandbox.name} slug={sandbox.slug} />
                      <span>{projectName(sandbox.projectId, props.projects)}</span>
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="sandbox-state-cell">
                      <StatusBadge status={sandbox.status} />
                      {validationRun.isValidationRun ? (
                        <span className={cn("sandbox-row-flag", validationRun.result && `sandbox-row-flag-${validationRun.result}`)}>
                          {validationRun.result
                            ? validationRun.result === "passed"
                              ? "Validation passed"
                              : "Validation failed"
                            : "Validation run"}
                        </span>
                      ) : null}
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="sandbox-environment-cell">
                      <strong>{templateName(sandbox.templateId, props.templates)}</strong>
                      <span>{template ? `${templateRuntimeType(template)} · ${templateUseCase(template)}` : "Environment unavailable"}</span>
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="sandbox-preview-cell">
                      <strong>{sandboxPreviewPortsText(sandbox)}</strong>
                      <span>{sandboxPreviewPortsHint(sandbox)}</span>
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="sandbox-runtime-cell">
                      <RuntimeCell refValue={sandbox.runtimeRef} />
                      <span className="mono" title={`${sandbox.namespace} · ${sandbox.serviceAccountName}`}>
                        {sandbox.namespace} · {sandbox.serviceAccountName}
                      </span>
                    </div>
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
                          Inspect
                        </Button>
                      </div>
                      <div className="row-action-group row-action-icon-group">
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          className="row-action-button row-action-lifecycle"
                          aria-label={`Open ${sandbox.name} workspace`}
                          title="Open workspace"
                          onClick={() => props.onOpenWorkspace(sandbox.id)}
                        >
                          <SquareTerminal />
                        </Button>
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
              )
            })
          )}
        </TableBody>
      </Table>
    </ConsolePanel>
  )
}

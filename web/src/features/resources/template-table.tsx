import { Badge } from "@/components/ui/badge"
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
import { ResourceTitleCell } from "@/components/console/resource-cells"
import { EmptyRow, SkeletonRows } from "@/components/console/table-state"
import { TemplateDialog } from "@/features/resources/template-dialog"
import {
  templateEntrypoints,
  templatePersistence,
  templateResourcePreset,
  templateRuntimeType,
  templateUseCase,
  templateValidationText,
} from "@/lib/resource-utils"
import { cn } from "@/lib/utils"
import type { FormRecord, Project, Selection, Template } from "@/types"

export function TemplateTable(props: {
  projects: Project[]
  templates: Template[]
  loading: boolean
  error: string | null
  selection: Selection | null
  onSelect: (id: string) => void
  onCreate: (data: FormRecord) => Promise<void>
  onUpdate: (id: string, data: FormRecord) => Promise<void>
}) {
  return (
    <ConsolePanel
      id="templates"
      eyebrow="Launch shape"
      title="Templates"
      action={<TemplateDialog projects={props.projects} onSubmit={props.onCreate} />}
    >
      <Table className="resource-table template-library-table">
        <TableHeader>
          <TableRow>
            <TableHead>Environment</TableHead>
            <TableHead>Use case</TableHead>
            <TableHead>Entrypoints</TableHead>
            <TableHead>Preset</TableHead>
            <TableHead>Status</TableHead>
            <TableHead />
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.loading ? (
            <SkeletonRows columns={6} />
          ) : props.error ? (
            <EmptyRow columns={6} title="Could not load templates" detail="Check the API server and refresh." />
          ) : props.templates.length === 0 ? (
            <EmptyRow columns={6} title="No templates yet" detail="Create a ready-to-run environment before launching sandboxes." />
          ) : (
            props.templates.map((template) => (
              <TableRow
                key={template.id}
                className={cn(props.selection?.kind === "template" && props.selection.id === template.id && "is-selected")}
                data-state={props.selection?.kind === "template" && props.selection.id === template.id ? "selected" : undefined}
              >
                <TableCell>
                  <div className="template-environment-cell">
                    <ResourceTitleCell name={template.name} slug={template.slug} />
                    <span className="mono">{template.image}</span>
                  </div>
                </TableCell>
                <TableCell>
                  <div className="template-use-case">
                    <Badge variant="secondary">{templateRuntimeType(template)}</Badge>
                    <span>{templateUseCase(template)}</span>
                  </div>
                </TableCell>
                <TableCell>{templateEntrypoints(template)}</TableCell>
                <TableCell>
                  <div className="template-preset-cell">
                    <strong>{templateResourcePreset(template)}</strong>
                    <span>{templatePersistence(template)}</span>
                  </div>
                </TableCell>
                <TableCell>{templateValidationText(template)}</TableCell>
                <TableCell>
                  <div className="row-actions" aria-label={`Actions for ${template.name}`}>
                    <div className="row-action-group">
                      <Button
                        variant="ghost"
                        size="sm"
                        className="row-action-workspace"
                        onClick={() => props.onSelect(template.id)}
                      >
                        Inspect
                      </Button>
                    </div>
                    <div className="row-action-group row-action-icon-group">
                      <TemplateDialog
                        projects={props.projects}
                        template={template}
                        triggerClassName="row-action-button row-action-lifecycle"
                        onSubmit={(data) => props.onUpdate(template.id, data)}
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

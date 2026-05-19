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
import { resourceText } from "@/lib/resource-utils"
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
}) {
  return (
    <ConsolePanel
      id="templates"
      eyebrow="Launch shape"
      title="Templates"
      action={<TemplateDialog projects={props.projects} onSubmit={props.onCreate} />}
    >
      <Table className="resource-table">
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Image</TableHead>
            <TableHead>Resources</TableHead>
            <TableHead />
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.loading ? (
            <SkeletonRows columns={4} />
          ) : props.error ? (
            <EmptyRow columns={4} title="Could not load templates" detail="Check the API server and refresh." />
          ) : props.templates.length === 0 ? (
            <EmptyRow columns={4} title="No templates yet" detail="Create a launch shape before starting sandboxes." />
          ) : (
            props.templates.map((template) => (
              <TableRow
                key={template.id}
                className={cn(props.selection?.kind === "template" && props.selection.id === template.id && "is-selected")}
                data-state={props.selection?.kind === "template" && props.selection.id === template.id ? "selected" : undefined}
              >
                <TableCell>
                  <ResourceTitleCell name={template.name} slug={template.slug} />
                </TableCell>
                <TableCell className="mono">{template.image}</TableCell>
                <TableCell>{resourceText(template)}</TableCell>
                <TableCell>
                  <Button variant="outline" size="sm" onClick={() => props.onSelect(template.id)}>
                    Inspect
                  </Button>
                </TableCell>
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>
    </ConsolePanel>
  )
}

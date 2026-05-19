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
import { ProjectDialog } from "@/features/resources/project-dialog"
import { templateName } from "@/lib/resource-utils"
import { cn } from "@/lib/utils"
import type { FormRecord, Project, Selection, Template } from "@/types"

export function ProjectTable(props: {
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
      id="projects"
      eyebrow="Scope"
      title="Projects"
      action={<ProjectDialog onSubmit={props.onCreate} />}
    >
      <Table className="resource-table">
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Namespace</TableHead>
            <TableHead>Default template</TableHead>
            <TableHead />
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.loading ? (
            <SkeletonRows columns={4} />
          ) : props.error ? (
            <EmptyRow columns={4} title="Could not load projects" detail="Check the API server and refresh." />
          ) : props.projects.length === 0 ? (
            <EmptyRow columns={4} title="No projects yet" detail="Create one to bind a repository and default namespace." />
          ) : (
            props.projects.map((project) => (
              <TableRow
                key={project.id}
                className={cn(props.selection?.kind === "project" && props.selection.id === project.id && "is-selected")}
                data-state={props.selection?.kind === "project" && props.selection.id === project.id ? "selected" : undefined}
              >
                <TableCell>
                  <ResourceTitleCell name={project.name} slug={project.slug} />
                </TableCell>
                <TableCell className="mono">{project.defaultNamespace}</TableCell>
                <TableCell>{templateName(project.defaultTemplateId, props.templates)}</TableCell>
                <TableCell>
                  <Button variant="outline" size="sm" onClick={() => props.onSelect(project.id)}>
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

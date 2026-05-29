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
import {
  templateName,
  templateRuntimeType,
  templateUseCase,
} from "@/lib/resource-utils"
import { cn } from "@/lib/utils"
import type { FormRecord, Project, Sandbox, Selection, Template } from "@/types"

export function ProjectTable(props: {
  projects: Project[]
  templates: Template[]
  sandboxes: Sandbox[]
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
      <Table className="resource-table project-table">
        <TableHeader>
          <TableRow>
            <TableHead>Project</TableHead>
            <TableHead>Namespace</TableHead>
            <TableHead>Default environment</TableHead>
            <TableHead>Activity</TableHead>
            <TableHead />
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.loading ? (
            <SkeletonRows columns={5} />
          ) : props.error ? (
            <EmptyRow columns={5} title="Could not load projects" detail="Check the API server and refresh." />
          ) : props.projects.length === 0 ? (
            <EmptyRow columns={5} title="No projects yet" detail="Create one to bind a repository and default namespace." />
          ) : (
            props.projects.map((project) => {
              const defaultEnvironment = props.templates.find((template) => template.id === project.defaultTemplateId)
              const projectSandboxes = props.sandboxes.filter((sandbox) => sandbox.projectId === project.id)
              const runningCount = projectSandboxes.filter((sandbox) => sandbox.status === "running").length
              return (
                <TableRow
                  key={project.id}
                  className={cn(props.selection?.kind === "project" && props.selection.id === project.id && "is-selected")}
                  data-state={props.selection?.kind === "project" && props.selection.id === project.id ? "selected" : undefined}
                >
                  <TableCell>
                    <div className="project-scope-cell">
                      <ResourceTitleCell name={project.name} slug={project.slug} />
                      <span>{project.repositoryUrl || "No repository URL recorded"}</span>
                    </div>
                  </TableCell>
                  <TableCell className="mono">{project.defaultNamespace}</TableCell>
                  <TableCell>
                    <div className="project-environment-cell">
                      <strong>{templateName(project.defaultTemplateId, props.templates)}</strong>
                      <span>
                        {defaultEnvironment
                          ? `${templateRuntimeType(defaultEnvironment)} · ${templateUseCase(defaultEnvironment)}`
                          : "No default environment"}
                      </span>
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="project-activity-cell">
                      <strong>{projectSandboxes.length} sandboxes</strong>
                      <span>{runningCount} running</span>
                    </div>
                  </TableCell>
                  <TableCell>
                    <Button variant="outline" size="sm" onClick={() => props.onSelect(project.id)}>
                      Inspect
                    </Button>
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

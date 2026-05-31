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
  projectPolicyText,
  projectQuotaPolicyText,
  templateName,
  templateRuntimeType,
  templateUseCase,
} from "@/lib/resource-utils"
import { cn } from "@/lib/utils"
import type {
  FormRecord,
  Project,
  ProjectCredential,
  ProjectPolicy,
  ProjectQuotaPolicy,
  ProjectUsage,
  Sandbox,
  Selection,
  Template,
} from "@/types"

export function ProjectTable(props: {
  projects: Project[]
  projectCredentials: Record<string, ProjectCredential[]>
  projectPolicies: Record<string, ProjectPolicy>
  projectQuotaPolicies: Record<string, ProjectQuotaPolicy>
  projectUsage: Record<string, ProjectUsage>
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
              const policy = props.projectPolicies[project.id]
              const quotaPolicy = props.projectQuotaPolicies[project.id]
              const credentialCount = props.projectCredentials[project.id]?.length || 0
              const usage = props.projectUsage[project.id]
              const taskCount = usage?.executionTasks.total || 0
              const retainedBytes = usage?.artifacts.retainedBytes || 0
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
                      <strong>{usage?.sandboxes.active ?? projectSandboxes.length} active sandboxes</strong>
                      <span>{usage?.sandboxes.running ?? runningCount} running · {taskCount} tasks · {formatBytes(retainedBytes)} retained</span>
                      <span>{projectPolicyText(policy)} · {projectQuotaPolicyText(quotaPolicy)} · {credentialCount} credentials</span>
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

function formatBytes(value: number) {
  if (!Number.isFinite(value) || value <= 0) {
    return "0 B"
  }
  const units = ["B", "KiB", "MiB", "GiB"]
  let size = value
  let unit = 0
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024
    unit += 1
  }
  const precision = unit === 0 || size >= 10 ? 0 : 1
  return `${size.toFixed(precision)} ${units[unit]}`
}

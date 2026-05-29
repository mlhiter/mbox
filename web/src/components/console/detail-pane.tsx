import { Box, FlaskConical, Layers3, SquareTerminal, X } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  collectionFor,
  formatDateTime,
  projectName,
  runtimeText,
  templateEntrypoints,
  templateName,
  templatePersistence,
  templateResourcePreset,
  templateRuntimeType,
  templateUseCase,
  templateValidationHint,
  templateValidationRun,
  templateValidationText,
  templateValidationTone,
} from "@/lib/resource-utils"
import { cn } from "@/lib/utils"
import type { Project, Sandbox, Selection, Template } from "@/types"

const emptySelectionCopy = {
  title: "No resource selected",
  body: "Select a row to see what it launches, where it runs, and what you can do next.",
  detail: "The inspector follows the selected project, environment, or sandbox.",
}

export function DetailPane({
  selection,
  projects,
  templates,
  sandboxes,
  onValidateTemplate,
  onOpenSandboxWorkspace,
  onClear,
}: {
  selection: Selection | null
  projects: Project[]
  templates: Template[]
  sandboxes: Sandbox[]
  onValidateTemplate?: (id: string) => Promise<void>
  onOpenSandboxWorkspace?: (id: string) => void
  onClear: () => void
}) {
  const selected = selection
    ? collectionFor(selection.kind, projects, templates, sandboxes).find((item) => item.id === selection.id)
    : null

  return (
    <aside className="detail" aria-label="Selected resource">
      <div className="detail-head">
        <div>
          <p className="eyebrow">{selection ? resourceLabel(selection.kind) : "Selection"}</p>
          <h2 className="panel-title">{selected ? selected.name || selected.slug || selected.id : emptySelectionCopy.title}</h2>
        </div>
        <Button variant="outline" size="icon" onClick={onClear} aria-label="Clear selection">
          <X />
        </Button>
      </div>
      <div className="detail-content">
        {!selection || !selected ? (
          <div className="detail-empty">
            <p>{emptySelectionCopy.body}</p>
            <span>{emptySelectionCopy.detail}</span>
          </div>
        ) : selection.kind === "project" ? (
          <ProjectInspector project={selected as Project} templates={templates} sandboxes={sandboxes} />
        ) : selection.kind === "template" ? (
          <TemplateInspector
            template={selected as Template}
            projects={projects}
            sandboxes={sandboxes}
            onValidateTemplate={onValidateTemplate}
            onOpenSandboxWorkspace={onOpenSandboxWorkspace}
          />
        ) : (
          <SandboxInspector
            sandbox={selected as Sandbox}
            projects={projects}
            templates={templates}
            onOpenSandboxWorkspace={onOpenSandboxWorkspace}
          />
        )}
      </div>
    </aside>
  )
}

function ProjectInspector({
  project,
  templates,
  sandboxes,
}: {
  project: Project
  templates: Template[]
  sandboxes: Sandbox[]
}) {
  const projectSandboxes = sandboxes.filter((sandbox) => sandbox.projectId === project.id)
  return (
    <>
      <ResourceBadge icon={<Box />} label="Project scope" />
      <InspectorSummary
        title={project.defaultNamespace || "No namespace"}
        body={project.repositoryUrl || "No repository URL recorded."}
      />
      <InspectorGroup
        title="Launch defaults"
        rows={[
          ["Default environment", templateName(project.defaultTemplateId, templates)],
          ["Active sandboxes", String(projectSandboxes.length)],
          ["Created", formatDateTime(project.createdAt)],
        ]}
      />
      <InspectorGroup
        title="Identity"
        rows={[
          ["Project key", project.slug],
          ["Project ID", project.id],
        ]}
      />
    </>
  )
}

function TemplateInspector({
  template,
  projects,
  sandboxes,
  onValidateTemplate,
  onOpenSandboxWorkspace,
}: {
  template: Template
  projects: Project[]
  sandboxes: Sandbox[]
  onValidateTemplate?: (id: string) => Promise<void>
  onOpenSandboxWorkspace?: (id: string) => void
}) {
  const launchedCount = sandboxes.filter((sandbox) => sandbox.templateId === template.id).length
  const validationRun = templateValidationRun(template, sandboxes)
  return (
    <>
      <ResourceBadge icon={<Layers3 />} label="Environment" />
      <InspectorSummary title={templateUseCase(template)} body={`${templateRuntimeType(template)} · ${template.image}`} />
      <TemplateValidationPanel
        template={template}
        validationRun={validationRun}
        onValidateTemplate={onValidateTemplate}
        onOpenSandboxWorkspace={onOpenSandboxWorkspace}
      />
      <InspectorGroup
        title="Launch shape"
        rows={[
          ["Preview ports", templateEntrypoints(template)],
          ["Size", templateResourcePreset(template)],
          ["Workspace", templatePersistence(template)],
          ["Validation", templateValidationText(template)],
        ]}
      />
      <InspectorGroup
        title="Runtime details"
        rows={[
          ["Scope", template.projectId ? projectName(template.projectId, projects) : "Global"],
          ["Working directory", template.workingDir || "/workspace"],
          ["Network access", template.networkPolicy || "default"],
          ["Launched sandboxes", String(launchedCount)],
        ]}
      />
    </>
  )
}

function TemplateValidationPanel({
  template,
  validationRun,
  onValidateTemplate,
  onOpenSandboxWorkspace,
}: {
  template: Template
  validationRun: ReturnType<typeof templateValidationRun>
  onValidateTemplate?: (id: string) => Promise<void>
  onOpenSandboxWorkspace?: (id: string) => void
}) {
  const sandbox = validationRun.sandbox
  return (
    <section className="environment-validation-panel" aria-label="Environment validation">
      <div className="environment-validation-head">
        <span>Validation</span>
        <Badge
          variant="secondary"
          className={cn(
            "template-validation-badge",
            `template-validation-badge-${templateValidationTone(template)}`,
          )}
          title={templateValidationHint(template)}
        >
          <span className="status-badge-dot" />
          {templateValidationText(template)}
        </Badge>
      </div>
      <dl>
        <div>
          <dt>Latest run</dt>
          <dd>{sandbox ? sandbox.name : "No validation sandbox"}</dd>
        </div>
        <div>
          <dt>Decision</dt>
          <dd>{validationRun.decidedAt ? formatDateTime(validationRun.decidedAt) : "Awaiting result"}</dd>
        </div>
      </dl>
      <div className="environment-validation-actions">
        <Button size="sm" onClick={() => void onValidateTemplate?.(template.id)}>
          <FlaskConical data-icon="inline-start" />
          Revalidate
        </Button>
        {sandbox ? (
          <Button variant="outline" size="sm" onClick={() => onOpenSandboxWorkspace?.(sandbox.id)}>
            <SquareTerminal data-icon="inline-start" />
            Open run
          </Button>
        ) : null}
      </div>
    </section>
  )
}

function SandboxInspector({
  sandbox,
  projects,
  templates,
  onOpenSandboxWorkspace,
}: {
  sandbox: Sandbox
  projects: Project[]
  templates: Template[]
  onOpenSandboxWorkspace?: (id: string) => void
}) {
  return (
    <>
      <ResourceBadge icon={<SquareTerminal />} label="Sandbox" />
      <InspectorSummary
        title={`${sandbox.status} runtime`}
        body={`${projectName(sandbox.projectId, projects)} · ${templateName(sandbox.templateId, templates)}`}
      />
      <div className="detail-actions">
        <Button onClick={() => onOpenSandboxWorkspace?.(sandbox.id)}>
          <SquareTerminal data-icon="inline-start" />
          Open workspace
        </Button>
      </div>
      <InspectorGroup
        title="Runtime"
        rows={[
          ["Environment", templateName(sandbox.templateId, templates)],
          ["Namespace", sandbox.namespace],
          ["Runtime identity", sandbox.serviceAccountName],
          ["Runtime resource", runtimeText(sandbox.runtimeRef)],
        ]}
      />
      <InspectorGroup
        title="Identity"
        rows={[
          ["Sandbox key", sandbox.slug],
          ["Created", formatDateTime(sandbox.createdAt)],
          ["Sandbox ID", sandbox.id],
        ]}
      />
    </>
  )
}

function ResourceBadge({ icon, label }: { icon: React.ReactNode; label: string }) {
  return (
    <Badge className="detail-badge bg-[var(--info-soft)] text-[var(--info-ink)] hover:bg-[var(--info-soft)]">
      {icon}
      {label}
    </Badge>
  )
}

function InspectorSummary({ title, body }: { title: string; body: string }) {
  return (
    <div className="detail-summary">
      <strong>{title}</strong>
      <span>{body}</span>
    </div>
  )
}

function InspectorGroup({
  title,
  rows,
}: {
  title: string
  rows: Array<[string, string]>
}) {
  return (
    <section className="detail-group">
      <h3>{title}</h3>
      <dl className="kv">
        {rows.map(([key, value]) => (
          <div key={key}>
            <dt>{key}</dt>
            <dd>{String(value || "-")}</dd>
          </div>
        ))}
      </dl>
    </section>
  )
}

function resourceLabel(kind: Selection["kind"]) {
  if (kind === "project") {
    return "Project"
  }
  if (kind === "template") {
    return "Environment"
  }
  return "Sandbox"
}

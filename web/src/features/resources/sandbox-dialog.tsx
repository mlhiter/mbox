import { useMemo, useState } from "react"
import { Play } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  FieldGroup,
  ResourceDialog,
  SelectField,
  TextField,
} from "@/features/resources/resource-dialog"
import {
  slugFromName,
  templateEntrypoints,
  templatePersistence,
  templateResourcePreset,
  templateRuntimeType,
  templateUseCase,
  templateValidationHint,
  templateValidationText,
} from "@/lib/resource-utils"
import type { FormRecord, Project, Template } from "@/types"

export function SandboxDialog({
  projects,
  templates,
  onSubmit,
}: {
  projects: Project[]
  templates: Template[]
  onSubmit: (data: FormRecord) => Promise<void>
}) {
  const canLaunch = projects.length > 0 && templates.length > 0
  const missing = [
    projects.length === 0 ? "Create a project first." : "",
    templates.length === 0 ? "Create an environment first." : "",
  ].filter(Boolean)
  const firstProjectID = projects[0]?.id || ""
  const [projectID, setProjectID] = useState(firstProjectID)
  const selectedProject = projects.find((project) => project.id === projectID) || projects[0]
  const defaultTemplate = templates.find((template) => template.id === selectedProject?.defaultTemplateId)
  const templateOptions = useMemo(() => {
    const projectTemplates = availableTemplatesForProject(selectedProject, templates)
    return defaultTemplate
      ? [{ value: "default", label: `Project default: ${defaultTemplate.name}` }, ...projectTemplates.map((template) => ({ value: template.id, label: template.name }))]
      : projectTemplates.map((template) => ({ value: template.id, label: template.name }))
  }, [defaultTemplate, selectedProject?.id, templates])
  const firstTemplateID = templateOptions[0]?.value || "none"
  const [templateID, setTemplateID] = useState(firstTemplateID)
  const [sandboxName, setSandboxName] = useState("")
  const selectedTemplateID = templateID === "default" ? defaultTemplate?.id : templateID || (firstTemplateID === "default" ? defaultTemplate?.id : firstTemplateID)
  const selectedTemplate = selectedTemplateID === "none" ? undefined : templates.find((template) => template.id === selectedTemplateID) || defaultTemplate
  const launchName = sandboxName.trim() || suggestedSandboxName(selectedProject, selectedTemplate)
  const launchKey = slugFromName(launchName) || "Generated on launch"
  const hasLaunchEnvironment = Boolean(selectedTemplate)

  function selectProject(nextProjectID: string) {
    const nextProject = projects.find((project) => project.id === nextProjectID)
    const nextDefaultTemplate = templates.find((template) => template.id === nextProject?.defaultTemplateId)
    const nextTemplates = availableTemplatesForProject(nextProject, templates)
    setProjectID(nextProjectID)
    setTemplateID(nextDefaultTemplate ? "default" : nextTemplates[0]?.id || "")
  }

  function resetLaunchPlan(open: boolean) {
    if (open) {
      setProjectID(firstProjectID)
      setTemplateID(firstTemplateID)
      setSandboxName("")
    }
  }

  return (
    <ResourceDialog
      title="Launch sandbox"
      description="Create a controlled runtime workspace from a project and environment."
      trigger={
        <Button disabled={!canLaunch} title={canLaunch ? undefined : missing.join(" ")}>
          <Play data-icon="inline-start" />
          Launch sandbox
        </Button>
      }
      submitLabel="Launch"
      onSubmit={(data) => {
        if (!selectedTemplate) {
          throw new Error("Select an environment before launching.")
        }
        return onSubmit(data)
      }}
      onOpenChange={resetLaunchPlan}
    >
      <FieldGroup>
        {!canLaunch ? (
          <div className="form-note">
            {missing.map((item) => (
              <span key={item}>{item}</span>
            ))}
          </div>
        ) : null}
        <SelectField
          name="projectId"
          label="Project"
          required
          value={projectID}
          onValueChange={selectProject}
          items={projects.map((project) => ({ value: project.id, label: project.name }))}
        />
        <SelectField
          name="templateId"
          label="Environment"
          value={templateID || firstTemplateID}
          onValueChange={setTemplateID}
          items={templateOptions.length ? templateOptions : [{ value: "none", label: "No environment available" }]}
          required
        />
        <TextField
          name="name"
          label="Name"
          required
          value={sandboxName}
          placeholder={launchName}
          onChange={(event) => setSandboxName(event.target.value)}
        />
        <LaunchPlan project={selectedProject} template={selectedTemplate} name={launchName} sandboxKey={launchKey} />
        {!hasLaunchEnvironment ? (
          <div className="form-note">
            <span>This project has no global or project-scoped environment to launch.</span>
          </div>
        ) : null}
      </FieldGroup>
    </ResourceDialog>
  )
}

function LaunchPlan({
  project,
  template,
  name,
  sandboxKey,
}: {
  project: Project | undefined
  template: Template | undefined
  name: string
  sandboxKey: string
}) {
  return (
    <section className="launch-plan" aria-label="Launch plan">
      <div className="launch-plan-head">
        <span>Launch plan</span>
        <strong>{name || "New sandbox"}</strong>
      </div>
      <dl>
        <LaunchPlanRow label="Environment" value={template ? template.name : "Select an environment"} hint={template ? `${templateRuntimeType(template)} · ${templateUseCase(template)}` : undefined} />
        <LaunchPlanRow label="Validation" value={template ? templateValidationText(template) : "-"} hint={template ? templateValidationHint(template) : undefined} />
        <LaunchPlanRow label="Preview ports" value={template ? templateEntrypoints(template) : "-"} />
        <LaunchPlanRow label="Size" value={template ? templateResourcePreset(template) : "-"} hint={template ? templatePersistence(template) : undefined} />
        <LaunchPlanRow label="Placement" value={project?.defaultNamespace || "No namespace"} hint="Runtime identity: mbox-sandbox" />
        <LaunchPlanRow label="Sandbox key" value={sandboxKey || "Generated from name"} />
      </dl>
    </section>
  )
}

function LaunchPlanRow({
  label,
  value,
  hint,
}: {
  label: string
  value: string
  hint?: string
}) {
  return (
    <div>
      <dt>{label}</dt>
      <dd>
        <span>{value}</span>
        {hint ? <small>{hint}</small> : null}
      </dd>
    </div>
  )
}

function suggestedSandboxName(project: Project | undefined, template: Template | undefined) {
  const projectPart = project?.slug || project?.name || "sandbox"
  const templatePart = template?.slug || template?.name || "workspace"
  return `${projectPart}-${templatePart}`.slice(0, 58)
}

function availableTemplatesForProject(project: Project | undefined, templates: Template[]) {
  return templates.filter((template) => !template.projectId || template.projectId === project?.id)
}

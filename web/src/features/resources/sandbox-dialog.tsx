import { Play } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  FieldGroup,
  ResourceDialog,
  SelectField,
  TextField,
} from "@/features/resources/resource-dialog"
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
    templates.length === 0 ? "Create a template first." : "",
  ].filter(Boolean)

  return (
    <ResourceDialog
      title="Launch sandbox"
      description="Create a controlled sandbox record from a project and template."
      trigger={
        <Button disabled={!canLaunch} title={canLaunch ? undefined : missing.join(" ")}>
          <Play data-icon="inline-start" />
          Launch sandbox
        </Button>
      }
      submitLabel="Launch"
      onSubmit={onSubmit}
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
          defaultValue={projects[0]?.id}
          items={projects.map((project) => ({ value: project.id, label: project.name }))}
        />
        <SelectField
          name="templateId"
          label="Template"
          defaultValue="default"
          items={[
            { value: "default", label: "Project default" },
            ...templates.map((template) => ({ value: template.id, label: template.name })),
          ]}
        />
        <TextField name="name" label="Name" required />
        <TextField name="slug" label="Slug" required pattern="[a-z0-9]([a-z0-9-]*[a-z0-9])?" />
        <div className="dialog-row">
          <TextField name="namespace" label="Namespace" />
          <TextField name="serviceAccountName" label="ServiceAccount" placeholder="mbox-sandbox" />
        </div>
      </FieldGroup>
    </ResourceDialog>
  )
}

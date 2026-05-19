import { Plus } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  CheckboxField,
  FieldGroup,
  ResourceDialog,
  SelectField,
  TextField,
} from "@/features/resources/resource-dialog"
import type { FormRecord, Project } from "@/types"

export function TemplateDialog({
  projects,
  onSubmit,
}: {
  projects: Project[]
  onSubmit: (data: FormRecord) => Promise<void>
}) {
  return (
    <ResourceDialog
      title="New template"
      description="Define a reusable sandbox launch shape."
      trigger={<Button variant="outline"><Plus data-icon="inline-start" />New template</Button>}
      submitLabel="Create"
      onSubmit={onSubmit}
    >
      <FieldGroup>
        <SelectField
          name="projectId"
          label="Project"
          defaultValue={projects[0]?.id || "global"}
          items={[
            { value: "global", label: "Global template" },
            ...projects.map((project) => ({ value: project.id, label: project.name })),
          ]}
        />
        <TextField name="name" label="Name" required />
        <TextField name="slug" label="Slug" required pattern="[a-z0-9]([a-z0-9-]*[a-z0-9])?" />
        <TextField name="image" label="Image" required defaultValue="busybox:1.37" />
        <TextField
          name="startupCommand"
          label="Startup command"
          defaultValue="sh -c 'mkdir -p /workspace && echo mbox sandbox ready && sleep 86400'"
        />
        <TextField name="workingDir" label="Working dir" defaultValue="/workspace" />
        <div className="dialog-row">
          <TextField name="cpuRequest" label="CPU" defaultValue="50m" />
          <TextField name="memoryRequest" label="Memory" defaultValue="64Mi" />
          <TextField name="storageRequest" label="Storage" defaultValue="1Gi" />
        </div>
        <TextField name="exposedPorts" label="Ports" placeholder="web:3000" />
        <CheckboxField name="setDefault" label="Set as project default" defaultChecked />
      </FieldGroup>
    </ResourceDialog>
  )
}

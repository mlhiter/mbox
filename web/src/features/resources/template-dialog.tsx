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
        <TextField name="name" label="Name" required defaultValue="Node.js Workspace" />
        <TextField
          name="slug"
          label="Slug"
          required
          pattern="[a-z0-9]([a-z0-9-]*[a-z0-9])?"
          defaultValue="nodejs-workspace"
        />
        <TextField name="image" label="Image" required defaultValue="node:22-bookworm-slim" />
        <TextField
          name="startupCommand"
          label="Startup command"
          defaultValue="sh -c 'mkdir -p /workspace && cd /workspace && echo mbox node sandbox ready && tail -f /dev/null'"
        />
        <TextField name="workingDir" label="Working dir" defaultValue="/workspace" />
        <div className="dialog-row">
          <TextField name="cpuRequest" label="CPU" defaultValue="250m" />
          <TextField name="memoryRequest" label="Memory" defaultValue="512Mi" />
          <TextField name="storageRequest" label="Storage" defaultValue="2Gi" />
        </div>
        <TextField name="exposedPorts" label="Ports" defaultValue="web:3000" placeholder="web:3000" />
        <CheckboxField name="setDefault" label="Set as project default" defaultChecked />
      </FieldGroup>
    </ResourceDialog>
  )
}

import { Plus } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  FieldGroup,
  ResourceDialog,
  TextField,
} from "@/features/resources/resource-dialog"
import type { FormRecord } from "@/types"

export function ProjectDialog({ onSubmit }: { onSubmit: (data: FormRecord) => Promise<void> }) {
  return (
    <ResourceDialog
      title="New project"
      description="Create the product record that binds a repository and default namespace."
      trigger={<Button variant="outline"><Plus data-icon="inline-start" />New project</Button>}
      submitLabel="Create"
      onSubmit={onSubmit}
    >
      <FieldGroup>
        <TextField name="name" label="Name" required />
        <TextField name="slug" label="Slug" required pattern="[a-z0-9]([a-z0-9-]*[a-z0-9])?" />
        <TextField name="repositoryUrl" label="Repository URL" />
        <TextField name="defaultNamespace" label="Default namespace" required />
      </FieldGroup>
    </ResourceDialog>
  )
}

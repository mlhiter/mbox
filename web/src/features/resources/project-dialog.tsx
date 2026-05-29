import { useState } from "react"
import { Plus } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  FieldGroup,
  ResourceDialog,
  TextField,
} from "@/features/resources/resource-dialog"
import { slugFromName } from "@/lib/resource-utils"
import type { FormRecord } from "@/types"

export function ProjectDialog({ onSubmit }: { onSubmit: (data: FormRecord) => Promise<void> }) {
  const [name, setName] = useState("")
  const [namespace, setNamespace] = useState("")
  const [namespaceEdited, setNamespaceEdited] = useState(false)

  function resetForm(open: boolean) {
    if (open) {
      setName("")
      setNamespace("")
      setNamespaceEdited(false)
    }
  }

  function updateName(nextName: string) {
    setName(nextName)
    if (!namespaceEdited) {
      setNamespace(suggestedNamespace(nextName))
    }
  }

  function updateNamespace(nextNamespace: string) {
    setNamespaceEdited(true)
    setNamespace(nextNamespace)
  }

  return (
    <ResourceDialog
      title="New project"
      description="Create the product record that binds a repository and default namespace."
      trigger={<Button variant="outline"><Plus data-icon="inline-start" />New project</Button>}
      submitLabel="Create"
      onSubmit={onSubmit}
      onOpenChange={resetForm}
    >
      <FieldGroup>
        <TextField
          name="name"
          label="Project name"
          required
          value={name}
          onChange={(event) => updateName(event.target.value)}
        />
        <TextField name="repositoryUrl" label="Repository URL" />
        <TextField
          name="defaultNamespace"
          label="Default namespace"
          required
          value={namespace}
          placeholder="mbox-project"
          onChange={(event) => updateNamespace(event.target.value)}
        />
      </FieldGroup>
    </ResourceDialog>
  )
}

function suggestedNamespace(name: string) {
  const slug = slugFromName(name)
  return slug ? `mbox-${slug}` : ""
}

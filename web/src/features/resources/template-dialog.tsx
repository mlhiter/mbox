import { useEffect, useState } from "react"
import { Plus, SquarePen } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import {
  CheckboxField,
  FieldGroup,
  ResourceDialog,
  SelectField,
  TextareaField,
  TextField,
} from "@/features/resources/resource-dialog"
import {
  defaultImageForRuntime,
  defaultPortsForRuntime,
  defaultStartupCommandForRuntime,
  defaultUseCaseForRuntime,
  formatCommand,
  formatJSONField,
  formatKeyValueLines,
  formatPorts,
  formatSecretRefs,
  presetForResources,
  resourcesForPreset,
  templateResourcePreset,
  templateRuntimeType,
  templateUseCase,
} from "@/lib/resource-utils"
import type { FormRecord, Project, Template } from "@/types"

const runtimeTypes = ["Node.js", "Python", "Go", "Browser", "Notebook", "Custom"]
const resourcePresets = ["Small", "Medium", "Large", "Custom"]

export function TemplateDialog({
  projects,
  template,
  triggerClassName,
  onSubmit,
}: {
  projects: Project[]
  template?: Template
  triggerClassName?: string
  onSubmit: (data: FormRecord) => Promise<void>
}) {
  const editing = Boolean(template)
  const initialRuntimeType = template ? templateRuntimeType(template) : "Node.js"
  const initialResourcePreset = template ? templateResourcePreset(template) : "Small"
  const initialResources = resourcesForPreset(initialResourcePreset)
  const initialValues = template
    ? {
        name: template.name,
        slug: template.slug,
        runtimeType: initialRuntimeType,
        useCase: templateUseCase(template),
        resourcePreset: initialResourcePreset,
        image: template.image,
        startupCommand: formatCommand(template.startupCommand),
        workingDir: template.workingDir || "",
        cpuRequest: template.cpuRequest || initialResources.cpuRequest,
        memoryRequest: template.memoryRequest || initialResources.memoryRequest,
        storageRequest: template.storageRequest || "2Gi",
        exposedPorts: formatPorts(template.exposedPorts || []),
        env: formatKeyValueLines(template.env),
        secretRefs: formatSecretRefs(template.secretRefs),
        networkPolicy: template.networkPolicy || "default",
        lifecyclePolicy: formatJSONField(template.lifecyclePolicy),
      }
    : {
        name: "Node.js Web App",
        slug: "nodejs-web-app",
        runtimeType: initialRuntimeType,
        useCase: defaultUseCaseForRuntime(initialRuntimeType),
        resourcePreset: initialResourcePreset,
        image: defaultImageForRuntime(initialRuntimeType),
        startupCommand: defaultStartupCommandForRuntime(initialRuntimeType),
        workingDir: "/workspace",
        cpuRequest: initialResources.cpuRequest,
        memoryRequest: initialResources.memoryRequest,
        storageRequest: "2Gi",
        exposedPorts: defaultPortsForRuntime(initialRuntimeType),
        env: "",
        secretRefs: "",
        networkPolicy: "default",
        lifecyclePolicy: "",
      }
  const [runtimeType, setRuntimeType] = useState(initialValues.runtimeType)
  const [resourcePreset, setResourcePreset] = useState(initialValues.resourcePreset)
  const [useCase, setUseCase] = useState(initialValues.useCase)
  const [image, setImage] = useState(initialValues.image)
  const [startupCommand, setStartupCommand] = useState(initialValues.startupCommand)
  const [ports, setPorts] = useState(initialValues.exposedPorts)
  const [cpuRequest, setCPURequest] = useState(initialValues.cpuRequest)
  const [memoryRequest, setMemoryRequest] = useState(initialValues.memoryRequest)

  function resetTemplateFields() {
    setRuntimeType(initialValues.runtimeType)
    setResourcePreset(initialValues.resourcePreset)
    setUseCase(initialValues.useCase)
    setImage(initialValues.image)
    setStartupCommand(initialValues.startupCommand)
    setPorts(initialValues.exposedPorts)
    setCPURequest(initialValues.cpuRequest)
    setMemoryRequest(initialValues.memoryRequest)
  }

  useEffect(() => {
    resetTemplateFields()
  }, [
    template?.id,
    template?.updatedAt,
    initialValues.runtimeType,
    initialValues.resourcePreset,
    initialValues.useCase,
    initialValues.image,
    initialValues.startupCommand,
    initialValues.exposedPorts,
    initialValues.cpuRequest,
    initialValues.memoryRequest,
  ])

  function selectRuntime(nextRuntimeType: string) {
    setRuntimeType(nextRuntimeType)
    if (!editing) {
      setUseCase(defaultUseCaseForRuntime(nextRuntimeType))
      setImage(defaultImageForRuntime(nextRuntimeType))
      setStartupCommand(defaultStartupCommandForRuntime(nextRuntimeType))
      setPorts(defaultPortsForRuntime(nextRuntimeType))
    }
  }

  function selectResourcePreset(nextResourcePreset: string) {
    if (nextResourcePreset === "Custom") {
      setResourcePreset(nextResourcePreset)
      return
    }
    const nextResources = resourcesForPreset(nextResourcePreset)
    setResourcePreset(nextResourcePreset)
    setCPURequest(nextResources.cpuRequest)
    setMemoryRequest(nextResources.memoryRequest)
  }

  function updateCPURequest(nextCPURequest: string) {
    setCPURequest(nextCPURequest)
    setResourcePreset(presetForResources(nextCPURequest, memoryRequest))
  }

  function updateMemoryRequest(nextMemoryRequest: string) {
    setMemoryRequest(nextMemoryRequest)
    setResourcePreset(presetForResources(cpuRequest, nextMemoryRequest))
  }

  return (
    <ResourceDialog
      title={editing ? "Edit environment template" : "New environment template"}
      description={editing ? "Tune the ready-to-run environment users launch from." : "Create a ready-to-run sandbox environment."}
      trigger={
        editing ? (
          <Button
            variant="ghost"
            size="icon-sm"
            className={triggerClassName}
            aria-label={`Edit ${template?.name}`}
            title="Edit template"
          >
            <SquarePen />
          </Button>
        ) : (
          <Button variant="outline">
            <Plus data-icon="inline-start" />
            New template
          </Button>
        )
      }
      submitLabel={editing ? "Save template" : "Create template"}
      onSubmit={onSubmit}
      onOpenChange={(nextOpen) => {
        if (!nextOpen) {
          resetTemplateFields()
        }
      }}
    >
      <FieldGroup>
        {!editing ? (
          <SelectField
            name="projectId"
            label="Scope"
            defaultValue={projects[0]?.id || "global"}
            items={[
              { value: "global", label: "Global template" },
              ...projects.map((project) => ({ value: project.id, label: project.name })),
            ]}
          />
        ) : null}

        <div className="template-form-section">
          <div>
            <p className="template-form-eyebrow">Essentials</p>
            <p className="template-form-note">Users pick templates by purpose, entrypoint, and resource fit.</p>
          </div>
          <TextField name="name" label="Template name" required defaultValue={initialValues.name} />
          {!editing ? (
            <TextField
              name="slug"
              label="Alias"
              required
              pattern="[a-z0-9]([a-z0-9-]*[a-z0-9])?"
              defaultValue={initialValues.slug}
            />
          ) : null}
          <div className="dialog-row">
            <SelectField
              name="runtimeType"
              label="Runtime"
              value={runtimeType}
              onValueChange={selectRuntime}
              items={runtimeTypes.map((item) => ({ value: item, label: item }))}
            />
            <SelectField
              name="resourcePreset"
              label="Resource preset"
              value={resourcePreset}
              onValueChange={selectResourcePreset}
              items={resourcePresets.map((item) => ({ value: item, label: item }))}
            />
          </div>
          <TextField name="useCase" label="Use case" value={useCase} onChange={(event) => setUseCase(event.target.value)} />
          <TextField
            name="exposedPorts"
            label="Entrypoints"
            value={ports}
            onChange={(event) => setPorts(event.target.value)}
            placeholder="web:3000, api:8080"
          />
          <TextField name="storageRequest" label="Workspace storage" defaultValue={initialValues.storageRequest} />
        </div>

        <Separator />

        <details className="advanced-template-settings">
          <summary>Advanced settings</summary>
          <FieldGroup>
            <TextField name="image" label="Base image" required value={image} onChange={(event) => setImage(event.target.value)} />
            <TextareaField
              name="startupCommand"
              label="Start command"
              value={startupCommand}
              onChange={(event) => setStartupCommand(event.target.value)}
              rows={3}
            />
            <TextField name="workingDir" label="Working dir" defaultValue={initialValues.workingDir} />
            <div className="dialog-row">
              <TextField name="cpuRequest" label="CPU" value={cpuRequest} onChange={(event) => updateCPURequest(event.target.value)} />
              <TextField name="memoryRequest" label="Memory" value={memoryRequest} onChange={(event) => updateMemoryRequest(event.target.value)} />
            </div>
            <TextareaField name="env" label="Environment" defaultValue={initialValues.env} placeholder="NODE_ENV=development" rows={3} />
            <TextField
              name="secretRefs"
              label="Secret refs"
              defaultValue={initialValues.secretRefs}
              placeholder="github-token:token, npm-token"
            />
            <SelectField
              name="networkPolicy"
              label="Network policy"
              defaultValue={initialValues.networkPolicy}
              items={[
                { value: "default", label: "Default" },
                { value: "restricted", label: "Restricted" },
                { value: "open", label: "Open" },
              ]}
            />
            <TextareaField
              name="lifecyclePolicy"
              label="Lifecycle policy"
              defaultValue={initialValues.lifecyclePolicy}
              placeholder='{"idleTimeoutSeconds":3600}'
              rows={3}
            />
          </FieldGroup>
        </details>

        {!editing ? <CheckboxField name="setDefault" label="Use as project default when scoped to a project" defaultChecked /> : null}
      </FieldGroup>
    </ResourceDialog>
  )
}

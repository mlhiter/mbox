import {
  useEffect,
  useMemo,
  useRef,
  useState,
  type FormEvent,
  type ReactNode,
} from "react"
import { FitAddon } from "@xterm/addon-fit"
import { Terminal } from "@xterm/xterm"
import "@xterm/xterm/css/xterm.css"
import { toast } from "sonner"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardAction, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from "@/components/ui/empty"
import {
  Field,
  FieldContent,
  FieldGroup,
  FieldLabel,
  FieldTitle,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Toaster } from "@/components/ui/sonner"
import { cn } from "@/lib/utils"

type APIState = "checking" | "ok" | "bad"
type ResourceKind = "project" | "template" | "sandbox"

type RuntimeRef = {
  kind: string
  namespace: string
  name: string
}

type RuntimeTarget = {
  namespace: string
  podName: string
  container: string
  phase: string
  selector: string
  commands?: string[]
}

type LogResult = {
  target: RuntimeTarget
  logs: string
}

type RuntimeEvent = {
  type?: string
  reason?: string
  message?: string
  count?: number
  firstTimestamp?: string
  lastTimestamp?: string
}

type Project = {
  id: string
  name: string
  slug: string
  repositoryUrl?: string
  defaultNamespace: string
  defaultTemplateId?: string
}

type Template = {
  id: string
  projectId?: string
  name: string
  slug: string
  image: string
  startupCommand?: string[]
  workingDir?: string
  cpuRequest?: string
  memoryRequest?: string
  storageRequest?: string
}

type Sandbox = {
  id: string
  projectId: string
  templateId?: string
  name: string
  slug: string
  namespace: string
  serviceAccountName: string
  status: string
  runtimeRef?: RuntimeRef
  ports?: Array<{ name: string; port: number; protocol: string; previewUrl?: string }>
}

type Selection = {
  kind: ResourceKind
  id: string
}

type ListResponse<T> = {
  items?: T[]
}

type FormRecord = Record<string, FormDataEntryValue>

const emptySelectionCopy = {
  title: "No resource selected",
  body: "Inspect a row to see IDs, runtime state, and configuration.",
  detail: "Nothing is selected yet.",
}

export function App() {
  const [projects, setProjects] = useState<Project[]>([])
  const [templates, setTemplates] = useState<Template[]>([])
  const [sandboxes, setSandboxes] = useState<Sandbox[]>([])
  const [selection, setSelection] = useState<Selection | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [apiState, setAPIState] = useState<{ state: APIState; label: string }>({
    state: "checking",
    label: "Checking API",
  })

  const counts = useMemo(
    () => ({
      projects: projects.length,
      templates: templates.length,
      sandboxes: sandboxes.length,
      running: sandboxes.filter((sandbox) => sandbox.status === "running").length,
    }),
    [projects, sandboxes, templates],
  )

  async function loadAll() {
    setLoading(true)
    setError(null)
    setAPIState({ state: "checking", label: "Checking API" })
    try {
      const [health, projectList, templateList, sandboxList] = await Promise.all([
        request<{ status?: string }>("/healthz"),
        request<ListResponse<Project>>("/v1/projects"),
        request<ListResponse<Template>>("/v1/templates"),
        request<ListResponse<Sandbox>>("/v1/sandboxes"),
      ])
      setProjects(projectList.items || [])
      setTemplates(templateList.items || [])
      setSandboxes(sandboxList.items || [])
      setAPIState({
        state: health.status === "ok" ? "ok" : "bad",
        label: health.status || "Unknown",
      })
    } catch (requestError) {
      const message = requestError instanceof Error ? requestError.message : "Request failed"
      setError(message)
      setAPIState({ state: "bad", label: "API unavailable" })
      toast.error(message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void loadAll()
  }, [])

  useEffect(() => {
    if (!selection) {
      return
    }
    const exists = collectionFor(selection.kind, projects, templates, sandboxes).some(
      (item) => item.id === selection.id,
    )
    if (!exists) {
      setSelection(null)
    }
  }, [projects, sandboxes, selection, templates])

  async function createProject(data: FormRecord) {
    await request<Project>("/v1/projects", {
      method: "POST",
      body: JSON.stringify({
        name: stringValue(data.name),
        slug: stringValue(data.slug),
        repositoryUrl: stringValue(data.repositoryUrl),
        defaultNamespace: stringValue(data.defaultNamespace),
      }),
    })
    await loadAll()
    toast.success("Project created")
  }

  async function createTemplate(data: FormRecord) {
    const projectId = stringValue(data.projectId)
    const payload = compactObject({
      projectId,
      name: stringValue(data.name),
      slug: stringValue(data.slug),
      image: stringValue(data.image),
      startupCommand: parseCommand(stringValue(data.startupCommand)),
      workingDir: stringValue(data.workingDir),
      cpuRequest: stringValue(data.cpuRequest),
      memoryRequest: stringValue(data.memoryRequest),
      storageRequest: stringValue(data.storageRequest),
    })
    const template = await request<Template>("/v1/templates", {
      method: "POST",
      body: JSON.stringify(payload),
    })
    if (data.setDefault === "on" && projectId && projectId !== "global") {
      await request<Project>(`/v1/projects/${projectId}`, {
        method: "PATCH",
        body: JSON.stringify({ defaultTemplateId: template.id }),
      })
    }
    await loadAll()
    toast.success("Template created")
  }

  async function createSandbox(data: FormRecord) {
    const payload = compactObject({
      projectId: stringValue(data.projectId),
      templateId: stringValue(data.templateId),
      name: stringValue(data.name),
      slug: stringValue(data.slug),
      namespace: stringValue(data.namespace),
      serviceAccountName: stringValue(data.serviceAccountName),
    })
    const sandbox = await request<Sandbox>("/v1/sandboxes", {
      method: "POST",
      body: JSON.stringify(payload),
    })
    await loadAll()
    setSelection({ kind: "sandbox", id: sandbox.id })
    toast.success("Sandbox launched")
  }

  async function deleteSandbox(id: string) {
    await request<void>(`/v1/sandboxes/${id}`, { method: "DELETE" })
    if (selection?.kind === "sandbox" && selection.id === id) {
      setSelection(null)
    }
    await loadAll()
    toast.success("Sandbox deleted")
  }

  return (
    <>
      <div className="shell">
        <Rail apiState={apiState} />
        <main className="workspace">
          <header className="topbar">
            <div>
              <p className="eyebrow">Kubernetes sandbox operations</p>
              <h1 className="page-title">Runtime console</h1>
              <p className="page-note">
                Create projects, shape reusable templates, and launch controlled sandbox records.
              </p>
            </div>
            <Button onClick={() => void loadAll()}>Refresh</Button>
          </header>

          <Summary counts={counts} />

          <div className="resource-grid">
            <ProjectTable
              projects={projects}
              templates={templates}
              loading={loading}
              error={error}
              selection={selection}
              onSelect={(id) => setSelection({ kind: "project", id })}
              onCreate={createProject}
            />
            <TemplateTable
              projects={projects}
              templates={templates}
              loading={loading}
              error={error}
              selection={selection}
              onSelect={(id) => setSelection({ kind: "template", id })}
              onCreate={createTemplate}
            />
            <SandboxTable
              projects={projects}
              templates={templates}
              sandboxes={sandboxes}
              loading={loading}
              error={error}
              selection={selection}
              onSelect={(id) => setSelection({ kind: "sandbox", id })}
              onCreate={createSandbox}
              onDelete={(id) => void deleteSandbox(id)}
            />
          </div>
        </main>
        <DetailPane
          selection={selection}
          projects={projects}
          templates={templates}
          sandboxes={sandboxes}
          onClear={() => setSelection(null)}
        />
      </div>
      <Toaster position="bottom-right" />
    </>
  )
}

function Rail({ apiState }: { apiState: { state: APIState; label: string } }) {
  return (
    <aside className="rail" aria-label="Main navigation">
      <div className="brand">
        <span className="brand-mark">m</span>
        <div>
          <strong>mbox</strong>
          <span>control plane</span>
        </div>
      </div>
      <nav className="nav">
        <a href="#projects">Projects</a>
        <a href="#templates">Templates</a>
        <a href="#sandboxes">Sandboxes</a>
      </nav>
      <div className="rail-status">
        <span
          className={cn(
            "status-dot",
            apiState.state === "ok" && "ok",
            apiState.state === "bad" && "bad",
          )}
        />
        <span>{apiState.label}</span>
      </div>
    </aside>
  )
}

function Summary({ counts }: { counts: Record<string, number> }) {
  return (
    <section className="summary" aria-label="Resource summary">
      <div>
        <span>{counts.projects}</span>
        <p>Projects</p>
      </div>
      <div>
        <span>{counts.templates}</span>
        <p>Templates</p>
      </div>
      <div>
        <span>{counts.sandboxes}</span>
        <p>Sandboxes</p>
      </div>
      <div>
        <span>{counts.running}</span>
        <p>Running</p>
      </div>
    </section>
  )
}

function ProjectTable(props: {
  projects: Project[]
  templates: Template[]
  loading: boolean
  error: string | null
  selection: Selection | null
  onSelect: (id: string) => void
  onCreate: (data: FormRecord) => Promise<void>
}) {
  return (
    <Panel
      id="projects"
      eyebrow="Scope"
      title="Projects"
      action={<ProjectDialog onSubmit={props.onCreate} />}
    >
      <Table className="resource-table">
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Namespace</TableHead>
            <TableHead>Default template</TableHead>
            <TableHead />
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.loading ? (
            <SkeletonRows columns={4} />
          ) : props.error ? (
            <EmptyRow columns={4} title="Could not load projects" detail="Check the API server and refresh." />
          ) : props.projects.length === 0 ? (
            <EmptyRow columns={4} title="No projects yet" detail="Create one to bind a repository and default namespace." />
          ) : (
            props.projects.map((project) => (
              <TableRow
                key={project.id}
                className={cn(props.selection?.kind === "project" && props.selection.id === project.id && "is-selected")}
                data-state={props.selection?.kind === "project" && props.selection.id === project.id ? "selected" : undefined}
              >
                <TableCell>{titleCell(project.name, project.slug)}</TableCell>
                <TableCell className="mono">{project.defaultNamespace}</TableCell>
                <TableCell>{templateName(project.defaultTemplateId, props.templates)}</TableCell>
                <TableCell>
                  <Button variant="outline" size="sm" onClick={() => props.onSelect(project.id)}>
                    Inspect
                  </Button>
                </TableCell>
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>
    </Panel>
  )
}

function TemplateTable(props: {
  projects: Project[]
  templates: Template[]
  loading: boolean
  error: string | null
  selection: Selection | null
  onSelect: (id: string) => void
  onCreate: (data: FormRecord) => Promise<void>
}) {
  return (
    <Panel
      id="templates"
      eyebrow="Launch shape"
      title="Templates"
      action={<TemplateDialog projects={props.projects} onSubmit={props.onCreate} />}
    >
      <Table className="resource-table">
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Image</TableHead>
            <TableHead>Resources</TableHead>
            <TableHead />
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.loading ? (
            <SkeletonRows columns={4} />
          ) : props.error ? (
            <EmptyRow columns={4} title="Could not load templates" detail="Check the API server and refresh." />
          ) : props.templates.length === 0 ? (
            <EmptyRow columns={4} title="No templates yet" detail="Create a launch shape before starting sandboxes." />
          ) : (
            props.templates.map((template) => (
              <TableRow
                key={template.id}
                className={cn(props.selection?.kind === "template" && props.selection.id === template.id && "is-selected")}
                data-state={props.selection?.kind === "template" && props.selection.id === template.id ? "selected" : undefined}
              >
                <TableCell>{titleCell(template.name, template.slug)}</TableCell>
                <TableCell className="mono">{template.image}</TableCell>
                <TableCell>{resourceText(template)}</TableCell>
                <TableCell>
                  <Button variant="outline" size="sm" onClick={() => props.onSelect(template.id)}>
                    Inspect
                  </Button>
                </TableCell>
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>
    </Panel>
  )
}

function SandboxTable(props: {
  projects: Project[]
  templates: Template[]
  sandboxes: Sandbox[]
  loading: boolean
  error: string | null
  selection: Selection | null
  onSelect: (id: string) => void
  onCreate: (data: FormRecord) => Promise<void>
  onDelete: (id: string) => void
}) {
  return (
    <Panel
      id="sandboxes"
      eyebrow="Execution"
      title="Sandboxes"
      wide
      action={<SandboxDialog projects={props.projects} templates={props.templates} onSubmit={props.onCreate} />}
    >
      <Table className="sandbox-table">
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Project</TableHead>
            <TableHead>Namespace</TableHead>
            <TableHead>Runtime</TableHead>
            <TableHead />
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.loading ? (
            <SkeletonRows columns={6} />
          ) : props.error ? (
            <EmptyRow columns={6} title="Could not load sandboxes" detail="Check the API server and refresh." />
          ) : props.sandboxes.length === 0 ? (
            <EmptyRow columns={6} title="No sandboxes yet" detail="Launch one from a project and template." />
          ) : (
            props.sandboxes.map((sandbox) => (
              <TableRow
                key={sandbox.id}
                className={cn(props.selection?.kind === "sandbox" && props.selection.id === sandbox.id && "is-selected")}
                data-state={props.selection?.kind === "sandbox" && props.selection.id === sandbox.id ? "selected" : undefined}
              >
                <TableCell>{titleCell(sandbox.name, sandbox.slug)}</TableCell>
                <TableCell>
                  <StatusBadge status={sandbox.status} />
                </TableCell>
                <TableCell>{projectName(sandbox.projectId, props.projects)}</TableCell>
                <TableCell className="mono">{sandbox.namespace}</TableCell>
                <TableCell>
                  <RuntimeCell refValue={sandbox.runtimeRef} />
                </TableCell>
                <TableCell>
                  <div className="flex justify-end gap-2">
                    <Button variant="outline" size="sm" onClick={() => props.onSelect(sandbox.id)}>
                      Inspect
                    </Button>
                    <Button variant="destructive" size="sm" onClick={() => props.onDelete(sandbox.id)}>
                      Delete
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>
    </Panel>
  )
}

function Panel({
  id,
  eyebrow,
  title,
  action,
  wide = false,
  children,
}: {
  id: string
  eyebrow: string
  title: string
  action: ReactNode
  wide?: boolean
  children: ReactNode
}) {
  return (
    <Card id={id} className={cn("panel records-table", wide && "wide")}>
      <CardHeader className="panel-head">
        <div>
          <p className="eyebrow">{eyebrow}</p>
          <CardTitle className="panel-title">{title}</CardTitle>
        </div>
        <CardAction>{action}</CardAction>
      </CardHeader>
      <CardContent className="p-0">{children}</CardContent>
    </Card>
  )
}

function ProjectDialog({ onSubmit }: { onSubmit: (data: FormRecord) => Promise<void> }) {
  return (
    <ResourceDialog
      title="New project"
      description="Create the product record that binds a repository and default namespace."
      trigger={<Button variant="outline">New project</Button>}
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

function TemplateDialog({
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
      trigger={<Button variant="outline">New template</Button>}
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
        <CheckboxField name="setDefault" label="Set as project default" defaultChecked />
      </FieldGroup>
    </ResourceDialog>
  )
}

function SandboxDialog({
  projects,
  templates,
  onSubmit,
}: {
  projects: Project[]
  templates: Template[]
  onSubmit: (data: FormRecord) => Promise<void>
}) {
  return (
    <ResourceDialog
      title="Launch sandbox"
      description="Create a controlled sandbox record from a project and template."
      trigger={<Button>Launch sandbox</Button>}
      submitLabel="Launch"
      onSubmit={onSubmit}
    >
      <FieldGroup>
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

function ResourceDialog({
  title,
  description,
  trigger,
  submitLabel,
  onSubmit,
  children,
}: {
  title: string
  description: string
  trigger: ReactNode
  submitLabel: string
  onSubmit: (data: FormRecord) => Promise<void>
  children: ReactNode
}) {
  const [open, setOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSubmitting(true)
    try {
      await onSubmit(Object.fromEntries(new FormData(event.currentTarget).entries()))
      event.currentTarget.reset()
      setOpen(false)
    } catch (submitError) {
      toast.error(submitError instanceof Error ? submitError.message : "Request failed")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>{trigger}</DialogTrigger>
      <DialogContent className="dialog-content">
        <form className="dialog-grid" onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{title}</DialogTitle>
            <DialogDescription>{description}</DialogDescription>
          </DialogHeader>
          {children}
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline" type="button">
                Cancel
              </Button>
            </DialogClose>
            <Button type="submit" disabled={submitting}>
              {submitting ? "Working..." : submitLabel}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function TextField({
  name,
  label,
  ...props
}: React.ComponentProps<typeof Input> & {
  name: string
  label: string
}) {
  return (
    <Field>
      <FieldLabel htmlFor={name}>{label}</FieldLabel>
      <Input id={name} name={name} autoComplete="off" {...props} />
    </Field>
  )
}

function SelectField({
  name,
  label,
  items,
  defaultValue,
  required,
}: {
  name: string
  label: string
  items: Array<{ value: string; label: string }>
  defaultValue?: string
  required?: boolean
}) {
  const value = defaultValue || items[0]?.value || ""
  return (
    <Field>
      <FieldLabel>{label}</FieldLabel>
      <Select name={name} defaultValue={value} required={required}>
        <SelectTrigger className="w-full">
          <SelectValue placeholder={`Select ${label.toLowerCase()}`} />
        </SelectTrigger>
        <SelectContent>
          <SelectGroup>
            {items.map((item) => (
              <SelectItem key={item.value} value={item.value}>
                {item.label}
              </SelectItem>
            ))}
          </SelectGroup>
        </SelectContent>
      </Select>
    </Field>
  )
}

function CheckboxField({
  name,
  label,
  defaultChecked = false,
}: {
  name: string
  label: string
  defaultChecked?: boolean
}) {
  return (
    <Field orientation="horizontal">
      <Checkbox id={name} name={name} defaultChecked={defaultChecked} />
      <FieldContent>
        <FieldTitle>
          <FieldLabel htmlFor={name}>{label}</FieldLabel>
        </FieldTitle>
      </FieldContent>
    </Field>
  )
}

function DetailPane({
  selection,
  projects,
  templates,
  sandboxes,
  onClear,
}: {
  selection: Selection | null
  projects: Project[]
  templates: Template[]
  sandboxes: Sandbox[]
  onClear: () => void
}) {
  const selected = selection
    ? collectionFor(selection.kind, projects, templates, sandboxes).find((item) => item.id === selection.id)
    : null
  const selectedSandbox = selection?.kind === "sandbox" ? (selected as Sandbox | null) : null

  return (
    <aside className="detail" aria-label="Selected resource">
      <div className="detail-head">
        <div>
          <p className="eyebrow">Selection</p>
          <h2 className="panel-title">{selected ? selected.name || selected.slug || selected.id : emptySelectionCopy.title}</h2>
        </div>
        <Button variant="outline" size="icon" onClick={onClear} aria-label="Clear selection">
          x
        </Button>
      </div>
      <div className="detail-content">
        {!selection || !selected ? (
          <div className="detail-empty">
            <p>{emptySelectionCopy.body}</p>
            <span>{emptySelectionCopy.detail}</span>
          </div>
        ) : (
          <>
            <Badge className="w-fit capitalize bg-[var(--info-soft)] text-[var(--info-ink)] hover:bg-[var(--info-soft)]">
              {selection.kind}
            </Badge>
            <dl className="kv">{detailRows(selection.kind, selected, projects, templates).map(([key, value]) => (
              <div key={key}>
                <dt>{key}</dt>
                <dd>{String(value || "-")}</dd>
              </div>
            ))}</dl>
            {selectedSandbox ? <SandboxRuntimePanel sandbox={selectedSandbox} /> : null}
          </>
        )}
      </div>
    </aside>
  )
}

function SandboxRuntimePanel({ sandbox }: { sandbox: Sandbox }) {
  const [target, setTarget] = useState<RuntimeTarget | null>(null)
  const [logs, setLogs] = useState("")
  const [events, setEvents] = useState<RuntimeEvent[]>([])
  const [runtimeError, setRuntimeError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  async function loadRuntime() {
    if (!sandbox.runtimeRef) {
      setRuntimeError("Runtime is not ready")
      setTarget(null)
      setLogs("")
      setEvents([])
      return
    }
    setLoading(true)
    setRuntimeError(null)
    try {
      const [runtimeTarget, logResult, eventResult] = await Promise.all([
        request<RuntimeTarget>(`/v1/sandboxes/${sandbox.id}/runtime`),
        request<LogResult>(`/v1/sandboxes/${sandbox.id}/logs?tailLines=120`),
        request<ListResponse<RuntimeEvent>>(`/v1/sandboxes/${sandbox.id}/events`),
      ])
      setTarget(runtimeTarget)
      setLogs(logResult.logs)
      setEvents(eventResult.items || [])
    } catch (requestError) {
      const message = requestError instanceof Error ? requestError.message : "Runtime request failed"
      setRuntimeError(message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void loadRuntime()
  }, [sandbox.id, sandbox.runtimeRef?.name])

  return (
    <section className="runtime-panel" aria-label="Sandbox runtime">
      <div className="runtime-panel-head">
        <div>
          <p className="eyebrow">Runtime</p>
          <h3>Terminal</h3>
        </div>
        <Button variant="outline" size="sm" onClick={() => void loadRuntime()} disabled={loading}>
          {loading ? "Loading..." : "Refresh"}
        </Button>
      </div>
      {runtimeError ? <p className="runtime-error">{runtimeError}</p> : null}
      <TerminalPane sandbox={sandbox} disabled={!sandbox.runtimeRef || sandbox.status !== "running"} />
      <div className="runtime-meta">
        <span>{target ? `${target.namespace}/${target.podName}` : "Pod pending"}</span>
        <span>{target ? `${target.container} · ${target.phase || "unknown"}` : "No runtime target"}</span>
      </div>
      <div className="runtime-observe">
        <div>
          <h4>Logs</h4>
          <pre>{logs || "No logs loaded."}</pre>
        </div>
        <div>
          <h4>Events</h4>
          {events.length === 0 ? (
            <p>No events loaded.</p>
          ) : (
            <ul>
              {events.slice(0, 6).map((event, index) => (
                <li key={`${event.reason}-${index}`}>
                  <strong>{event.reason || event.type || "Event"}</strong>
                  <span>{event.message || "-"}</span>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </section>
  )
}

function TerminalPane({ sandbox, disabled }: { sandbox: Sandbox; disabled: boolean }) {
  const hostRef = useRef<HTMLDivElement | null>(null)
  const terminalRef = useRef<Terminal | null>(null)
  const socketRef = useRef<WebSocket | null>(null)
  const inputDisposableRef = useRef<{ dispose: () => void } | null>(null)
  const [connected, setConnected] = useState(false)

  useEffect(() => {
    const host = hostRef.current
    if (!host) {
      return
    }
    const terminal = new Terminal({
      cursorBlink: true,
      convertEol: true,
      fontFamily: '"SF Mono", SFMono-Regular, ui-monospace, Menlo, Consolas, monospace',
      fontSize: 12,
      rows: 16,
      theme: {
        background: "#151512",
        foreground: "#ebe7dc",
        cursor: "#8ccf9f",
        selectionBackground: "#4e5a47",
      },
    })
    const fit = new FitAddon()
    terminal.loadAddon(fit)
    terminal.open(host)
    fit.fit()
    terminal.write("Select Connect to open a shell.\r\n")
    terminalRef.current = terminal

    const resizeObserver = new ResizeObserver(() => fit.fit())
    resizeObserver.observe(host)

    return () => {
      resizeObserver.disconnect()
      socketRef.current?.close()
      inputDisposableRef.current?.dispose()
      terminal.dispose()
      terminalRef.current = null
    }
  }, [sandbox.id])

  function connect() {
    if (disabled || socketRef.current?.readyState === WebSocket.OPEN) {
      return
    }
    const terminal = terminalRef.current
    if (!terminal) {
      return
    }
    terminal.clear()
    terminal.write("Connecting...\r\n")
    const socket = new WebSocket(terminalURL(sandbox.id))
    socketRef.current = socket

    socket.onopen = () => {
      setConnected(true)
      terminal.write("Connected.\r\n")
      terminal.focus()
    }
    socket.onmessage = (event) => {
      terminal.write(typeof event.data === "string" ? event.data : "")
    }
    socket.onerror = () => {
      terminal.write("\r\nConnection error.\r\n")
    }
    socket.onclose = () => {
      setConnected(false)
      terminal.write("\r\nConnection closed.\r\n")
      inputDisposableRef.current?.dispose()
      inputDisposableRef.current = null
    }
    inputDisposableRef.current?.dispose()
    inputDisposableRef.current = terminal.onData((data) => {
      if (socket.readyState === WebSocket.OPEN) {
        socket.send(data)
      }
    })
  }

  function disconnect() {
    socketRef.current?.close()
    socketRef.current = null
    inputDisposableRef.current?.dispose()
    inputDisposableRef.current = null
    setConnected(false)
  }

  return (
    <div className="terminal-shell">
      <div className="terminal-toolbar">
        <span>{connected ? "Connected" : disabled ? "Runtime pending" : "Disconnected"}</span>
        <div>
          <Button size="sm" onClick={connect} disabled={disabled || connected}>
            Connect
          </Button>
          <Button size="sm" variant="outline" onClick={disconnect} disabled={!connected}>
            Disconnect
          </Button>
        </div>
      </div>
      <div ref={hostRef} className="terminal-host" />
    </div>
  )
}

function SkeletonRows({ columns }: { columns: number }) {
  return (
    <>
      {Array.from({ length: 3 }, (_, index) => (
        <TableRow key={index}>
          <TableCell colSpan={columns}>
            <Skeleton className="h-3.5 w-[min(320px,80%)] rounded-full bg-[linear-gradient(90deg,oklch(0.93_0.006_82),oklch(0.975_0.004_82),oklch(0.93_0.006_82))] bg-[length:220%_100%]" />
          </TableCell>
        </TableRow>
      ))}
    </>
  )
}

function EmptyRow({ columns, title, detail }: { columns: number; title: string; detail: string }) {
  return (
    <TableRow>
      <TableCell colSpan={columns} className="empty">
        <Empty>
          <EmptyHeader>
            <EmptyTitle>{title}</EmptyTitle>
            <EmptyDescription>{detail}</EmptyDescription>
          </EmptyHeader>
        </Empty>
      </TableCell>
    </TableRow>
  )
}

function StatusBadge({ status }: { status: string }) {
  const running = status === "running"
  const failed = status === "failed"
  return (
    <Badge
      variant="secondary"
      className={cn(
        "h-6 gap-1.5 rounded-full px-2.5 font-semibold",
        running && "bg-[var(--runtime-soft)] text-[var(--runtime-ink)] hover:bg-[var(--runtime-soft)]",
        failed && "bg-[var(--danger-soft)] text-[var(--danger)] hover:bg-[var(--danger-soft)]",
      )}
    >
      <span className="status-badge-dot" />
      {status}
    </Badge>
  )
}

function RuntimeCell({ refValue }: { refValue: RuntimeRef | undefined }) {
  if (!refValue) {
    return <span className="runtime-cell empty">-</span>
  }
  return (
    <span className="runtime-cell" title={runtimeText(refValue)}>
      <strong>{refValue.kind}</strong>
      <span>{refValue.namespace}/{refValue.name}</span>
    </span>
  )
}

function titleCell(name: string, slug: string) {
  return (
    <span className="cell-title">
      <strong>{name}</strong>
      <span>{slug}</span>
    </span>
  )
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch(path, {
    headers: { "content-type": "application/json" },
    ...options,
  })
  if (!response.ok) {
    let message = `${response.status} ${response.statusText}`
    try {
      const body = (await response.json()) as { error?: string }
      message = body.error || message
    } catch {
      // Keep the HTTP status message.
    }
    throw new Error(message)
  }
  if (response.status === 204) {
    return undefined as T
  }
  return response.json() as Promise<T>
}

function collectionFor(
  kind: ResourceKind,
  projects: Project[],
  templates: Template[],
  sandboxes: Sandbox[],
) {
  if (kind === "project") {
    return projects
  }
  if (kind === "template") {
    return templates
  }
  return sandboxes
}

function detailRows(
  kind: ResourceKind,
  item: Project | Template | Sandbox,
  projects: Project[],
  templates: Template[],
): Array<[string, string]> {
  const rows: Array<[string, string]> = [
    ["ID", item.id],
    ["Slug", item.slug],
  ]
  if (kind === "project") {
    const project = item as Project
    rows.push(["Namespace", project.defaultNamespace])
    rows.push(["Repository", project.repositoryUrl || ""])
    rows.push(["Default template", templateName(project.defaultTemplateId, templates)])
  }
  if (kind === "template") {
    const template = item as Template
    rows.push(["Image", template.image])
    rows.push(["Working dir", template.workingDir || ""])
    rows.push(["Resources", resourceText(template)])
    rows.push(["Project", template.projectId ? projectName(template.projectId, projects) : "Global"])
  }
  if (kind === "sandbox") {
    const sandbox = item as Sandbox
    rows.push(["Status", sandbox.status])
    rows.push(["Project", projectName(sandbox.projectId, projects)])
    rows.push(["Template", templateName(sandbox.templateId, templates)])
    rows.push(["Namespace", sandbox.namespace])
    rows.push(["ServiceAccount", sandbox.serviceAccountName])
    rows.push(["Runtime", runtimeText(sandbox.runtimeRef)])
  }
  return rows
}

function projectName(id: string | undefined, projects: Project[]) {
  return projects.find((project) => project.id === id)?.name || shortID(id)
}

function templateName(id: string | undefined, templates: Template[]) {
  if (!id) {
    return "-"
  }
  return templates.find((template) => template.id === id)?.name || shortID(id)
}

function resourceText(template: Template) {
  return [template.cpuRequest, template.memoryRequest, template.storageRequest].filter(Boolean).join(" / ") || "-"
}

function runtimeText(ref: RuntimeRef | undefined) {
  if (!ref) {
    return "-"
  }
  return `${ref.kind} ${ref.namespace}/${ref.name}`
}

function terminalURL(sandboxID: string) {
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:"
  return `${protocol}//${window.location.host}/v1/sandboxes/${sandboxID}/terminal`
}

function shortID(id: string | undefined) {
  return id ? `${id.slice(0, 8)}...` : "-"
}

function parseCommand(value: string) {
  const trimmed = value.trim()
  if (!trimmed) {
    return []
  }
  if (trimmed.startsWith("[") && trimmed.endsWith("]")) {
    try {
      const parsed = JSON.parse(trimmed)
      return Array.isArray(parsed) ? parsed.map(String) : [trimmed]
    } catch {
      return [trimmed]
    }
  }
  if (trimmed.startsWith("sh -c ")) {
    return ["sh", "-c", trimmed.slice(6).replace(/^['"]|['"]$/g, "")]
  }
  return trimmed.split(/\s+/)
}

function stringValue(value: FormDataEntryValue | undefined) {
  return typeof value === "string" ? value.trim() : ""
}

function compactObject<T extends Record<string, unknown>>(value: T) {
  return Object.fromEntries(
    Object.entries(value).filter(([, entry]) => {
      if (entry === undefined || entry === null || entry === "") {
        return false
      }
      if (Array.isArray(entry) && entry.length === 0) {
        return false
      }
      if (entry === "global" || entry === "default") {
        return false
      }
      return true
    }),
  )
}

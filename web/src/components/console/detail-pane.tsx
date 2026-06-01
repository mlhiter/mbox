import { useState } from "react"
import { Box, FlaskConical, Layers3, Search, SquareTerminal, X } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
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
import type {
  AuditEvent,
  Project,
  ProjectCredential,
  ProjectPolicy,
  ProjectQuotaPolicy,
  ProjectUsage,
  SandboxResourceRequestUsage,
  Sandbox,
  Selection,
  Template,
} from "@/types"

const emptySelectionCopy = {
  title: "No resource selected",
  body: "Select a row to see what it launches, where it runs, and what you can do next.",
  detail: "The inspector follows the selected project, environment, or sandbox.",
}

export function DetailPane({
  selection,
  projects,
  projectAuditEvents,
  projectCredentials,
  projectPolicies,
  projectQuotaPolicies,
  projectUsage,
  templates,
  sandboxes,
  onValidateTemplate,
  onOpenSandboxWorkspace,
  onRefreshProjectAuditEvents,
  onClear,
}: {
  selection: Selection | null
  projects: Project[]
  projectAuditEvents: Record<string, AuditEvent[]>
  projectCredentials: Record<string, ProjectCredential[]>
  projectPolicies: Record<string, ProjectPolicy>
  projectQuotaPolicies: Record<string, ProjectQuotaPolicy>
  projectUsage: Record<string, ProjectUsage>
  templates: Template[]
  sandboxes: Sandbox[]
  onValidateTemplate?: (id: string) => Promise<void>
  onOpenSandboxWorkspace?: (id: string) => void
  onRefreshProjectAuditEvents?: (
    projectID: string,
    filters?: AuditEventFilters,
  ) => Promise<AuditEvent[]>
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
          <ProjectInspector
            key={(selected as Project).id}
            project={selected as Project}
            auditEvents={projectAuditEvents[(selected as Project).id] || []}
            credentials={projectCredentials[(selected as Project).id] || []}
            policy={projectPolicies[(selected as Project).id]}
            quotaPolicy={projectQuotaPolicies[(selected as Project).id]}
            usage={projectUsage[(selected as Project).id]}
            templates={templates}
            sandboxes={sandboxes}
            onRefreshAuditEvents={onRefreshProjectAuditEvents}
          />
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
  auditEvents,
  credentials,
  policy,
  quotaPolicy,
  usage,
  templates,
  sandboxes,
  onRefreshAuditEvents,
}: {
  project: Project
  auditEvents: AuditEvent[]
  credentials: ProjectCredential[]
  policy?: ProjectPolicy
  quotaPolicy?: ProjectQuotaPolicy
  usage?: ProjectUsage
  templates: Template[]
  sandboxes: Sandbox[]
  onRefreshAuditEvents?: (
    projectID: string,
    filters?: AuditEventFilters,
  ) => Promise<AuditEvent[]>
}) {
  const projectSandboxes = sandboxes.filter((sandbox) => sandbox.projectId === project.id)
  const [auditAction, setAuditAction] = useState("")
  const [auditActor, setAuditActor] = useState("")
  const [auditSource, setAuditSource] = useState("")
  const [auditRequestId, setAuditRequestId] = useState("")
  const [auditOperation, setAuditOperation] = useState("")
  const [auditSince, setAuditSince] = useState("")
  const [auditUntil, setAuditUntil] = useState("")
  const [auditLoading, setAuditLoading] = useState(false)
  const [auditError, setAuditError] = useState<string | null>(null)

  async function submitAuditFilters(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!onRefreshAuditEvents) {
      return
    }
    setAuditLoading(true)
    setAuditError(null)
    try {
      await onRefreshAuditEvents(project.id, {
        action: auditAction,
        actor: auditActor,
        source: auditSource,
        requestId: auditRequestId,
        operation: auditOperation,
        since: auditSince,
        until: auditUntil,
      })
    } catch (refreshError) {
      setAuditError(refreshError instanceof Error ? refreshError.message : "Could not load audit events")
    } finally {
      setAuditLoading(false)
    }
  }

  async function clearAuditFilters() {
    setAuditAction("")
    setAuditActor("")
    setAuditSource("")
    setAuditRequestId("")
    setAuditOperation("")
    setAuditSince("")
    setAuditUntil("")
    if (!onRefreshAuditEvents) {
      return
    }
    setAuditLoading(true)
    setAuditError(null)
    try {
      await onRefreshAuditEvents(project.id)
    } catch (refreshError) {
      setAuditError(refreshError instanceof Error ? refreshError.message : "Could not load audit events")
    } finally {
      setAuditLoading(false)
    }
  }

  return (
    <>
      <ResourceBadge icon={<Box />} label="Project scope" />
      <InspectorSummary
        title={project.defaultNamespace || "No namespace"}
        body={project.repositoryUrl || "No repository URL recorded."}
      />
      <InspectorGroup
        title="Launch policy"
        rows={[
          ["Enforcement", projectPolicyText(policy)],
          ["Image prefixes", formatPolicyList(policy?.allowedImagePrefixes)],
          ["Runtime identities", formatPolicyList(policy?.allowedServiceAccounts)],
          ["Secret refs", formatPolicyList(policy?.allowedSecretRefs)],
        ]}
      />
      <InspectorGroup
        title="Quota policy"
        rows={[
          ["Enforcement", projectQuotaPolicyText(quotaPolicy)],
          ["Active sandboxes", formatQuotaLimit(quotaPolicy?.maxActiveSandboxes, usage?.sandboxes.active, "active")],
          ["Retained artifact bytes", formatByteQuotaLimit(quotaPolicy?.maxRetainedArtifactBytes, usage?.artifacts.retainedBytes)],
        ]}
      />
      <InspectorGroup
        title="Credential refs"
        rows={credentialRows(credentials)}
      />
      <InspectorGroup
        title="Usage"
        rows={usageRows(usage, projectSandboxes.length)}
      />
      <AuditEventGroup
        events={auditEvents}
        action={auditAction}
        actor={auditActor}
        source={auditSource}
        requestId={auditRequestId}
        operation={auditOperation}
        since={auditSince}
        until={auditUntil}
        loading={auditLoading}
        error={auditError}
        onActionChange={setAuditAction}
        onActorChange={setAuditActor}
        onSourceChange={setAuditSource}
        onRequestIdChange={setAuditRequestId}
        onOperationChange={setAuditOperation}
        onSinceChange={setAuditSince}
        onUntilChange={setAuditUntil}
        onSubmit={submitAuditFilters}
        onClear={() => void clearAuditFilters()}
      />
      <InspectorGroup
        title="Output"
        rows={[
          ["Artifacts", usage ? String(usage.artifacts.total) : "-"],
          ["Retained bytes", usage ? formatBytes(usage.artifacts.retainedBytes) : "-"],
          ["Template requests", usage ? templateRequestSummary(usage) : "-"],
        ]}
      />
      <InspectorGroup
        title="Launch defaults"
        rows={[
          ["Default environment", templateName(project.defaultTemplateId, templates)],
          ["Active sandboxes", String(usage?.sandboxes.active ?? projectSandboxes.length)],
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

type AuditEventFilters = {
  action?: string
  actor?: string
  source?: string
  requestId?: string
  operation?: string
  since?: string
  until?: string
}

function projectPolicyText(policy: ProjectPolicy | undefined) {
  if (policy?.enforcement === "enforced") {
    return "Enforced"
  }
  return "Disabled"
}

function projectQuotaPolicyText(policy: ProjectQuotaPolicy | undefined) {
  if (policy?.enforcement === "enforced") {
    return "Enforced"
  }
  return "Disabled"
}

function formatQuotaLimit(limit: number | undefined, current: number | undefined, unit: string) {
  if (limit === undefined) {
    return "-"
  }
  return `${current ?? 0} / ${limit} ${unit}`
}

function formatByteQuotaLimit(limit: number | undefined, current: number | undefined) {
  if (limit === undefined) {
    return "-"
  }
  return `${formatBytes(current ?? 0)} / ${formatBytes(limit)}`
}

function formatPolicyList(values: string[] | undefined) {
  return values?.length ? values.join(", ") : "-"
}

function usageRows(usage: ProjectUsage | undefined, fallbackSandboxes: number): Array<[string, string]> {
  if (!usage) {
    return [
      ["Sandboxes", String(fallbackSandboxes)],
      ["Sessions", "-"],
      ["Tasks", "-"],
    ]
  }
  return [
    ["Sandboxes", `${usage.sandboxes.active} active · ${usage.sandboxes.running} running`],
    ["Declared requests", sandboxRequestSummary(usage.sandboxes.activeRequests)],
    ["Running requests", sandboxRequestSummary(usage.sandboxes.runningRequests)],
    ["Sessions", `${usage.runtimeSessions.total} total · ${usage.runtimeSessions.active} active`],
    ["Tasks", `${usage.executionTasks.total} total · ${usage.executionTasks.running} running`],
    ["Cleanup pending", String(usage.sandboxes.cleanupPending)],
  ]
}

function sandboxRequestSummary(requests: SandboxResourceRequestUsage | undefined) {
  if (!requests || requests.count === 0) {
    return "-"
  }
  const parts = [
    requests.cpu.total ? `${requests.cpu.total} CPU` : undefined,
    requests.memory.total ? `${requests.memory.total} memory` : undefined,
    requests.storage.total ? `${requests.storage.total} storage` : undefined,
  ].filter(Boolean)
  const gaps = requestGaps(requests)
  const summary = parts.length > 0 ? parts.join(" · ") : "No declared requests"
  return gaps ? `${summary} · ${gaps}` : summary
}

function requestGaps(requests: SandboxResourceRequestUsage) {
  const missing = requests.cpu.missing + requests.memory.missing + requests.storage.missing
  const invalid = requests.cpu.invalid + requests.memory.invalid + requests.storage.invalid
  const notes = []
  if (missing > 0) {
    notes.push(`${missing} missing`)
  }
  if (invalid > 0) {
    notes.push(`${invalid} invalid`)
  }
  return notes.join(" · ")
}

function templateRequestSummary(usage: ProjectUsage) {
  const storage = usage.templates.storageRequests?.[0]?.value
  const memory = usage.templates.memoryRequests?.[0]?.value
  const cpu = usage.templates.cpuRequests?.[0]?.value
  return [cpu, memory, storage].filter(Boolean).join(" · ") || "-"
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

function credentialRows(credentials: ProjectCredential[]): Array<[string, string]> {
  if (credentials.length === 0) {
    return [["Registered", "-"]]
  }
  return credentials.slice(0, 4).map((credential) => [
    credential.name,
    `${credential.type}${credential.target ? ` · ${credential.target}` : ""} · secret:${credential.secretRef.name}`,
  ])
}

function auditEventRows(events: AuditEvent[]) {
  if (events.length === 0) {
    return [["Events", "-"]]
  }
  return events.slice(0, 6).flatMap((event) => {
    const subject = event.resourceName || event.resourceType
    const actor = [event.actor || "unknown actor", event.source || "unknown source"].join(" · ")
    const metadata = event.metadata || {}
    const requestId = typeof metadata.requestId === "string" ? metadata.requestId : ""
    const operation = typeof metadata.operation === "string" ? metadata.operation : ""
    const trace = [operation ? `op:${operation}` : "", requestId ? `req:${requestId}` : ""]
      .filter(Boolean)
      .join(" · ")
    return [
      [event.action, `${subject}${event.createdAt ? ` · ${formatDateTime(event.createdAt)}` : ""}`],
      ["Actor/source", actor],
      ...(trace ? ([["Trace", trace]] as Array<[string, string]>) : []),
    ] as Array<[string, string]>
  })
}

function AuditEventGroup({
  events,
  action,
  actor,
  source,
  requestId,
  operation,
  since,
  until,
  loading,
  error,
  onActionChange,
  onActorChange,
  onSourceChange,
  onRequestIdChange,
  onOperationChange,
  onSinceChange,
  onUntilChange,
  onSubmit,
  onClear,
}: {
  events: AuditEvent[]
  action: string
  actor: string
  source: string
  requestId: string
  operation: string
  since: string
  until: string
  loading: boolean
  error: string | null
  onActionChange: (value: string) => void
  onActorChange: (value: string) => void
  onSourceChange: (value: string) => void
  onRequestIdChange: (value: string) => void
  onOperationChange: (value: string) => void
  onSinceChange: (value: string) => void
  onUntilChange: (value: string) => void
  onSubmit: (event: React.FormEvent<HTMLFormElement>) => void
  onClear: () => void
}) {
  return (
    <section className="detail-group audit-event-group">
      <div className="detail-group-head">
        <h3>Recent activity</h3>
        <Badge variant="secondary">{events.length} shown</Badge>
      </div>
      <form className="audit-event-filters" onSubmit={onSubmit}>
        <div>
          <Label htmlFor="audit-filter-action">Action</Label>
          <Input
            id="audit-filter-action"
            value={action}
            onChange={(event) => onActionChange(event.target.value)}
            placeholder="policy.denied"
          />
        </div>
        <div>
          <Label htmlFor="audit-filter-actor">Actor</Label>
          <Input
            id="audit-filter-actor"
            value={actor}
            onChange={(event) => onActorChange(event.target.value)}
            placeholder="agent-runner"
          />
        </div>
        <div>
          <Label htmlFor="audit-filter-source">Source</Label>
          <Input
            id="audit-filter-source"
            value={source}
            onChange={(event) => onSourceChange(event.target.value)}
            placeholder="sdk"
          />
        </div>
        <div>
          <Label htmlFor="audit-filter-request-id">Request ID</Label>
          <Input
            id="audit-filter-request-id"
            value={requestId}
            onChange={(event) => onRequestIdChange(event.target.value)}
            placeholder="req-..."
          />
        </div>
        <div>
          <Label htmlFor="audit-filter-operation">Operation</Label>
          <Input
            id="audit-filter-operation"
            value={operation}
            onChange={(event) => onOperationChange(event.target.value)}
            placeholder="sandbox.launch"
          />
        </div>
        <div>
          <Label htmlFor="audit-filter-since">Since</Label>
          <Input
            id="audit-filter-since"
            value={since}
            onChange={(event) => onSinceChange(event.target.value)}
            placeholder="2026-06-01T00:00:00Z"
          />
        </div>
        <div>
          <Label htmlFor="audit-filter-until">Until</Label>
          <Input
            id="audit-filter-until"
            value={until}
            onChange={(event) => onUntilChange(event.target.value)}
            placeholder="2026-06-01T23:59:59Z"
          />
        </div>
        <div className="audit-event-filter-actions">
          <Button type="submit" size="sm" disabled={loading}>
            <Search data-icon="inline-start" />
            Filter
          </Button>
          <Button type="button" variant="outline" size="sm" disabled={loading} onClick={onClear}>
            Clear
          </Button>
        </div>
      </form>
      {error ? <p className="audit-event-error">{error}</p> : null}
      <dl className="kv audit-event-list">
        {auditEventRows(events).map(([key, value], index) => (
          <div key={`${key}-${index}`}>
            <dt>{key}</dt>
            <dd>{String(value || "-")}</dd>
          </div>
        ))}
      </dl>
    </section>
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

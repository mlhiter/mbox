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
  ProjectAuthorizationDecision,
  ProjectCredential,
  ProjectMember,
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
  projectAuthorizations,
  projectMembers,
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
  projectAuthorizations: Record<string, ProjectAuthorizationDecision[]>
  projectMembers: Record<string, ProjectMember[]>
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
            authorizations={projectAuthorizations[(selected as Project).id]}
            members={projectMembers[(selected as Project).id] || []}
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
  authorizations,
  members,
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
  authorizations?: ProjectAuthorizationDecision[]
  members: ProjectMember[]
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
  const [auditReason, setAuditReason] = useState("")
  const [auditSince, setAuditSince] = useState("")
  const [auditUntil, setAuditUntil] = useState("")
  const [auditLoading, setAuditLoading] = useState(false)
  const [auditError, setAuditError] = useState<string | null>(null)

  async function loadAuditEvents(filters?: AuditEventFilters) {
    if (!onRefreshAuditEvents) {
      return
    }
    setAuditLoading(true)
    setAuditError(null)
    try {
      await onRefreshAuditEvents(project.id, filters)
    } catch (refreshError) {
      setAuditError(refreshError instanceof Error ? refreshError.message : "Could not load audit events")
    } finally {
      setAuditLoading(false)
    }
  }

  async function submitAuditFilters(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await loadAuditEvents({
      action: auditAction,
      actor: auditActor,
      source: auditSource,
      requestId: auditRequestId,
      operation: auditOperation,
      reason: auditReason,
      since: auditSince,
      until: auditUntil,
    })
  }

  async function clearAuditFilters() {
    setAuditAction("")
    setAuditActor("")
    setAuditSource("")
    setAuditRequestId("")
    setAuditOperation("")
    setAuditReason("")
    setAuditSince("")
    setAuditUntil("")
    await loadAuditEvents()
  }

  async function applyPolicyDeniedGroupFilter(filters: Pick<AuditEventFilters, "operation" | "reason">) {
    setAuditAction("policy.denied")
    setAuditActor("")
    setAuditSource("")
    setAuditRequestId("")
    setAuditOperation(filters.operation || "")
    setAuditReason(filters.reason || "")
    setAuditSince("")
    setAuditUntil("")
    await loadAuditEvents({
      action: "policy.denied",
      operation: filters.operation,
      reason: filters.reason,
    })
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
        title="Authorization preflight"
        rows={authorizationRows(authorizations)}
      />
      <InspectorGroup
        title="Members"
        rows={memberRows(members)}
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
        reason={auditReason}
        since={auditSince}
        until={auditUntil}
        loading={auditLoading}
        error={auditError}
        onActionChange={setAuditAction}
        onActorChange={setAuditActor}
        onSourceChange={setAuditSource}
        onRequestIdChange={setAuditRequestId}
        onOperationChange={setAuditOperation}
        onReasonChange={setAuditReason}
        onSinceChange={setAuditSince}
        onUntilChange={setAuditUntil}
        onSubmit={submitAuditFilters}
        onClear={() => void clearAuditFilters()}
        onApplyPolicyDeniedGroup={(filters) => void applyPolicyDeniedGroupFilter(filters)}
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
  reason?: string
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

function memberRows(members: ProjectMember[]): Array<[string, string]> {
  if (members.length === 0) {
    return [["Registered", "-"]]
  }
  return members.slice(0, 5).map((member) => [
    member.principal,
    `${member.role} · ${member.principalType}`,
  ])
}

function authorizationRows(authorizations?: ProjectAuthorizationDecision[]): Array<[string, string]> {
  if (!authorizations?.length) {
    return [["Actions", "Unavailable"]]
  }
  const [first] = authorizations
  return [
    ...authorizations.map((authorization) => [
      authorization.action,
      `${authorization.evaluation}${authorization.enforced ? " · enforced" : " · not enforced"} · roles:${authorization.requiredRoles.join(", ") || "-"}`,
    ] as [string, string]),
    ["Caller", `${first.caller.mode} · ${first.caller.principal}`],
    ["RBAC trust", first.caller.rbacTrusted ? "Trusted project identity" : "Not trusted"],
    ["Matched member", first.matchedMember ? `${first.matchedMember.role} · ${first.matchedMember.principal}` : "-"],
    ["Members", `${first.memberCount} registered`],
  ]
}

type AuditEventRow = {
  key: string
  value: string
  tone?: "muted" | "denied"
}

type PolicyDeniedGroup = {
  key: string
  label: string
  detail: string
  operation?: string
  reason?: string
  count: number
  latest?: string
}

function policyDeniedGroups(events: AuditEvent[]): PolicyDeniedGroup[] {
  const groups = new Map<string, PolicyDeniedGroup>()
  for (const event of events) {
    if (event.action !== "policy.denied") {
      continue
    }
    const metadata = auditMetadata(event)
    const operation = auditMetadataString(metadata, "operation")
    const reason = auditMetadataString(metadata, "reason")
    const key = `${operation || "unknown"}\u0000${reason || ""}`
    const existing = groups.get(key)
    if (existing) {
      existing.count += 1
      if (isLaterAuditEvent(event.createdAt, existing.latest)) {
        existing.latest = event.createdAt
      }
      continue
    }
    groups.set(key, {
      key,
      label: operation || "policy.denied",
      detail: reason || "No reason metadata",
      operation: operation || undefined,
      reason: reason || undefined,
      count: 1,
      latest: event.createdAt,
    })
  }
  return Array.from(groups.values())
    .sort((a, b) => {
      if (b.count !== a.count) {
        return b.count - a.count
      }
      return auditTimeValue(b.latest) - auditTimeValue(a.latest)
    })
    .slice(0, 4)
}

function isLaterAuditEvent(next: string | undefined, current: string | undefined) {
  return auditTimeValue(next) > auditTimeValue(current)
}

function auditTimeValue(value: string | undefined) {
  if (!value) {
    return 0
  }
  const parsed = Date.parse(value)
  return Number.isFinite(parsed) ? parsed : 0
}

function auditEventRows(events: AuditEvent[]): AuditEventRow[] {
  if (events.length === 0) {
    return [{ key: "Events", value: "-" }]
  }
  return events.slice(0, 6).flatMap((event) => {
    const subject = event.resourceName || event.resourceType
    const actor = [event.actor || "unknown actor", event.source || "unknown source"].join(" · ")
    const metadata = auditMetadata(event)
    const trace = auditTrace(metadata)
    const rows: AuditEventRow[] = [
      { key: event.action, value: `${subject}${event.createdAt ? ` · ${formatDateTime(event.createdAt)}` : ""}` },
      ...policyDeniedAuditRows(event, metadata),
      { key: "Actor/source", value: actor, tone: "muted" },
    ]
    if (trace) {
      rows.push({ key: "Trace", value: trace, tone: "muted" })
    }
    return rows
  })
}

function auditMetadata(event: AuditEvent): Record<string, unknown> {
  if (!event.metadata || typeof event.metadata !== "object" || Array.isArray(event.metadata)) {
    return {}
  }
  return event.metadata
}

function auditMetadataString(metadata: Record<string, unknown>, key: string): string {
  const value = metadata[key]
  return typeof value === "string" ? value.trim() : ""
}

function auditTrace(metadata: Record<string, unknown>): string {
  const requestId = auditMetadataString(metadata, "requestId")
  const operation = auditMetadataString(metadata, "operation")
  const reason = auditMetadataString(metadata, "reason")
  return [operation ? `op:${operation}` : "", reason ? `reason:${reason}` : "", requestId ? `req:${requestId}` : ""]
    .filter(Boolean)
    .join(" · ")
}

function policyDeniedAuditRows(event: AuditEvent, metadata: Record<string, unknown>): AuditEventRow[] {
  if (event.action !== "policy.denied") {
    return []
  }
  const operation = auditMetadataString(metadata, "operation")
  const reason = auditMetadataString(metadata, "reason")
  const authorizationAction = auditMetadataString(metadata, "authorizationAction")
  const callerMode = auditMetadataString(metadata, "callerMode")
  const callerPrincipalType = auditMetadataString(metadata, "callerPrincipalType")
  const callerPrincipal = auditMetadataString(metadata, "callerPrincipal")
  const principalType = auditMetadataString(metadata, "principalType")
  const principal = auditMetadataString(metadata, "principal")
  const role = auditMetadataString(metadata, "role")
  const templateName = auditMetadataString(metadata, "templateName")
  const templateId = auditMetadataString(metadata, "templateId")
  const image = auditMetadataString(metadata, "image")
  const serviceAccountName = auditMetadataString(metadata, "serviceAccountName")
  const sandboxId = auditMetadataString(metadata, "sandboxId")
  const artifactKind = auditMetadataString(metadata, "artifactKind")
  const policyKind = auditMetadataString(metadata, "policyKind")
  const enforcement = auditMetadataString(metadata, "enforcement")
  const maxActiveSandboxes = typeof metadata.maxActiveSandboxes === "number" && Number.isFinite(metadata.maxActiveSandboxes)
    ? String(metadata.maxActiveSandboxes)
    : ""
  const maxRetainedArtifactBytes = typeof metadata.maxRetainedArtifactBytes === "number" && Number.isFinite(metadata.maxRetainedArtifactBytes)
    ? formatBytes(metadata.maxRetainedArtifactBytes)
    : ""
  const incomingBytes = typeof metadata.incomingBytes === "number" && Number.isFinite(metadata.incomingBytes)
    ? formatBytes(metadata.incomingBytes)
    : ""
  const requestShape = [
    templateName || templateId ? `template:${templateName || templateId}` : "",
    image ? `image:${image}` : "",
    serviceAccountName ? `sa:${serviceAccountName}` : "",
    policyKind ? `policy:${policyKind}` : "",
    enforcement ? `enforcement:${enforcement}` : "",
  ].filter(Boolean)
  const resourceHints = [
    sandboxId ? `sandbox:${sandboxId}` : "",
    artifactKind ? `artifact:${artifactKind}` : "",
    maxActiveSandboxes ? `max sandboxes:${maxActiveSandboxes}` : "",
    maxRetainedArtifactBytes ? `max retained:${maxRetainedArtifactBytes}` : "",
    incomingBytes ? `incoming:${incomingBytes}` : "",
  ].filter(Boolean)
  const caller = [
    callerPrincipalType && callerPrincipal ? `${callerPrincipalType}:${callerPrincipal}` : callerPrincipal,
    callerMode,
  ].filter(Boolean)
  const member = [
    principalType && principal ? `${principalType}:${principal}` : principal,
    role ? `role:${role}` : "",
  ].filter(Boolean)
  return [
    {
      key: "Policy denial",
      value: [operation || "policy", reason ? `reason:${reason}` : ""].filter(Boolean).join(" · "),
      tone: "denied",
    },
    ...(authorizationAction ? [{ key: "Authorization", value: authorizationAction, tone: "muted" as const }] : []),
    ...(caller.length ? [{ key: "Caller", value: caller.join(" · "), tone: "muted" as const }] : []),
    ...(member.length ? [{ key: "Member", value: member.join(" · "), tone: "muted" as const }] : []),
    ...(requestShape.length ? [{ key: "Request", value: requestShape.join(" · "), tone: "muted" as const }] : []),
    ...(resourceHints.length ? [{ key: "Resource", value: resourceHints.join(" · "), tone: "muted" as const }] : []),
  ]
}

function AuditEventGroup({
  events,
  action,
  actor,
  source,
  requestId,
  operation,
  reason,
  since,
  until,
  loading,
  error,
  onApplyPolicyDeniedGroup,
  onActionChange,
  onActorChange,
  onSourceChange,
  onRequestIdChange,
  onOperationChange,
  onReasonChange,
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
  reason: string
  since: string
  until: string
  loading: boolean
  error: string | null
  onApplyPolicyDeniedGroup: (filters: Pick<AuditEventFilters, "operation" | "reason">) => void
  onActionChange: (value: string) => void
  onActorChange: (value: string) => void
  onSourceChange: (value: string) => void
  onRequestIdChange: (value: string) => void
  onOperationChange: (value: string) => void
  onReasonChange: (value: string) => void
  onSinceChange: (value: string) => void
  onUntilChange: (value: string) => void
  onSubmit: (event: React.FormEvent<HTMLFormElement>) => void
  onClear: () => void
}) {
  const deniedGroups = policyDeniedGroups(events)
  return (
    <section className="detail-group audit-event-group">
      <div className="detail-group-head">
        <h3>Recent activity</h3>
        <Badge variant="secondary">{events.length} shown</Badge>
      </div>
      {deniedGroups.length ? (
        <div className="policy-denial-groups" aria-label="Policy denial groups">
          {deniedGroups.map((group) => (
            <button
              key={group.key}
              type="button"
              className="policy-denial-group"
              disabled={loading}
              onClick={() => onApplyPolicyDeniedGroup({ operation: group.operation, reason: group.reason })}
            >
              <span>
                <strong>{group.label}</strong>
                <small>{group.detail}</small>
              </span>
              <span className="policy-denial-group-meta">
                {group.count} {group.count === 1 ? "event" : "events"}
                {group.latest ? ` · latest ${formatDateTime(group.latest)}` : ""}
              </span>
            </button>
          ))}
        </div>
      ) : null}
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
          <Label htmlFor="audit-filter-reason">Reason</Label>
          <Input
            id="audit-filter-reason"
            value={reason}
            onChange={(event) => onReasonChange(event.target.value)}
            placeholder="active sandbox quota exceeded"
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
        {auditEventRows(events).map((row, index) => (
          <div
            key={`${row.key}-${index}`}
            className={cn(row.tone === "muted" && "audit-event-row-muted", row.tone === "denied" && "audit-event-row-denied")}
          >
            <dt>{row.key}</dt>
            <dd>{String(row.value || "-")}</dd>
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

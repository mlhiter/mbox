import {
  ArrowLeft,
  CheckCircle2,
  CircleAlert,
  CircleDashed,
  Play,
  Power,
  RefreshCw,
  XCircle,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import { StatusBadge } from "@/components/console/status-badge"
import { DeleteSandboxDialog } from "@/features/resources/delete-sandbox-dialog"
import { RuntimeWorkspace } from "@/features/runtime/runtime-workspace"
import {
  formatDateTime,
  projectName,
  runtimeText,
  sandboxValidationRun,
  templateEntrypoints,
  templateForSandbox,
  templateName,
  templatePersistence,
  templateResourcePreset,
  templateRuntimeType,
  templateUseCase,
} from "@/lib/resource-utils"
import type { Project, Sandbox, Template } from "@/types"

type SandboxDetailPageProps = {
  sandbox: Sandbox
  projects: Project[]
  templates: Template[]
  onBack: () => void
  onRefresh: (id: string) => Promise<Sandbox>
  onStart: (id: string) => Promise<Sandbox>
  onStop: (id: string) => Promise<Sandbox>
  onDecideValidation: (id: string, status: "passed" | "failed") => Promise<void>
  onDelete: (id: string) => Promise<void>
}

export function SandboxDetailFallback({
  sandboxId,
  loading,
  error,
  onBack,
  onRefresh,
}: {
  sandboxId: string | undefined
  loading: boolean
  error: string | null
  onBack: () => void
  onRefresh: () => Promise<void>
}) {
  return (
    <div className="sandbox-detail-page">
      <header className="sandbox-detail-hero">
        <div className="sandbox-detail-nav">
          <Button variant="outline" size="sm" onClick={onBack}>
            <ArrowLeft data-icon="inline-start" />
            Sandboxes
          </Button>
        </div>
        <div className="sandbox-detail-title">
          <div>
            <p className="eyebrow">Sandbox workspace</p>
            <h1 className="page-title">{loading ? "Loading sandbox" : "Sandbox unavailable"}</h1>
            <p className="page-note">
              {sandboxId ? `Sandbox ID ${sandboxId}` : "No sandbox ID was provided."}
            </p>
          </div>
          <StatusBadge status={loading ? "pending" : "failed"} />
        </div>
        <div className="sandbox-detail-actions">
          <Button variant="outline" size="sm" onClick={() => void onRefresh()} disabled={loading}>
            <RefreshCw data-icon="inline-start" />
            Refresh
          </Button>
        </div>
      </header>

      <section className="sandbox-detail-empty" aria-live="polite">
        <strong>{loading ? "Resolving workspace" : "Could not resolve this sandbox"}</strong>
        <span>
          {loading
            ? "mbox is loading projects, environments, and sandboxes before opening this workspace."
            : error || "The sandbox may have been deleted or filtered out of the current API response."}
        </span>
      </section>
    </div>
  )
}

export function SandboxDetailPage({
  sandbox,
  projects,
  templates,
  onBack,
  onRefresh,
  onStart,
  onStop,
  onDecideValidation,
  onDelete,
}: SandboxDetailPageProps) {
  const template = templateForSandbox(sandbox, templates)
  const validationRun = sandboxValidationRun(sandbox)
  const isStopped = sandbox.status === "stopped"
  const isDeleted = sandbox.status === "deleted"

  return (
    <div className="sandbox-detail-page">
      <header className="sandbox-detail-hero">
        <div className="sandbox-detail-nav">
          <Button variant="outline" size="sm" onClick={onBack}>
            <ArrowLeft data-icon="inline-start" />
            Sandboxes
          </Button>
        </div>
        <div className="sandbox-detail-title">
          <div>
            <p className="eyebrow">Sandbox workspace</p>
            <h1 className="page-title">{sandbox.name}</h1>
            <p className="page-note">
              {projectName(sandbox.projectId, projects)} · {templateName(sandbox.templateId, templates)}
            </p>
          </div>
          <StatusBadge status={sandbox.status} />
        </div>
        <div className="sandbox-detail-actions" aria-label={`Actions for ${sandbox.name}`}>
          <Button variant="outline" size="sm" onClick={() => void onRefresh(sandbox.id)}>
            <RefreshCw data-icon="inline-start" />
            Refresh
          </Button>
          {isStopped ? (
            <Button size="sm" onClick={() => void onStart(sandbox.id)}>
              <Play data-icon="inline-start" />
              Start
            </Button>
          ) : (
            <Button variant="outline" size="sm" disabled={isDeleted} onClick={() => void onStop(sandbox.id)}>
              <Power data-icon="inline-start" />
              Stop
            </Button>
          )}
          <DeleteSandboxDialog sandbox={sandbox} onDelete={onDelete} className="sandbox-detail-delete" />
        </div>
      </header>

      <section className="sandbox-detail-layout">
        <div className="sandbox-detail-main">
          <SandboxOverview sandbox={sandbox} template={template} projects={projects} templates={templates} />
          <WorkspaceReadiness sandbox={sandbox} template={template} validationRun={validationRun} />
          <RuntimeWorkspace sandbox={sandbox} onSandboxChange={onRefresh} />
        </div>
        <SandboxInspector
          sandbox={sandbox}
          template={template}
          projects={projects}
          templates={templates}
          validationRun={validationRun}
          onDecideValidation={onDecideValidation}
        />
      </section>
    </div>
  )
}

function WorkspaceReadiness({
  sandbox,
  template,
  validationRun,
}: {
  sandbox: Sandbox
  template: Template | undefined
  validationRun: ReturnType<typeof sandboxValidationRun>
}) {
  const runtimeProjected = sandbox.status === "running" && Boolean(sandbox.runtimeRef)
  const previewPorts = sandbox.ports?.length || 0
  const workspacePersistence = template?.storageRequest ? `Persistent ${template.storageRequest}` : "Ephemeral"
  const validationText = validationRun.isValidationRun
    ? validationRun.result
      ? validationRun.result === "passed"
        ? "Validation passed"
        : "Validation failed"
      : "Validation pending"
    : "Normal sandbox"
  const items = [
    {
      label: "Runtime record",
      value: runtimeProjected ? "Projected" : sandbox.status === "pending" ? "Starting" : "Unavailable",
      tone: runtimeProjected ? "success" : sandbox.status === "pending" ? "warning" : "neutral",
      detail: runtimeProjected
        ? runtimeText(sandbox.runtimeRef)
        : sandbox.runtimeRef
          ? `${sandbox.status} · ${runtimeText(sandbox.runtimeRef)}`
          : "Waiting for runtime projection",
    },
    {
      label: "Preview surface",
      value: previewPorts ? `${previewPorts} declared` : "No ports",
      tone: previewPorts ? "success" : "neutral",
      detail: previewPorts
        ? (sandbox.ports || []).map((port) => `${port.name || "port"}:${port.port}`).join(", ")
        : "Add ports from the Preview tab",
    },
    {
      label: "Workspace",
      value: workspacePersistence,
      tone: template?.storageRequest ? "success" : "neutral",
      detail: template ? templatePersistence(template) : "Environment unavailable",
    },
    {
      label: "Run intent",
      value: validationText,
      tone: validationRun.result === "failed" ? "danger" : validationRun.isValidationRun ? "warning" : "neutral",
      detail: template ? `${templateRuntimeType(template)} · ${templateUseCase(template)}` : "No environment shape",
    },
  ] as const

  return (
    <section className="workspace-readiness" aria-label="Workspace readiness">
      <div className="workspace-readiness-head">
        <div>
          <p className="eyebrow">Workspace readiness</p>
          <h2>Runtime checks</h2>
        </div>
        <span>{runtimeProjected ? "Projected" : "Blocked"}</span>
      </div>
      <div className="workspace-readiness-grid">
        {items.map((item) => (
          <div key={item.label} className={`workspace-readiness-item tone-${item.tone}`}>
            <ReadinessIcon tone={item.tone} />
            <div>
              <span>{item.label}</span>
              <strong>{item.value}</strong>
              <small>{item.detail}</small>
            </div>
          </div>
        ))}
      </div>
    </section>
  )
}

function ReadinessIcon({ tone }: { tone: "success" | "warning" | "danger" | "neutral" }) {
  if (tone === "success") {
    return <CheckCircle2 aria-hidden="true" />
  }
  if (tone === "danger") {
    return <XCircle aria-hidden="true" />
  }
  if (tone === "warning") {
    return <CircleAlert aria-hidden="true" />
  }
  return <CircleDashed aria-hidden="true" />
}

function SandboxOverview({
  sandbox,
  template,
  projects,
  templates,
}: {
  sandbox: Sandbox
  template: Template | undefined
  projects: Project[]
  templates: Template[]
}) {
  const previewPorts = sandbox.ports?.length ? sandbox.ports.map((port) => `${port.name || "port"}:${port.port}`).join(", ") : "No preview ports"
  return (
    <section className="sandbox-overview" aria-label="Sandbox overview">
      <OverviewCell label="Project" value={projectName(sandbox.projectId, projects)} />
      <OverviewCell label="Environment" value={templateName(sandbox.templateId, templates)} hint={template ? templateUseCase(template) : undefined} />
      <OverviewCell label="Runtime" value={runtimeText(sandbox.runtimeRef)} mono />
      <OverviewCell label="Preview ports" value={previewPorts} mono />
    </section>
  )
}

function OverviewCell({
  label,
  value,
  hint,
  mono = false,
}: {
  label: string
  value: string
  hint?: string
  mono?: boolean
}) {
  return (
    <div>
      <span>{label}</span>
      <strong className={mono ? "mono" : undefined}>{value}</strong>
      {hint ? <small>{hint}</small> : null}
    </div>
  )
}

function SandboxInspector({
  sandbox,
  template,
  projects,
  templates,
  validationRun,
  onDecideValidation,
}: {
  sandbox: Sandbox
  template: Template | undefined
  projects: Project[]
  templates: Template[]
  validationRun: ReturnType<typeof sandboxValidationRun>
  onDecideValidation: (id: string, status: "passed" | "failed") => Promise<void>
}) {
  const templateRows: Array<[string, string]> = template
    ? [
        ["Runtime", templateRuntimeType(template)],
        ["Use case", templateUseCase(template)],
        ["Size", templateResourcePreset(template)],
        ["Workspace", templatePersistence(template)],
        ["Preview ports", templateEntrypoints(template)],
        ["Image", template.image],
      ]
    : [["Environment", templateName(sandbox.templateId, templates)]]

  return (
    <aside className="sandbox-inspector" aria-label="Sandbox details">
      {validationRun.isValidationRun ? (
        <ValidationRunPanel
          sandbox={sandbox}
          templateName={templateName(validationRun.templateId, templates)}
          result={validationRun.result}
          decidedAt={validationRun.decidedAt}
          onDecideValidation={onDecideValidation}
        />
      ) : null}
      <InspectorGroup
        title="Identity"
        rows={[
          ["Name", sandbox.name],
          ["Key", sandbox.slug],
          ["Project", projectName(sandbox.projectId, projects)],
          ["Environment", templateName(sandbox.templateId, templates)],
          ["Created", formatDateTime(sandbox.createdAt)],
        ]}
      />
      <InspectorGroup
        title="Runtime"
        rows={[
          ["Status", sandbox.status],
          ["Runtime resource", runtimeText(sandbox.runtimeRef)],
          ["Namespace", sandbox.namespace],
          ["Runtime identity", sandbox.serviceAccountName],
        ]}
      />
      <InspectorGroup title="Environment shape" rows={templateRows} />
    </aside>
  )
}

function ValidationRunPanel({
  sandbox,
  templateName,
  result,
  decidedAt,
  onDecideValidation,
}: {
  sandbox: Sandbox
  templateName: string
  result: "passed" | "failed" | undefined
  decidedAt: string | undefined
  onDecideValidation: (id: string, status: "passed" | "failed") => Promise<void>
}) {
  return (
    <section className="validation-run-panel" aria-label="Validation run">
      <div className="validation-run-head">
        <span>Validation run</span>
        <strong>{result ? (result === "passed" ? "Validated" : "Failed") : "Awaiting decision"}</strong>
      </div>
      <p>
        This sandbox was launched to validate {templateName}. Mark the result after checking terminal,
        preview ports, logs, and artifacts.
      </p>
      {decidedAt ? <small>Decided {formatDateTime(decidedAt)}</small> : null}
      <div className="validation-run-actions">
        <Button size="sm" onClick={() => void onDecideValidation(sandbox.id, "passed")}>
          <CheckCircle2 data-icon="inline-start" />
          Passed
        </Button>
        <Button
          variant="outline"
          size="sm"
          className="validation-run-failed"
          onClick={() => void onDecideValidation(sandbox.id, "failed")}
        >
          <XCircle data-icon="inline-start" />
          Failed
        </Button>
      </div>
    </section>
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
    <section className="inspector-group">
      <h2>{title}</h2>
      <dl>
        {rows.map(([key, value]) => (
          <div key={key}>
            <dt>{key}</dt>
            <dd>{value || "-"}</dd>
          </div>
        ))}
      </dl>
    </section>
  )
}

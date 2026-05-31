import { AlertTriangle, CheckCircle2, ShieldCheck, XCircle } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import type { BoundaryCheck, BoundarySummary } from "@/types"

type RuntimeBoundaryProps = {
  boundary: BoundarySummary | null
  loading: boolean
  error: string | null
  onRefresh: () => void
}

export function RuntimeBoundary({ boundary, loading, error, onRefresh }: RuntimeBoundaryProps) {
  if (error) {
    return (
      <div className="runtime-boundary">
        <div className="runtime-boundary-empty">
          <strong>Boundary summary unavailable</strong>
          <span>{error}</span>
          <Button type="button" variant="outline" size="sm" onClick={onRefresh}>Retry</Button>
        </div>
      </div>
    )
  }
  if (!boundary) {
    return (
      <div className="runtime-boundary">
        <div className="runtime-boundary-empty">
          <strong>{loading ? "Loading boundary" : "No boundary summary"}</strong>
          <span>{loading ? "Resolving namespace, identity, secret, network, and cleanup state." : "Refresh this workspace to load policy boundaries."}</span>
        </div>
      </div>
    )
  }

  return (
    <div className="runtime-boundary">
      <div className="boundary-summary-grid">
        <BoundaryFact label="Namespace" value={boundary.namespace || "Not resolved"} />
        <BoundaryFact label="Runtime identity" value={boundary.serviceAccountName || "Not resolved"} />
        <BoundaryFact label="Token automount" value={boundary.serviceAccountTokenAutomount ? "Enabled" : "Disabled"} />
        <BoundaryFact label="Network" value={boundary.networkPolicyProjection} />
        <BoundaryFact label="Secrets" value={boundary.secretProjection} />
        <BoundaryFact label="Credentials" value={boundary.credentialProjection} />
        <BoundaryFact label="Lifecycle" value={boundary.lifecyclePolicyProjection} />
        <BoundaryFact label="Launch policy" value={boundary.policyEnforcement} />
      </div>
      <section className="boundary-checks" aria-label="Boundary checks">
        {boundary.checks.map((check) => (
          <BoundaryCheckRow key={check.id} check={check} />
        ))}
      </section>
      <section className="boundary-columns" aria-label="Boundary details">
        <BoundaryList title="Controller permissions" items={boundary.controllerPermissions} />
        <BoundaryList title="Runtime access" items={boundary.runtimeAccess} />
        <BoundaryList title="Cleanup" items={boundary.cleanup} />
      </section>
      <section className="boundary-template-shape" aria-label="Template boundary shape">
        <div>
          <span>Image</span>
          <strong className="mono">{boundary.image}</strong>
        </div>
        <div>
          <span>Working directory</span>
          <strong className="mono">{boundary.workingDir}</strong>
        </div>
        <div>
          <span>Storage</span>
          <strong>{boundary.storageRequest || "Ephemeral"}</strong>
        </div>
        <div>
          <span>Preview ports</span>
          <strong>{boundary.previewPorts?.length ? boundary.previewPorts.map((port) => `${port.name}:${port.port}`).join(", ") : "None"}</strong>
        </div>
        <div>
          <span>Allowed images</span>
          <strong className="mono">{boundary.allowedImagePrefixes?.length ? boundary.allowedImagePrefixes.join(", ") : "Unrestricted"}</strong>
        </div>
        <div>
          <span>Credential refs</span>
          <strong>{boundary.credentialRefs?.length ? boundary.credentialRefs.map((ref) => `${ref.type}:${ref.slug}`).join(", ") : "None"}</strong>
        </div>
      </section>
    </div>
  )
}

function BoundaryFact({ label, value }: { label: string; value: string }) {
  return (
    <div className="boundary-fact">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  )
}

function BoundaryCheckRow({ check }: { check: BoundaryCheck }) {
  return (
    <article className={`boundary-check boundary-check-${check.status}`}>
      <BoundaryIcon status={check.status} />
      <div>
        <div className="boundary-check-head">
          <strong>{check.label}</strong>
          <Badge variant="secondary">{check.status}</Badge>
        </div>
        <p>{check.message}</p>
        {check.evidence?.length ? <small>{check.evidence.join(" · ")}</small> : null}
      </div>
    </article>
  )
}

function BoundaryIcon({ status }: { status: BoundaryCheck["status"] }) {
  if (status === "pass") {
    return <CheckCircle2 aria-hidden="true" />
  }
  if (status === "fail") {
    return <XCircle aria-hidden="true" />
  }
  return <AlertTriangle aria-hidden="true" />
}

function BoundaryList({ title, items }: { title: string; items: string[] }) {
  return (
    <div className="boundary-list">
      <h3>
        <ShieldCheck aria-hidden="true" />
        {title}
      </h3>
      <ul>
        {items.map((item) => (
          <li key={item}>{item}</li>
        ))}
      </ul>
    </div>
  )
}

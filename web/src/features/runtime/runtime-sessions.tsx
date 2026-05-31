import { useMemo } from "react"
import { RefreshCw } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  formatClockTime,
  formatDuration,
  shortID,
} from "@/lib/resource-utils"
import { RuntimeSectionHead } from "@/features/runtime/runtime-section-head"
import type { RuntimeSession } from "@/types"

type RuntimeSessionsProps = {
  sessions: RuntimeSession[]
  loading: boolean
  onRefresh: () => Promise<void>
}

export function RuntimeSessions({ sessions, loading, onRefresh }: RuntimeSessionsProps) {
  const sortedSessions = useMemo(
    () => [...sessions].sort((left, right) => (right.startedAt || "").localeCompare(left.startedAt || "")),
    [sessions],
  )
  const sessionSummary = useMemo(
    () => ({
      total: sessions.length,
      active: sessions.filter((session) => session.status === "active").length,
      ended: sessions.filter((session) => session.status === "ended").length,
      failed: sessions.filter((session) => session.status === "failed").length,
    }),
    [sessions],
  )

  return (
    <div className="runtime-sessions">
      <RuntimeSectionHead eyebrow="Access" title="Sessions" />
      <div className="runtime-ledger-head runtime-session-ledger-head" aria-label="Session summary">
        <div>
          <span>Total</span>
          <strong>{sessionSummary.total}</strong>
        </div>
        <div>
          <span>Active</span>
          <strong>{sessionSummary.active}</strong>
        </div>
        <div>
          <span>Ended</span>
          <strong>{sessionSummary.ended}</strong>
        </div>
        <div>
          <span>Failed</span>
          <strong>{sessionSummary.failed}</strong>
        </div>
      </div>
      <div className="runtime-session-toolbar">
        <Button type="button" variant="outline" disabled={loading} onClick={() => void onRefresh()}>
          <RefreshCw data-icon="inline-start" />
          Refresh
        </Button>
      </div>
      {sortedSessions.length === 0 ? (
        <p>No runtime sessions have attached to this sandbox.</p>
      ) : (
        <ul>
          {sortedSessions.map((session) => (
            <li key={session.id}>
              <div className="runtime-session-row-head">
                <div>
                  <strong>{session.type}</strong>
                  <span className="runtime-session-meta">
                    {shortID(session.id)} · {session.client || "unknown client"} · {formatClockTime(session.startedAt)} ·{" "}
                    {formatDuration(session.startedAt, session.endedAt)}
                  </span>
                </div>
                <div className="runtime-session-row-actions">
                  <span className={`runtime-status-badge tone-${sessionStatusTone(session.status)}`}>
                    {session.status}
                  </span>
                  <time>{formatClockTime(session.endedAt || session.updatedAt || session.startedAt)}</time>
                </div>
              </div>
              <small>
                {session.runtimeRef
                  ? `${session.runtimeRef.kind} ${session.runtimeRef.namespace}/${session.runtimeRef.name}`
                  : "No runtime reference"}
              </small>
              {session.userAgent ? <code className="runtime-session-user-agent">{session.userAgent}</code> : null}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

function sessionStatusTone(status: RuntimeSession["status"]) {
  if (status === "ended") {
    return "success"
  }
  if (status === "failed") {
    return "danger"
  }
  if (status === "active") {
    return "warning"
  }
  return "neutral"
}

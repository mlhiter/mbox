import { RuntimeSectionHead } from "@/features/runtime/runtime-section-head"
import type { RuntimeEvent } from "@/types"

export function RuntimeLogs({ logs }: { logs: string }) {
  return (
    <div className="runtime-observe">
      <RuntimeSectionHead eyebrow="Observe" title="Logs" />
      <pre>{logs || "No logs loaded."}</pre>
    </div>
  )
}

export function RuntimeEvents({ events }: { events: RuntimeEvent[] }) {
  return (
    <div className="runtime-observe">
      <RuntimeSectionHead eyebrow="Observe" title="Events" />
      {events.length === 0 ? (
        <p>No events loaded.</p>
      ) : (
        <ul>
          {events.map((event, index) => (
            <li key={`${event.reason}-${index}`}>
              <strong>{event.reason || event.type || "Event"}</strong>
              <span>{event.message || "-"}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

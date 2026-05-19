import { RuntimeSectionHead } from "@/features/runtime/runtime-section-head"
import type { RuntimeStorage } from "@/types"

export function RuntimeStoragePanel({ storage }: { storage: RuntimeStorage[] }) {
  return (
    <div className="runtime-storage">
      <RuntimeSectionHead eyebrow="Workspace" title="Storage" />
      {storage.length === 0 ? (
        <p>No persistent workspace mount resolved.</p>
      ) : (
        <ul>
          {storage.map((item) => (
            <li key={`${item.name}-${item.mountPath}`}>
              <div>
                <strong>{item.name}</strong>
                <span>{item.mountPath}</span>
              </div>
              <div>
                <span>Claim</span>
                <strong>{item.claimName || "-"}</strong>
              </div>
              <div>
                <span>Status</span>
                <strong>{[item.phase, item.capacity, item.storageClassName].filter(Boolean).join(" · ") || "-"}</strong>
              </div>
              {item.message ? <small>{item.message}</small> : null}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

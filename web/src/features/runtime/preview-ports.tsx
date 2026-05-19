import { ExternalLink } from "lucide-react"
import { Button } from "@/components/ui/button"
import { RuntimeSectionHead } from "@/features/runtime/runtime-section-head"
import type { PreviewPort } from "@/types"

export function PreviewPorts({ ports }: { ports: PreviewPort[] }) {
  return (
    <div className="preview-ports">
      <RuntimeSectionHead eyebrow="Preview" title="Ports" />
      {ports.length === 0 ? (
        <p>No preview ports declared.</p>
      ) : (
        <ul>
          {ports.map((port) => (
            <li key={`${port.name}-${port.port}`}>
              <div>
                <strong>{port.name}</strong>
                <span>
                  {port.protocol || "TCP"} · {port.port}
                </span>
                {port.message ? <small>{port.message}</small> : null}
              </div>
              {port.available && port.previewUrl ? (
                <Button asChild size="sm" variant="outline">
                  <a href={port.previewUrl} target="_blank" rel="noreferrer">
                    <ExternalLink data-icon="inline-start" />
                    Open
                  </a>
                </Button>
              ) : (
                <Button size="sm" variant="outline" disabled>
                  Open
                </Button>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

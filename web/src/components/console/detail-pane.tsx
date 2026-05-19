import { X } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { collectionFor, detailRows } from "@/lib/resource-utils"
import type { Project, Sandbox, Selection, Template } from "@/types"

const emptySelectionCopy = {
  title: "No resource selected",
  body: "Select a row to see IDs, runtime state, and configuration.",
  detail: "Nothing is selected yet.",
}

export function DetailPane({
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

  return (
    <aside className="detail" aria-label="Selected resource">
      <div className="detail-head">
        <div>
          <p className="eyebrow">Selection</p>
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
        ) : (
          <>
            <Badge className="w-fit capitalize bg-[var(--info-soft)] text-[var(--info-ink)] hover:bg-[var(--info-soft)]">
              {selection.kind}
            </Badge>
            <dl className="kv">
              {detailRows(selection.kind, selected, projects, templates).map(([key, value]) => (
                <div key={key}>
                  <dt>{key}</dt>
                  <dd>{String(value || "-")}</dd>
                </div>
              ))}
            </dl>
          </>
        )}
      </div>
    </aside>
  )
}

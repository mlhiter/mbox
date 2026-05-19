import { runtimeText } from "@/lib/resource-utils"
import type { RuntimeRef } from "@/types"

export function RuntimeCell({ refValue }: { refValue: RuntimeRef | undefined }) {
  if (!refValue) {
    return <span className="runtime-cell empty">-</span>
  }
  return (
    <span className="runtime-cell" title={runtimeText(refValue)}>
      <strong>{refValue.kind}</strong>
      <span>
        {refValue.namespace}/{refValue.name}
      </span>
    </span>
  )
}

export function ResourceTitleCell({ name, slug }: { name: string; slug: string }) {
  return (
    <span className="cell-title">
      <strong>{name}</strong>
      <span>{slug}</span>
    </span>
  )
}

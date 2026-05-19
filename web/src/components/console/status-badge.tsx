import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"

export function StatusBadge({ status }: { status: string }) {
  const running = status === "running"
  const failed = status === "failed"
  return (
    <Badge
      variant="secondary"
      className={cn(
        "h-6 gap-1.5 rounded-full px-2.5 font-semibold",
        running && "bg-[var(--runtime-soft)] text-[var(--runtime-ink)] hover:bg-[var(--runtime-soft)]",
        failed && "bg-[var(--danger-soft)] text-[var(--danger)] hover:bg-[var(--danger-soft)]",
      )}
    >
      <span className="status-badge-dot" />
      {status}
    </Badge>
  )
}

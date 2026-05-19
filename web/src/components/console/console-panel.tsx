import {
  Card,
  CardAction,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { cn } from "@/lib/utils"
import type { ConsolePanelProps } from "@/types"

export function ConsolePanel({
  id,
  eyebrow,
  title,
  action,
  wide = false,
  children,
}: ConsolePanelProps) {
  return (
    <Card id={id} className={cn("panel records-table", wide && "wide")}>
      <CardHeader className="panel-head">
        <div>
          <p className="eyebrow">{eyebrow}</p>
          <CardTitle className="panel-title">{title}</CardTitle>
        </div>
        <CardAction>{action}</CardAction>
      </CardHeader>
      <CardContent className="p-0">{children}</CardContent>
    </Card>
  )
}

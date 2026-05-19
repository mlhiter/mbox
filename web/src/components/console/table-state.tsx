import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from "@/components/ui/empty"
import { Skeleton } from "@/components/ui/skeleton"
import { TableCell, TableRow } from "@/components/ui/table"

export function SkeletonRows({ columns }: { columns: number }) {
  return (
    <>
      {Array.from({ length: 3 }, (_, index) => (
        <TableRow key={index}>
          <TableCell colSpan={columns}>
            <Skeleton className="h-3.5 w-[min(320px,80%)] rounded-full bg-[linear-gradient(90deg,oklch(0.93_0.006_82),oklch(0.975_0.004_82),oklch(0.93_0.006_82))] bg-[length:220%_100%]" />
          </TableCell>
        </TableRow>
      ))}
    </>
  )
}

export function EmptyRow({
  columns,
  title,
  detail,
}: {
  columns: number
  title: string
  detail: string
}) {
  return (
    <TableRow>
      <TableCell colSpan={columns} className="empty">
        <Empty>
          <EmptyHeader>
            <EmptyTitle>{title}</EmptyTitle>
            <EmptyDescription>{detail}</EmptyDescription>
          </EmptyHeader>
        </Empty>
      </TableCell>
    </TableRow>
  )
}

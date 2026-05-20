import { useState } from "react"
import { Trash2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { runtimeText } from "@/lib/resource-utils"
import type { Sandbox } from "@/types"

export function DeleteSandboxDialog({
  sandbox,
  onDelete,
  className,
}: {
  sandbox: Sandbox
  onDelete: (id: string) => Promise<void>
  className?: string
}) {
  const [open, setOpen] = useState(false)
  const [deleting, setDeleting] = useState(false)

  async function confirmDelete() {
    setDeleting(true)
    try {
      await onDelete(sandbox.id)
      setOpen(false)
    } finally {
      setDeleting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(nextOpen) => !deleting && setOpen(nextOpen)}>
      <DialogTrigger asChild>
        <Button variant="ghost" size="icon-sm" className={className} aria-label={`Delete ${sandbox.name}`} title="Delete sandbox">
          <Trash2 />
        </Button>
      </DialogTrigger>
      <DialogContent className="dialog-content">
        <DialogHeader>
          <DialogTitle>Delete sandbox</DialogTitle>
          <DialogDescription>
            This removes the mbox sandbox record and asks the runtime controller to clean up its projected runtime.
          </DialogDescription>
        </DialogHeader>
        <dl className="confirm-list">
          <div>
            <dt>Sandbox</dt>
            <dd>{sandbox.name}</dd>
          </div>
          <div>
            <dt>Namespace</dt>
            <dd>{sandbox.namespace}</dd>
          </div>
          <div>
            <dt>Runtime</dt>
            <dd>{runtimeText(sandbox.runtimeRef)}</dd>
          </div>
        </dl>
        <p className="confirm-warning">
          Workspace storage follows the runtime cleanup policy. Check the Storage tab before deleting if persistence matters.
        </p>
        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline" type="button" disabled={deleting}>
              Cancel
            </Button>
          </DialogClose>
          <Button variant="destructive" type="button" onClick={() => void confirmDelete()} disabled={deleting}>
            {deleting ? "Deleting..." : "Delete sandbox"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

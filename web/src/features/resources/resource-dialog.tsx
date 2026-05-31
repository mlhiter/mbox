import { useState, type FormEvent, type ReactNode } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
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
import {
  Field,
  FieldContent,
  FieldGroup,
  FieldLabel,
  FieldTitle,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Textarea } from "@/components/ui/textarea"
import type { FormRecord } from "@/types"

export function ResourceDialog({
  title,
  description,
  trigger,
  submitLabel,
  submitDisabled = false,
  onSubmit,
  onOpenChange,
  children,
}: {
  title: string
  description: string
  trigger: ReactNode
  submitLabel: string
  submitDisabled?: boolean
  onSubmit: (data: FormRecord) => Promise<void>
  onOpenChange?: (open: boolean) => void
  children: ReactNode
}) {
  const [open, setOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)

  function setDialogOpen(nextOpen: boolean) {
    setOpen(nextOpen)
    onOpenChange?.(nextOpen)
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = event.currentTarget
    setSubmitting(true)
    try {
      await onSubmit(Object.fromEntries(new FormData(form).entries()))
      form.reset()
      setDialogOpen(false)
    } catch (submitError) {
      toast.error(submitError instanceof Error ? submitError.message : "Request failed")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setDialogOpen}>
      <DialogTrigger asChild>{trigger}</DialogTrigger>
      <DialogContent className="dialog-content">
        <form className="dialog-grid" onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{title}</DialogTitle>
            <DialogDescription>{description}</DialogDescription>
          </DialogHeader>
          {children}
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline" type="button">
                Cancel
              </Button>
            </DialogClose>
            <Button type="submit" disabled={submitting || submitDisabled}>
              {submitting ? "Working..." : submitLabel}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

export function TextField({
  name,
  label,
  ...props
}: React.ComponentProps<typeof Input> & {
  name: string
  label: string
}) {
  return (
    <Field>
      <FieldLabel htmlFor={name}>{label}</FieldLabel>
      <Input id={name} name={name} autoComplete="off" {...props} />
    </Field>
  )
}

export function TextareaField({
  name,
  label,
  ...props
}: React.ComponentProps<typeof Textarea> & {
  name: string
  label: string
}) {
  return (
    <Field>
      <FieldLabel htmlFor={name}>{label}</FieldLabel>
      <Textarea id={name} name={name} autoComplete="off" {...props} />
    </Field>
  )
}

export function SelectField({
  name,
  label,
  items,
  defaultValue,
  required,
  value,
  onValueChange,
}: {
  name: string
  label: string
  items: Array<{ value: string; label: string }>
  defaultValue?: string
  required?: boolean
  value?: string
  onValueChange?: (value: string) => void
}) {
  const fallbackValue = defaultValue || items[0]?.value || ""
  return (
    <Field>
      <FieldLabel>{label}</FieldLabel>
      <Select name={name} defaultValue={value === undefined ? fallbackValue : undefined} value={value} onValueChange={onValueChange} required={required}>
        <SelectTrigger className="w-full">
          <SelectValue placeholder={`Select ${label.toLowerCase()}`} />
        </SelectTrigger>
        <SelectContent>
          <SelectGroup>
            {items.map((item) => (
              <SelectItem key={item.value} value={item.value}>
                {item.label}
              </SelectItem>
            ))}
          </SelectGroup>
        </SelectContent>
      </Select>
    </Field>
  )
}

export function CheckboxField({
  name,
  label,
  defaultChecked = false,
}: {
  name: string
  label: string
  defaultChecked?: boolean
}) {
  return (
    <Field orientation="horizontal">
      <Checkbox id={name} name={name} defaultChecked={defaultChecked} />
      <FieldContent>
        <FieldTitle>
          <FieldLabel htmlFor={name}>{label}</FieldLabel>
        </FieldTitle>
      </FieldContent>
    </Field>
  )
}

export { FieldGroup }

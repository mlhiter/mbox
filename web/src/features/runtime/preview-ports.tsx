import { FormEvent, useMemo, useState } from "react"
import { ExternalLink, Plus, Trash2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { RuntimeSectionHead } from "@/features/runtime/runtime-section-head"
import type { PreviewPort, Sandbox, SandboxPort } from "@/types"

type PreviewPortsProps = {
  ports: PreviewPort[]
  sandboxPorts: SandboxPort[]
  sandboxStatus: Sandbox["status"]
  onSave: (ports: SandboxPort[]) => Promise<Sandbox>
}

export function PreviewPorts({
  ports,
  sandboxPorts,
  sandboxStatus,
  onSave,
}: PreviewPortsProps) {
  const [name, setName] = useState("")
  const [port, setPort] = useState("")
  const [formError, setFormError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  const declaredPorts = useMemo(() => new Set(sandboxPorts.map((item) => item.port)), [sandboxPorts])

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const number = Number(port)
    const label = name.trim() || `port-${number}`
    if (!Number.isInteger(number) || number < 1 || number > 65535) {
      setFormError("Port must be a number between 1 and 65535.")
      return
    }
    if (declaredPorts.has(number)) {
      setFormError("That port is already declared.")
      return
    }
    setSaving(true)
    setFormError(null)
    try {
      await onSave([...sandboxPorts, { name: label, port: number, protocol: "TCP" }])
      setName("")
      setPort("")
    } catch (error) {
      setFormError(error instanceof Error ? error.message : "Could not save preview port.")
    } finally {
      setSaving(false)
    }
  }

  async function removePort(portNumber: number) {
    setSaving(true)
    setFormError(null)
    try {
      await onSave(sandboxPorts.filter((item) => item.port !== portNumber))
    } catch (error) {
      setFormError(error instanceof Error ? error.message : "Could not remove preview port.")
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="preview-ports">
      <RuntimeSectionHead eyebrow="Preview" title="Ports" />
      <form className="preview-port-form" onSubmit={handleSubmit}>
        <div>
          <Label htmlFor="preview-port-name">Name</Label>
          <Input
            id="preview-port-name"
            value={name}
            placeholder="web"
            onChange={(event) => setName(event.target.value)}
            disabled={saving}
          />
        </div>
        <div>
          <Label htmlFor="preview-port-number">Port</Label>
          <Input
            id="preview-port-number"
            inputMode="numeric"
            pattern="[0-9]*"
            value={port}
            placeholder="3000"
            onChange={(event) => setPort(event.target.value)}
            disabled={saving}
          />
        </div>
        <Button type="submit" disabled={saving}>
          <Plus data-icon="inline-start" />
          {saving ? "Saving..." : "Add port"}
        </Button>
      </form>
      {formError ? <p className="preview-port-error">{formError}</p> : null}
      {sandboxStatus !== "running" ? (
        <p className="preview-port-note">Preview links become active after the sandbox is running.</p>
      ) : null}
      {ports.length === 0 ? (
        <p>No preview ports declared. Start a service in the terminal, then add its TCP port here.</p>
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
              <Button
                size="icon-sm"
                variant="ghost"
                aria-label={`Remove ${port.name} preview port`}
                disabled={saving}
                onClick={() => void removePort(port.port)}
              >
                <Trash2 />
              </Button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

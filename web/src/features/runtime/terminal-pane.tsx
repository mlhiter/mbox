import { useEffect, useRef, useState } from "react"
import { FitAddon } from "@xterm/addon-fit"
import { Terminal } from "@xterm/xterm"
import "@xterm/xterm/css/xterm.css"
import { Plug, Unplug } from "lucide-react"
import { Button } from "@/components/ui/button"
import { terminalURL } from "@/lib/resource-utils"
import type { Sandbox } from "@/types"

export function TerminalPane({
  sandbox,
  disabled,
  disabledReason,
  onSessionChange,
}: {
  sandbox: Sandbox
  disabled: boolean
  disabledReason?: string
  onSessionChange?: () => Promise<void>
}) {
  const hostRef = useRef<HTMLDivElement | null>(null)
  const terminalRef = useRef<Terminal | null>(null)
  const socketRef = useRef<WebSocket | null>(null)
  const inputDisposableRef = useRef<{ dispose: () => void } | null>(null)
  const [connected, setConnected] = useState(false)

  useEffect(() => {
    const host = hostRef.current
    if (!host) {
      return
    }
    const terminal = new Terminal({
      cursorBlink: true,
      convertEol: true,
      fontFamily: '"SF Mono", SFMono-Regular, ui-monospace, Menlo, Consolas, monospace',
      fontSize: 12,
      rows: 16,
      theme: {
        background: "#151512",
        foreground: "#ebe7dc",
        cursor: "#8ccf9f",
        selectionBackground: "#4e5a47",
      },
    })
    const fit = new FitAddon()
    terminal.loadAddon(fit)
    terminal.open(host)
    fit.fit()
    terminal.write("mbox terminal standby.\r\n")
    terminalRef.current = terminal

    const resizeObserver = new ResizeObserver(() => fit.fit())
    resizeObserver.observe(host)

    return () => {
      resizeObserver.disconnect()
      socketRef.current?.close()
      inputDisposableRef.current?.dispose()
      terminal.dispose()
      terminalRef.current = null
    }
  }, [sandbox.id])

  function connect() {
    if (disabled || socketRef.current?.readyState === WebSocket.OPEN) {
      return
    }
    const terminal = terminalRef.current
    if (!terminal) {
      return
    }
    terminal.clear()
    terminal.write("Connecting...\r\n")
    const socket = new WebSocket(terminalURL(sandbox.id))
    socketRef.current = socket

    socket.onopen = () => {
      setConnected(true)
      terminal.write("Connected.\r\n")
      terminal.focus()
    }
    socket.onmessage = (event) => {
      terminal.write(typeof event.data === "string" ? event.data : "")
    }
    socket.onerror = () => {
      terminal.write("\r\nConnection error.\r\n")
    }
    socket.onclose = () => {
      setConnected(false)
      terminal.write("\r\nConnection closed.\r\n")
      inputDisposableRef.current?.dispose()
      inputDisposableRef.current = null
      void onSessionChange?.()
    }
    inputDisposableRef.current?.dispose()
    inputDisposableRef.current = terminal.onData((data) => {
      if (socket.readyState === WebSocket.OPEN) {
        socket.send(data)
      }
    })
  }

  function disconnect() {
    socketRef.current?.close()
    socketRef.current = null
    inputDisposableRef.current?.dispose()
    inputDisposableRef.current = null
    setConnected(false)
  }

  return (
    <div className="terminal-shell">
      <div className="terminal-toolbar">
        <span>{connected ? "Connected" : disabled ? disabledReason || "Runtime unavailable" : "Disconnected"}</span>
        <div>
          <Button size="sm" onClick={connect} disabled={disabled || connected} title={disabled ? disabledReason : undefined}>
            <Plug data-icon="inline-start" />
            Connect
          </Button>
          <Button size="sm" variant="outline" onClick={disconnect} disabled={!connected}>
            <Unplug data-icon="inline-start" />
            Disconnect
          </Button>
        </div>
      </div>
      <div ref={hostRef} className="terminal-host" />
    </div>
  )
}

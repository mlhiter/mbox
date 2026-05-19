package httpapi

import (
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	mboxruntime "github.com/mlhiter/mbox/internal/runtime"
)

var terminalUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		originURL, err := url.Parse(origin)
		if err != nil {
			return false
		}
		return sameOriginHost(originURL.Host, r.Host)
	},
}

func (api *API) getSandboxRuntime(w http.ResponseWriter, r *http.Request) {
	_, ref, ok := api.sandboxRuntimeRef(w, r)
	if !ok {
		return
	}
	target, err := api.access.ResolveRuntime(r.Context(), ref)
	if err != nil {
		writeRuntimeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, target)
}

func (api *API) getSandboxLogs(w http.ResponseWriter, r *http.Request) {
	_, ref, ok := api.sandboxRuntimeRef(w, r)
	if !ok {
		return
	}
	tailLines := int64(200)
	if raw := r.URL.Query().Get("tailLines"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 1 || parsed > 5000 {
			writeError(w, http.StatusBadRequest, "tailLines must be between 1 and 5000")
			return
		}
		tailLines = parsed
	}
	logs, err := api.access.ReadLogs(r.Context(), ref, mboxruntime.LogOptions{
		Container: r.URL.Query().Get("container"),
		TailLines: tailLines,
	})
	if err != nil {
		writeRuntimeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

func (api *API) getSandboxEvents(w http.ResponseWriter, r *http.Request) {
	_, ref, ok := api.sandboxRuntimeRef(w, r)
	if !ok {
		return
	}
	events, err := api.access.ListEvents(r.Context(), ref)
	if err != nil {
		writeRuntimeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": events})
}

func (api *API) connectSandboxTerminal(w http.ResponseWriter, r *http.Request) {
	sandbox, ref, ok := api.sandboxRuntimeRef(w, r)
	if !ok {
		return
	}
	if sandbox.Status != "running" {
		writeError(w, http.StatusConflict, "sandbox must be running before opening terminal")
		return
	}
	command, ok := execCommandFromQuery(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "shell must be sh or bash")
		return
	}
	conn, err := terminalUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Time{})
	ctx, cancel := contextWithRequest(r)
	defer cancel()

	streamErr := make(chan error, 1)
	go func() {
		streamErr <- api.access.Exec(ctx, ref, mboxruntime.ExecOptions{
			Command: command,
			Stdin:   &websocketReader{conn: conn},
			Stdout:  &websocketWriter{conn: conn},
			TTY:     true,
		})
	}()

	if err := <-streamErr; err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("\r\nterminal closed: "+err.Error()+"\r\n"))
	}
}

func sameOriginHost(originHost string, requestHost string) bool {
	if strings.EqualFold(originHost, requestHost) {
		return true
	}
	originName := hostWithoutPort(originHost)
	requestName := hostWithoutPort(requestHost)
	return isLoopbackHost(originName) && isLoopbackHost(requestName)
}

func hostWithoutPort(host string) string {
	value, _, err := net.SplitHostPort(host)
	if err == nil {
		return strings.Trim(value, "[]")
	}
	return strings.Trim(host, "[]")
}

func isLoopbackHost(host string) bool {
	normalized := strings.ToLower(strings.TrimSuffix(host, "."))
	if normalized == "localhost" {
		return true
	}
	ip := net.ParseIP(normalized)
	return ip != nil && ip.IsLoopback()
}

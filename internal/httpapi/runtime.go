package httpapi

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/mlhiter/mbox/internal/domain"
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

func (api *API) getSandboxPorts(w http.ResponseWriter, r *http.Request) {
	sandbox, ref, ok := api.sandboxRuntimeRef(w, r)
	if !ok {
		return
	}
	target, err := api.access.ResolveRuntime(r.Context(), ref)
	if err != nil {
		writeRuntimeError(w, err)
		return
	}
	ports := make([]mboxruntime.PreviewPort, 0, len(sandbox.Ports))
	for _, port := range sandbox.Ports {
		protocol := defaultProtocol(port.Protocol)
		preview := mboxruntime.PreviewPort{
			Name:      port.Name,
			Port:      port.Port,
			Protocol:  protocol,
			Available: sandbox.Status == "running" && (port.PreviewURL != "" || protocol == "TCP"),
		}
		if preview.Available && port.PreviewURL != "" {
			preview.PreviewURL = port.PreviewURL
		} else if preview.Available {
			preview.PreviewURL = sandboxPortProxyURL(r, sandbox.ID.String(), port.Port, "/")
		} else if protocol != "TCP" {
			preview.Message = "only TCP ports can be opened in browser preview"
		} else {
			preview.Message = "sandbox must be running before preview is available"
		}
		ports = append(ports, preview)
	}
	writeJSON(w, http.StatusOK, mboxruntime.PreviewPortsResult{
		Target: target,
		Items:  ports,
	})
}

func (api *API) proxySandboxPort(w http.ResponseWriter, r *http.Request) {
	sandbox, ref, ok := api.sandboxRuntimeRef(w, r)
	if !ok {
		return
	}
	if sandbox.Status != "running" {
		writeError(w, http.StatusConflict, "sandbox must be running before opening preview")
		return
	}
	port, ok := parseSandboxPortParam(w, r)
	if !ok {
		return
	}
	if !sandboxAllowsPreviewPort(sandbox, port) {
		writeError(w, http.StatusNotFound, "sandbox port is not declared or is not previewable")
		return
	}
	response, err := api.access.ProxyPreview(r.Context(), ref, mboxruntime.PreviewProxyRequest{
		Port:  port,
		Path:  strings.TrimPrefix(r.URL.Path, fmt.Sprintf("/v1/sandboxes/%s/ports/%d/proxy", sandbox.ID.String(), port)),
		Query: r.URL.RawQuery,
	})
	if err != nil {
		writeRuntimeError(w, err)
		return
	}
	defer response.Body.Close()

	copyProxyHeaders(w.Header(), response.Header)
	if response.StatusCode > 0 {
		w.WriteHeader(response.StatusCode)
	}
	_, _ = io.Copy(w, response.Body)
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
	clientName := strings.TrimSpace(r.URL.Query().Get("client"))
	if clientName == "" {
		clientName = "web-terminal"
	}
	if len(clientName) > maxRuntimeSessionClientLength {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("\r\nterminal client label is too long\r\n"))
		return
	}
	session, err := api.store.CreateRuntimeSession(r.Context(), domain.RuntimeSessionCreate{
		ProjectID:  sandbox.ProjectID,
		SandboxID:  sandbox.ID,
		Type:       domain.RuntimeSessionTypeTerminal,
		Client:     clientName,
		UserAgent:  runtimeSessionUserAgent(r),
		RuntimeRef: sandbox.RuntimeRef,
	})
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("\r\nterminal session record failed\r\n"))
		return
	}

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
		api.finishRuntimeSession(session.ID, domain.RuntimeSessionStatusFailed)
		_ = conn.WriteMessage(websocket.TextMessage, []byte("\r\nterminal closed: "+err.Error()+"\r\n"))
		return
	}
	api.finishRuntimeSession(session.ID, domain.RuntimeSessionStatusEnded)
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

func parseSandboxPortParam(w http.ResponseWriter, r *http.Request) (int, bool) {
	port, err := strconv.Atoi(r.PathValue("port"))
	if err != nil || port < 1 || port > 65535 {
		writeError(w, http.StatusBadRequest, "invalid sandbox port")
		return 0, false
	}
	return port, true
}

func sandboxAllowsPreviewPort(sandbox domain.Sandbox, port int) bool {
	for _, item := range sandbox.Ports {
		if item.Port == port && defaultProtocol(item.Protocol) == "TCP" {
			return true
		}
	}
	return false
}

func defaultProtocol(protocol string) string {
	if strings.TrimSpace(protocol) == "" {
		return "TCP"
	}
	return strings.ToUpper(protocol)
}

func sandboxPortProxyURL(r *http.Request, sandboxID string, port int, proxyPath string) string {
	if proxyPath == "" {
		proxyPath = "/"
	}
	if !strings.HasPrefix(proxyPath, "/") {
		proxyPath = "/" + proxyPath
	}
	return fmt.Sprintf("/v1/sandboxes/%s/ports/%d/proxy%s", sandboxID, port, proxyPath)
}

func copyProxyHeaders(target http.Header, source map[string][]string) {
	for key, values := range source {
		if strings.EqualFold(key, "Content-Length") || strings.EqualFold(key, "Transfer-Encoding") {
			continue
		}
		for _, value := range values {
			target.Add(key, value)
		}
	}
}

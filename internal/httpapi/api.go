package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/mlhiter/mbox/internal/domain"
	mboxruntime "github.com/mlhiter/mbox/internal/runtime"
)

type API struct {
	store  domain.Store
	access mboxruntime.Access
	mux    *http.ServeMux
}

func New(store domain.Store) *API {
	return NewWithRuntimeAccess(store, nil)
}

func NewWithRuntimeAccess(store domain.Store, access mboxruntime.Access) *API {
	api := &API{
		store:  store,
		access: access,
		mux:    http.NewServeMux(),
	}
	api.routes()
	return api
}

func (api *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	api.mux.ServeHTTP(w, r)
}

func (api *API) routes() {
	api.mux.HandleFunc("GET /healthz", api.healthz)
	api.mux.HandleFunc("GET /v1/projects", api.listProjects)
	api.mux.HandleFunc("POST /v1/projects", api.createProject)
	api.mux.HandleFunc("GET /v1/projects/{projectID}", api.getProject)
	api.mux.HandleFunc("PATCH /v1/projects/{projectID}", api.updateProject)
	api.mux.HandleFunc("DELETE /v1/projects/{projectID}", api.deleteProject)

	api.mux.HandleFunc("GET /v1/templates", api.listTemplates)
	api.mux.HandleFunc("POST /v1/templates", api.createTemplate)
	api.mux.HandleFunc("GET /v1/templates/{templateID}", api.getTemplate)
	api.mux.HandleFunc("PATCH /v1/templates/{templateID}", api.updateTemplate)
	api.mux.HandleFunc("DELETE /v1/templates/{templateID}", api.deleteTemplate)

	api.mux.HandleFunc("GET /v1/sandboxes", api.listSandboxes)
	api.mux.HandleFunc("POST /v1/sandboxes", api.createSandbox)
	api.mux.HandleFunc("GET /v1/sandboxes/{sandboxID}", api.getSandbox)
	api.mux.HandleFunc("PATCH /v1/sandboxes/{sandboxID}", api.updateSandbox)
	api.mux.HandleFunc("DELETE /v1/sandboxes/{sandboxID}", api.deleteSandbox)
	api.mux.HandleFunc("POST /v1/sandboxes/{sandboxID}/start", api.startSandbox)
	api.mux.HandleFunc("POST /v1/sandboxes/{sandboxID}/stop", api.stopSandbox)
	api.mux.HandleFunc("GET /v1/sandboxes/{sandboxID}/runtime", api.getSandboxRuntime)
	api.mux.HandleFunc("GET /v1/sandboxes/{sandboxID}/logs", api.getSandboxLogs)
	api.mux.HandleFunc("GET /v1/sandboxes/{sandboxID}/events", api.getSandboxEvents)
	api.mux.HandleFunc("GET /v1/sandboxes/{sandboxID}/ports", api.getSandboxPorts)
	api.mux.HandleFunc("GET /v1/sandboxes/{sandboxID}/ports/{port}/proxy/", api.proxySandboxPort)
	api.mux.HandleFunc("GET /v1/sandboxes/{sandboxID}/terminal", api.connectSandboxTerminal)
}

func (api *API) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

var slugPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

func parseUUIDParam(r *http.Request, name string) (uuid.UUID, bool) {
	value := r.PathValue(name)
	id, err := uuid.Parse(value)
	return id, err == nil
}

func parseOptionalUUIDQuery(r *http.Request, name string) (*uuid.UUID, bool) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return nil, true
	}
	id, err := uuid.Parse(value)
	if err != nil {
		return nil, false
	}
	return &id, true
}

func decodeJSON(r *http.Request, dest any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(dest)
}

type nullableUUID struct {
	Set   bool
	Value *uuid.UUID
}

func parseNullableUUID(data json.RawMessage) (nullableUUID, error) {
	value := nullableUUID{Set: true}
	if string(data) == "null" {
		return value, nil
	}
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return nullableUUID{}, err
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nullableUUID{}, err
	}
	value.Value = &id
	return value, nil
}

type nullableRuntimeRef struct {
	Set   bool
	Value *domain.RuntimeRef
}

func parseNullableRuntimeRef(data json.RawMessage) (nullableRuntimeRef, error) {
	value := nullableRuntimeRef{Set: true}
	if string(data) == "null" {
		return value, nil
	}
	var ref domain.RuntimeRef
	if err := json.Unmarshal(data, &ref); err != nil {
		return nullableRuntimeRef{}, err
	}
	value.Value = &ref
	return value, nil
}

func rejectUnknownFields(fields map[string]json.RawMessage, allowed map[string]struct{}) error {
	for field := range fields {
		if _, ok := allowed[field]; !ok {
			return fmt.Errorf("unknown field %q", field)
		}
	}
	return nil
}

func validateSlug(slug string) bool {
	return slugPattern.MatchString(slug)
}

func slugOrName(slug string, name string) string {
	normalized := strings.TrimSpace(slug)
	if normalized == "" {
		return slugFromName(name)
	}
	return normalized
}

func slugFromName(name string) string {
	var builder strings.Builder
	lastDash := false
	for _, value := range strings.ToLower(strings.TrimSpace(name)) {
		if value >= 'a' && value <= 'z' || value >= '0' && value <= '9' {
			builder.WriteRune(value)
			lastDash = false
			continue
		}
		if unicode.IsSpace(value) || value == '-' || value == '_' || value == '.' || value == '/' {
			if builder.Len() > 0 && !lastDash {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(builder.String(), "-")
}

func validateRequired(value string) bool {
	return strings.TrimSpace(value) != ""
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "resource not found")
	case errors.Is(err, domain.ErrConflict):
		writeError(w, http.StatusConflict, "resource conflict")
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func (api *API) sandboxRuntimeRef(w http.ResponseWriter, r *http.Request) (domain.Sandbox, domain.RuntimeRef, bool) {
	if api.access == nil {
		writeError(w, http.StatusServiceUnavailable, "runtime access is not configured")
		return domain.Sandbox{}, domain.RuntimeRef{}, false
	}
	id, ok := parseUUIDParam(r, "sandboxID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid sandbox id")
		return domain.Sandbox{}, domain.RuntimeRef{}, false
	}
	sandbox, err := api.store.GetSandbox(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return domain.Sandbox{}, domain.RuntimeRef{}, false
	}
	if sandbox.RuntimeRef == nil {
		writeError(w, http.StatusConflict, "sandbox runtime is not ready")
		return domain.Sandbox{}, domain.RuntimeRef{}, false
	}
	return sandbox, *sandbox.RuntimeRef, true
}

func writeRuntimeError(w http.ResponseWriter, err error) {
	writeError(w, http.StatusBadGateway, err.Error())
}

type websocketReader struct {
	conn    *websocket.Conn
	pending []byte
}

func (r *websocketReader) Read(p []byte) (int, error) {
	if len(r.pending) > 0 {
		n := copy(p, r.pending)
		r.pending = r.pending[n:]
		return n, nil
	}
	for {
		messageType, payload, err := r.conn.ReadMessage()
		if err != nil {
			return 0, err
		}
		if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
			continue
		}
		if len(payload) == 0 {
			continue
		}
		n := copy(p, payload)
		r.pending = payload[n:]
		return n, nil
	}
}

type websocketWriter struct {
	conn *websocket.Conn
}

func (w *websocketWriter) Write(p []byte) (int, error) {
	if err := w.conn.WriteMessage(websocket.TextMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

var _ io.Reader = (*websocketReader)(nil)
var _ io.Writer = (*websocketWriter)(nil)

func execCommandFromQuery(r *http.Request) ([]string, bool) {
	shell := strings.TrimSpace(r.URL.Query().Get("shell"))
	switch shell {
	case "", "sh":
		return []string{"/bin/sh"}, true
	case "bash":
		return []string{"/bin/bash"}, true
	default:
		return nil, false
	}
}

func contextWithRequest(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithCancel(r.Context())
}

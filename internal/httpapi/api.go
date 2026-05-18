package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/mlhiter/mbox/internal/domain"
)

type API struct {
	store domain.Store
	mux   *http.ServeMux
}

func New(store domain.Store) *API {
	api := &API{
		store: store,
		mux:   http.NewServeMux(),
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

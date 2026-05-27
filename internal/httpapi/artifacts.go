package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/mlhiter/mbox/internal/domain"
)

const (
	maxArtifactNameLength        = 256
	maxArtifactURILength         = 4096
	maxArtifactContentTypeLength = 255
)

type createArtifactRequest struct {
	TaskID      *uuid.UUID          `json:"taskId"`
	Kind        domain.ArtifactKind `json:"kind"`
	Name        string              `json:"name"`
	URI         string              `json:"uri"`
	ContentType string              `json:"contentType"`
	SizeBytes   *int64              `json:"sizeBytes"`
	Metadata    json.RawMessage     `json:"metadata"`
}

func (api *API) listSandboxArtifacts(w http.ResponseWriter, r *http.Request) {
	sandbox, ok := api.sandboxFromPath(w, r)
	if !ok {
		return
	}
	artifacts, err := api.store.ListArtifacts(r.Context(), sandbox.ID, nil)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": artifacts})
}

func (api *API) listTaskArtifacts(w http.ResponseWriter, r *http.Request) {
	task, ok := api.taskFromPath(w, r)
	if !ok {
		return
	}
	artifacts, err := api.store.ListArtifacts(r.Context(), task.SandboxID, &task.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": artifacts})
}

func (api *API) getArtifact(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(r, "artifactID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid artifact id")
		return
	}
	artifact, err := api.store.GetArtifact(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, artifact)
}

func (api *API) createSandboxArtifact(w http.ResponseWriter, r *http.Request) {
	sandbox, ok := api.sandboxFromPath(w, r)
	if !ok {
		return
	}
	var req createArtifactRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	if message := validateArtifactRequest(req); message != "" {
		writeError(w, http.StatusBadRequest, message)
		return
	}
	if req.TaskID != nil {
		task, err := api.store.GetExecutionTask(r.Context(), *req.TaskID)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		if task.SandboxID != sandbox.ID {
			writeError(w, http.StatusConflict, "task does not belong to sandbox")
			return
		}
	}

	artifact, err := api.store.CreateArtifact(r.Context(), domain.ArtifactCreate{
		ProjectID:   sandbox.ProjectID,
		SandboxID:   sandbox.ID,
		TaskID:      req.TaskID,
		Kind:        req.Kind,
		Name:        strings.TrimSpace(req.Name),
		URI:         strings.TrimSpace(req.URI),
		ContentType: strings.TrimSpace(req.ContentType),
		SizeBytes:   req.SizeBytes,
		Metadata:    req.Metadata,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, artifact)
}

func (api *API) taskFromPath(w http.ResponseWriter, r *http.Request) (domain.ExecutionTask, bool) {
	id, ok := parseUUIDParam(r, "taskID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid task id")
		return domain.ExecutionTask{}, false
	}
	task, err := api.store.GetExecutionTask(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return domain.ExecutionTask{}, false
	}
	return task, true
}

func validateArtifactRequest(req createArtifactRequest) string {
	if !validArtifactKind(req.Kind) {
		return "kind is invalid"
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return "name is required"
	}
	if len(name) > maxArtifactNameLength {
		return "name is too long"
	}
	uri := strings.TrimSpace(req.URI)
	if uri == "" {
		return "uri is required"
	}
	if len(uri) > maxArtifactURILength {
		return "uri is too long"
	}
	if len(strings.TrimSpace(req.ContentType)) > maxArtifactContentTypeLength {
		return "contentType is too long"
	}
	if req.SizeBytes != nil && *req.SizeBytes < 0 {
		return "sizeBytes cannot be negative"
	}
	return ""
}

func validArtifactKind(kind domain.ArtifactKind) bool {
	switch kind {
	case domain.ArtifactKindFile,
		domain.ArtifactKindDirectory,
		domain.ArtifactKindLog,
		domain.ArtifactKindReport,
		domain.ArtifactKindScreenshot,
		domain.ArtifactKindImage,
		domain.ArtifactKindLink,
		domain.ArtifactKindOther:
		return true
	default:
		return false
	}
}

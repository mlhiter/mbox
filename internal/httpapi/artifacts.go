package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/mlhiter/mbox/internal/domain"
	mboxruntime "github.com/mlhiter/mbox/internal/runtime"
)

const (
	maxArtifactNameLength        = 256
	maxArtifactURILength         = 4096
	maxArtifactContentTypeLength = 255
	maxArtifactContentBytes      = 8 * 1024 * 1024
	workspaceURIPrefix           = "workspace://"
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

func (api *API) getArtifactContent(w http.ResponseWriter, r *http.Request) {
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
	if artifact.Kind == domain.ArtifactKindDirectory {
		writeError(w, http.StatusBadRequest, "directory artifacts cannot be downloaded as content")
		return
	}
	if content, err := api.store.GetArtifactContent(r.Context(), artifact.ID); err == nil {
		bytes, err := api.artifactContents.Read(r.Context(), content)
		if err != nil {
			writeArtifactContentStorageError(w, err)
			return
		}
		content.Content = bytes
		writeArtifactContent(w, artifact, content)
		return
	} else if err != domain.ErrNotFound {
		writeStoreError(w, err)
		return
	}
	artifactPath, ok := workspacePathFromArtifactURI(artifact.URI)
	if !ok {
		writeError(w, http.StatusBadRequest, "artifact content is only available for workspace:// file references")
		return
	}
	if api.access == nil {
		writeError(w, http.StatusServiceUnavailable, "runtime access is not configured")
		return
	}
	sandbox, err := api.store.GetSandbox(r.Context(), artifact.SandboxID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if sandbox.Status != domain.SandboxStatusRunning {
		writeError(w, http.StatusConflict, "sandbox must be running before reading workspace artifact content")
		return
	}
	if sandbox.RuntimeRef == nil {
		writeError(w, http.StatusConflict, "sandbox runtime is not ready")
		return
	}

	result, err := api.access.ReadFile(r.Context(), *sandbox.RuntimeRef, mboxruntime.FileReadRequest{
		Path:          artifactPath,
		MaxBytes:      maxArtifactContentBytes,
		WorkspaceOnly: true,
	})
	if err != nil {
		writeRuntimeError(w, err)
		return
	}
	defer result.Body.Close()

	contentType := artifactContentType(artifact, result.ContentType)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Mbox-Artifact-ID", artifact.ID.String())
	w.Header().Set("X-Mbox-Artifact-Path", result.Path)
	w.Header().Set("X-Mbox-Artifact-Retained", "false")
	if result.Truncated {
		w.Header().Set("X-Mbox-Content-Truncated", "true")
	}
	if !result.Truncated {
		w.Header().Set("Content-Length", strconv.FormatInt(result.SizeBytes, 10))
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, result.Body)
}

func (api *API) captureArtifactContent(w http.ResponseWriter, r *http.Request) {
	if api.access == nil {
		writeError(w, http.StatusServiceUnavailable, "runtime access is not configured")
		return
	}
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
	if artifact.Kind == domain.ArtifactKindDirectory {
		writeError(w, http.StatusBadRequest, "directory artifacts cannot be captured as retained content")
		return
	}
	artifactPath, ok := workspacePathFromArtifactURI(artifact.URI)
	if !ok {
		writeError(w, http.StatusBadRequest, "artifact capture is only available for workspace:// file references")
		return
	}
	sandbox, err := api.store.GetSandbox(r.Context(), artifact.SandboxID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if sandbox.Status != domain.SandboxStatusRunning {
		writeError(w, http.StatusConflict, "sandbox must be running before capturing workspace artifact content")
		return
	}
	if sandbox.RuntimeRef == nil {
		writeError(w, http.StatusConflict, "sandbox runtime is not ready")
		return
	}
	result, err := api.access.ReadFile(r.Context(), *sandbox.RuntimeRef, mboxruntime.FileReadRequest{
		Path:          artifactPath,
		MaxBytes:      maxArtifactContentBytes,
		WorkspaceOnly: true,
	})
	if err != nil {
		writeRuntimeError(w, err)
		return
	}
	defer result.Body.Close()
	if result.Truncated {
		writeError(w, http.StatusRequestEntityTooLarge, "artifact content exceeds retention limit")
		return
	}
	content, err := io.ReadAll(io.LimitReader(result.Body, maxArtifactContentBytes+1))
	if err != nil {
		writeRuntimeError(w, err)
		return
	}
	if len(content) > maxArtifactContentBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "artifact content exceeds retention limit")
		return
	}
	hash := sha256.Sum256(content)
	sizeBytes := int64(len(content))
	if err := api.enforceArtifactRetentionQuotaPolicy(r.Context(), artifact.ProjectID, sizeBytes); err != nil {
		api.recordPolicyDeniedAuditEvent(r.Context(), artifact.ProjectID, "artifact.content.capture", "artifact", &artifact.ID, artifact.Name, err, map[string]any{
			"sandboxId":     artifact.SandboxID.String(),
			"artifactKind":  artifact.Kind,
			"incomingBytes": sizeBytes,
		})
		if writePolicyError(w, err) {
			return
		}
		writeStoreError(w, err)
		return
	}
	captureInput, err := api.artifactContents.Capture(r.Context(), artifact, domain.ArtifactContentCapture{
		ArtifactID:  artifact.ID,
		ContentType: artifactContentType(artifact, result.ContentType),
		SizeBytes:   sizeBytes,
		SHA256:      hex.EncodeToString(hash[:]),
		SourceURI:   artifact.URI,
	}, content)
	if err != nil {
		writeArtifactContentStorageError(w, err)
		return
	}
	captured, err := api.store.CaptureArtifactContent(r.Context(), captureInput)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	api.recordAuditEvent(r.Context(), domain.AuditEventCreate{
		ProjectID:    &captured.ProjectID,
		Action:       "artifact.content.captured",
		ResourceType: "artifact",
		ResourceID:   &captured.ID,
		ResourceName: captured.Name,
		Metadata: auditMetadata(map[string]any{
			"sandboxId":       captured.SandboxID.String(),
			"storageProvider": captured.RetainedContent.StorageProvider,
			"sizeBytes":       captured.RetainedContent.SizeBytes,
		}),
	})
	writeJSON(w, http.StatusOK, captured)
}

func (api *API) uploadArtifactContent(w http.ResponseWriter, r *http.Request) {
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
	if artifact.Kind == domain.ArtifactKindDirectory {
		writeError(w, http.StatusBadRequest, "directory artifacts cannot store retained content")
		return
	}
	defer r.Body.Close()
	content, err := io.ReadAll(io.LimitReader(r.Body, maxArtifactContentBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not read artifact content")
		return
	}
	if len(content) > maxArtifactContentBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "artifact content exceeds retention limit")
		return
	}
	hash := sha256.Sum256(content)
	sizeBytes := int64(len(content))
	if err := api.enforceArtifactRetentionQuotaPolicy(r.Context(), artifact.ProjectID, sizeBytes); err != nil {
		api.recordPolicyDeniedAuditEvent(r.Context(), artifact.ProjectID, "artifact.content.upload", "artifact", &artifact.ID, artifact.Name, err, map[string]any{
			"sandboxId":     artifact.SandboxID.String(),
			"artifactKind":  artifact.Kind,
			"incomingBytes": sizeBytes,
		})
		if writePolicyError(w, err) {
			return
		}
		writeStoreError(w, err)
		return
	}
	sourceURI := strings.TrimSpace(r.Header.Get("X-Mbox-Artifact-Source-URI"))
	if sourceURI == "" {
		sourceURI = artifact.URI
	}
	captureInput, err := api.artifactContents.Capture(r.Context(), artifact, domain.ArtifactContentCapture{
		ArtifactID:  artifact.ID,
		ContentType: artifactContentType(artifact, r.Header.Get("Content-Type")),
		SizeBytes:   sizeBytes,
		SHA256:      hex.EncodeToString(hash[:]),
		SourceURI:   sourceURI,
	}, content)
	if err != nil {
		writeArtifactContentStorageError(w, err)
		return
	}
	captured, err := api.store.CaptureArtifactContent(r.Context(), captureInput)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	api.recordAuditEvent(r.Context(), domain.AuditEventCreate{
		ProjectID:    &captured.ProjectID,
		Action:       "artifact.content.uploaded",
		ResourceType: "artifact",
		ResourceID:   &captured.ID,
		ResourceName: captured.Name,
		Metadata: auditMetadata(map[string]any{
			"sandboxId":       captured.SandboxID.String(),
			"storageProvider": captured.RetainedContent.StorageProvider,
			"sizeBytes":       captured.RetainedContent.SizeBytes,
		}),
	})
	writeJSON(w, http.StatusOK, captured)
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
	api.recordAuditEvent(r.Context(), domain.AuditEventCreate{
		ProjectID:    &artifact.ProjectID,
		Action:       "artifact.created",
		ResourceType: "artifact",
		ResourceID:   &artifact.ID,
		ResourceName: artifact.Name,
		Metadata: auditMetadata(map[string]any{
			"sandboxId": artifact.SandboxID.String(),
			"kind":      artifact.Kind,
			"taskId":    optionalUUIDString(artifact.TaskID),
		}),
	})
	writeJSON(w, http.StatusCreated, artifact)
}

func optionalUUIDString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

func artifactContentType(artifact domain.Artifact, fallback string) string {
	contentType := strings.TrimSpace(artifact.ContentType)
	if contentType == "" {
		contentType = strings.TrimSpace(fallback)
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return contentType
}

func writeArtifactContent(w http.ResponseWriter, artifact domain.Artifact, content domain.ArtifactContent) {
	contentType := strings.TrimSpace(content.ContentType)
	if contentType == "" {
		contentType = artifactContentType(artifact, "")
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.FormatInt(content.SizeBytes, 10))
	w.Header().Set("X-Mbox-Artifact-ID", artifact.ID.String())
	w.Header().Set("X-Mbox-Artifact-Retained", "true")
	w.Header().Set("X-Mbox-Artifact-SHA256", content.SHA256)
	w.Header().Set("X-Mbox-Artifact-Captured-At", content.CapturedAt.Format(http.TimeFormat))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content.Content)
}

func writeArtifactContentStorageError(w http.ResponseWriter, err error) {
	if err == domain.ErrNotFound {
		writeError(w, http.StatusInternalServerError, "retained artifact content is missing from storage")
		return
	}
	writeError(w, http.StatusInternalServerError, "retained artifact content storage error")
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

func workspacePathFromArtifactURI(uri string) (string, bool) {
	cleanURI := strings.TrimSpace(uri)
	if !strings.HasPrefix(cleanURI, workspaceURIPrefix) {
		return "", false
	}
	rawPath := strings.TrimPrefix(cleanURI, workspaceURIPrefix)
	if rawPath == "" {
		return "", false
	}
	if !strings.HasPrefix(rawPath, "/") {
		rawPath = "/" + rawPath
	}
	cleanPath := path.Clean(rawPath)
	if cleanPath == "/" || strings.Contains(cleanPath, "\x00") {
		return "", false
	}
	return cleanPath, true
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

package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mlhiter/mbox/internal/domain"
)

const (
	maxRuntimeSessionClientLength    = 120
	maxRuntimeSessionUserAgentLength = 512
)

type createRuntimeSessionRequest struct {
	Type     domain.RuntimeSessionType `json:"type"`
	Client   string                    `json:"client"`
	Metadata json.RawMessage           `json:"metadata"`
}

func (api *API) listRuntimeSessions(w http.ResponseWriter, r *http.Request) {
	sandbox, ok := api.sandboxFromPath(w, r)
	if !ok {
		return
	}
	sessions, err := api.store.ListRuntimeSessions(r.Context(), sandbox.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": sessions})
}

func (api *API) createRuntimeSession(w http.ResponseWriter, r *http.Request) {
	sandbox, ok := api.sandboxFromPath(w, r)
	if !ok {
		return
	}
	var req createRuntimeSessionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	session, ok := api.createRuntimeSessionRecord(w, r, sandbox, req.Type, req.Client, req.Metadata)
	if !ok {
		return
	}
	api.recordAuditEvent(r.Context(), domain.AuditEventCreate{
		ProjectID:    &session.ProjectID,
		Action:       "runtime.session.created",
		ResourceType: "runtime-session",
		ResourceID:   &session.ID,
		ResourceName: session.Client,
		Metadata: auditMetadata(map[string]any{
			"sandboxId": session.SandboxID.String(),
			"type":      session.Type,
		}),
	})
	writeJSON(w, http.StatusCreated, session)
}

func (api *API) getRuntimeSession(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(r, "sessionID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid session id")
		return
	}
	session, err := api.store.GetRuntimeSession(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	api.recordAuditEvent(r.Context(), domain.AuditEventCreate{
		ProjectID:    &session.ProjectID,
		Action:       "runtime.session.ended",
		ResourceType: "runtime-session",
		ResourceID:   &session.ID,
		ResourceName: session.Client,
		Metadata: auditMetadata(map[string]any{
			"sandboxId": session.SandboxID.String(),
			"type":      session.Type,
			"status":    session.Status,
		}),
	})
	writeJSON(w, http.StatusOK, session)
}

func (api *API) endRuntimeSession(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(r, "sessionID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid session id")
		return
	}
	status := domain.RuntimeSessionStatusEnded
	endedAt := time.Now().UTC()
	session, err := api.store.UpdateRuntimeSession(r.Context(), id, domain.RuntimeSessionUpdate{
		Status:  &status,
		EndedAt: &endedAt,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (api *API) createRuntimeSessionRecord(
	w http.ResponseWriter,
	r *http.Request,
	sandbox domain.Sandbox,
	sessionType domain.RuntimeSessionType,
	client string,
	metadata json.RawMessage,
) (domain.RuntimeSession, bool) {
	if !validRuntimeSessionType(sessionType) {
		writeError(w, http.StatusBadRequest, "session type is invalid")
		return domain.RuntimeSession{}, false
	}
	cleanClient := strings.TrimSpace(client)
	if len(cleanClient) > maxRuntimeSessionClientLength {
		writeError(w, http.StatusBadRequest, "session client is too long")
		return domain.RuntimeSession{}, false
	}
	session, err := api.store.CreateRuntimeSession(r.Context(), domain.RuntimeSessionCreate{
		ProjectID:  sandbox.ProjectID,
		SandboxID:  sandbox.ID,
		Type:       sessionType,
		Client:     cleanClient,
		UserAgent:  runtimeSessionUserAgent(r),
		RuntimeRef: sandbox.RuntimeRef,
		Metadata:   jsonDefaultRaw(metadata),
	})
	if err != nil {
		writeStoreError(w, err)
		return domain.RuntimeSession{}, false
	}
	return session, true
}

func (api *API) finishRuntimeSession(id uuid.UUID, status domain.RuntimeSessionStatus) {
	endedAt := time.Now().UTC()
	_, _ = api.store.UpdateRuntimeSession(context.Background(), id, domain.RuntimeSessionUpdate{
		Status:  &status,
		EndedAt: &endedAt,
	})
}

func runtimeSessionUserAgent(r *http.Request) string {
	userAgent := strings.TrimSpace(r.UserAgent())
	if len(userAgent) > maxRuntimeSessionUserAgentLength {
		return userAgent[:maxRuntimeSessionUserAgentLength]
	}
	return userAgent
}

func validRuntimeSessionType(sessionType domain.RuntimeSessionType) bool {
	switch sessionType {
	case domain.RuntimeSessionTypeTerminal,
		domain.RuntimeSessionTypeIDE,
		domain.RuntimeSessionTypeNotebook,
		domain.RuntimeSessionTypeBrowser,
		domain.RuntimeSessionTypeCommand,
		domain.RuntimeSessionTypeCustom:
		return true
	default:
		return false
	}
}

func jsonDefaultRaw(data json.RawMessage) []byte {
	if len(data) == 0 {
		return nil
	}
	return data
}

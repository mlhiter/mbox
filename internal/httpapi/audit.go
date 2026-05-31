package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/mlhiter/mbox/internal/domain"
)

const auditSourceHTTPAPI = "http-api"
const auditActorHeader = "X-Mbox-Audit-Actor"
const auditSourceHeader = "X-Mbox-Audit-Source"
const maxAuditAttributionRunes = 128

type auditAttributionContextKey struct{}

type auditAttribution struct {
	Actor  string
	Source string
}

func (api *API) listAuditEvents(w http.ResponseWriter, r *http.Request) {
	filter, ok := api.auditEventFilterFromRequest(w, r, nil)
	if !ok {
		return
	}
	events, err := api.store.ListAuditEvents(r.Context(), filter)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": events})
}

func (api *API) listProjectAuditEvents(w http.ResponseWriter, r *http.Request) {
	projectID, ok := parseUUIDParam(r, "projectID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	if _, err := api.store.GetProject(r.Context(), projectID); err != nil {
		writeStoreError(w, err)
		return
	}
	filter, ok := api.auditEventFilterFromRequest(w, r, &projectID)
	if !ok {
		return
	}
	events, err := api.store.ListAuditEvents(r.Context(), filter)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": events})
}

func (api *API) auditEventFilterFromRequest(
	w http.ResponseWriter,
	r *http.Request,
	projectID *uuid.UUID,
) (domain.AuditEventFilter, bool) {
	filter := domain.AuditEventFilter{ProjectID: projectID}
	if projectID == nil {
		if value := strings.TrimSpace(r.URL.Query().Get("projectId")); value != "" {
			id, err := uuid.Parse(value)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid projectId query")
				return domain.AuditEventFilter{}, false
			}
			filter.ProjectID = &id
		}
	}
	filter.Action = strings.TrimSpace(r.URL.Query().Get("action"))
	filter.ResourceType = strings.TrimSpace(r.URL.Query().Get("resourceType"))
	if value := strings.TrimSpace(r.URL.Query().Get("resourceId")); value != "" {
		id, err := uuid.Parse(value)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid resourceId query")
			return domain.AuditEventFilter{}, false
		}
		filter.ResourceID = &id
	}
	filter.Actor = sanitizeAuditAttribution(r.URL.Query().Get("actor"))
	filter.Source = sanitizeAuditAttribution(r.URL.Query().Get("source"))
	filter.RequestID = sanitizeRequestID(r.URL.Query().Get("requestId"))
	filter.Operation = strings.TrimSpace(r.URL.Query().Get("operation"))
	if value := strings.TrimSpace(r.URL.Query().Get("since")); value != "" {
		since, err := parseAuditEventTime(value)
		if err != nil {
			writeError(w, http.StatusBadRequest, "since must be an RFC3339 timestamp")
			return domain.AuditEventFilter{}, false
		}
		filter.Since = &since
	}
	if value := strings.TrimSpace(r.URL.Query().Get("until")); value != "" {
		until, err := parseAuditEventTime(value)
		if err != nil {
			writeError(w, http.StatusBadRequest, "until must be an RFC3339 timestamp")
			return domain.AuditEventFilter{}, false
		}
		filter.Until = &until
	}
	if filter.Since != nil && filter.Until != nil && filter.Since.After(*filter.Until) {
		writeError(w, http.StatusBadRequest, "since must be before or equal to until")
		return domain.AuditEventFilter{}, false
	}
	if value := strings.TrimSpace(r.URL.Query().Get("limit")); value != "" {
		limit, err := strconv.Atoi(value)
		if err != nil || limit < 1 || limit > 200 {
			writeError(w, http.StatusBadRequest, "limit must be between 1 and 200")
			return domain.AuditEventFilter{}, false
		}
		filter.Limit = limit
	}
	return filter, true
}

func parseAuditEventTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, err
	}
	return parsed, nil
}

func (api *API) recordAuditEvent(ctx context.Context, event domain.AuditEventCreate) {
	event.Action = strings.TrimSpace(event.Action)
	event.ResourceType = strings.TrimSpace(event.ResourceType)
	event.ResourceName = strings.TrimSpace(event.ResourceName)
	event.Metadata = auditMetadataWithRequestID(ctx, event.Metadata)
	attribution, _ := ctx.Value(auditAttributionContextKey{}).(auditAttribution)
	event.Actor = sanitizeAuditAttribution(event.Actor)
	if event.Actor == "" {
		event.Actor = attribution.Actor
	}
	event.Source = sanitizeAuditAttribution(event.Source)
	if event.Source == "" {
		event.Source = attribution.Source
	}
	if event.Action == "" || event.ResourceType == "" {
		return
	}
	if event.Source == "" {
		event.Source = auditSourceHTTPAPI
	}
	auditCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	_, _ = api.store.CreateAuditEvent(auditCtx, event)
}

func withAuditAttribution(ctx context.Context, r *http.Request) context.Context {
	attribution := auditAttribution{
		Actor:  sanitizeAuditAttribution(r.Header.Get(auditActorHeader)),
		Source: sanitizeAuditAttribution(r.Header.Get(auditSourceHeader)),
	}
	if attribution.Actor == "" && attribution.Source == "" {
		return ctx
	}
	return context.WithValue(ctx, auditAttributionContextKey{}, attribution)
}

func sanitizeAuditAttribution(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var builder strings.Builder
	count := 0
	for len(value) > 0 && count < maxAuditAttributionRunes {
		r, size := utf8.DecodeRuneInString(value)
		value = value[size:]
		if r == utf8.RuneError && size == 0 {
			break
		}
		if unicode.IsControl(r) {
			continue
		}
		builder.WriteRune(r)
		count++
	}
	return strings.TrimSpace(builder.String())
}

func auditMetadata(values map[string]any) []byte {
	if len(values) == 0 {
		return nil
	}
	data, err := json.Marshal(values)
	if err != nil {
		return nil
	}
	return data
}

func auditMetadataWithRequestID(ctx context.Context, metadata []byte) []byte {
	requestID := requestIDFromContext(ctx)
	if requestID == "" {
		return metadata
	}
	values := map[string]any{}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &values); err != nil {
			return metadata
		}
	}
	if values == nil {
		values = map[string]any{}
	}
	if _, exists := values["requestId"]; exists {
		return metadata
	}
	values["requestId"] = requestID
	data, err := json.Marshal(values)
	if err != nil {
		return metadata
	}
	return data
}

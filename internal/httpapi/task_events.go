package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/mlhiter/mbox/internal/domain"
)

const (
	taskEventTypeSnapshot = "snapshot"
	taskEventTypeStatus   = "status"
	taskEventTypeOutput   = "output"
	taskEventTypeDone     = "done"
)

type executionTaskEvent struct {
	Type      string                `json:"type"`
	Task      *domain.ExecutionTask `json:"task,omitempty"`
	Stream    string                `json:"stream,omitempty"`
	Data      string                `json:"data,omitempty"`
	Offset    int                   `json:"offset,omitempty"`
	CreatedAt time.Time             `json:"createdAt"`
}

type taskEventHub struct {
	mu          sync.Mutex
	events      []executionTaskEvent
	subscribers map[chan executionTaskEvent]struct{}
	closed      bool
}

func newTaskEventHub() *taskEventHub {
	return &taskEventHub{
		events:      []executionTaskEvent{},
		subscribers: map[chan executionTaskEvent]struct{}{},
	}
}

func (h *taskEventHub) publish(event executionTaskEvent) {
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events = append(h.events, event)
	if len(h.events) > 512 {
		h.events = h.events[len(h.events)-512:]
	}
	if h.closed {
		return
	}
	for subscriber := range h.subscribers {
		select {
		case subscriber <- event:
		default:
		}
	}
}

func (h *taskEventHub) subscribe() ([]executionTaskEvent, <-chan executionTaskEvent, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	backlog := append([]executionTaskEvent(nil), h.events...)
	ch := make(chan executionTaskEvent, 64)
	if !h.closed {
		h.subscribers[ch] = struct{}{}
	}
	unsubscribe := func() {
		h.mu.Lock()
		delete(h.subscribers, ch)
		h.mu.Unlock()
	}
	if h.closed {
		close(ch)
	}
	return backlog, ch, unsubscribe
}

func (h *taskEventHub) close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for subscriber := range h.subscribers {
		close(subscriber)
		delete(h.subscribers, subscriber)
	}
}

type taskOutputBuffer struct {
	mu        sync.Mutex
	buffer    bytes.Buffer
	limit     int
	truncated bool
	taskID    uuid.UUID
	stream    string
	hub       *taskEventHub
}

func newTaskOutputBuffer(taskID uuid.UUID, stream string, hub *taskEventHub, limit int) *taskOutputBuffer {
	return &taskOutputBuffer{
		taskID: taskID,
		stream: stream,
		hub:    hub,
		limit:  limit,
	}
}

func (b *taskOutputBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	remaining := b.limit - b.buffer.Len()
	if remaining <= 0 {
		b.truncated = true
		b.mu.Unlock()
		return len(p), nil
	}
	written := p
	if len(p) > remaining {
		written = p[:remaining]
		b.truncated = true
	}
	offset := b.buffer.Len()
	_, _ = b.buffer.Write(written)
	b.mu.Unlock()

	if len(written) > 0 && b.hub != nil {
		b.hub.publish(executionTaskEvent{
			Type:   taskEventTypeOutput,
			Stream: b.stream,
			Data:   string(written),
			Offset: offset,
		})
	}
	return len(p), nil
}

func (b *taskOutputBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

func (b *taskOutputBuffer) Truncated() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.truncated
}

func (api *API) getTaskEventHub(id uuid.UUID) (*taskEventHub, bool) {
	api.taskMu.Lock()
	defer api.taskMu.Unlock()
	hub, ok := api.taskEvents[id]
	return hub, ok
}

func (api *API) getOrCreateTaskEventHub(id uuid.UUID) *taskEventHub {
	api.taskMu.Lock()
	defer api.taskMu.Unlock()
	hub := api.taskEvents[id]
	if hub == nil {
		hub = newTaskEventHub()
		api.taskEvents[id] = hub
	}
	return hub
}

func (api *API) scheduleTaskEventHubCleanup(id uuid.UUID, hub *taskEventHub) {
	time.AfterFunc(10*time.Minute, func() {
		api.taskMu.Lock()
		defer api.taskMu.Unlock()
		if api.taskEvents[id] == hub {
			delete(api.taskEvents, id)
		}
	})
}

func (api *API) watchExecutionTask(w http.ResponseWriter, r *http.Request) {
	task, ok := api.taskFromPath(w, r)
	if !ok {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is not supported")
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(w)
	writeEvent := func(event executionTaskEvent) bool {
		if event.CreatedAt.IsZero() {
			event.CreatedAt = time.Now().UTC()
		}
		if err := encoder.Encode(event); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	if !writeEvent(executionTaskEvent{Type: taskEventTypeSnapshot, Task: &task}) {
		return
	}
	if isTerminalExecutionTaskStatus(task.Status) {
		_ = writeEvent(executionTaskEvent{Type: taskEventTypeDone, Task: &task})
		return
	}

	hub, ok := api.getTaskEventHub(task.ID)
	var events <-chan executionTaskEvent
	var unsubscribe func()
	if ok {
		var backlog []executionTaskEvent
		backlog, events, unsubscribe = hub.subscribe()
		defer unsubscribe()
		for _, event := range backlog {
			if !writeEvent(event) {
				return
			}
			if event.Type == taskEventTypeDone {
				return
			}
		}
	}

	lastUpdatedAt := task.UpdatedAt
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case event, open := <-events:
			if !open {
				events = nil
				continue
			}
			if !writeEvent(event) {
				return
			}
			if event.Task != nil {
				lastUpdatedAt = event.Task.UpdatedAt
			}
			if event.Type == taskEventTypeDone {
				return
			}
		case <-ticker.C:
			latest, err := api.store.GetExecutionTask(r.Context(), task.ID)
			if err != nil {
				return
			}
			if !latest.UpdatedAt.Equal(lastUpdatedAt) {
				eventType := taskEventTypeStatus
				if isTerminalExecutionTaskStatus(latest.Status) {
					eventType = taskEventTypeDone
				}
				if !writeEvent(executionTaskEvent{Type: eventType, Task: &latest}) {
					return
				}
				lastUpdatedAt = latest.UpdatedAt
				if eventType == taskEventTypeDone {
					return
				}
			}
		}
	}
}

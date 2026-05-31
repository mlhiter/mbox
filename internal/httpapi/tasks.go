package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	k8sexec "k8s.io/client-go/util/exec"

	"github.com/mlhiter/mbox/internal/domain"
	mboxruntime "github.com/mlhiter/mbox/internal/runtime"
)

const (
	defaultExecutionTaskTimeoutSeconds = 60
	maxExecutionTaskTimeoutSeconds     = 600
	maxExecutionTaskCommandArgs        = 32
	maxExecutionTaskCommandArgLength   = 4096
	maxExecutionTaskOutputBytes        = 64 * 1024
)

type createExecutionTaskRequest struct {
	Command        []string        `json:"command"`
	TimeoutSeconds int             `json:"timeoutSeconds"`
	Metadata       json.RawMessage `json:"metadata"`
}

func (api *API) listExecutionTasks(w http.ResponseWriter, r *http.Request) {
	sandbox, ok := api.sandboxFromPath(w, r)
	if !ok {
		return
	}
	tasks, err := api.store.ListExecutionTasks(r.Context(), sandbox.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": tasks})
}

func (api *API) getExecutionTask(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(r, "taskID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid task id")
		return
	}
	task, err := api.store.GetExecutionTask(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (api *API) createExecutionTask(w http.ResponseWriter, r *http.Request) {
	if api.access == nil {
		writeError(w, http.StatusServiceUnavailable, "runtime access is not configured")
		return
	}
	sandbox, ok := api.sandboxFromPath(w, r)
	if !ok {
		return
	}
	if sandbox.Status != domain.SandboxStatusRunning {
		writeError(w, http.StatusConflict, "sandbox must be running before starting a task")
		return
	}
	if sandbox.RuntimeRef == nil {
		writeError(w, http.StatusConflict, "sandbox runtime is not ready")
		return
	}

	var req createExecutionTaskRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	timeoutSeconds := defaultExecutionTaskTimeoutSeconds
	if req.TimeoutSeconds != 0 {
		timeoutSeconds = req.TimeoutSeconds
	}
	if err := validateExecutionTaskRequest(req.Command, timeoutSeconds); err != "" {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	runtimeRef := *sandbox.RuntimeRef
	task, err := api.store.CreateExecutionTask(r.Context(), domain.ExecutionTaskCreate{
		ProjectID:      sandbox.ProjectID,
		SandboxID:      sandbox.ID,
		Command:        req.Command,
		TimeoutSeconds: timeoutSeconds,
		RuntimeRef:     &runtimeRef,
		Metadata:       req.Metadata,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}

	api.recordAuditEvent(r.Context(), domain.AuditEventCreate{
		ProjectID:    &task.ProjectID,
		Action:       "execution.task.created",
		ResourceType: "execution-task",
		ResourceID:   &task.ID,
		ResourceName: task.ID.String(),
		Metadata: auditMetadata(map[string]any{
			"sandboxId":       task.SandboxID.String(),
			"commandArgCount": len(task.Command),
			"timeoutSeconds":  task.TimeoutSeconds,
		}),
	})
	api.startExecutionTask(task, runtimeRef)
	writeJSON(w, http.StatusCreated, task)
}

func (api *API) cancelExecutionTask(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(r, "taskID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid task id")
		return
	}
	task, err := api.store.GetExecutionTask(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if isTerminalExecutionTaskStatus(task.Status) {
		writeError(w, http.StatusConflict, "task is already finished")
		return
	}

	api.taskMu.Lock()
	cancel := api.taskCancels[id]
	api.taskMu.Unlock()
	if cancel == nil {
		writeError(w, http.StatusConflict, "task is not running on this server")
		return
	}
	cancel()
	task, err = api.store.GetExecutionTask(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	api.recordAuditEvent(r.Context(), domain.AuditEventCreate{
		ProjectID:    &task.ProjectID,
		Action:       "execution.task.cancel.requested",
		ResourceType: "execution-task",
		ResourceID:   &task.ID,
		ResourceName: task.ID.String(),
		Metadata: auditMetadata(map[string]any{
			"sandboxId": task.SandboxID.String(),
			"status":    task.Status,
		}),
	})
	writeJSON(w, http.StatusOK, task)
}

func (api *API) startExecutionTask(task domain.ExecutionTask, ref domain.RuntimeRef) {
	runCtx, cancel := context.WithCancel(context.Background())
	hub := api.getOrCreateTaskEventHub(task.ID)
	api.taskMu.Lock()
	api.taskCancels[task.ID] = cancel
	api.taskMu.Unlock()

	go func() {
		defer func() {
			cancel()
			api.taskMu.Lock()
			delete(api.taskCancels, task.ID)
			api.taskMu.Unlock()
		}()
		api.runExecutionTask(runCtx, task, ref, hub)
		hub.close()
		api.scheduleTaskEventHubCleanup(task.ID, hub)
	}()
}

func (api *API) runExecutionTask(ctx context.Context, task domain.ExecutionTask, ref domain.RuntimeRef, hub *taskEventHub) domain.ExecutionTask {
	started := time.Now().UTC()
	running := domain.ExecutionTaskStatusRunning
	startUpdateCtx, cancelStartUpdate := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelStartUpdate()
	updated, err := api.store.UpdateExecutionTask(startUpdateCtx, task.ID, domain.ExecutionTaskUpdate{
		Status:    &running,
		StartedAt: &started,
	})
	if err != nil {
		task.Status = running
		task.StartedAt = &started
		if hub != nil {
			hub.publish(executionTaskEvent{Type: taskEventTypeStatus, Task: &task})
			hub.publish(executionTaskEvent{Type: taskEventTypeDone, Task: &task})
		}
		return task
	}
	task = updated
	if hub != nil {
		hub.publish(executionTaskEvent{Type: taskEventTypeStatus, Task: &task})
	}

	runCtx, cancelRun := context.WithTimeout(ctx, time.Duration(task.TimeoutSeconds)*time.Second)
	defer cancelRun()

	stdout := newTaskOutputBuffer(task.ID, "stdout", hub, maxExecutionTaskOutputBytes)
	stderr := newTaskOutputBuffer(task.ID, "stderr", hub, maxExecutionTaskOutputBytes)
	err = api.access.Exec(runCtx, ref, mboxruntime.ExecOptions{
		Command: task.Command,
		Stdout:  stdout,
		Stderr:  stderr,
		TTY:     false,
	})

	finished := time.Now().UTC()
	status := domain.ExecutionTaskStatusSucceeded
	taskError := ""
	successCode := 0
	exitCode := &successCode
	if err != nil {
		status = domain.ExecutionTaskStatusFailed
		taskError = err.Error()
		exitCode = nil
		var exitErr k8sexec.ExitError
		if errors.As(err, &exitErr) && exitErr.Exited() {
			code := exitErr.ExitStatus()
			exitCode = &code
		}
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
			status = domain.ExecutionTaskStatusTimedOut
			taskError = "task timed out"
		} else if errors.Is(runCtx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
			status = domain.ExecutionTaskStatusCanceled
			taskError = "task canceled"
		}
	}
	stdoutString := stdout.String()
	stderrString := stderr.String()
	outputTruncated := stdout.Truncated() || stderr.Truncated()

	updateCtx, cancelUpdate := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancelUpdate()
	updated, updateErr := api.store.UpdateExecutionTask(updateCtx, task.ID, domain.ExecutionTaskUpdate{
		Status:          &status,
		ExitCode:        exitCode,
		Stdout:          &stdoutString,
		Stderr:          &stderrString,
		OutputTruncated: &outputTruncated,
		Error:           &taskError,
		FinishedAt:      &finished,
	})
	if updateErr != nil {
		task.Status = status
		task.ExitCode = exitCode
		task.Stdout = stdoutString
		task.Stderr = stderrString
		task.OutputTruncated = outputTruncated
		task.Error = taskError
		task.FinishedAt = &finished
		if hub != nil {
			hub.publish(executionTaskEvent{Type: taskEventTypeDone, Task: &task})
		}
		return task
	}
	if hub != nil {
		hub.publish(executionTaskEvent{Type: taskEventTypeDone, Task: &updated})
	}
	return updated
}

func (api *API) sandboxFromPath(w http.ResponseWriter, r *http.Request) (domain.Sandbox, bool) {
	id, ok := parseUUIDParam(r, "sandboxID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid sandbox id")
		return domain.Sandbox{}, false
	}
	sandbox, err := api.store.GetSandbox(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return domain.Sandbox{}, false
	}
	return sandbox, true
}

func validateExecutionTaskRequest(command []string, timeoutSeconds int) string {
	if len(command) == 0 {
		return "command is required"
	}
	if len(command) > maxExecutionTaskCommandArgs {
		return "command has too many arguments"
	}
	for _, arg := range command {
		if strings.TrimSpace(arg) == "" {
			return "command arguments cannot be empty"
		}
		if len(arg) > maxExecutionTaskCommandArgLength {
			return "command argument is too long"
		}
	}
	if timeoutSeconds < 1 || timeoutSeconds > maxExecutionTaskTimeoutSeconds {
		return "timeoutSeconds must be between 1 and 600"
	}
	return ""
}

func isTerminalExecutionTaskStatus(status domain.ExecutionTaskStatus) bool {
	switch status {
	case domain.ExecutionTaskStatusSucceeded,
		domain.ExecutionTaskStatusFailed,
		domain.ExecutionTaskStatusCanceled,
		domain.ExecutionTaskStatusTimedOut:
		return true
	default:
		return false
	}
}

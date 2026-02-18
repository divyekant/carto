package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ProgressEvent is sent over SSE to report indexing progress.
type ProgressEvent struct {
	Phase string `json:"phase"`
	Done  int    `json:"done"`
	Total int    `json:"total"`
}

// IndexResult is the final summary sent when an index run completes.
type IndexResult struct {
	Modules int           `json:"modules"`
	Files   int           `json:"files"`
	Atoms   int           `json:"atoms"`
	Errors  int           `json:"errors"`
	Elapsed time.Duration `json:"elapsed"`
	ErrMsgs []string      `json:"error_messages,omitempty"`
}

// IndexRun tracks a single in-flight indexing run for a project.
type IndexRun struct {
	events chan sseEvent
	done   chan struct{}
}

// sseEvent is a typed SSE message sent over the events channel.
type sseEvent struct {
	Event string // SSE event type: "progress", "result", "error"
	Data  string // JSON-encoded payload
}

// SendProgress sends a progress event to the SSE stream.
func (r *IndexRun) SendProgress(phase string, done, total int) {
	data, _ := json.Marshal(ProgressEvent{Phase: phase, Done: done, Total: total})
	select {
	case r.events <- sseEvent{Event: "progress", Data: string(data)}:
	default:
		// Drop event if channel is full (client too slow).
	}
}

// SendResult sends the final result event and closes the done channel.
func (r *IndexRun) SendResult(result IndexResult) {
	data, _ := json.Marshal(result)
	select {
	case r.events <- sseEvent{Event: "result", Data: string(data)}:
	default:
	}
}

// SendError sends an error event.
func (r *IndexRun) SendError(msg string) {
	data, _ := json.Marshal(map[string]string{"error": msg})
	select {
	case r.events <- sseEvent{Event: "error", Data: string(data)}:
	default:
	}
}

// WriteSSE streams events to the HTTP response as text/event-stream.
// It blocks until the run completes or the client disconnects.
func (r *IndexRun) WriteSSE(w http.ResponseWriter, req *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ctx := req.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-r.events:
			if !ok {
				// Channel closed — run finished.
				return
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Event, ev.Data)
			flusher.Flush()
		case <-r.done:
			// Drain remaining events then return.
			for {
				select {
				case ev, ok := <-r.events:
					if !ok {
						return
					}
					fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Event, ev.Data)
					flusher.Flush()
				default:
					return
				}
			}
		}
	}
}

// RunManager tracks active indexing runs by project name.
type RunManager struct {
	mu   sync.Mutex
	runs map[string]*IndexRun
}

// NewRunManager creates an empty RunManager.
func NewRunManager() *RunManager {
	return &RunManager{
		runs: make(map[string]*IndexRun),
	}
}

// Start creates a new IndexRun for the given project.
// Returns nil if a run is already active for that project.
func (m *RunManager) Start(project string) *IndexRun {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.runs[project]; exists {
		return nil
	}

	run := &IndexRun{
		events: make(chan sseEvent, 100),
		done:   make(chan struct{}),
	}
	m.runs[project] = run
	return run
}

// Finish marks the run as done and removes it from the active set.
func (m *RunManager) Finish(project string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if run, exists := m.runs[project]; exists {
		close(run.done)
		close(run.events)
		delete(m.runs, project)
	}
}

// Get returns the active run for a project, or nil if none is active.
func (m *RunManager) Get(project string) *IndexRun {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.runs[project]
}

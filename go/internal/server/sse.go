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
	events    chan sseEvent
	done      chan struct{}
	mu        sync.Mutex
	lastEvent *sseEvent // buffered final event for late-connecting clients
	finished  bool
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

// SendResult sends the final result event.
func (r *IndexRun) SendResult(result IndexResult) {
	data, _ := json.Marshal(result)
	ev := sseEvent{Event: "complete", Data: string(data)}
	r.mu.Lock()
	r.lastEvent = &ev
	r.mu.Unlock()
	select {
	case r.events <- ev:
	default:
	}
}

// SendError sends a pipeline error event.
// Uses "pipeline_error" to avoid collision with the SSE built-in "error" event.
func (r *IndexRun) SendError(msg string) {
	data, _ := json.Marshal(map[string]string{"message": msg})
	ev := sseEvent{Event: "pipeline_error", Data: string(data)}
	r.mu.Lock()
	r.lastEvent = &ev
	r.mu.Unlock()
	select {
	case r.events <- ev:
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

	// If the run already finished (late-connecting client), send the
	// buffered final event immediately.
	r.mu.Lock()
	if r.finished && r.lastEvent != nil {
		ev := *r.lastEvent
		r.mu.Unlock()
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Event, ev.Data)
		flusher.Flush()
		return
	}
	r.mu.Unlock()

	ctx := req.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-r.events:
			if !ok {
				// Channel closed — run finished. Send last event if we missed it.
				r.mu.Lock()
				last := r.lastEvent
				r.mu.Unlock()
				if last != nil {
					fmt.Fprintf(w, "event: %s\ndata: %s\n\n", last.Event, last.Data)
					flusher.Flush()
				}
				return
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Event, ev.Data)
			flusher.Flush()
		case <-r.done:
			// Drain remaining events then send last event.
			for {
				select {
				case _, ok := <-r.events:
					if !ok {
						break
					}
					continue
				default:
				}
				break
			}
			r.mu.Lock()
			last := r.lastEvent
			r.mu.Unlock()
			if last != nil {
				fmt.Fprintf(w, "event: %s\ndata: %s\n\n", last.Event, last.Data)
				flusher.Flush()
			}
			return
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
// Returns nil if a run is already active (and not finished) for that project.
func (m *RunManager) Start(project string) *IndexRun {
	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, exists := m.runs[project]; exists {
		existing.mu.Lock()
		done := existing.finished
		existing.mu.Unlock()
		if !done {
			return nil // still running
		}
		// Old run finished — replace it.
	}

	run := &IndexRun{
		events: make(chan sseEvent, 100),
		done:   make(chan struct{}),
	}
	m.runs[project] = run
	return run
}

// Finish marks the run as done. The run stays in the map for 30 seconds
// so late-connecting SSE clients can still read the final event.
func (m *RunManager) Finish(project string) {
	m.mu.Lock()
	run, exists := m.runs[project]
	if !exists {
		m.mu.Unlock()
		return
	}
	run.mu.Lock()
	run.finished = true
	run.mu.Unlock()
	close(run.done)
	close(run.events)
	m.mu.Unlock()

	// Clean up after a delay so late SSE clients can still connect.
	go func() {
		time.Sleep(30 * time.Second)
		m.mu.Lock()
		delete(m.runs, project)
		m.mu.Unlock()
	}()
}

// Get returns the active run for a project, or nil if none is active.
func (m *RunManager) Get(project string) *IndexRun {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.runs[project]
}

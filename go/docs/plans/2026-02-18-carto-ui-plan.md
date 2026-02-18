# Carto Management UI Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a React SPA management UI for Carto — Dashboard, Index with SSE progress, Query, and Settings — embedded into the Go binary via `go:embed`.

**Architecture:** Go HTTP server (existing skeleton) gets REST API handlers + SSE streaming. React 19 + Vite + Tailwind + shadcn/ui SPA in `go/web/`. Built output embedded via `//go:embed web/dist/*` for single-binary distribution.

**Tech Stack:** Go 1.22+, React 19, Vite, Tailwind CSS, shadcn/ui, Server-Sent Events

---

## Phase 1: Go API Endpoints

Build all REST API handlers and SSE streaming before touching frontend. Each endpoint is independently testable via `httptest`.

---

### Task 1.1: List Projects endpoint

**Files:**
- Modify: `internal/server/server.go` — add `projectsDir` field
- Modify: `internal/server/routes.go` — register route
- Create: `internal/server/handlers.go` — handler implementations
- Modify: `internal/server/server_test.go` — add test

Projects are discovered by scanning a configurable directory for `.carto/manifest.json` files. For simplicity, the server accepts a `--projects-dir` flag (defaults to `$HOME/projects`).

**Step 1: Write the failing test**

In `internal/server/server_test.go`, add:

```go
func TestListProjects(t *testing.T) {
	// Create temp dir with two fake projects that have manifests.
	tmpDir := t.TempDir()

	proj1 := filepath.Join(tmpDir, "proj1", ".carto")
	os.MkdirAll(proj1, 0o755)
	os.WriteFile(filepath.Join(proj1, "manifest.json"), []byte(`{
		"version": "1.0",
		"project": "proj1",
		"indexed_at": "2026-02-18T10:00:00Z",
		"files": {"main.go": {"hash": "abc", "size": 100, "indexed_at": "2026-02-18T10:00:00Z"}}
	}`), 0o644)

	proj2 := filepath.Join(tmpDir, "proj2", ".carto")
	os.MkdirAll(proj2, 0o755)
	os.WriteFile(filepath.Join(proj2, "manifest.json"), []byte(`{
		"version": "1.0",
		"project": "proj2",
		"indexed_at": "2026-02-18T09:00:00Z",
		"files": {}
	}`), 0o644)

	// No manifest — should not appear.
	os.MkdirAll(filepath.Join(tmpDir, "no-manifest"), 0o755)

	memSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer memSrv.Close()

	memoriesClient := storage.NewMemoriesClient(memSrv.URL, "test-key")
	srv := New(config.Config{}, memoriesClient, tmpDir)

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Projects []ProjectInfo `json:"projects"`
	}
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(resp.Projects))
	}
}
```

Add required imports: `"os"`, `"path/filepath"`.

**Step 2: Update Server struct and New() signature**

In `internal/server/server.go`, add `projectsDir string` field:

```go
type Server struct {
	cfg            config.Config
	memoriesClient *storage.MemoriesClient
	projectsDir    string
	mux            *http.ServeMux
}

func New(cfg config.Config, memoriesClient *storage.MemoriesClient, projectsDir string) *Server {
	s := &Server{
		cfg:            cfg,
		memoriesClient: memoriesClient,
		projectsDir:    projectsDir,
		mux:            http.NewServeMux(),
	}
	s.routes()
	return s
}
```

Update **all existing callers** of `New()`:
- `internal/server/server_test.go`: `TestHealthEndpoint` and `TestHealthEndpoint_MemoriesDown` — add `""` as third arg
- `cmd/carto/main.go` in `runServe`: add `projectsDir` arg

**Step 3: Create handlers.go with ListProjects**

Create `internal/server/handlers.go`:

```go
package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/anthropic/indexer/internal/manifest"
)

// ProjectInfo is the API representation of an indexed project.
type ProjectInfo struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	IndexedAt time.Time `json:"indexed_at"`
	FileCount int       `json:"file_count"`
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	var projects []ProjectInfo

	if s.projectsDir == "" {
		writeJSON(w, http.StatusOK, map[string]any{"projects": projects})
		return
	}

	entries, err := os.ReadDir(s.projectsDir)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "read projects dir: " + err.Error()})
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		projPath := filepath.Join(s.projectsDir, entry.Name())
		mf, err := manifest.Load(projPath)
		if err != nil || mf.IsEmpty() {
			continue
		}
		projects = append(projects, ProjectInfo{
			Name:      mf.Project,
			Path:      projPath,
			IndexedAt: mf.IndexedAt,
			FileCount: len(mf.Files),
		})
	}

	if projects == nil {
		projects = []ProjectInfo{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"projects": projects})
}

// writeJSON encodes v as JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
```

**Step 4: Register route in routes.go**

Add to `routes()`:

```go
s.mux.HandleFunc("GET /api/projects", s.handleListProjects)
```

Also move `handleHealth` to use `writeJSON`:

```go
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	healthy, _ := s.memoriesClient.Health()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"memories_healthy": healthy,
	})
}
```

**Step 5: Update cmd/carto/main.go**

In `serveCmd()`, add `--projects-dir` flag:

```go
func serveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the Carto web UI",
		RunE:  runServe,
	}
	cmd.Flags().String("port", "8950", "Port to listen on")
	cmd.Flags().String("projects-dir", "", "Directory containing projects to manage")
	return cmd
}

func runServe(cmd *cobra.Command, args []string) error {
	cfg := config.Load()
	port, _ := cmd.Flags().GetString("port")
	projectsDir, _ := cmd.Flags().GetString("projects-dir")

	memoriesClient := storage.NewMemoriesClient(cfg.MemoriesURL, cfg.MemoriesKey)

	srv := server.New(cfg, memoriesClient, projectsDir)
	fmt.Printf("%s%sCarto server%s starting on http://localhost:%s\n", bold, cyan, reset, port)
	return srv.Start(":" + port)
}
```

**Step 6: Run tests**

```bash
cd /Users/dk/projects/indexer/go && go test ./internal/server/... -v -count=1
cd /Users/dk/projects/indexer/go && go test ./cmd/carto/... -v -count=1
cd /Users/dk/projects/indexer/go && go build ./...
```

Expected: ALL PASS, clean build.

**Step 7: Commit**

```bash
git add internal/server/ cmd/carto/main.go
git commit -m "feat(server): add GET /api/projects endpoint with manifest discovery"
```

---

### Task 1.2: Query endpoint

**Files:**
- Modify: `internal/server/handlers.go`
- Modify: `internal/server/routes.go`
- Modify: `internal/server/server_test.go`

**Step 1: Write the failing test**

In `internal/server/server_test.go`, add:

```go
func TestQueryEndpoint(t *testing.T) {
	memSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/search" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"results": []map[string]any{
					{"id": 1, "text": "func main() {}", "score": 0.95, "source": "carto/proj/mod/layer:atoms"},
				},
			})
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer memSrv.Close()

	memoriesClient := storage.NewMemoriesClient(memSrv.URL, "test-key")
	srv := New(config.Config{}, memoriesClient, "")

	body := `{"text":"how does auth work","project":"myapp","tier":"standard","k":5}`
	req := httptest.NewRequest(http.MethodPost, "/api/query", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Results []storage.SearchResult `json:"results"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Results) == 0 {
		t.Fatal("expected at least one result")
	}
}

func TestQueryEndpoint_MissingText(t *testing.T) {
	memSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer memSrv.Close()

	memoriesClient := storage.NewMemoriesClient(memSrv.URL, "test-key")
	srv := New(config.Config{}, memoriesClient, "")

	body := `{"text":"","project":"myapp"}`
	req := httptest.NewRequest(http.MethodPost, "/api/query", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty text, got %d", w.Code)
	}
}
```

Add `"strings"` to imports.

**Step 2: Implement handler in handlers.go**

```go
// QueryRequest is the body for POST /api/query.
type QueryRequest struct {
	Text    string `json:"text"`
	Project string `json:"project"`
	Tier    string `json:"tier"`
	K       int    `json:"k"`
}

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	var req QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	if req.Text == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "text is required"})
		return
	}

	k := req.K
	if k <= 0 {
		k = 10
	}

	if req.Project != "" {
		store := storage.NewStore(s.memoriesClient, req.Project)
		tier := storage.Tier(req.Tier)
		if tier == "" {
			tier = storage.TierStandard
		}
		results, err := store.RetrieveByTier(req.Text, tier)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		// Flatten tier results into a single list.
		var flat []storage.SearchResult
		for _, entries := range results {
			flat = append(flat, entries...)
		}
		writeJSON(w, http.StatusOK, map[string]any{"results": flat})
		return
	}

	// Free-form search across all projects.
	results, err := s.memoriesClient.Search(req.Text, storage.SearchOptions{
		K:      k,
		Hybrid: true,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}
```

Add `"github.com/anthropic/indexer/internal/storage"` to `handlers.go` imports.

**Step 3: Register route**

In `routes.go`, add:

```go
s.mux.HandleFunc("POST /api/query", s.handleQuery)
```

**Step 4: Run tests**

```bash
cd /Users/dk/projects/indexer/go && go test ./internal/server/... -v -count=1
```

Expected: ALL PASS.

**Step 5: Commit**

```bash
git add internal/server/
git commit -m "feat(server): add POST /api/query endpoint with tier-based retrieval"
```

---

### Task 1.3: Config endpoints (GET + PATCH)

**Files:**
- Modify: `internal/server/handlers.go`
- Modify: `internal/server/server.go` — store mutable config
- Modify: `internal/server/routes.go`
- Modify: `internal/server/server_test.go`

The server needs a mutable config for runtime changes. Store a `*config.Config` (pointer) in the Server struct, protected by a `sync.RWMutex`.

**Step 1: Write tests**

In `internal/server/server_test.go`:

```go
func TestGetConfig(t *testing.T) {
	memSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer memSrv.Close()

	memoriesClient := storage.NewMemoriesClient(memSrv.URL, "test-key")
	cfg := config.Config{
		LLMProvider: "anthropic",
		LLMApiKey:   "sk-ant-secret-key-12345",
		MemoriesURL: "http://localhost:8900",
		MemoriesKey: "my-secret-key",
		HaikuModel:  "claude-haiku-4-5-20251001",
		OpusModel:   "claude-opus-4-6",
	}
	srv := New(cfg, memoriesClient, "")

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)

	// API keys should be redacted.
	if resp["llm_api_key"] == "sk-ant-secret-key-12345" {
		t.Error("API key should be redacted")
	}
	if !strings.Contains(resp["llm_api_key"], "****") {
		t.Error("redacted key should contain ****")
	}
	if resp["llm_provider"] != "anthropic" {
		t.Errorf("expected provider 'anthropic', got %q", resp["llm_provider"])
	}
}

func TestPatchConfig(t *testing.T) {
	memSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer memSrv.Close()

	memoriesClient := storage.NewMemoriesClient(memSrv.URL, "test-key")
	srv := New(config.Config{LLMProvider: "anthropic"}, memoriesClient, "")

	body := `{"llm_provider":"openai","llm_base_url":"https://api.openai.com"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify it changed by GETting config.
	req2 := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)

	var resp map[string]string
	json.NewDecoder(w2.Body).Decode(&resp)
	if resp["llm_provider"] != "openai" {
		t.Errorf("expected provider 'openai' after patch, got %q", resp["llm_provider"])
	}
}
```

**Step 2: Update Server struct for mutable config**

In `internal/server/server.go`, change `cfg` from value to pointer, add mutex:

```go
import (
	"log"
	"net/http"
	"sync"

	"github.com/anthropic/indexer/internal/config"
	"github.com/anthropic/indexer/internal/storage"
)

type Server struct {
	cfg            config.Config
	cfgMu          sync.RWMutex
	memoriesClient *storage.MemoriesClient
	projectsDir    string
	mux            *http.ServeMux
}
```

**Step 3: Implement handlers**

In `internal/server/handlers.go`:

```go
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	s.cfgMu.RLock()
	cfg := s.cfg
	s.cfgMu.RUnlock()

	writeJSON(w, http.StatusOK, map[string]string{
		"llm_provider":  cfg.LLMProvider,
		"llm_api_key":   redactKey(cfg.LLMApiKey),
		"llm_base_url":  cfg.LLMBaseURL,
		"haiku_model":   cfg.HaikuModel,
		"opus_model":    cfg.OpusModel,
		"memories_url":  cfg.MemoriesURL,
		"memories_key":  redactKey(cfg.MemoriesKey),
		"anthropic_key": redactKey(cfg.AnthropicKey),
	})
}

func (s *Server) handlePatchConfig(w http.ResponseWriter, r *http.Request) {
	var patch map[string]string
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	s.cfgMu.Lock()
	for k, v := range patch {
		switch k {
		case "llm_provider":
			s.cfg.LLMProvider = v
		case "llm_api_key":
			s.cfg.LLMApiKey = v
		case "llm_base_url":
			s.cfg.LLMBaseURL = v
		case "haiku_model":
			s.cfg.HaikuModel = v
		case "opus_model":
			s.cfg.OpusModel = v
		case "memories_url":
			s.cfg.MemoriesURL = v
		case "memories_key":
			s.cfg.MemoriesKey = v
		}
	}
	s.cfgMu.Unlock()

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// redactKey masks all but the first 8 and last 4 characters of a key.
func redactKey(key string) string {
	if len(key) <= 12 {
		if len(key) == 0 {
			return ""
		}
		return "****"
	}
	return key[:8] + "****" + key[len(key)-4:]
}
```

**Step 4: Register routes**

```go
s.mux.HandleFunc("GET /api/config", s.handleGetConfig)
s.mux.HandleFunc("PATCH /api/config", s.handlePatchConfig)
```

**Step 5: Run tests**

```bash
cd /Users/dk/projects/indexer/go && go test ./internal/server/... -v -count=1
```

Expected: ALL PASS.

**Step 6: Commit**

```bash
git add internal/server/
git commit -m "feat(server): add GET/PATCH /api/config endpoints with key redaction"
```

---

### Task 1.4: Index trigger endpoint + SSE progress

**Files:**
- Create: `internal/server/sse.go`
- Modify: `internal/server/handlers.go`
- Modify: `internal/server/server.go` — add runs map
- Modify: `internal/server/routes.go`
- Modify: `internal/server/server_test.go`

This is the most complex API endpoint. `POST /api/projects/index` starts the pipeline in a goroutine. `GET /api/projects/{name}/progress` is an SSE stream that forwards pipeline progress events.

**Step 1: Write SSE types in sse.go**

Create `internal/server/sse.go`:

```go
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ProgressEvent is a single progress update from the pipeline.
type ProgressEvent struct {
	Phase string `json:"phase"`
	Done  int    `json:"done"`
	Total int    `json:"total"`
}

// IndexResult is the final summary sent when indexing completes.
type IndexResult struct {
	Modules int      `json:"modules"`
	Files   int      `json:"files"`
	Atoms   int      `json:"atoms"`
	Errors  int      `json:"errors"`
	Elapsed string   `json:"elapsed"`
	ErrMsgs []string `json:"error_messages,omitempty"`
}

// IndexRun tracks an active index operation.
type IndexRun struct {
	Project   string
	StartedAt time.Time
	events    chan any    // ProgressEvent, IndexResult, or error
	done      chan struct{}
}

// RunManager tracks active index runs, one per project.
type RunManager struct {
	mu   sync.Mutex
	runs map[string]*IndexRun
}

// NewRunManager creates an empty run manager.
func NewRunManager() *RunManager {
	return &RunManager{runs: make(map[string]*IndexRun)}
}

// Start begins tracking a new run. Returns nil if a run is already active.
func (rm *RunManager) Start(project string) *IndexRun {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.runs[project]; exists {
		return nil
	}

	run := &IndexRun{
		Project:   project,
		StartedAt: time.Now(),
		events:    make(chan any, 100),
		done:      make(chan struct{}),
	}
	rm.runs[project] = run
	return run
}

// Finish removes a run from tracking.
func (rm *RunManager) Finish(project string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	delete(rm.runs, project)
}

// Get returns the active run for a project, or nil.
func (rm *RunManager) Get(project string) *IndexRun {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	return rm.runs[project]
}

// SendProgress sends a progress event to the run's event channel.
func (run *IndexRun) SendProgress(phase string, done, total int) {
	select {
	case run.events <- ProgressEvent{Phase: phase, Done: done, Total: total}:
	default:
		// Drop if channel is full — SSE client is too slow.
	}
}

// SendResult sends the final result and closes the run.
func (run *IndexRun) SendResult(result IndexResult) {
	select {
	case run.events <- result:
	default:
	}
	close(run.done)
}

// SendError sends an error event and closes the run.
func (run *IndexRun) SendError(err error) {
	select {
	case run.events <- err:
	default:
	}
	close(run.done)
}

// WriteSSE streams events to an HTTP response as Server-Sent Events.
func (run *IndexRun) WriteSSE(w http.ResponseWriter) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	flusher.Flush()

	for {
		select {
		case evt, ok := <-run.events:
			if !ok {
				return
			}
			switch v := evt.(type) {
			case ProgressEvent:
				data, _ := json.Marshal(v)
				fmt.Fprintf(w, "event: progress\ndata: %s\n\n", data)
			case IndexResult:
				data, _ := json.Marshal(v)
				fmt.Fprintf(w, "event: complete\ndata: %s\n\n", data)
			case error:
				data, _ := json.Marshal(map[string]string{"message": v.Error()})
				fmt.Fprintf(w, "event: error\ndata: %s\n\n", data)
			}
			flusher.Flush()
		case <-run.done:
			// Drain remaining events.
			for evt := range run.events {
				switch v := evt.(type) {
				case ProgressEvent:
					data, _ := json.Marshal(v)
					fmt.Fprintf(w, "event: progress\ndata: %s\n\n", data)
				case IndexResult:
					data, _ := json.Marshal(v)
					fmt.Fprintf(w, "event: complete\ndata: %s\n\n", data)
				case error:
					data, _ := json.Marshal(map[string]string{"message": v.Error()})
					fmt.Fprintf(w, "event: error\ndata: %s\n\n", data)
				}
				flusher.Flush()
			}
			return
		}
	}
}
```

**Step 2: Add RunManager to Server**

In `internal/server/server.go`:

```go
type Server struct {
	cfg            config.Config
	cfgMu          sync.RWMutex
	memoriesClient *storage.MemoriesClient
	projectsDir    string
	runs           *RunManager
	mux            *http.ServeMux
}

func New(cfg config.Config, memoriesClient *storage.MemoriesClient, projectsDir string) *Server {
	s := &Server{
		cfg:            cfg,
		memoriesClient: memoriesClient,
		projectsDir:    projectsDir,
		runs:           NewRunManager(),
		mux:            http.NewServeMux(),
	}
	s.routes()
	return s
}
```

**Step 3: Implement handlers**

In `internal/server/handlers.go`, add:

```go
import (
	"path/filepath"
	"time"

	"github.com/anthropic/indexer/internal/config"
	"github.com/anthropic/indexer/internal/llm"
	"github.com/anthropic/indexer/internal/pipeline"
	"github.com/anthropic/indexer/internal/scanner"
	"github.com/anthropic/indexer/internal/signals"
)

// IndexRequest is the body for POST /api/projects/index.
type IndexRequest struct {
	Path        string `json:"path"`
	Incremental bool   `json:"incremental"`
	Module      string `json:"module"`
	Project     string `json:"project"`
}

func (s *Server) handleStartIndex(w http.ResponseWriter, r *http.Request) {
	var req IndexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	if req.Path == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "path is required"})
		return
	}

	absPath, err := filepath.Abs(req.Path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid path: " + err.Error()})
		return
	}

	projectName := req.Project
	if projectName == "" {
		projectName = filepath.Base(absPath)
	}

	run := s.runs.Start(projectName)
	if run == nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "index already running for project " + projectName})
		return
	}

	// Launch pipeline in background goroutine.
	go func() {
		defer s.runs.Finish(projectName)

		s.cfgMu.RLock()
		cfg := s.cfg
		s.cfgMu.RUnlock()

		apiKey := cfg.LLMApiKey
		if apiKey == "" {
			apiKey = cfg.AnthropicKey
		}

		llmClient := llm.NewClient(llm.Options{
			APIKey:        apiKey,
			HaikuModel:    cfg.HaikuModel,
			OpusModel:     cfg.OpusModel,
			MaxConcurrent: cfg.MaxConcurrent,
			IsOAuth:       config.IsOAuthToken(apiKey),
			BaseURL:       cfg.LLMBaseURL,
		})

		registry := signals.NewRegistry()
		registry.Register(signals.NewGitSignalSource(absPath))

		startTime := time.Now()

		result, pipeErr := pipeline.Run(pipeline.Config{
			ProjectName:    projectName,
			RootPath:       absPath,
			LLMClient:      llmClient,
			MemoriesClient: s.memoriesClient,
			SignalRegistry: registry,
			MaxWorkers:     cfg.MaxConcurrent,
			ProgressFn: func(phase string, done, total int) {
				run.SendProgress(phase, done, total)
			},
			Incremental:  req.Incremental,
			ModuleFilter: req.Module,
		})

		if pipeErr != nil {
			run.SendError(pipeErr)
			return
		}

		errMsgs := make([]string, len(result.Errors))
		for i, e := range result.Errors {
			errMsgs[i] = e.Error()
		}

		run.SendResult(IndexResult{
			Modules: result.Modules,
			Files:   result.FilesIndexed,
			Atoms:   result.AtomsCreated,
			Errors:  len(result.Errors),
			Elapsed: time.Since(startTime).Round(time.Millisecond).String(),
			ErrMsgs: errMsgs,
		})
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{
		"project": projectName,
		"status":  "started",
	})
}

func (s *Server) handleProgress(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project name required"})
		return
	}

	run := s.runs.Get(name)
	if run == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no active index run for project " + name})
		return
	}

	run.WriteSSE(w)
}
```

**Step 4: Register routes**

```go
s.mux.HandleFunc("POST /api/projects/index", s.handleStartIndex)
s.mux.HandleFunc("GET /api/projects/{name}/progress", s.handleProgress)
```

**Step 5: Write tests**

In `internal/server/server_test.go`:

```go
func TestStartIndex_Conflict(t *testing.T) {
	memSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer memSrv.Close()

	memoriesClient := storage.NewMemoriesClient(memSrv.URL, "test-key")
	srv := New(config.Config{}, memoriesClient, "")

	// Manually start a run to simulate one in progress.
	srv.runs.Start("myproject")

	body := `{"path":"/tmp/fake","project":"myproject"}`
	req := httptest.NewRequest(http.MethodPost, "/api/projects/index", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 conflict, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStartIndex_MissingPath(t *testing.T) {
	memSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer memSrv.Close()

	memoriesClient := storage.NewMemoriesClient(memSrv.URL, "test-key")
	srv := New(config.Config{}, memoriesClient, "")

	body := `{"path":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/projects/index", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSSE_NoActiveRun(t *testing.T) {
	memSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer memSrv.Close()

	memoriesClient := storage.NewMemoriesClient(memSrv.URL, "test-key")
	srv := New(config.Config{}, memoriesClient, "")

	req := httptest.NewRequest(http.MethodGet, "/api/projects/nonexistent/progress", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestRunManager_StartAndFinish(t *testing.T) {
	rm := NewRunManager()

	run := rm.Start("proj1")
	if run == nil {
		t.Fatal("expected run to start")
	}

	// Second start for same project should return nil.
	if rm.Start("proj1") != nil {
		t.Fatal("expected nil for duplicate start")
	}

	rm.Finish("proj1")

	// Should be able to start again.
	run2 := rm.Start("proj1")
	if run2 == nil {
		t.Fatal("expected run to start after finish")
	}
}
```

**Step 6: Run tests**

```bash
cd /Users/dk/projects/indexer/go && go test ./internal/server/... -v -count=1
cd /Users/dk/projects/indexer/go && go build ./...
```

Expected: ALL PASS, clean build.

**Step 7: Commit**

```bash
git add internal/server/
git commit -m "feat(server): add index trigger, SSE progress streaming, and run manager"
```

---

## Phase 2: React SPA Scaffold

Set up the React project with Vite, Tailwind, shadcn/ui, and basic routing.

---

### Task 2.1: Initialize React project with Vite + Tailwind

**Files:**
- Create: `go/web/` directory with Vite React TypeScript scaffold
- Create: `go/web/vite.config.ts` — with API proxy
- Create: `go/web/tailwind.config.js`
- Create: `go/web/postcss.config.js`

**Step 1: Scaffold Vite project**

```bash
cd /Users/dk/projects/indexer/go
npm create vite@latest web -- --template react-ts
```

**Step 2: Install dependencies**

```bash
cd /Users/dk/projects/indexer/go/web
npm install
npm install -D tailwindcss @tailwindcss/vite
```

**Step 3: Configure Vite with API proxy and Tailwind**

Replace `go/web/vite.config.ts`:

```ts
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    proxy: {
      '/api': 'http://localhost:8950',
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
```

**Step 4: Add Tailwind to CSS**

Replace `go/web/src/index.css`:

```css
@import "tailwindcss";
```

**Step 5: Clean up scaffolded files**

Remove `go/web/src/App.css`. Replace `go/web/src/App.tsx`:

```tsx
function App() {
  return (
    <div className="min-h-screen bg-zinc-950 text-zinc-100 flex items-center justify-center">
      <h1 className="text-3xl font-bold">Carto</h1>
    </div>
  )
}

export default App
```

**Step 6: Verify it builds**

```bash
cd /Users/dk/projects/indexer/go/web && npm run build
```

Expected: `dist/` directory created with `index.html` and JS/CSS assets.

**Step 7: Update .gitignore**

In `/Users/dk/projects/indexer/.gitignore`, verify `node_modules/` and `dist/` are already present (they are). Add `go/web/dist/` if not covered by `dist/`.

**Step 8: Commit**

```bash
cd /Users/dk/projects/indexer
git add go/web/ -f
git add .gitignore
git commit -m "feat(web): scaffold React + Vite + Tailwind project"
```

Note: Use `git add go/web/ -f` if gitignore is blocking. But be selective — do NOT add `go/web/node_modules/` or `go/web/dist/`.

Actually, better:
```bash
cd /Users/dk/projects/indexer
git add go/web/package.json go/web/package-lock.json go/web/tsconfig*.json go/web/vite.config.ts go/web/index.html go/web/src/ go/web/public/ go/web/eslint.config.js
git commit -m "feat(web): scaffold React + Vite + Tailwind project"
```

---

### Task 2.2: Install shadcn/ui and add core components

**Files:**
- Modify: `go/web/` — add shadcn/ui dependencies and components

**Step 1: Install shadcn/ui**

```bash
cd /Users/dk/projects/indexer/go/web
npx shadcn@latest init
```

When prompted:
- Style: New York
- Base color: Zinc
- CSS variables: Yes

**Step 2: Add needed components**

```bash
cd /Users/dk/projects/indexer/go/web
npx shadcn@latest add button card input label select tabs badge progress separator
```

**Step 3: Verify build**

```bash
cd /Users/dk/projects/indexer/go/web && npm run build
```

**Step 4: Commit**

```bash
cd /Users/dk/projects/indexer
git add go/web/
git commit -m "feat(web): add shadcn/ui with core components"
```

---

### Task 2.3: Add React Router + Layout shell

**Files:**
- Modify: `go/web/src/App.tsx`
- Create: `go/web/src/components/Layout.tsx`
- Create: `go/web/src/pages/Dashboard.tsx` (placeholder)
- Create: `go/web/src/pages/IndexRun.tsx` (placeholder)
- Create: `go/web/src/pages/Query.tsx` (placeholder)
- Create: `go/web/src/pages/Settings.tsx` (placeholder)

**Step 1: Install React Router**

```bash
cd /Users/dk/projects/indexer/go/web
npm install react-router-dom
```

**Step 2: Create Layout component**

Create `go/web/src/components/Layout.tsx`:

```tsx
import { NavLink, Outlet } from 'react-router-dom'
import { cn } from '@/lib/utils'

const navItems = [
  { to: '/', label: 'Dashboard', icon: '◫' },
  { to: '/index', label: 'Index', icon: '⟳' },
  { to: '/query', label: 'Query', icon: '⌕' },
  { to: '/settings', label: 'Settings', icon: '⚙' },
]

export function Layout() {
  return (
    <div className="flex h-screen bg-zinc-950 text-zinc-100">
      <aside className="w-56 border-r border-zinc-800 flex flex-col">
        <div className="p-4 border-b border-zinc-800">
          <h1 className="text-xl font-bold tracking-tight">Carto</h1>
          <p className="text-xs text-zinc-500">Codebase Intelligence</p>
        </div>
        <nav className="flex-1 p-2 space-y-1">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === '/'}
              className={({ isActive }) =>
                cn(
                  'flex items-center gap-2 px-3 py-2 rounded-md text-sm transition-colors',
                  isActive
                    ? 'bg-zinc-800 text-zinc-100'
                    : 'text-zinc-400 hover:text-zinc-100 hover:bg-zinc-800/50'
                )
              }
            >
              <span className="text-base">{item.icon}</span>
              {item.label}
            </NavLink>
          ))}
        </nav>
      </aside>
      <main className="flex-1 overflow-y-auto p-6">
        <Outlet />
      </main>
    </div>
  )
}
```

**Step 3: Create placeholder pages**

Create `go/web/src/pages/Dashboard.tsx`:
```tsx
export default function Dashboard() {
  return <div><h2 className="text-2xl font-bold mb-4">Dashboard</h2><p className="text-zinc-400">Project overview coming soon.</p></div>
}
```

Create `go/web/src/pages/IndexRun.tsx`:
```tsx
export default function IndexRun() {
  return <div><h2 className="text-2xl font-bold mb-4">Index</h2><p className="text-zinc-400">Index trigger coming soon.</p></div>
}
```

Create `go/web/src/pages/Query.tsx`:
```tsx
export default function Query() {
  return <div><h2 className="text-2xl font-bold mb-4">Query</h2><p className="text-zinc-400">Query interface coming soon.</p></div>
}
```

Create `go/web/src/pages/Settings.tsx`:
```tsx
export default function Settings() {
  return <div><h2 className="text-2xl font-bold mb-4">Settings</h2><p className="text-zinc-400">Settings coming soon.</p></div>
}
```

**Step 4: Wire up App.tsx with router**

Replace `go/web/src/App.tsx`:

```tsx
import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { Layout } from './components/Layout'
import Dashboard from './pages/Dashboard'
import IndexRun from './pages/IndexRun'
import Query from './pages/Query'
import Settings from './pages/Settings'

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<Layout />}>
          <Route path="/" element={<Dashboard />} />
          <Route path="/index" element={<IndexRun />} />
          <Route path="/query" element={<Query />} />
          <Route path="/settings" element={<Settings />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}

export default App
```

**Step 5: Build and verify**

```bash
cd /Users/dk/projects/indexer/go/web && npm run build
```

**Step 6: Commit**

```bash
cd /Users/dk/projects/indexer
git add go/web/
git commit -m "feat(web): add React Router layout shell with 4 page placeholders"
```

---

## Phase 3: go:embed Integration

Wire the built React SPA into the Go binary.

---

### Task 3.1: Embed SPA and add fallback routing

**Files:**
- Modify: `internal/server/server.go` — add go:embed + SPA handler
- Modify: `internal/server/routes.go` — register SPA fallback
- Modify: `internal/server/server_test.go`

**Step 1: Write test for SPA serving**

In `internal/server/server_test.go`:

```go
func TestSPAFallback(t *testing.T) {
	memSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer memSrv.Close()

	memoriesClient := storage.NewMemoriesClient(memSrv.URL, "test-key")
	srv := New(config.Config{}, memoriesClient, "")

	// Request to a non-API path should get HTML (SPA fallback).
	req := httptest.NewRequest(http.MethodGet, "/query", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// Should return 200 with HTML content (or 404 if no embed — that's ok in tests).
	// In test mode without embedded files, we expect the fallback handler to return
	// something. The key test is that it doesn't 405 or panic.
	if w.Code == http.StatusMethodNotAllowed {
		t.Fatalf("SPA routes should not return 405")
	}
}
```

**Step 2: Add embed to server.go**

In `internal/server/server.go`, add at the package level:

```go
import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"sync"

	"github.com/anthropic/indexer/internal/config"
	"github.com/anthropic/indexer/internal/storage"
)

//go:embed all:web/dist
var webFS embed.FS
```

Wait — the embed path is relative to the file's package location. The server.go is at `internal/server/server.go` but the web dist is at `web/dist/`. `go:embed` can only embed files relative to the package directory or subdirectories.

We need to put the embed directive in a file at the `go/` root level (e.g., `cmd/carto/` or a new `embed.go` at root). The best approach: create `go/web.go` at the module root with the embed, then pass the FS to the server.

**Revised approach:**

Create `go/web.go`:

```go
package indexer

import "embed"

//go:embed web/dist/*
var WebFS embed.FS
```

No — this creates an `indexer` package that nothing imports. Better: put the embed in `cmd/carto/` since that's where `main` is and it already references `server`.

Create `go/cmd/carto/embed.go`:

```go
package main

import "embed"

//go:embed web/dist/*
var webDistFS embed.FS
```

Wait, this also has path issues. `cmd/carto/embed.go` can only embed from `cmd/carto/` subtree.

The cleanest approach: **pass the embed.FS to the server from main**, and have the embed directive in a file at the Go module root that's in `package main` — no, that's not valid.

OK, the standard pattern is:
1. Put the embed in the server package, but the `web/dist` needs to be at `internal/server/web/dist`
2. OR symlink
3. OR pass fs.FS to server.New()

**Option 3 is cleanest**: The server accepts an `fs.FS` parameter. In production, `main.go` passes the embedded FS. In tests, pass `nil` or an in-memory FS.

Let me revise:

In `go/cmd/carto/main.go`, we can't embed `../../web/dist` — embed only works for the package's directory tree.

**Final approach**: Put a `go/web/embed.go` file in a `web` package:

```go
package web

import "embed"

//go:embed all:dist/*
var DistFS embed.FS
```

Then `cmd/carto/main.go` imports `github.com/anthropic/indexer/web` and passes `web.DistFS` to `server.New()`.

This works because `go/web/embed.go` is in the `go/web/` directory, and `dist/` is a subdirectory of `go/web/`.

**Step 2 (revised): Create web/embed.go**

Create `go/web/embed.go`:

```go
package web

import "embed"

// DistFS contains the built React SPA files from web/dist/.
// In development, run: cd web && npm run build
//
//go:embed all:dist
var DistFS embed.FS
```

**Step 3: Update Server to accept fs.FS**

In `internal/server/server.go`:

```go
import (
	"io/fs"
	"log"
	"net/http"
	"sync"

	"github.com/anthropic/indexer/internal/config"
	"github.com/anthropic/indexer/internal/storage"
)

type Server struct {
	cfg            config.Config
	cfgMu          sync.RWMutex
	memoriesClient *storage.MemoriesClient
	projectsDir    string
	runs           *RunManager
	webFS          fs.FS    // embedded SPA files (may be nil in tests)
	mux            *http.ServeMux
}

func New(cfg config.Config, memoriesClient *storage.MemoriesClient, projectsDir string, webFS fs.FS) *Server {
	s := &Server{
		cfg:            cfg,
		memoriesClient: memoriesClient,
		projectsDir:    projectsDir,
		runs:           NewRunManager(),
		webFS:          webFS,
		mux:            http.NewServeMux(),
	}
	s.routes()
	return s
}
```

**Step 4: Add SPA fallback in routes.go**

```go
func (s *Server) routes() {
	// API routes.
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/projects", s.handleListProjects)
	s.mux.HandleFunc("POST /api/projects/index", s.handleStartIndex)
	s.mux.HandleFunc("GET /api/projects/{name}/progress", s.handleProgress)
	s.mux.HandleFunc("POST /api/query", s.handleQuery)
	s.mux.HandleFunc("GET /api/config", s.handleGetConfig)
	s.mux.HandleFunc("PATCH /api/config", s.handlePatchConfig)

	// SPA static files + fallback.
	if s.webFS != nil {
		s.mux.HandleFunc("GET /", s.handleSPA)
	}
}

func (s *Server) handleSPA(w http.ResponseWriter, r *http.Request) {
	// Try to serve the exact file.
	path := r.URL.Path
	if path == "/" {
		path = "/index.html"
	}

	// Strip leading slash for fs.Open.
	fsPath := path[1:]

	f, err := s.webFS.Open(fsPath)
	if err == nil {
		f.Close()
		http.FileServerFS(s.webFS).ServeHTTP(w, r)
		return
	}

	// SPA fallback: serve index.html for client-side routing.
	data, err := fs.ReadFile(s.webFS, "index.html")
	if err != nil {
		http.Error(w, "index.html not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}
```

Add `"io/fs"` to routes.go imports.

**Step 5: Update cmd/carto/main.go**

```go
import (
	cartoWeb "github.com/anthropic/indexer/web"
	"io/fs"
)
```

In `runServe`:

```go
func runServe(cmd *cobra.Command, args []string) error {
	cfg := config.Load()
	port, _ := cmd.Flags().GetString("port")
	projectsDir, _ := cmd.Flags().GetString("projects-dir")

	memoriesClient := storage.NewMemoriesClient(cfg.MemoriesURL, cfg.MemoriesKey)

	// Extract the dist subdirectory from the embedded FS.
	distFS, err := fs.Sub(cartoWeb.DistFS, "dist")
	if err != nil {
		return fmt.Errorf("embedded web assets: %w", err)
	}

	srv := server.New(cfg, memoriesClient, projectsDir, distFS)
	fmt.Printf("%s%sCarto server%s starting on http://localhost:%s\n", bold, cyan, reset, port)
	return srv.Start(":" + port)
}
```

**Step 6: Update all test callers of New()**

In `internal/server/server_test.go`, update every `New()` call to add `nil` as the 4th arg:

```go
srv := New(config.Config{}, memoriesClient, "", nil)
```

Do this for ALL test functions.

**Step 7: Run tests and build**

```bash
cd /Users/dk/projects/indexer/go/web && npm run build
cd /Users/dk/projects/indexer/go && go test ./internal/server/... -v -count=1
cd /Users/dk/projects/indexer/go && go test ./cmd/carto/... -v -count=1
cd /Users/dk/projects/indexer/go && go build ./cmd/carto
```

Expected: ALL PASS, binary builds successfully with embedded web assets.

**Step 8: Commit**

```bash
cd /Users/dk/projects/indexer
git add go/web/embed.go go/internal/server/ go/cmd/carto/
git commit -m "feat(server): embed React SPA via go:embed with SPA fallback routing"
```

---

## Phase 4: Frontend Pages

Build each page one at a time. Each page is a self-contained React component that calls the Go API.

---

### Task 4.1: Dashboard page

**Files:**
- Modify: `go/web/src/pages/Dashboard.tsx`
- Create: `go/web/src/components/ProjectCard.tsx`

**Step 1: Create ProjectCard component**

Create `go/web/src/components/ProjectCard.tsx`:

```tsx
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'

interface ProjectCardProps {
  name: string
  path: string
  indexedAt: string
  fileCount: number
}

export function ProjectCard({ name, path, indexedAt, fileCount }: ProjectCardProps) {
  const timeAgo = getTimeAgo(indexedAt)

  return (
    <Card className="bg-zinc-900 border-zinc-800 hover:border-zinc-700 transition-colors">
      <CardHeader className="pb-2">
        <div className="flex items-center justify-between">
          <CardTitle className="text-base font-semibold">{name}</CardTitle>
          <Badge variant="secondary" className="text-xs">
            {fileCount} files
          </Badge>
        </div>
      </CardHeader>
      <CardContent>
        <p className="text-xs text-zinc-500 truncate mb-1" title={path}>{path}</p>
        <p className="text-xs text-zinc-400">Indexed {timeAgo}</p>
      </CardContent>
    </Card>
  )
}

function getTimeAgo(dateStr: string): string {
  const date = new Date(dateStr)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffMins = Math.floor(diffMs / 60000)
  if (diffMins < 1) return 'just now'
  if (diffMins < 60) return `${diffMins}m ago`
  const diffHours = Math.floor(diffMins / 60)
  if (diffHours < 24) return `${diffHours}h ago`
  const diffDays = Math.floor(diffHours / 24)
  return `${diffDays}d ago`
}
```

**Step 2: Build Dashboard page**

Replace `go/web/src/pages/Dashboard.tsx`:

```tsx
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { ProjectCard } from '@/components/ProjectCard'

interface Project {
  name: string
  path: string
  indexed_at: string
  file_count: number
}

interface HealthStatus {
  status: string
  memories_healthy: boolean
}

export default function Dashboard() {
  const [projects, setProjects] = useState<Project[]>([])
  const [health, setHealth] = useState<HealthStatus | null>(null)
  const [loading, setLoading] = useState(true)
  const navigate = useNavigate()

  useEffect(() => {
    Promise.all([
      fetch('/api/projects').then(r => r.json()),
      fetch('/api/health').then(r => r.json()),
    ]).then(([projData, healthData]) => {
      setProjects(projData.projects || [])
      setHealth(healthData)
    }).catch(console.error)
      .finally(() => setLoading(false))
  }, [])

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-2xl font-bold">Dashboard</h2>
          <p className="text-sm text-zinc-400 mt-1">
            {projects.length} indexed project{projects.length !== 1 ? 's' : ''}
          </p>
        </div>
        <div className="flex items-center gap-3">
          {health && (
            <div className="flex items-center gap-2">
              <span className="text-xs text-zinc-500">Memories</span>
              <Badge variant={health.memories_healthy ? 'default' : 'destructive'} className="text-xs">
                {health.memories_healthy ? 'Connected' : 'Offline'}
              </Badge>
            </div>
          )}
          <Button onClick={() => navigate('/index')}>
            Index Project
          </Button>
        </div>
      </div>

      {loading ? (
        <p className="text-zinc-400">Loading...</p>
      ) : projects.length === 0 ? (
        <div className="text-center py-12">
          <p className="text-zinc-400 mb-4">No indexed projects yet.</p>
          <Button onClick={() => navigate('/index')}>
            Index Your First Project
          </Button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {projects.map((p) => (
            <ProjectCard
              key={p.name}
              name={p.name}
              path={p.path}
              indexedAt={p.indexed_at}
              fileCount={p.file_count}
            />
          ))}
        </div>
      )}
    </div>
  )
}
```

**Step 3: Build and verify**

```bash
cd /Users/dk/projects/indexer/go/web && npm run build
```

**Step 4: Commit**

```bash
cd /Users/dk/projects/indexer
git add go/web/src/
git commit -m "feat(web): build Dashboard page with project cards and health status"
```

---

### Task 4.2: Index Run page with SSE progress

**Files:**
- Modify: `go/web/src/pages/IndexRun.tsx`
- Create: `go/web/src/components/ProgressBar.tsx`

**Step 1: Create ProgressBar component**

Create `go/web/src/components/ProgressBar.tsx`:

```tsx
import { Progress } from '@/components/ui/progress'

interface ProgressBarProps {
  phase: string
  done: number
  total: number
}

export function ProgressBar({ phase, done, total }: ProgressBarProps) {
  const percent = total > 0 ? Math.round((done / total) * 100) : 0

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between text-sm">
        <span className="text-zinc-300 capitalize">{phase}</span>
        <span className="text-zinc-400">{done}/{total}</span>
      </div>
      <Progress value={percent} className="h-2" />
    </div>
  )
}
```

**Step 2: Build IndexRun page**

Replace `go/web/src/pages/IndexRun.tsx`:

```tsx
import { useState, useRef } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { ProgressBar } from '@/components/ProgressBar'

interface ProgressEvent {
  phase: string
  done: number
  total: number
}

interface IndexResultData {
  modules: number
  files: number
  atoms: number
  errors: number
  elapsed: string
  error_messages?: string[]
}

type Status = 'idle' | 'starting' | 'running' | 'complete' | 'error'

export default function IndexRun() {
  const [path, setPath] = useState('')
  const [incremental, setIncremental] = useState(true)
  const [moduleFilter, setModuleFilter] = useState('')
  const [status, setStatus] = useState<Status>('idle')
  const [progress, setProgress] = useState<ProgressEvent | null>(null)
  const [result, setResult] = useState<IndexResultData | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [projectName, setProjectName] = useState('')
  const eventSourceRef = useRef<EventSource | null>(null)

  const startIndex = async () => {
    setStatus('starting')
    setProgress(null)
    setResult(null)
    setError(null)

    try {
      const resp = await fetch('/api/projects/index', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          path,
          incremental,
          module: moduleFilter || undefined,
        }),
      })

      const data = await resp.json()

      if (!resp.ok) {
        setStatus('error')
        setError(data.error || 'Failed to start indexing')
        return
      }

      const name = data.project
      setProjectName(name)
      setStatus('running')

      // Connect to SSE stream.
      const es = new EventSource(`/api/projects/${encodeURIComponent(name)}/progress`)
      eventSourceRef.current = es

      es.addEventListener('progress', (e) => {
        const evt: ProgressEvent = JSON.parse(e.data)
        setProgress(evt)
      })

      es.addEventListener('complete', (e) => {
        const res: IndexResultData = JSON.parse(e.data)
        setResult(res)
        setStatus('complete')
        es.close()
      })

      es.addEventListener('error', (e) => {
        // SSE error event — could be server-sent or connection lost.
        if (e instanceof MessageEvent && e.data) {
          const errData = JSON.parse(e.data)
          setError(errData.message || 'Unknown error')
        } else {
          setError('Connection to server lost')
        }
        setStatus('error')
        es.close()
      })
    } catch (err) {
      setStatus('error')
      setError(err instanceof Error ? err.message : 'Network error')
    }
  }

  const reset = () => {
    eventSourceRef.current?.close()
    setStatus('idle')
    setProgress(null)
    setResult(null)
    setError(null)
    setProjectName('')
  }

  return (
    <div className="max-w-2xl">
      <h2 className="text-2xl font-bold mb-6">Index Project</h2>

      {status === 'idle' || status === 'starting' ? (
        <Card className="bg-zinc-900 border-zinc-800">
          <CardContent className="pt-6 space-y-4">
            <div className="space-y-2">
              <Label htmlFor="path">Project Path</Label>
              <Input
                id="path"
                placeholder="/Users/you/projects/myapp"
                value={path}
                onChange={(e) => setPath(e.target.value)}
                className="bg-zinc-800 border-zinc-700"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="module">Module Filter (optional)</Label>
              <Input
                id="module"
                placeholder="e.g. backend"
                value={moduleFilter}
                onChange={(e) => setModuleFilter(e.target.value)}
                className="bg-zinc-800 border-zinc-700"
              />
            </div>
            <div className="flex items-center gap-2">
              <input
                type="checkbox"
                id="incremental"
                checked={incremental}
                onChange={(e) => setIncremental(e.target.checked)}
                className="rounded"
              />
              <Label htmlFor="incremental" className="text-sm">
                Incremental (only re-index changed files)
              </Label>
            </div>
            <Button
              onClick={startIndex}
              disabled={!path || status === 'starting'}
              className="w-full"
            >
              {status === 'starting' ? 'Starting...' : 'Start Indexing'}
            </Button>
          </CardContent>
        </Card>
      ) : status === 'running' ? (
        <Card className="bg-zinc-900 border-zinc-800">
          <CardHeader>
            <CardTitle className="text-base">
              Indexing <span className="text-zinc-400">{projectName}</span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            {progress ? (
              <ProgressBar
                phase={progress.phase}
                done={progress.done}
                total={progress.total}
              />
            ) : (
              <p className="text-sm text-zinc-400">Waiting for progress...</p>
            )}
          </CardContent>
        </Card>
      ) : status === 'complete' && result ? (
        <Card className="bg-zinc-900 border-zinc-800">
          <CardHeader>
            <div className="flex items-center gap-2">
              <CardTitle className="text-base">Indexing Complete</CardTitle>
              <Badge variant="default">✓</Badge>
            </div>
          </CardHeader>
          <CardContent className="space-y-2">
            <div className="grid grid-cols-2 gap-2 text-sm">
              <span className="text-zinc-400">Modules</span>
              <span>{result.modules}</span>
              <span className="text-zinc-400">Files</span>
              <span>{result.files}</span>
              <span className="text-zinc-400">Atoms</span>
              <span>{result.atoms}</span>
              <span className="text-zinc-400">Errors</span>
              <span>{result.errors}</span>
              <span className="text-zinc-400">Elapsed</span>
              <span>{result.elapsed}</span>
            </div>
            {result.error_messages && result.error_messages.length > 0 && (
              <div className="mt-3 p-2 bg-zinc-800 rounded text-xs text-zinc-400">
                {result.error_messages.slice(0, 5).map((msg, i) => (
                  <p key={i}>• {msg}</p>
                ))}
              </div>
            )}
            <Button onClick={reset} variant="outline" className="mt-4">
              Index Another
            </Button>
          </CardContent>
        </Card>
      ) : status === 'error' ? (
        <Card className="bg-zinc-900 border-red-900/50">
          <CardHeader>
            <CardTitle className="text-base text-red-400">Error</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-sm text-zinc-300">{error}</p>
            <Button onClick={reset} variant="outline" className="mt-4">
              Try Again
            </Button>
          </CardContent>
        </Card>
      ) : null}
    </div>
  )
}
```

**Step 3: Build and verify**

```bash
cd /Users/dk/projects/indexer/go/web && npm run build
```

**Step 4: Commit**

```bash
cd /Users/dk/projects/indexer
git add go/web/src/
git commit -m "feat(web): build Index page with SSE progress streaming"
```

---

### Task 4.3: Query page

**Files:**
- Modify: `go/web/src/pages/Query.tsx`
- Create: `go/web/src/components/QueryResult.tsx`

**Step 1: Create QueryResult component**

Create `go/web/src/components/QueryResult.tsx`:

```tsx
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { useState } from 'react'

interface QueryResultProps {
  index: number
  source: string
  score: number
  text: string
}

export function QueryResult({ index, source, score, text }: QueryResultProps) {
  const [expanded, setExpanded] = useState(false)
  const preview = text.length > 200 && !expanded ? text.slice(0, 200) + '...' : text

  return (
    <Card
      className="bg-zinc-900 border-zinc-800 cursor-pointer hover:border-zinc-700 transition-colors"
      onClick={() => setExpanded(!expanded)}
    >
      <CardContent className="pt-4">
        <div className="flex items-center justify-between mb-2">
          <div className="flex items-center gap-2">
            <span className="text-xs text-zinc-500 font-mono">{index}.</span>
            <span className="text-sm text-zinc-300 font-mono truncate max-w-md" title={source}>
              {source}
            </span>
          </div>
          <Badge variant="secondary" className="text-xs">
            {score.toFixed(3)}
          </Badge>
        </div>
        <pre className="text-xs text-zinc-400 whitespace-pre-wrap font-mono leading-relaxed">
          {preview}
        </pre>
      </CardContent>
    </Card>
  )
}
```

**Step 2: Build Query page**

Replace `go/web/src/pages/Query.tsx`:

```tsx
import { useState, useEffect } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { QueryResult } from '@/components/QueryResult'

interface SearchResult {
  id: number
  text: string
  score: number
  source: string
}

interface Project {
  name: string
}

const tiers = [
  { value: 'mini', label: 'Mini', desc: 'Zones + blueprint (~5KB)' },
  { value: 'standard', label: 'Standard', desc: '+ atoms + wiring (~50KB)' },
  { value: 'full', label: 'Full', desc: '+ history + signals (~500KB)' },
]

export default function Query() {
  const [query, setQuery] = useState('')
  const [project, setProject] = useState('')
  const [tier, setTier] = useState('standard')
  const [k, setK] = useState(10)
  const [results, setResults] = useState<SearchResult[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [loading, setLoading] = useState(false)
  const [searched, setSearched] = useState(false)

  useEffect(() => {
    fetch('/api/projects')
      .then(r => r.json())
      .then(data => setProjects(data.projects || []))
      .catch(console.error)
  }, [])

  const search = async () => {
    if (!query.trim()) return
    setLoading(true)
    setSearched(true)

    try {
      const resp = await fetch('/api/query', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text: query, project, tier, k }),
      })
      const data = await resp.json()
      setResults(data.results || [])
    } catch (err) {
      console.error('Query failed:', err)
      setResults([])
    } finally {
      setLoading(false)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') search()
  }

  return (
    <div className="max-w-3xl">
      <h2 className="text-2xl font-bold mb-6">Query</h2>

      <div className="space-y-4 mb-6">
        <div className="space-y-2">
          <Label htmlFor="query">Question</Label>
          <Input
            id="query"
            placeholder="How does authentication work?"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={handleKeyDown}
            className="bg-zinc-800 border-zinc-700"
          />
        </div>

        <div className="flex gap-4">
          <div className="flex-1 space-y-2">
            <Label htmlFor="project">Project</Label>
            <select
              id="project"
              value={project}
              onChange={(e) => setProject(e.target.value)}
              className="w-full h-9 rounded-md border border-zinc-700 bg-zinc-800 px-3 text-sm text-zinc-100"
            >
              <option value="">All projects</option>
              {projects.map((p) => (
                <option key={p.name} value={p.name}>{p.name}</option>
              ))}
            </select>
          </div>

          <div className="space-y-2">
            <Label>Tier</Label>
            <div className="flex gap-1">
              {tiers.map((t) => (
                <button
                  key={t.value}
                  onClick={() => setTier(t.value)}
                  title={t.desc}
                  className={`px-3 py-1.5 text-xs rounded-md border transition-colors ${
                    tier === t.value
                      ? 'bg-zinc-700 border-zinc-600 text-zinc-100'
                      : 'bg-zinc-800 border-zinc-700 text-zinc-400 hover:text-zinc-300'
                  }`}
                >
                  {t.label}
                </button>
              ))}
            </div>
          </div>

          <div className="w-20 space-y-2">
            <Label htmlFor="k">Count</Label>
            <Input
              id="k"
              type="number"
              min={1}
              max={50}
              value={k}
              onChange={(e) => setK(parseInt(e.target.value) || 10)}
              className="bg-zinc-800 border-zinc-700"
            />
          </div>
        </div>

        <Button onClick={search} disabled={!query.trim() || loading}>
          {loading ? 'Searching...' : 'Search'}
        </Button>
      </div>

      {searched && !loading && results.length === 0 && (
        <p className="text-zinc-400 text-sm">No results found. Try indexing a project first.</p>
      )}

      <div className="space-y-3">
        {results.map((r, i) => (
          <QueryResult
            key={r.id || i}
            index={i + 1}
            source={r.source}
            score={r.score}
            text={r.text}
          />
        ))}
      </div>
    </div>
  )
}
```

**Step 3: Build and verify**

```bash
cd /Users/dk/projects/indexer/go/web && npm run build
```

**Step 4: Commit**

```bash
cd /Users/dk/projects/indexer
git add go/web/src/
git commit -m "feat(web): build Query page with tier picker and expandable results"
```

---

### Task 4.4: Settings page

**Files:**
- Modify: `go/web/src/pages/Settings.tsx`

**Step 1: Build Settings page**

Replace `go/web/src/pages/Settings.tsx`:

```tsx
import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'

interface ConfigData {
  llm_provider: string
  llm_api_key: string
  llm_base_url: string
  haiku_model: string
  opus_model: string
  memories_url: string
  memories_key: string
}

const providers = ['anthropic', 'openai', 'ollama']

export default function Settings() {
  const [config, setConfig] = useState<ConfigData | null>(null)
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)
  const [healthStatus, setHealthStatus] = useState<{ memories: boolean | null }>({ memories: null })

  useEffect(() => {
    fetch('/api/config')
      .then(r => r.json())
      .then(setConfig)
      .catch(console.error)
  }, [])

  const save = async () => {
    if (!config) return
    setSaving(true)
    setMessage(null)

    try {
      const resp = await fetch('/api/config', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config),
      })

      if (resp.ok) {
        setMessage({ type: 'success', text: 'Settings saved.' })
      } else {
        const data = await resp.json()
        setMessage({ type: 'error', text: data.error || 'Save failed.' })
      }
    } catch (err) {
      setMessage({ type: 'error', text: 'Network error.' })
    } finally {
      setSaving(false)
    }
  }

  const testMemories = async () => {
    try {
      const resp = await fetch('/api/health')
      const data = await resp.json()
      setHealthStatus({ memories: data.memories_healthy })
    } catch {
      setHealthStatus({ memories: false })
    }
  }

  const updateField = (field: keyof ConfigData, value: string) => {
    setConfig(prev => prev ? { ...prev, [field]: value } : null)
  }

  if (!config) return <p className="text-zinc-400">Loading...</p>

  return (
    <div className="max-w-2xl">
      <h2 className="text-2xl font-bold mb-6">Settings</h2>

      <div className="space-y-6">
        <Card className="bg-zinc-900 border-zinc-800">
          <CardHeader>
            <CardTitle className="text-base">LLM Provider</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="provider">Provider</Label>
              <select
                id="provider"
                value={config.llm_provider}
                onChange={(e) => updateField('llm_provider', e.target.value)}
                className="w-full h-9 rounded-md border border-zinc-700 bg-zinc-800 px-3 text-sm text-zinc-100"
              >
                {providers.map((p) => (
                  <option key={p} value={p}>{p}</option>
                ))}
              </select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="api-key">API Key</Label>
              <Input
                id="api-key"
                type="password"
                value={config.llm_api_key}
                onChange={(e) => updateField('llm_api_key', e.target.value)}
                placeholder="sk-..."
                className="bg-zinc-800 border-zinc-700"
              />
            </div>
            {config.llm_provider !== 'anthropic' && (
              <div className="space-y-2">
                <Label htmlFor="base-url">Base URL</Label>
                <Input
                  id="base-url"
                  value={config.llm_base_url}
                  onChange={(e) => updateField('llm_base_url', e.target.value)}
                  placeholder="https://api.openai.com"
                  className="bg-zinc-800 border-zinc-700"
                />
              </div>
            )}
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="haiku">Haiku Model</Label>
                <Input
                  id="haiku"
                  value={config.haiku_model}
                  onChange={(e) => updateField('haiku_model', e.target.value)}
                  className="bg-zinc-800 border-zinc-700"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="opus">Opus Model</Label>
                <Input
                  id="opus"
                  value={config.opus_model}
                  onChange={(e) => updateField('opus_model', e.target.value)}
                  className="bg-zinc-800 border-zinc-700"
                />
              </div>
            </div>
          </CardContent>
        </Card>

        <Card className="bg-zinc-900 border-zinc-800">
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle className="text-base">Memories Server</CardTitle>
              {healthStatus.memories !== null && (
                <Badge variant={healthStatus.memories ? 'default' : 'destructive'} className="text-xs">
                  {healthStatus.memories ? 'Connected' : 'Unreachable'}
                </Badge>
              )}
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="mem-url">URL</Label>
              <Input
                id="mem-url"
                value={config.memories_url}
                onChange={(e) => updateField('memories_url', e.target.value)}
                className="bg-zinc-800 border-zinc-700"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="mem-key">API Key</Label>
              <Input
                id="mem-key"
                type="password"
                value={config.memories_key}
                onChange={(e) => updateField('memories_key', e.target.value)}
                className="bg-zinc-800 border-zinc-700"
              />
            </div>
            <Button variant="outline" onClick={testMemories} size="sm">
              Test Connection
            </Button>
          </CardContent>
        </Card>

        <div className="flex items-center gap-3">
          <Button onClick={save} disabled={saving}>
            {saving ? 'Saving...' : 'Save Settings'}
          </Button>
          {message && (
            <span className={`text-sm ${message.type === 'success' ? 'text-green-400' : 'text-red-400'}`}>
              {message.text}
            </span>
          )}
        </div>
      </div>
    </div>
  )
}
```

**Step 2: Build and verify**

```bash
cd /Users/dk/projects/indexer/go/web && npm run build
```

**Step 3: Commit**

```bash
cd /Users/dk/projects/indexer
git add go/web/src/
git commit -m "feat(web): build Settings page with provider config and connection test"
```

---

## Phase 5: Final Integration + Polish

---

### Task 5.1: Build production binary and smoke test

**Files:**
- No new files — integration verification

**Step 1: Build the full binary**

```bash
cd /Users/dk/projects/indexer/go/web && npm run build
cd /Users/dk/projects/indexer/go && go build -o carto ./cmd/carto
```

**Step 2: Verify the binary includes web assets**

```bash
./carto serve --port 8951 &
sleep 1
curl -s http://localhost:8951/api/health | head -1
curl -s http://localhost:8951/ | head -5
kill %1
```

Expected: `/api/health` returns JSON, `/` returns HTML with React root div.

**Step 3: Run all tests**

```bash
cd /Users/dk/projects/indexer/go && go test ./... -count=1
```

Expected: ALL PASS across all 14+ packages.

**Step 4: Commit any final fixes if needed**

```bash
git add -A
git commit -m "chore: final integration polish for Carto UI"
```

---

## Summary

| Phase | Tasks | Commits | Focus |
|-------|-------|---------|-------|
| 1 | 1.1 – 1.4 | 4 | Go API endpoints (projects, query, config, index+SSE) |
| 2 | 2.1 – 2.3 | 3 | React scaffold (Vite + Tailwind + shadcn + routing) |
| 3 | 3.1 | 1 | go:embed integration + SPA fallback |
| 4 | 4.1 – 4.4 | 4 | Frontend pages (Dashboard, Index, Query, Settings) |
| 5 | 5.1 | 1 | Final build + smoke test |
| **Total** | **13** | **13** | |

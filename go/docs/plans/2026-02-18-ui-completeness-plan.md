# UI Completeness Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix critical config bugs, add error display, dashboard run status, toast notifications, and polish across all Carto UI pages.

**Architecture:** Backend fixes in Go handlers (fresh MemoriesClient per run, Docker volume). Frontend changes in React/TypeScript (expandable errors, enriched dashboard cards, sonner toasts, query pagination). All changes are additive — no architectural shifts.

**Tech Stack:** Go 1.25, React 19, shadcn/ui, sonner (new), Radix primitives, Tailwind CSS 4

**Design doc:** `docs/plans/2026-02-18-ui-completeness-design.md`

---

### Task 1: Fix Docker volume to read-write

**Files:**
- Modify: `docker-compose.yml:7`

**Step 1: Change volume mount from `:ro` to `:rw`**

In `docker-compose.yml`, change line 7:
```yaml
# Before:
      - ${PROJECTS_DIR:-~/projects}:/projects:ro
# After:
      - ${PROJECTS_DIR:-~/projects}:/projects
```

Removing `:ro` defaults to `:rw`. This lets the pipeline write `.carto/manifest.json` for incremental indexing.

**Step 2: Commit**

```bash
git add docker-compose.yml
git commit -m "fix: remove read-only mount so pipeline can write manifests"
```

---

### Task 2: Fix MemoriesClient not refreshed after Settings changes

The `s.memoriesClient` field is created once at boot and never updated. When Settings changes `memories_url` or `memories_key`, the pipeline still uses the stale client. Fix: create a fresh client in `runIndex()` from the config snapshot, same pattern as `llmClient`.

**Files:**
- Modify: `internal/server/handlers.go:322-359`
- Test: `internal/server/server_test.go`

**Step 1: Write the test**

Add to `internal/server/server_test.go`:

```go
func TestRunIndex_UsesCurrentConfig(t *testing.T) {
	// Boot server with one Memories URL
	cfg := config.Config{
		MemoriesURL: "http://original:8900",
		MemoriesKey: "original-key",
	}
	memoriesClient := storage.NewMemoriesClient("http://original:8900", "original-key")
	srv := New(cfg, memoriesClient, "", nil)

	// Patch config to a different URL (simulating Settings save)
	patchBody := strings.NewReader(`{"memories_url": "http://updated:8900", "memories_key": "new-key"}`)
	patchReq := httptest.NewRequest(http.MethodPatch, "/api/config", patchBody)
	patchReq.Header.Set("Content-Type", "application/json")
	pw := httptest.NewRecorder()
	srv.ServeHTTP(pw, patchReq)

	if pw.Code != http.StatusOK {
		t.Fatalf("PATCH expected 200, got %d", pw.Code)
	}

	// Verify the in-memory config was updated
	srv.cfgMu.RLock()
	if srv.cfg.MemoriesURL != "http://updated:8900" {
		t.Errorf("expected updated memories_url, got %q", srv.cfg.MemoriesURL)
	}
	if srv.cfg.MemoriesKey != "new-key" {
		t.Errorf("expected updated memories_key, got %q", srv.cfg.MemoriesKey)
	}
	srv.cfgMu.RUnlock()
}
```

**Step 2: Run test to verify it passes**

```bash
cd go && go test ./internal/server/ -run TestRunIndex_UsesCurrentConfig -v
```

This test verifies config patching works. The actual fix is in `runIndex()`.

**Step 3: Update `runIndex()` to create fresh MemoriesClient**

In `internal/server/handlers.go`, in the `runIndex` function (around line 322), add Memories client creation from config snapshot. Change:

```go
// BEFORE (line 344-348):
	result, err := pipeline.Run(pipeline.Config{
		ProjectName:    projectName,
		RootPath:       absPath,
		LLMClient:      llmClient,
		MemoriesClient: s.memoriesClient,
```

To:

```go
	// Create a fresh Memories client from the current config so Settings
	// changes take effect without server restart.
	memoriesURL := cfg.MemoriesURL
	if isDocker() {
		memoriesURL = strings.Replace(memoriesURL, "localhost", "host.docker.internal", 1)
		memoriesURL = strings.Replace(memoriesURL, "127.0.0.1", "host.docker.internal", 1)
	}
	memoriesClient := storage.NewMemoriesClient(memoriesURL, cfg.MemoriesKey)

	result, err := pipeline.Run(pipeline.Config{
		ProjectName:    projectName,
		RootPath:       absPath,
		LLMClient:      llmClient,
		MemoriesClient: memoriesClient,
```

Also add `"strings"` to the import block if not already present.

**Step 4: Run all server tests**

```bash
cd go && go test ./internal/server/ -v -race
```

**Step 5: Commit**

```bash
git add internal/server/handlers.go internal/server/server_test.go
git commit -m "fix: create fresh MemoriesClient per index run from current config

Previously the Memories client was created once at boot and never
refreshed. Settings changes to memories_url or memories_key were
silently ignored by the pipeline, causing 401 errors."
```

---

### Task 3: Persist last run status beyond 30-second window

Currently `RunManager.Finish()` deletes the run after 30 seconds. This means navigating away loses the result. Fix: keep a separate `lastRuns` map that persists the most recent `RunStatus` per project.

**Files:**
- Modify: `internal/server/sse.go:166-233`
- Test: `internal/server/server_test.go`

**Step 1: Write the test**

Add to `internal/server/server_test.go`:

```go
func TestRunManager_LastRunPersists(t *testing.T) {
	mgr := NewRunManager()

	run := mgr.Start("persist-test")
	if run == nil {
		t.Fatal("expected to start run")
	}

	// Send a result, then finish
	run.SendResult(IndexResult{Modules: 1, Files: 5, Atoms: 20})
	mgr.Finish("persist-test")

	// ListRuns should include the finished run
	runs := mgr.ListRuns()
	found := false
	for _, r := range runs {
		if r.Project == "persist-test" && r.Status == "complete" {
			found = true
			if r.Result == nil || r.Result.Modules != 1 {
				t.Errorf("expected result with 1 module, got %+v", r.Result)
			}
		}
	}
	if !found {
		t.Error("expected persist-test in ListRuns after finish")
	}

	// Even after the run is removed from the active map (simulated by
	// directly deleting), the last run should still be accessible.
	mgr.mu.Lock()
	delete(mgr.runs, "persist-test")
	mgr.mu.Unlock()

	runs2 := mgr.ListRuns()
	found2 := false
	for _, r := range runs2 {
		if r.Project == "persist-test" {
			found2 = true
		}
	}
	if !found2 {
		t.Error("expected persist-test in ListRuns even after active run deleted")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd go && go test ./internal/server/ -run TestRunManager_LastRunPersists -v
```

Expected: FAIL (lastRuns map doesn't exist yet)

**Step 3: Add `lastRuns` map to RunManager**

In `internal/server/sse.go`, modify the `RunManager` struct and related methods:

```go
// RunManager tracks active indexing runs by project name.
type RunManager struct {
	mu       sync.Mutex
	runs     map[string]*IndexRun
	lastRuns map[string]RunStatus // persists last run per project
}

// NewRunManager creates an empty RunManager.
func NewRunManager() *RunManager {
	return &RunManager{
		runs:     make(map[string]*IndexRun),
		lastRuns: make(map[string]RunStatus),
	}
}
```

Update `Finish()` to snapshot the run status into `lastRuns` before cleanup:

```go
func (m *RunManager) Finish(project string) {
	m.mu.Lock()
	run, exists := m.runs[project]
	if !exists {
		m.mu.Unlock()
		return
	}
	run.mu.Lock()
	run.finished = true

	// Snapshot for persistent last-run tracking.
	status := RunStatus{Project: project}
	if run.FinalError != "" {
		status.Status = "error"
		status.Error = run.FinalError
	} else if run.FinalResult != nil {
		status.Status = "complete"
		status.Result = run.FinalResult
	} else {
		status.Status = "complete"
	}
	m.lastRuns[project] = status

	run.mu.Unlock()
	close(run.done)
	close(run.events)
	m.mu.Unlock()

	// Clean up active run after a delay so late SSE clients can still connect.
	go func() {
		time.Sleep(30 * time.Second)
		m.mu.Lock()
		delete(m.runs, project)
		m.mu.Unlock()
	}()
}
```

Update `ListRuns()` to merge active runs with persisted lastRuns:

```go
func (m *RunManager) ListRuns() []RunStatus {
	m.mu.Lock()
	defer m.mu.Unlock()

	seen := make(map[string]bool)
	var runs []RunStatus

	// Active runs take priority.
	for name, run := range m.runs {
		run.mu.Lock()
		status := RunStatus{Project: name}
		if !run.finished {
			status.Status = "running"
		} else if run.FinalError != "" {
			status.Status = "error"
			status.Error = run.FinalError
		} else if run.FinalResult != nil {
			status.Status = "complete"
			status.Result = run.FinalResult
		} else {
			status.Status = "complete"
		}
		run.mu.Unlock()
		runs = append(runs, status)
		seen[name] = true
	}

	// Add persisted last runs that aren't currently active.
	for name, status := range m.lastRuns {
		if !seen[name] {
			runs = append(runs, status)
		}
	}

	return runs
}
```

**Step 4: Run test to verify it passes**

```bash
cd go && go test ./internal/server/ -run TestRunManager_LastRunPersists -v
```

**Step 5: Run all server tests**

```bash
cd go && go test ./internal/server/ -v -race
```

**Step 6: Commit**

```bash
git add internal/server/sse.go internal/server/server_test.go
git commit -m "feat: persist last run status per project beyond 30s window

Adds lastRuns map to RunManager that snapshots the final status
when a run finishes. ListRuns merges active runs with persisted
last runs so the UI can restore state after navigation."
```

---

### Task 4: Add Docker flag to health endpoint

**Files:**
- Modify: `internal/server/handlers.go:31-37`
- Test: `internal/server/server_test.go`

**Step 1: Update health handler**

In `internal/server/handlers.go`, modify `handleHealth`:

```go
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	healthy, _ := s.memoriesClient.Health()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"memories_healthy": healthy,
		"docker":           isDocker(),
	})
}
```

**Step 2: Update health test**

In `TestHealthEndpoint`, add assertion for docker field:

```go
	// Should include docker field (will be false in test env)
	if _, ok := resp["docker"]; !ok {
		t.Error("expected docker field in health response")
	}
```

**Step 3: Run tests**

```bash
cd go && go test ./internal/server/ -run TestHealthEndpoint -v
```

**Step 4: Commit**

```bash
git add internal/server/handlers.go internal/server/server_test.go
git commit -m "feat: add docker flag to health endpoint for UI hints"
```

---

### Task 5: Install sonner and add Toaster to app layout

**Files:**
- Modify: `web/package.json` (via npm install)
- Modify: `web/src/App.tsx`

**Step 1: Install sonner**

```bash
cd go/web && npm install sonner
```

**Step 2: Add Toaster component to App.tsx**

```tsx
import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { Toaster } from 'sonner'
import { ThemeProvider } from './components/ThemeProvider'
import { Layout } from './components/Layout'
import Dashboard from './pages/Dashboard'
import IndexRun from './pages/IndexRun'
import Query from './pages/Query'
import Settings from './pages/Settings'

function App() {
  return (
    <ThemeProvider>
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
      <Toaster richColors position="bottom-right" />
    </ThemeProvider>
  )
}

export default App
```

**Step 3: Verify build**

```bash
cd go/web && npm run build
```

**Step 4: Commit**

```bash
git add web/package.json web/package-lock.json web/src/App.tsx
git commit -m "feat: install sonner and add Toaster to app layout"
```

---

### Task 6: Add toasts to Settings page

Replace inline `message` state with toast calls. Keep validation errors inline (they're field-specific), but use toasts for save/connection feedback.

**Files:**
- Modify: `web/src/pages/Settings.tsx`

**Step 1: Replace inline messages with toasts**

Add import at top:
```tsx
import { toast } from 'sonner'
```

Remove `message` state variable and its inline rendering. In `save()`, replace:
```tsx
      setMessage({ type: 'success', text: 'Settings saved successfully.' })
```
with:
```tsx
      toast.success('Settings saved')
```

And replace the error:
```tsx
      setMessage({ type: 'error', text: err instanceof Error ? err.message : 'Failed to save' })
```
with:
```tsx
      toast.error(err instanceof Error ? err.message : 'Failed to save')
```

In `testConnection()`, add toasts:
- On success: `toast.success('Memories server connected')`
- On failure: `toast.error(data.error || 'Connection failed')`

Add Docker environment hint. After loading config, also fetch health:

```tsx
const [isDockerEnv, setIsDockerEnv] = useState(false)

useEffect(() => {
  Promise.all([
    fetch('/api/config').then(r => r.json()),
    fetch('/api/health').then(r => r.json()),
  ]).then(([configData, healthData]) => {
    const memoriesUrl = configData.memories_url?.replace('host.docker.internal', 'localhost') || configData.memories_url
    setConfig({ ...configData, memories_url: memoriesUrl })
    setIsDockerEnv(healthData.docker === true)
  }).catch(console.error)
    .finally(() => setLoading(false))
}, [])
```

Render Docker hint banner at top of settings (inside the return, before the first Card):

```tsx
{isDockerEnv && (
  <div className="rounded-md border border-blue-500/30 bg-blue-500/10 p-3 text-sm text-blue-400">
    Running in Docker — <code className="text-xs bg-muted px-1 rounded">localhost</code> URLs are automatically routed to your host machine.
  </div>
)}
```

Remove the `message` state and inline message rendering near the Save button. Replace with just the Button.

**Step 2: Verify build**

```bash
cd go/web && npm run build
```

**Step 3: Commit**

```bash
git add web/src/pages/Settings.tsx
git commit -m "feat: replace inline messages with toasts in Settings

Adds Docker environment hint banner and uses sonner toasts for
save confirmation and connection test results."
```

---

### Task 7: Add expandable error messages to IndexRun results

**Files:**
- Modify: `web/src/pages/IndexRun.tsx`

**Step 1: Update CompleteData interface**

Add `error_messages` field:

```tsx
interface CompleteData {
  modules: number
  files: number
  atoms: number
  errors: number
  elapsed: string
  error_messages?: string[]
}
```

**Step 2: Add expandable error section to the complete state card**

Add state for expansion toggle:
```tsx
const [errorsExpanded, setErrorsExpanded] = useState(false)
```

In the results card (inside `state === 'complete' && result`), after the Errors grid cell, add:

```tsx
{result.errors > 0 && result.error_messages && result.error_messages.length > 0 && (
  <div className="border-t border-border pt-3">
    <button
      onClick={() => setErrorsExpanded(!errorsExpanded)}
      className="flex items-center gap-2 text-sm text-red-400 hover:text-red-300 transition-colors w-full text-left"
    >
      <span className={`transition-transform ${errorsExpanded ? 'rotate-90' : ''}`}>▶</span>
      <span>{result.error_messages.length} error{result.error_messages.length !== 1 ? 's' : ''} — click to {errorsExpanded ? 'collapse' : 'expand'}</span>
    </button>
    {errorsExpanded && (
      <div className="mt-2 bg-muted/50 rounded-md p-3 max-h-48 overflow-y-auto font-mono text-xs space-y-1">
        {result.error_messages.map((msg, i) => (
          <div key={i} className="flex gap-2">
            <span className="text-red-400 shrink-0">✗</span>
            <span className="text-red-400">{msg}</span>
          </div>
        ))}
      </div>
    )}
  </div>
)}
```

**Step 3: Add toast for indexing started and SSE errors**

Add import:
```tsx
import { toast } from 'sonner'
```

In `startIndexing()`, after successful POST:
```tsx
toast.success('Indexing started')
```

In `es.onerror`, replace or supplement:
```tsx
toast.error('Connection to progress stream lost')
```

In the `pipeline_error` handler:
```tsx
toast.error(msg)
```

**Step 4: Verify build**

```bash
cd go/web && npm run build
```

**Step 5: Commit**

```bash
git add web/src/pages/IndexRun.tsx
git commit -m "feat: show expandable error messages in indexing results

Adds error_messages to CompleteData interface and renders them
as a collapsible list when errors > 0. Also adds toast
notifications for indexing started and stream errors."
```

---

### Task 8: Enrich Dashboard with last run status

**Files:**
- Modify: `web/src/pages/Dashboard.tsx`
- Modify: `web/src/components/ProjectCard.tsx`

**Step 1: Update Dashboard to fetch and merge run status**

```tsx
// Add RunStatus interface
interface RunStatus {
  project: string
  status: string // "running" | "complete" | "error"
  result?: {
    modules: number
    files: number
    atoms: number
    errors: number
  }
  error?: string
}
```

Update the `useEffect` to also fetch runs:

```tsx
useEffect(() => {
  Promise.all([
    fetch('/api/projects').then(r => r.json()),
    fetch('/api/health').then(r => r.json()),
    fetch('/api/projects/runs').then(r => r.json()).catch(() => []),
  ]).then(([projData, healthData, runsData]) => {
    setProjects(projData.projects || [])
    setHealth(healthData)
    // Build run status lookup
    const runMap: Record<string, RunStatus> = {}
    for (const run of (runsData as RunStatus[])) {
      runMap[run.project] = run
    }
    setRunStatuses(runMap)
  }).catch(console.error)
    .finally(() => setLoading(false))
}, [])
```

Add state:
```tsx
const [runStatuses, setRunStatuses] = useState<Record<string, RunStatus>>({})
```

Pass run status to ProjectCard:
```tsx
<ProjectCard
  key={p.name}
  name={p.name}
  path={p.path}
  indexedAt={p.indexed_at}
  fileCount={p.file_count}
  runStatus={runStatuses[p.name]}
  onReindex={() => navigate(`/index?path=${encodeURIComponent(p.path)}`)}
/>
```

**Step 2: Update ProjectCard to show run status**

Update interface:
```tsx
interface ProjectCardProps {
  name: string
  path: string
  indexedAt: string
  fileCount: number
  runStatus?: {
    status: string
    result?: { modules: number; files: number; atoms: number; errors: number }
    error?: string
  }
  onReindex?: () => void
}
```

Update the component to show status badge and re-index button:

```tsx
export function ProjectCard({ name, path, indexedAt, fileCount, runStatus, onReindex }: ProjectCardProps) {
  const timeAgo = getTimeAgo(indexedAt)

  const statusBadge = runStatus ? (
    runStatus.status === 'running' ? (
      <Badge variant="secondary" className="text-xs">⟳ Running</Badge>
    ) : runStatus.status === 'error' ? (
      <Badge variant="destructive" className="text-xs">✗ Error</Badge>
    ) : (
      <Badge variant="default" className="text-xs">✓ Indexed</Badge>
    )
  ) : null

  return (
    <Card className="bg-card hover:border-primary/30 transition-colors">
      <CardHeader className="pb-2">
        <div className="flex items-center justify-between">
          <CardTitle className="text-base font-semibold">{name}</CardTitle>
          <div className="flex items-center gap-2">
            {statusBadge}
            <Badge variant="secondary" className="text-xs">{fileCount} files</Badge>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <p className="text-xs text-muted-foreground truncate mb-1" title={path}>{path}</p>
        <p className="text-xs text-muted-foreground mb-2">Indexed {timeAgo}</p>
        {runStatus?.status === 'error' && runStatus.error && (
          <p className="text-xs text-red-400 mb-2 truncate" title={runStatus.error}>
            {runStatus.error}
          </p>
        )}
        {onReindex && (
          <button
            onClick={onReindex}
            className="text-xs text-primary hover:text-primary/80 transition-colors"
          >
            Re-index →
          </button>
        )}
      </CardContent>
    </Card>
  )
}
```

**Step 3: Update IndexRun to accept pre-filled path from URL params**

In `IndexRun.tsx`, add URL search params support so the Dashboard "Re-index" link works:

```tsx
import { useSearchParams } from 'react-router-dom'
```

Inside the component, add:
```tsx
const [searchParams] = useSearchParams()

useEffect(() => {
  const urlPath = searchParams.get('path')
  if (urlPath) setPath(urlPath)
}, [searchParams])
```

**Step 4: Verify build**

```bash
cd go/web && npm run build
```

**Step 5: Commit**

```bash
git add web/src/pages/Dashboard.tsx web/src/components/ProjectCard.tsx web/src/pages/IndexRun.tsx
git commit -m "feat: enrich Dashboard with run status and re-index button

Project cards now show last run status (success/error/running)
via badges. Error messages shown inline. Re-index button
navigates to Index page with pre-filled path."
```

---

### Task 9: Add client-side pagination to Query results

**Files:**
- Modify: `web/src/pages/Query.tsx`

**Step 1: Add pagination state**

```tsx
const PAGE_SIZE = 20
const [visibleCount, setVisibleCount] = useState(PAGE_SIZE)
```

Reset `visibleCount` when new search runs:
```tsx
async function search() {
  // ... existing code ...
  setVisibleCount(PAGE_SIZE)
  // ... rest of search
}
```

**Step 2: Slice results and add "Show more" button**

Replace the results rendering:

```tsx
<div className="space-y-3">
  {results.slice(0, visibleCount).map((r, i) => (
    <QueryResult key={r.id || i} index={i + 1} source={r.source} score={r.score} text={r.text} />
  ))}
  {results.length > visibleCount && (
    <div className="text-center py-4">
      <Button
        variant="secondary"
        onClick={() => setVisibleCount(prev => prev + PAGE_SIZE)}
      >
        Show more ({results.length - visibleCount} remaining)
      </Button>
    </div>
  )}
  {searched && results.length === 0 && (
    <p className="text-muted-foreground text-sm py-8 text-center">No results found.</p>
  )}
</div>
```

**Step 3: Verify build**

```bash
cd go/web && npm run build
```

**Step 4: Commit**

```bash
git add web/src/pages/Query.tsx
git commit -m "feat: add client-side pagination to Query results

Shows first 20 results with a 'Show more' button that reveals
the next batch. Resets on new search."
```

---

### Task 10: Build Docker image and verify

**Files:** None (verification only)

**Step 1: Build Docker image**

```bash
cd go && docker compose build
```

**Step 2: Start container and verify health**

```bash
docker compose up -d
curl http://localhost:8950/api/health
```

Expected: `{"docker":true,"memories_healthy":...,"status":"ok"}`

**Step 3: Verify all tests pass**

```bash
cd go && go test ./... -race
```

**Step 4: Final commit (if any fixups needed)**

Only if build/test revealed issues.

---

## Summary of all changes

| # | What | Files |
|---|------|-------|
| 1 | Docker volume `:rw` | `docker-compose.yml` |
| 2 | Fresh MemoriesClient per run | `handlers.go`, `server_test.go` |
| 3 | Persist last run status | `sse.go`, `server_test.go` |
| 4 | Docker flag in health | `handlers.go`, `server_test.go` |
| 5 | Install sonner + Toaster | `package.json`, `App.tsx` |
| 6 | Toasts in Settings + Docker hint | `Settings.tsx` |
| 7 | Expandable error messages | `IndexRun.tsx` |
| 8 | Dashboard run status + re-index | `Dashboard.tsx`, `ProjectCard.tsx`, `IndexRun.tsx` |
| 9 | Query pagination | `Query.tsx` |
| 10 | Docker build + verify | — |

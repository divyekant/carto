# Carto Management UI — Core F2 Design

**Date**: 2026-02-18
**Status**: Approved
**Author**: DK + Claude
**Scope**: Core F2 only (Dashboard, Index, Query, Settings). Integrations (F3) deferred.

## Overview

Web-based management UI for Carto, embedded into the single Go binary via `go:embed`. Lets users trigger indexing, watch progress in real-time via SSE, query indexed codebases, and configure LLM providers — all from a browser instead of the CLI.

## Architecture

```
go/internal/server/          ← Go HTTP server (skeleton exists)
  server.go                  ← go:embed, SPA fallback routing
  routes.go                  ← API route registration
  handlers.go                ← REST handlers (projects, query, config)
  sse.go                     ← SSE endpoint for live index progress

go/web/                      ← React SPA (new)
  src/
    App.tsx                  ← Router + layout shell
    pages/
      Dashboard.tsx          ← Health status, project cards, quick actions
      IndexRun.tsx           ← Trigger index, live SSE progress stream
      Query.tsx              ← Search with tier picker, formatted results
      Settings.tsx           ← LLM provider, models, Memories URL
    components/
      Layout.tsx             ← Sidebar nav + content area
      ProjectCard.tsx        ← Project summary card
      ProgressBar.tsx        ← Animated index progress
      QueryResult.tsx        ← Single search result display
  package.json
  vite.config.ts             ← Proxy /api → Go server in dev
  dist/                      ← Build output, go:embed target
```

### Tech Stack

- **Backend**: Go 1.22+ `http.ServeMux` method-based routing (already in use)
- **Frontend**: React 19 + Vite + Tailwind CSS + shadcn/ui
- **Embedding**: `//go:embed web/dist/*` for single-binary distribution
- **Streaming**: Server-Sent Events (SSE) for live index progress
- **State**: `useState`/`useEffect` + fetch — no external state library needed at this scope

## API Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/api/health` | Server + Memories health *(exists)* |
| `GET` | `/api/projects` | List indexed projects from manifest files |
| `POST` | `/api/projects/index` | Trigger index run `{path, incremental, module}` |
| `GET` | `/api/projects/{name}/progress` | SSE stream of index progress |
| `POST` | `/api/query` | Execute query `{text, project, tier, k}` |
| `GET` | `/api/config` | Current config (API keys redacted) |
| `PATCH` | `/api/config` | Update config fields at runtime |
| `GET` | `/*` | SPA fallback — serve `index.html` |

### Request/Response Schemas

**POST /api/projects/index**
```json
{
  "path": "/Users/dk/projects/myapp",
  "incremental": true,
  "module": "",
  "project": ""
}
```
Returns `202 Accepted` with `{"project": "myapp", "status": "started"}`.

**GET /api/projects/{name}/progress** (SSE)
```
event: progress
data: {"phase":"Scanning","done":0,"total":1}

event: progress
data: {"phase":"Atoms","done":15,"total":42}

event: complete
data: {"modules":3,"files":42,"atoms":156,"errors":0,"elapsed":"12.3s"}

event: error
data: {"message":"pipeline failed: memories server unreachable"}
```

**POST /api/query**
```json
{
  "text": "how does authentication work",
  "project": "myapp",
  "tier": "standard",
  "k": 10
}
```

**GET /api/config**
```json
{
  "llm_provider": "anthropic",
  "llm_api_key": "sk-ant-...****",
  "llm_base_url": "",
  "haiku_model": "claude-sonnet-4-20250514",
  "opus_model": "claude-opus-4-20250514",
  "memories_url": "http://localhost:8900",
  "memories_key": "****"
}
```

**PATCH /api/config**
```json
{
  "llm_provider": "openai",
  "llm_base_url": "https://api.openai.com"
}
```

## SSE Progress Architecture

The server tracks active index runs in a `sync.Map[string]*IndexRun`:

```go
type IndexRun struct {
    Project    string
    StartedAt  time.Time
    Progress   chan ProgressEvent  // buffered channel
    Done       chan struct{}
}
```

Flow:
1. `POST /api/projects/index` creates an `IndexRun`, starts `pipeline.Run` in a goroutine with a `ProgressFn` that sends to the channel
2. `GET /api/projects/{name}/progress` opens SSE connection, reads from the channel, writes `text/event-stream` events
3. On pipeline completion, sends `complete` event with summary, closes channel
4. Only one run per project at a time — second request returns `409 Conflict`

## Pages

### Dashboard
- Grid of project cards showing: name, last indexed timestamp, file count, module count
- Health indicators: green/red dots for Memories server and LLM provider
- "Index New Project" button → navigates to Index page
- Projects discovered by scanning for `.carto-manifest.json` files in common locations (or a configurable project registry)

### Index Run
- Path input (text field with folder icon)
- Toggles: incremental mode, full re-index
- Module filter dropdown (populated after path is entered via a lightweight scan)
- "Start Indexing" button → POST to API → switch to progress view
- Progress view: animated progress bar with current phase label, done/total counts
- Completion: summary card with modules, files, atoms, errors, elapsed time

### Query
- Text input for the query
- Project selector dropdown (populated from `/api/projects`)
- Tier picker: three buttons (mini / standard / full) with descriptions
- K slider (1–50, default 10)
- Results: list of expandable cards with source path, relevance score, and content preview
- Empty state with helpful text about indexing first

### Settings
- Form fields:
  - LLM Provider (dropdown: anthropic / openai / ollama)
  - API Key (password input, shows masked value)
  - Base URL (text input, shown conditionally for openai/ollama)
  - Haiku Model (text input)
  - Opus Model (text input)
  - Memories URL (text input)
  - Memories Key (password input)
- "Save" button → PATCH /api/config
- "Test Connection" buttons for LLM and Memories health checks
- Success/error toast notifications on save

## go:embed Strategy

```go
//go:embed web/dist/*
var webFS embed.FS

func (s *Server) routes() {
    // API routes
    s.mux.HandleFunc("GET /api/health", s.handleHealth)
    s.mux.HandleFunc("GET /api/projects", s.handleListProjects)
    // ... other API routes ...

    // SPA: serve static files, fallback to index.html
    distFS, _ := fs.Sub(webFS, "web/dist")
    fileServer := http.FileServer(http.FS(distFS))
    s.mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
        // Try static file first; if not found, serve index.html for SPA routing
        path := r.URL.Path
        if path != "/" {
            if _, err := fs.Stat(distFS, path[1:]); err == nil {
                fileServer.ServeHTTP(w, r)
                return
            }
        }
        // SPA fallback
        index, _ := fs.ReadFile(distFS, "index.html")
        w.Header().Set("Content-Type", "text/html")
        w.Write(index)
    })
}
```

### Build Process

```bash
cd go/web && npm run build    # → go/web/dist/
cd go && go build ./cmd/carto # embeds web/dist/* into binary
```

### Dev Mode

`vite.config.ts` proxies `/api/*` to `localhost:8950`:
```ts
export default defineConfig({
  server: {
    proxy: { '/api': 'http://localhost:8950' }
  }
})
```

Run both: `carto serve --port 8950` + `cd web && npm run dev` (Vite on :5173).

## Error Handling

- API errors return JSON `{"error": "message"}` with appropriate HTTP status codes
- SSE errors sent as `event: error` before closing the stream
- Frontend shows toast notifications for transient errors, inline errors for form validation
- Config PATCH validates fields server-side before applying

## What's Deferred (F3 — Follow-up)

- Integration management (GitHub, Jira, Linear signal sources)
- Token encryption (AES-256-GCM) for stored credentials
- Integrations settings page
- `.carto/integrations.json` config persistence

## Files Changed/Created

### New
- `go/web/` — Entire React SPA directory
- `go/internal/server/handlers.go` — REST API handlers
- `go/internal/server/sse.go` — SSE streaming implementation

### Modified
- `go/internal/server/server.go` — Add `go:embed`, SPA fallback
- `go/internal/server/routes.go` — Register all API routes
- `go/.gitignore` — Add `web/node_modules/`, `web/dist/`

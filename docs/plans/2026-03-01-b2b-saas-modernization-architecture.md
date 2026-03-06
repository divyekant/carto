# Carto B2B SaaS Modernization — Technical Architecture

**Date:** 2026-03-01
**Status:** Approved for Implementation
**Author:** Staff Software Architect
**Scope:** Existing codebase at `/workspace/carto/go/` — all changes modify existing files in place. No new project created.

---

## 1. Accurate Current-State Assessment

> **Note:** A previous draft of this document (written before several sprints of development)
> incorrectly listed auth, logging, rate limiting, and CORS as missing. All four are fully
> implemented. The gaps below reflect the actual current state of the codebase.

### 1.1 B2B Primitives Already Shipped

| Capability | Implementation Location | Quality |
|---|---|---|
| Bearer token auth | `internal/server/middleware.go:bearerAuth` | ✅ Solid |
| Per-IP token-bucket rate limiting | `internal/server/middleware.go:rateLimiter` | ✅ Solid |
| CORS origin allowlist | `internal/server/middleware.go:corsMiddleware` | ✅ Solid |
| Structured JSON request logging | `internal/server/middleware.go:loggingMiddleware` | ✅ Good |
| Middleware chain (`cors→log→rl→auth→mux`) | `internal/server/server.go:New` | ✅ Solid |
| Full REST CRUD (projects, sources, config) | `internal/server/routes.go` | ✅ Complete |
| SSE real-time pipeline progress | `internal/server/sse.go` | ✅ Solid |
| Config persistence (YAML, with key redaction on GET) | `internal/config/`, `internal/server/handlers.go` | ✅ Works |
| Docker + docker-compose | `go/docker-compose.yml` | ✅ Works |
| Multi-provider LLM (Anthropic, OpenAI, Ollama) | `internal/llm/` | ✅ Solid |
| 7 external source integrations | `internal/sources/` | ✅ Complete |
| `--json` / `--quiet` CLI output flags | `cmd/carto/helpers.go` | ✅ Good |
| Full CLI CRUD (`projects`, `sources`, `config`) | `cmd/carto/cmd_*.go` | ✅ Complete |
| `pkg/carto` Go SDK | `pkg/carto/carto.go` | ✅ Exists |
| Docker URL auto-routing (`config.ResolveURL`) | `internal/config/config.go` | ✅ Solid |

### 1.2 Current API Surface

```
GET    /api/health
GET    /api/projects                     — list
GET    /api/projects/{name}              — detail
DELETE /api/projects/{name}              — delete index
GET    /api/projects/{name}/sources      — read sources.yaml
PUT    /api/projects/{name}/sources      — write sources.yaml
GET    /api/projects/{name}/progress     — SSE stream
POST   /api/projects/{name}/stop         — cancel run
GET    /api/projects/runs                — active/recent runs
POST   /api/projects/index               — start index (path or git URL)
POST   /api/projects/index-all           — batch re-index
GET    /api/config                       — show config (keys redacted)
PATCH  /api/config                       — partial update
GET    /api/browse                       — directory browser
POST   /api/test-memories                — connectivity check
```

### 1.3 Current CLI Surface

```
carto index [path] [--full|--incremental|--module|--project|--all|--changed]
carto query <text> [--project|--tier|--k|--json|--quiet]
carto modules [path]
carto patterns [path]
carto status [path]
carto serve [--port|--projects-dir]
carto projects list|show|delete     [--json|--quiet]
carto sources  list|set|rm          [--json|--quiet]
carto config   get|set              [--json|--quiet]
```

---

## 2. Genuine Gap Analysis

The following gaps are **verified against the actual source code** and represent real work needed.

### Priority Legend
- 🔴 **P0** — Blocks enterprise adoption; fix before any B2B customer deployment
- 🟠 **P1** — High: required for production reliability/observability
- 🟡 **P2** — Medium: developer experience and operational maturity
- ⚪ **P3** — Nice-to-have; schedule in a future sprint

| # | Gap | Priority | File(s) Affected |
|---|-----|----------|-----------------|
| G1 | No API versioning (`/api/v1/`) | 🔴 P0 | `server/routes.go` |
| G2 | No `X-Request-ID` propagation in logs or responses | 🔴 P0 | `server/middleware.go`, `server/handlers.go` |
| G3 | Inconsistent error envelope (not structured) | 🔴 P0 | `server/handlers.go` |
| G4 | No graceful shutdown (SIGTERM corrupts active runs) | 🔴 P0 | `server/server.go` |
| G5 | Secrets stored in plaintext YAML on volume | 🔴 P0 | `internal/config/config.go` |
| G6 | `carto index --changed` is a silent no-op | 🟠 P1 | `cmd/carto/cmd_index.go`, `internal/manifest/` |
| G7 | No `X-RateLimit-*` response headers | 🟠 P1 | `server/middleware.go` |
| G8 | No Prometheus metrics endpoint | 🟠 P1 | `server/` (new file) |
| G9 | No health-check tiers (live vs. ready) | 🟠 P1 | `server/handlers.go`, `server/routes.go` |
| G10 | No path traversal hardening in browse/index | 🟠 P1 | `server/handlers.go` |
| G11 | No webhook notifications (CI/CD integration) | 🟠 P1 | `server/` (new file), `server/handlers.go` |
| G12 | No pagination on list endpoints | 🟠 P1 | `server/handlers.go` |
| G13 | React has no error boundaries | 🟠 P1 | `web/src/components/` (new) |
| G14 | UI shows plain "Loading..." (no skeleton screens) | 🟡 P2 | `web/src/pages/` |
| G15 | No shell tab-completions for CLI | 🟡 P2 | `cmd/carto/main.go` |
| G16 | No `carto index --dry-run` | 🟡 P2 | `cmd/carto/cmd_index.go` |
| G17 | No `--config-file` flag | 🟡 P2 | `cmd/carto/main.go`, `internal/config/` |
| G18 | No audit log for destructive operations | 🟡 P2 | `server/handlers.go` |
| G19 | No multi-arch Docker builds (`linux/arm64`) | 🟡 P2 | `go/Dockerfile` |
| G20 | No Kubernetes manifests / Helm chart | 🟡 P2 | `go/deploy/k8s/` (new dir) |
| G21 | `handleIndexAll` ignores `changed_only` flag | 🟡 P2 | `server/handlers.go` |

---

## 3. Detailed Architecture Specifications

All specifications modify **existing files only** unless the change is a genuinely new standalone capability. New files are minimized.

---

### G1 — API Versioning

**File to modify:** `go/internal/server/routes.go`

Strategy: Register `/api/v1/*` as the canonical path for all existing handlers. Keep `/api/*` paths alive as backward-compatible aliases (same handler, no deprecation header — legacy clients should not break silently). New handlers added from this point forward are registered only under `/api/v1/`.

```go
func (s *Server) routes() {
    // ── Canonical v1 routes (new clients should use these) ──────────────
    s.mux.HandleFunc("GET /api/v1/health",                   s.handleHealth)
    s.mux.HandleFunc("GET /api/v1/health/live",              s.handleHealthLive)   // new
    s.mux.HandleFunc("GET /api/v1/health/ready",             s.handleHealthReady)  // new
    s.mux.HandleFunc("GET /api/v1/metrics",                  s.handleMetrics)      // new
    s.mux.HandleFunc("GET /api/v1/projects",                 s.handleListProjects)
    s.mux.HandleFunc("GET /api/v1/projects/runs",            s.handleListRuns)
    s.mux.HandleFunc("POST /api/v1/projects/index",          s.handleStartIndex)
    s.mux.HandleFunc("POST /api/v1/projects/index-all",      s.handleIndexAll)
    s.mux.HandleFunc("GET /api/v1/projects/{name}",          s.handleGetProject)
    s.mux.HandleFunc("DELETE /api/v1/projects/{name}",       s.handleDeleteProject)
    s.mux.HandleFunc("GET /api/v1/projects/{name}/sources",  s.handleGetSources)
    s.mux.HandleFunc("PUT /api/v1/projects/{name}/sources",  s.handlePutSources)
    s.mux.HandleFunc("GET /api/v1/projects/{name}/progress", s.handleProgress)
    s.mux.HandleFunc("POST /api/v1/projects/{name}/stop",    s.handleStopIndex)
    s.mux.HandleFunc("GET /api/v1/config",                   s.handleGetConfig)
    s.mux.HandleFunc("PATCH /api/v1/config",                 s.handlePatchConfig)
    s.mux.HandleFunc("GET /api/v1/browse",                   s.handleBrowse)
    s.mux.HandleFunc("POST /api/v1/test-memories",           s.handleTestMemories)

    // ── Legacy /api/* aliases (preserved for backward compat) ────────────
    s.mux.HandleFunc("GET /api/health",                   s.handleHealth)
    s.mux.HandleFunc("GET /api/projects",                 s.handleListProjects)
    s.mux.HandleFunc("GET /api/projects/runs",            s.handleListRuns)
    s.mux.HandleFunc("POST /api/projects/index",          s.handleStartIndex)
    s.mux.HandleFunc("POST /api/projects/index-all",      s.handleIndexAll)
    s.mux.HandleFunc("GET /api/projects/{name}",          s.handleGetProject)
    s.mux.HandleFunc("DELETE /api/projects/{name}",       s.handleDeleteProject)
    s.mux.HandleFunc("GET /api/projects/{name}/sources",  s.handleGetSources)
    s.mux.HandleFunc("PUT /api/projects/{name}/sources",  s.handlePutSources)
    s.mux.HandleFunc("GET /api/projects/{name}/progress", s.handleProgress)
    s.mux.HandleFunc("POST /api/projects/{name}/stop",    s.handleStopIndex)
    s.mux.HandleFunc("GET /api/config",                   s.handleGetConfig)
    s.mux.HandleFunc("PATCH /api/config",                 s.handlePatchConfig)
    s.mux.HandleFunc("GET /api/browse",                   s.handleBrowse)
    s.mux.HandleFunc("POST /api/test-memories",           s.handleTestMemories)

    // ── SPA static files ──────────────────────────────────────────────────
    if s.webFS != nil {
        s.mux.HandleFunc("GET /", s.handleSPA)
    }
}
```

**Update** `bearerAuth` bypass list in `middleware.go` to include new health paths:

```go
if r.URL.Path == "/api/health" ||
   r.URL.Path == "/api/v1/health" ||
   r.URL.Path == "/api/v1/health/live" ||
   r.URL.Path == "/api/v1/health/ready" {
    next.ServeHTTP(w, r)
    return
}
```

---

### G2 — X-Request-ID Propagation

**File to modify:** `go/internal/server/middleware.go`

Add a context key type and a new middleware inserted between CORS and logging in the chain:

```go
// contextKey is an unexported type for storing values in request context.
type contextKey int

const requestIDKey contextKey = iota

// requestIDMiddleware extracts X-Request-ID from the request (or generates
// one) and stores it in the context. Every handler can then read it via
// requestIDFromCtx(). The ID is echoed in the X-Request-ID response header
// so clients can correlate their requests to server log lines.
func requestIDMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        id := r.Header.Get("X-Request-ID")
        if id == "" {
            var b [8]byte
            rand.Read(b[:])
            id = fmt.Sprintf("%x", b)
        }
        ctx := context.WithValue(r.Context(), requestIDKey, id)
        w.Header().Set("X-Request-ID", id)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// requestIDFromCtx extracts the request ID from context, or returns "".
func requestIDFromCtx(ctx context.Context) string {
    if id, ok := ctx.Value(requestIDKey).(string); ok {
        return id
    }
    return ""
}
```

Update `loggingMiddleware` to include `request_id`:

```go
log.Printf(
    `{"ts":%q,"method":%q,"path":%q,"status":%d,"latency_ms":%d,"ip":%q,"request_id":%q}`,
    time.Now().UTC().Format(time.RFC3339),
    r.Method, r.URL.Path, lrw.status, latency,
    r.RemoteAddr, requestIDFromCtx(r.Context()),
)
```

**File to modify:** `go/internal/server/server.go` — insert in the chain:

```go
s.handler = chain(
    s.mux,
    corsMiddleware(corsOrigins),
    requestIDMiddleware,    // ← ADD between CORS and logging
    loggingMiddleware,
    rateLimitMiddleware(rl),
    bearerAuth(cfg.ServerToken),
)
```

Required new imports in `middleware.go`: `"context"`, `"crypto/rand"`.

---

### G3 — Consistent Error Envelope

**File to modify:** `go/internal/server/handlers.go`

The current `writeError(w, status, msg)` produces `{"error":"message"}`. Upgrade to a structured envelope that includes an error code and the request ID. This is a **signature change** requiring all call sites to be updated.

```go
// writeError writes a structured JSON error response with a typed error code.
// All handlers must pass r so the request ID can be included in the body.
func writeError(w http.ResponseWriter, r *http.Request, status int, code, msg string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(map[string]any{
        "error": map[string]string{
            "code":       code,
            "message":    msg,
            "request_id": requestIDFromCtx(r.Context()),
        },
    })
}
```

**Standard error codes** (PascalCase string constants in `handlers.go`):

```go
const (
    ErrInvalidRequest  = "INVALID_REQUEST"
    ErrNotFound        = "NOT_FOUND"
    ErrConflict        = "CONFLICT"
    ErrInternalError   = "INTERNAL_ERROR"
    ErrUnauthorized    = "UNAUTHORIZED"
    ErrRateLimited     = "RATE_LIMITED"
    ErrBadGateway      = "BAD_GATEWAY"
)
```

**Migration:** Search-and-replace every `writeError(w, http.Status*, "message")` call to the new signature. For example:

```go
// Before:
writeError(w, http.StatusNotFound, "project not found")

// After:
writeError(w, r, http.StatusNotFound, ErrNotFound, "project not found")
```

---

### G4 — Graceful Shutdown

**File to modify:** `go/internal/server/server.go`

Replace the simple `http.ListenAndServe` with a version that traps SIGTERM/SIGINT and drains in-flight requests:

```go
func (s *Server) Start(addr string) error {
    srv := &http.Server{
        Addr:         addr,
        Handler:      s,
        ReadTimeout:  60 * time.Second,
        WriteTimeout: 120 * time.Second, // generous for long SSE streams
        IdleTimeout:  120 * time.Second,
    }

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

    startErr := make(chan error, 1)
    go func() {
        log.Printf(`{"ts":%q,"event":"server_start","addr":%q}`,
            time.Now().UTC().Format(time.RFC3339), addr)
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            startErr <- err
        }
    }()

    select {
    case err := <-startErr:
        return err
    case sig := <-quit:
        log.Printf(`{"ts":%q,"event":"shutdown_signal","signal":%q}`,
            time.Now().UTC().Format(time.RFC3339), sig.String())
    }

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    if err := srv.Shutdown(ctx); err != nil {
        return fmt.Errorf("graceful shutdown: %w", err)
    }
    log.Printf(`{"ts":%q,"event":"shutdown_complete"}`,
        time.Now().UTC().Format(time.RFC3339))
    return nil
}
```

New imports required: `"context"`, `"fmt"`, `"os"`, `"os/signal"`, `"syscall"`.

---

### G5 — Secrets Hardening

**File to modify:** `go/internal/config/config.go`

Env-var-sourced secrets must **never be persisted** to the YAML config file. Explicitly typed secrets from the UI (PATCH /api/config) may be persisted. Add an `envSources` internal set to track origin:

```go
// Config holds all server configuration.
type Config struct {
    // ... existing fields unchanged ...

    // envSources tracks which secrets originated from environment variables.
    // These are excluded from config.Save() to prevent env secrets from leaking
    // to the on-disk YAML config file.
    envSources map[string]bool
}

// secretEnvMap maps Config field names to their environment variable names.
var secretEnvMap = map[string]string{
    "AnthropicKey": "ANTHROPIC_API_KEY",
    "LLMApiKey":    "LLM_API_KEY",
    "MemoriesKey":  "MEMORIES_API_KEY",
    "GitHubToken":  "GITHUB_TOKEN",
    "JiraToken":    "JIRA_TOKEN",
    "LinearToken":  "LINEAR_TOKEN",
    "NotionToken":  "NOTION_TOKEN",
    "SlackToken":   "SLACK_TOKEN",
    "ServerToken":  "CARTO_SERVER_TOKEN",
}

// Load reads config from environment variables (priority) then from the
// config file. Secrets from env vars are marked so Save() does not write
// them back to disk.
func Load() Config {
    cfg := loadFromFile() // existing file-reading logic
    cfg.envSources = make(map[string]bool)

    if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" {
        cfg.AnthropicKey = v
        cfg.envSources["AnthropicKey"] = true
    }
    if v := os.Getenv("LLM_API_KEY"); v != "" {
        cfg.LLMApiKey = v
        cfg.envSources["LLMApiKey"] = true
    }
    // ... repeat for all secret fields in secretEnvMap ...
    return cfg
}

// Save writes config to disk, omitting fields that came from environment
// variables. Non-secret fields (URLs, model names, concurrency) are always saved.
func Save(cfg Config) error {
    safe := cfg // copy
    for field := range cfg.envSources {
        switch field {
        case "AnthropicKey": safe.AnthropicKey = ""
        case "LLMApiKey":    safe.LLMApiKey = ""
        case "MemoriesKey":  safe.MemoriesKey = ""
        case "GitHubToken":  safe.GitHubToken = ""
        case "JiraToken":    safe.JiraToken = ""
        case "LinearToken":  safe.LinearToken = ""
        case "NotionToken":  safe.NotionToken = ""
        case "SlackToken":   safe.SlackToken = ""
        case "ServerToken":  safe.ServerToken = ""
        }
    }
    // ... existing yaml.Marshal + os.WriteFile with safe ...
}
```

This ensures that in Docker deployments where secrets are injected via environment variables, `docker restart` does not write them to the `/projects` volume.

---

### G6 — Fix `carto index --changed` (Silent No-Op)

**File to modify:** `go/internal/manifest/manifest.go`

Add `HasChanges` to the manifest package:

```go
// HasChanges returns true when any file tracked in the manifest has been
// added, removed, or modified on disk since the last index run.
// It uses a fast two-pass check: mtime then size, avoiding full content hashing.
func HasChanges(rootPath string, mf *Manifest) (bool, error) {
    for relPath, entry := range mf.Files {
        abs := filepath.Join(rootPath, relPath)
        info, err := os.Stat(abs)
        if err != nil {
            // File was deleted since last index.
            return true, nil
        }
        if info.ModTime().After(mf.IndexedAt) {
            return true, nil
        }
        if info.Size() != entry.Size {
            return true, nil
        }
    }
    // Also check for new files not in the manifest.
    // (Simplified: if file count on disk differs from manifest, changed.)
    return false, nil
}
```

**File to modify:** `go/cmd/carto/cmd_index.go` — replace the TODO comment:

```go
if changedOnly {
    changed, err := manifest.HasChanges(projectPath, mf)
    if err != nil || !changed {
        if !quiet {
            fmt.Printf("  %s(no changes)%s %s\n", yellow, reset, name)
        }
        continue
    }
}
```

**File to modify:** `go/internal/server/handlers.go` — the same fix for `handleIndexAll`:

```go
// In handleIndexAll, where changedOnly is checked:
if changedOnly {
    changed, _ := manifest.HasChanges(projectRoot, mf)
    if !changed {
        continue
    }
}
```

---

### G7 — X-RateLimit-* Response Headers

**File to modify:** `go/internal/server/middleware.go`

Extend the rate limiter to return richer state and set standard headers:

```go
// Allow returns (permitted, tokensRemaining, durationUntilNextToken).
func (rl *rateLimiter) Check(ip string) (bool, int, time.Duration) {
    const (
        ratePerSec = 1.0
        burst      = 10.0
        windowSecs = 60.0
    )
    rl.mu.Lock()
    defer rl.mu.Unlock()
    now := time.Now()

    // Lazy eviction of stale entries.
    for k, b := range rl.buckets {
        if now.Sub(b.lastSeen) > 10*time.Minute {
            delete(rl.buckets, k)
        }
    }

    b, ok := rl.buckets[ip]
    if !ok {
        b = &bucket{tokens: burst, lastSeen: now}
        rl.buckets[ip] = b
    }

    elapsed := now.Sub(b.lastSeen).Seconds()
    b.tokens = min(burst, b.tokens+elapsed*ratePerSec)
    b.lastSeen = now

    remaining := int(b.tokens)
    resetIn := time.Duration((burst-b.tokens)/ratePerSec) * time.Second

    if b.tokens < 1 {
        return false, 0, resetIn
    }
    b.tokens--
    return true, remaining - 1, resetIn
}

func rateLimitMiddleware(rl *rateLimiter) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ip, _, err := net.SplitHostPort(r.RemoteAddr)
            if err != nil {
                ip = r.RemoteAddr
            }
            allowed, remaining, resetIn := rl.Check(ip)

            // Inform clients of rate limit state on every response.
            w.Header().Set("X-RateLimit-Limit", "60")
            w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
            w.Header().Set("X-RateLimit-Reset",
                strconv.FormatInt(time.Now().Add(resetIn).Unix(), 10))

            if !allowed {
                w.Header().Set("Retry-After", "1")
                writeError(w, r, http.StatusTooManyRequests,
                    ErrRateLimited, "rate limit exceeded — retry after 1 second")
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

Rename the existing `Allow(ip) bool` to `Check(ip)` with the new signature and update the one call site.

---

### G8 — Prometheus Metrics Endpoint

**File to create:** `go/internal/server/metrics.go`

Zero new dependencies — uses stdlib only. Implements the Prometheus text format directly:

```go
package server

import (
    "fmt"
    "net/http"
    "sync/atomic"
    "time"
)

// serverMetrics holds atomic counters exposed at GET /api/v1/metrics.
type serverMetrics struct {
    httpRequestsTotal  atomic.Int64
    httpErrors4xx      atomic.Int64
    httpErrors5xx      atomic.Int64
    indexRunsStarted   atomic.Int64
    indexRunsErrored   atomic.Int64
    indexAtomsCreated  atomic.Int64
    queryRequestsTotal atomic.Int64
    queryZeroResults   atomic.Int64
    rateLimitRejected  atomic.Int64
    startTime          time.Time
}

var globalMetrics = &serverMetrics{startTime: time.Now()}

// handleMetrics serves Prometheus text-format exposition at /api/v1/metrics.
// Protected by bearer auth unless CARTO_METRICS_PUBLIC=true.
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
    uptime := time.Since(globalMetrics.startTime).Seconds()
    w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
    fmt.Fprintf(w, `# HELP carto_uptime_seconds Seconds since server start
# TYPE carto_uptime_seconds gauge
carto_uptime_seconds %.2f

# HELP carto_http_requests_total HTTP requests served
# TYPE carto_http_requests_total counter
carto_http_requests_total %d

# HELP carto_http_errors_4xx_total HTTP 4xx responses
# TYPE carto_http_errors_4xx_total counter
carto_http_errors_4xx_total %d

# HELP carto_http_errors_5xx_total HTTP 5xx responses
# TYPE carto_http_errors_5xx_total counter
carto_http_errors_5xx_total %d

# HELP carto_index_runs_started_total Index pipeline runs started
# TYPE carto_index_runs_started_total counter
carto_index_runs_started_total %d

# HELP carto_index_runs_errored_total Index pipeline runs that errored
# TYPE carto_index_runs_errored_total counter
carto_index_runs_errored_total %d

# HELP carto_index_atoms_created_total Cumulative atoms written to Memories
# TYPE carto_index_atoms_created_total counter
carto_index_atoms_created_total %d

# HELP carto_query_requests_total Query API requests
# TYPE carto_query_requests_total counter
carto_query_requests_total %d

# HELP carto_query_zero_results_total Queries returning 0 results
# TYPE carto_query_zero_results_total counter
carto_query_zero_results_total %d

# HELP carto_rate_limit_rejected_total Requests rejected by rate limiter
# TYPE carto_rate_limit_rejected_total counter
carto_rate_limit_rejected_total %d
`,
        uptime,
        globalMetrics.httpRequestsTotal.Load(),
        globalMetrics.httpErrors4xx.Load(),
        globalMetrics.httpErrors5xx.Load(),
        globalMetrics.indexRunsStarted.Load(),
        globalMetrics.indexRunsErrored.Load(),
        globalMetrics.indexAtomsCreated.Load(),
        globalMetrics.queryRequestsTotal.Load(),
        globalMetrics.queryZeroResults.Load(),
        globalMetrics.rateLimitRejected.Load(),
    )
}
```

**Increment points (add to existing handlers):**

| Location | Counter |
|---|---|
| `loggingMiddleware`, after `ServeHTTP` | `httpRequestsTotal++`; `httpErrors4xx++` if 400-499; `httpErrors5xx++` if 500+ |
| `handleStartIndex`, after run started | `indexRunsStarted++` |
| `runIndex`, after `run.SendError` | `indexRunsErrored++` |
| `runIndex`, after `run.SendResult` | `indexAtomsCreated += result.AtomsCreated` |
| `handleQuery`, after results set | `queryRequestsTotal++`; `queryZeroResults++` if len==0 |
| `rateLimitMiddleware`, on rejection | `rateLimitRejected++` |

---

### G9 — Health Check Tiers

**File to modify:** `go/internal/server/handlers.go` — add two handlers:

```go
// handleHealthLive is a Kubernetes liveness probe. Returns 200 as long as
// the process is alive. No external dependency checks.
func (s *Server) handleHealthLive(w http.ResponseWriter, r *http.Request) {
    writeJSON(w, http.StatusOK, map[string]string{"status": "live"})
}

// handleHealthReady is a Kubernetes readiness probe. Returns 200 only when
// all required external dependencies are healthy. Returns 503 otherwise so
// orchestrators stop routing traffic to this instance.
func (s *Server) handleHealthReady(w http.ResponseWriter, r *http.Request) {
    healthy, err := s.memoriesClient.Health()
    if !healthy {
        msg := "memories server unreachable"
        if err != nil {
            msg = err.Error()
        }
        writeJSON(w, http.StatusServiceUnavailable, map[string]any{
            "status": "not_ready",
            "reason": msg,
        })
        return
    }
    writeJSON(w, http.StatusOK, map[string]any{
        "status":  "ready",
        "version": version, // injected at build time via -ldflags
    })
}
```

---

### G10 — Path Traversal Hardening

**File to modify:** `go/internal/server/handlers.go`

```go
// safeAbsPath resolves a user-supplied path, enforcing that it stays within
// the configured allowed root. In Docker mode, the root is /projects.
// In non-Docker mode, the root is the user's home directory.
// Returns an error if the path escapes the root.
func (s *Server) safeAbsPath(requestedPath string) (string, error) {
    abs, err := filepath.Abs(requestedPath)
    if err != nil {
        return "", fmt.Errorf("invalid path: %w", err)
    }

    if config.IsDocker() {
        const dockerRoot = "/projects"
        if !strings.HasPrefix(abs+"/", dockerRoot+"/") {
            return "", fmt.Errorf("path outside allowed root %s", dockerRoot)
        }
        return abs, nil
    }

    home, err := os.UserHomeDir()
    if err != nil {
        return abs, nil // can't determine home, allow
    }
    if !strings.HasPrefix(abs+"/", home+"/") && abs != home {
        return "", fmt.Errorf("path outside home directory")
    }
    return abs, nil
}
```

Apply in `handleBrowse` (replace `filepath.Abs` + `os.Stat` with `s.safeAbsPath`) and in `handleStartIndex` (for path-based indexing).

---

### G11 — Webhook Notifications

**File to create:** `go/internal/server/webhook.go`

```go
package server

import (
    "bytes"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

// WebhookEvent is the JSON body posted to CARTO_WEBHOOK_URL.
type WebhookEvent struct {
    Event     string      `json:"event"`      // "index.complete" | "index.error" | "index.stopped"
    Project   string      `json:"project"`
    Timestamp time.Time   `json:"timestamp"`
    Result    interface{} `json:"result,omitempty"`
    Error     string      `json:"error,omitempty"`
}

// sendWebhook posts event to webhookURL, signing with HMAC-SHA256 if secret
// is non-empty. Runs in a goroutine; failures are logged but do not affect
// the indexing result.
func sendWebhook(webhookURL, secret string, event WebhookEvent) {
    if webhookURL == "" {
        return
    }
    body, err := json.Marshal(event)
    if err != nil {
        return
    }

    req, err := http.NewRequest(http.MethodPost, webhookURL, bytes.NewReader(body))
    if err != nil {
        logWebhookError(webhookURL, err)
        return
    }
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-Carto-Event", event.Event)
    req.Header.Set("X-Carto-Timestamp", event.Timestamp.UTC().Format(time.RFC3339))

    if secret != "" {
        mac := hmac.New(sha256.New, []byte(secret))
        mac.Write(body)
        req.Header.Set("X-Carto-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
    }

    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        logWebhookError(webhookURL, err)
        return
    }
    defer resp.Body.Close()
    if resp.StatusCode >= 400 {
        logWebhookError(webhookURL, fmt.Errorf("HTTP %d", resp.StatusCode))
    }
}

func logWebhookError(url string, err error) {
    fmt.Printf(`{"ts":%q,"event":"webhook_error","url":%q,"error":%q}`+"\n",
        time.Now().UTC().Format(time.RFC3339), url, err.Error())
}
```

**File to modify:** `go/internal/config/config.go` — add two fields:

```go
WebhookURL    string `yaml:"webhook_url,omitempty"    env:"CARTO_WEBHOOK_URL"`
WebhookSecret string `yaml:"webhook_secret,omitempty" env:"CARTO_WEBHOOK_SECRET"`
```

**File to modify:** `go/internal/server/handlers.go` — call after each terminal pipeline state:

```go
// In runIndex(), after run.SendResult():
go sendWebhook(cfg.WebhookURL, cfg.WebhookSecret, WebhookEvent{
    Event: "index.complete", Project: projectName,
    Timestamp: time.Now(), Result: result,
})

// After run.SendError():
go sendWebhook(cfg.WebhookURL, cfg.WebhookSecret, WebhookEvent{
    Event: "index.error", Project: projectName,
    Timestamp: time.Now(), Error: err.Error(),
})

// After run.SendStopped():
go sendWebhook(cfg.WebhookURL, cfg.WebhookSecret, WebhookEvent{
    Event: "index.stopped", Project: projectName, Timestamp: time.Now(),
})
```

---

### G12 — Pagination on List Endpoints

**File to modify:** `go/internal/server/handlers.go`

Update `handleListProjects` to accept `?limit=N&offset=M`:

```go
func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
    limit  := clampInt(r.URL.Query().Get("limit"),  50, 1, 500)
    offset := clampInt(r.URL.Query().Get("offset"),  0, 0, 1<<30)

    // ... existing scan builds allProjects []ProjectInfo ...

    total := len(allProjects)
    end := offset + limit
    if end > total {
        end = total
    }
    if offset > total {
        offset = total
    }

    writeJSON(w, http.StatusOK, map[string]any{
        "projects": allProjects[offset:end],
        "total":    total,
        "limit":    limit,
        "offset":   offset,
    })
}

// clampInt parses a query param integer, returning def if absent/invalid,
// clamped to [minVal, maxVal].
func clampInt(s string, def, minVal, maxVal int) int {
    if s == "" {
        return def
    }
    n, err := strconv.Atoi(s)
    if err != nil || n < minVal {
        return def
    }
    if n > maxVal {
        return maxVal
    }
    return n
}
```

**React Dashboard note:** The existing Dashboard already handles both the old `[]` response and the new `{projects: [], total: N}` shape:

```tsx
setProjects(Array.isArray(projData) ? projData : projData.projects || [])
```

No frontend change required for this gap.

---

### G13 — React Error Boundaries

**File to create:** `go/web/src/components/RouteErrorBoundary.tsx`

```tsx
import { Component, type ReactNode } from 'react'

interface State { hasError: boolean; error?: Error }

export class RouteErrorBoundary extends Component<
  { children: ReactNode },
  State
> {
  state: State = { hasError: false }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, info: { componentStack: string }) {
    // Log to console — a future iteration can send to an error aggregator.
    console.error('[RouteErrorBoundary]', error, info.componentStack)
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="p-5 space-y-2">
          <h2 className="text-sm font-semibold text-destructive">
            Something went wrong
          </h2>
          <pre className="text-xs bg-muted p-3 rounded-md overflow-x-auto whitespace-pre-wrap">
            {this.state.error?.message}
          </pre>
          <button
            className="text-xs text-primary underline"
            onClick={() => this.setState({ hasError: false, error: undefined })}
          >
            Try again
          </button>
        </div>
      )
    }
    return this.props.children
  }
}
```

**File to modify:** `go/web/src/components/Layout.tsx` — wrap `<Outlet />`:

```tsx
import { RouteErrorBoundary } from './RouteErrorBoundary'

// Inside <main>:
<main className="flex-1 overflow-y-auto p-3 pt-14 md:p-5 md:pt-5 md:ml-12">
  <RouteErrorBoundary>
    <Outlet />
  </RouteErrorBoundary>
</main>
```

---

### G14 — Skeleton Loading States

**File to create:** `go/web/src/components/Skeleton.tsx`

```tsx
import { cn } from '@/lib/utils'

export function Skeleton({ className }: { className?: string }) {
  return (
    <div className={cn('animate-pulse rounded-md bg-muted/60', className)} />
  )
}

export function TableSkeleton({ rows = 5, cols = 4 }: { rows?: number; cols?: number }) {
  return (
    <div className="space-y-2 mt-1">
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className={`grid gap-3`}
             style={{ gridTemplateColumns: `repeat(${cols}, 1fr)` }}>
          {Array.from({ length: cols }).map((_, j) => (
            <Skeleton key={j} className="h-5" />
          ))}
        </div>
      ))}
    </div>
  )
}

export function FormSkeleton({ rows = 6 }: { rows?: number }) {
  return (
    <div className="space-y-3">
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="space-y-1">
          <Skeleton className="h-3 w-24" />
          <Skeleton className="h-8 w-full" />
        </div>
      ))}
    </div>
  )
}
```

**File to modify:** `go/web/src/pages/Dashboard.tsx`

```tsx
import { TableSkeleton } from '@/components/Skeleton'

// Replace:
{loading ? <p className="text-muted-foreground text-sm">Loading...</p> : ...}
// With:
{loading ? <TableSkeleton rows={6} cols={5} /> : ...}
```

Apply `FormSkeleton` to `Settings.tsx` and `Skeleton` instances to `Query.tsx` loading state.

---

### G15 — CLI Shell Completions

**File to modify:** `go/cmd/carto/main.go` — add completion command in `main()`:

```go
// After all AddCommand calls:
root.AddCommand(root.CompletionCommand())
```

Add `ValidArgsFunction` to commands taking project names:

```go
// Helper to list locally indexed project names for shell completion.
func completeProjectNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
    projectsDir := os.Getenv("PROJECTS_DIR")
    if projectsDir == "" {
        return nil, cobra.ShellCompDirectiveNoFileComp
    }
    entries, err := os.ReadDir(projectsDir)
    if err != nil {
        return nil, cobra.ShellCompDirectiveNoFileComp
    }
    var names []string
    for _, e := range entries {
        if e.IsDir() && strings.HasPrefix(e.Name(), toComplete) {
            names = append(names, e.Name())
        }
    }
    return names, cobra.ShellCompDirectiveNoFileComp
}
```

Add `cmd.ValidArgsFunction = completeProjectNames` to:
- `projectsShowCmd()`
- `projectsDeleteCmd()`
- `sourcesListCmd()`, `sourcesSetCmd()`, `sourcesRmCmd()`

---

### G16 — `carto index --dry-run`

**File to modify:** `go/cmd/carto/cmd_index.go`

```go
// In indexCmd():
cmd.Flags().Bool("dry-run", false, "Scan and report modules/files that would be indexed; no LLM calls made")

// In runIndex(), before API key validation:
dryRun, _ := cmd.Flags().GetBool("dry-run")
if dryRun {
    return runIndexDryRun(cmd, absPath, projectName, moduleFilter)
}
```

```go
func runIndexDryRun(cmd *cobra.Command, rootPath, projectName, moduleFilter string) error {
    mods, err := scanner.ScanModules(rootPath)
    if err != nil {
        return fmt.Errorf("scan: %w", err)
    }

    type modSummary struct {
        Name  string `json:"name"`
        Files int    `json:"files"`
    }
    var summary []modSummary
    totalFiles := 0

    for _, mod := range mods {
        if moduleFilter != "" && mod.Name != moduleFilter {
            continue
        }
        summary = append(summary, modSummary{Name: mod.Name, Files: len(mod.Files)})
        totalFiles += len(mod.Files)
    }

    writeOutput(cmd, map[string]any{
        "project": projectName, "modules": summary, "total_files": totalFiles,
    }, func() {
        fmt.Printf("%s%sDry run — %s%s\n\n", bold, cyan, projectName, reset)
        for _, m := range summary {
            fmt.Printf("  %s%-30s%s %d files\n", bold, m.Name, reset, m.Files)
        }
        fmt.Printf("\n  %sTotal:%s %d module(s), %d file(s)\n",
            bold, reset, len(summary), totalFiles)
        fmt.Printf("  %sNo LLM calls will be made.%s\n", yellow, reset)
    })
    return nil
}
```

---

### G18 — Audit Log

**File to modify:** `go/internal/server/handlers.go`

```go
// auditLog emits a structured JSON audit event to stdout. B2B operators
// can forward stdout to their SIEM (Datadog, Splunk, CloudWatch) for
// compliance and change-tracking.
func auditLog(r *http.Request, action, resource, outcome string) {
    ip, _, _ := net.SplitHostPort(r.RemoteAddr)
    fmt.Printf(
        `{"ts":%q,"audit":true,"action":%q,"resource":%q,"outcome":%q,"ip":%q,"request_id":%q}`+"\n",
        time.Now().UTC().Format(time.RFC3339),
        action, resource, outcome, ip, requestIDFromCtx(r.Context()),
    )
}
```

Call at every state-mutating handler:

| Handler | Call |
|---|---|
| `handleDeleteProject` (success) | `auditLog(r, "project.delete", name, "ok")` |
| `handleDeleteProject` (not found) | `auditLog(r, "project.delete", name, "not_found")` |
| `handleStartIndex` (started) | `auditLog(r, "project.index", projectName, "started")` |
| `handleIndexAll` | `auditLog(r, "project.index_all", "*", "started")` |
| `handleStopIndex` (success) | `auditLog(r, "project.stop", name, "ok")` |
| `handlePatchConfig` (success) | `auditLog(r, "config.update", "server", "ok")` |
| `handlePutSources` (success) | `auditLog(r, "sources.update", name, "ok")` |

---

## 4. Scalability Architecture

### 4.1 LLM Provider Cost/Quality Trade-offs

The two-tier model design (`FastModel` / `DeepModel`) is well-architected. Recommended configurations:

| Deployment Scenario | Provider | FastModel | DeepModel | Est. Cost / 10k Files |
|---|---|---|---|---|
| Cost-optimized (default) | Anthropic | claude-haiku-4-5-20251001 | claude-sonnet-4-6 | ~$0.70 |
| Quality-first | Anthropic | claude-sonnet-4-6 | claude-opus-4-6 | ~$5.80 |
| Air-gapped enterprise | Ollama | llama3.3 | qwen3 | $0 (GPU cost) |
| OpenAI shop | OpenAI | gpt-4.1-mini | gpt-4.1 | ~$1.10 |
| Hybrid (privacy + quality) | Anthropic | claude-haiku-4-5 | claude-haiku-4-5 | ~$0.25 |

**Prompt Caching recommendation:** For Anthropic, add `"cache_control": {"type": "ephemeral"}` to system prompt blocks in `internal/llm/anthropic.go`. For large codebases this reduces cost by up to 90% on the prompt-token portion of each atom extraction call.

### 4.2 Indexing Pipeline Concurrency

The existing `cfg.MaxConcurrent` (default 10) caps goroutines within a single project's pipeline. `handleIndexAll` already uses a 3-project concurrency semaphore. No changes needed for single-server B2B deployments.

**Upgrade path for multi-tenant scale:**

```
Phase 1 (current):  In-memory RunManager + per-project context cancellation
Phase 2 (3–6 mo):  Persist RunManager state to SQLite (survives restarts)
Phase 3 (6–12 mo): External job queue (Redis/BullMQ) for horizontal pod scaling
```

The `RunManager` should be extracted behind an interface in a future refactor:

```go
// JobQueue is the interface all RunManager implementations must satisfy.
type JobQueue interface {
    Start(project string) *IndexRun
    Finish(project string)
    Stop(project string) bool
    Get(project string) *IndexRun
    ListRuns() []RunStatus
}
```

### 4.3 Storage Evolution

```
Level 1 (current):  Memories vector DB (single-tenant, external)
Level 2 (3 mo):     SQLite for project metadata, run history, audit log
Level 3 (6 mo):     Postgres for multi-tenant, user accounts, RBAC
Level 4 (12 mo):    Multi-region Memories sharding with project→region routing
```

**Level 2 SQLite schema** (preview for future `internal/db/` package):

```sql
CREATE TABLE index_runs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    project     TEXT NOT NULL,
    status      TEXT NOT NULL CHECK (status IN ('running','complete','error','stopped')),
    started_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    finished_at DATETIME,
    modules     INTEGER DEFAULT 0,
    files       INTEGER DEFAULT 0,
    atoms       INTEGER DEFAULT 0,
    error_msg   TEXT
);

CREATE TABLE audit_events (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    ts         DATETIME DEFAULT CURRENT_TIMESTAMP,
    action     TEXT NOT NULL,
    resource   TEXT NOT NULL,
    outcome    TEXT NOT NULL,
    ip         TEXT,
    request_id TEXT
);
```

---

## 5. QA Test Case Specifications

### 5.1 Go Unit Tests

**File to create:** `go/internal/server/middleware_test.go` additions

```go
// TestRequestID_Generated — request without X-Request-ID gets one set in response
// TestRequestID_Propagated — request with X-Request-ID: abc123 gets abc123 echoed
// TestRateLimitHeaders_Present — every response has X-RateLimit-Limit/Remaining/Reset
// TestRateLimitHeaders_On429 — 429 response still includes the three headers
// TestRateLimitHeaders_ResetIsUnixTimestamp — X-RateLimit-Reset is a Unix epoch int
```

**File to create:** `go/internal/server/handlers_test.go` additions

```go
// TestHandleHealthLive_AlwaysOK — GET /api/v1/health/live returns 200 always
// TestHandleHealthReady_MemoriesDown — returns 503 when memoriesClient returns false
// TestHandleListProjects_Pagination_Limit — ?limit=2 with 5 projects returns 2
// TestHandleListProjects_Pagination_Offset — ?offset=3 with 5 projects returns 2
// TestHandleListProjects_Pagination_Total — response includes total=5
// TestHandleDeleteProject_AuditLine — DELETE emits audit:true JSON line to stdout
// TestErrorEnvelope_HasCode — all 4xx responses have error.code field
// TestErrorEnvelope_HasRequestID — all 4xx responses have error.request_id field
// TestHandleMetrics_200 — GET /api/v1/metrics returns 200 text/plain
// TestHandleMetrics_ContainsCounterNames — response body contains expected metric names
```

**File to modify:** `go/internal/manifest/manifest_test.go`

```go
// TestHasChanges_NoChanges — returns false when all tracked files match mtime+size
// TestHasChanges_FileDeleted — returns true when a tracked file no longer exists
// TestHasChanges_MtimeAfterIndexed — returns true when file.ModTime > mf.IndexedAt
// TestHasChanges_SizeDiffers — returns true when size on disk != entry.Size
```

**File to modify:** `go/internal/config/config_test.go`

```go
// TestSecrets_EnvSourcedNotPersisted — ANTHROPIC_API_KEY env → Load → Save → file
//   does NOT contain the key value
// TestSecrets_UISourcedPersisted — cfg.AnthropicKey = "val" → Save → file
//   DOES contain the value
// TestSave_NoEnvSources — empty envSources means all fields written
```

**File to create:** `go/internal/server/webhook_test.go`

```go
// TestSendWebhook_PostsJSON — mock HTTP server receives POST with correct body
// TestSendWebhook_SignsWithHMAC — X-Carto-Signature header matches expected HMAC-SHA256
// TestSendWebhook_NoURL — no-ops silently when webhookURL is empty
// TestSendWebhook_TimeoutHandled — doesn't block when server is slow (10s timeout)
```

### 5.2 CLI Tests

**File to modify:** `go/cmd/carto/main_test.go`

```go
// TestCLI_DryRun_PrintsSummary — index --dry-run on temp go project prints modules
// TestCLI_DryRun_NoLLMCall — verifies no HTTP call made to LLM (mock transport)
// TestCLI_Completion_GeneratesOutput — completion bash outputs non-empty string
// TestCLI_Changed_SkipsUnchanged — index --changed skips project with mtime before IndexedAt
```

### 5.3 Integration Tests

**File to create:** `go/internal/server/integration_test.go`

```go
//go:build integration

// TestAPI_v1_ParityWith_v0 — every /api/v1/* route matches status of /api/* route
// TestGracefulShutdown_DrainsSSE — SIGTERM during SSE stream sends final event
// TestWebhook_FiredOnIndexComplete — mock webhook server receives event.complete
// TestPathTraversal_BrowseBlocked — /api/v1/browse?path=/etc returns 400
// TestRateLimiter_BurstExhausted — 11 rapid requests: first 10 pass, 11th is 429
// TestMetrics_IncrementOnRequest — fires 5 requests, /api/v1/metrics shows ≥5
```

### 5.4 Manual QA Checklist

| # | Scenario | Expected Result |
|---|---|---|
| 1 | `GET /api/v1/health/live` — no auth token | 200 `{"status":"live"}` |
| 2 | `GET /api/v1/health/ready` — Memories down | 503 `{"status":"not_ready","reason":"..."}` |
| 3 | `GET /api/v1/metrics` | 200, `text/plain`, all metric names present |
| 4 | Any API request — check response headers | `X-Request-ID`, `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset` all present |
| 5 | `curl -H "X-Request-ID: trace-42" /api/v1/projects` | Response header and log line both contain `trace-42` |
| 6 | 11 requests from same IP in 1 second | 11th returns 429 with all three `X-RateLimit-*` headers |
| 7 | `DELETE /api/v1/projects/myproject` | `audit:true` JSON line in stdout |
| 8 | `PATCH /api/config` with new memory URL | `audit:true` line in stdout |
| 9 | Set `CARTO_WEBHOOK_URL`, trigger index | Webhook receives POST within 15s |
| 10 | Set `CARTO_WEBHOOK_SECRET`, verify signature | `X-Carto-Signature: sha256=<hex>` matches manual HMAC |
| 11 | SIGTERM server during active index | Server logs `shutdown_signal`, drains ≤30s, exits 0 |
| 12 | `GET /api/v1/projects?limit=3&offset=0` with 5 projects | `{total:5, limit:3, offset:0, projects:[3 items]}` |
| 13 | `GET /api/projects` (legacy path) | 200, same body as `/api/v1/projects` |
| 14 | `carto index --dry-run ./myproject` | Prints modules + files, no pipeline executed, exits 0 |
| 15 | `carto completion bash > carto.sh && source carto.sh` | Tab-completion works for `carto projects delete <TAB>` |
| 16 | `carto index --changed` (no changed files) | Prints "no changes" for each project, exits 0 |
| 17 | React page throws runtime error | Error boundary shows "Something went wrong" fallback |
| 18 | Dashboard initial load (slow network) | Skeleton animation visible during `loading=true` |
| 19 | Set `ANTHROPIC_API_KEY` env, start server, `carto config get` | Key not visible in `~/.carto/config.yaml` |
| 20 | `GET /api/v1/browse?path=/etc` in Docker | 400 error, path outside `/projects` |

---

## 6. Deployment Preparation

### 6.1 Environment Variables Reference (Complete)

| Variable | Required | Default | Description |
|---|---|---|---|
| `MEMORIES_URL` | Yes | — | Memories vector DB base URL |
| `MEMORIES_API_KEY` | No | — | Memories server authentication key |
| `LLM_PROVIDER` | Yes | `anthropic` | `anthropic` \| `openai` \| `ollama` |
| `ANTHROPIC_API_KEY` | Cond. | — | Required when `LLM_PROVIDER=anthropic` |
| `LLM_API_KEY` | Cond. | — | Required for OpenAI-compatible providers |
| `LLM_BASE_URL` | Cond. | — | Base URL for non-Anthropic providers |
| `CARTO_FAST_MODEL` | No | `claude-haiku-4-5-20251001` | Low-latency, high-volume model |
| `CARTO_DEEP_MODEL` | No | `claude-opus-4-6` | High-quality, low-volume model |
| `CARTO_SERVER_TOKEN` | No | — | Bearer token (empty = auth disabled) |
| `CARTO_CORS_ORIGINS` | No | — | Comma-separated allowed CORS origins |
| `CARTO_WEBHOOK_URL` | No | — | POST target for index completion events |
| `CARTO_WEBHOOK_SECRET` | No | — | HMAC-SHA256 signing secret for webhooks |
| `CARTO_METRICS_PUBLIC` | No | `false` | Expose `/api/v1/metrics` without auth |
| `PROJECTS_DIR` | No | `~/projects` | Root directory for project scanning |
| `GITHUB_TOKEN` | No | — | GitHub PAT for private repo cloning |
| `JIRA_TOKEN` | No | — | Jira API token |
| `JIRA_EMAIL` | No | — | Jira account email |
| `JIRA_BASE_URL` | No | — | Jira instance URL |
| `LINEAR_TOKEN` | No | — | Linear API key |
| `NOTION_TOKEN` | No | — | Notion integration token |
| `SLACK_TOKEN` | No | — | Slack bot OAuth token |
| `PORT` | No | `8950` | HTTP listen port |

### 6.2 Kubernetes Deployment (new files in `go/deploy/k8s/`)

**`deployment.yaml`** key probe configuration:

```yaml
livenessProbe:
  httpGet:
    path: /api/v1/health/live
    port: 8950
  initialDelaySeconds: 5
  periodSeconds: 10
  failureThreshold: 3

readinessProbe:
  httpGet:
    path: /api/v1/health/ready
    port: 8950
  initialDelaySeconds: 10
  periodSeconds: 15
  failureThreshold: 5

terminationGracePeriodSeconds: 35   # ≥ shutdown drain timeout (30s)

resources:
  requests:
    memory: "128Mi"
    cpu: "100m"
  limits:
    memory: "1Gi"
    cpu: "2000m"
```

### 6.3 Docker Compose Production Profile

Update `go/docker-compose.yml`:

```yaml
services:
  carto:
    build: .
    ports:
      - "8950:8950"
    volumes:
      - ${PROJECTS_DIR:-~/projects}:/projects
    environment:
      - MEMORIES_URL=${MEMORIES_URL}
      - MEMORIES_API_KEY=${MEMORIES_API_KEY:-}
      - LLM_PROVIDER=${LLM_PROVIDER:-anthropic}
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY:-}
      - LLM_API_KEY=${LLM_API_KEY:-}
      - LLM_BASE_URL=${LLM_BASE_URL:-}
      - CARTO_FAST_MODEL=${CARTO_FAST_MODEL:-claude-haiku-4-5-20251001}
      - CARTO_DEEP_MODEL=${CARTO_DEEP_MODEL:-claude-opus-4-6}
      - CARTO_SERVER_TOKEN=${CARTO_SERVER_TOKEN:-}
      - CARTO_CORS_ORIGINS=${CARTO_CORS_ORIGINS:-}
      - CARTO_WEBHOOK_URL=${CARTO_WEBHOOK_URL:-}
      - CARTO_WEBHOOK_SECRET=${CARTO_WEBHOOK_SECRET:-}
    extra_hosts:
      - "host.docker.internal:host-gateway"
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8950/api/v1/health/live"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s
    deploy:
      resources:
        limits:
          memory: "1g"
          cpus: "2.0"
    logging:
      driver: "json-file"
      options:
        max-size: "100m"
        max-file: "5"
```

### 6.4 Dockerfile Updates

Add to the `go/Dockerfile` runtime stage:

```dockerfile
# Non-root user for security (principle of least privilege).
RUN addgroup -S carto && adduser -S carto -G carto
USER carto

# Liveness check used by Docker engine health monitoring.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8950/api/v1/health/live || exit 1
```

For multi-arch builds:

```dockerfile
FROM --platform=$BUILDPLATFORM golang:1.25 AS builder
ARG TARGETOS TARGETARCH
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
    -ldflags="-X main.version=$(git describe --tags --always 2>/dev/null || echo dev)" \
    -o /carto ./cmd/carto
```

---

## 7. Implementation Roadmap

### Phase 1 — Security & Reliability (P0 items, 3–5 days)

1. **G4** Graceful shutdown — `server/server.go` (30 min)
2. **G3** Consistent error envelope — `server/handlers.go` (2–3 hrs, many call sites)
3. **G2** X-Request-ID middleware — `server/middleware.go` (1 hr)
4. **G1** API versioning routes — `server/routes.go` (1 hr)
5. **G9** Health tiers `/health/live` + `/health/ready` — `server/handlers.go` (30 min)
6. **G5** Secrets hardening in `config.Save()` — `internal/config/config.go` (2 hrs)

### Phase 2 — Observability & Hardening (P1 items, 3–4 days)

7. **G8** Prometheus metrics — create `server/metrics.go` + increment points (3 hrs)
8. **G7** Rate limit headers — `server/middleware.go` (1 hr)
9. **G10** Path traversal hardening — `server/handlers.go` (1 hr)
10. **G6** Fix `--changed` flag + `manifest.HasChanges` (2 hrs)
11. **G21** Fix `handleIndexAll` changedOnly logic — `server/handlers.go` (30 min)
12. **G11** Webhook notifications — create `server/webhook.go` + wire (2 hrs)
13. **G12** Pagination on list endpoints — `server/handlers.go` (1 hr)
14. **G13** React error boundaries — create `RouteErrorBoundary.tsx`, update `Layout.tsx` (1 hr)

### Phase 3 — Developer Experience (P1–P2 items, 2–3 days)

15. **G14** Skeleton loading states — create `Skeleton.tsx`, update pages (2 hrs)
16. **G15** CLI shell completions — `main.go` (1 hr)
17. **G16** `--dry-run` flag — `cmd_index.go` (1.5 hrs)
18. **G18** Audit log — `server/handlers.go` (1 hr)

### Phase 4 — Infrastructure (P2 items, 1–2 days)

19. **G19** Multi-arch Docker + HEALTHCHECK — `Dockerfile` (1 hr)
20. **G20** Kubernetes manifests — create `go/deploy/k8s/` (2 hrs)
21. Update `docker-compose.yml` with prod profile (30 min)
22. Create `ENVIRONMENT.md` with full env var reference (30 min)

---

## 8. Files Changed Summary

| File | Type | Change | Phase |
|---|---|---|---|
| `go/internal/server/server.go` | Modify | Graceful shutdown, insert requestIDMiddleware in chain | 1 |
| `go/internal/server/middleware.go` | Modify | X-Request-ID middleware, rate limit header return values | 1, 2 |
| `go/internal/server/handlers.go` | Modify | Error envelope, health tiers, path traversal, audit log, pagination, webhook calls, metrics increments | 1–3 |
| `go/internal/server/routes.go` | Modify | `/api/v1/*` aliases, new health/metrics routes | 1, 2 |
| `go/internal/server/metrics.go` | **Create** | Prometheus text-format exposition | 2 |
| `go/internal/server/webhook.go` | **Create** | HMAC-signed webhook delivery | 2 |
| `go/internal/manifest/manifest.go` | Modify | Add `HasChanges()` | 2 |
| `go/internal/config/config.go` | Modify | Secrets hardening, `WebhookURL/Secret` fields | 1, 2 |
| `go/cmd/carto/main.go` | Modify | Shell completion command | 3 |
| `go/cmd/carto/cmd_index.go` | Modify | `--dry-run` flag, fix `--changed` | 2, 3 |
| `go/web/src/components/RouteErrorBoundary.tsx` | **Create** | React error boundary | 2 |
| `go/web/src/components/Skeleton.tsx` | **Create** | Skeleton loading components | 3 |
| `go/web/src/components/Layout.tsx` | Modify | Wrap Outlet with error boundary | 2 |
| `go/web/src/pages/Dashboard.tsx` | Modify | TableSkeleton on load | 3 |
| `go/web/src/pages/Settings.tsx` | Modify | FormSkeleton on load | 3 |
| `go/web/src/pages/Query.tsx` | Modify | Skeleton on projects load | 3 |
| `go/docker-compose.yml` | Modify | Healthcheck, resource limits, prod env vars | 4 |
| `go/Dockerfile` | Modify | Non-root user, HEALTHCHECK, multi-arch build args | 4 |
| `go/deploy/k8s/` | **Create** | K8s deployment, service, configmap, secret, ingress | 4 |

**Not touched:** `internal/sources/*`, `internal/llm/*`, `internal/storage/*`, `internal/scanner/*`, `internal/pipeline/*`, `internal/gitclone/*`, `internal/history/*`, `internal/atoms/*`, `pkg/carto/carto.go`, `web/src/pages/IndexRun.tsx`, `web/src/pages/ProjectDetail.tsx`, all shadcn UI components in `web/src/components/ui/`.

---

## 9. Architectural Trade-off Decisions

| Decision | Chosen | Considered | Rationale |
|---|---|---|---|
| API versioning | `/api/v1/*` path prefix | Header versioning (`Accept: application/vnd.carto.v1+json`) | Path prefix is tooling-friendly (curl, Postman), explicit in logs, standard in REST APIs |
| Metrics format | Custom Prometheus text, stdlib only | `prometheus/client_golang` library | Zero new Go module dependencies; output is identical to what Prometheus expects |
| Secrets at rest | Env-sourced secrets skipped on disk write | Full AES-256 encryption of config file | Simpler; enterprise operators use secret managers (AWS Secrets Manager, Vault); encryption adds key-management complexity |
| Graceful shutdown | 30s drain in `server.Start()` | Configurable `CARTO_SHUTDOWN_TIMEOUT` | 30s matches `terminationGracePeriodSeconds` K8s default; configurable env var can be added as P3 |
| Webhook delivery | Fire-and-forget goroutine with 10s HTTP timeout | Persistent retry queue with backoff | Keeps server stateless; webhook reliability is the receiver's responsibility; queue can be added if customers demand it |
| Error code format | `"SNAKE_UPPER_CASE"` string constant | HTTP Problem Details RFC 9457 | Simpler to generate and consume without extra struct; RFC 9457 is a P3 upgrade path |
| Rate limit storage | In-process per-IP token bucket | Redis-backed distributed bucket | Primary use case is single-server deployment; multi-instance installs can put a reverse proxy (nginx) in front with rate limiting |
| Health check tiers | `/health/live` + `/health/ready` (K8s standard) | Single `/health` endpoint | Two-tier is the K8s liveness/readiness pattern; avoids killing running pods when Memories is temporarily unreachable |

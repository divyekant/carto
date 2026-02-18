# Carto v3 Enhancements Design

**Date**: 2026-02-18
**Status**: Approved
**Author**: DK + Claude

## Overview

Comprehensive enhancement plan for Carto (Indexer) covering four tiers of improvements plus three major feature additions. Transforms Carto from a CLI-only Anthropic-locked tool into a multi-provider, UI-managed codebase intelligence platform.

## Housekeeping: FAISS → Memories Rename

Remove every remaining reference to "faiss" / "FAISS" across the codebase. Pure rename, no behavior changes.

| File | Change |
|------|--------|
| `internal/storage/faiss.go` | Rename file to `memories.go`. Rename `FaissClient` → `MemoriesClient`, `NewFaissClient` → `NewMemoriesClient` |
| `internal/storage/store.go` | Rename `FaissAPI` interface → `MemoriesAPI` |
| `internal/storage/faiss_test.go` | Rename file to `memories_test.go`, update all type references |
| `internal/storage/store_test.go` | Update mock type names |
| `internal/pipeline/pipeline.go` | `FaissClient` field → `MemoriesClient` in Config struct |
| `cmd/carto/main.go` | Update client creation variable names and config field |
| `internal/config/config.go` | Remove `FAISS_URL` / `FAISS_API_KEY` legacy fallbacks entirely |
| `internal/config/config_test.go` | Remove legacy fallback tests |
| `README.md`, `ARCHITECTURE.md`, `CONTRIBUTING.md` | Replace any remaining "FAISS" text with "Memories" |

---

## Tier 1 — Critical (Blocks Production)

### E1. Binary File Filtering

**Problem**: Scanner includes files with unknown extensions (`.pyc`, `.o`, `.so`, `.exe`, ONNX blobs). Sending these to Haiku causes 413 "request too large" errors.

**Solution**:
- Add `isBinary()` function to `internal/scanner/scanner.go`
- Two-layer detection:
  1. **Extension blocklist**: `.pyc`, `.pyo`, `.o`, `.so`, `.dylib`, `.dll`, `.exe`, `.wasm`, `.class`, `.jar`, `.war`, `.onnx`, `.bin`, `.dat`, `.db`, `.sqlite`, `.png`, `.jpg`, `.jpeg`, `.gif`, `.bmp`, `.ico`, `.webp`, `.svg`, `.pdf`, `.doc`, `.docx`, `.xls`, `.xlsx`, `.ppt`, `.pptx`, `.zip`, `.tar`, `.gz`, `.bz2`, `.7z`, `.rar`, `.mp3`, `.mp4`, `.avi`, `.mov`, `.wav`, `.ttf`, `.woff`, `.woff2`, `.eot`
  2. **Magic byte detection**: Read first 512 bytes, check for null bytes (`\x00`)
- Skip binary files during `scanner.Scan()`, log debug message
- Add tests to `scanner_test.go`

### E2. Wire Up `carto patterns` Command

**Problem**: CLI command is stubbed — prints "not yet implemented". But `patterns/generator.go` has full implementation already written and tested.

**Solution**:
In `cmd/carto/main.go:runPatterns()`:
1. Load config, create LLM client + Memories client
2. Create a `storage.Store` and retrieve existing blueprint/patterns/zones from Memories for the project
3. Build `patterns.Input` from retrieved data
4. Call `patterns.WriteFiles(absPath, input, format)`
5. Print success message with files written

---

## Tier 2 — High (Correctness / Reliability)

### E3. Memories Health Check at Startup

**Problem**: `MemoriesClient.Health()` method exists but is never called. Pipeline can burn 6+ minutes of LLM calls before discovering Memories is unreachable.

**Solution**:
- Call `MemoriesClient.Health()` at start of `pipeline.Run()` before Phase 1
- If unhealthy, return early: `"memories server unreachable at %s — start it or check MEMORIES_URL"`

### E4. OAuth Beta Headers Per Model Tier

**Problem**: `interleaved-thinking-2025-05-14` beta header sent on ALL calls including Haiku.

**Solution**:
- Add `isOpus bool` parameter to the internal HTTP request builder in `llm/client.go`
- Only include thinking beta header when `isOpus == true`
- `Complete()` (Haiku) passes `false`, `CompleteOpus()` passes `true`

### E5. OAuth Token Refresh Race Fix

**Problem**: Expiry check in `Complete()` happens outside lock — multiple goroutines trigger redundant refresh calls.

**Solution**:
- Remove the pre-check outside the lock in `Complete()` (lines ~106-112)
- Move all expiry checking into `refreshOAuthToken()` which already has the double-check pattern with proper locking
- Call `refreshOAuthToken()` unconditionally when `opts.IsOAuth` is true — it returns immediately if token is still valid

### E6. OAuth Token Payload Validation

**Problem**: `refreshOAuthToken` accepts empty tokens from response without validation.

**Solution**:
- After JSON decode, validate: `if result.AccessToken == "" { return fmt.Errorf("llm: oauth refresh returned empty access token") }`

### E7. Manifest Concurrent Write Safety

**Problem**: Two concurrent `carto index` on the same project can corrupt `manifest.json`.

**Solution**:
- In `manifest.Save()`: open file with `os.OpenFile(O_CREATE|O_WRONLY)`, acquire `syscall.Flock(fd, LOCK_EX)`, write, unlock
- In `manifest.Load()`: acquire shared lock `syscall.Flock(fd, LOCK_SH)` during read

---

## Tier 3 — Medium (Robustness)

### E8. Git History: Distinguish Expected vs Unexpected Errors

**Problem**: `history/extractor.go` returns empty history for ALL `git log` errors.

**Solution**:
- Check `exec.ExitError` — if exit code 128 (not a git repo) or 127 (git not found), return empty silently
- All other errors: `log.Printf("history: warning: %s: %v", relPath, err)`, still return empty but also attach to a warnings channel

### E9. Signal Plugin Errors: Add Logging

**Problem**: `signals/source.go:49-54` has comment "Log warning" but no actual logging.

**Solution**:
- Add `log.Printf("signals: warning: source %s failed for module %s: %v", s.Name(), module.Name, err)` before `continue`

### E10. Manifest Hash Errors Collected in Result

**Problem**: Hash computation failures are logged but not added to `result.Errors`.

**Solution**:
- Append `fmt.Errorf("hash failed for %s: %w", relPath, hashErr)` to `result.Errors`

### E11. Content Truncation Warning

**Problem**: `storage/store.go` silently truncates atoms > 49KB.

**Solution**:
- Add `log.Printf("storage: warning: content truncated from %d to %d chars for source %s", len(content), maxContentLen, source)` when truncation occurs

### E12. Module Filter Validation

**Problem**: `--module foo` silently indexes nothing if "foo" doesn't exist.

**Solution**:
- After scan in `pipeline.go`, if `cfg.ModuleFilter != ""`, check against `scanResult.Modules`
- If not found, return error: `"module %q not found. available: %v"`

---

## Tier 4 — Polish

### E13. Memories Health Response Body Consumed

**Problem**: `memories.go` `Health()` doesn't read body before close, hurting HTTP connection reuse.

**Solution**:
- Add `io.Copy(io.Discard, resp.Body)` before `resp.Body.Close()` in `Health()`

### E14. Progress Callback Outside Lock

**Problem**: `pipeline.go:224-230` calls progress callback while holding `atomsMu` lock.

**Solution**:
```go
atomsMu.Lock()
// ... update state ...
atomsDone++
d := atomsDone
atomsMu.Unlock()
progress("atoms", d, len(work))  // After lock release
```

### E15. CLI Tests

**Problem**: `cmd/carto/main.go` has zero test coverage.

**Solution**:
- Add `cmd/carto/main_test.go` with smoke tests:
  - `carto --help` exits 0
  - `carto modules <testdata>` lists modules
  - `carto status <testdata>` shows status
  - `carto index` without args shows usage

---

## Feature F1: Multi-LLM Provider Support

### Architecture

Abstract the LLM client behind a `Provider` interface.

```
internal/llm/
  ├── provider.go        // Provider interface + factory
  ├── anthropic.go       // Current client.go refactored (Anthropic-specific)
  ├── openai.go          // OpenAI-compatible provider (OpenAI + OpenRouter)
  ├── ollama.go          // Ollama provider (local models)
  └── client.go          // Backward-compat wrapper (delegates to Provider)
```

### Provider Interface

```go
type Provider interface {
    Complete(ctx context.Context, req CompletionRequest) (string, error)
    Name() string
}

type CompletionRequest struct {
    Model      string
    System     string
    User       string
    MaxTokens  int
    IsDeepTier bool  // Signals this is an expensive/deep analysis call
}
```

### Supported Providers

| Provider | Base URL | Auth | Notes |
|----------|----------|------|-------|
| `anthropic` | `https://api.anthropic.com` | API key or OAuth | Current default, full feature parity |
| `openai` | `https://api.openai.com` | API key | GPT-4o as Opus, GPT-4o-mini as Haiku |
| `openrouter` | `https://openrouter.ai/api` | API key | Any model via OpenAI-compatible API |
| `ollama` | `http://localhost:11434` | None | Local models, `/api/chat` endpoint |

OpenRouter and OpenAI share the same API shape (OpenAI-compatible), so `openai.go` handles both with different base URLs.

### Config Additions

```
LLM_PROVIDER=anthropic|openai|openrouter|ollama  (default: anthropic)
LLM_API_KEY=...                                   (falls back to ANTHROPIC_API_KEY)
LLM_BASE_URL=...                                  (override for OpenRouter/Ollama)
LLM_HAIKU_MODEL=...                               (cheap/fast model per provider)
LLM_OPUS_MODEL=...                                (expensive/smart model per provider)
```

### JSON Extraction

The existing `extractJSON()` function in `client.go` is provider-agnostic — it strips markdown fences and finds JSON objects. This stays as a shared utility used by all providers.

---

## Feature F2: Management UI

### Architecture

Go HTTP server with embedded React SPA. Single binary distribution.

```
internal/server/
  ├── server.go          // HTTP server setup, go:embed, SPA fallback
  ├── routes.go          // API route registration
  ├── handlers.go        // REST API handler functions
  └── sse.go             // Server-Sent Events for live progress
web/
  ├── src/
  │   ├── App.tsx
  │   ├── pages/
  │   │   ├── Dashboard.tsx     // Project overview, last index times, health
  │   │   ├── Projects.tsx      // List/add/remove indexed projects
  │   │   ├── IndexRun.tsx      // Trigger index, live progress stream
  │   │   ├── Query.tsx         // Interactive query with tier picker
  │   │   └── Settings.tsx      // LLM provider, models, Memories URL
  │   └── components/
  │       ├── ProjectCard.tsx
  │       ├── ProgressBar.tsx
  │       └── QueryResult.tsx
  ├── package.json              // React + Vite + Tailwind
  └── dist/                     // Built output, embedded via go:embed
```

### New CLI Command

```
carto serve [--port 8950] [--no-browser]
```

Starts the HTTP server, optionally opens browser.

### API Endpoints

```
GET    /api/health                  — server + Memories health
GET    /api/projects                — list indexed projects (from Memories)
POST   /api/projects/index          — trigger index run { path, incremental, module }
GET    /api/projects/:name/status   — SSE stream of index progress
POST   /api/query                   — execute query { text, project, tier, k }
GET    /api/config                  — current config (redacted keys)
PATCH  /api/config                  — update config fields
GET    /api/integrations            — list configured signal sources
POST   /api/integrations            — add signal source { type, config }
DELETE /api/integrations/:name      — remove signal source
GET    /                            — serve embedded SPA
GET    /*                           — SPA fallback for client-side routing
```

### Key Features

- **Real-time progress**: SSE stream during index runs (reuses existing `ProgressFn` callback)
- **Project management**: Add project path, trigger full/incremental index, view status
- **Interactive query**: Free-text search with tier selection, formatted results
- **Config editing**: Change LLM provider, models, Memories URL through the UI
- **Single binary**: `go:embed` the built React app into the Go binary

### Tech Stack

- **Frontend**: React 19 + Vite + Tailwind CSS (shadcn/ui components)
- **Embedding**: `//go:embed web/dist/*` in `server.go`
- **Build**: `cd web && npm run build` before `go build`

---

## Feature F3: Integration Management via UI

### Architecture

Extends the existing `signals/` plugin system with persistent configuration and new sources.

```
internal/signals/
  ├── source.go          // Existing interface + registry (unchanged)
  ├── git.go             // Existing git signal source (unchanged)
  ├── github.go          // NEW: GitHub PRs, issues via REST API
  ├── jira.go            // NEW: Jira tickets via REST API
  ├── linear.go          // NEW: Linear issues via GraphQL
  └── config.go          // NEW: Persistent config (.carto/integrations.json)
```

### Integration Config Format

```json
// .carto/integrations.json
{
  "sources": [
    {
      "type": "github",
      "name": "my-github",
      "config": {
        "token": "<encrypted>",
        "owner": "divyekant",
        "repo": "indexer"
      }
    },
    {
      "type": "jira",
      "name": "work-jira",
      "config": {
        "url": "https://company.atlassian.net",
        "email": "dk@company.com",
        "token": "<encrypted>"
      }
    }
  ]
}
```

### UI Flow

Settings → Integrations tab → Add Integration → Select type (GitHub/Jira/Linear) → Enter credentials → Test connection → Save

### Signal Source Implementations

| Source | Fetches | API |
|--------|---------|-----|
| `github` | Open PRs, recent issues, PR reviews touching module files | GitHub REST v3 |
| `jira` | Tickets linked via commit messages or branch names | Jira REST v3 |
| `linear` | Issues linked via branch names or commit refs | Linear GraphQL |

Each source implements the existing `SignalSource` interface — no changes needed to the pipeline. The registry auto-loads from `.carto/integrations.json` at startup.

### Token Encryption

Tokens encrypted at rest using AES-256-GCM with a machine-derived key (`os.Hostname()` + fixed salt). Not bank-grade but prevents plaintext secrets in config files.

---

## Execution Order

| Phase | Items | Est. Scope |
|-------|-------|------------|
| 1 | FAISS → Memories rename | Small (rename + tests) |
| 2 | Tier 1: E1 (binary filter), E2 (patterns cmd) | Small |
| 3 | Tier 2: E3–E7 (reliability fixes) | Medium |
| 4 | F1: Multi-LLM providers | Medium |
| 5 | Tier 3: E8–E12 (robustness) | Small |
| 6 | Tier 4: E13–E15 (polish) | Small |
| 7 | F2 + F3: UI + Integrations | Large |

---

## Files Changed/Created Summary

### Modified (existing)
- `internal/storage/faiss.go` → `memories.go`
- `internal/storage/faiss_test.go` → `memories_test.go`
- `internal/storage/store.go`, `store_test.go`
- `internal/pipeline/pipeline.go`, `pipeline_test.go`
- `internal/scanner/scanner.go`, `scanner_test.go`
- `internal/llm/client.go`, `client_test.go`
- `internal/config/config.go`, `config_test.go`
- `internal/history/extractor.go`
- `internal/signals/source.go`
- `internal/manifest/manifest.go`
- `cmd/carto/main.go`
- `README.md`, `ARCHITECTURE.md`, `CONTRIBUTING.md`

### New Files
- `internal/llm/provider.go` — Provider interface + factory
- `internal/llm/anthropic.go` — Refactored Anthropic provider
- `internal/llm/openai.go` — OpenAI/OpenRouter provider
- `internal/llm/ollama.go` — Ollama provider
- `internal/server/server.go` — HTTP server + embedded SPA
- `internal/server/routes.go` — API route registration
- `internal/server/handlers.go` — REST handlers
- `internal/server/sse.go` — SSE for live progress
- `internal/signals/github.go` — GitHub signal source
- `internal/signals/jira.go` — Jira signal source
- `internal/signals/linear.go` — Linear signal source
- `internal/signals/config.go` — Integration config persistence
- `cmd/carto/main_test.go` — CLI smoke tests
- `web/` — Entire React SPA directory

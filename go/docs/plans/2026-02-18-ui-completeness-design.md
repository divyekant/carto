# UI Completeness Design

**Date:** 2026-02-18
**Status:** Approved

## Problem

The Carto web UI has several gaps that make it incomplete for real usage:
- Critical bugs where config changes are silently ignored by the pipeline
- Missing error detail display after indexing
- No useful dashboard information
- No toast notification system
- Docker volume prevents manifest writes

## Changes

### Tier 1 — Critical Bugs

#### 1. Memories client not refreshed after Settings changes

**Bug:** `s.memoriesClient` is created once at server boot from env vars. When a user updates `memories_url` or `memories_key` via Settings, only `s.cfg` is updated — `runIndex()` still uses the stale `s.memoriesClient`. This causes 401 errors when the API key was set via UI rather than env var.

**Fix:** In `runIndex()`, create a fresh `storage.NewMemoriesClient()` from the current `s.cfg` snapshot (same pattern used for `llmClient`). Apply Docker localhost rewrite (`host.docker.internal`) at pipeline time when `isDocker()` returns true.

**Files:** `internal/server/handlers.go`

#### 2. Docker volume mounted read-only

**Bug:** `docker-compose.yml` mounts projects as `:ro`. The pipeline needs to write `.carto/manifest.json` for incremental indexing. This also breaks `handleListProjects` since it looks for manifests.

**Fix:** Change `:ro` to `:rw` in docker-compose.yml.

**Files:** `docker-compose.yml`

### Tier 2 — Missing Features

#### 3. Error messages not shown in IndexRun results

The Go `IndexResult` struct has `ErrMsgs []string` but the frontend `CompleteData` TypeScript interface omits it. Users see "Errors: 8" but can't see what went wrong.

**Fix:**
- Add `error_messages?: string[]` to `CompleteData` interface
- When `errors > 0`, render an expandable section in the results card
- Clickable row shows "N errors" with a chevron; expands to show individual error messages in a scrollable list
- Each message styled with red text and error icon

**Files:** `web/src/pages/IndexRun.tsx`

#### 4. Dashboard — last run status per project

Currently shows name, path, file count, indexed_at. No indication of whether the last run succeeded or failed.

**Fix:**
- Merge data from `/api/projects` and `/api/projects/runs` on the frontend
- Update `ProjectCard` to show a status badge: checkmark (success), X (error), spinner (running)
- Add a "Re-index" button that navigates to `/index` with the project path pre-filled
- Show error summary if the last run failed

**Files:** `web/src/pages/Dashboard.tsx`, `web/src/components/ProjectCard.tsx`

#### 5. Toast notifications

No global notification system. Config save shows an inline message that's easy to miss.

**Fix:**
- Install `sonner` (lightweight toast library, standard in shadcn projects)
- Add `<Toaster />` to the app layout
- Replace inline save messages in Settings with toasts
- Add toasts for: indexing started, connection test results, SSE disconnect errors

**Files:** `web/src/App.tsx` or layout, `web/src/pages/Settings.tsx`, `web/src/pages/IndexRun.tsx`

### Tier 3 — Polish

#### 6. Docker environment hint in Settings

When running in Docker, localhost URLs don't reach the host. We rewrite silently but users don't know.

**Fix:** Show a small info banner at the top of Settings when the health endpoint indicates Docker mode. Add `docker` field to health response.

**Files:** `internal/server/handlers.go` (health endpoint), `web/src/pages/Settings.tsx`

#### 7. Query page — client-side pagination

Large result sets dump everything at once.

**Fix:** Show first 20 results with a "Show more" button that reveals the next batch. Pure client-side since the server `K` param already limits.

**Files:** `web/src/pages/Query.tsx`

#### 8. Run history — in-memory persistence

`RunManager` deletes runs after 30 seconds. Navigating away and back loses the result.

**Fix:** Keep the 30-second cleanup for SSE late-connect, but maintain a separate `lastRuns map[string]*RunStatus` that persists the most recent run per project until server restart. The `/api/projects/runs` endpoint reads from this map. Full disk persistence is YAGNI.

**Files:** `internal/server/sse.go`

## Architecture Notes

### Config flow (both paths)

Support both env vars (default) and Settings UI (override):
1. Server boots with `config.Load()` from env vars
2. User can override any field via Settings UI → `PATCH /api/config`
3. `runIndex()` snapshots `s.cfg` and creates fresh clients each time
4. Env vars remain the source of truth on restart; UI changes are session-only

### No new dependencies except sonner

All changes use existing shadcn components (Collapsible, Badge) plus `sonner` for toasts. No new backend dependencies.

## Non-goals

- Persistent config to disk (security risk with API keys)
- Full run history with logs (YAGNI — only last run per project)
- WebSocket replacement for SSE (SSE is fine for unidirectional progress)
- User authentication (single-user tool)

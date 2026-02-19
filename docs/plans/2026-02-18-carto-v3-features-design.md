# Carto v3 Features Design

**Date:** 2026-02-18
**Status:** Approved

## Overview

Three features to make Carto deployable anywhere, indexable from anywhere, and context-rich from external sources. Plus UX improvements to the indexing flow.

## Feature 1: Universal Deployment (Memories Client Auto-Routing)

### Problem

Docker URL rewriting (`localhost` → `host.docker.internal`) is scattered across 3 places: `main.go` startup, `handlePatchConfig`, and `runIndex`. Non-Docker users can't deploy without hitting connection issues. Config persistence saves raw `localhost` URLs that break inside Docker on restart.

### Design

Centralize URL resolution into a single function in `config`:

```go
// config/config.go
func ResolveURL(rawURL string) string {
    if !isDocker() {
        return rawURL
    }
    url = strings.Replace(url, "localhost", "host.docker.internal", 1)
    url = strings.Replace(url, "127.0.0.1", "host.docker.internal", 1)
    return url
}
```

**Rules:**
- Config **stores** the user-facing URL (e.g., `http://localhost:8900`)
- Config **resolves** at runtime based on environment
- `isDocker()` moves from `server/routes.go` to `config/config.go`
- All callers use `config.ResolveURL(cfg.MemoriesURL)` instead of inline rewriting
- Remote URLs (not localhost/127.0.0.1) pass through unchanged

**Files changed:**
- `internal/config/config.go` — add `ResolveURL()`, `isDocker()`
- `cmd/carto/main.go` — remove inline Docker rewrite, use `config.ResolveURL()`
- `internal/server/handlers.go` — remove inline Docker rewrites in `handlePatchConfig` and `runIndex`, use `config.ResolveURL()`
- `internal/server/routes.go` — remove `isDocker()` (moved to config)

## Feature 2: Git Repo URL Indexing

### Problem

Indexing only works with local filesystem paths. Users should be able to paste a GitHub URL and index any public or private repo.

### Design

**New package:** `internal/gitclone/`

```go
// gitclone/clone.go
type CloneOptions struct {
    URL      string
    Branch   string // optional, defaults to HEAD
    Token    string // GitHub PAT for private repos
    Depth    int    // default 1 (shallow clone)
}

type CloneResult struct {
    Dir     string // temp directory with cloned repo
    Cleanup func() // removes temp dir
}

func Clone(opts CloneOptions) (*CloneResult, error)
```

**Flow:**
1. Detect if input is a URL (`https://` or `git@`) or local path
2. If URL: clone to temp dir with `git clone --depth 1`, inject token into HTTPS URL for private repos
3. Run normal pipeline against cloned directory
4. Store repo URL in manifest metadata as `origin_url`
5. Cleanup temp dir after indexing

**Index request changes:**
```go
type indexRequest struct {
    Path        string `json:"path"`
    URL         string `json:"url"`          // NEW: git repo URL
    Branch      string `json:"branch"`       // NEW: optional branch
    Incremental bool   `json:"incremental"`
    Module      string `json:"module"`
    Project     string `json:"project"`
}
```

If `url` is provided, it takes precedence over `path`. Project name defaults to repo name.

**Settings addition:**
- `github_token` field in Settings UI and config persistence
- Used automatically when cloning private repos

**UI changes to IndexRun.tsx:**
- Tab toggle: "Local Path" | "Git URL"
- Git URL tab: URL input + optional branch input
- Local Path tab: folder picker (see Feature 4 below)

## Feature 3: External Source Integrations

### Problem

Only git commit history is extracted as signals. Codebases have rich context in GitHub issues/PRs, Jira, Linear, Google Docs, and local PDFs.

### Design: Two Integration Categories

**Category A: Code-Linked Signals** (extends existing `SignalSource` interface)

These produce `Signal` structs linked to modules/files. The existing interface works:

```go
type SignalSource interface {
    Name() string
    Configure(cfg map[string]string) error
    FetchSignals(module Module) ([]Signal, error)
}
```

Sources:
1. **GitSignalSource** (existing) — commits + PR references
2. **GitHubSignalSource** (new) — issues, PRs, discussions via GitHub API
3. **JiraSignalSource** (future) — tickets linked via commit messages
4. **LinearSignalSource** (future) — issues linked via branch names

**Category B: Knowledge Sources** (new interface for project-level documents)

These produce standalone documents not tied to specific modules:

```go
// internal/knowledge/knowledge.go
type KnowledgeSource interface {
    Name() string
    Configure(cfg map[string]string) error
    FetchDocuments(project string) ([]Document, error)
}

type Document struct {
    Title   string
    Content string
    URL     string // original location
    Type    string // "pdf", "gdoc", "jira-doc", etc.
}
```

Sources:
1. **LocalPDFSource** — reads PDFs from a configured directory
2. **GoogleDocsSource** — fetches docs via Google Docs API (requires OAuth)
3. **Confluence/Notion** (future)

**Storage:**
- Code-linked signals: stored as `carto/{project}/{module}/layer:signals` (existing)
- Knowledge docs: stored as `carto/{project}/_knowledge/source:{name}/{doc-title}` (new)
- Knowledge docs included in `standard` and `full` tier retrieval

**New package:** `internal/knowledge/`

**Registry pattern** (mirrors signals):
```go
type Registry struct {
    sources []KnowledgeSource
}
func (r *Registry) Register(s KnowledgeSource)
func (r *Registry) FetchAll(project string) ([]Document, error)
```

**Pipeline integration:**
- New Phase 3b after signals: fetch knowledge documents
- Store in Phase 5 alongside other layers

**Settings UI:**
- New "Integrations" tab/section in Settings
- Per-source configuration: API tokens, paths, project IDs
- Test connectivity button per source

### Implementation Order

| Step | Source | Type | Complexity |
|------|--------|------|------------|
| 1 | GitHub Issues + PRs | Signal | Medium — REST API, needs repo owner/name parsing |
| 2 | Local PDFs | Knowledge | Low — file read + text extraction (Go PDF lib) |
| 3 | Google Docs | Knowledge | Medium — OAuth flow + API integration |
| 4 | Jira | Signal | Medium — REST API + JQL queries |
| 5 | Linear | Signal | Low — GraphQL API, similar pattern to GitHub |

## Feature 4: Folder Picker for Local Path

### Problem

Manually typing filesystem paths is error-prone. Users can easily mistype paths or not know the exact path.

### Design

**Server-side directory browser API:**

```
GET /api/browse?path=/Users/dk/projects
```

Returns:
```json
{
  "current": "/Users/dk/projects",
  "parent": "/Users/dk",
  "directories": [
    {"name": "indexer", "path": "/Users/dk/projects/indexer"},
    {"name": "WebChat", "path": "/Users/dk/projects/WebChat"},
    ...
  ]
}
```

- Only returns directories (not files) — this is for picking a project root
- In Docker mode, scoped to `/projects` mount (can't browse outside)
- In non-Docker mode, starts at home directory or configured projects dir

**UI: FolderPicker component**

- Replaces the raw text input on the IndexRun page (Local Path tab)
- Shows current directory with breadcrumb navigation
- Click a folder to navigate into it
- "Select" button to confirm the current directory
- "Up" button / breadcrumb to go to parent
- Compact modal or inline dropdown style

**Security:**
- Server validates path is within allowed roots
- Docker: only `/projects` and subdirectories
- Non-Docker: configurable root, defaults to home directory

## Summary of New Packages/Files

| Package/File | Purpose |
|---|---|
| `internal/gitclone/` | Git repo cloning with token auth |
| `internal/knowledge/` | Knowledge source interface, registry, PDF/GDocs sources |
| `internal/signals/github.go` | GitHub Issues + PRs signal source |
| `internal/config/config.go` | Add `ResolveURL()`, `isDocker()` |
| `internal/server/handlers.go` | Add `handleBrowse()`, update `handleStartIndex()` |
| `go/web/src/pages/IndexRun.tsx` | Tab toggle, folder picker, git URL input |
| `go/web/src/components/FolderPicker.tsx` | Directory browser component |
| `go/web/src/pages/Settings.tsx` | Integrations section |

## Non-Goals (for now)

- Webhook-based real-time signal updates (poll on index for now)
- Full Notion API integration (complex, defer)
- Git submodule support in cloned repos
- Multi-branch indexing (one branch at a time)

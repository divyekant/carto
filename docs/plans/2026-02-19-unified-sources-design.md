# Carto Unified Sources Design

**Date:** 2026-02-19
**Status:** Approved

## Overview

Replace the separate `SignalSource` and `KnowledgeSource` interfaces with a single unified `Source` interface. All external integrations — code-linked signals, project-level knowledge, and hybrid context — produce `Artifact` structs with explicit categories. This enables 8 source types with consistent configuration, storage, and retrieval.

## Problem

The current two-interface split (`signals.SignalSource` + `knowledge.KnowledgeSource`) has structural issues:

1. **Redundant API calls:** GitHub/Jira are project-scoped but `SignalSource.FetchSignals` runs per-module, causing N redundant API calls per index run.
2. **Knowledge not retrievable:** Knowledge docs are stored but not wired into `RetrieveByTier` — they can't be queried.
3. **No file linking:** `Signal.Files` exists but no source populates it.
4. **No per-source config:** Sources are hardcoded in `runIndex`. No UI or yaml-based configuration.
5. **Local repos miss GitHub signals:** Only Git-URL-based indexing activates GitHub signals, even when a local repo has a GitHub remote.
6. **80% shared pattern:** Both interfaces share Name/Configure/Fetch but diverge awkwardly.

## Core Data Model

### Source Interface

```go
type Source interface {
    Name() string
    Scope() Scope
    Configure(cfg SourceConfig) error
    Fetch(ctx context.Context, req FetchRequest) ([]Artifact, error)
}

type Scope int
const (
    ProjectScope Scope = iota  // fetch once per project
    ModuleScope                // fetch per module (git commits only)
)

type FetchRequest struct {
    Project    string
    Module     string   // set only for ModuleScope
    ModulePath string   // filesystem path, ModuleScope only
    RepoRoot   string   // root of the codebase
}

type SourceConfig struct {
    Credentials map[string]string  // from Settings / env vars
    Settings    map[string]string  // from .carto/sources.yaml or auto-detect
}
```

### Artifact

```go
type Artifact struct {
    Source   string            // "github", "jira", "notion", "local-pdf", "slack", "web"
    Category Category
    ID       string            // unique within source
    Title    string
    Body     string
    URL      string
    Files    []string          // optional: linked file paths relative to repo root
    Module   string            // optional: linked module name
    Date     time.Time
    Author   string
    Tags     map[string]string // structured metadata: "state", "priority", "channel", etc.
}

type Category string
const (
    Signal    Category = "signal"    // file-linked: commits, PRs, issues, tickets
    Knowledge Category = "knowledge" // project-level: PDFs, docs, RFCs, web pages
    Context   Category = "context"   // hybrid: Slack threads, GitHub discussions
)
```

### Design Decisions

- **Scope** fixes redundant API calls. Only `git` is `ModuleScope`; all others are `ProjectScope`.
- **Category** determines storage key pattern and retrieval tier inclusion.
- **Tags** is for structured metadata (state/priority/labels), not a generic dumping ground.
- **Files** enables file-level linking for any source that can provide it.

## Sources

| Source | Category | Scope | Auto-detect | Credentials |
|---|---|---|---|---|
| `git` | Signal | Module | Always (.git exists) | None |
| `github` | Signal + Context | Project | git remote is github.com | `github_token` |
| `jira` | Signal | Project | .carto/sources.yaml | `jira_token`, `jira_email` |
| `linear` | Signal | Project | .carto/sources.yaml | `linear_token` |
| `local-pdf` | Knowledge | Project | {project}/docs/*.pdf | None |
| `notion` | Knowledge | Project | .carto/sources.yaml | `notion_token` |
| `slack` | Context | Project | .carto/sources.yaml | `slack_token` |
| `web` | Knowledge | Project | .carto/sources.yaml | None |

### Source Details

**git** — Existing `GitSignalSource` refactored to new interface. Runs `git log` per module. Emits Signal artifacts (type: commit). No behavior change.

**github** — Expanded from current `GitHubSignalSource`:
- Auto-detect from `git remote -v` (fixes local-repo gap).
- Paginate with `Link:` header (configurable max pages, default 3).
- Fetch issues (Signal), PRs (Signal), optionally discussions (Context, opt-in).
- PR artifacts populate `Files` from GitHub Files Changed API for file-level linking.
- Append top 5 comments to issue/PR body (truncated).

**jira** — REST API v3: `GET /rest/api/3/search?jql=project={key} ORDER BY updated DESC`. Basic auth (email:token). File linking via branch name parsing (e.g. `PROJ-123-fix-auth` matches `PROJ-123`).

**linear** — GraphQL API: query issues by team, sorted by updatedAt. File linking via branch name parsing.

**local-pdf** — Existing `LocalPDFSource` refactored. Configurable directory via yaml (not hardcoded). Optional recursive scan.

**notion** — API: `POST /v1/databases/{id}/query` for page list, `GET /v1/blocks/{id}/children` for content. Converts Notion blocks to plain text.

**slack** — API: `conversations.history` + `conversations.replies` for configured channels. Groups messages into threads. Each thread with 3+ messages becomes a Context artifact. Filters: last 30 days, skip bots, skip short threads.

**web** — HTTP GET each configured URL, extract readable content (go-readability or similar). Strips nav/footer/ads. Truncate at 50KB per page. Respects `robots.txt`.

## Configuration

### Auto-detect Logic

Runs before every index:

1. Always register: `git`
2. If git remote contains `github.com` → register `github` (parse owner/repo from remote URL)
3. If `{project}/docs/*.pdf` exists → register `local-pdf`
4. Read `.carto/sources.yaml` → register configured sources with their settings

### .carto/sources.yaml

```yaml
sources:
  jira:
    url: https://mycompany.atlassian.net
    project: PROJ

  linear:
    team: engineering

  notion:
    database_id: abc123

  slack:
    channels:
      - "#engineering"
      - "#architecture"

  web:
    urls:
      - https://docs.example.com/api
      - https://wiki.internal.com/architecture

  # Override auto-detected sources
  github:
    discussions: true

  local-pdf:
    dir: /custom/path/to/pdfs
    recursive: true
```

Credentials live in Settings UI / env vars — never in the yaml.

### Settings UI Credentials

| Setting | Env Var | Sources |
|---|---|---|
| `github_token` | `GITHUB_TOKEN` | github |
| `jira_token` | `JIRA_API_TOKEN` | jira |
| `jira_email` | `JIRA_EMAIL` | jira |
| `linear_token` | `LINEAR_API_KEY` | linear |
| `notion_token` | `NOTION_API_KEY` | notion |
| `slack_token` | `SLACK_BOT_TOKEN` | slack |

## Storage & Retrieval

### Storage Key Patterns

| Category | Key Pattern | Example |
|---|---|---|
| Signal (file-linked) | `carto/{project}/{module}/layer:signals/{source}` | `carto/my-app/root/layer:signals/github` |
| Signal (project-level) | `carto/{project}/_project/layer:signals/{source}` | `carto/my-app/_project/layer:signals/jira` |
| Knowledge | `carto/{project}/_knowledge/{source}/{artifact.ID}` | `carto/my-app/_knowledge/notion/Design-RFC-Auth` |
| Context | `carto/{project}/_context/{source}/{artifact.ID}` | `carto/my-app/_context/slack/thread-C04ABC-1234` |

### Retrieval Tier Inclusion

| Tier | Size | Includes |
|---|---|---|
| Mini (~5KB) | Map + Blueprint | No external sources |
| Standard (~50KB) | + Atoms, Signals, Patterns | + Signal artifacts (all sources, merged per module) |
| Full (~500KB) | + History, Wiring, Zones | + Knowledge + Context artifacts |

Signal artifacts with `Files` populated get stored under the matching module's `layer:signals`. Signals without file links go to `_project/layer:signals/{source}` and appear in Full tier only.

## Pipeline Integration

### Updated Phase Flow

```
Phase 1: Scan (unchanged)
Phase 2: Chunk + Atoms (unchanged)
Phase 3: History + Sources
  ├── 3a: Module-scoped sources (git) — parallel per module (unchanged)
  └── 3b: Project-scoped sources — once, parallel per source (new)
Phase 4: Deep Analysis (unchanged)
Phase 5: Store — extended to route artifacts by category (updated)
```

Phase 3b runs all project-scoped sources concurrently with `sync.WaitGroup`. Errors are logged per-source but don't block other sources.

Phase 5 routes artifacts by category:
- Signal with Files → module's `layer:signals/{source}`
- Signal without Files → `_project/layer:signals/{source}`
- Knowledge → `_knowledge/{source}/{artifact.ID}`
- Context → `_context/{source}/{artifact.ID}`

### Pipeline Config Change

```go
type Config struct {
    // ...existing fields...
    SourceRegistry *sources.Registry  // replaces SignalRegistry + KnowledgeRegistry
}
```

## Package Structure

```
go/internal/sources/
├── source.go          // Source interface, Artifact, Category, Registry, SourceConfig
├── registry.go        // Registry with auto-detect + yaml loading
├── git.go             // GitSource (from signals/git.go)
├── github.go          // GitHubSource (expanded from signals/github.go)
├── jira.go            // JiraSource
├── linear.go          // LinearSource
├── pdf.go             // PDFSource (from knowledge/pdf.go)
├── notion.go          // NotionSource
├── slack.go           // SlackSource
├── web.go             // WebSource
└── *_test.go
```

Old `internal/signals/` and `internal/knowledge/` packages are deprecated and deleted after migration.

## Migration Path

1. Create `internal/sources/` with new interface and `Artifact` type
2. Port `git` and `github` sources (already built, just adapt types)
3. Port `local-pdf` source
4. Update pipeline to use `sources.Registry` (replace both old registries)
5. Fix storage to handle `_knowledge` and `_context` in retrieval tiers
6. Build new sources: `jira`, `linear`, `notion`, `slack`, `web`
7. Add `.carto/sources.yaml` parsing + git remote auto-detect
8. Update Settings UI with all credential fields + connectivity test buttons
9. Delete old `internal/signals/` and `internal/knowledge/` packages

## Non-Goals

- Real-time/webhook-based signal updates (poll on index for now)
- Full Confluence API integration (complex auth, defer)
- Git submodule support
- Multi-branch indexing
- Source-specific UIs beyond the Settings credentials page
- LLM-based summarization of fetched artifacts (store raw, let analysis layers handle it)

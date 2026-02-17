# Carto v2 Design — Intent-Aware Codebase Intelligence

**Project**: Indexer (CLI tool: `carto`)
**Language**: Go (CLI + backend), HTML/CSS/JS (future UI)
**Date**: 2026-02-17

## Problem

Large/legacy codebases with bad naming, no comments, and no uniform patterns are opaque to AI agents. Current tools capture *what* code does but not *why* it was written. Agents need persistent, queryable intent context to plan, debug, and build effectively.

## Solution

Carto scans a codebase end-to-end, extracts every available signal (code, git history, PRs, Jira tickets, docs), and produces a layered context graph stored in FAISS. Each layer answers progressively deeper questions, with **intent** as a first-class concept at every level.

## Layer Model

| Layer | Name | Source | Parallel? | What it captures |
|-------|------|--------|-----------|-----------------|
| 0 | Map | Scanner (no LLM) | N/A | File tree, languages, entry points, module boundaries |
| 1a | Atoms | Haiku (per file) | ✅ goroutines | Clarified code (x→userId) + summary + imports/exports |
| 1b | History | Git CLI (per file) | ✅ parallel w/ 1a | Blame, commit messages, PR titles per file |
| 1c | Signals | Plugins (per module) | ✅ parallel w/ 1a/1b | Jira tickets, Confluence docs, Linear issues, etc. |
| 2 | Wiring | Opus (per module) | ✅ per-module | Cross-unit dependencies + why they're connected |
| 3 | Zones | Opus (per module) | ✅ per-module | Business domains with intent statements |
| 4 | Blueprint | Opus (system-wide) | Single call | System narrative, cross-module interactions, business purpose |
| 5 | Patterns | Opus (system-wide) | Single call | Coding conventions → CLAUDE.md / .cursorrules |

## Pipeline Architecture

```
Phase 0: Scan + Module Detection
    ├── Walk file tree, detect languages
    ├── Find module boundaries (pom.xml, package.json, go.mod, Cargo.toml)
    ├── Build module dependency graph
    └── Output: Module[] with file lists

Phase 1: Parallel Extraction (goroutines)
    ├── 1a: Atoms — per file, worker pool — Haiku clarifies + summarizes
    ├── 1b: History — per file, worker pool — git blame + commit messages
    └── 1c: Signals — per module, plugins — Jira, Confluence, etc.
         (all three run concurrently via goroutine groups)

Phase 2: Module Analysis (parallel per module)
    └── For each module (goroutine):
          Opus receives: atoms + history + signals for THIS module
          Produces: wiring (L2) + zones (L3) + module intent summary

Phase 3: System Synthesis (single Opus call)
    └── Opus receives: all module summaries + cross-module deps
          Produces: blueprint (L4) + patterns (L5)

Phase 4: Store + Generate
    ├── Store all layers to FAISS (tagged: carto/{project}/{module}/layer:N)
    ├── Generate CLAUDE.md / .cursorrules
    └── Save manifest (.carto/manifest.json)
```

### Scale Estimates

| Codebase | Files | Phase 1 | Phase 2 | Phase 3 | Total |
|----------|-------|---------|---------|---------|-------|
| Small (poets-pad) | ~100 | ~2 min | ~2 min | ~2 min | ~6 min |
| Medium (WebChat) | ~20 | ~30 sec | ~1 min | ~1 min | ~3 min |
| Large (Ultron) | ~35K | ~15 min | ~10 min | ~3 min | ~30 min |

## Plugin System (Signals)

```go
type SignalSource interface {
    Name() string
    Configure(cfg map[string]string) error
    FetchSignals(module Module) ([]Signal, error)
}

type Signal struct {
    Type    string // "ticket", "doc", "pr", "comment"
    ID      string // "JIRA-1892", "PR #247"
    Title   string
    Body    string
    URL     string
    Files   []string // linked file paths
    Date    time.Time
    Author  string
}
```

Built-in: `git` (commits, PRs via GitHub API)
Plugins: `jira`, `confluence`, `linear`, `notion`, `slack` (configured in .carto/config.yaml)

## Deobfuscation (Atom Clarification)

Before summarizing, Haiku produces a "clarified" version of each code unit:
- Renames cryptic variables: `x` → `userId`, `cb` → `onComplete`
- Adds inline annotations for complex logic
- Original code stays on disk untouched
- FAISS stores the clarified version for agent consumption

## Tiered Retrieval

Agents can request context at different depths:

| Tier | Content | Size | Use case |
|------|---------|------|----------|
| mini | Zone summaries + blueprint only | ~5KB | Quick orientation |
| standard | + atom summaries + wiring | ~50KB | Feature planning |
| full | + clarified code + history + signals | ~500KB | Deep debugging |

## Go Project Structure

```
indexer/
├── cmd/carto/main.go          # CLI entry point
├── internal/
│   ├── scanner/               # Phase 0: file tree + module detection
│   ├── chunker/               # Tree-sitter based code splitting
│   ├── atoms/                 # Phase 1a: Haiku clarification + summary
│   ├── history/               # Phase 1b: git archaeology
│   ├── signals/               # Phase 1c: plugin interface + built-in git
│   │   ├── source.go          # SignalSource interface
│   │   ├── git.go             # Built-in git/GitHub signal source
│   │   └── jira/              # Jira plugin
│   ├── analyzer/              # Phase 2+3: Opus deep analysis
│   ├── storage/               # FAISS client + layer serialization
│   ├── manifest/              # Index state tracking
│   ├── patterns/              # Pattern extraction + skill generation
│   ├── llm/                   # Anthropic client (OAuth + API key)
│   └── config/                # Config loading (.carto/config.yaml)
├── web/                       # Future: UI dashboard (HTML/CSS/JS)
│   ├── index.html
│   ├── styles.css
│   └── app.js
├── go.mod
├── go.sum
└── docs/plans/
```

## CLI Commands

```bash
# Core
carto index <path>               # Full index
carto index <path> --module core  # Index single module
carto index <path> --incremental  # Only changed files
carto query "how does auth work?" --project myapp
carto query "why does payment retry 3 times?" --tier full

# Output
carto patterns <path>            # Generate CLAUDE.md + .cursorrules
carto status <path>              # Show index status
carto modules <path>             # List detected modules

# Plugins
carto plugins list               # Show available signal sources
carto plugins configure jira     # Set up Jira integration

# Server (for UI)
carto serve                      # Start web UI on :8080
```

## UI Dashboard (Future — HTML/CSS/JS)

- Project overview: modules, zones, file count, last indexed
- Module explorer: drill into zones → atoms → code
- Search: natural language query with tiered results
- Signal timeline: git commits + Jira tickets on a timeline per module
- Plugin management: configure Jira/Confluence/etc.
- Re-index trigger: button to kick off incremental re-index
- Pattern viewer: see discovered conventions, edit before generating CLAUDE.md

## Key Decisions

1. **Go over TypeScript** — Single binary, goroutines for parallelism, no runtime dependency
2. **Tree-sitter over regex** — Accurate AST-based chunking across 100+ languages (via go-tree-sitter)
3. **Module-aware pipeline** — Scales to monorepos by decomposing into independent units
4. **Plugin system for signals** — Extensible without modifying core. Interface-based.
5. **Clarified code storage** — Agents see intelligible code even when source is obfuscated
6. **FAISS for storage** — Leverages existing MCP infrastructure. Source-tagged for project isolation.
7. **Two-tier LLM** — Haiku for volume (atoms), Opus for depth (wiring, zones, blueprint)
8. **Tiered retrieval** — Right amount of context for the task at hand

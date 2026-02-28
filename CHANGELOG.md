# Changelog

All notable changes to this project will be documented in this file.

## [1.0.0] - 2026-02-28

First stable release of Carto — an intent-aware codebase intelligence tool that scans codebases, builds a 7-layer semantic index using LLMs, and stores it in Memories for fast retrieval.

### Added

- **Core indexing pipeline** — 6-phase orchestrator (Scan → Chunk+Atoms → History+Signals → Deep Analysis → Store → Skill Files) with cancellation support
- **Two-tier LLM strategy** — fast tier for high-volume atom summaries, deep tier for cross-component analysis
- **7-layer context graph** — Map, Atoms, History, Signals, Wiring, Zones, Blueprint
- **Multi-provider LLM support** — Anthropic, OpenAI, OpenRouter, and Ollama backends
- **Tree-sitter AST chunking** — language-agnostic code splitting for Go, TypeScript, Python, Java, Rust, and more
- **Tiered retrieval** — mini (~5KB), standard (~50KB), full (~500KB) query tiers
- **Skill file generation** — produces CLAUDE.md and .cursorrules with active index workflow instructions (query before edit, write back after changes)
- **Unified source system** — Git, GitHub, Jira, Linear, Notion, Slack, local PDF, and web sources with concurrent fetching
- **CLI** — `index`, `query`, `modules`, `patterns`, `status`, `serve`, `projects`, `sources`, `config` commands with `--json` support
- **REST API** — full CRUD for projects, sources, config, query, and index trigger with SSE progress streaming
- **Thin SDK** (`pkg/carto`) — programmatic access to Index, Query, and Sources
- **Web UI** — React + Vite + shadcn/ui with Dashboard, Index, Query, Project Detail, and Settings pages
- **Incremental indexing** — SHA-256 manifest tracking for change-only re-indexing
- **Docker support** — multi-stage build with Docker Compose orchestration
- **Per-project source configuration** — customizable source settings per indexed project

### Fixed

- Deep cancellation in pipeline goroutines
- Indexing robustness for large codebases
- OAuth token refresh race conditions
- SSE event naming and late-client buffering
- Responsive mobile layout

### Changed

- Skill files now drive active index usage — agents query Memories before editing and write back after changes
- UI redesigned with dense data tables, icon-only sidebar, and two-column layouts
- Unified Source interface replaces separate Signals and Knowledge registries

---
type: changelog
audience: external
generated: 2026-02-28
hermes-version: 1.0.0
---

# Changelog

All notable changes to Carto are documented here.

This project follows [Semantic Versioning](https://semver.org/) and the [Keep a Changelog](https://keepachangelog.com/) format.

---

## [1.0.0] - 2026-02-28

The first stable release of Carto. You can now scan, index, query, and generate skill files for any codebase with full language support, external integrations, and multiple deployment options.

### Added

- **Codebase indexing with AST-aware analysis.** Carto parses your code using tree-sitter to understand structure at the syntax level, not just text. Supported languages: Go, TypeScript, Python, Java, and Rust.

- **Natural language querying at three detail levels.** Ask questions about your codebase in plain English. Choose `mini` for quick lookups (~5 KB), `standard` for balanced answers (~50 KB), or `full` for deep dives (~500 KB).

- **AI assistant skill file generation.** Carto generates `CLAUDE.md` and `.cursorrules` files that give AI coding assistants rich context about your project's architecture, patterns, and conventions. Generated files include instructions for the AI to query and update the index itself.

- **External source integration.** Pull in context from outside your code: GitHub issues and PRs, Jira tickets, Linear issues, Notion pages, Slack threads, PDFs, and web pages. Your index becomes a complete picture of your project.

- **Command-line interface with 9 commands.** Everything you need from the terminal: `index`, `query`, `modules`, `patterns`, `status`, `serve`, `projects`, `sources`, and `config`. All commands support `--json` for scripting and automation.

- **REST API with real-time progress streaming.** Manage projects, trigger indexing, and run queries over HTTP. The indexing endpoint uses Server-Sent Events (SSE) so you can monitor progress as it happens.

- **Web dashboard for visual project management.** Run `carto serve` to get a browser-based UI for managing your projects, viewing index status, and running queries without the terminal.

- **Incremental indexing.** After the initial scan, Carto tracks file changes by content hash and only re-indexes what changed. Re-indexing a large project after a small change takes seconds, not minutes.

- **Docker deployment.** Run Carto and Memories together with a single `docker compose up -d`. Mount your projects directory and you're ready to go.

- **Per-project source configuration.** Each project can have its own set of external sources. Connect your frontend repo to its GitHub issues while your backend repo pulls from Jira.

- **Multi-provider LLM support.** Use Anthropic (default), OpenAI, or Ollama for all AI-powered analysis. Bring your own models and API keys.

### Fixed

- **Indexing no longer stalls on large codebases.** Deep cancellation throughout the pipeline ensures that all goroutines clean up properly, even when processing thousands of files.

- **Progress updates stream reliably during long-running indexes.** SSE events are now delivered consistently without drops or delays, so you always know where indexing stands.

- **Mobile layout now properly hides non-essential columns.** The web dashboard is usable on smaller screens without horizontal scrolling.

### Changed

- **Skill files now include active index instructions.** Generated `CLAUDE.md` and `.cursorrules` files tell AI assistants how to query the Carto index and write back updates when they make changes. Your AI assistant stays in sync with your codebase.

- **Dashboard uses compact data tables.** The project list switched from a card grid to a data table layout, making it easier to scan and manage many projects at once.

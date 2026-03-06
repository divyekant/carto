---
type: changelog
audience: external
generated: 2026-03-06
hermes-version: 1.0.0
---

# Changelog

All notable changes to Carto are documented here.

This project follows [Semantic Versioning](https://semver.org/) and the [Keep a Changelog](https://keepachangelog.com/) format.

---

## [1.2.0] - 2026-03-06

### Added

- **6 new commands: init, completions, export, import, logs, upgrade.** `carto init` provides an interactive (and `--non-interactive`) configuration wizard. `carto completions` generates shell completion scripts for bash, zsh, fish, and powershell. `carto export` streams index data as NDJSON for backup and migration. `carto import` ingests NDJSON from stdin with `add` or `replace` strategies. `carto logs` queries and tails the structured audit log with filtering by command and result. `carto upgrade` checks GitHub for newer versions and offers to install them.

- **JSON envelope contract for all commands.** Every command now returns structured JSON when piped or when `--json` is passed: `{"ok": true, "data": {...}}` on success and `{"ok": false, "error": "message", "code": "ERROR_CODE"}` on failure. Five error codes map to specific exit codes: `GENERAL_ERROR` (1), `NOT_FOUND` (2), `CONFIG_ERROR` (3), `CONNECTION_ERROR` (4), `AUTH_FAILURE` (5).

- **TTY auto-detection for smart output formatting.** Terminal stdout produces human-readable colored output; piped stdout automatically emits JSON without requiring `--json`. The `--pretty` flag overrides to human-readable even when piped.

- **`--pretty` and `--yes`/`-y` global flags.** `--pretty` forces human-readable output in any context. `--yes` skips all confirmation prompts for automation and agent usage.

- **Shell completion scripts for bash, zsh, fish, and powershell.** Run `carto completions <shell>` to generate completion scripts that enable tab-completion of commands, subcommands, and flags.

### Changed

- **All CLI output now uses the gold brand palette.** Terminal output across all commands uses a consistent branded color scheme.

- **All commands wrapped in JSON envelope when piped.** Previously only `--json` triggered structured output. Now piping any command to another process automatically switches to the JSON envelope format.

- **`projects delete` now requires confirmation.** Deleting a project prompts for yes/no confirmation to prevent accidental data loss. Use `--yes` to skip the prompt in automated environments.

### Fixed

- **4 pre-existing test failures in JSON output tests.** Fixed incorrect assertions in output formatting tests that were checking for the old plain-text format instead of the new envelope structure.

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

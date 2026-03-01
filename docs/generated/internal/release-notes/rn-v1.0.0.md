---
id: rn-v1.0.0
type: release-notes
audience: internal
version: 1.0.0
status: draft
generated: 2026-02-28
commit-range: initial..b3f5ce3
source-tier: carto
hermes-version: 1.0.0
---

# Release Notes: Carto v1.0.0

## Summary

Carto v1.0.0 is the first stable release of the intent-aware codebase intelligence tool. The system scans codebases, builds a 7-layer semantic index using a two-tier LLM strategy, stores the index in Memories for fast retrieval, and generates skill files (CLAUDE.md, .cursorrules) that give AI assistants structured codebase context. This release delivers the complete feature set: indexing pipeline, multi-provider LLM integration, unified external sources, tiered retrieval, CLI, REST API, Web UI, Go SDK, and Docker deployment.

---

## New Features

### 1. Indexing Pipeline

**What:** A 6-phase pipeline that transforms raw source code into a 7-layer semantic context graph. Phases: Scan, Chunk+Atoms, History+Signals, Deep Analysis, Store, Skill Files.

**How:** The `pipeline` package orchestrates all phases sequentially with deep cancellation support. The scanner discovers files (respecting .gitignore), the chunker splits code using Tree-sitter AST, the fast-tier LLM extracts atoms, external sources provide signals, and the deep-tier LLM produces wiring, zones, and a blueprint. Results are stored in Memories and output as skill files.

**Config:**
- `CARTO_MAX_CONCURRENT` -- controls parallel LLM calls during atom extraction (default: 10)

**Who it affects:** All users. This is the core functionality.

**CS Notes:** Indexing duration scales with codebase size and LLM latency. For large codebases (1000+ files), expect 30-60 minutes on first run. Subsequent runs use incremental indexing (manifest-based SHA-256 change detection) and are significantly faster. If a user reports slow indexing, check `CARTO_MAX_CONCURRENT` and LLM provider responsiveness.

---

### 2. LLM Integration

**What:** Multi-provider LLM support with a two-tier strategy (fast tier for volume, deep tier for quality) and OAuth authentication support.

**How:** The `llm` package provides a unified client that routes requests to Anthropic, OpenAI, or Ollama based on configuration. The fast tier handles per-chunk atom extraction; the deep tier handles cross-component analysis.

**Config:**
- `LLM_PROVIDER` -- `anthropic` (default), `openai`, or `ollama`
- `LLM_API_KEY` / `ANTHROPIC_API_KEY` -- API credentials
- `LLM_BASE_URL` -- custom endpoint (required for Ollama, optional for OpenAI-compatible APIs)
- `CARTO_FAST_MODEL` -- fast tier model (default: `claude-haiku-4-5-20251001`)
- `CARTO_DEEP_MODEL` -- deep tier model (default: `claude-opus-4-6`)

**Who it affects:** All users. Provider choice affects cost, speed, and output quality.

**CS Notes:** Users on Ollama get zero API costs but slower throughput and potentially lower-quality analysis. OpenAI users need to set model names to valid OpenAI models (e.g., `gpt-4o-mini` for fast, `gpt-4o` for deep). The OAuth fix in this release addresses race conditions under concurrent requests.

---

### 3. Unified Sources

**What:** A plugin-based system for fetching external context from Git, GitHub, Jira, Linear, Notion, Slack, PDF, and Web sources. All sources implement a unified interface with concurrent fetching.

**How:** The `signals` package manages source plugins. Each plugin implements the Source interface, fetches data from its respective service, and produces signals (layer 1c of the context graph). Sources run concurrently during the History+Signals phase. Failures are non-fatal.

**Config:**
- `GITHUB_TOKEN` -- GitHub API access
- `JIRA_URL`, `JIRA_TOKEN` -- Jira instance
- `LINEAR_TOKEN` -- Linear API access
- `NOTION_TOKEN` -- Notion API access
- `SLACK_TOKEN` -- Slack API access

**Who it affects:** Users who want enriched indexes with external context. Sources are opt-in.

**CS Notes:** This replaces the previous separate Signals and Knowledge interfaces with a unified Source interface. Users only need credentials for the sources they want to use. Per-project source configuration is available via the API and Web UI.

---

### 4. Storage & Retrieval

**What:** Layered storage in Memories with tiered retrieval (mini ~5KB, standard ~50KB, full ~500KB) for flexible context delivery.

**How:** The `storage` package provides a REST client for Memories. Data is stored with source tags that encode project, module, and layer information. Retrieval tiers control how many layers are included in query results. The mini tier returns architecture summaries; the full tier returns the complete 7-layer context graph.

**Config:**
- `MEMORIES_URL` -- Memories server address (default: `http://localhost:8900`)
- `MEMORIES_API_KEY` -- Memories authentication

**Who it affects:** All users. Retrieval tier selection affects query response size and relevance.

**CS Notes:** If a user reports empty query results, verify that: (1) the project has been indexed, (2) the Memories server is running at the configured URL, (3) the API key is correct. Use `carto status` to check.

---

### 5. Skill File Generation

**What:** Automatic generation of CLAUDE.md and .cursorrules files containing structured codebase context and active index-maintenance instructions for AI assistants.

**How:** The `patterns` package templates generate skill files from the blueprint, wiring, zones, modules, and patterns produced during deep analysis. Files are written to the project root with `<!-- BEGIN CARTO INDEX -->` / `<!-- END CARTO INDEX -->` markers. Content outside the markers is preserved on regeneration.

**Config:** No dedicated configuration. Output is driven by analysis results.

**Who it affects:** Users of Claude-based assistants (CLAUDE.md) and Cursor IDE (.cursorrules).

**CS Notes:** Skill files now include active workflow instructions that direct AI assistants to write back to the Carto index when making code changes. This is a key v1.0.0 design decision -- skill files are not passive documentation but active participants in index maintenance.

---

### 6. CLI

**What:** A command-line interface with 9 commands: `index`, `query`, `modules`, `patterns`, `status`, `serve`, `projects`, `sources`, `config`. All commands support `--json` for machine-readable output.

**How:** Entry point is `cmd/carto/`. The CLI wraps the internal packages and provides human-friendly output with optional JSON mode for scripting and integration.

**Config:** CLI reads all environment variables listed in `.env.example`.

**Who it affects:** All users who interact with Carto from the terminal.

**CS Notes:** `carto status` is the first command to run for diagnostics. `carto serve` starts both the REST API and Web UI.

---

### 7. REST API

**What:** HTTP API for project CRUD, source management, configuration, querying, and index triggering. Supports SSE streaming for real-time pipeline progress.

**How:** Implemented in the `server` package. Endpoints: `/api/projects`, `/api/projects/{id}/sources`, `/api/config`, `/api/query`, `/api/index`. Indexing progress is streamed via Server-Sent Events.

**Config:** API server starts with `carto serve`. Port is configurable via CLI flags.

**Who it affects:** Users building integrations, the Web UI (which consumes this API), and SDK users.

**CS Notes:** The API is the same server that hosts the Web UI. SSE event naming was fixed in this release for consistency.

---

### 8. Web UI

**What:** An embedded React single-page application with Dashboard, Index, Query, Project Detail, and Settings views. Built with Vite and shadcn/ui.

**How:** The `server` package embeds the compiled React SPA. The UI communicates with the REST API on the same server. Indexing can be triggered from the UI with live SSE progress. The UI uses dense data tables (redesigned in v1.0.0) for efficient information display.

**Config:** No separate configuration. Runs as part of `carto serve`.

**Who it affects:** Users who prefer a visual interface over the CLI.

**CS Notes:** The responsive mobile layout was fixed in this release. If users report layout issues on mobile, verify they are on v1.0.0.

---

### 9. SDK

**What:** A thin Go SDK at `pkg/carto` exposing `Index()`, `Query()`, and `Sources()` functions for programmatic access to Carto's core capabilities.

**How:** The SDK wraps the REST API client. It is designed for embedding Carto functionality into other Go applications or build tools.

**Config:** SDK consumers pass configuration programmatically.

**Who it affects:** Go developers integrating Carto into their toolchains.

**CS Notes:** The SDK is intentionally thin. For advanced use cases, consumers may need to use the REST API directly.

---

### 10. Docker & Deployment

**What:** Multi-stage Dockerfile and docker-compose configuration for running Carto alongside a Memories instance.

**How:** The Dockerfile handles CGO compilation (required for Tree-sitter) in a build stage and produces a minimal runtime image. The docker-compose file orchestrates both Carto and Memories services, wiring them together via environment variables.

**Config:** All standard environment variables apply. The compose file pre-configures `MEMORIES_URL` to point to the co-located Memories service.

**Who it affects:** Users deploying Carto in containerized environments.

**CS Notes:** Volume mounts are important: mount the target codebase and a persistent volume for the `.carto` directory (to preserve the manifest across container restarts). Without the manifest volume, every restart triggers a full re-index.

---

## Bug Fixes

### Deep cancellation in pipeline goroutines

**What was fixed:** Pipeline phases now properly propagate cancellation signals through all goroutines. Previously, cancelling a pipeline run could leave orphaned goroutines processing LLM calls.

**Impact:** Prevents resource leaks and stale state when stopping an indexing run midway (via the Web UI stop button, CLI interrupt, or API cancellation).

---

### Indexing robustness for large codebases

**What was fixed:** Improved handling of edge cases that occurred when indexing large codebases (1000+ files), including memory management during concurrent atom extraction and graceful handling of oversized chunks.

**Impact:** Large codebase indexing is more reliable and less likely to fail due to resource exhaustion.

---

### OAuth race conditions

**What was fixed:** The LLM client's OAuth token refresh logic had a race condition under concurrent requests, potentially causing authentication failures when multiple goroutines attempted token refresh simultaneously.

**Impact:** Users authenticating via OAuth (particularly in multi-tenant or enterprise configurations) no longer see intermittent auth failures during high-concurrency indexing.

---

### SSE event naming

**What was fixed:** Server-Sent Event names emitted during pipeline progress were inconsistent (mixed casing, inconsistent prefixes). Event names are now standardized.

**Impact:** Web UI and API consumers receive consistently-named events. Any custom SSE consumers built during pre-release testing should update their event name matching.

---

### Responsive mobile layout

**What was fixed:** The Web UI had layout issues on mobile viewports, including overflow and truncation problems in the data tables and navigation.

**Impact:** The Web UI is now usable on mobile devices and small screens.

---

## Configuration Changes

All environment variables are new in v1.0.0 (first release):

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `LLM_PROVIDER` | No | `anthropic` | LLM provider selection |
| `LLM_API_KEY` | Yes* | -- | API key for the configured provider |
| `ANTHROPIC_API_KEY` | Yes* | -- | Anthropic-specific API key (alternative to LLM_API_KEY) |
| `LLM_BASE_URL` | No | -- | Custom LLM endpoint URL |
| `CARTO_FAST_MODEL` | No | `claude-haiku-4-5-20251001` | Fast tier model name |
| `CARTO_DEEP_MODEL` | No | `claude-opus-4-6` | Deep tier model name |
| `CARTO_MAX_CONCURRENT` | No | `10` | Max concurrent LLM calls |
| `MEMORIES_URL` | No | `http://localhost:8900` | Memories server URL |
| `MEMORIES_API_KEY` | No | -- | Memories authentication key |
| `GITHUB_TOKEN` | No | -- | GitHub source access |
| `JIRA_URL` | No | -- | Jira instance URL |
| `JIRA_TOKEN` | No | -- | Jira authentication |
| `LINEAR_TOKEN` | No | -- | Linear API access |
| `NOTION_TOKEN` | No | -- | Notion API access |
| `SLACK_TOKEN` | No | -- | Slack API access |

\* At least one of `LLM_API_KEY` or `ANTHROPIC_API_KEY` is required unless using Ollama.

---

## Known Issues

The following are known rough edges in v1.0.0:

1. **No partial module indexing.** There is no single CLI flag to index only one module within a multi-module project. Workaround: point Carto at the module's subdirectory.

2. **Skill file templates are not customizable.** The patterns package uses internal templates. Users cannot modify the structure or content of generated CLAUDE.md or .cursorrules beyond what the deep analysis produces.

3. **No automatic LLM retry.** If an LLM call fails (rate limit, timeout, transient error), the pipeline does not retry automatically. The run fails and must be restarted. Incremental indexing ensures the retry picks up where it left off.

4. **Ollama model compatibility.** Not all Ollama models produce output quality sufficient for reliable deep-tier analysis. Results may vary depending on the local model used. Recommended: use models with at least 13B parameters for deep tier.

5. **Memories server required.** Carto has no embedded storage mode. A running Memories instance is required for all indexing and querying operations. The docker-compose setup provides one, but standalone CLI users must run Memories separately.

6. **Tree-sitter language coverage.** The set of supported languages depends on which Tree-sitter grammars are compiled into the binary. Adding new languages requires code changes to the chunker package. Unsupported languages fall back to line-based chunking.

---

## Internal Notes

### Deployment

- The docker-compose setup is the recommended deployment path for most environments. It co-locates Carto and Memories, handles networking, and provides sensible defaults.
- For bare-metal or VM deployment, install the `carto` binary, ensure a C compiler is available (CGO requirement), and run a Memories instance separately.
- The Web UI is embedded in the binary -- no separate static file hosting is needed.

### Recommended first-time setup

1. Copy `.env.example` to `.env` and fill in `LLM_API_KEY` (or `ANTHROPIC_API_KEY`).
2. Start Memories: `docker-compose up memories` (or run Memories standalone).
3. Build: `go build -o carto ./cmd/carto`
4. Index a project: `carto index /path/to/codebase`
5. Check status: `carto status`
6. Query: `carto query "How does authentication work?"`
7. Start Web UI: `carto serve`

### Testing considerations

- Run `go test -short ./...` for unit tests without a Memories server.
- Run `go test ./...` for full integration tests (requires running Memories at `MEMORIES_URL`).
- Always run with `-race` before release: `go test -race ./...`
- CGO is required for building and testing (Tree-sitter dependency).

### Architecture decision records

Architecture decisions are tracked in `docs/decisions/`. Key decisions for v1.0.0:
- Two-tier LLM strategy (cost vs quality tradeoff)
- 7-layer context graph design
- Unified Source interface (replacing separate Signals + Knowledge)
- Skill files as active index participants (not passive documentation)
- Embedded Web UI in the Go binary (single-binary distribution)
- Memories as external storage (no embedded vector DB)

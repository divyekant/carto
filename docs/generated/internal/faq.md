---
type: faq
audience: internal
status: draft
generated: 2026-03-06
source-tier: carto
hermes-version: 1.0.0
---

# Carto FAQ

Frequently asked questions organized by topic. Updated for Carto v1.2.0.

---

## Indexing

### How long does indexing take?

It depends on codebase size, the number of files, and LLM response latency. A small project (50-100 files) typically indexes in 2-5 minutes. A large codebase (1000+ files) can take 30-60 minutes or more. The bottleneck is LLM API calls -- the fast tier handles atom extraction in parallel (controlled by `CARTO_MAX_CONCURRENT`, default 10), and the deep tier runs sequentially for cross-component analysis. Incremental re-indexing is significantly faster since only changed files are reprocessed.

### What happens if indexing fails partway through?

The pipeline supports deep cancellation across all 6 phases. If a phase fails, the pipeline stops and reports the error. Work completed in earlier phases (e.g., atoms already stored in Memories) is retained. On the next run, the manifest (SHA-256 hash tracking) detects which files were already processed and skips them, so a retry picks up roughly where it left off. Check `carto status` to see what was indexed.

### Can I index only one module?

Not directly via a single flag. However, the scanner respects project boundaries defined by manifest files (go.mod, package.json, etc.), and per-project source configuration determines what gets indexed. To index a subset, point Carto at the subdirectory containing the module's root, or use the project-scoped configuration via the REST API or Web UI.

### What languages are supported?

Carto uses Tree-sitter for AST-based code chunking. Supported languages depend on the Tree-sitter grammars compiled into the binary. The chunker package handles language detection and parsing. If a file's language is not supported by Tree-sitter, Carto falls back to line-based chunking so the file is still indexed, just without AST-aware boundaries.

### What is incremental vs full indexing?

The manifest package maintains SHA-256 hashes for every indexed file. On subsequent runs, only files whose hash has changed (or new files) are reprocessed through the pipeline. This is incremental indexing. A full re-index can be triggered by deleting the manifest or using the appropriate CLI/API flag, which forces all files through the pipeline regardless of hash state.

### Does indexing respect .gitignore?

Yes. The scanner package honors .gitignore rules when discovering files. Files and directories matched by .gitignore patterns are excluded from indexing.

---

## LLM & Costs

### Which models does Carto use?

Carto employs a two-tier LLM strategy:

- **Fast tier** (`CARTO_FAST_MODEL`, default: `claude-haiku-4-5-20251001`): Used for high-volume atom extraction -- one call per code chunk. Optimized for speed and cost.
- **Deep tier** (`CARTO_DEEP_MODEL`, default: `claude-opus-4-6`): Used for cross-component analysis (wiring, zones, blueprint layers). Fewer calls, higher quality.

### Can I use a different LLM provider?

Yes. Set `LLM_PROVIDER` to `anthropic`, `openai`, or `ollama`. For OpenAI, set `LLM_API_KEY` and optionally `LLM_BASE_URL` for compatible endpoints. For Ollama, set `LLM_BASE_URL` to the Ollama server address. Model names should be updated in `CARTO_FAST_MODEL` and `CARTO_DEEP_MODEL` to match the provider's model catalog.

### How much does indexing cost in API calls?

Rough estimate: the fast tier makes one LLM call per code chunk (a function, class, or logical block), plus the deep tier makes a handful of calls (typically 3-5) for wiring, zone, and blueprint analysis. For a 500-file Go project with ~2000 chunks, expect ~2000 fast-tier calls and ~5 deep-tier calls. Actual token costs depend on the provider's pricing. Using Ollama for local models eliminates API costs entirely.

### What happens when the LLM is unavailable?

The pipeline fails at the phase requiring LLM access (Chunk+Atoms for fast tier, Deep Analysis for deep tier). The error is reported and the pipeline halts. Previously completed phases are preserved. Retry after the LLM service is restored. The system does not queue or retry LLM calls automatically.

### Does Carto support OAuth for LLM providers?

Yes. The LLM client package includes OAuth support. Note that v1.0.0 fixed OAuth race conditions that could occur under concurrent requests.

---

## Sources

### Which external sources are supported?

Carto supports a unified Source interface with the following plugins: Git (built-in), GitHub, Jira, Linear, Notion, Slack, PDF, and Web. Each source fetches external signals that enrich the index with context beyond the code itself -- issue trackers, documentation, design docs, chat threads.

### Do I need all source credentials configured?

No. Sources are opt-in. Only configure credentials for the sources relevant to the project. If a token environment variable (e.g., `GITHUB_TOKEN`, `JIRA_TOKEN`) is not set, that source is simply skipped during the History+Signals phase. Git history is always available if the project is a Git repository.

### What if a source fails during indexing?

Source failures are non-fatal. If a single source plugin fails (e.g., Jira returns an auth error), the pipeline logs the error and continues with the remaining sources. The index is built with whatever signals were successfully collected. Check the pipeline output or `carto status` for source-specific errors.

### Can I configure sources per-project?

Yes. Source configuration is per-project. Use the REST API (`/api/projects/{id}/sources`), the Web UI (Project Detail > Sources tab), or the CLI (`carto sources`) to manage which sources are active for a given project and their credentials/settings.

---

## Querying

### What are the retrieval tiers?

Carto supports three retrieval tiers that control how much context is returned:

- **Mini** (~5KB): Compact summary. Module names, top-level patterns, and key relationships. Good for quick orientation.
- **Standard** (~50KB): Moderate detail. Includes atom summaries, wiring information, and zone descriptions. Suitable for most development tasks.
- **Full** (~500KB): Complete context dump. All 7 layers of the context graph. Used for deep analysis or generating comprehensive skill files.

### How do I search across projects?

Use `carto query --project <name>` to target a specific project's index in Memories. The query command searches the stored layers semantically. Cross-project querying (searching all projects at once) is done by omitting the project filter, though results are scoped by what is stored in the connected Memories instance.

### Why am I getting 0 results?

Common causes:
1. **Project not indexed yet.** Run `carto status` to verify. Run `carto index` if needed.
2. **Memories server not running.** Check that `MEMORIES_URL` points to a live instance (default: `http://localhost:8900`).
3. **Wrong project name.** Use `carto projects` to list indexed projects.
4. **Query too specific.** Memories uses semantic search; try broader or differently-worded queries.

### Can I query from both the CLI and API?

Yes. The CLI command `carto query "your question"` returns results to stdout (use `--json` for structured output). The REST API exposes `POST /api/query` with the same functionality, returning JSON. The Web UI provides an interactive query interface with the same backend. All three use the same storage/retrieval layer.

---

## Skill Files

### What are CLAUDE.md and .cursorrules?

These are skill files generated by Carto after indexing. They contain a structured summary of the codebase -- architecture, modules, patterns, and instructions for keeping the index current. `CLAUDE.md` is consumed by Claude-based AI assistants (Claude Code, etc.). `.cursorrules` is consumed by Cursor IDE. Both serve the same purpose: giving AI assistants codebase-aware context so they produce better code suggestions.

### Will Carto overwrite my existing CLAUDE.md?

The patterns package handles skill file generation. It writes to the project root. If a `CLAUDE.md` or `.cursorrules` file already exists, Carto replaces the section between `<!-- BEGIN CARTO INDEX -->` and `<!-- END CARTO INDEX -->` markers, preserving any content outside those markers. Content you write outside the markers is safe.

### How do I customize the generated content?

The generated skill file content is driven by what the deep tier analysis produces (blueprint, wiring, zones, patterns). To influence the output, adjust the project's source configuration (more sources = richer context) or re-index after significant code changes. Direct template customization is not exposed in v1.0.0 -- the patterns package uses internal templates.

### What does "skill files drive active index usage" mean?

As of v1.0.0, generated skill files include workflow instructions that tell AI assistants how to write back to the Carto index when they make code changes. This creates a feedback loop: the AI reads the skill file for context, makes changes, and writes updated atoms back to Memories. The skill file is not just a snapshot -- it actively directs ongoing index maintenance.

---

## Web UI & API

### How do I start the web UI?

Run `carto serve` to start the built-in web server. The server package hosts an embedded React SPA (built with Vite and shadcn/ui). By default it listens on port 8080 (check `carto serve --help` for port configuration). Open `http://localhost:8080` in a browser. The UI provides Dashboard, Index, Query, Project Detail, and Settings views.

### Can I trigger indexing from the UI?

Yes. The Web UI's Index view provides a button to trigger indexing for a project. This calls the REST API endpoint, which starts the pipeline. Progress is streamed to the UI via Server-Sent Events (SSE).

### What is the SSE progress stream?

During indexing triggered via the REST API, the server emits Server-Sent Events that report pipeline progress in real-time: which phase is running, files being processed, errors encountered, and completion status. The Web UI consumes this stream to show a live progress indicator. Note that v1.0.0 fixed SSE event naming for consistency.

### How do I use the API programmatically?

The REST API supports: project CRUD (`/api/projects`), source management (`/api/projects/{id}/sources`), configuration (`/api/config`), querying (`/api/query`), and index triggering (`/api/index`). All endpoints accept and return JSON. Use `carto serve` to start the API server. The SDK (`pkg/carto`) provides a Go client with `Index()`, `Query()`, and `Sources()` functions that wrap these endpoints.

### What are the available CLI commands?

The CLI exposes 18 commands organized in four groups: core pipeline (`index`, `query`, `modules`, `patterns`, `status`), infrastructure (`serve`, `projects`, `sources`, `config`, `auth`, `doctor`), setup and lifecycle (`init`, `completions`, `version`, `about`, `upgrade`), and data portability (`export`, `import`, `logs`). All commands support the JSON envelope contract with TTY auto-detection.

---

## Docker

### How do I run Carto in Docker?

Carto provides a multi-stage Dockerfile. Build with `docker build -t carto .` and run with appropriate environment variables. The image requires CGO (for Tree-sitter), which the multi-stage build handles. Pass LLM credentials and Memories connection details via environment variables or a `.env` file.

### What about volume mounts?

Mount the target codebase into the container (e.g., `-v /path/to/code:/workspace`). If using the manifest for incremental indexing, mount a persistent volume for the `.carto` directory so hashes survive container restarts.

### Can I use docker-compose?

Yes. The project includes a `docker-compose.yml` that runs both Carto and a Memories instance together. This is the recommended setup for local development or self-hosted deployments. Start with `docker-compose up`. The compose file wires up the `MEMORIES_URL` environment variable automatically so Carto connects to the co-located Memories service.

### Does the Docker setup include the Web UI?

Yes. The Docker image includes the embedded React SPA. When running `carto serve` inside the container (or if the compose file starts the server), the Web UI is accessible on the exposed port.

---

## CLI & Agent Usability

### What is the JSON envelope contract and how does it work?

As of v1.2.0, all CLI commands wrap their output in a standard JSON envelope: `{"ok": true, "data": ...}` for success and `{"ok": false, "error": "...", "code": "..."}` for errors. This matches the Memories CLI convention. Success envelopes go to stdout; error envelopes go to stderr. The `code` field contains a machine-readable error code (`GENERAL_ERROR`, `NOT_FOUND`, `CONFIG_ERROR`, `CONNECTION_ERROR`, `AUTH_FAILURE`) and the process exit code maps to the error category (0-5). Scripts should parse the envelope rather than raw output, and use both the `code` field and exit code for programmatic error handling.

### How does TTY auto-detection work? Do I need to pass `--json` in CI pipelines?

No. When stdout is not a terminal (e.g., piped to another program, redirected to a file, or captured by a subprocess), the CLI automatically emits JSON envelope output without needing `--json`. This means CI pipelines and AI agents get structured JSON by default. Use `--json` only when you explicitly want JSON on a terminal. Use `--pretty` to force human-readable colored output even when piped (e.g., for logging pipelines that want readable text). If both `--json` and `--pretty` are set, `--pretty` wins.

### What is the `--yes` flag and when do I need it?

The `--yes` (or `-y`) global flag skips interactive confirmation prompts. It is required for destructive operations when running in non-interactive contexts (CI, agents, piped output). Without `--yes`, the `confirmAction()` function returns false in JSON/non-TTY mode, silently cancelling the operation. Commands that require confirmation: `projects delete`, `import --strategy replace`, and `upgrade` (when an update is available). Always pass `--yes` when automating destructive operations.

### How do I use the new `carto init` command to set up Carto in a CI pipeline?

Use `--non-interactive` mode with all required flags:
```bash
carto init --non-interactive \
  --llm-provider anthropic \
  --api-key "$ANTHROPIC_KEY" \
  --memories-url "http://memories:8900" \
  --projects-dir "/workspace/projects"
```
The `--api-key` flag is required in non-interactive mode. Other flags are optional and fall back to environment variables or defaults. The command writes the config file and emits an envelope with the config path, provider, and Memories URL.

### How do I back up and transfer a project's index between environments?

Use the `export` and `import` commands:
```bash
# Export from source environment
carto export --project myapp > myapp.ndjson

# Transfer the file to the target environment, then import
cat myapp.ndjson | carto import --project myapp --strategy replace --yes
```
Use `--layer` on export to back up only a specific layer (atoms, wiring, zones, blueprint, patterns). Use `--strategy add` (the default) on import to append without deleting existing data. Use `--strategy replace` to replace the entire index for that project (requires `--yes` in non-interactive contexts).

---
id: fh-001
type: feature-handoff
audience: internal
topic: Indexing Pipeline
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Feature Handoff: Indexing Pipeline

## What It Does

The indexing pipeline is the core workflow that transforms a raw codebase into a structured, queryable semantic index. It walks a directory tree, parses source files into meaningful chunks, produces LLM-generated summaries (atoms), collects contextual signals from external sources, runs deep cross-component analysis, stores everything in Memories, and optionally generates skill files for AI assistants.

The pipeline executes six sequential phases. Each phase depends on the output of the previous one. Scan failure is fatal and aborts the entire run. All other phase failures are non-fatal: the pipeline collects errors in `Result.Errors` and continues with whatever data it has.

## How It Works

### Phase 1: Scan

**Component:** `scanner/`

The scanner walks the target directory tree, respecting `.gitignore` rules and applying built-in exclusions (node_modules, .git, vendor, build artifacts). It detects module boundaries by looking for manifest files:

| Manifest File | Language/Ecosystem |
|---|---|
| `go.mod` | Go |
| `package.json` | JavaScript / TypeScript |
| `Cargo.toml` | Rust |
| `pyproject.toml`, `setup.py` | Python |
| `pom.xml`, `build.gradle` | Java |
| `Gemfile` | Ruby |
| `*.csproj`, `*.sln` | C# / .NET |

Output: a list of discovered files grouped by module, with metadata (path, size, language).

**Fatal behavior:** If the scan phase fails (e.g., directory does not exist, permission denied on the root), the entire pipeline aborts immediately.

### Phase 2: Chunk + Atoms

**Components:** `chunker/`, `atoms/`

**Chunking:** Each source file is parsed using Tree-sitter AST grammars for supported languages (Go, JavaScript, TypeScript, Python, Java, Rust). The chunker splits files into semantic units: functions, methods, structs, classes, interfaces, and top-level declarations. Unsupported languages are chunked as whole files.

Large files are truncated at 49,000 characters before processing.

**Atoms:** Each chunk is sent to the fast-tier LLM (default: Claude Haiku) for structured summarization. The atom output includes:

- `name` -- identifier for the chunk
- `kind` -- function, struct, class, interface, etc.
- `summary` -- natural-language description of what the code does
- `clarified_code` -- cleaned-up version of the code with inline explanations
- `imports` -- what the chunk depends on
- `exports` -- what the chunk exposes to other code

Atoms are produced concurrently using a semaphore (default: 10 concurrent LLM calls, controlled by `CARTO_MAX_CONCURRENT`).

### Phase 3: History + Signals

**Components:** `history/`, `sources/`

**History:** Extracts git log data from the repository: recent commits, file churn (change frequency), and ownership (who changed what most recently).

**Signals:** The unified sources system (see fh-003) fetches external context concurrently. Each configured source (GitHub issues, Jira tickets, Linear issues, Notion pages, Slack messages, PDFs, web pages) runs its `FetchSignals()` method. Source failures are non-fatal and logged.

### Phase 4: Deep Analysis

**Component:** `analyzer/`

The deep-tier LLM (default: Claude Opus) performs two levels of analysis:

1. **Per-module analysis:** For each detected module, the analyzer examines atoms, history, and signals to produce:
   - **Wiring** (Layer 4): How components connect -- API boundaries, dependency flows, data pathways
   - **Zones** (Layer 5): Business domains and functional areas within the module
   - **Intent** (Layer 6 input): What each module is trying to accomplish

2. **System synthesis:** Across all modules, the analyzer produces:
   - **Blueprint** (Layer 6): System-wide architecture description, design rationale, business purpose
   - **Patterns**: Cross-cutting coding conventions, architectural patterns, anti-patterns discovered

### Phase 5: Store

**Component:** `storage/`

All analysis results are written to Memories via its REST API. Each piece of data is tagged with a source identifier: `carto/{project}/{module}/layer:{layer}`. Batch writes are chunked to 500 items per request. Content exceeding 49,000 characters is truncated.

Before storing, the pipeline clears previous data for the project using `delete-by-prefix` to avoid stale entries.

### Phase 6: Skill Files

**Component:** `patterns/`

Optionally generates CLAUDE.md and/or .cursorrules files containing the indexed knowledge. These files use marker-based injection (`<!-- BEGIN CARTO INDEX -->` / `<!-- END CARTO INDEX -->`) to preserve any user-authored content outside the markers.

Controlled by the `SkipSkillFiles` flag in pipeline configuration.

## User-Facing Behavior

### CLI

```
carto index /path/to/project [flags]
```

| Flag | Description |
|---|---|
| `--project` | Project name (default: directory name) |
| `--module` | Index only a specific module |
| `--incremental` | Only process files changed since last index |
| `--full` | Ignore manifest and reindex everything |
| `--format` | Skill file format: `claude`, `cursor`, `all` |

The CLI prints phase-by-phase progress to stderr. On completion, it prints a summary: files scanned, atoms produced, signals collected, analysis completed, storage status.

### API (SSE Progress Streaming)

The web server exposes an SSE endpoint that streams real-time progress events during indexing. Events include phase transitions, per-file status, error reports, and completion summaries.

### Incremental Indexing

When `--incremental` is set, the manifest (`manifest/`) compares SHA-256 hashes of each file against the stored manifest in `.carto/manifest.json`. Only files whose hash has changed (or new files) are reprocessed. Deleted files are cleaned up from storage.

## Configuration

| Variable | Default | Description |
|---|---|---|
| `CARTO_FAST_MODEL` | `claude-haiku-4-5-20251001` | Model for atom extraction (fast tier) |
| `CARTO_DEEP_MODEL` | `claude-opus-4-6` | Model for deep analysis (deep tier) |
| `CARTO_MAX_CONCURRENT` | `10` | Max concurrent LLM calls |
| `CARTO_MAX_WORKERS` | `4` | Max parallel file processing workers |
| `MEMORIES_URL` | `http://localhost:8900` | Memories server URL |
| `MEMORIES_API_KEY` | -- | API key for Memories |
| `ANTHROPIC_API_KEY` | -- | Anthropic API key (or `LLM_API_KEY`) |

## Edge Cases

| Scenario | Behavior |
|---|---|
| Scan phase fails (dir not found, permissions) | Pipeline aborts immediately. Fatal error returned. |
| LLM call fails for some atoms | Those atoms are skipped. Pipeline continues with partial data. Errors collected in `Result.Errors`. |
| Deep analysis times out | Module-level analysis may be incomplete. System synthesis uses whatever data is available. |
| File exceeds 49,000 characters | Truncated before chunking. Atom summary covers only the truncated content. |
| Unsupported language (e.g., Haskell, Elixir) | File is chunked as a single whole-file chunk. Atom still produced. |
| Module detection finds no manifest files | Entire project treated as a single unnamed module. |
| Empty project directory | Scan succeeds with zero files. Pipeline completes with empty results. |
| Manifest file missing (first run) | All files treated as new. Full index performed. Manifest written on completion. |

## Common Questions

**Q1: How long does indexing take?**
Depends on codebase size, LLM response times, and concurrency settings. A typical Go project with 200 files takes 3-8 minutes. The atoms phase (Phase 2) is usually the bottleneck due to per-file LLM calls. Increase `CARTO_MAX_CONCURRENT` to speed it up, at the cost of higher API usage.

**Q2: When should I use `--incremental` vs `--full`?**
Use `--incremental` for routine re-indexing after code changes. It only reprocesses files whose content hash has changed. Use `--full` when the index seems stale, after upgrading Carto, or when switching LLM models (since different models produce different atoms).

**Q3: What happens if the LLM provider is down mid-indexing?**
The scan phase completes (no LLM needed). Atoms phase will fail for all files -- those errors are collected but the pipeline continues. Deep analysis will also fail. The store phase writes whatever partial data exists. The result will contain a large number of errors, and the index will be incomplete. Re-run once the provider is back.

**Q4: Which languages does the AST-based chunker support?**
Go, JavaScript, TypeScript, Python, Java, and Rust have Tree-sitter grammars. All other languages fall back to whole-file chunking.

**Q5: How does module detection work?**
The scanner looks for manifest files (go.mod, package.json, Cargo.toml, etc.) in the directory tree. Each manifest file defines a module boundary. Nested modules are supported (e.g., a Go workspace with multiple go.mod files). If no manifest is found, the entire project is one module.

## Troubleshooting

| Symptom | Likely Cause | Resolution |
|---|---|---|
| "Error scanning directory" or pipeline aborts at Phase 1 | Directory does not exist, insufficient permissions, or broken symlinks | Verify the path exists and is readable. Check for permission issues. Resolve broken symlinks. |
| Atoms phase is very slow (>30 min for <100 files) | Low concurrency, slow LLM provider, or rate limiting | Increase `CARTO_MAX_CONCURRENT`. Check LLM provider status. Verify API key has sufficient rate limits. |
| "deep analysis timed out" | Deep-tier model is slow or overloaded | Check Anthropic API status. Try a different deep model. Reduce project size by using `--module` to index one module at a time. |
| Manifest appears stale (changes not detected) | File hashes match despite content changes (unlikely), or manifest.json was manually edited | Delete `.carto/manifest.json` and run with `--full`. |
| "0 atoms produced" | LLM API key invalid, provider unreachable, or all files unsupported | Verify `ANTHROPIC_API_KEY` or `LLM_API_KEY` is set and valid. Test connectivity to the LLM provider. Check that the project contains source code files. |
| Store phase fails | Memories server unreachable or returning errors | Verify `MEMORIES_URL` is correct and the server is running. Check Memories server logs. |

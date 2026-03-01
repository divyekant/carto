---
id: uc-001
type: use-case
audience: internal
topic: Full Codebase Indexing
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Use Case: Full Codebase Indexing

## Trigger

User runs:

```
carto index /path/to/project
```

Or submits an index request through the web API.

## Preconditions

1. `ANTHROPIC_API_KEY` or `LLM_API_KEY` is set to a valid API key.
2. Memories server is running and reachable at `MEMORIES_URL` (default: `http://localhost:8900`).
3. The target directory exists and contains source code files.
4. (Optional) External source credentials are configured if external signals are desired (GITHUB_TOKEN, JIRA_TOKEN, etc.).

## Primary Flow

### Step 1: Scan (Phase 1)

The scanner walks the target directory, applies `.gitignore` rules and built-in exclusions, discovers all source files, and detects module boundaries via manifest files (go.mod, package.json, Cargo.toml, etc.).

**Output:** File list grouped by module with metadata (path, size, detected language).

**Failure mode:** Fatal. If the scan fails, the pipeline aborts and returns an error.

### Step 2: Chunk + Atoms (Phase 2)

Each discovered file is parsed into semantic chunks using Tree-sitter AST grammars (for supported languages) or treated as a whole file (for unsupported languages). Each chunk is sent to the fast-tier LLM for atom extraction.

**Output:** Structured atom per chunk (name, kind, summary, clarified_code, imports, exports).

**Failure mode:** Non-fatal. Failed atoms are skipped and errors collected. The pipeline continues with partial data.

### Step 3: History + Signals (Phase 3)

Git history is extracted from the repository (commits, churn, ownership). Configured external sources (GitHub, Jira, Linear, Notion, Slack, PDF, Web) run concurrently to fetch additional context signals.

**Output:** History records and signal entries from each source.

**Failure mode:** Non-fatal. Individual source failures are logged and skipped.

### Step 4: Deep Analysis (Phase 4)

The deep-tier LLM analyzes each module using the collected atoms, history, and signals. Per-module analysis produces wiring diagrams, zone classifications, and intent summaries. System-level synthesis produces a blueprint and cross-cutting patterns.

**Output:** ModuleAnalysis (per module) and SystemSynthesis (project-wide).

**Failure mode:** Non-fatal. Partial analysis is used if some modules fail.

### Step 5: Store (Phase 5)

All results are written to Memories via the REST API, tagged with source identifiers (`carto/{project}/{module}/layer:{layer}`). Previous data for the project is cleared first via `delete-by-prefix`. Batch writes are chunked to 500 items.

**Output:** Data persisted in Memories. Manifest file written to `.carto/manifest.json`.

**Failure mode:** Non-fatal. Store failures are logged. The manifest is still written so incremental indexing can function.

### Step 6: Skill Files (Phase 6)

CLAUDE.md and/or .cursorrules files are generated from the synthesis results and written to the project root. Existing user content outside Carto markers is preserved.

**Output:** Skill files written to disk.

**Failure mode:** Non-fatal. Generation failures are logged.

## Variations

### Incremental Indexing

**Trigger:** `carto index /path/to/project --incremental`

The manifest compares SHA-256 hashes of each file against `.carto/manifest.json`. Only files with changed hashes are sent through Phase 2 (Chunk + Atoms). Phases 3-6 run normally using a mix of new and previously stored data.

### Single Module Indexing

**Trigger:** `carto index /path/to/project --module mymodule`

Only the specified module is processed. The scan phase still runs on the full directory to detect module boundaries, but only the matching module proceeds through Phases 2-6.

### Full Re-index

**Trigger:** `carto index /path/to/project --full`

The manifest is ignored entirely. All files are treated as new and processed through all phases. Previous data in Memories is cleared and replaced.

## Edge Cases

| Scenario | Behavior |
|---|---|
| No manifest file exists (first run) | All files are treated as new. Full indexing is performed. `.carto/manifest.json` is created on completion. |
| Empty project directory | Scan succeeds with zero files. Pipeline completes with empty results. No data stored. |
| All LLM calls fail | Scan and history succeed. Zero atoms produced, zero analysis completed. Errors collected in `Result.Errors`. History and signals (if any) are still stored. |
| Project was previously indexed with a different model | Old data is cleared during Phase 5. New atoms and analysis reflect the current model. Use `--full` to ensure a clean re-index. |
| Concurrent index requests for the same project | Not explicitly prevented. Both runs will attempt to clear and write data to Memories, potentially causing inconsistencies. Avoid running concurrent indexes for the same project. |

## Data Impact

**Written:**

| Location | Content |
|---|---|
| Memories: `carto/{project}/{module}/layer:atoms` | Atom summaries for each code chunk |
| Memories: `carto/{project}/{module}/layer:history` | Git commit history, churn, ownership |
| Memories: `carto/{project}/{module}/layer:signals` | External source data (issues, docs, messages) |
| Memories: `carto/{project}/{module}/layer:wiring` | Component connectivity and dependency flows |
| Memories: `carto/{project}/{module}/layer:zones` | Business domain and functional area classifications |
| Memories: `carto/{project}/layer:blueprint` | System-wide architecture and design rationale |
| Memories: `carto/{project}/layer:patterns` | Cross-cutting coding patterns and conventions |
| Disk: `.carto/manifest.json` | SHA-256 hashes of all indexed files |
| Disk: `CLAUDE.md` (optional) | Skill file for Claude-based assistants |
| Disk: `.cursorrules` (optional) | Skill file for Cursor IDE |

**Deleted:**

Previous Memories entries with source prefix `carto/{project}/` are deleted before new data is written (Phase 5).

## Post-Conditions

1. Memories contains a complete (or partial, on failure) semantic index of the codebase.
2. `.carto/manifest.json` reflects the current state of indexed files.
3. Skill files are written to the project root (unless `SkipSkillFiles` is set).
4. `Result.Errors` contains any non-fatal errors encountered during the run.

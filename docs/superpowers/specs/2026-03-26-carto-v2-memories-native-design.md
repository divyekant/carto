# Carto v2: Memories-Native Storage & Write-Back

**Date:** 2026-03-26
**Status:** Draft
**Approach:** Pipeline-Native Graph (Approach B)

## Problem Statement

Carto was built against Memories v1.0. It uses 8 of 83 available endpoints, stores atoms as unstructured text blobs, uses delete-and-replace on re-indexing (losing all history), and doesn't leverage graph links, metadata, confidence, feedback, or temporal features. This leads to:

1. **Stale indexes** — After code changes, the index is wrong until a full re-index (minutes, dollars). No incremental update path.
2. **Flat retrieval** — Only 2 of 6 ranking signals used (vector + BM25). Cross-component queries miss multi-hop relationships.
3. **No structured filtering** — Atom data (name, kind, module, language) embedded in text strings, not queryable fields.
4. **No version history** — Delete-and-replace destroys all previous atom versions, making confidence decay and temporal queries impossible.

## Target Codebase

**Ultron** — CleverTap's monorepo. 41K files, 106 Maven modules, 16K Java classes, 354K commits, 300+ contributors. Deep cross-module wiring (LC -> LP -> ES -> NB -> Delivery pipeline). If Carto v2 works on Ultron, it works on anything.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Write-back trigger | Agent-triggered via CLI | Cheapest path; agent already knows what changed. Automation can wrap CLI. |
| Graph link depth | Hybrid (module + high-signal function) | Module-level links are cheap; function-level only where analyzer surfaces them. Avoids link explosion on large codebases. |
| Atom metadata fields | name, kind, filepath, module, language | Serves agent retrieval. No derived data (churn, lines) — agents get those from git/code. |
| Writeback granularity | File-level | Agent says "I changed this file." Carto handles chunking, LLM analysis, superseding. Correct by construction. |
| Backward compatibility | Breaking change | Clean break. Old indexes require full re-index with new pipeline. No dual-format support needed. |

## Prerequisites

Before implementation, these struct changes are required:

1. **Add `Language` field to `atoms.Atom`** — Currently carried on the input `Chunk` but discarded during atom construction (`atoms/analyzer.go:104-116`). Thread `Chunk.Language` through to the `Atom` struct.
2. **Thread module name into atom analysis** — Module name is only known at the pipeline level. Either add `Module` to the `Chunk` struct, or have the pipeline set it on atoms after `AnalyzeBatchCtx` returns.
3. **Bump manifest version** — `manifest.Version` goes from `"1.0"` to `"2.0"`. All commands check version on load; if `"1.0"` is found, print "Run `carto index` to upgrade to v2 format" and exit.

## Architecture

### 1. Storage Layer — Memories v5 Client

`internal/storage/memories.go` upgraded to speak Memories v5 natively.

#### New Methods

| Method | Signature | Memories Endpoint | Purpose |
|--------|-----------|------------------|---------|
| `UpsertBatch` | `(memories []Memory) ([]UpsertResult, error)` | `POST /memory/upsert-batch` | Store atoms with metadata, return IDs |
| `Supersede` | `(oldID int, newText string, newMeta map[string]any) (newID int, err error)` | `POST /memory/supersede` | Replace atom, archive old, preserve links |
| `CreateLink` | `(fromID, toID int, linkType string) error` | `POST /memory/{id}/link` | Create graph edge between atoms |
| `DeleteLinks` | `(id int) error` | `DELETE /memory/{id}/link/{target}` | Remove stale links before re-wiring |
| `GetLinks` | `(id int) ([]Link, error)` | `GET /memory/{id}/links` | Retrieve edges for an atom |
| `SearchAdvanced` | `(opts SearchOptions) ([]SearchResult, error)` | `POST /search` | Full 6-signal search with all weight params |

```go
// UpsertResult is returned per-item from UpsertBatch.
type UpsertResult struct {
    ID     int    `json:"id"`
    Status string `json:"status"` // "created" or "updated"
}

// Link represents a graph edge between two memories.
type Link struct {
    ToID      int    `json:"to_id"`
    Type      string `json:"type"`
    CreatedAt string `json:"created_at"`
}
```

**Precondition:** Memories v5 `POST /memory/upsert-batch` returns per-item IDs in its response. This has been verified against the Memories API — each item in the response array includes `id` and `status`.

#### Removed Methods

| Method | Replaced By |
|--------|------------|
| `StoreBatch()` (add-batch) | `UpsertBatch()` |
| `Search()` (basic) | `SearchAdvanced()` |

#### Kept Methods

| Method | Notes |
|--------|-------|
| `DeleteMemory(id)` | Used in writeback to remove atoms for deleted functions/classes |
| `DeleteBySource(prefix)` | Used in full (non-incremental) re-index to clear a module before storing. Retained because per-atom supersede on a full re-index of Ultron (16K atoms) would mean 16K sequential HTTP calls — too slow. Full re-index continues to use delete-and-replace; only writeback and incremental use supersede. |
| `ListBySource(source, limit, offset)` | Used to enumerate existing atoms by filepath for writeback matching |
| `HealthCheck()` | Unchanged |
| `Count(prefix)` | Unchanged |

#### Memory Payload — New Format

```go
type Memory struct {
    Text       string         `json:"text"`                    // summary + clarified code
    Source     string         `json:"source"`                  // carto/{project}/{module}/layer:atoms
    Metadata   map[string]any `json:"metadata"`                // structured fields
    DocumentAt string         `json:"document_at,omitempty"`   // git commit date (ISO 8601)
}
```

Metadata fields on every atom:

```json
{
    "name": "TranslationManager",
    "kind": "class",
    "filepath": "core/nb/src/.../TranslationManager.java",
    "module": "nb",
    "language": "java"
}
```

#### Search — Full 6-Signal Passthrough

```go
type SearchOptions struct {
    Query            string
    K                int
    Hybrid           bool
    SourcePrefix     string
    Threshold        float64
    ConfidenceWeight float64  // new
    FeedbackWeight   float64  // new
    GraphWeight      float64  // new
    Since            string   // new (ISO 8601)
    Until            string   // new (ISO 8601)
}
```

The 3x over-fetch + client-side source filtering is removed. Memories does server-side `source_prefix` filtering natively via Qdrant payload index. **Precondition:** Memories v5 `source_prefix` filtering is reliable and does not require client-side fallback. If edge cases are discovered during integration testing, re-add a filtered fallback at that point.

#### SearchResult — Extended Response

```go
type SearchResult struct {
    ID           int            `json:"id"`
    Text         string         `json:"text"`
    Score        float64        `json:"score"`
    Source       string         `json:"source"`
    Metadata     map[string]any `json:"metadata,omitempty"`
    MatchType    string         `json:"match_type"`    // "direct" or "graph"
    Confidence   float64        `json:"confidence"`
    GraphSupport float64        `json:"graph_support"`
}
```

#### Atom Lookup by Filepath

Writeback needs to find existing atoms for a specific file. This uses `ListBySource` with the module's atom source prefix, then client-side filtering by the `filepath` metadata field. This is efficient because atom source tags scope to the module level (`carto/{project}/{module}/layer:atoms`), keeping the result set small (typically <500 atoms per module even on Ultron).

### 2. Pipeline Changes

The 6-phase pipeline stays the same sequence. Phases 2, 4, and 5 change how they handle data.

#### Phase 2: Chunk + Atoms — Now Returns IDs

- Chunks files, sends to fast-tier LLM, stores atoms via `UpsertBatch()` with metadata
- `document_at` is NOT set in Phase 2 (git history isn't available until Phase 3). Instead, Phase 5 patches `document_at` onto atoms after Phase 3 completes, using the per-file last-commit dates from `history.ExtractBulkHistory`. Alternative: use `os.Stat` mtime in Phase 2 as a cheaper approximation — but git date is more accurate.
- Returns an atom ID map keyed by `{module}:{filepath}:{name}:{kind}` (composite key avoids collisions from overloaded Java methods or multiple `init` functions in Go files)
- Text field contains only summary + clarified code (structured data moves to metadata)

#### Phase 4: Deep Analysis — Outputs Edges

Zones and blueprint output unchanged (narrative text, not graph-structured).

Wiring output changes from a JSON blob to a list of typed edges:

```go
type WiringEdge struct {
    FromAtom   string  // e.g., "TranslationManager"
    ToAtom     string  // e.g., "EmailDelivery"
    FromModule string  // e.g., "nb"
    ToModule   string  // e.g., "delivery-email"
    LinkType   string  // "related_to" | "blocked_by" | "caused_by"
    Reason     string  // "NB dispatches email rendering via EmailDelivery.send()"
}
```

The analyzer LLM prompt changes to request edges instead of a monolithic wiring blob. The current prompt at `analyzer/deep.go:132-137` asks for `"wiring": [{"from", "to", "reason"}]`. The new prompt requests:

```json
{
  "wiring": [
    {
      "from_atom": "<name>",
      "from_module": "<module>",
      "to_atom": "<name>",
      "to_module": "<module>",
      "link_type": "related_to|blocked_by|caused_by",
      "reason": "<why connected>"
    }
  ]
}
```

Module-level edges are always created. Function-level edges only where the analyzer identifies high-signal cross-boundary calls. **Hard cap: 50 edges per module** to prevent link explosion on large codebases. If the LLM returns more, truncate to the 50 with the most specific `reason` fields.

**Link type semantics for Carto:**
- `related_to` — Cross-component dependency (import, call, data flow). Default for most wiring.
- `blocked_by` — Module cannot function without this dependency (hard coupling).
- `caused_by` — Data flow direction (B is caused by A producing data).
- `supersedes` and `reinforces` — Managed by Memories internally (supersede creates `supersedes` links automatically). Carto does not create these directly.

#### Phase 5: Store — Creates Links

- **Atoms**: already stored in Phase 2 (via `UpsertBatch`)
- **document_at**: patched onto atoms using git dates from Phase 3 history extraction
- **Wiring**: resolves edge atom names to memory IDs using Phase 2 map, calls `CreateLink(fromID, toID, linkType)` for each edge. **Error handling:** if an atom ID is not found in the map (name mismatch between analyzer output and stored atoms), log a warning and skip that edge. Continue creating remaining links. This matches the existing pipeline pattern of collecting errors without failing the run.
- **Zones, blueprint, patterns, history, signals**: stored as text memories, unchanged

#### Full Re-index (non-incremental)

Full re-index retains `DeleteBySource(modulePrefix)` to clear old data before storing. Per-atom supersede on 16K atoms would be 16K sequential HTTP round trips — impractical at Ultron scale. Full re-index is a clean-slate operation: delete module, store all atoms fresh, create all links fresh.

#### Incremental Re-indexing (`--incremental`)

- Manifest diff identifies changed files
- Changed files: re-chunk, re-analyze atoms, `Supersede()` old atoms with new ones (returns new IDs)
- Removed files: delete atoms by filepath metadata match via `DeleteMemory(id)`
- Removed modules (all files gone): `DeleteBySource(modulePrefix)` to clean up atoms, wiring, zones, blueprint for that module
- Unchanged files: no-op (confidence reinforced by search access)
- After atom updates: re-run Phase 4 for affected modules only, delete old links for those modules, create new links
- Phase 6 (skill files) runs as normal

#### Supersede Throughput at Scale

Incremental re-indexing uses `Supersede` per changed atom. At ~100ms per HTTP call, 500 changed atoms = 50 seconds of API calls. Mitigations:
1. **Parallelism:** Run supersede calls concurrently (bounded by `CARTO_MAX_CONCURRENT`, default 10). 500 atoms / 10 workers = ~5 seconds.
2. **Batch scope:** Incremental only processes changed files (manifest diff), not all files. Typical incremental run touches 1-20 files, not 500.
3. **Future:** If Memories adds a `supersede-batch` endpoint, switch to that.

### 3. Write-Back System

New CLI command that solves the stale-index problem.

#### Command Signature

```
carto writeback <path> [flags]

Flags:
  --file <filepath>    Re-index a single file (repeatable)
  --module <name>      Re-index an entire module
  --project <name>     Override project name (default: auto-detect)
  --json               Machine-readable output
  --quiet              No spinners/progress
  --verbose            Show per-atom detail
```

#### Usage Patterns

```bash
# Agent changed one file
carto writeback /path/to/project --file src/auth/handler.go

# Agent changed multiple files
carto writeback /path/to/project --file src/auth/handler.go --file src/auth/token.go

# Re-index a whole module after significant changes
carto writeback /path/to/project --module auth

# No flags — auto-detect changed files via manifest diff
carto writeback /path/to/project
```

#### Writeback Flow (--file)

1. Load manifest, get old SHA-256 for file
2. Compute new SHA-256 — if unchanged, skip (exit 0)
3. Tree-sitter chunk the file, extract atoms via fast-tier LLM
4. Find existing atoms for this filepath (search by metadata `filepath` field + source prefix)
5. Match new atoms to old atoms by `name` + `kind` + `filepath`:
   - Match found: `Supersede(oldID, newText, newMetadata)`
   - No match (new function/class): `UpsertBatch([newAtom])`
6. Old atoms with no new match (deleted function/class): `Delete(oldID)`
7. If atoms changed: re-run wiring analysis for affected module(s), update links
8. Update manifest with new SHA-256
9. Output summary: `3 atoms superseded, 1 added, 1 removed, 4 links updated`

#### Cost Profile

Single file writeback on Ultron:
- 1-5 Haiku calls (one per chunk) — ~$0.001-0.005
- 0-1 deep-tier calls if wiring changed — ~$0.01-0.05
- Total: under $0.06 per file, typically under $0.01
- Time: 2-5 seconds

Compare to full re-index: thousands of Haiku + dozens of Opus calls. Minutes, dollars.

### 4. CLI Changes

#### `carto query` — Advanced Search Params

```
carto query "what handles authentication?" --project ultron [flags]

New flags:
  --graph-weight <float>       Graph traversal weight (default 0.1)
  --confidence-weight <float>  Confidence decay weight (default 0)
  --feedback-weight <float>    Feedback signal weight (default 0.1)
  --since <date>               Filter atoms after date
  --until <date>               Filter atoms before date
```

JSON output gains new fields per result:

```json
{
  "id": 12345,
  "text": "...",
  "score": 0.87,
  "source": "carto/ultron/nb/layer:atoms",
  "match_type": "graph",
  "confidence": 0.92,
  "graph_support": 0.15,
  "metadata": {
    "name": "TranslationManager",
    "kind": "class",
    "module": "nb",
    "filepath": "core/nb/src/.../TranslationManager.java",
    "language": "java"
  }
}
```

TTY output shows `[graph]` tag next to graph-discovered results.

#### `carto status` — Index Health

New output fields:

```
Project: ultron
Modules: 106
Atoms: 16,234
Links: 892 (748 related_to, 112 blocked_by, 32 caused_by)
Oldest atom: 2026-01-15
Newest atom: 2026-03-26
Avg confidence: 0.78
```

**Note:** `carto status` currently reads only the local manifest file (works offline). The new fields (atoms, links, confidence) require querying Memories. If Memories is unreachable, show manifest-only data with a warning: "Memories unavailable — showing local manifest only." The manifest still provides: modules, file count, last indexed timestamp.

#### `carto export` / `carto import` — Metadata-Aware

Export includes metadata and link data in NDJSON. Each line is one of two types:

```jsonl
{"type": "atom", "id": 123, "text": "...", "source": "...", "metadata": {...}, "document_at": "..."}
{"type": "link", "from_id": 123, "to_id": 456, "link_type": "related_to"}
```

Import uses two passes: first upsert all atom lines to get new IDs (building an `oldID -> newID` map), then create links using the remapped IDs. Atom lines must appear before link lines in the NDJSON file.

**Breaking change:** The current export format has no `type` field. v2 export/import is not compatible with v1 exports. Old exports must be re-exported with the v2 CLI.

#### `carto index` — Updated Pipeline

No new flags. Internal behavior changes to use `UpsertBatch`, create graph links, set `document_at`. `--incremental` uses `Supersede` instead of delete-and-replace.

### 5. Patterns Generator

Generated CLAUDE.md instructions change from manual `memory_add` to CLI-driven writeback:

**Before:**
```markdown
### After Changes: Write Back
memory_add({ text: "handleAuth (function) in src/auth/handler.go:15-42\nSummary: ...", source: "carto/project/module/layer:atoms" })
```

**After:**
```markdown
### After Changes: Write Back
After editing files, update the Carto index:
carto writeback /path/to/project --file <changed-file>
```

Simpler for agents. No atom format to get right. Correct by construction.

### 6. Server & Web UI

#### Server Handler Changes (`handleQuery`)

- Accept optional `confidence_weight`, `feedback_weight`, `graph_weight`, `since`, `until` in query request body. These are additive fields — the existing `text`, `project`, `tier`, `k` fields are unchanged. Old API consumers that don't send the new fields get default values (0 for weights, empty for dates).
- Pass through to `SearchAdvanced()`
- Remove 3x over-fetch hack
- Remove `ListBySource` fallback (rely on Memories server-side `source_prefix` filtering)

#### Web UI Query Page

- Collapsible "Advanced Filters" section below search bar (collapsed by default)
- Three sliders: Graph weight, Confidence weight, Feedback weight (0-1 range)
- Two date pickers: Since / Until
- `QueryResult` component enhanced:
  - `[graph]` badge for graph-discovered results (`match_type === "graph"`)
  - Confidence score indicator
  - Metadata tags (module, language, kind) under results

No other UI changes. Dashboard, Settings, IndexRun, ProjectDetail unchanged.

## Testing Strategy

### Unit Tests

- `storage/memories_test.go` — New methods (`UpsertBatch`, `Supersede`, `CreateLink`, `SearchAdvanced`) against mock HTTP server
- `pipeline/pipeline_test.go` — Phase 2 returns ID map, Phase 5 creates links
- `analyzer/analyzer_test.go` — Edge output format parsing
- `cmd/carto/cmd_writeback_test.go` — Writeback flow with mock storage

### Integration Tests

- Full pipeline against real Memories — verify atoms have metadata, links exist, graph search works
- Writeback: index file, modify, writeback, verify supersede + metadata + links
- Incremental: index project, change one file, `--incremental`, verify only affected atoms/links change

### Ultron Validation

- Index Ultron with new pipeline
- Verify ~500-1000 cross-module links for 106 modules
- Query "what handles email delivery?" — verify graph traversal finds NB -> delivery-email -> cloud-services chain
- Query with `--since` — verify temporal filtering
- Writeback single Ultron file — verify <$0.06, <5 seconds

## Out of Scope

- Watch mode / git hooks (CLI-only writeback)
- Feedback collection UI (no "was this helpful?" button yet)
- MCP server (agents use CLI)
- Query-time LLM synthesis (still returns raw results)
- Custom LLM prompts (no user-customizable prompts)
- Multi-backend routing (single Memories instance)
- Lifecycle policies (no auto-archive TTL configuration)

## Known Debt

- **Archived atom accumulation:** `Supersede` creates archived atoms that accumulate forever. Without lifecycle policies (out of scope), the Memories store grows unbounded. Expected timeline: address in a follow-up when lifecycle policies are built. For now, archived atoms are harmless — they don't appear in search results unless `include_archived=true` is passed.
- **Web UI advanced filters:** The Query page changes (sliders, date pickers, badges) are additive and could ship as a follow-up PR if the core storage + writeback + CLI work runs long. They are included in this spec because the server-side changes naturally expose them.
- **Interface and mock updates:** The `MemoriesAPI` interface in `storage/store.go` gains new methods (`UpsertBatch`, `Supersede`, `CreateLink`, `DeleteLinks`, `GetLinks`, `SearchAdvanced`) and `DeleteMemory` (currently exists on `MemoriesClient` but is not in the interface). `Search` is replaced by `SearchAdvanced`. The `SearchResult.Meta` field is renamed to `Metadata` for clarity. All mock implementations in test files (at least 3) need updating. This is mechanical but worth noting in implementation scope.
- **`document_at` via git vs mtime:** Phase 2 cannot set `document_at` from git (history comes in Phase 3). Current design patches it in Phase 5. The patch uses Memories' `PATCH /memory/{id}` endpoint to update the `document_at` field without creating a new version (not `Supersede`, which would create unnecessary archives). If Memories lacks a patch endpoint for `document_at`, fall back to storing `document_at` as a metadata field (`metadata.document_at`) that is set during Phase 2 using `os.Stat` mtime as an approximation. The Memories `document_at` query parameter reads from the top-level field, so the metadata fallback would not support `since`/`until` filtering — this would be a degraded-but-functional mode. `document_at` also round-trips through export/import as a top-level field in the NDJSON atom line.

## Change Surface

| Area | Changes |
|------|---------|
| `internal/storage/` | New Memories v5 client (upsert, supersede, links, advanced search) |
| `internal/pipeline/` | Phase 2 returns ID map, Phase 5 creates links |
| `internal/analyzer/` | Edge output format + updated prompt |
| `internal/atoms/` | Metadata fields on atom payloads |
| `internal/patterns/` | Skill file instructions use `carto writeback` |
| `cmd/carto/` | New `writeback` command, updated `query`/`status`/`export`/`import` |
| `internal/server/` | Query handler accepts advanced params |
| `go/web/` | Query page advanced section + result enhancements |
| Tests | New unit + integration tests across all changed packages |

## Expected Impact

| Area | Improvement |
|------|------------|
| Cross-component queries | +15-25% (graph traversal finds multi-hop relationships) |
| Index freshness | Step change (stale-index problem eliminated) |
| Temporal queries | New capability (impossible today, trivial with document_at) |
| Single-atom lookups | ~Same (already works with vector + BM25) |
| Pipeline speed | Marginal (LLM calls dominate, not storage I/O) |
| Write-back cost | <$0.06 per file, <5 seconds |

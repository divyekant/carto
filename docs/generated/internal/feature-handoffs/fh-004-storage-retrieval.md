---
id: fh-004
type: feature-handoff
audience: internal
topic: Storage & Retrieval
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Feature Handoff: Storage & Retrieval

## What It Does

The `storage/` package manages all interaction with the Memories server -- a REST API service that provides persistent, searchable storage for the semantic index. The package handles writing indexed data (atoms, history, signals, analysis results) during pipeline Phase 5 and retrieving it for queries, skill file generation, and AI assistant context.

The storage layer implements two key patterns: layered storage (data organized by semantic layer) and tiered retrieval (data returned at different levels of detail depending on the use case).

## How It Works

### Memories REST Client

`MemoriesClient` is an HTTP client that communicates with the Memories server. All data is stored as memory entries, each with:

- **text**: The content (JSON-serialized analysis data, summaries, etc.)
- **source**: A structured tag identifying the project, module, and layer

The client wraps the Memories REST API endpoints:

| Endpoint | Method | Purpose |
|---|---|---|
| `/memory/add` | POST | Add a single memory entry |
| `/memory/add-batch` | POST | Add multiple entries in one request |
| `/search` | POST | Search by text query with optional filters |
| `/memories` | GET | List memories with optional source prefix filter |
| `/memories/count` | GET | Count memories by source prefix |
| `/memory/delete-by-prefix` | DELETE | Delete all memories matching a source prefix |
| `/memory/{id}` | DELETE | Delete a specific memory by ID |

### Layered Storage

Each piece of indexed data is tagged with a source string that encodes the project, module, and semantic layer:

```
carto/{project}/{module}/layer:{layer}
```

The seven layers are:

| Layer | Source Tag | Content |
|---|---|---|
| Map | `layer:map` | File list, module structure |
| Atoms | `layer:atoms` | Per-chunk summaries (name, kind, summary, imports, exports) |
| History | `layer:history` | Git commits, churn, ownership |
| Signals | `layer:signals` | External source data (issues, docs, messages) |
| Wiring | `layer:wiring` | Component connectivity, dependency flows |
| Zones | `layer:zones` | Business domains, functional areas |
| Blueprint | `layer:blueprint` | System architecture, design rationale |

An additional tag `layer:patterns` stores cross-cutting coding patterns discovered during synthesis.

### Source Tag Scoping

Source tags enable project-scoped operations:

- **Delete before write:** `delete-by-prefix` with `carto/{project}/` clears all previous data for a project before storing new results.
- **Project-scoped queries:** `source_prefix` filtering retrieves data for a specific project without cross-project contamination.
- **Module-scoped queries:** Filtering by `carto/{project}/{module}/` retrieves data for a single module.

### Tiered Retrieval

`RetrieveByTier()` returns different amounts of data based on the requested tier:

| Tier | Layers Included | Approximate Size | Use Case |
|---|---|---|---|
| Mini | Zones + Blueprint | ~5 KB | Quick project overview, skill file headers |
| Standard | Mini + Atoms + Wiring | ~50 KB | Code assistance, navigation, context for AI tools |
| Full | Standard + History + Signals | ~500 KB | Deep analysis, comprehensive understanding, debugging |

The tier system balances context quality against token/bandwidth costs. AI assistants typically use the standard tier. Full tier is reserved for deep investigation tasks.

### Batch Writes

During Phase 5, the pipeline stores potentially thousands of entries (one per atom, plus history, signals, and analysis results). The client chunks batch writes into groups of 500 entries per request. The Memories server further chunks these internally to 100 per database transaction.

### Content Truncation

Content exceeding 49,000 characters is truncated before storage. This prevents extremely large files from overwhelming the storage backend and the retrieval context window. Truncation is applied at the Carto client level before sending to Memories.

### Search

The `Search()` method uses the Memories hybrid search (BM25 + vector similarity). It accepts:

- A text query
- Optional `source_prefix` for project/module scoping
- Number of results (`k`)
- Hybrid mode flag (BM25 + vector, recommended)

This enables free-form queries across the indexed codebase, such as "how does authentication work" or "database connection pooling".

## Configuration

| Variable | Default | Description |
|---|---|---|
| `MEMORIES_URL` | `http://localhost:8900` | Base URL of the Memories server |
| `MEMORIES_API_KEY` | -- | API key for authenticating with Memories |

### Memories Server Requirements

The Memories server must be running and accessible before Carto can store or retrieve data. The server is an external dependency -- Carto does not start or manage it.

Health check: `GET {MEMORIES_URL}/health`

## Edge Cases

| Scenario | Behavior |
|---|---|
| Memories server is down during Phase 5 | Store phase fails. Errors collected in `Result.Errors`. Pipeline completes but data is not persisted. Manifest is still written locally. |
| Memories server is down during query | Query returns an error. No fallback. |
| Content exceeds 49,000 characters | Truncated to 49,000 characters before storage. Atom summary covers only truncated content. |
| Batch write partially fails | Entries in the failed batch are lost. The client does not retry individual entries. Errors logged. |
| Very large project (thousands of files) | Produces thousands of atoms. Batch writes handle this, but total storage time may be significant. |
| `delete-by-prefix` fails before new data is written | Old data remains. New data may be written alongside old data, causing duplicates. Re-index with `--full` to resolve. |
| Source prefix collision between projects | Occurs if two projects have the same name. Use distinct `--project` names to avoid. |
| Memories server returns 413 (payload too large) | Batch is too large. The 500-item chunking should prevent this, but if the server has a lower limit, reduce batch size. |

## Common Questions

**Q1: Can I use a remote Memories server?**
Yes. Set `MEMORIES_URL` to the remote server's URL (e.g., `https://memories.example.com`). Ensure network connectivity and that `MEMORIES_API_KEY` is valid for the remote instance.

**Q2: How much storage does a typical project use?**
A 200-file Go project produces roughly 200-300 memory entries totaling 1-5 MB of text content. Storage grows linearly with codebase size. The full tier retrieval for such a project is approximately 500 KB.

**Q3: How do I clear indexed data for a project?**
Use the Memories API directly: `DELETE {MEMORIES_URL}/memory/delete-by-prefix?prefix=carto/{project}/`. Carto also does this automatically during Phase 5 before writing new data.

**Q4: What happens if I query a project that was never indexed?**
`RetrieveByTier()` returns zero results. The query completes without error but with empty content. The CLI indicates that no data is available for the project.

**Q5: Can I query across multiple projects?**
Yes. The `Search()` method without a `source_prefix` filter searches across all stored data. This enables cross-project queries like "how is logging handled across all services."

## Troubleshooting

| Symptom | Likely Cause | Resolution |
|---|---|---|
| "connection refused" to Memories | Server is not running | Start the Memories server. Verify `MEMORIES_URL` is correct. |
| "0 results" for a known indexed project | Source prefix mismatch or data was cleared | Verify the project name used during indexing matches the query. Check if data exists: `GET {MEMORIES_URL}/memories/count?source=carto/{project}/` |
| Stale data returned (old analysis) | Re-indexing failed at Phase 5, leaving old data | Re-index the project with `--full`. Verify Memories server is healthy during indexing. |
| Slow queries | Large number of stored memories, or server under load | Check Memories server performance. The hybrid search is more expensive than keyword-only. Consider reducing `k` (number of results). |
| "payload too large" errors during store | Batch exceeds server limits | Should not occur with default 500-item chunking. Check if Memories server has custom size limits. |
| Authentication errors to Memories | `MEMORIES_API_KEY` is wrong or missing | Verify the key. Test directly: `curl -H "X-API-Key: $MEMORIES_API_KEY" $MEMORIES_URL/memories/count` |

---
id: uc-004
type: use-case
audience: internal
topic: Querying Indexed Codebase
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Use Case: Querying Indexed Codebase

## Trigger

User runs a query against an indexed project:

```bash
carto query "how does authentication work" --project myapp
```

Or submits a query through the web API.

## Preconditions

1. The target project has been indexed (at least one successful `carto index` run).
2. Memories server is running and reachable at `MEMORIES_URL`.
3. `MEMORIES_API_KEY` is set (if required by the Memories server).

## Primary Flow: Project-Scoped Tiered Query

### Step 1: Parse Query

The system parses the user's query string and options:
- Query text: the natural-language question
- Project: the `--project` flag (or inferred from the current directory)
- Tier: the `--tier` flag (default: `standard`)

### Step 2: Select Retrieval Method

For project-scoped queries with a specified tier, the system uses `RetrieveByTier()`:

- Constructs the source prefix: `carto/{project}/`
- Selects the layers to retrieve based on the tier:
  - **Mini:** zones + blueprint
  - **Standard:** zones + blueprint + atoms + wiring
  - **Full:** zones + blueprint + atoms + wiring + history + signals

### Step 3: Fetch Data from Memories

The client makes requests to the Memories API:
- `GET /memories?source=carto/{project}/layer:{layer}` for each layer in the selected tier.
- Results are aggregated into a structured response.

### Step 4: Format and Return Results

The retrieved data is formatted for the output target:
- **CLI:** Printed as structured text (architecture summary, module descriptions, relevant code context).
- **API:** Returned as JSON.
- **Skill file context:** Injected into the skill file template.

## Variation: Free-Form Cross-Project Search

### Trigger

```bash
carto query "database connection pooling"
```

No `--project` flag. The query searches across all indexed projects.

### Flow Differences

- Step 2: The system uses `Search()` instead of `RetrieveByTier()`.
- The search is a hybrid BM25 + vector query against all stored memories.
- No source prefix filter is applied.
- Results are ranked by relevance score.
- The response includes the source tag for each result, indicating which project and module it belongs to.

### Step 3: Search Request

```
POST /search
{
  "text": "database connection pooling",
  "k": 20,
  "hybrid": true
}
```

### Step 4: Format Results

Results are grouped by project and module. Each result includes the matched content, its source tag, and a relevance score.

## Variation: Module-Scoped Query

### Trigger

```bash
carto query "error handling" --project myapp --module api
```

### Flow Differences

- Step 2: Source prefix is scoped to `carto/{project}/{module}/`.
- Only data from the specified module is retrieved.
- Useful for large projects where a full-project query returns too much context.

## Edge Cases

| Scenario | Behavior |
|---|---|
| Project has not been indexed | `RetrieveByTier()` returns zero results. The CLI reports that no data is available for the project. |
| Memories server is unreachable | Query fails with a connection error. No fallback mechanism. |
| Query returns zero results (project is indexed) | The query text may not match any stored content. Try broader terms or use `--tier full` for more data. |
| Very large project with full tier | Full-tier retrieval may return ~500 KB of text. This is by design for comprehensive context. The response may be slow to transmit. |
| Project name mismatch | If the project was indexed as "myapp" but queried as "my-app", zero results are returned. Project names are exact-match. |
| Stale data | If the codebase changed since the last index, query results reflect the old state. Re-index to update. |
| Multiple projects with overlapping names | Source prefix filtering is exact. `carto/app/` will not match `carto/app-v2/`. Each project has its own namespace. |

## Data Impact

**Read-only.** Queries do not modify any stored data. No entries are created, updated, or deleted during a query operation.

## Post-Conditions

1. The query results are returned to the user (CLI output or API response).
2. No data is modified in Memories.
3. If the query was made via the API, the response includes structured JSON with layer-grouped content.

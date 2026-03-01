---
id: feat-004
type: feature-doc
audience: external
topic: Querying & Retrieval
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Querying & Retrieval

Once your codebase is indexed, you can search it using natural language. Ask questions like "How does authentication work?" or "Where are database migrations handled?" and Carto returns relevant code context, architectural insights, and related external signals.

Carto uses a tiered retrieval system that gives you the right amount of context for your task -- from a quick overview to a deep architectural analysis.

## How to Use It

```bash
carto query "How does the payment flow work?"
```

That's it. Carto searches your indexed project and returns the most relevant context.

## Retrieval Tiers

Carto offers three retrieval tiers so you get the right amount of context for what you're doing:

| Tier | Size | Best For |
|------|------|----------|
| **mini** | ~5 KB | Quick overviews, "what is this?" questions, orientation |
| **standard** | ~50 KB | Coding tasks, understanding a feature, making changes |
| **full** | ~500 KB | Deep analysis, architecture reviews, large refactors |

The default tier is **standard**, which works well for most coding tasks.

## Configuration

| Flag | Description | Default |
|------|-------------|---------|
| `--project <name>` | Scope the query to a specific project | all projects |
| `--tier <mini\|standard\|full>` | Choose the retrieval tier | `standard` |
| `-k <count>` | Number of results to return | `10` |

## Examples

**Simple query:**

```bash
carto query "How does user authentication work?"
```

Returns standard-tier results across all indexed projects.

**Project-scoped query:**

```bash
carto query "Where are API routes defined?" --project my-backend
```

Limits results to a specific project. Useful when you have multiple codebases indexed.

**Quick overview with mini tier:**

```bash
carto query "What does this project do?" --tier mini
```

Returns a compact ~5 KB overview -- great for getting oriented in an unfamiliar codebase.

**Deep analysis with full tier:**

```bash
carto query "How do all the services communicate?" --tier full
```

Returns up to ~500 KB of context including cross-module wiring, architectural zones, and the project blueprint. Use this when you need the complete picture.

**Adjust result count:**

```bash
carto query "error handling patterns" -k 20
```

**Query through the web UI:**

```bash
carto serve
```

Open `http://localhost:8950` and use the search interface. The web UI provides a visual way to browse results with syntax highlighting and source references.

## What Results Look Like

Query results include context from multiple layers of the index:

- **Code atoms:** Relevant functions, types, and exports with summaries
- **Wiring:** How the matched code connects to other parts of the system
- **History:** Recent changes and who works on this code
- **Signals:** Related tickets, PRs, or docs from external sources (if configured)

The tier you choose determines how much of each layer is included.

## Limitations

- **Requires an indexed project.** You need to run `carto index` before you can query. If you haven't indexed yet, Carto will let you know.
- **Memories server must be running.** Queries are served from the Memories server at `http://localhost:8900` (or your configured URL). Make sure it's running before querying.
- **Result quality depends on index quality.** The better your index (with external sources configured and deep analysis completed), the better your query results will be.

## Related

- [Indexing Pipeline](feat-001-indexing-pipeline.md) -- build the index that powers queries
- [External Sources](feat-003-unified-sources.md) -- enrich query results with external context
- [Skill File Generation](feat-005-skill-file-generation.md) -- generate persistent context files from the index

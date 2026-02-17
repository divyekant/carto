# Codex - Codebase Intelligence System

## Problem

Large, legacy, or monolithic codebases are hostile to AI agents. They can't hold enough context, the code lacks documentation, naming is inconsistent, and dependencies are tangled. Agents choke. Humans struggle to onboard. Feature work and debugging become painful.

## Solution

A tool that pre-digests any codebase into a layered, semantic context graph stored in a vector database. Both humans and AI agents query it for the right context at the right depth. A pattern enforcement skill ensures new code stays consistent, reducing re-indexing.

## Consumers

- **AI agents** (Claude Code, Cursor, Copilot, etc.) - query for task-relevant context via MCP
- **Human developers** - browse for understanding, onboarding, architecture review

## Design Principles

- Language-agnostic (works on any codebase)
- LLM-powered analysis (Opus for deep understanding, Haiku for bulk work)
- Layered context (right depth for every query)
- Start small, design for large (10K LOC now, 1M+ LOC later)
- Incremental updates (don't re-index the world on every change)
- Closed loop (enforce patterns so new code stays indexed)

---

## Architecture

### Tech Stack

- **Runtime:** Node.js / TypeScript
- **CLI:** `codex index ./my-project`
- **LLM:** Claude API (Haiku for Layer 1 bulk, Opus for Layers 2-4)
- **Storage:** FAISS vector database (via existing MCP memory infrastructure)
- **Context windows:** Leverage 1M token inputs for holistic analysis

### Pipeline

```
Codebase --> Scanner --> Layered Analyzers --> Context Chunks --> FAISS
                                                                   ^
                                                     Queryable by agents
                                                     and humans via MCP
```

Phases run sequentially, each feeding the next:

1. **Scan** - Walk file tree, detect languages, gather raw structure
2. **Chunk** - Break files into logical units (functions, classes, blocks)
3. **Analyze** - LLM enriches each chunk with summaries and metadata
4. **Connect** - LLM infers cross-file relationships and dependencies
5. **Cluster** - LLM groups related chunks into domains/bounded contexts
6. **Narrate** - LLM generates system-level architecture overview

Each phase produces artifacts that feed the next. Phases can be re-run independently.

---

## The Five Layers

### Layer 0 - Structure (Static, No LLM)

Pure file system analysis. Produces a map of the codebase.

**What it captures:**
- File tree with language detection (extension + shebang)
- Directory role inference (heuristic: `src/`, `test/`, `docs/`, `config/`)
- File size, last modified, gitignore-aware filtering
- Package manifest detection (`package.json`, `requirements.txt`, `go.mod`, `Cargo.toml`)
- Entry point detection (main files, index files, app files)

**Storage:** One FAISS entry per directory. Metadata: `layer:0`, `type:structure`, `path:...`

### Layer 1 - Units (LLM: Haiku)

Each file broken into logical units. Chunking strategy:
- Split on top-level declarations (functions, classes, interfaces, type defs)
- Files with no clear declarations (scripts, config) are treated as one unit
- Each unit summarized by Haiku

**Context chunk schema:**
```json
{
  "id": "auth/middleware.ts::validateToken",
  "layer": 1,
  "path": "src/auth/middleware.ts",
  "lines": [24, 58],
  "language": "typescript",
  "kind": "function",
  "name": "validateToken",
  "summary": "Validates JWT token from Authorization header...",
  "exports": ["validateToken"],
  "imports": ["jsonwebtoken", "../config"],
  "raw_code": "..."
}
```

### Layer 2 - Relationships (LLM: Opus)

Cross-references between units. Two sources:
- **Static:** Import/export analysis, file references, shared types
- **LLM-inferred:** Opus analyzes large sections of the codebase in single passes (leveraging 1M context) to identify dependencies and roles

**Edge schema:**
```json
{
  "layer": 2,
  "type": "calls | imports | implements | extends | configures",
  "from": "auth/middleware.ts::validateToken",
  "to": "auth/jwt.ts::verifyJWT",
  "description": "validateToken delegates JWT verification to verifyJWT"
}
```

### Layer 3 - Domains (LLM: Opus)

Logical bounded contexts inferred by Opus from all Layer 1 summaries + Layer 2 edges.

**Domain schema:**
```json
{
  "layer": 3,
  "domain": "Authentication System",
  "description": "Handles user login, JWT token management, sessions...",
  "units": ["auth/middleware.ts::*", "auth/jwt.ts::*", "models/user.ts::User"],
  "entry_points": ["auth/middleware.ts::validateToken"],
  "data_flow": "Request -> validateToken -> verifyJWT -> User model lookup",
  "concerns": ["Token expiry not handled consistently across endpoints"]
}
```

A single unit can belong to multiple domains.

### Layer 4 - System (LLM: Opus)

Single high-level narrative of the entire system:
- What the application does
- Architectural patterns (MVC, microservices, event-driven, etc.)
- How domains interact
- Main entry points and request flows
- Tech stack summary
- Known risks and concerns

Stored as one or a few FAISS entries tagged `layer:4`.

### Large Context Window Optimization

For small-to-medium codebases (fits in 1M tokens): Layers 2+3+4 can collapse into a **single Opus call**. Feed all Layer 1 summaries (and raw code for critical files) and produce relationships, domains, and system narrative in one pass. This gives Opus full visibility for holistic understanding.

For large codebases exceeding context limits: Fall back to batched approach with much larger batches per call (entire domain's worth of code per call rather than file-by-file).

---

## Query & Retrieval

Queries go through FAISS MCP tools with metadata filtering and expansion.

**Example: "Fix the authentication bug where tokens expire silently"**

1. Semantic search hits Layer 3 "Authentication System" -> domain overview
2. Also hits Layer 1 units: `validateToken`, `verifyJWT`, token expiry logic
3. Agent receives focused context: domain overview + relevant functions + relationships

**Features:**
- **Query expansion:** Getting a Layer 1 unit auto-includes its Layer 2 relationships and Layer 3 domain
- **Metadata filtering:** Filter by layer, path, domain, language
- **Depth control:** "Give me Layer 3 only" for high-level, "Layer 1 in src/auth/" for deep dive

---

## Incremental Updates

Full re-index on every change is wasteful.

1. **Git-aware diffing:** On re-index, `git diff` since last indexed commit
2. **Changed files -> re-run Layer 1** for those files only
3. **Significant Layer 1 changes -> re-run Layer 2** for affected relationships
4. **Layers 3+4 re-run less frequently:** On explicit request or when enough changes accumulate

**Index manifest:**
```json
{
  "last_commit": "abc123",
  "last_full_index": "2026-02-17T10:00:00Z",
  "file_hashes": { "src/auth/middleware.ts": "sha256:..." },
  "layer_timestamps": {
    "0": "2026-02-17T10:00:00Z",
    "1": "2026-02-17T10:01:00Z",
    "2": "2026-02-17T10:02:00Z",
    "3": "2026-02-17T10:03:00Z",
    "4": "2026-02-17T10:03:30Z"
  }
}
```

**LLM response caching:** Raw responses cached by content hash. Unchanged functions reuse their Layer 1 summaries without API calls.

---

## Pattern Enforcement Skill

After indexing, Codex knows the codebase's conventions. It exports this knowledge as an agent skill so new code follows existing patterns.

### Pattern Extraction (During Indexing)

Opus identifies patterns while building Layers 2-4:
- **Naming conventions:** camelCase functions, PascalCase classes, kebab-case files
- **File organization:** "controllers in `src/controllers/`, one per resource"
- **Architectural rules:** "all DB access through repository classes, never direct queries"
- **Import conventions:** "absolute imports from `@/`, relative only within same domain"
- **Error handling:** "API endpoints wrap in try/catch, return standardized error shape"
- **Testing patterns:** "one test file per module, co-located in `__tests__/`"
- **Domain boundaries:** "Auth domain should not import from Payment domain directly"

### Skill Generation

Codex generates agent instruction files from extracted patterns:
- `CLAUDE.md` for Claude Code
- `.cursorrules` for Cursor
- Generic JSON/YAML for any tool
- All generated from the same extracted pattern data

### Living Rules

When Codex re-indexes and detects pattern drift (new code violating conventions), it:
1. Flags violations
2. Optionally updates the skill file
3. Reports drift in the re-index summary

### Closed Loop

```
Index -> Understand Patterns -> Generate Skill -> Agent Writes Code
  ^                                                      |
  |                                                      v
  +---- Code follows patterns, stays pre-indexed --------+
```

---

## Cost Estimates

| Scale | Layer 0 | Layer 1 (Haiku) | Layers 2-4 (Opus) | Total |
|-------|---------|-----------------|---------------------|-------|
| 10K LOC | Free | ~500 calls, ~$0.50 | 1-3 calls, ~$5-15 | ~$6-16 |
| 100K LOC | Free | ~5000 calls, ~$5 | 5-15 calls, ~$25-75 | ~$30-80 |
| 1M+ LOC | Free | ~50K calls, ~$50 | 50-100 calls, ~$250-500 | ~$300-550 |

Incremental updates cost a fraction (only changed files re-analyzed).

---

## Future Considerations (Not in v1)

- **Watch mode:** Filesystem watcher for real-time incremental updates
- **Multi-repo support:** Index related repos together, understand cross-repo dependencies
- **Visual explorer:** Web UI for browsing the context graph
- **PR review context:** Auto-attach relevant domain context to pull request reviews
- **Ticket/issue integration:** Link indexed domains to project management tickets

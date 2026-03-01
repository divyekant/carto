# Use Case: CLI Workflow

**ID:** uc-006
**Topic:** CLI Workflow
**Trigger:** A developer installs Carto and uses the CLI to index, query, and generate skill files for a codebase.

---

## Primary Flow

### 1. Build the Binary

```bash
# CGO is required for tree-sitter parsing
go build -o carto ./cmd/carto
```

The binary is a single self-contained executable. Move it to a directory in `$PATH` for convenience.

### 2. Configure Environment

Set the required environment variables:

```bash
export LLM_API_KEY="sk-ant-..."
export MEMORIES_URL="http://localhost:8900"
```

Optional variables (`LLM_PROVIDER`, `LLM_MODEL`, `LLM_FAST_MODEL`, `LLM_DEEP_MODEL`) can be set to customize the LLM backend. See `.env.example` for the full list.

Verify configuration:

```bash
carto config
```

### 3. Index a Codebase

```bash
carto index --project myapp
```

The first run performs a full scan. Subsequent runs use `--incremental` (the default) to re-index only changed files based on SHA-256 manifest comparison.

### 4. Query the Index

```bash
carto query "How does authentication work?" --project myapp
```

Adjust retrieval depth with `--tier`:
- `mini` (~5KB) for quick lookups
- `standard` (~50KB) for typical questions
- `full` (~500KB) for deep exploration

### 5. Generate Skill Files

```bash
carto patterns --project myapp --format all
```

This writes `CLAUDE.md` and `.cursorrules` files into the project directory, populated with architecture, module, and pattern information from the index.

---

## Variation: CI/CD Integration with `--json`

In a CI pipeline, the CLI operates non-interactively using `--json` for structured output.

```yaml
# Example GitHub Actions step
- name: Index codebase
  run: |
    carto index --project ${{ github.repository }} --json > index-result.json

- name: Check index status
  run: |
    carto status --project ${{ github.repository }} --json | jq '.last_indexed'
```

The `--json` flag ensures all output is machine-parseable NDJSON. Errors are emitted as JSON objects with an `"error"` field and non-zero exit codes.

---

## Variation: Incremental Indexing Workflow

For ongoing development, the incremental workflow avoids re-indexing unchanged files.

```bash
# First index: full scan
carto index --project myapp --full

# After code changes: incremental (default)
carto index --project myapp

# Index only a specific module
carto index --project myapp --module auth

# Index only projects with changes
carto index --all --changed
```

The manifest (SHA-256 hashes per file) persists in Memories. The `--full` flag forces a clean re-index when needed (e.g., after a major refactor or Carto version upgrade).

---

## Variation: Multi-Project Management

When working across multiple codebases:

```bash
# List all registered projects
carto projects

# Index all projects
carto index --all

# Index only projects with file changes
carto index --all --changed

# Query a specific project
carto query "What patterns does the API use?" --project backend-api

# Generate skill files for all projects
carto patterns --project backend-api --format claude
carto patterns --project frontend-app --format cursor
```

---

## Postconditions

- The 7-layer semantic index is stored in Memories and available for queries.
- Skill files (`CLAUDE.md`, `.cursorrules`) are written to the project directory.
- The manifest is updated to reflect the current file state, enabling future incremental runs.
- Status can be checked at any time with `carto status --project <name>`.

# CLI Reference

Carto ships a single binary with nine commands that cover every stage of the workflow — from indexing a codebase to querying the semantic graph to generating skill files. Every command supports a `--json` flag so you can script Carto into CI/CD pipelines or feed its output to other tools.

## Building the Binary

Carto uses tree-sitter for code parsing, which requires CGO. Make sure you have Go 1.25+ and a C compiler available, then build:

```bash
go build -o carto ./cmd/carto
```

Move the resulting `carto` binary somewhere on your `$PATH` (for example, `~/bin/` or `/usr/local/bin/`).

## Commands at a Glance

| Command | What It Does |
|---------|-------------|
| `index` | Scan a codebase and build (or update) the semantic index |
| `query` | Search the index with a natural-language question |
| `modules` | List the modules Carto discovered in a project |
| `patterns` | Generate skill files (CLAUDE.md, .cursorrules) from the index |
| `status` | Show the current indexing state of a project |
| `serve` | Start the web UI and REST API server |
| `projects` | List, add, or remove projects |
| `sources` | View or edit the external signal sources for a project |
| `config` | View or update global Carto configuration |

## Core Workflow

The most common sequence is **index, query, patterns**. Here is a start-to-finish example:

### 1. Index a Codebase

```bash
carto index --path /path/to/your/repo --name my-project
```

Carto scans the directory, chunks the code with tree-sitter, runs LLM analysis, and stores the results in Memories. On subsequent runs it performs incremental indexing — only changed files are re-processed.

### 2. Query the Index

```bash
carto query --project my-project "How does authentication work?"
```

You get back a natural-language answer grounded in the indexed codebase context.

### 3. Generate Skill Files

```bash
carto patterns --project my-project
```

This writes a `CLAUDE.md` and/or `.cursorrules` file into the project root so that AI assistants automatically pick up the codebase context.

## Command Details

### `carto index`

Indexes a codebase or updates an existing index.

```bash
carto index --path /repos/backend --name backend-api
```

Key flags:

| Flag | Description |
|------|-------------|
| `--path` | Path to the codebase directory |
| `--name` | Project name (used as the key in Memories) |
| `--full` | Force a full re-index, ignoring the incremental manifest |
| `--json` | Output progress and results as JSON |

### `carto query`

Search the semantic index with a question.

```bash
carto query --project backend-api "Where are database migrations defined?"
```

Key flags:

| Flag | Description |
|------|-------------|
| `--project` | Project to query |
| `--tier` | Retrieval tier: `mini`, `standard`, or `full` (default: `standard`) |
| `--json` | Return the result as JSON |

### `carto modules`

List all modules discovered during indexing.

```bash
carto modules --project backend-api
```

### `carto patterns`

Generate skill files from the current index.

```bash
carto patterns --project backend-api
```

Key flags:

| Flag | Description |
|------|-------------|
| `--project` | Project to generate patterns for |
| `--output` | Output directory (defaults to the project root) |
| `--json` | Output the generated content as JSON instead of writing files |

### `carto status`

Check the indexing state of a project — when it was last indexed, how many files were processed, and whether an index run is currently in progress.

```bash
carto status --project backend-api
```

### `carto serve`

Start the web UI and REST API server.

```bash
carto serve --port 8950
```

Key flags:

| Flag | Description |
|------|-------------|
| `--port` | HTTP port (default: `8950`) |
| `--projects-dir` | Base directory for discovering projects |

### `carto projects`

Manage the list of known projects.

```bash
# List all projects
carto projects list

# Add a project
carto projects add --name my-app --path /repos/my-app

# Remove a project
carto projects remove --name my-app
```

### `carto sources`

View or update external signal sources (CI, issue trackers, docs) for a project.

```bash
# View current sources
carto sources --project backend-api

# Update sources
carto sources --project backend-api --set ci=https://github.com/org/repo/actions
```

### `carto config`

View or update global Carto configuration (LLM provider, API keys, Memories URL).

```bash
# View current config
carto config

# Set a value
carto config --set llm_provider=anthropic
```

## Using `--json` in CI/CD

Every command accepts `--json` for machine-readable output. This makes it straightforward to integrate Carto into automated pipelines:

```bash
# Index and capture the result
RESULT=$(carto index --path . --name my-project --json)

# Check status programmatically
STATUS=$(carto status --project my-project --json)

# Query and pipe to jq
carto query --project my-project "What are the main API endpoints?" --json | jq '.answer'
```

A typical CI step might look like:

```yaml
# .github/workflows/carto.yml
- name: Update Carto index
  run: carto index --path . --name ${{ github.repository }} --json

- name: Generate skill files
  run: carto patterns --project ${{ github.repository }}
```

## Managing Multiple Projects

You can index and query as many codebases as you like. Each project is stored under its own name in Memories:

```bash
carto index --path /repos/frontend --name frontend
carto index --path /repos/backend  --name backend
carto index --path /repos/infra    --name infra

carto projects list
# frontend
# backend
# infra

carto query --project frontend "How is routing configured?"
carto query --project backend  "Where is the auth middleware?"
```

## Limitations

- **Build requirements** — Carto requires Go 1.25+ with CGO enabled and a C compiler (gcc) because of the tree-sitter dependency. Pre-built binaries are available via Docker if you prefer not to build from source.
- **Memories server** — A running Memories server is required for storing and retrieving the index. See the [Docker Deployment](feat-010-docker-deployment.md) guide for the easiest way to run both together.
- **LLM API key** — You need a valid `LLM_API_KEY` or `ANTHROPIC_API_KEY` for the indexing and query commands.

## Related

- [REST API](feat-007-rest-api.md) — programmatic access to the same features over HTTP
- [Web Dashboard](feat-008-web-ui.md) — visual interface for browsing projects and querying the index
- [Docker Deployment](feat-010-docker-deployment.md) — run Carto without local build dependencies

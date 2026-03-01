# Feature Handoff: CLI

**ID:** fh-006
**Feature:** Command-Line Interface
**Components:** `cmd/carto/` — `main.go`, `cmd_index.go`, `cmd_query.go`, `cmd_modules.go`, `cmd_patterns.go`, `cmd_status.go`, `cmd_serve.go`, `cmd_projects.go`, `cmd_sources.go`, `cmd_config.go`, `helpers.go`

---

## Overview

The Carto CLI is the primary user interface for interacting with the indexing pipeline, querying the semantic index, and managing projects. It is built on the [Cobra](https://github.com/spf13/cobra) framework and exposes 9 subcommands. All commands that produce structured output support a `--json` flag for machine-readable output, making the CLI suitable for both interactive use and CI/CD integration.

The binary is built from `cmd/carto/main.go`, which registers all subcommands and handles global flags (`--version`, `--help`).

---

## Commands

### `index`

Triggers the 6-phase indexing pipeline for a project.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--incremental` | bool | `true` | Only re-index files that changed since the last run (uses SHA-256 manifest). |
| `--full` | bool | `false` | Force a full re-index, ignoring the manifest. |
| `--module` | string | `""` | Index only the specified module within the project. |
| `--project` | string | `""` | Project name. Required if not inferred from the current directory. |
| `--all` | bool | `false` | Index all registered projects. |
| `--changed` | bool | `false` | Index only projects with detected file changes. |
| `--json` | bool | `false` | Output progress and results as JSON lines. |

### `query`

Queries the semantic index and returns context from the stored layers.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--project` | string | `""` | Project to query against. Required. |
| `--tier` | string | `"standard"` | Retrieval tier: `mini` (~5KB), `standard` (~50KB), or `full` (~500KB). |
| `-k` | int | `5` | Number of results to return. |
| `--json` | bool | `false` | Output results as JSON. |

### `modules`

Lists all detected modules for a project.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--project` | string | `""` | Project name. |
| `--json` | bool | `false` | Output as JSON. |

### `patterns`

Generates skill files from the indexed data.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--format` | string | `"all"` | Output format: `claude` (CLAUDE.md), `cursor` (.cursorrules), or `all`. |
| `--project` | string | `""` | Project name. |
| `--json` | bool | `false` | Output as JSON. |

### `status`

Shows the current indexing status, last run time, and manifest state for a project.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--project` | string | `""` | Project name. |
| `--json` | bool | `false` | Output as JSON. |

### `serve`

Starts the HTTP server with the embedded Web UI and REST API.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--port` | int | `8950` | Port to listen on. |
| `--projects-dir` | string | `""` | Base directory for project discovery. |

### `projects`

Lists or manages registered projects.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--json` | bool | `false` | Output as JSON. |

### `sources`

Lists or manages external signal sources for a project.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--project` | string | `""` | Project name. |
| `--json` | bool | `false` | Output as JSON. |

### `config`

Displays or modifies the current configuration.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--json` | bool | `false` | Output as JSON. |

---

## `--json` Support

All commands that produce output support `--json`. When enabled, the command writes one or more JSON objects to stdout (one per line for streaming commands like `index`). Errors are also emitted as JSON objects with an `"error"` field. This makes it straightforward to pipe output into `jq`, consume it from scripts, or integrate with CI systems.

Example:
```bash
carto index --project myapp --json | jq '.phase'
carto query "How does auth work?" --project myapp --json
```

---

## Configuration

The CLI reads configuration from environment variables. See `.env.example` for the full list.

| Variable | Required | Description |
|----------|----------|-------------|
| `LLM_API_KEY` or `ANTHROPIC_API_KEY` | Yes | API key for the LLM provider used during indexing. |
| `MEMORIES_URL` | Yes | Base URL of the Memories server (e.g., `http://localhost:8900`). |
| `MEMORIES_API_KEY` | No | API key for Memories, if authentication is enabled. |
| `LLM_PROVIDER` | No | LLM provider: `anthropic`, `openai`, or `ollama`. Defaults to `anthropic`. |
| `LLM_MODEL` | No | Model name override. |
| `LLM_FAST_MODEL` | No | Model for the fast tier (atom extraction). |
| `LLM_DEEP_MODEL` | No | Model for the deep tier (cross-component analysis). |

---

## Edge Cases

- **Missing API key:** If `LLM_API_KEY` / `ANTHROPIC_API_KEY` is not set, the `index` command fails immediately with a clear error message. Query and status commands may still work if the Memories server is reachable and data already exists.
- **Memories unreachable:** Commands that require Memories (`index`, `query`, `status`, `modules`) fail with a connection error. The error message includes the configured `MEMORIES_URL` to aid debugging.
- **Invalid project path:** If `--project` references a project that does not exist or whose path is inaccessible, the CLI exits with a non-zero code and a descriptive error.
- **Port conflict on `serve`:** If the requested port is already in use, the server fails to bind and exits with an "address already in use" error. Use `--port` to specify an alternative.
- **Concurrent index runs:** The CLI does not enforce single-instance locking. Running two `index` commands against the same project concurrently can produce unpredictable results. The pipeline does not coordinate across processes.

---

## Common Questions

**Q1: What commands are available?**
Run `carto --help` to see all commands. Each command also supports `--help` (e.g., `carto index --help`).

**Q2: How do I use JSON output in scripts?**
Append `--json` to any command. The output is newline-delimited JSON. Use `jq` for parsing:
```bash
carto modules --project myapp --json | jq '.[].name'
```

**Q3: How do I index a specific module instead of the whole project?**
Use `--module`:
```bash
carto index --project myapp --module auth
```
This indexes only the `auth` module. The manifest tracks module-level changes, so subsequent `--incremental` runs only re-index changed files within that module.

**Q4: The serve port 8950 is already in use. How do I change it?**
Use the `--port` flag:
```bash
carto serve --port 9000
```

**Q5: How do I list and manage projects?**
Use `carto projects` to list all registered projects. Use `carto projects --json` for machine-readable output. Projects are managed through the REST API or by editing the project configuration directly. The `sources` command manages external signal sources per project.

---

## Troubleshooting

| Symptom | Likely Cause | Resolution |
|---------|-------------|------------|
| `command not found: carto` | Binary not in `$PATH` or not built. | Build with `go build -o carto ./cmd/carto` and ensure the output location is in `$PATH`, or run with `./carto`. |
| `missing API key` error on `index` | `LLM_API_KEY` or `ANTHROPIC_API_KEY` not set. | Export the variable: `export LLM_API_KEY=sk-...` or add it to `.env`. |
| `connection refused` on any command | Memories server is not running or `MEMORIES_URL` is wrong. | Start Memories (`docker compose up memories`) and verify `MEMORIES_URL` matches the running instance. |
| `--json` output looks malformed | Mixing stdout and stderr. Logs may be going to stdout. | Redirect stderr: `carto index --project x --json 2>/dev/null`. Check that you are parsing line-by-line (NDJSON), not as a single JSON object. |
| `address already in use` on `serve` | Port 8950 (or the specified port) is occupied. | Use `--port` to pick another port, or find and stop the process on that port: `lsof -i :8950`. |
| `unknown command "xyz"` | Typo or using a command that does not exist. | Run `carto --help` to see valid commands. |
| Index runs but produces no output | Project has no scannable files, or all files are excluded by `.gitignore` rules. | Check `carto status --project x` and verify the project path contains supported source files. |

---
id: feat-006
type: feature-doc
audience: external
topic: CLI Reference
status: draft
generated: 2026-03-06
source-tier: carto
hermes-version: 1.0.0
---

# CLI Reference

Carto ships a single binary with 18 commands that cover every stage of the workflow -- from initial setup and indexing to querying, exporting, and upgrading. All commands produce structured JSON when piped or when you pass `--json`, making Carto easy to script into CI/CD pipelines or feed into other tools.

## Overview

The `carto` CLI is the primary interface for scanning codebases, building semantic indexes, querying code with natural language, and generating skill files for AI assistants. You can also manage projects, credentials, configuration, and audit logs entirely from the command line.

## Commands at a Glance

| Command | What It Does |
|---------|-------------|
| `init` | Run the configuration wizard (interactive or automated) |
| `index` | Scan a codebase and build (or update) the semantic index |
| `query` | Search the index with a natural-language question |
| `modules` | List the modules Carto discovered in a project |
| `patterns` | Generate skill files (CLAUDE.md, .cursorrules) from the index |
| `status` | Show the current indexing state of a project |
| `serve` | Start the web UI and REST API server |
| `projects` | List, add, or remove projects |
| `sources` | View or edit the external signal sources for a project |
| `config` | View or update global Carto configuration |
| `auth` | Manage API keys and credentials |
| `doctor` | Run pre-flight environment health checks |
| `export` | Stream index data as NDJSON |
| `import` | Ingest NDJSON index data from stdin |
| `logs` | Query or tail the audit log |
| `completions` | Generate shell completion scripts |
| `upgrade` | Check for (and install) new versions |
| `version` | Show version information |
| `about` | Display product identity and branding |

## Installation and Setup

### Building from Source

Carto uses tree-sitter for code parsing, which requires CGO. Make sure you have Go 1.25+ and a C compiler available, then build:

```bash
git clone https://github.com/anthropics/carto.git
cd carto
go build -o carto ./cmd/carto
```

Move the resulting `carto` binary somewhere on your `$PATH`:

```bash
mv carto ~/bin/
```

### Initial Configuration

Run the setup wizard to configure your LLM provider, API key, and Memories server:

```bash
carto init
```

The wizard walks you through each setting interactively. For automated environments (CI, Docker, scripting), use the non-interactive mode:

```bash
carto init --non-interactive \
  --llm-provider anthropic \
  --api-key "$ANTHROPIC_API_KEY" \
  --memories-url http://localhost:8900
```

After setup, verify your environment is ready:

```bash
carto doctor
```

## Core Workflow

The most common sequence is **index, query, patterns**. Here is a start-to-finish example:

### 1. Index a Codebase

```bash
carto index --path /path/to/your/repo --name my-project
```

Carto scans the directory, chunks the code with tree-sitter, runs LLM analysis, and stores the results in Memories. On subsequent runs it performs incremental indexing -- only changed files are re-processed.

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

### `carto init`

Set up Carto configuration interactively or via flags for automation. The wizard prompts for your LLM provider, API key, Memories server URL, and projects directory.

```bash
# Interactive mode (default)
carto init

# Non-interactive mode for automation
carto init --non-interactive --llm-provider anthropic --api-key sk-ant-... --memories-url http://localhost:8900
```

| Flag | Description |
|------|-------------|
| `--non-interactive` | Skip prompts; require all values via flags or env vars |
| `--llm-provider` | LLM provider: `anthropic`, `openai`, or `ollama` |
| `--api-key` | LLM API key |
| `--memories-url` | Memories server URL |
| `--memories-key` | Memories API key |
| `--projects-dir` | Directory for indexed projects |

---

### `carto index`

Scan a codebase and build (or update) the semantic index.

```bash
carto index --path /repos/backend --name backend-api
```

| Flag | Description |
|------|-------------|
| `--path` | Path to the codebase directory |
| `--name` | Project name (used as the key in Memories) |
| `--full` | Force a full re-index, ignoring the incremental manifest |

---

### `carto query`

Search the semantic index with a natural-language question.

```bash
carto query --project backend-api "Where are database migrations defined?"
```

| Flag | Description |
|------|-------------|
| `--project` | Project to query |
| `--tier` | Retrieval tier: `mini`, `standard`, or `full` (default: `standard`) |

---

### `carto modules`

List all modules discovered during indexing.

```bash
carto modules --project backend-api
```

---

### `carto patterns`

Generate skill files from the current index.

```bash
carto patterns --project backend-api
```

| Flag | Description |
|------|-------------|
| `--project` | Project to generate patterns for |
| `--output` | Output directory (defaults to the project root) |

---

### `carto status`

Check the indexing state of a project -- when it was last indexed, how many files were processed, and whether an index run is currently in progress.

```bash
carto status --project backend-api
```

---

### `carto serve`

Start the web UI and REST API server.

```bash
carto serve --port 8950
```

| Flag | Description |
|------|-------------|
| `--port` | HTTP port (default: `8950`) |
| `--projects-dir` | Base directory for discovering projects |

---

### `carto projects`

Manage the list of known projects.

```bash
# List all projects
carto projects list

# Add a project
carto projects add --name my-app --path /repos/my-app

# Remove a project (prompts for confirmation)
carto projects delete --name my-app

# Remove without confirmation (for scripting)
carto projects delete --name my-app --yes
```

The `projects delete` subcommand now asks for confirmation before removing a project. Use `--yes` (or `-y`) to skip the prompt in automated environments.

---

### `carto sources`

View or update external signal sources (CI, issue trackers, docs) for a project.

```bash
# View current sources
carto sources --project backend-api

# Update sources
carto sources --project backend-api --set ci=https://github.com/org/repo/actions
```

---

### `carto config`

View or update global Carto configuration (LLM provider, API keys, Memories URL).

```bash
# View current config
carto config

# Set a value
carto config --set llm_provider=anthropic
```

---

### `carto auth`

Manage API keys and credentials. The `auth` command has three subcommands:

```bash
# Check which credentials are configured
carto auth status

# Store an API key in the persisted config
carto auth set-key anthropic sk-ant-api03-your-key-here

# Test connectivity to your LLM provider
carto auth validate
```

**`auth status`** shows all configured credentials with masked values and flags any that are missing.

**`auth set-key <provider> <key>`** saves a key to the config file. Supported providers: `anthropic`, `openai`, `memories`, `github`, `jira`, `linear`, `notion`, `slack`, `server`.

**`auth validate`** makes a lightweight HTTP request to the configured LLM provider to verify the API key works. No tokens are consumed.

| Flag (validate) | Description |
|------|-------------|
| `--timeout` | Probe timeout (default: `10s`) |

---

### `carto doctor`

Run pre-flight environment health checks. This inspects your LLM key, Memories server connectivity, projects directory, config files, audit logging, and server authentication.

```bash
carto doctor
```

| Flag | Description |
|------|-------------|
| `--timeout` | Timeout for network probes (default: `8s`) |
| `--skip-network` | Skip network connectivity checks |

The exit code is non-zero if any check fails.

---

### `carto export`

Stream index data for a project as newline-delimited JSON (NDJSON). Each line is a JSON object with `id`, `text`, `source`, and optional `metadata` fields.

```bash
# Export everything for a project
carto export --project myapp > backup.ndjson

# Export only a specific layer
carto export --project myapp --layer atoms

# Pipe to jq for processing
carto export --project myapp | jq '.text'
```

| Flag | Description |
|------|-------------|
| `--project`, `-p` | Project name (required) |
| `--layer` | Filter to a specific layer: `atoms`, `wiring`, `zones`, `blueprint`, `patterns` |

---

### `carto import`

Ingest NDJSON index data from stdin into the Memories store for a project. Each line should be a JSON object with at least `text` and `source` fields.

```bash
# Append entries to an existing index
cat backup.ndjson | carto import --project myapp

# Replace all existing data and import fresh
carto import --project myapp --strategy replace --yes < data.ndjson
```

| Flag | Description |
|------|-------------|
| `--project`, `-p` | Project name (required) |
| `--strategy` | Import strategy: `add` (default) or `replace` |

The `replace` strategy deletes all existing entries for the project before importing. It prompts for confirmation unless you pass `--yes`.

---

### `carto logs`

Query or tail the Carto audit log. The audit log path is set via the `--log-file` global flag or the `CARTO_AUDIT_LOG` environment variable.

```bash
# Show the 20 most recent entries
carto logs --last 20

# Filter by command name
carto logs --command index

# Show only errors
carto logs --result error

# Tail the log in real time
carto logs --follow
```

| Flag | Description |
|------|-------------|
| `--follow`, `-f` | Tail the log file for new entries (Ctrl+C to stop) |
| `--last`, `-n` | Number of recent entries to display (default: `20`) |
| `--command` | Filter by command name (substring match) |
| `--result` | Filter by result: `ok` or `error` |

---

### `carto completions`

Generate shell completion scripts for tab-completion of commands, flags, and arguments.

```bash
# Bash
source <(carto completions bash)

# Zsh — add to your fpath
carto completions zsh > "${fpath[1]}/_carto"

# Fish
carto completions fish > ~/.config/fish/completions/carto.fish

# PowerShell
carto completions powershell | Out-String | Invoke-Expression
```

Supported shells: `bash`, `zsh`, `fish`, `powershell`.

---

### `carto upgrade`

Check for newer versions of Carto on GitHub and optionally upgrade.

```bash
# Check for updates without installing
carto upgrade --check

# Upgrade (prompts for confirmation)
carto upgrade

# Upgrade without confirmation
carto upgrade --yes
```

| Flag | Description |
|------|-------------|
| `--check` | Check for updates without installing |

---

### `carto version`

Display the current Carto version. Supports `--json` for structured output.

```bash
carto version
```

---

### `carto about`

Show product identity information including the Carto brand palette and tagline.

```bash
carto about
```

## JSON Output for Automation

### Envelope Contract

All commands return structured JSON when output is piped or when you pass `--json`. The envelope has a consistent shape:

**Success:**

```json
{
  "ok": true,
  "data": {
    "project": "my-app",
    "files_indexed": 142
  }
}
```

**Error:**

```json
{
  "ok": false,
  "error": "memories server unreachable at http://localhost:8900",
  "code": "CONNECTION_ERROR"
}
```

You can always check the `ok` field to determine success or failure, and use the `code` field for programmatic error handling.

### Error Codes

| Code | Exit Code | Meaning |
|------|-----------|---------|
| `GENERAL_ERROR` | 1 | Unhandled error |
| `NOT_FOUND` | 2 | Resource doesn't exist |
| `CONFIG_ERROR` | 3 | Invalid configuration |
| `CONNECTION_ERROR` | 4 | Can't reach a required service |
| `AUTH_FAILURE` | 5 | Bad or missing API key |

### TTY Auto-Detection

Carto automatically detects whether its output is going to a terminal or a pipe:

- **Terminal (interactive)** -- you see human-readable, colored output
- **Piped (non-interactive)** -- JSON is emitted automatically, no `--json` needed
- **`--json`** -- forces JSON even when running in a terminal
- **`--pretty`** -- forces human-readable output even when piped

This means you can write scripts that simply pipe `carto` output to `jq` without remembering to add `--json`:

```bash
# These are equivalent when piped:
carto status --project myapp | jq '.data.files_indexed'
carto status --project myapp --json | jq '.data.files_indexed'

# Force human output even in a pipe:
carto status --project myapp --pretty | less
```

### CI/CD Example

```bash
# Index and capture the result
RESULT=$(carto index --path . --name my-project)
echo "$RESULT" | jq '.ok'

# Check if an update is available
carto upgrade --check | jq '.data.update_available'

# Query and extract the answer
carto query --project my-project "What are the main API endpoints?" | jq '.data'
```

A typical GitHub Actions step:

```yaml
- name: Update Carto index
  run: |
    carto index --path . --name ${{ github.repository }} --yes
    carto patterns --project ${{ github.repository }}
```

## Shell Completions

Set up tab-completion for faster command entry. Choose the instructions for your shell:

**Bash** -- add to your `~/.bashrc`:
```bash
source <(carto completions bash)
```

**Zsh** -- generate and place in your fpath:
```bash
carto completions zsh > "${fpath[1]}/_carto"
```

**Fish** -- save to the completions directory:
```bash
carto completions fish > ~/.config/fish/completions/carto.fish
```

**PowerShell** -- add to your profile:
```powershell
carto completions powershell | Out-String | Invoke-Expression
```

## Global Flags

These flags are available on every command:

| Flag | Short | Description |
|------|-------|-------------|
| `--json` | | Output machine-readable JSON (auto-detected when piped) |
| `--pretty` | | Force human-readable output even when piped |
| `--yes` | `-y` | Skip confirmation prompts |
| `--quiet` | `-q` | Suppress progress spinners; only output the result |
| `--verbose` | `-v` | Print verbose/debug output to stderr |
| `--log-file` | | Append structured JSON audit events to this file |
| `--profile` | | Config profile to use (overrides `CARTO_PROFILE` env var) |

## Examples

### Index multiple projects and query across them

```bash
carto index --path /repos/frontend --name frontend
carto index --path /repos/backend  --name backend

carto projects list
carto query --project frontend "How is routing configured?"
carto query --project backend  "Where is the auth middleware?"
```

### Back up and restore a project index

```bash
# Export
carto export --project myapp > myapp-index.ndjson

# Restore on another machine
cat myapp-index.ndjson | carto import --project myapp --strategy replace --yes
```

### Set up a fresh environment from scratch

```bash
# 1. Configure
carto init --non-interactive \
  --llm-provider anthropic \
  --api-key "$ANTHROPIC_API_KEY" \
  --memories-url http://localhost:8900

# 2. Verify
carto doctor

# 3. Index
carto index --path /repos/my-project --name my-project

# 4. Generate skill files
carto patterns --project my-project
```

### Monitor for errors in the audit log

```bash
# Show recent errors
carto logs --result error --last 10

# Tail the log for new events
carto logs --follow
```

## Limitations

- **Build requirements** -- Carto requires Go 1.25+ with CGO enabled and a C compiler (gcc) because of the tree-sitter dependency. Pre-built binaries are available via Docker if you prefer not to build from source.
- **Memories server** -- A running Memories server is required for storing and retrieving the index. See the [Docker Deployment](feat-010-docker-deployment.md) guide for the easiest way to run both together.
- **LLM API key** -- You need a valid `LLM_API_KEY` or `ANTHROPIC_API_KEY` for the indexing and query commands.

## Related

- [Getting Started](../getting-started.md) -- first-time setup walkthrough
- [Configuration Reference](../config-reference.md) -- all environment variables and settings
- [Error Reference](../error-reference.md) -- error codes, messages, and resolutions
- [REST API](feat-007-rest-api.md) -- programmatic access to the same features over HTTP
- [Web Dashboard](feat-008-web-ui.md) -- visual interface for browsing projects and querying the index
- [Docker Deployment](feat-010-docker-deployment.md) -- run Carto without local build dependencies

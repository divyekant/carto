---
id: uc-006
type: use-case
audience: internal
topic: CLI Workflow
status: draft
generated: 2026-03-06
source-tier: carto
hermes-version: 1.0.0
---

# Use Case: CLI Workflow

**ID:** uc-006
**Topic:** CLI Workflow
**Trigger:** A developer installs Carto and uses the CLI to set up, index, query, export, and manage codebases.

---

## Primary Flow: First-Time Setup Through Skill File Generation

### 1. Build the Binary

```bash
# CGO is required for tree-sitter parsing
go build -o carto ./cmd/carto
```

The binary is a single self-contained executable. Move it to a directory in `$PATH` for convenience.

### 2. Configure Shell Completions (Optional)

```bash
# bash
source <(carto completions bash)

# zsh
carto completions zsh > "${fpath[1]}/_carto"

# fish
carto completions fish > ~/.config/fish/completions/carto.fish
```

### 3. Run the Configuration Wizard

```bash
carto init
```

The wizard prompts for LLM provider, API key, Memories URL, Memories API key, and projects directory. It writes the configuration to `~/.config/carto/config.json`.

For automation or scripting, use non-interactive mode:

```bash
carto init --non-interactive \
  --llm-provider anthropic \
  --api-key "sk-ant-..." \
  --memories-url "http://localhost:8900" \
  --projects-dir "/projects"
```

### 4. Verify the Environment

```bash
carto doctor
```

The doctor command runs pre-flight checks: API key presence, provider configuration, projects directory writability, config file existence, audit log setup, Memories connectivity, and LLM endpoint reachability. Fix any failures before proceeding.

### 5. Index a Codebase

```bash
carto index --project myapp
```

The first run performs a full scan. Subsequent runs use `--incremental` (the default) to re-index only changed files based on SHA-256 manifest comparison.

### 6. Query the Index

```bash
carto query "How does authentication work?" --project myapp
```

Adjust retrieval depth with `--tier`:
- `mini` (~5KB) for quick lookups
- `standard` (~50KB) for typical questions
- `full` (~500KB) for deep exploration

### 7. Generate Skill Files

```bash
carto patterns --project myapp --format all
```

This writes `CLAUDE.md` and `.cursorrules` files into the project directory, populated with architecture, module, and pattern information from the index.

---

## Variation: CI/CD Integration with Envelope Contract

In a CI pipeline, the CLI operates non-interactively. TTY auto-detection triggers JSON envelope output automatically when stdout is not a terminal.

```yaml
# Example GitHub Actions workflow
- name: Set up Carto
  run: |
    carto init --non-interactive \
      --llm-provider anthropic \
      --api-key "${{ secrets.ANTHROPIC_KEY }}" \
      --memories-url "${{ vars.MEMORIES_URL }}"

- name: Index codebase
  run: |
    result=$(carto index --project ${{ github.repository }})
    echo "$result" | jq '.data'

- name: Check index status
  run: |
    carto status --project ${{ github.repository }} | jq '.data.last_indexed'
```

The JSON envelope contract guarantees every command returns `{"ok": true, "data": ...}` or `{"ok": false, "error": ..., "code": ...}`. Check the `ok` field or the process exit code (0 for success, 1-5 for categorized failures).

For destructive operations in CI, pass `--yes` to skip confirmation prompts:

```bash
carto import --project myapp --strategy replace --yes < data.ndjson
```

---

## Variation: AI Agent Consumption

AI agents (Claude Code, Cursor, custom agents) consume the CLI programmatically. The TTY auto-detection and `--yes` flag are designed for this use case.

```bash
# Agent reads project list (auto-JSON because piped)
projects=$(carto projects list)
echo "$projects" | jq -r '.data[].name'

# Agent indexes a project (--yes for non-interactive)
carto index --project myapp --yes

# Agent queries the index
result=$(carto query "What patterns does the auth module use?" --project myapp)
echo "$result" | jq '.data'
```

Key agent-usability patterns:
- **TTY auto-detection:** No need to pass `--json` when output is captured by a subprocess.
- **`--yes` flag:** Required for destructive operations (`projects delete`, `import --strategy replace`, `upgrade`). Without `--yes`, `confirmAction()` returns false in non-TTY mode and the operation is silently cancelled.
- **Error codes:** The `code` field in error envelopes enables programmatic error classification. Agents can differentiate between config errors (fix config), connection errors (retry later), and auth failures (re-authenticate).
- **Exit codes:** Process exit codes 0-5 map directly to error categories for shell-level branching.

---

## Variation: Export and Import Workflow

For backup, migration, or index transfer between environments:

### Export

```bash
# Export full index as NDJSON
carto export --project myapp > myapp-backup.ndjson

# Export only atoms layer
carto export --project myapp --layer atoms > myapp-atoms.ndjson

# Get export summary in JSON envelope mode
carto export --project myapp --json | jq '.data.exported'
```

Each NDJSON line is a JSON object with `id`, `text`, `source`, and optional `metadata` fields. Default output is raw NDJSON (for piping). With `--json`, the command emits an envelope with the export count instead of the raw stream.

### Import

```bash
# Append to existing index
cat myapp-backup.ndjson | carto import --project myapp

# Replace existing index entirely
cat myapp-backup.ndjson | carto import --project myapp --strategy replace --yes
```

The `replace` strategy deletes all existing entries for the project before importing. Each NDJSON line must have at least `text` and `source` fields. Invalid lines are skipped with a warning.

### Cross-Environment Transfer

```bash
# On source machine
carto export --project myapp > transfer.ndjson

# On target machine (after configuring Memories)
cat transfer.ndjson | carto import --project myapp --strategy replace --yes
```

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
carto projects list

# Show details for a specific project
carto projects show backend-api

# Index all projects
carto index --all

# Index only projects with file changes
carto index --all --changed

# Query a specific project
carto query "What patterns does the API use?" --project backend-api

# Generate skill files per project
carto patterns --project backend-api --format claude
carto patterns --project frontend-app --format cursor

# Delete a project's index (with confirmation)
carto projects delete stale-project --yes
```

---

## Variation: Audit Log Monitoring

For operational observability, enable audit logging and use the logs command:

```bash
# Enable audit logging
export CARTO_AUDIT_LOG=~/.carto/audit.log

# Run some commands (they auto-log)
carto index --project myapp
carto query "auth flow" --project myapp

# View recent log entries
carto logs --last 20

# Filter by command
carto logs --command index

# Filter by result
carto logs --result error

# Tail the log for new entries
carto logs --follow

# Get log entries as JSON
carto logs --json | jq '.data.entries'
```

---

## Variation: Credential and Config Management

### Setting Up Credentials

```bash
# Store credentials via auth command
carto auth set-key anthropic sk-ant-api03-...
carto auth set-key memories mem-key-...
carto auth set-key github ghp_...

# Verify credential status
carto auth status

# Test LLM connectivity
carto auth validate
```

### Managing Configuration

```bash
# View all config
carto config get

# View a specific key
carto config get fast_model

# Update a setting
carto config set fast_model claude-haiku-4-5-20251001
carto config set max_concurrent 20

# Validate configuration
carto config validate

# Show config file paths
carto config path
```

---

## Variation: Version Management

```bash
# Check current version
carto version

# Check for updates
carto upgrade --check

# Check for updates (JSON, for automation)
carto upgrade --check --json | jq '.data.update_available'
```

---

## Postconditions

- The 7-layer semantic index is stored in Memories and available for queries.
- Skill files (`CLAUDE.md`, `.cursorrules`) are written to the project directory.
- The manifest is updated to reflect the current file state, enabling future incremental runs.
- Status can be checked at any time with `carto status --project <name>`.
- The audit log records all command executions for operational review.
- Exported NDJSON files can be used for backup, migration, or cross-environment transfer.
- Shell completions enable tab-completion for all 18 commands and their flags.

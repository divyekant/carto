---
id: fh-006
type: feature-handoff
audience: internal
topic: CLI
status: draft
generated: 2026-03-06
source-tier: carto
context-files: [output.go, errors.go, helpers.go, main.go, branding.go, cmd_completions.go, cmd_init.go, cmd_export.go, cmd_import.go, cmd_logs.go, cmd_upgrade.go]
hermes-version: 1.0.0
---

# Feature Handoff: CLI

**ID:** fh-006
**Feature:** Command-Line Interface
**Components:** `cmd/carto/` -- `main.go`, `output.go`, `errors.go`, `helpers.go`, `branding.go`, `cmd_index.go`, `cmd_query.go`, `cmd_modules.go`, `cmd_patterns.go`, `cmd_status.go`, `cmd_serve.go`, `cmd_projects.go`, `cmd_sources.go`, `cmd_config.go`, `cmd_auth.go`, `cmd_doctor.go`, `cmd_version.go`, `cmd_about.go`, `cmd_completions.go`, `cmd_init.go`, `cmd_export.go`, `cmd_import.go`, `cmd_logs.go`, `cmd_upgrade.go`

---

## What It Does

The Carto CLI is the primary user interface for interacting with the indexing pipeline, querying the semantic index, managing projects, and operating the system. It is built on the [Cobra](https://github.com/spf13/cobra) framework and exposes 18 subcommands across four functional groups: core pipeline operations (index, query, modules, patterns, status), infrastructure and management (serve, projects, sources, config, auth, doctor), setup and lifecycle (init, completions, version, about, upgrade), and data portability (export, import, logs).

All commands produce output through a JSON envelope contract. Terminal users see colored human-readable output using the gold brand palette. Non-terminal consumers (piped output, CI pipelines, AI agents) receive structured JSON automatically via TTY detection. The binary is built from `cmd/carto/main.go`, which registers all subcommands and handles global flags.

---

## How It Works

### JSON Envelope Contract

Every command wraps its output in a standard envelope structure that matches the Memories CLI convention:

**Success:**
```json
{
  "ok": true,
  "data": { ... }
}
```

**Error:**
```json
{
  "ok": false,
  "error": "human-readable message",
  "code": "ERROR_CODE"
}
```

The envelope is produced by `writeEnvelopeHuman()` in `output.go`. In human mode, the function calls a custom renderer function instead. In JSON mode, success envelopes go to stdout; error envelopes go to stderr.

### TTY Auto-Detection

The function `isJSONMode(cmd)` in `output.go` determines whether to emit JSON or human output. The priority chain is:

1. `--json` flag explicitly set -- emit JSON.
2. `--pretty` flag explicitly set -- emit human output (overrides `--json`).
3. Neither flag set -- fall back to TTY detection: if stdout is not a terminal (`term.IsTerminal` returns false), emit JSON; otherwise emit human output.

This means piping `carto` output to another program, a file, or consuming it from an AI agent automatically triggers JSON mode without any flags. The `--pretty` flag forces human-readable output even when piped, which is useful for logging pipelines that want colored text.

### Typed Errors

The `errors.go` file defines the `cliError` struct that carries three pieces of information:

- **`msg`** (string): Human-readable error message.
- **`code`** (string): Machine-readable error code for the JSON envelope.
- **`exit`** (int): Process exit code.

Constructor functions create typed errors: `newConnectionError()`, `newAuthError()`, `newNotFoundError()`, `newConfigError()`. The `toCliError()` classifier extracts a `*cliError` from any error via `errors.As`; untyped errors become `GENERAL_ERROR` with exit code 1.

### Error Codes and Exit Codes

| Error Code | Exit Code | When |
|---|---|---|
| -- | 0 | Success |
| `GENERAL_ERROR` | 1 | Unhandled or unclassified errors |
| `NOT_FOUND` | 2 | Project, resource, or file does not exist |
| `CONFIG_ERROR` | 3 | Missing or invalid configuration |
| `CONNECTION_ERROR` | 4 | Cannot reach Memories server or LLM provider |
| `AUTH_FAILURE` | 5 | Bad or missing API key |

### runWithEnvelope

The `runWithEnvelope()` function in `errors.go` is the centralized command runner. It:

1. Executes the command function `fn() (any, error)`.
2. On error: classifies the error via `toCliError()`, prints a colored error in human mode, writes the error envelope in JSON mode, logs an audit event, and calls `os.Exit()` with the appropriate exit code.
3. On success: writes the success envelope (JSON) or calls the human renderer, and logs an audit event.

New commands should adopt `runWithEnvelope` as the standard entry point. Existing commands use `writeEnvelopeHuman` directly.

### Audit Logging

Every command can emit a structured JSON audit event via `logAuditEvent()` in `helpers.go`. Events are appended to the file specified by `--log-file` flag or the `CARTO_AUDIT_LOG` environment variable. When neither is set, logging is a silent no-op.

Each audit event contains:

```json
{
  "ts": "2026-03-06T12:00:00Z",
  "level": "audit",
  "command": "carto index",
  "args": ["--project", "myapp"],
  "result": "ok",
  "error": "",
  "extra": { "project": "myapp" }
}
```

### Gold Brand Palette

All CLI colors use the Carto gold brand palette defined in `branding.go` and `helpers.go`:

| Name | ANSI Code | Hex | Role |
|---|---|---|---|
| Brand Gold | `\033[33m` | #d4af37 | Primary accent, headers, active states |
| Stone | `\033[38;5;249m` | #78716c | De-emphasis, neutral text |
| Amber | `\033[38;5;214m` | #F59E0B | Warnings |
| Rose/Red | `\033[31m` | #F43F5E | Errors, destructive actions |
| Emerald/Green | `\033[32m` | #10B981 | Success indicators |

### Confirmation Prompts

The `confirmAction()` function in `output.go` gates destructive operations:

- If `--yes` is set: returns true immediately (for automation).
- If JSON mode (no `--yes`): returns false (agents must pass `--yes` explicitly).
- Otherwise: prints prompt to stderr and reads from stdin.

Commands using `confirmAction()`: `projects delete`, `import --strategy replace`, `upgrade`.

---

## User-Facing Behavior: All 18 Commands

### Core Pipeline Commands

#### `carto index`

Triggers the 6-phase indexing pipeline for a project.

| Flag | Type | Default | Description |
|---|---|---|---|
| `--incremental` | bool | `true` | Only re-index changed files (SHA-256 manifest). |
| `--full` | bool | `false` | Force a full re-index, ignoring the manifest. |
| `--module` | string | `""` | Index only the specified module within the project. |
| `--project` | string | `""` | Project name. Required if not inferred from cwd. |
| `--all` | bool | `false` | Index all registered projects. |
| `--changed` | bool | `false` | Index only projects with detected file changes. |

#### `carto query`

Queries the semantic index and returns context from stored layers.

| Flag | Type | Default | Description |
|---|---|---|---|
| `--project` | string | `""` | Project to query against. Required. |
| `--tier` | string | `"standard"` | Retrieval tier: `mini` (~5KB), `standard` (~50KB), `full` (~500KB). |
| `-k` | int | `5` | Number of results to return. |

#### `carto modules`

Lists all detected modules for a project.

| Flag | Type | Default | Description |
|---|---|---|---|
| `--project` | string | `""` | Project name. |

#### `carto patterns`

Generates skill files from indexed data.

| Flag | Type | Default | Description |
|---|---|---|---|
| `--format` | string | `"all"` | Output format: `claude`, `cursor`, or `all`. |
| `--project` | string | `""` | Project name. |

#### `carto status`

Shows current indexing status, last run time, and manifest state.

| Flag | Type | Default | Description |
|---|---|---|---|
| `--project` | string | `""` | Project name. |

### Infrastructure & Management Commands

#### `carto serve`

Starts the HTTP server with the embedded Web UI and REST API.

| Flag | Type | Default | Description |
|---|---|---|---|
| `--port` | int | `8950` | Port to listen on. |
| `--projects-dir` | string | `""` | Base directory for project discovery. |

#### `carto projects`

Manages indexed projects. Subcommands:

- **`projects list`** -- List all indexed projects with name, file count, and last-indexed timestamp.
- **`projects show <name>`** -- Show detailed info for a project (path, files, size, sources).
- **`projects delete <name>`** -- Delete a project's `.carto` directory. Requires `--yes` or interactive confirmation.

#### `carto sources`

Lists or manages external signal sources for a project.

| Flag | Type | Default | Description |
|---|---|---|---|
| `--project` | string | `""` | Project name. |

#### `carto config`

Manages runtime configuration. Subcommands:

- **`config get [key]`** -- Show all config values (or a specific key). Credentials are masked.
- **`config set <key> <value>`** -- Set a non-secret config value. Writable keys: `memories_url`, `fast_model`, `deep_model`, `max_concurrent`, `fast_max_tokens`, `deep_max_tokens`, `llm_provider`, `llm_base_url`.
- **`config validate`** -- Check that all required settings are present and consistent. Non-zero exit on failure.
- **`config path`** -- Show the config directory, default file path, and active file path.

#### `carto auth`

Manages credentials. Subcommands:

- **`auth status`** -- Show which credentials are configured (masked), with warnings for missing required keys.
- **`auth set-key <provider> <api-key>`** -- Store an API key in the persisted config. Providers: `anthropic`, `openai`, `memories`, `github`, `jira`, `linear`, `notion`, `slack`, `server`.
- **`auth validate`** -- Probe the configured LLM provider endpoint to verify the API key is accepted. No tokens consumed.

| Flag (auth validate) | Type | Default | Description |
|---|---|---|---|
| `--timeout` | duration | `10s` | Probe timeout. |

#### `carto doctor`

Runs pre-flight environment health checks. Validates: LLM API key, LLM provider, PROJECTS_DIR, config directory, audit log, server auth token, runtime environment, Memories connectivity, and LLM connectivity.

| Flag | Type | Default | Description |
|---|---|---|---|
| `--timeout` | duration | `8s` | Timeout for network probe checks. |
| `--skip-network` | bool | `false` | Skip network connectivity checks. |

Exit code is non-zero when any check fails.

### Setup & Lifecycle Commands

#### `carto init`

Interactive configuration wizard. Prompts for LLM provider, API key, Memories URL, Memories key, and projects directory. Writes to the persisted config file.

| Flag | Type | Default | Description |
|---|---|---|---|
| `--non-interactive` | bool | `false` | Skip prompts; use flags and env vars only. |
| `--llm-provider` | string | `""` | LLM provider (anthropic, openai, ollama). |
| `--api-key` | string | `""` | LLM API key. Required in non-interactive mode. |
| `--memories-url` | string | `""` | Memories server URL. |
| `--memories-key` | string | `""` | Memories API key. |
| `--projects-dir` | string | `""` | Directory for indexed projects. |

#### `carto completions <shell>`

Generates shell completion scripts. Accepts one argument: `bash`, `zsh`, `fish`, or `powershell`. Uses Cobra's built-in completion generators.

Installation:
```bash
source <(carto completions bash)
carto completions zsh > "${fpath[1]}/_carto"
carto completions fish > ~/.config/fish/completions/carto.fish
```

#### `carto version`

Shows build version and runtime information (Go version, OS, architecture). Emits an envelope in JSON mode.

#### `carto about`

Displays the full product identity card: tagline, description, target audience, how it works, headline features, brand color palette, and project URL.

#### `carto upgrade`

Checks GitHub releases for newer versions of Carto.

| Flag | Type | Default | Description |
|---|---|---|---|
| `--check` | bool | `false` | Check for updates without installing. |

The actual binary download is currently stubbed -- `carto upgrade` (without `--check`) prints a message directing the user to the GitHub releases page. The `--yes` flag would skip the upgrade confirmation prompt once download is implemented.

### Data Portability Commands

#### `carto export`

Streams index data for a project as NDJSON (one JSON object per line). Default output is raw NDJSON for piping. With `--json`, emits an envelope with an export count summary instead of the raw stream.

| Flag | Type | Default | Description |
|---|---|---|---|
| `--project`, `-p` | string | -- | Project name. Required. |
| `--layer` | string | `""` | Filter to a specific layer (atoms, wiring, zones, blueprint, patterns). |

#### `carto import`

Ingests NDJSON from stdin into the Memories store for a given project.

| Flag | Type | Default | Description |
|---|---|---|---|
| `--project`, `-p` | string | -- | Project name. Required. |
| `--strategy` | string | `"add"` | Import strategy: `add` (append) or `replace` (delete existing first). |

The `replace` strategy requires `--yes` or interactive confirmation. Each NDJSON line must have at least `text` and `source` fields. Lines with empty `text` are skipped. Import uses batched writes (100 entries per batch) with a 1MB line buffer.

#### `carto logs`

Queries and tails the Carto audit log file.

| Flag | Type | Default | Description |
|---|---|---|---|
| `--follow`, `-f` | bool | `false` | Tail the log file for new entries. |
| `--last`, `-n` | int | `20` | Number of recent entries to display. |
| `--command` | string | `""` | Filter by command name (substring match). |
| `--result` | string | `""` | Filter by result: `ok` or `error`. |

Requires `CARTO_AUDIT_LOG` or `--log-file` to be set. Follow mode polls the file every 500ms and does not work in JSON mode.

---

## Global Flags

These flags are available on every command:

| Flag | Type | Default | Description |
|---|---|---|---|
| `--json` | bool | `false` | Output machine-readable JSON envelope. |
| `--pretty` | bool | `false` | Force human-readable output even when piped. |
| `--yes`, `-y` | bool | `false` | Skip confirmation prompts. |
| `--quiet`, `-q` | bool | `false` | Suppress progress spinners. |
| `--verbose`, `-v` | bool | `false` | Print verbose/debug output to stderr. |
| `--log-file` | string | `""` | Append structured JSON audit events to this file. |
| `--profile` | string | `""` | Config profile to use (overrides `CARTO_PROFILE`). |

---

## Configuration

The CLI reads configuration from environment variables and the persisted config file. Environment variables take precedence for credential-type settings; the persisted file stores non-secret operational settings.

### Environment Variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `LLM_API_KEY` | Yes* | -- | Generic LLM API key (overrides ANTHROPIC_API_KEY). |
| `ANTHROPIC_API_KEY` | Yes* | -- | Anthropic-specific API key. |
| `LLM_PROVIDER` | No | `anthropic` | Provider: `anthropic`, `openai`, or `ollama`. |
| `LLM_MODEL` | No | -- | Model name override. |
| `LLM_BASE_URL` | No | -- | Base URL for OpenAI-compatible providers or Ollama. |
| `LLM_FAST_MODEL` / `CARTO_FAST_MODEL` | No | `claude-haiku-4-5-20251001` | Fast tier model. |
| `LLM_DEEP_MODEL` / `CARTO_DEEP_MODEL` | No | `claude-opus-4-6` | Deep tier model. |
| `CARTO_MAX_CONCURRENT` | No | `10` | Max concurrent LLM calls during atom extraction. |
| `MEMORIES_URL` | Yes | `http://localhost:8900` | Memories server URL. |
| `MEMORIES_API_KEY` | No | -- | Memories authentication key. |
| `PROJECTS_DIR` | No | -- | Base directory for multi-project management. |
| `CARTO_SERVER_TOKEN` | No | -- | Bearer token for the web server (empty = dev mode). |
| `CARTO_CORS_ORIGINS` | No | -- | Comma-separated allowed CORS origins. |
| `CARTO_AUDIT_LOG` | No | -- | File path for structured JSON audit log. |
| `CARTO_PROFILE` | No | `default` | Active config profile name. |
| `GITHUB_TOKEN` | No | -- | GitHub source access. |
| `JIRA_URL` | No | -- | Jira instance URL. |
| `JIRA_TOKEN` | No | -- | Jira authentication. |
| `LINEAR_TOKEN` | No | -- | Linear API access. |
| `NOTION_TOKEN` | No | -- | Notion API access. |
| `SLACK_TOKEN` | No | -- | Slack API access. |

\* At least one of `LLM_API_KEY` or `ANTHROPIC_API_KEY` is required unless using Ollama.

### Persisted Config File

The config file lives at `~/.config/carto/config.json` (or the path shown by `carto config path`). It stores operational settings and credentials. Use `carto config set` for non-secret values and `carto auth set-key` for credentials.

---

## Edge Cases & Limitations

- **Missing API key:** If no effective LLM API key is found, `index` fails immediately. Other commands (`query`, `status`, `modules`) may still work if Memories is reachable and data already exists.
- **Memories unreachable:** Commands requiring Memories (`index`, `query`, `status`, `modules`, `export`, `import`) fail with `CONNECTION_ERROR` (exit code 4). The error message includes the configured `MEMORIES_URL`.
- **Invalid project name in export/import:** If `--project` references a project with no stored data, `export` outputs zero entries. `import` succeeds but writes to the specified project namespace.
- **Port conflict on serve:** If the port is occupied, the server fails to bind with "address already in use." Use `--port` to pick an alternative.
- **Concurrent index runs:** The CLI does not enforce single-instance locking. Running two `index` commands against the same project concurrently can produce unpredictable results.
- **TTY detection in Docker:** Inside containers without a TTY (common in CI), stdout is automatically non-terminal, so JSON mode activates. Use `--pretty` to override.
- **Follow mode in JSON:** `carto logs --follow` is disabled in JSON mode. The command outputs existing entries as an envelope and exits.
- **Upgrade binary download:** The `carto upgrade` command currently only checks for updates. Actual binary download and replacement is not yet implemented; the command directs users to the GitHub releases page.
- **Import line buffer:** Each NDJSON line in `carto import` is capped at 1MB. Lines exceeding this limit are silently dropped by the scanner.
- **Replace strategy without --yes in JSON mode:** `carto import --strategy replace` without `--yes` silently cancels in JSON mode because `confirmAction()` returns false for non-interactive JSON consumers.
- **Shell completions do not auto-install:** `carto completions` outputs the script to stdout. The user must redirect it to the appropriate shell config location.

---

## Common Questions

**Q1: How do I get JSON output from every command?**

There are three ways: (1) Pass the `--json` flag explicitly. (2) Pipe the command to another program -- TTY auto-detection kicks in and emits JSON automatically. (3) Redirect stdout to a file. In all JSON cases, the output is wrapped in `{"ok": true, "data": ...}` or `{"ok": false, "error": ..., "code": ...}`.

**Q2: How do I use the CLI in a CI pipeline or from an AI agent?**

The TTY auto-detection handles this automatically. When stdout is not a terminal, the CLI emits JSON envelopes. For destructive operations (delete, replace-import, upgrade), pass `--yes` to skip confirmation prompts. Check the process exit code to determine success (0) or failure (1-5). Parse the `code` field in the error envelope for programmatic error handling.

**Q3: What is the difference between `--json` and `--pretty`?**

`--json` forces JSON envelope output regardless of TTY status. `--pretty` forces human-readable colored output even when piped. `--pretty` overrides `--json` if both are set. When neither is set, the mode is determined by whether stdout is a terminal.

**Q4: How do I back up and restore a project's index?**

Use export and import:
```bash
carto export --project myapp > backup.ndjson
cat backup.ndjson | carto import --project myapp --strategy replace --yes
```
Use `--layer` on export to back up a specific layer. Use `--strategy add` (the default) on import to merge without deleting existing data.

**Q5: How do I set up shell completions?**

Run the completions command for your shell and follow the instructions:
```bash
# bash
source <(carto completions bash)

# zsh
carto completions zsh > "${fpath[1]}/_carto"

# fish
carto completions fish > ~/.config/fish/completions/carto.fish
```

**Q6: How do I check if there is a newer version of Carto?**

Run `carto upgrade --check`. The command queries the GitHub releases API and compares the latest release tag against the current build version. The JSON envelope includes `current`, `latest`, and `update_available` fields.

**Q7: What does `carto doctor` check?**

It validates 9 areas: LLM API key presence, LLM provider validity, PROJECTS_DIR existence and writability, config directory, audit log configuration, server auth token, runtime environment (Docker vs native), Memories server health, and LLM endpoint reachability. Use `--skip-network` to skip the last two.

---

## Troubleshooting

| Symptom | Error Code | Exit Code | Resolution |
|---|---|---|---|
| `command not found: carto` | -- | -- | Build with `go build -o carto ./cmd/carto` and add to `$PATH`. |
| `missing API key` or `API key not set` | `CONFIG_ERROR` | 3 | Set `LLM_API_KEY` or `ANTHROPIC_API_KEY`, or run `carto auth set-key`. |
| `connection refused` or `failed to connect to Memories` | `CONNECTION_ERROR` | 4 | Start the Memories server and verify `MEMORIES_URL`. |
| `authentication failed` from `auth validate` | `AUTH_FAILURE` | 5 | Re-run `carto auth set-key <provider> <new-key>`. |
| `project not found or has no index` | `NOT_FOUND` | 2 | Verify the project name with `carto projects list`. |
| `no audit log configured` from `carto logs` | `CONFIG_ERROR` | 3 | Set `CARTO_AUDIT_LOG` env var or pass `--log-file`. |
| `audit log file not found` from `carto logs` | `NOT_FOUND` | 2 | The audit log file does not exist yet. Run a command with `--log-file` to create it. |
| `address already in use` on `carto serve` | `GENERAL_ERROR` | 1 | Use `--port` to pick another port or stop the process on port 8950. |
| `--json` output appears malformed | -- | -- | Ensure you are consuming the envelope (not raw NDJSON). Error envelopes go to stderr; data envelopes go to stdout. |
| `import cancelled` when using `--strategy replace` | -- | -- | Pass `--yes` to skip the confirmation prompt. In JSON mode, `--yes` is required for destructive operations. |
| `unsupported shell` from `completions` | `GENERAL_ERROR` | 1 | Valid shells: `bash`, `zsh`, `fish`, `powershell`. |
| `unknown command` error | -- | -- | Run `carto --help` to see all 18 valid commands. |
| `--api-key is required in non-interactive mode` from `init` | `CONFIG_ERROR` | 3 | Pass `--api-key` when using `--non-interactive`. |
| `failed to check for updates` from `upgrade` | `CONNECTION_ERROR` | 4 | Network issue reaching GitHub API. Check connectivity and proxy settings. |

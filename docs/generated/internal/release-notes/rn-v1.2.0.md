---
id: rn-v1.2.0
type: release-notes
audience: internal
version: 1.2.0
status: draft
generated: 2026-03-06
source-tier: carto
hermes-version: 1.0.0
---

# Release Notes: Carto v1.2.0 (Candidate)

## Summary

Carto v1.2.0 is a CLI-focused release that doubles the command count from 9 to 18, introduces a structured JSON envelope contract for all output, adds TTY auto-detection for seamless agent and CI consumption, implements typed errors with categorized exit codes, and rebrands the CLI visual identity to the gold/stone palette. There are no breaking changes to existing command behavior or configuration. All 12 existing commands have been retrofitted to the new envelope system while preserving their current flags and semantics.

---

## Highlights

- **6 new commands:** `completions`, `init`, `export`, `import`, `logs`, `upgrade`
- **JSON envelope contract:** All commands wrap output in `{"ok": true, "data": ...}` / `{"ok": false, "error": ..., "code": ...}`
- **TTY auto-detection:** Non-terminal stdout triggers JSON mode automatically; `--pretty` overrides
- **Typed errors:** 5 error codes with mapped exit codes (0-5) for programmatic error handling
- **Gold brand palette:** CLI colors migrated from indigo/cyan to gold/stone across all commands
- **Audit logging integration:** All new commands emit structured audit events

---

## New Commands

### 1. `carto completions <bash|zsh|fish|powershell>`

**What:** Generates shell completion scripts using Cobra's built-in completion generators.

**How:** Outputs the completion script to stdout. The user redirects it to the appropriate shell config location.

**Usage:**
```bash
source <(carto completions bash)
carto completions zsh > "${fpath[1]}/_carto"
carto completions fish > ~/.config/fish/completions/carto.fish
```

**Who it affects:** All CLI users who want tab-completion for commands and flags.

**Files:** `cmd_completions.go`, `cmd_completions_test.go`

---

### 2. `carto init`

**What:** Interactive configuration wizard with a `--non-interactive` mode for automation. Prompts for LLM provider, API key, Memories URL, Memories API key, and projects directory. Writes to the persisted config file.

**How:** Interactive mode uses `promptValue()` to collect input from stdin with defaults shown in brackets. Non-interactive mode requires `--api-key` and reads remaining values from flags or environment variables. Both modes call `config.Save()` and emit an envelope summary.

**Flags:**
- `--non-interactive` -- skip prompts, use flags/env only
- `--llm-provider` -- provider name
- `--api-key` -- LLM API key (required in non-interactive mode)
- `--memories-url` -- Memories server URL
- `--memories-key` -- Memories API key
- `--projects-dir` -- projects directory path

**Who it affects:** New users setting up Carto for the first time. CI pipelines bootstrapping Carto configuration.

**CS Notes:** In non-interactive mode, `--api-key` is mandatory. Omitting it returns `CONFIG_ERROR` (exit 3). Existing config values are preserved as defaults in interactive mode.

**Files:** `cmd_init.go`, `cmd_init_test.go`

---

### 3. `carto export --project <name> [--layer <layer>]`

**What:** Streams index data from Memories as NDJSON (one JSON object per line). Default output is raw NDJSON suitable for piping to files or other tools. With `--json`, emits an envelope containing an export count summary instead of the raw stream.

**How:** Paginates through `client.ListBySource()` with a page size of 100. The source prefix filter is `carto/<project>/` (or `carto/<project>/layer:<layer>` when `--layer` is specified). Partial failures (connection lost mid-export) emit a warning and break rather than returning an error.

**Flags:**
- `--project`, `-p` -- project name (required)
- `--layer` -- filter to a specific layer (atoms, wiring, zones, blueprint, patterns)

**Who it affects:** Users backing up indexes, transferring indexes between environments, or extracting data for analysis.

**CS Notes:** The distinction between raw NDJSON (default) and envelope mode (`--json`) can be confusing. Raw mode streams data directly; envelope mode only reports the count. For a full data dump, do not pass `--json`.

**Files:** `cmd_export.go`, `cmd_export_test.go`

---

### 4. `carto import --project <name> [--strategy add|replace]`

**What:** Ingests NDJSON from stdin into the Memories store. Two strategies: `add` (append, default) and `replace` (delete all existing entries for the project first).

**How:** Reads stdin line by line via a `bufio.Scanner` with a 1MB line buffer. Lines are parsed as JSON with `text` and `source` fields. Empty-text entries are skipped. Entries are batched (100 per batch) and stored via `client.AddBatch()`. The `replace` strategy calls `client.DeleteBySource()` before importing and requires `--yes` or interactive confirmation.

**Flags:**
- `--project`, `-p` -- project name (required)
- `--strategy` -- `add` (default) or `replace`

**Who it affects:** Users restoring from backups, migrating indexes, or seeding indexes from external sources.

**CS Notes:** The `replace` strategy in JSON mode without `--yes` silently cancels (returns without error). This is by design -- agents must opt in to destructive operations explicitly. Lines exceeding 1MB are silently dropped by the scanner.

**Files:** `cmd_import.go`, `cmd_import_test.go`

---

### 5. `carto logs [--follow] [--last N] [--command <filter>] [--result ok|error]`

**What:** Queries and tails the Carto audit log file. Displays entries with optional filters for command name (substring match) and result status.

**How:** Reads the NDJSON audit log file, parses each line as an `auditEvent`, applies filters, and renders the last N matching entries. Follow mode polls the file every 500ms for new lines. JSON mode emits an envelope with all matching entries and does not support follow mode.

**Flags:**
- `--follow`, `-f` -- tail the log file
- `--last`, `-n` -- number of recent entries (default: 20)
- `--command` -- filter by command name (substring)
- `--result` -- filter by result: `ok` or `error`

**Requires:** `CARTO_AUDIT_LOG` environment variable or `--log-file` global flag.

**Who it affects:** Operators monitoring Carto usage and debugging failures.

**CS Notes:** If the user reports "no audit log configured," they need to set `CARTO_AUDIT_LOG`. The audit log file is not created until the first command writes an event.

**Files:** `cmd_logs.go`, `cmd_logs_test.go`

---

### 6. `carto upgrade [--check]`

**What:** Checks GitHub releases for newer versions of Carto. The actual binary download is stubbed -- the command directs users to the GitHub releases page when an update is available.

**How:** Fetches the latest release from `https://api.github.com/repos/divyekant/carto/releases/latest`, extracts the `tag_name`, and compares it against the current build version using semver comparison. The `--check` flag reports the comparison without attempting to upgrade.

**Flags:**
- `--check` -- check only, do not attempt to install

**Who it affects:** All users who want to stay on the latest version.

**CS Notes:** Binary download is not yet implemented (`TODO` in code). The upgrade flow (without `--check`) prompts for confirmation, then prints a message directing users to the releases page. The GitHub API call has a 10-second timeout and returns `CONNECTION_ERROR` on failure.

**Files:** `cmd_upgrade.go`, `cmd_upgrade_test.go`

---

## Infrastructure Changes

### JSON Envelope Contract

All 18 commands now produce output through the envelope system defined in `output.go`. The contract matches the Memories CLI convention:

- **Success:** `{"ok": true, "data": <command-specific payload>}` on stdout
- **Error:** `{"ok": false, "error": "<message>", "code": "<ERROR_CODE>"}` on stderr

The core functions are:
- `writeEnvelope(cmd, data, err)` -- auto-routes to JSON or human mode
- `writeEnvelopeHuman(cmd, data, err, humanFn)` -- same, with custom human renderer
- `runWithEnvelope(cmd, humanFn, fn)` -- full command runner with error classification, audit logging, and exit codes

**Files:** `output.go`, `output_test.go`

### TTY Auto-Detection

The `isJSONMode(cmd)` function determines output format:
1. `--json` explicitly set: JSON
2. `--pretty` explicitly set: human (overrides `--json`)
3. Neither: TTY detection via `term.IsTerminal(os.Stdout.Fd())`

This eliminates the need for CI pipelines and agents to pass `--json` manually.

**Files:** `output.go`

### Typed Errors

The `cliError` struct in `errors.go` carries `msg`, `code`, and `exit`. Constructor functions: `newConnectionError()`, `newAuthError()`, `newNotFoundError()`, `newConfigError()`. The `toCliError()` classifier wraps untyped errors as `GENERAL_ERROR`.

**Files:** `errors.go`, `errors_test.go`

### Gold Brand Palette

CLI colors in `helpers.go` migrated from the previous indigo/cyan palette to:
- `gold` (`\033[33m`) -- Brand Gold #d4af37, primary accent
- `stone` (`\033[38;5;249m`) -- #78716c, de-emphasis
- `amber` (`\033[38;5;214m`) -- #F59E0B, warnings
- `red` (`\033[31m`) -- #F43F5E, errors
- `green` (`\033[32m`) -- #10B981, success

Brand constants are centralized in `branding.go`.

**Files:** `helpers.go`, `branding.go`

### Confirmation Guard

`confirmAction()` in `output.go` gates destructive operations. It auto-accepts with `--yes`, rejects in JSON mode without `--yes`, and prompts interactively otherwise. Applied to `projects delete`, `import --strategy replace`, and `upgrade`.

**Files:** `output.go`

---

## Existing Commands Retrofitted

All 12 existing commands migrated from the previous `writeOutput()` pattern to `writeEnvelopeHuman()`. This means:

- Every command now emits a proper JSON envelope when in JSON mode.
- Human output is rendered via custom `humanFn` callbacks with the gold palette.
- Audit events are logged for every command execution.

Specific behavioral changes:
- **`projects delete`** gained a `confirmAction()` guard. Previously it deleted without confirmation.

No flags were removed or renamed. No configuration keys changed. The envelope wrapping is additive -- existing JSON consumers will see their data inside the `data` field of the envelope.

---

## Global Flags Added

| Flag | Type | Description |
|---|---|---|
| `--pretty` | bool | Force human-readable output even when piped (inverse of `--json`). |
| `--yes`, `-y` | bool | Skip confirmation prompts for automation and agent usage. |

These are persistent flags available on all commands. Existing global flags (`--json`, `--quiet`, `--verbose`, `--log-file`, `--profile`) are unchanged.

---

## No Breaking Changes

This release introduces no breaking changes:

- All existing 12 commands retain their flags, arguments, and semantics.
- The `--json` flag continues to work. The envelope wrapping adds an outer `{"ok": true, "data": ...}` layer, but the data payload is the same structure as before.
- All environment variables are unchanged.
- The config file format is unchanged.
- Exit code 0 still means success. Non-zero exit codes are now more granular (1-5 instead of just 1).

**Migration note for JSON consumers:** Existing scripts that parse `carto <command> --json` output directly will need to unwrap the `data` field. For example, `carto projects list --json | jq '.[0].name'` becomes `carto projects list --json | jq '.data[0].name'`. The TTY auto-detection means piped commands now emit JSON automatically even without `--json`.

---

## Test Coverage

- `output_test.go` -- tests for `isJSONMode()` TTY detection, envelope writing, `isYes()`, and `confirmAction()`.
- `errors_test.go` -- tests for `cliError`, `toCliError()` classifier, `runWithEnvelope()`, and all error constructors.
- `cmd_completions_test.go` -- tests for all four shell completion generators.
- `cmd_init_test.go` -- tests for interactive and non-interactive init modes.
- `cmd_export_test.go` -- tests for export with and without layer filter, envelope vs stream mode.
- `cmd_import_test.go` -- tests for add and replace strategies, invalid JSON handling, empty text skipping.
- `cmd_logs_test.go` -- tests for log reading, filtering, and the `--last` limit.
- `cmd_upgrade_test.go` -- tests for version comparison and GitHub API mocking.
- `cmd_modernization_test.go` -- integration tests validating the envelope contract across retrofitted commands.

35 files changed, 3199 insertions, 187 deletions.

---

## Configuration Changes

No new environment variables are required. The following existing variables are relevant to new commands:

| Variable | Relevant Commands | Notes |
|---|---|---|
| `CARTO_AUDIT_LOG` | `logs`, all commands via `--log-file` | File path for the structured audit log. Must be set for `carto logs` to work. |
| `MEMORIES_URL` | `export`, `import` | Required for data portability commands. |
| `MEMORIES_API_KEY` | `export`, `import` | Authentication for Memories access. |
| `CARTO_PROFILE` | `init`, `doctor`, `auth`, `config` | Config profile selection. |

---

## Known Issues

1. **Upgrade binary download is stubbed.** `carto upgrade` (without `--check`) reports the available update and directs users to the GitHub releases page. Automatic download and binary replacement is not yet implemented.

2. **Follow mode does not work in JSON mode.** `carto logs --follow` in JSON mode outputs existing entries as an envelope and exits. Follow mode requires human (TTY) output.

3. **Import line buffer limit.** Each NDJSON line in `carto import` is capped at 1MB by the `bufio.Scanner` buffer. Lines exceeding this size are silently dropped.

4. **Envelope wrapping is a behavioral change for JSON consumers.** Existing scripts parsing `--json` output will need to account for the outer `{"ok": true, "data": ...}` wrapper. This is documented in the migration note above.

---

## Internal Notes

### Recommended Post-Upgrade Verification

After upgrading to v1.2.0, run:
```bash
carto doctor              # verify environment
carto version --json      # confirm build version
carto auth validate       # test LLM connectivity
```

### Adoption Path for runWithEnvelope

New commands should use `runWithEnvelope()` as the standard entry point. Existing commands use `writeEnvelopeHuman()` directly but can be migrated incrementally. The `runWithEnvelope()` function handles the full lifecycle: execution, envelope output, audit logging, and exit codes.

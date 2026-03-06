# CLI Overhaul Design

**Date:** 2026-03-06
**Status:** Approved
**Approach:** Layer-by-Layer (Foundation → Brand → Retrofits → New Commands → Tests)

---

## Goals

1. **Agent-usability** — TTY auto-detection, JSON envelope contract, typed exit codes, stdin piping, `--yes` flag
2. **Brand sync** — Gold/stone palette across all CLI output (matching web UI)
3. **New commands** — `init`, `completions`, `export`, `import`, `logs`, `upgrade`
4. **Test coverage** — Fill gaps, ensure every command has success/error/envelope tests
5. **Backward compatible** — No breaking changes to existing behavior

## Design Decisions

- **JSON envelope matches Memories CLI** — `{"ok": true, "data": ...}` / `{"ok": false, "error": ..., "code": ...}` for consistency across the toolchain
- **TTY auto-detection** — Agents get JSON automatically when piping; humans get colored output in terminals
- **`--json` still works** — Explicit flag overrides auto-detection (backward compatible)
- **`--pretty` added** — Force human output when piped (inverse of `--json`)
- **Gold ANSI primary** — `cyan` → `gold` (yellow ANSI), `yellow` warnings → `amber` (256-color), `dimmed` → `stone` (256-color)

---

## Layer 1: Agent-Usability Foundation

### JSON Envelope Contract

All commands output this structure in JSON mode:

```json
{"ok": true, "data": { /* command-specific payload */ }}

{"ok": false, "error": "human-readable message", "code": "ERROR_CODE"}
```

### Error Codes & Exit Codes

| Code | Exit Code | When |
|------|-----------|------|
| — | 0 | Success |
| `GENERAL_ERROR` | 1 | Unhandled / unexpected errors |
| `NOT_FOUND` | 2 | Project/resource doesn't exist |
| `CONNECTION_ERROR` | 3 | Can't reach Memories or LLM |
| `AUTH_FAILURE` | 4 | Bad/missing API key |
| `CONFIG_ERROR` | 5 | Invalid or missing config |

### TTY Auto-Detection

```go
func isJSONMode(cmd *cobra.Command) bool {
    if flagSet, _ := cmd.Flags().GetBool("json"); flagSet {
        return true
    }
    if flagSet, _ := cmd.Flags().GetBool("pretty"); flagSet {
        return false
    }
    return !term.IsTerminal(int(os.Stdout.Fd()))
}
```

### Centralized Error Handler

```go
func runWithEnvelope(cmd *cobra.Command, fn func() (any, error)) {
    data, err := fn()
    if err != nil {
        code, exitCode := classifyError(err)
        writeEnvelope(cmd, nil, err.Error(), code)
        os.Exit(exitCode)
    }
    writeEnvelope(cmd, data, "", "")
}
```

Typed errors (`ErrConnection`, `ErrAuth`, `ErrNotFound`, `ErrConfig`) mapped by `classifyError()`.

### Stdin Support

```go
func readInputOrStdin(arg string) ([]byte, error) {
    if arg == "-" || !term.IsTerminal(int(os.Stdin.Fd())) {
        return io.ReadAll(os.Stdin)
    }
    return []byte(arg), nil
}
```

### New Global Flags

- `--pretty` — force human-readable output even when piped
- `--yes` / `-y` — skip confirmation prompts (for automation)

### New Files

- `output.go` — `isJSONMode()`, `writeEnvelope()`, TTY detection, `readInputOrStdin()`
- `errors.go` — typed errors, `classifyError()`, `runWithEnvelope()`

---

## Layer 2: Gold Brand

### ANSI Color Palette

```go
const (
    bold   = "\033[1m"
    gold   = "\033[33m"        // brand gold #d4af37 — primary accent
    green  = "\033[32m"        // success #10B981
    amber  = "\033[38;5;214m"  // warnings #F59E0B — distinct from gold
    red    = "\033[31m"        // errors #F43F5E
    stone  = "\033[38;5;249m"  // de-emphasis — warm neutral
    reset  = "\033[0m"
)
```

### Brand Mapping

| Role | Old | New |
|------|-----|-----|
| Primary actions, headers | cyan (indigo #5B50F5) | gold (gold #d4af37) |
| Success | green | green (unchanged) |
| Warnings | yellow (amber) | amber (256-color, distinct from gold) |
| Errors | red | red (unchanged) |
| De-emphasis | dimmed | stone (256-color warm neutral) |
| Info/Teal | green (teal #0EA5A0) | Removed |

### Changes

- `helpers.go` — color constants, `dimmed()` → `stone()` helper
- `branding.go` — hex codes, role descriptions
- `cmd_about.go` — gold palette color table
- All output across all commands shifts from cyan → gold

---

## Layer 3: Existing Command Retrofits

Every existing command migrated from `writeOutput` → `runWithEnvelope` + `writeEnvelope`.

| Command | Envelope Data | Error Mapping | Other |
|---------|--------------|---------------|-------|
| `index` | `{modules, files, atoms, errors, elapsed}` | connection→3, auth→4 | — |
| `query` | `{results: [...]}` | not-found→2, connection→3 | stdin support for question |
| `modules` | `{modules: [...], total: N}` | — | — |
| `patterns` | `{files_written: [...]}` | — | — |
| `status` | `{project, files, size, indexed_at}` | missing-index→2 | — |
| `serve` | startup errors through envelope | — | runtime stays as-is |
| `projects list` | `{projects: [...]}` | — | — |
| `projects show` | project object | not-found→2 | — |
| `projects delete` | `{deleted: name}` | not-found→2 | add `--yes` confirmation |
| `sources *` | per-subcommand | not-found→2 | — |
| `auth status` | `{credentials: [...]}` | — | — |
| `auth set-key` | `{provider, stored: true}` | — | — |
| `auth validate` | `{provider, status, latency_ms}` | connection→3, auth→4 | — |
| `config get` | `{settings: {key: {value, source}}}` | — | source attribution added |
| `config set` | `{key, value, stored: true}` | — | — |
| `config validate` | `{valid: bool, errors: [...]}` | config→5 | — |
| `config path` | `{dir, file, active, exists}` | — | — |
| `doctor` | `{checks: [...], failures, warnings}` | — | — |
| `version` | `{version, go_version, os, arch}` | — | — |
| `about` | `{name, version, tagline, ...}` | — | gold palette |

### Config Source Attribution

`config get` gains per-value source tracking:

```json
{
  "ok": true,
  "data": {
    "settings": {
      "llm_provider": {"value": "anthropic", "source": "env"},
      "memories_url": {"value": "http://localhost:8900", "source": "default"}
    }
  }
}
```

Human mode: `llm_provider    anthropic    (from env)`

### Backward Compatibility

- `--json` flag still works explicitly (now redundant with auto-detection, but not removed)
- Exit codes 0-5 unchanged in meaning
- Human output format unchanged (just gold instead of cyan)
- `writeOutput` removed only after all commands migrated

---

## Layer 4: New Commands

### `carto init`

Setup wizard for first-time users.

```bash
# Human (interactive)
carto init

# Agent (non-interactive)
carto init --non-interactive --memories-url http://localhost:8900 --api-key sk-ant-... --projects-dir ~/projects
```

**Wizard steps:**
1. Check existing config → offer reconfigure or skip
2. Prompt LLM provider + API key → validate connectivity
3. Prompt Memories URL + key → validate health
4. Prompt projects directory
5. Write config → display doctor summary

**Non-interactive:** All values from flags/env. Fails with `CONFIG_ERROR` if required values missing.

**Envelope:** `{"ok": true, "data": {"config_written": "/path/to/config.json", "checks": [...]}}`

### `carto completions`

Shell completion scripts via Cobra built-in.

```bash
carto completions <bash|zsh|fish|powershell>
```

Outputs completion script to stdout for sourcing.

### `carto export`

Export index data as NDJSON.

```bash
# All layers
carto export --project myapp > backup.ndjson

# Specific layer
carto export --project myapp --layer atoms

# Pipe
carto export --project myapp | jq '.text'
```

**Flags:** `--project` (required), `--layer` (optional filter).
**Default:** Raw NDJSON streaming to stdout (no envelope — same as Memories export).
**With `--json`:** Envelope with `{"exported": 142, "project": "myapp"}`.

### `carto import`

Import NDJSON index data.

```bash
# From stdin
cat backup.ndjson | carto import --project myapp

# With strategy
carto import --project myapp --strategy replace --yes < data.ndjson
```

**Flags:** `--project` (required), `--strategy` (`add`|`replace`, default: `add`), `--yes`.
**`replace` strategy:** Deletes existing index first. Requires confirmation or `--yes`.

**Envelope:** `{"ok": true, "data": {"imported": 142, "project": "myapp", "strategy": "add"}}`

### `carto logs`

Query or tail the audit log.

```bash
carto logs --follow
carto logs --last 20
carto logs --command index
carto logs --result error
```

**Flags:** `--follow`/`-f`, `--last`/`-n` (int), `--command` (string filter), `--result` (`ok`|`error`).
**Error:** `CONFIG_ERROR` if no audit log configured.

**Envelope:** `{"ok": true, "data": {"entries": [...], "total": N}}`

### `carto upgrade`

Check for and install newer versions.

```bash
# Check only
carto upgrade --check

# Upgrade
carto upgrade --yes
```

**Check:** Queries GitHub releases API, compares with current version.
**Upgrade:** Downloads binary for OS/arch, verifies checksum, replaces in-place. Confirmation required unless `--yes`.

**Envelope:** `{"ok": true, "data": {"current": "1.1.0", "latest": "1.2.0", "update_available": true}}`

---

## Layer 5: Test Coverage

### Foundation Tests

- `TestIsJSONMode` — explicit `--json`, explicit `--pretty`, TTY fallback
- `TestWriteEnvelope` — success and error envelopes
- `TestClassifyError` — typed errors → correct codes + exit codes
- `TestReadInputOrStdin` — argument vs stdin
- `TestGoldColors` — color constants

### Test Helpers

- `execCmdJSON(t, cmd, args)` — runs command, unmarshals envelope, returns typed result
- `assertEnvelopeOK(t, output)` — verify `ok: true` + data present
- `assertEnvelopeError(t, output, code)` — verify `ok: false` + correct error code

### Per-Command Coverage

Every command gets:
- Success path with human output
- Success path with JSON envelope
- Error path with correct error code and exit code
- TTY auto-detection verification (for key commands)

| Command | Existing Tests | Tests to Add |
|---------|---------------|--------------|
| `index` | 1 | connection→3, auth→4, success envelope |
| `query` | 0 | success, not-found, stdin, tier validation |
| `modules` | 1 | envelope, empty dir |
| `patterns` | 1 | envelope, missing project |
| `status` | 1 | envelope, success |
| `serve` | 0 | startup validation |
| `projects *` | 0 | list/show/delete, `--yes`, not-found |
| `sources *` | 0 | list/set/rm, not-found |
| `auth *` | 7 | envelope format |
| `config *` | 18 | source attribution, envelope |
| `doctor` | 5 | envelope format |
| `version` | 2 | envelope format |
| `about` | 0 | gold palette, envelope |
| `init` | — | interactive mock, non-interactive, validation |
| `completions` | — | bash/zsh/fish output |
| `export` | — | NDJSON format, layer filter, empty project |
| `import` | — | add/replace strategy, stdin, `--yes` |
| `logs` | — | filters, no-log error |
| `upgrade` | — | check-only, version comparison |

---

## File Changes Summary

### New Files (Layer 1)
- `go/cmd/carto/output.go` — envelope, TTY detection, stdin helper
- `go/cmd/carto/errors.go` — typed errors, classifier, runWithEnvelope

### New Files (Layer 4)
- `go/cmd/carto/cmd_init.go`
- `go/cmd/carto/cmd_completions.go`
- `go/cmd/carto/cmd_export.go`
- `go/cmd/carto/cmd_import.go`
- `go/cmd/carto/cmd_logs.go`
- `go/cmd/carto/cmd_upgrade.go`

### Modified Files
- `go/cmd/carto/helpers.go` — gold colors, remove old writeOutput after migration
- `go/cmd/carto/branding.go` — gold palette constants
- `go/cmd/carto/main.go` — new global flags (`--pretty`, `--yes`), register new commands
- All `cmd_*.go` files — migrate to runWithEnvelope
- `go/cmd/carto/main_test.go` — new test helpers
- `go/cmd/carto/cmd_modernization_test.go` — retrofit tests + new command tests

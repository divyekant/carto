---
id: ts-006
type: troubleshooting
audience: internal
topic: CLI Issues
status: draft
generated: 2026-03-06
source-tier: carto
hermes-version: 1.0.0
---

# Troubleshooting: CLI Issues

**ID:** ts-006
**Topic:** CLI Issues
**Components:** `cmd/carto/` -- `output.go`, `errors.go`, `helpers.go`, all `cmd_*.go` files

---

## Error Envelope Format

All CLI errors in JSON mode follow this structure:

```json
{
  "ok": false,
  "error": "human-readable error message",
  "code": "ERROR_CODE"
}
```

Error envelopes are written to stderr. The process exit code corresponds to the error category. Use both the `code` field and the exit code for programmatic error handling.

| Error Code | Exit Code | Category |
|---|---|---|
| `GENERAL_ERROR` | 1 | Unhandled or unclassified errors |
| `NOT_FOUND` | 2 | Project, resource, or file does not exist |
| `CONFIG_ERROR` | 3 | Missing or invalid configuration |
| `CONNECTION_ERROR` | 4 | Cannot reach Memories or LLM provider |
| `AUTH_FAILURE` | 5 | Bad or missing API key |

---

## Symptom: `command not found: carto`

**Cause:** The `carto` binary has not been built, or the built binary is not in `$PATH`.

**Resolution:**

1. Build the binary:
   ```bash
   go build -o carto ./cmd/carto
   ```
   CGO must be enabled (it is by default). On Alpine Linux, install `gcc` and `musl-dev` first.

2. Either run it directly (`./carto`) or move it to a directory in `$PATH`:
   ```bash
   mv carto /usr/local/bin/
   ```

3. Verify:
   ```bash
   carto --version
   ```

---

## Symptom: `CONFIG_ERROR` -- Missing API Key

**Error envelope:**
```json
{"ok": false, "error": "No API key found for provider anthropic", "code": "CONFIG_ERROR"}
```

**Exit code:** 3

**Cause:** Neither `LLM_API_KEY` nor `ANTHROPIC_API_KEY` is set, and the provider is not `ollama`.

**Resolution:**

1. Set the key via environment variable:
   ```bash
   export LLM_API_KEY="sk-ant-..."
   ```

2. Or store it in the config file:
   ```bash
   carto auth set-key anthropic sk-ant-...
   ```

3. Or run the init wizard:
   ```bash
   carto init
   ```

4. Verify with:
   ```bash
   carto auth status
   carto doctor
   ```

**Note:** Commands that do not call the LLM (e.g., `status`, `modules`, `query`, `export`) may work without an API key if the Memories server already contains indexed data.

---

## Symptom: `CONFIG_ERROR` -- `--api-key is required in non-interactive mode`

**Error envelope:**
```json
{"ok": false, "error": "--api-key is required in non-interactive mode", "code": "CONFIG_ERROR"}
```

**Exit code:** 3

**Cause:** `carto init --non-interactive` was called without the `--api-key` flag.

**Resolution:**

Provide the API key via the flag:
```bash
carto init --non-interactive --api-key "sk-ant-..." --llm-provider anthropic
```

---

## Symptom: `CONFIG_ERROR` -- No Audit Log Configured

**Error envelope:**
```json
{"ok": false, "error": "no audit log configured (set CARTO_AUDIT_LOG or use --log-file)", "code": "CONFIG_ERROR"}
```

**Exit code:** 3

**Cause:** `carto logs` was run without an audit log path configured.

**Resolution:**

Set the audit log path:
```bash
export CARTO_AUDIT_LOG=~/.carto/audit.log
```

Or pass it per-invocation:
```bash
carto logs --log-file ~/.carto/audit.log
```

The audit log file is created automatically when any command writes an audit event. To start generating audit events for other commands, set `CARTO_AUDIT_LOG` or pass `--log-file` globally.

---

## Symptom: `CONFIG_ERROR` -- Invalid Import Strategy

**Error envelope:**
```json
{"ok": false, "error": "invalid strategy: foo (use add or replace)", "code": "CONFIG_ERROR"}
```

**Exit code:** 3

**Cause:** An invalid value was passed to `carto import --strategy`.

**Resolution:**

Use one of the two valid strategies:
```bash
cat data.ndjson | carto import --project myapp --strategy add
cat data.ndjson | carto import --project myapp --strategy replace --yes
```

---

## Symptom: `CONFIG_ERROR` -- Memories URL Not Configured

**Error envelope:**
```json
{"ok": false, "error": "memories URL not configured (set MEMORIES_URL or run carto init)", "code": "CONFIG_ERROR"}
```

**Exit code:** 3

**Cause:** `MEMORIES_URL` is not set and no Memories URL is in the config file. This affects `export` and `import` commands.

**Resolution:**

```bash
export MEMORIES_URL="http://localhost:8900"
```

Or run `carto init` to configure it persistently.

---

## Symptom: `CONNECTION_ERROR` -- Cannot Reach Memories

**Error envelope:**
```json
{"ok": false, "error": "failed to connect to Memories: connection refused", "code": "CONNECTION_ERROR"}
```

**Exit code:** 4

**Cause:** The Memories server is not running, or `MEMORIES_URL` points to the wrong address.

**Resolution:**

1. Check that Memories is running:
   ```bash
   curl -s http://localhost:8900/health
   ```

2. If using Docker Compose:
   ```bash
   docker compose up memories
   ```

3. Verify `MEMORIES_URL` matches the running server:
   ```bash
   echo $MEMORIES_URL
   carto config get memories_url
   ```

4. If Memories is on a non-default port:
   ```bash
   carto config set memories_url http://localhost:9100
   ```

---

## Symptom: `CONNECTION_ERROR` -- Failed to Check for Updates

**Error envelope:**
```json
{"ok": false, "error": "failed to check for updates: ...", "code": "CONNECTION_ERROR"}
```

**Exit code:** 4

**Cause:** `carto upgrade` could not reach the GitHub API to check for new releases. This can be caused by network issues, proxy configuration, or corporate firewalls.

**Resolution:**

1. Check network connectivity:
   ```bash
   curl -s https://api.github.com/repos/divyekant/carto/releases/latest | jq '.tag_name'
   ```

2. If behind a proxy, ensure `HTTPS_PROXY` is set.

3. If behind a corporate firewall (e.g., Cloudflare WARP), verify that `api.github.com` is reachable.

---

## Symptom: `CONNECTION_ERROR` -- Import Batch Failure

**Error envelope:**
```json
{"ok": false, "error": "failed to store batch: ...", "code": "CONNECTION_ERROR"}
```

**Exit code:** 4

**Cause:** The Memories server became unreachable during a `carto import` operation.

**Resolution:**

1. Verify Memories is still running.
2. Re-run the import. With `--strategy add`, duplicate entries may be created for already-imported records. With `--strategy replace --yes`, the existing data is deleted first, so a partial import leaves the index incomplete.
3. For large imports, consider splitting the NDJSON file and importing in batches.

---

## Symptom: `NOT_FOUND` -- Audit Log File Not Found

**Error envelope:**
```json
{"ok": false, "error": "audit log file not found: /path/to/audit.log", "code": "NOT_FOUND"}
```

**Exit code:** 2

**Cause:** `carto logs` was pointed at an audit log file that does not exist on disk.

**Resolution:**

The audit log file is created on first write. Run any command with audit logging enabled to create it:
```bash
carto version --log-file /path/to/audit.log
carto logs --log-file /path/to/audit.log
```

---

## Symptom: `NOT_FOUND` -- Project Not Found

**Error envelope:**
```json
{"ok": false, "error": "project \"foo\" not found or has no index", "code": "NOT_FOUND"}
```

**Exit code:** 2

**Cause:** The project name does not match any indexed project, or the project has no `.carto` manifest.

**Resolution:**

1. List available projects:
   ```bash
   carto projects list
   ```

2. Verify the project has been indexed:
   ```bash
   carto status --project <name>
   ```

3. If the project has not been indexed, run:
   ```bash
   carto index --project <name>
   ```

---

## Symptom: `AUTH_FAILURE` -- Authentication Failed

**Exit code:** 5

**Cause:** The API key provided to the LLM provider was rejected (HTTP 401 or 403).

**Resolution:**

1. Check the current key status:
   ```bash
   carto auth status
   ```

2. Test connectivity explicitly:
   ```bash
   carto auth validate
   ```

3. Set a new key:
   ```bash
   carto auth set-key anthropic sk-ant-new-key-...
   ```

---

## Symptom: JSON Output Appears Malformed

**Cause:** Confusion between the envelope format and raw NDJSON, or mixing stdout and stderr.

**Resolution:**

1. Understand the two output modes:
   - Most commands emit a **single JSON envelope** to stdout: `{"ok": true, "data": ...}`
   - `carto export` (without `--json`) emits **raw NDJSON** to stdout (one line per entry), not an envelope.
   - Error envelopes go to **stderr**, not stdout.

2. Separate stdout and stderr:
   ```bash
   carto index --project myapp 2>errors.json >output.json
   ```

3. For `carto export`, use `--json` to get an envelope summary instead of raw NDJSON:
   ```bash
   carto export --project myapp --json | jq '.data.exported'
   ```

4. When consuming NDJSON from `export`, parse line by line:
   ```bash
   carto export --project myapp | while read -r line; do echo "$line" | jq .; done
   ```

---

## Symptom: Destructive Operation Silently Cancelled

**Cause:** `confirmAction()` was called in JSON mode without `--yes`. In non-TTY JSON mode, `confirmAction()` returns false to prevent accidental destructive operations by agents.

**Affected commands:** `projects delete`, `import --strategy replace`, `upgrade`.

**Resolution:**

Pass `--yes` explicitly:
```bash
carto projects delete myproject --yes
carto import --project myapp --strategy replace --yes
carto upgrade --yes
```

---

## Symptom: `address already in use` on `carto serve`

**Cause:** Port 8950 (or the specified port) is already bound by another process.

**Resolution:**

1. Find what is using the port:
   ```bash
   lsof -i :8950
   ```

2. Either stop the conflicting process or use a different port:
   ```bash
   carto serve --port 9000
   ```

---

## Symptom: `unknown command` Error

**Cause:** Typographical error or attempting to use a command that does not exist.

**Resolution:**

1. List all available commands:
   ```bash
   carto --help
   ```

2. The 18 valid commands are: `index`, `query`, `modules`, `patterns`, `status`, `serve`, `projects`, `sources`, `config`, `auth`, `doctor`, `version`, `about`, `completions`, `init`, `export`, `import`, `logs`, `upgrade`.

3. Each command has its own help:
   ```bash
   carto index --help
   ```

4. Some commands have subcommands (e.g., `projects list`, `config get`, `auth status`).

---

## Symptom: `unsupported shell` from `carto completions`

**Cause:** An invalid shell name was passed.

**Resolution:**

Valid shells are: `bash`, `zsh`, `fish`, `powershell`.

```bash
carto completions bash
carto completions zsh
carto completions fish
carto completions powershell
```

---

## Symptom: Index Completes but Produces No Output

**Cause:** The project path contains no files that the scanner recognizes, or all files are excluded by `.gitignore` rules.

**Resolution:**

1. Check the project status:
   ```bash
   carto status --project myapp
   ```

2. Verify the project path exists and contains source code:
   ```bash
   carto projects show myapp
   ```

3. Ensure the project contains files in supported languages. The scanner uses Tree-sitter grammars and will skip unsupported file types (falling back to line-based chunking for recognized text files).

4. Check `.gitignore` rules. The scanner respects `.gitignore` and may be excluding files unintentionally.

---

## Symptom: `carto logs --follow` Does Not Show New Entries

**Cause:** The follow mode polls the file every 500ms. If the audit log path has changed between the command writing events and the logs command reading them, entries will not appear.

**Resolution:**

1. Verify the log file path matches:
   ```bash
   echo $CARTO_AUDIT_LOG
   ```

2. Ensure commands are writing to the same log file. Both `--log-file` and `CARTO_AUDIT_LOG` must resolve to the same absolute path.

3. Follow mode does not work in JSON mode. If output is piped, the command outputs existing entries as an envelope and exits.

---

## Quick Reference

| Symptom | Error Code | Exit Code | First Check |
|---|---|---|---|
| `command not found` | -- | -- | `go build -o carto ./cmd/carto` |
| Missing API key | `CONFIG_ERROR` | 3 | `carto auth status` |
| Connection refused | `CONNECTION_ERROR` | 4 | `curl $MEMORIES_URL/health` |
| Auth failed | `AUTH_FAILURE` | 5 | `carto auth validate` |
| Project not found | `NOT_FOUND` | 2 | `carto projects list` |
| No audit log | `CONFIG_ERROR` | 3 | `echo $CARTO_AUDIT_LOG` |
| Audit log missing | `NOT_FOUND` | 2 | Run any command with `--log-file` |
| Address in use | `GENERAL_ERROR` | 1 | `lsof -i :8950` |
| Unknown command | -- | -- | `carto --help` |
| JSON malformed | -- | -- | Separate `2>err.json >out.json` |
| Silent cancellation | -- | 0 | Add `--yes` flag |
| Upgrade check fails | `CONNECTION_ERROR` | 4 | Check network / proxy |
| Import batch fails | `CONNECTION_ERROR` | 4 | Verify Memories health |
| Init needs --api-key | `CONFIG_ERROR` | 3 | Add `--api-key` flag |

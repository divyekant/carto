---
type: error-reference
audience: external
status: draft
generated: 2026-03-06
source-tier: carto
hermes-version: 1.0.0
---

# Error Reference

When something goes wrong, Carto returns errors in a consistent format. This guide documents the most common errors you may encounter, explains what causes them, and tells you how to fix them.

## Error Formats

Carto uses two error formats depending on the output mode.

### Human Mode (Terminal)

When you run Carto in a terminal, errors are printed to stderr as colored text:

```
Error: memories server unreachable at http://localhost:8900
```

### JSON Mode (Piped or `--json`)

When output is piped or you pass `--json`, errors are returned as a structured JSON envelope:

```json
{
  "ok": false,
  "error": "memories server unreachable at http://localhost:8900",
  "code": "CONNECTION_ERROR"
}
```

The envelope always has three fields:

| Field | Type | Description |
|-------|------|-------------|
| `ok` | boolean | Always `false` for errors |
| `error` | string | Human-readable error message |
| `code` | string | Machine-readable error code (see table below) |

### API Errors (REST)

The REST API returns errors as JSON with an appropriate HTTP status code:

```json
{
  "error": "memories server unreachable at http://localhost:8900"
}
```

---

## CLI Error Codes

Every CLI error carries both a machine-readable error code (in the JSON `code` field) and a process exit code. You can use these for programmatic error handling in scripts and CI/CD pipelines.

| Code | Exit Code | Meaning |
|------|-----------|---------|
| `GENERAL_ERROR` | 1 | An unhandled or uncategorized error |
| `NOT_FOUND` | 2 | The requested resource (project, file, log) does not exist |
| `CONFIG_ERROR` | 3 | Invalid or missing configuration (bad flags, missing env vars) |
| `CONNECTION_ERROR` | 4 | Cannot reach a required service (Memories server, LLM API) |
| `AUTH_FAILURE` | 5 | Bad, missing, or expired API key |

### Using Error Codes in Scripts

You can check both the exit code and the JSON `code` field:

```bash
# Check exit code
carto status --project myapp
if [ $? -eq 4 ]; then
  echo "Memories server is down"
fi

# Check JSON error code
RESULT=$(carto status --project myapp 2>&1)
CODE=$(echo "$RESULT" | jq -r '.code // empty')
if [ "$CODE" = "CONNECTION_ERROR" ]; then
  echo "Cannot reach Memories server"
fi
```

### JSON Error Example

When you run a command that fails in JSON mode (piped or `--json`), the error envelope is written to stderr:

```bash
$ carto status --project nonexistent --json 2>&1
{
  "ok": false,
  "error": "project \"nonexistent\" has not been indexed",
  "code": "NOT_FOUND"
}
```

The process exits with code 2 (`NOT_FOUND`).

### Human Error Example

The same error in a terminal looks like:

```
Error: project "nonexistent" has not been indexed
```

The process still exits with code 2 so scripts that check `$?` work the same way regardless of output mode.

---

## Authentication Errors

These errors occur when Carto can't authenticate with an LLM provider or external service.

### Missing API Key

**Message:**
```
Error: no API key configured. Set LLM_API_KEY or ANTHROPIC_API_KEY
```

**Code:** `AUTH_FAILURE` (exit 5)

**Cause:** You haven't set an LLM API key, or the environment variable isn't reaching Carto.

**Resolution:**

1. Set your API key as an environment variable:
   ```bash
   export ANTHROPIC_API_KEY="sk-ant-api03-your-key-here"
   ```
2. Or store it via the CLI:
   ```bash
   carto auth set-key anthropic sk-ant-api03-your-key-here
   ```
3. Or run `carto init` to configure everything interactively.
4. Verify the variable is set: `echo $ANTHROPIC_API_KEY`

---

### Invalid API Key

**Message:**
```
Error: LLM API returned 401 Unauthorized: invalid API key
```

**Code:** `AUTH_FAILURE` (exit 5)

**Cause:** The API key you provided is incorrect, revoked, or belongs to a different provider than the one configured.

**Resolution:**

1. Double-check your API key for typos or extra whitespace.
2. Make sure the key matches your configured provider. For example, an Anthropic key won't work if `LLM_PROVIDER` is set to `openai`.
3. If you recently rotated your key, update it:
   ```bash
   carto auth set-key anthropic sk-ant-api03-new-key
   ```
4. Validate your credentials:
   ```bash
   carto auth validate
   ```

---

### Expired or Invalid OAuth Token

**Message:**
```
Error: source authentication failed for github: 401 Bad credentials
```

**Code:** `AUTH_FAILURE` (exit 5)

**Cause:** A token for an external source (GitHub, Jira, Linear, Notion, or Slack) is expired, revoked, or invalid.

**Resolution:**

1. Generate a new token from the relevant service's settings.
2. Update the corresponding credential:
   ```bash
   carto auth set-key github ghp_your-new-token
   ```
3. Ensure the token has the required scopes. For GitHub, Carto needs `repo` scope to read issues and pull requests.

---

### Wrong Provider Configuration

**Message:**
```
Error: LLM API returned 404: model "claude-haiku-4-5-20251001" not found
```

**Code:** `CONFIG_ERROR` (exit 3)

**Cause:** You've set a model name that doesn't exist for your configured provider. For example, using an Anthropic model name with the OpenAI provider.

**Resolution:**

1. Check that `LLM_PROVIDER` matches your API key and model names.
2. Make sure `CARTO_FAST_MODEL` and `CARTO_DEEP_MODEL` are valid model IDs for your provider.
3. See the [Configuration Reference](config-reference.md) for provider-specific examples.

---

## Connection Errors

These errors occur when Carto can't reach a server it depends on.

### Memories Server Unreachable

**Message:**
```
Error: memories server unreachable at http://localhost:8900
```

**Code:** `CONNECTION_ERROR` (exit 4)

**Cause:** The Memories server isn't running, or Carto is configured to connect to the wrong address.

**Resolution:**

1. Start the Memories server:
   ```bash
   # If using Docker:
   docker run -p 8900:8900 memories-server
   ```
2. Verify it's running: `curl http://localhost:8900/health`
3. If you're running Memories on a different port or host, update `MEMORIES_URL`:
   ```bash
   export MEMORIES_URL="http://your-host:8900"
   ```
4. Run `carto doctor` to check all connectivity at once.

---

### LLM API Timeout

**Message:**
```
Error: LLM API request timed out after 120s
```

**Code:** `CONNECTION_ERROR` (exit 4)

**Cause:** The LLM provider took too long to respond. This can happen with very large code chunks, slow network connections, or provider-side issues.

**Resolution:**

1. Retry the command -- this is often a transient issue.
2. If indexing large codebases, try lowering `CARTO_MAX_CONCURRENT` to reduce load:
   ```bash
   export CARTO_MAX_CONCURRENT=3
   ```
3. Check the provider's status page for outages.
4. If using Ollama locally, make sure the model is fully loaded and your machine has enough resources.

---

### LLM API Rate Limit

**Message:**
```
Error: LLM API rate limit exceeded. Retry after 30s
```

**Code:** `CONNECTION_ERROR` (exit 4)

**Cause:** You've sent too many requests to the LLM provider in a short period. This is common when indexing large codebases.

**Resolution:**

1. Wait the suggested time and retry.
2. Lower `CARTO_MAX_CONCURRENT` to reduce the request rate:
   ```bash
   export CARTO_MAX_CONCURRENT=3
   ```
3. Consider upgrading your API plan for higher rate limits.

---

### Source API Failure

**Message:**
```
Error: failed to fetch GitHub issues: connection refused
```

**Code:** `CONNECTION_ERROR` (exit 4)

**Cause:** An external source (GitHub, Jira, etc.) is unreachable. This could be a network issue, firewall restriction, or service outage.

**Resolution:**

1. Check your network connection.
2. Verify the service URL is correct (especially for self-hosted Jira).
3. If behind a corporate firewall or VPN, make sure the service is accessible.
4. You can continue indexing without external sources -- they provide optional extra context. Disable the source in your project's sources configuration to skip it.

---

## Configuration Errors

These errors occur when Carto's configuration is invalid or incomplete.

### Missing Configuration

**Message:**
```
Error: memories URL not configured (set MEMORIES_URL or run carto init)
```

**Code:** `CONFIG_ERROR` (exit 3)

**Cause:** A required configuration value is missing.

**Resolution:**

Run `carto init` to set up all required values, or set the environment variable directly:
```bash
export MEMORIES_URL="http://localhost:8900"
```

### Invalid Strategy

**Message:**
```
Error: invalid strategy: merge (use add or replace)
```

**Code:** `CONFIG_ERROR` (exit 3)

**Cause:** You passed an invalid value for a flag that expects specific options.

**Resolution:**

Check the command's help for valid values:
```bash
carto import --help
```

### No Audit Log Configured

**Message:**
```
Error: no audit log configured (set CARTO_AUDIT_LOG or use --log-file)
```

**Code:** `CONFIG_ERROR` (exit 3)

**Cause:** You ran `carto logs` but no audit log file path is configured.

**Resolution:**

Set the audit log path:
```bash
export CARTO_AUDIT_LOG=/path/to/carto-audit.log
```

Or pass it as a flag:
```bash
carto --log-file /path/to/carto-audit.log logs
```

---

## Indexing Errors

These errors occur during the `carto index` pipeline.

### Permission Denied

**Message:**
```
Error: scan failed: open /path/to/file: permission denied
```

**Code:** `GENERAL_ERROR` (exit 1)

**Cause:** Carto tried to read a file it doesn't have permission to access.

**Resolution:**

1. Check file permissions: `ls -la /path/to/file`
2. Carto respects `.gitignore` rules, so ignored files are already skipped. If this file should be excluded, add it to `.gitignore`.
3. If running in Docker, make sure the mounted `PROJECTS_DIR` has correct permissions.

---

### Path Not Found

**Message:**
```
Error: scan failed: stat /path/to/project: no such file or directory
```

**Code:** `NOT_FOUND` (exit 2)

**Cause:** The project path doesn't exist, or it's a relative path that resolved incorrectly.

**Resolution:**

1. Verify the path exists: `ls /path/to/project`
2. Use an absolute path to avoid ambiguity:
   ```bash
   carto index --path /home/user/projects/my-app --name my-app
   ```
3. If using the API, check the `path` field in the project configuration.

---

### Chunking Failure

**Message:**
```
Error: chunking failed for internal/parser.go: tree-sitter parse error
```

**Code:** `GENERAL_ERROR` (exit 1)

**Cause:** The tree-sitter parser encountered a syntax error or unsupported language construct in a file.

**Resolution:**

1. Check if the file has valid syntax. Try compiling or linting it.
2. Make sure you built Carto with CGO enabled (`CGO_ENABLED=1`), as tree-sitter requires it.
3. This error is typically non-fatal -- Carto will skip the problematic file and continue indexing the rest of the project.

---

### Manifest Corruption

**Message:**
```
Error: manifest read failed: invalid checksum in .carto/manifest.json
```

**Code:** `GENERAL_ERROR` (exit 1)

**Cause:** The incremental indexing manifest file is corrupted, possibly due to an interrupted previous run.

**Resolution:**

1. Delete the manifest and re-index from scratch:
   ```bash
   rm -rf /path/to/project/.carto/manifest.json
   carto index --path /path/to/project --name my-project --full
   ```
2. The `--full` flag forces a complete re-index regardless of the manifest state.

---

## Query Errors

These errors occur when running `carto query` or calling the `/api/query` endpoint.

### No Results Found

**Message:**
```
No results found for "your query"
```

**Cause:** The query didn't match any content in the index. This could mean the project hasn't been indexed, the query is too specific, or the relevant code hasn't been indexed yet.

**Resolution:**

1. Verify the project is indexed: `carto status --project my-project`
2. Try broadening your query. Instead of asking about a specific function name, describe what it does.
3. Try a higher tier for more context: `carto query --project my-project "your query" --tier full`
4. If you recently added new code, re-index to pick up the changes:
   ```bash
   carto index --path /path/to/project --name my-project
   ```

---

### Project Not Indexed

**Message:**
```
Error: project "my-app" has not been indexed
```

**Code:** `NOT_FOUND` (exit 2)

**Cause:** You're trying to query a project that exists but hasn't been indexed yet.

**Resolution:**

Run the indexer first:

```bash
carto index --path /path/to/my-app --name my-app
```

---

### Invalid Tier

**Message:**
```
Error: invalid tier "detailed". Valid tiers: mini, standard, full
```

**Code:** `CONFIG_ERROR` (exit 3)

**Cause:** You specified a tier name that doesn't exist.

**Resolution:**

Use one of the three valid tiers:

```bash
carto query --project my-project "your query" --tier mini      # ~5 KB of context
carto query --project my-project "your query" --tier standard   # ~50 KB (default)
carto query --project my-project "your query" --tier full       # ~500 KB
```

---

## Server Errors

These errors occur when running `carto serve`.

### Port Already in Use

**Message:**
```
Error: listen tcp :8950: bind: address already in use
```

**Code:** `GENERAL_ERROR` (exit 1)

**Cause:** Another process is already using the port Carto wants to bind to.

**Resolution:**

1. Find what's using the port:
   ```bash
   lsof -i :8950
   ```
2. Either stop the other process or use a different port:
   ```bash
   carto serve --port 9000
   ```

---

### Static Files Not Found

**Message:**
```
Error: web UI static files not found at internal/server/static
```

**Code:** `NOT_FOUND` (exit 2)

**Cause:** The embedded web UI files are missing. This typically happens when building from source without the frontend assets.

**Resolution:**

1. Make sure you cloned the complete repository including the web UI.
2. Build the frontend assets before building Carto:
   ```bash
   cd web && npm install && npm run build && cd ..
   go build -o carto ./cmd/carto
   ```
3. If using a pre-built binary, download the full release that includes the web UI.

---

## Getting More Help

If you encounter an error not listed here:

1. Run `carto doctor` to check your environment setup.
2. Run the failing command with `--verbose` to get detailed diagnostic output.
3. Check the audit log for error patterns: `carto logs --result error --last 10`
4. Review the [Configuration Reference](config-reference.md) to make sure your setup is correct.
5. Search the project's issue tracker for similar errors.
6. Open a new issue with the full error message and your configuration (redact API keys).

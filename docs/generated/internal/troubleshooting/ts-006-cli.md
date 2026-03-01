# Troubleshooting: CLI Issues

**ID:** ts-006
**Topic:** CLI Issues
**Components:** `cmd/carto/`

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

## Symptom: `missing API key` Error

**Cause:** The `LLM_API_KEY` or `ANTHROPIC_API_KEY` environment variable is not set. The `index` command requires an LLM API key to perform atom extraction and deep analysis.

**Resolution:**

1. Set the API key:
   ```bash
   export LLM_API_KEY="sk-ant-..."
   ```

2. Alternatively, create a `.env` file in the project root. Check `.env.example` for the expected format.

3. Verify the config is loaded:
   ```bash
   carto config
   ```

**Note:** Commands that do not call the LLM (e.g., `status`, `modules`, `query`) may work without an API key if the Memories server already contains indexed data.

---

## Symptom: `connection refused` Error

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
   # Should be http://localhost:8900 or similar
   ```

4. If Memories is on a non-default port, update the variable:
   ```bash
   export MEMORIES_URL="http://localhost:9100"
   ```

---

## Symptom: `--json` Output Appears Malformed

**Cause:** Log messages or other non-JSON text is mixed into stdout, or the consumer is expecting a single JSON object instead of newline-delimited JSON (NDJSON).

**Resolution:**

1. Redirect stderr to separate logs from structured output:
   ```bash
   carto index --project myapp --json 2>/dev/null > output.json
   ```

2. Parse line-by-line. Each line is a valid JSON object:
   ```bash
   carto index --project myapp --json 2>/dev/null | while read -r line; do
     echo "$line" | jq .
   done
   ```

3. If piping to `jq`, use the `--slurp` flag to handle multiple objects:
   ```bash
   carto modules --project myapp --json | jq -s '.'
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

2. The valid commands are: `index`, `query`, `modules`, `patterns`, `status`, `serve`, `projects`, `sources`, `config`.

3. Each command has its own help:
   ```bash
   carto index --help
   ```

---

## Symptom: Index Completes but Produces No Output

**Cause:** The project path contains no files that the scanner recognizes, or all files are excluded by `.gitignore` or internal ignore rules.

**Resolution:**

1. Check the project status:
   ```bash
   carto status --project myapp
   ```

2. Verify the project path exists and contains source code:
   ```bash
   ls $(carto projects --json | jq -r '.[] | select(.name=="myapp") | .path')
   ```

3. Ensure the project contains files in supported languages (Go, Python, JavaScript, TypeScript, Java, Rust, etc.). The scanner uses tree-sitter grammars and will skip unsupported file types.

4. Check `.gitignore` rules. The scanner respects `.gitignore` and may be excluding files unintentionally.

---

## Quick Reference

| Symptom | First Check |
|---------|-------------|
| `command not found` | `go build -o carto ./cmd/carto` |
| `missing API key` | `echo $LLM_API_KEY` |
| `connection refused` | `curl http://localhost:8900/health` |
| `--json` malformed | Redirect stderr: `2>/dev/null` |
| `address already in use` | `lsof -i :8950` |
| `unknown command` | `carto --help` |
| No index output | `carto status --project <name>` |

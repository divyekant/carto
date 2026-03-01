---
id: ts-001
type: troubleshooting
audience: internal
topic: Indexing Pipeline Failures
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Troubleshooting: Indexing Pipeline Failures

## Quick Check

Before diving into specific symptoms, verify these basics:

1. **API key is set:** `echo $ANTHROPIC_API_KEY` or `echo $LLM_API_KEY`. At least one must be non-empty.
2. **Memories server is reachable:** `curl -s $MEMORIES_URL/health` (default: `http://localhost:8900/health`). Should return a 200 response.
3. **Target directory exists and is readable:** `ls /path/to/project` should list files without permission errors.

---

## Symptom: "Error scanning directory" / Pipeline Aborts at Phase 1

Scan failure is the only fatal failure mode. The pipeline cannot proceed without a file list.

### Diagnostic Steps

1. Confirm the target path exists: `ls -la /path/to/project`
2. Check for permission issues: `stat /path/to/project`
3. Look for broken symlinks: `find /path/to/project -maxdepth 1 -type l -xtype l`
4. Check if `.gitignore` rules are excluding everything: review `.gitignore` for overly broad patterns like `*` or `/`

### Root Causes & Resolutions

| Root Cause | Resolution |
|---|---|
| Directory does not exist | Verify the path. Check for typos. |
| Insufficient read permissions | Ensure the user running Carto has read access to the directory tree. |
| Broken symlinks in the root directory | Remove or fix broken symlinks. The scanner follows symlinks and fails on broken ones. |
| `.gitignore` excludes all files | Review `.gitignore`. The scanner respects gitignore rules. Overly broad patterns can result in zero files discovered. |
| Disk full or I/O errors | Check disk space with `df -h`. Review system logs for I/O errors. |

---

## Symptom: "0 atoms produced"

The atoms phase (Phase 2) completed but produced no atom summaries. This means every LLM call in the phase failed.

### Diagnostic Steps

1. Check if any source files were found: look at the scan phase output for file count. If 0 files were scanned, the problem is in Phase 1, not Phase 2.
2. Verify the LLM API key: `echo $ANTHROPIC_API_KEY` (or `$LLM_API_KEY`). Test it directly:
   ```
   curl -s https://api.anthropic.com/v1/messages \
     -H "x-api-key: $ANTHROPIC_API_KEY" \
     -H "anthropic-version: 2023-06-01" \
     -H "content-type: application/json" \
     -d '{"model":"claude-haiku-4-5-20251001","max_tokens":10,"messages":[{"role":"user","content":"hi"}]}'
   ```
3. Check if the fast model name is valid: `echo $CARTO_FAST_MODEL`. If set, it must be a valid model identifier for the configured provider.
4. Review `Result.Errors` for specific error messages (401 auth failures, 429 rate limits, 500 server errors).

### Root Causes & Resolutions

| Root Cause | Resolution |
|---|---|
| API key is missing or invalid | Set `ANTHROPIC_API_KEY` or `LLM_API_KEY` to a valid key. |
| API key has insufficient permissions or rate limits | Check the API key's usage tier and rate limits on the provider dashboard. |
| Fast model name is incorrect | Verify `CARTO_FAST_MODEL` matches a valid model ID. Unset it to use the default (`claude-haiku-4-5-20251001`). |
| Provider is experiencing an outage | Check the provider's status page (status.anthropic.com for Anthropic). Retry later. |
| Network connectivity issues (corporate proxy, firewall) | Ensure outbound HTTPS to the LLM provider is allowed. Check proxy settings. |
| All files are binary or empty | The scanner may have picked up non-source files. Check the file list from Phase 1. |

---

## Symptom: "Deep analysis timed out"

Phase 4 (Deep Analysis) is the most resource-intensive phase, using the deep-tier model. Timeouts can occur for large modules or when the provider is slow.

### Diagnostic Steps

1. Check the deep model configuration: `echo $CARTO_DEEP_MODEL` (default: `claude-opus-4-6`).
2. Check if per-module analysis succeeded for any modules (partial success is common).
3. Look at the module size -- modules with hundreds of atoms may exceed the model's context window or take very long to process.
4. Test the deep model directly with a simple prompt to verify availability.

### Root Causes & Resolutions

| Root Cause | Resolution |
|---|---|
| Deep model is overloaded or slow | Retry later. The deep tier uses a more expensive model that may have higher latency. |
| Module is too large (too many atoms for context window) | Use `--module` to index specific modules individually. Consider splitting large modules in the codebase. |
| HTTP timeout too short | Check if `CARTO_HTTP_TIMEOUT` is set. The default should be sufficient for most cases, but very large analyses may need more time. |
| Provider rate limiting on the deep tier | The deep tier makes fewer but larger requests. Check rate limit headers in the error details. Space out indexing runs. |

---

## Symptom: "Manifest mismatch" / Incremental Indexing Not Working

The `--incremental` flag relies on `.carto/manifest.json` to determine which files have changed. If the manifest is corrupted or out of sync, incremental indexing may behave unexpectedly.

### Diagnostic Steps

1. Check if `.carto/manifest.json` exists: `ls -la /path/to/project/.carto/manifest.json`
2. Verify the manifest is valid JSON: `python3 -m json.tool /path/to/project/.carto/manifest.json`
3. Compare a file's actual hash with the manifest entry: `shasum -a 256 /path/to/project/some/file.go`

### Root Causes & Resolutions

| Root Cause | Resolution |
|---|---|
| Manifest file was manually edited or corrupted | Delete `.carto/manifest.json` and run `carto index --full` to rebuild it. |
| Manifest was generated by a different version of Carto | Delete the manifest and re-index with `--full`. |
| File system clock skew (rare) | SHA-256 hashing is content-based, not timestamp-based, so clock skew should not affect it. If issues persist, run `--full`. |
| `.carto/` directory is gitignored and was not committed | This is expected behavior. The manifest is local state, not shared. Each developer generates their own. |

---

## Symptom: Store Phase Fails / Data Not Persisting

Phase 5 writes all analysis results to Memories. Failures here mean the index was computed but not saved.

### Diagnostic Steps

1. Verify Memories server is running: `curl -s $MEMORIES_URL/health`
2. Check the API key: `curl -s -H "X-API-Key: $MEMORIES_API_KEY" $MEMORIES_URL/memories/count`
3. Check Memories server logs for errors (disk space, database issues).
4. Look for specific error messages in `Result.Errors` related to HTTP status codes from Memories.

### Root Causes & Resolutions

| Root Cause | Resolution |
|---|---|
| Memories server is not running | Start the Memories server. Check its process and logs. |
| `MEMORIES_URL` is incorrect | Verify the URL. Default is `http://localhost:8900`. |
| `MEMORIES_API_KEY` is wrong or missing | Set the correct API key for the Memories server. |
| Memories server disk is full | Free disk space on the Memories server. Check database size. |
| Network timeout between Carto and Memories | Check network connectivity. If Memories is remote, ensure latency is acceptable. Consider running Memories locally. |
| Batch write exceeds server limits | Carto chunks to 500 items per batch. If the Memories server has lower limits, check its configuration. |

---

## Escalation

Escalate when:

- The LLM provider returns consistent 500 errors: check the provider's status page (status.anthropic.com, status.openai.com).
- Memories server crashes during store phase: check Memories server logs and consider filing a bug.
- Scan phase fails on a valid, readable directory with no permission issues: file a Carto bug with the directory structure (redacted if necessary).
- Atoms phase produces garbled or structurally invalid output: the LLM model may have changed behavior. Try a different model version.

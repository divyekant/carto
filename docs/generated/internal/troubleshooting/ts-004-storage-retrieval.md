---
id: ts-004
type: troubleshooting
audience: internal
topic: Storage & Query Issues
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Troubleshooting: Storage & Query Issues

## Quick Check

1. **Memories server is running:** `curl -s $MEMORIES_URL/health` (default: `http://localhost:8900/health`).
2. **API key is valid:** `curl -s -H "X-API-Key: $MEMORIES_API_KEY" $MEMORIES_URL/memories/count`
3. **Data exists for the project:** `curl -s -H "X-API-Key: $MEMORIES_API_KEY" "$MEMORIES_URL/memories/count?source=carto/{project}/"`

---

## Symptom: "0 results" for a Query

A query returns no data even though the project has been indexed.

### Diagnostic Steps

1. Verify the project name matches exactly:
   ```bash
   # Check what projects have data
   curl -s -H "X-API-Key: $MEMORIES_API_KEY" "$MEMORIES_URL/memories?limit=10" | grep -o '"source":"carto/[^/]*' | sort -u
   ```

2. Check if the specific layers exist:
   ```bash
   curl -s -H "X-API-Key: $MEMORIES_API_KEY" "$MEMORIES_URL/memories/count?source=carto/{project}/layer:atoms"
   curl -s -H "X-API-Key: $MEMORIES_API_KEY" "$MEMORIES_URL/memories/count?source=carto/{project}/layer:blueprint"
   ```

3. If counts are zero, the project data was either never stored or was deleted. Check indexing logs for Phase 5 errors.

4. If counts are non-zero, the query text may not match any stored content. Try a broader query or use the `--tier full` flag for more data.

### Root Causes & Resolutions

| Root Cause | Resolution |
|---|---|
| Project name mismatch (e.g., "myapp" vs "my-app") | Use the exact project name that was used during `carto index`. Check with the count query above. |
| Data was deleted | Re-index the project: `carto index /path/to/project --full` |
| Indexing Phase 5 failed (data never stored) | Check the indexing output for Phase 5 errors. Verify Memories was reachable during indexing. Re-index. |
| Query is too specific | Broaden the query terms. Search uses hybrid BM25 + vector matching. |
| Wrong tier selected | Mini tier only includes zones + blueprint. Use `--tier standard` or `--tier full` for more data. |

---

## Symptom: "connection refused" to Memories

Carto cannot connect to the Memories server.

### Diagnostic Steps

1. Check if the Memories server process is running:
   ```bash
   ps aux | grep memories
   # or
   lsof -i :8900
   ```

2. Verify `MEMORIES_URL` is correct:
   ```bash
   echo $MEMORIES_URL
   curl -s $MEMORIES_URL/health
   ```

3. If the server is running on a different port or host, update `MEMORIES_URL`.

4. Check for firewall or network issues if the server is remote.

### Root Causes & Resolutions

| Root Cause | Resolution |
|---|---|
| Memories server is not running | Start the Memories server. Check its startup logs for errors. |
| `MEMORIES_URL` is incorrect | Set `MEMORIES_URL` to the correct URL (including protocol and port). |
| Port conflict | Another process is using port 8900. Change the Memories server port or stop the conflicting process. |
| Firewall blocking the connection | For remote servers, ensure the port is open. For local servers, check local firewall rules. |
| Server crashed | Check server logs. Restart the server. Investigate the crash cause. |

---

## Symptom: Stale Data Returned

Queries return outdated information that does not reflect recent code changes.

### Diagnostic Steps

1. Check when the project was last indexed. Look at the timestamps on stored memories:
   ```bash
   curl -s -H "X-API-Key: $MEMORIES_API_KEY" "$MEMORIES_URL/memories?source=carto/{project}/layer:blueprint&limit=1"
   ```

2. Compare the stored data timestamps with the most recent code changes.

3. Check if the last indexing run completed successfully (Phase 5 in particular).

4. Check the manifest file:
   ```bash
   ls -la /path/to/project/.carto/manifest.json
   ```

### Root Causes & Resolutions

| Root Cause | Resolution |
|---|---|
| Project was not re-indexed after code changes | Run `carto index /path/to/project --incremental` to update. |
| Incremental indexing missed some changes | Run `carto index /path/to/project --full` for a clean re-index. |
| Phase 5 failed during re-indexing (old data not replaced) | Check indexing logs. Verify Memories connectivity. Re-index with `--full`. |
| `delete-by-prefix` failed but new data was written | Old and new data coexist. Run `--full` to clear and rewrite. |

---

## Symptom: Slow Queries

Queries take noticeably long (> 5 seconds) to return results.

### Diagnostic Steps

1. Test Memories server response time directly:
   ```bash
   time curl -s -H "X-API-Key: $MEMORIES_API_KEY" "$MEMORIES_URL/memories/count"
   ```

2. Check total memory count:
   ```bash
   curl -s -H "X-API-Key: $MEMORIES_API_KEY" "$MEMORIES_URL/memories/count"
   ```
   Large databases (> 100,000 entries) may slow down hybrid search.

3. Check server resource usage (CPU, memory, disk I/O).

4. If using full-tier retrieval, the response payload itself may be large (~500 KB). Network transfer time may be a factor for remote servers.

### Root Causes & Resolutions

| Root Cause | Resolution |
|---|---|
| Memories server is resource-constrained | Increase CPU/memory allocation for the Memories server. |
| Very large database (many projects indexed) | Consider running separate Memories instances per environment. Clean up unused project data. |
| Hybrid search is expensive | For simple lookups, use `source_prefix` filtering instead of free-form search. Reduce `k` (number of results). |
| Network latency to remote Memories server | Consider running Memories locally. Check network path for bottlenecks. |
| Full-tier retrieval for a very large project | Use `--tier standard` or `--tier mini` if full context is not needed. |

---

## Symptom: Store Phase Errors During Indexing

Phase 5 of the pipeline reports errors when writing data to Memories.

### Diagnostic Steps

1. Check Memories connectivity during the time of indexing.
2. Look at the specific error messages in `Result.Errors`.
3. Check Memories server logs for corresponding error entries.
4. Verify the API key is valid for write operations.

### Root Causes & Resolutions

| Root Cause | Resolution |
|---|---|
| Memories server went down during indexing | Restart Memories and re-index with `--full`. |
| API key lacks write permissions | Verify the API key has write access. Check Memories server configuration. |
| Payload too large | Should not occur with default 500-item batching. Check if individual entries exceed server limits. Content > 49K chars is truncated by Carto. |
| Disk full on Memories server | Free disk space. Check database size. |
| Concurrent indexing caused conflicts | Avoid indexing the same project simultaneously from multiple Carto instances. |

---

## Escalation

Escalate when:

- Memories server crashes repeatedly under normal load.
- Data corruption is suspected (queries return garbled content).
- Hybrid search returns incorrect relevance rankings consistently.
- Batch write failures cannot be resolved by retrying.
- The `delete-by-prefix` operation does not fully clear old data (phantom entries remain).

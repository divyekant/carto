# Troubleshooting: API & SSE Issues

**ID:** ts-007
**Topic:** API & SSE Issues
**Components:** `server/` — `server.go`, `routes.go`, `handlers.go`, `sse.go`

---

## Symptom: CORS Error in Browser Console

**Cause:** The browser blocks cross-origin requests when the Web UI's origin does not match the API server's origin. This typically occurs during development when the Vite dev server (e.g., `localhost:5173`) makes requests to the Go server (`localhost:8950`).

**Resolution:**

1. During development, use the Vite proxy configuration to route `/api/` requests to the Go server. The Vite config should already include this proxy rule.

2. In production, serve the SPA through `carto serve`. The SPA and API share the same origin, so CORS is not a factor.

3. If making API calls from an external origin (e.g., a separate dashboard), verify that the server's CORS middleware allows the request origin. Check `server.go` for the CORS header configuration.

---

## Symptom: SSE Stream Disconnects Unexpectedly

**Cause:** One of the following:
- The HTTP client imposes a read timeout, closing the connection when no data arrives within the timeout window.
- A reverse proxy or load balancer (nginx, Cloudflare, etc.) terminates idle connections.
- The client-side `EventSource` is not handling reconnection.

**Resolution:**

1. Ensure the HTTP client does not set a read timeout on SSE connections. In curl, use `-N` to disable buffering. In JavaScript, `EventSource` handles this natively.

2. Check for intermediate proxies. Nginx, for example, defaults to a 60-second proxy read timeout. Increase it:
   ```nginx
   proxy_read_timeout 3600s;
   proxy_buffering off;
   ```

3. If using `EventSource` in the browser, it automatically reconnects on disconnect. However, the index operation on the server side does not resume; reconnection yields a new stream (likely a 200 with no events if no index is running).

4. For long-running indexes, verify the server remains responsive by checking its logs. If the LLM provider is slow, the SSE stream is legitimately idle between events.

---

## Symptom: `404` on API Endpoints

**Cause:** The requested path does not match any registered route. Common causes:
- Typo in the URL path.
- Project name does not match a registered project (for project-specific endpoints).
- The server is not running, and a different service is responding.

**Resolution:**

1. Verify the server is running:
   ```bash
   curl http://localhost:8950/api/projects
   ```

2. Check the URL path matches the documented endpoints. All API paths start with `/api/`.

3. For project-specific endpoints, verify the project exists:
   ```bash
   curl http://localhost:8950/api/projects | jq '.[].name'
   ```

4. Check for trailing slashes. The router may or may not redirect `/api/projects/` to `/api/projects`.

---

## Symptom: Index Stuck in Progress

**Cause:** The pipeline is blocked on an external call (LLM API or Memories server), or a goroutine has stalled.

**Resolution:**

1. Check the server's stderr output for error messages or stalled log lines. The pipeline logs each phase transition.

2. Verify the LLM provider is responsive:
   ```bash
   curl https://api.anthropic.com/v1/messages -H "x-api-key: $LLM_API_KEY" -H "anthropic-version: 2023-06-01" -d '{"model":"claude-sonnet-4-20250514","max_tokens":10,"messages":[{"role":"user","content":"test"}]}'
   ```

3. Verify Memories is responsive:
   ```bash
   curl http://localhost:8900/health
   ```

4. If the index is truly stuck, disconnect the SSE client. This triggers pipeline cancellation via the request context. Restart the server if needed.

5. Check system resources. Large codebases with many files can consume significant memory during the atom extraction phase.

---

## Symptom: `500` Internal Server Error on API Calls

**Cause:** An unhandled error in the handler, typically from a downstream dependency (LLM client, Memories client, filesystem).

**Resolution:**

1. Check the server's stderr for the full error stack. The handler logs the error before returning the 500 response.

2. Common causes:
   - Expired or invalid `LLM_API_KEY`.
   - Memories server returned an error (disk full, corrupted index).
   - Project path does not exist or is not readable.

3. Fix the underlying issue and retry the request.

---

## Symptom: Query Returns Empty Results

**Cause:** The project has not been indexed, or the indexed data was deleted from Memories.

**Resolution:**

1. Check the project status:
   ```bash
   curl http://localhost:8950/api/projects/myapp
   ```
   Look for `last_indexed` or similar status fields.

2. If not indexed, trigger an index first:
   ```bash
   curl -N -X POST http://localhost:8950/api/projects/myapp/index
   ```

3. If indexed but returning empty, verify Memories contains data for the project:
   ```bash
   curl "$MEMORIES_URL/memory/search" -d '{"query":"project:myapp","k":1}'
   ```

---

## Quick Reference

| Symptom | First Check |
|---------|-------------|
| CORS error | Is the request from the same origin? Check Vite proxy config. |
| SSE disconnects | Check client timeout settings and proxy configs. |
| 404 on endpoints | `curl http://localhost:8950/api/projects` to verify server is up. |
| Index stuck | Check server stderr and LLM/Memories availability. |
| 500 errors | Check server stderr for the full error message. |
| Empty query results | Verify the project has been indexed. |

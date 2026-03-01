# Feature Handoff: REST API

**ID:** fh-007
**Feature:** REST API
**Components:** `server/` — `server.go`, `routes.go`, `handlers.go`, `sse.go`, `handlers_test.go`, `server_test.go`

---

## Overview

The Carto REST API provides HTTP endpoints for managing projects, triggering indexing, querying the semantic index, and configuring the system. It is served by the same process that hosts the embedded Web UI (`carto serve`). The server uses Go's standard `net/http` with route multiplexing, CORS middleware for development, and Server-Sent Events (SSE) for streaming index progress to clients.

---

## Endpoints

### Projects

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/projects` | List all registered projects. Returns an array of project objects. |
| `POST` | `/api/projects` | Create a new project. Request body: `{"name": "...", "path": "..."}`. |
| `GET` | `/api/projects/{name}` | Get details for a specific project, including status and module list. |
| `DELETE` | `/api/projects/{name}` | Remove a project registration. Does not delete indexed data from Memories. |

### Indexing

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/projects/{name}/index` | Trigger indexing for the named project. Returns an SSE stream. |

The index endpoint accepts an optional JSON body:
```json
{
  "full": false,
  "module": "auth"
}
```

### Sources

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/projects/{name}/sources` | Get the external signal source configuration for a project. |
| `PUT` | `/api/projects/{name}/sources` | Update external signal sources. Request body: source config JSON. |

### Query

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/query` | Query the semantic index. Query parameters: `q` (query string), `project` (project name), `tier` (mini/standard/full), `k` (result count). |

### Configuration

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/config` | Get the current system configuration. |
| `PUT` | `/api/config` | Update configuration. Request body: config JSON. |

---

## SSE (Server-Sent Events)

The `POST /api/projects/{name}/index` endpoint returns a `text/event-stream` response. The server streams events as the 6-phase pipeline progresses.

### Event Types

| Event | Data Format | Description |
|-------|-------------|-------------|
| `progress` | `{"phase": "scan", "progress": 0.45, "message": "Scanning files..."}` | Emitted during each pipeline phase with progress percentage and status message. |
| `complete` | `{"phases_completed": 6, "duration_ms": 12345}` | Emitted when the pipeline finishes successfully. |
| `error` | `{"error": "missing API key", "phase": "atoms"}` | Emitted when the pipeline encounters a fatal error. |
| `stopped` | `{"reason": "user_cancelled", "phase": "atoms"}` | Emitted when the client requests cancellation via the stop mechanism. |

### SSE Wire Format

```
event: progress
data: {"phase":"scan","progress":0.25,"message":"Discovered 142 files"}

event: progress
data: {"phase":"atoms","progress":0.60,"message":"Processing atoms (85/142)"}

event: complete
data: {"phases_completed":6,"duration_ms":45200}
```

### Cancellation

The client can stop an in-progress index operation. When the SSE client disconnects or sends a stop signal, the server propagates cancellation through the pipeline's context. The server emits a `stopped` event before closing the stream.

---

## CORS

CORS is enabled to allow the Web UI (which may run on a different port during development with Vite) to communicate with the API. The middleware sets permissive headers for development. In production, the SPA is served from the same origin, so CORS is not strictly needed but remains enabled.

---

## Static File Serving

The server serves the embedded React SPA from the root path (`/`). Any request that does not match an `/api/` route is served from the embedded filesystem. This enables client-side routing in the SPA (all non-API paths resolve to `index.html`).

---

## Configuration

| Setting | Source | Default | Description |
|---------|--------|---------|-------------|
| Port | `--port` flag on `carto serve` | `8950` | TCP port for the HTTP server. |
| Projects dir | `--projects-dir` flag | `""` | Base directory for project discovery. |

All other configuration (LLM keys, Memories URL) is read from environment variables by the underlying pipeline and storage packages.

---

## Edge Cases

- **Concurrent index requests for the same project:** The server does not serialize index requests. Two simultaneous `POST /api/projects/{name}/index` calls for the same project run two pipelines concurrently, which can cause data races in Memories storage. The Web UI prevents this by disabling the index button during active indexing, but API consumers must handle this themselves.
- **SSE client disconnect:** When the SSE client disconnects mid-stream, the server detects the closed connection via the request context and initiates pipeline cancellation. Goroutines in the pipeline check the context and wind down.
- **Large project indexing timeout:** Long-running indexes (large codebases, slow LLM) can exceed HTTP client timeouts. SSE connections should not have a read timeout on the client side. The server does not impose a timeout on the index operation itself.
- **Project not found on index trigger:** Returns `404` with a JSON error body.
- **Invalid request body:** Returns `400` with a JSON error describing the validation failure.

---

## Common Questions

**Q1: What port does the API run on?**
Default is `8950`. Override with `carto serve --port <port>`.

**Q2: How do I trigger indexing via curl?**
```bash
curl -N -X POST http://localhost:8950/api/projects/myapp/index
```
The `-N` flag disables output buffering, which is needed to see SSE events in real time.

**Q3: How do I query via the API?**
```bash
curl "http://localhost:8950/api/query?q=how+does+auth+work&project=myapp&tier=standard&k=5"
```

**Q4: Can I stop an index in progress?**
Yes. Disconnect the SSE client (close the HTTP connection). The server cancels the pipeline context and emits a `stopped` event. The Web UI provides a stop button that triggers this.

**Q5: Why do I get CORS errors when calling the API from a browser?**
The server includes CORS middleware, but if the request originates from a domain not covered by the CORS policy, the browser blocks it. During local development, the Vite dev server proxies API requests to avoid this. In production, the SPA and API share the same origin.

---

## Troubleshooting

| Symptom | Likely Cause | Resolution |
|---------|-------------|------------|
| `CORS error` in browser console | Request origin not allowed, or preflight OPTIONS request failing. | Use the embedded SPA (same origin) or configure the Vite proxy during development. |
| SSE stream disconnects immediately | Client is not reading the stream, or a proxy/load balancer is closing idle connections. | Ensure the HTTP client does not set a read timeout. Disable buffering (`-N` in curl). Check for reverse proxies that close idle connections. |
| `404` on `/api/projects/...` | Project name does not match any registered project, or the API path is incorrect. | Verify the project name with `GET /api/projects`. Check for typos in the URL. |
| Index appears stuck (no SSE events) | LLM calls are slow, or the pipeline is blocked waiting on Memories. | Check LLM provider status. Verify Memories is responsive (`curl http://localhost:8900/health`). Review server logs for errors. |
| `500` error on index trigger | Internal error in the pipeline (e.g., LLM auth failure, Memories write failure). | Check the server's stderr output for the full error. Common causes: expired API key, Memories disk full. |

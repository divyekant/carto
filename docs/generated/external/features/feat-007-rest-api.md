# REST API

Carto exposes a REST API that gives you programmatic access to every feature — listing projects, triggering indexing, querying the semantic graph, and managing configuration. The API is the same one that powers the web dashboard, so anything you can do in the UI you can do with HTTP requests.

## Starting the Server

```bash
carto serve --port 8950
```

The API is now available at `http://localhost:8950/api/`. The `--port` flag is optional and defaults to `8950`.

## Endpoints

### Projects

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/projects` | List all known projects |
| `POST` | `/api/projects` | Add a new project |
| `GET` | `/api/projects/{name}` | Get details for a single project |
| `DELETE` | `/api/projects/{name}` | Remove a project |

### Indexing

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/projects/{name}/index` | Trigger indexing (returns SSE stream) |

### Sources

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/projects/{name}/sources` | Get a project's signal sources |
| `PUT` | `/api/projects/{name}/sources` | Update a project's signal sources |

### Query

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/query` | Query the semantic index |

### Configuration

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/config` | Get current configuration |
| `PUT` | `/api/config` | Update configuration |

## Examples

### List All Projects

```bash
curl http://localhost:8950/api/projects
```

Response:

```json
[
  {
    "name": "backend-api",
    "path": "/repos/backend",
    "lastIndexed": "2026-02-27T14:30:00Z",
    "fileCount": 342
  }
]
```

### Add a Project

```bash
curl -X POST http://localhost:8950/api/projects \
  -H "Content-Type: application/json" \
  -d '{"name": "frontend", "path": "/repos/frontend"}'
```

### Get Project Details

```bash
curl http://localhost:8950/api/projects/backend-api
```

### Remove a Project

```bash
curl -X DELETE http://localhost:8950/api/projects/old-project
```

### Trigger Indexing with SSE Progress

The indexing endpoint returns a Server-Sent Events (SSE) stream so you can monitor progress in real time:

```bash
curl -N http://localhost:8950/api/projects/backend-api/index -X POST
```

The stream emits events as indexing progresses:

```
data: {"phase":"scan","progress":10,"message":"Scanning files..."}

data: {"phase":"chunk","progress":30,"message":"Chunking 342 files..."}

data: {"phase":"analyze","progress":60,"message":"Running deep analysis..."}

data: {"phase":"store","progress":90,"message":"Storing index in Memories..."}

data: {"phase":"complete","progress":100,"message":"Indexing complete"}
```

To consume SSE from code, use any SSE client library. Here is a JavaScript example:

```javascript
const source = new EventSource(
  "http://localhost:8950/api/projects/backend-api/index"
);

source.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log(`[${data.phase}] ${data.progress}% — ${data.message}`);
  if (data.phase === "complete") {
    source.close();
  }
};
```

### Query the Index

```bash
curl "http://localhost:8950/api/query?project=backend-api&q=How+does+authentication+work%3F"
```

Response:

```json
{
  "answer": "Authentication is handled by the auth middleware in...",
  "sources": [
    {"file": "internal/middleware/auth.go", "module": "backend-api"}
  ]
}
```

You can also specify a retrieval tier:

```bash
curl "http://localhost:8950/api/query?project=backend-api&q=auth&tier=full"
```

### View Project Sources

```bash
curl http://localhost:8950/api/projects/backend-api/sources
```

### Update Project Sources

```bash
curl -X PUT http://localhost:8950/api/projects/backend-api/sources \
  -H "Content-Type: application/json" \
  -d '{
    "ci": "https://github.com/org/repo/actions",
    "issues": "https://github.com/org/repo/issues"
  }'
```

### View Configuration

```bash
curl http://localhost:8950/api/config
```

### Update Configuration

```bash
curl -X PUT http://localhost:8950/api/config \
  -H "Content-Type: application/json" \
  -d '{"llm_provider": "anthropic"}'
```

## Configuration

The only server-level configuration is the port:

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--port` | `CARTO_PORT` | `8950` | HTTP port the server listens on |

All other configuration (LLM keys, Memories URL, etc.) is read from environment variables or the Carto config. See the [CLI docs](feat-006-cli.md) for `carto config`.

## Limitations

- **Single server** — The API runs as a single-process HTTP server. It is designed for local and small-team use, not high-availability production deployments.
- **No built-in authentication** — The API does not include authentication or authorization. If you expose it beyond localhost, put it behind a reverse proxy with your own auth layer.
- **SSE for indexing** — The indexing endpoint uses Server-Sent Events rather than WebSockets. Make sure your HTTP client or proxy supports SSE streaming.

## Related

- [CLI Reference](feat-006-cli.md) — the same features accessible from the command line
- [Web Dashboard](feat-008-web-ui.md) — the visual interface that consumes this API
- [Go SDK](feat-009-sdk.md) — embed Carto directly in Go programs without HTTP

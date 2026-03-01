---
type: api-reference
audience: external
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# API Reference

Carto exposes a REST API when you run `carto serve`. You can use this API to manage projects, trigger indexing, configure sources, and query your codebase programmatically.

## Overview

| Detail | Value |
|--------|-------|
| **Base URL** | `http://localhost:8950` |
| **Authentication** | None (local server) |
| **Content Type** | `application/json` |
| **Error Format** | `{ "error": "message" }` |

Start the server with:

```bash
carto serve --port 8950
```

---

## Projects

Projects represent codebases that Carto has indexed or is configured to index.

### List All Projects

Returns a list of all registered projects.

**Request**

```
GET /api/projects
```

**Parameters**

None.

**Response**

```json
[
  {
    "name": "my-app",
    "path": "/home/user/projects/my-app",
    "indexed_at": "2026-02-28T14:30:00Z",
    "file_count": 142,
    "module_count": 3
  }
]
```

**Status Codes**

| Code | Meaning |
|------|---------|
| 200 | Success |

**Example**

```bash
curl http://localhost:8950/api/projects
```

---

### Create a Project

Registers a new project with Carto.

**Request**

```
POST /api/projects
```

**Request Body**

```json
{
  "name": "my-app",
  "path": "/home/user/projects/my-app"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | A unique name for the project |
| `path` | string | Yes | Absolute path to the project directory |

**Response**

```json
{
  "name": "my-app",
  "path": "/home/user/projects/my-app",
  "created_at": "2026-02-28T14:30:00Z"
}
```

**Status Codes**

| Code | Meaning |
|------|---------|
| 201 | Project created |
| 400 | Invalid request (missing fields or bad path) |
| 409 | Project with that name already exists |

**Example**

```bash
curl -X POST http://localhost:8950/api/projects \
  -H "Content-Type: application/json" \
  -d '{"name": "my-app", "path": "/home/user/projects/my-app"}'
```

---

### Get Project Details

Returns detailed information about a specific project.

**Request**

```
GET /api/projects/{name}
```

**Parameters**

| Parameter | Location | Type | Required | Description |
|-----------|----------|------|----------|-------------|
| `name` | path | string | Yes | The project name |

**Response**

```json
{
  "name": "my-app",
  "path": "/home/user/projects/my-app",
  "indexed_at": "2026-02-28T14:30:00Z",
  "file_count": 142,
  "module_count": 3,
  "modules": [
    { "name": "api", "path": "cmd/api", "file_count": 28 },
    { "name": "core", "path": "internal/core", "file_count": 67 },
    { "name": "web", "path": "web", "file_count": 47 }
  ]
}
```

**Status Codes**

| Code | Meaning |
|------|---------|
| 200 | Success |
| 404 | Project not found |

**Example**

```bash
curl http://localhost:8950/api/projects/my-app
```

---

### Delete a Project

Removes a project and its index data.

**Request**

```
DELETE /api/projects/{name}
```

**Parameters**

| Parameter | Location | Type | Required | Description |
|-----------|----------|------|----------|-------------|
| `name` | path | string | Yes | The project name |

**Response**

```json
{
  "deleted": true
}
```

**Status Codes**

| Code | Meaning |
|------|---------|
| 200 | Project deleted |
| 404 | Project not found |

**Example**

```bash
curl -X DELETE http://localhost:8950/api/projects/my-app
```

---

## Indexing

### Trigger Project Indexing

Starts an indexing run for a project. This endpoint returns a **Server-Sent Events (SSE)** stream so you can monitor progress in real time.

**Request**

```
POST /api/projects/{name}/index
```

**Parameters**

| Parameter | Location | Type | Required | Description |
|-----------|----------|------|----------|-------------|
| `name` | path | string | Yes | The project name |

**Response**

The response uses `Content-Type: text/event-stream`. Events are sent as the indexing pipeline progresses.

#### SSE Event Types

| Event | Description | Data Format |
|-------|-------------|-------------|
| `progress` | Reports current progress | `{ "phase": "chunking", "current": 42, "total": 142, "message": "Chunking files..." }` |
| `complete` | Indexing finished successfully | `{ "files": 142, "duration": "47s" }` |
| `error` | An error occurred during indexing | `{ "error": "LLM API rate limit exceeded" }` |
| `stopped` | Indexing was cancelled by the user | `{ "message": "Indexing stopped by user" }` |

**Status Codes**

| Code | Meaning |
|------|---------|
| 200 | SSE stream started |
| 404 | Project not found |
| 409 | Indexing already in progress for this project |

**Example**

```bash
curl -N http://localhost:8950/api/projects/my-app/index -X POST
```

You'll receive output like:

```
event: progress
data: {"phase":"scanning","current":0,"total":0,"message":"Scanning files..."}

event: progress
data: {"phase":"chunking","current":42,"total":142,"message":"Chunking files..."}

event: progress
data: {"phase":"chunking","current":142,"total":142,"message":"Chunking files..."}

event: progress
data: {"phase":"analyzing","current":0,"total":0,"message":"Running deep analysis..."}

event: complete
data: {"files":142,"duration":"47s"}
```

**Consuming SSE in JavaScript**

```javascript
const source = new EventSource("http://localhost:8950/api/projects/my-app/index", {
  method: "POST"
});

source.addEventListener("progress", (e) => {
  const data = JSON.parse(e.data);
  console.log(`${data.phase}: ${data.current}/${data.total}`);
});

source.addEventListener("complete", (e) => {
  const data = JSON.parse(e.data);
  console.log(`Done! Indexed ${data.files} files in ${data.duration}`);
  source.close();
});

source.addEventListener("error", (e) => {
  const data = JSON.parse(e.data);
  console.error(`Error: ${data.error}`);
  source.close();
});
```

---

## Sources

Sources are external integrations (GitHub, Jira, Linear, Notion, Slack, etc.) that provide additional context for a project's index.

### Get Sources Configuration

Returns the current source configuration for a project.

**Request**

```
GET /api/projects/{name}/sources
```

**Parameters**

| Parameter | Location | Type | Required | Description |
|-----------|----------|------|----------|-------------|
| `name` | path | string | Yes | The project name |

**Response**

```json
{
  "github": {
    "enabled": true,
    "repo": "org/my-app"
  },
  "jira": {
    "enabled": false
  },
  "linear": {
    "enabled": true,
    "team": "ENG"
  },
  "notion": {
    "enabled": false
  },
  "slack": {
    "enabled": false
  }
}
```

**Status Codes**

| Code | Meaning |
|------|---------|
| 200 | Success |
| 404 | Project not found |

**Example**

```bash
curl http://localhost:8950/api/projects/my-app/sources
```

---

### Update Sources Configuration

Updates the source configuration for a project.

**Request**

```
PUT /api/projects/{name}/sources
```

**Parameters**

| Parameter | Location | Type | Required | Description |
|-----------|----------|------|----------|-------------|
| `name` | path | string | Yes | The project name |

**Request Body**

```json
{
  "github": {
    "enabled": true,
    "repo": "org/my-app"
  },
  "jira": {
    "enabled": true,
    "project": "MYAPP"
  }
}
```

You only need to include the sources you want to change.

**Response**

```json
{
  "github": {
    "enabled": true,
    "repo": "org/my-app"
  },
  "jira": {
    "enabled": true,
    "project": "MYAPP"
  },
  "linear": {
    "enabled": true,
    "team": "ENG"
  },
  "notion": {
    "enabled": false
  },
  "slack": {
    "enabled": false
  }
}
```

**Status Codes**

| Code | Meaning |
|------|---------|
| 200 | Sources updated |
| 400 | Invalid source configuration |
| 404 | Project not found |

**Example**

```bash
curl -X PUT http://localhost:8950/api/projects/my-app/sources \
  -H "Content-Type: application/json" \
  -d '{"jira": {"enabled": true, "project": "MYAPP"}}'
```

---

## Configuration

### Get Global Configuration

Returns the current global Carto configuration.

**Request**

```
GET /api/config
```

**Parameters**

None.

**Response**

```json
{
  "llm_provider": "anthropic",
  "fast_model": "claude-haiku-4-5-20251001",
  "deep_model": "claude-opus-4-6",
  "max_concurrent": 10,
  "memories_url": "http://localhost:8900"
}
```

**Status Codes**

| Code | Meaning |
|------|---------|
| 200 | Success |

**Example**

```bash
curl http://localhost:8950/api/config
```

---

### Update Global Configuration

Updates global Carto configuration values.

**Request**

```
PUT /api/config
```

**Request Body**

```json
{
  "fast_model": "claude-haiku-4-5-20251001",
  "deep_model": "claude-sonnet-4-20250514",
  "max_concurrent": 5
}
```

You only need to include the fields you want to change.

**Response**

```json
{
  "llm_provider": "anthropic",
  "fast_model": "claude-haiku-4-5-20251001",
  "deep_model": "claude-sonnet-4-20250514",
  "max_concurrent": 5,
  "memories_url": "http://localhost:8900"
}
```

**Status Codes**

| Code | Meaning |
|------|---------|
| 200 | Configuration updated |
| 400 | Invalid configuration value |

**Example**

```bash
curl -X PUT http://localhost:8950/api/config \
  -H "Content-Type: application/json" \
  -d '{"max_concurrent": 5}'
```

---

## Query

### Search Indexed Codebases

Searches your indexed projects using natural language. Carto retrieves the most relevant context from the semantic index and returns it.

**Request**

```
GET /api/query
```

**Parameters**

| Parameter | Location | Type | Required | Default | Description |
|-----------|----------|------|----------|---------|-------------|
| `q` | query | string | Yes | -- | The natural language search query |
| `project` | query | string | No | -- | Limit search to a specific project |
| `tier` | query | string | No | `standard` | Detail level: `mini`, `standard`, or `full` |
| `k` | query | integer | No | 10 | Maximum number of results to return |

**Tier Descriptions**

| Tier | Context Size | Best For |
|------|-------------|----------|
| `mini` | ~5 KB | Quick lookups, function signatures, short answers |
| `standard` | ~50 KB | General questions, understanding flow and relationships |
| `full` | ~500 KB | Deep dives, architectural questions, complex debugging |

**Response**

```json
{
  "results": [
    {
      "source": "internal/auth/middleware.go",
      "score": 0.94,
      "content": "The auth middleware validates JWT tokens on every request...",
      "layer": "atoms"
    },
    {
      "source": "internal/auth/jwt.go",
      "score": 0.87,
      "content": "Token verification uses RS256 with public keys loaded from...",
      "layer": "atoms"
    }
  ],
  "query": "How does authentication work?",
  "project": "my-app",
  "tier": "standard",
  "count": 2
}
```

**Status Codes**

| Code | Meaning |
|------|---------|
| 200 | Success |
| 400 | Missing or invalid query parameter |
| 404 | Specified project not found or not indexed |

**Example**

```bash
# Basic query
curl "http://localhost:8950/api/query?q=How+does+authentication+work?"

# Query a specific project with mini tier
curl "http://localhost:8950/api/query?q=database+migrations&project=my-app&tier=mini"

# Get more results
curl "http://localhost:8950/api/query?q=error+handling&k=20"
```

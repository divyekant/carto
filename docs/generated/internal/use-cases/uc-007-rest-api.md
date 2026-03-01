# Use Case: API-Driven Indexing

**ID:** uc-007
**Topic:** API-Driven Indexing
**Trigger:** An external system (CI pipeline, dashboard, or custom tool) calls `POST /api/projects/{name}/index` to start indexing and monitors progress via SSE.

---

## Primary Flow

### 1. Register the Project

Before indexing, the project must exist in Carto. Create it via the API:

```bash
curl -X POST http://localhost:8950/api/projects \
  -H "Content-Type: application/json" \
  -d '{"name": "backend-api", "path": "/projects/backend-api"}'
```

Response: `201 Created` with the project object.

### 2. Trigger Indexing

```bash
curl -N -X POST http://localhost:8950/api/projects/backend-api/index \
  -H "Content-Type: application/json" \
  -d '{"full": false}'
```

The `-N` flag disables curl buffering, which is required to see SSE events as they arrive.

### 3. Monitor SSE Events

The response is a `text/event-stream`. Events arrive as the pipeline progresses:

```
event: progress
data: {"phase":"scan","progress":0.10,"message":"Scanning files..."}

event: progress
data: {"phase":"atoms","progress":0.45,"message":"Extracting atoms (64/142)"}

event: progress
data: {"phase":"deep_analysis","progress":0.80,"message":"Analyzing cross-component wiring"}

event: complete
data: {"phases_completed":6,"duration_ms":38200}
```

The client reads events until it receives `complete`, `error`, or `stopped`.

### 4. Query After Indexing

Once indexing completes, query the semantic index:

```bash
curl "http://localhost:8950/api/query?q=authentication+flow&project=backend-api&tier=standard"
```

---

## Variation: Index with Sources Configuration

Before triggering the index, configure external signal sources (e.g., Jira, GitHub Issues):

```bash
curl -X PUT http://localhost:8950/api/projects/backend-api/sources \
  -H "Content-Type: application/json" \
  -d '{"github": {"repo": "org/backend-api", "token": "ghp_..."}}'
```

Then trigger the index. The pipeline's signals phase collects data from the configured sources and incorporates it into the semantic index.

---

## Variation: Stop Mid-Index

If the client needs to abort an in-progress index:

1. Close the SSE connection (e.g., abort the HTTP request).
2. The server detects the disconnection and cancels the pipeline context.
3. Pipeline goroutines check the context and wind down.
4. The server emits a `stopped` event (received by any other listeners, if applicable).

From the Web UI, the user clicks the stop button, which closes the EventSource and triggers the same cancellation flow.

---

## Variation: Programmatic Client (Go)

```go
resp, err := http.Post("http://localhost:8950/api/projects/backend-api/index",
    "application/json", strings.NewReader(`{"full":false}`))
if err != nil {
    log.Fatal(err)
}
defer resp.Body.Close()

scanner := bufio.NewScanner(resp.Body)
for scanner.Scan() {
    line := scanner.Text()
    if strings.HasPrefix(line, "data: ") {
        data := line[6:]
        fmt.Println(data)
    }
}
```

---

## Postconditions

- The 7-layer semantic index is stored in Memories for the specified project.
- The manifest is updated to reflect the current file state.
- The query endpoint returns results based on the freshly indexed data.
- If cancelled, partial data may exist in Memories from completed phases.

# Use Case: Programmatic Indexing via SDK

**ID:** uc-009
**Topic:** Programmatic Indexing via SDK
**Trigger:** A Go program imports `pkg/carto` and calls `Index()` to programmatically index a codebase and `Query()` to retrieve context.

---

## Primary Flow

### 1. Import the SDK

```go
import "github.com/user/carto/pkg/carto"
```

Add the dependency:
```bash
go get github.com/user/carto/pkg/carto
```

### 2. Configure Environment

The SDK reads configuration from environment variables. Set them before the program runs:

```bash
export LLM_API_KEY="sk-ant-..."
export MEMORIES_URL="http://localhost:8900"
```

### 3. Index a Project

```go
ctx := context.Background()

err := carto.Index(ctx, carto.IndexOptions{
    Project:     "backend-api",
    Path:        "/projects/backend-api",
    Incremental: true,
})
if err != nil {
    log.Fatalf("Index failed: %v", err)
}
```

The `Index()` call blocks until the 6-phase pipeline completes. On success, the 7-layer semantic index is stored in Memories.

### 4. Query the Index

```go
results, err := carto.Query(ctx, carto.QueryOptions{
    Project: "backend-api",
    Query:   "How does the payment processing flow work?",
    Tier:    "standard",
    K:       5,
})
if err != nil {
    log.Fatalf("Query failed: %v", err)
}

for _, result := range results {
    fmt.Println(result)
}
```

---

## Variation: Batch Indexing Multiple Projects

A CI tool or platform service indexes multiple projects in sequence:

```go
projects := []struct {
    Name string
    Path string
}{
    {"auth-service", "/projects/auth-service"},
    {"api-gateway", "/projects/api-gateway"},
    {"frontend", "/projects/frontend"},
}

for _, p := range projects {
    log.Printf("Indexing %s...", p.Name)
    err := carto.Index(ctx, carto.IndexOptions{
        Project: p.Name,
        Path:    p.Path,
    })
    if err != nil {
        log.Printf("Failed to index %s: %v", p.Name, err)
        continue
    }
    log.Printf("Indexed %s successfully", p.Name)
}
```

Each project is indexed sequentially. Concurrent indexing of different projects is safe; concurrent indexing of the same project is not.

---

## Variation: Custom Source Configuration

Before indexing, configure external signal sources programmatically:

```go
sources, err := carto.Sources(ctx, "backend-api")
if err != nil {
    log.Fatal(err)
}

// Inspect or log current sources
fmt.Printf("Current sources: %+v\n", sources)

// Index with current source config
err = carto.Index(ctx, carto.IndexOptions{
    Project: "backend-api",
    Path:    "/projects/backend-api",
})
```

Sources are managed through the API or CLI and are read by the pipeline during the signals phase.

---

## Variation: Timeout-Bounded Indexing

For large codebases or resource-constrained environments, bound the indexing time:

```go
ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
defer cancel()

err := carto.Index(ctx, carto.IndexOptions{
    Project: "monorepo",
    Path:    "/projects/monorepo",
    Full:    true,
})
if errors.Is(err, context.DeadlineExceeded) {
    log.Println("Indexing timed out after 15 minutes")
    // Partial data from completed phases is available in Memories
}
```

---

## Variation: Embedding in a Web Service

A custom Go web service uses the SDK to provide on-demand indexing and querying:

```go
http.HandleFunc("/index", func(w http.ResponseWriter, r *http.Request) {
    project := r.URL.Query().Get("project")
    err := carto.Index(r.Context(), carto.IndexOptions{
        Project: project,
        Path:    filepath.Join("/projects", project),
    })
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }
    w.WriteHeader(200)
})

http.HandleFunc("/query", func(w http.ResponseWriter, r *http.Request) {
    results, err := carto.Query(r.Context(), carto.QueryOptions{
        Project: r.URL.Query().Get("project"),
        Query:   r.URL.Query().Get("q"),
        Tier:    "standard",
    })
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }
    json.NewEncoder(w).Encode(results)
})
```

---

## Postconditions

- The 7-layer semantic index is stored in Memories and available for queries.
- The calling program receives structured results from `Query()`.
- Errors are returned as Go error values, suitable for programmatic handling.
- Context cancellation propagates through the pipeline, cleaning up goroutines.

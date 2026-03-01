# Feature Handoff: SDK

**ID:** fh-009
**Feature:** SDK (Programmatic Go API)
**Components:** `pkg/carto/` — `carto.go`, `carto_test.go`

---

## Overview

The Carto SDK is a thin Go package (`pkg/carto/`) that provides programmatic access to Carto's core operations: indexing, querying, and source management. It is designed for embedding Carto's capabilities into other Go programs, custom tooling, or higher-level orchestration systems. The SDK wraps the underlying pipeline and storage packages, offering a simplified API surface.

---

## API Surface

### `Index()`

Triggers the 6-phase indexing pipeline for a project.

```go
import "github.com/user/carto/pkg/carto"

err := carto.Index(ctx, carto.IndexOptions{
    Project:     "myapp",
    Path:        "/path/to/myapp",
    Incremental: true,
    Module:      "",  // empty = all modules
})
```

**Parameters:**
- `ctx` — Context for cancellation and timeout control.
- `IndexOptions` — Configuration struct specifying project name, path, incremental mode, and optional module filter.

**Behavior:** Runs the full pipeline (scan, chunk+atoms, history+signals, deep analysis, store, skill files). Blocks until completion or context cancellation. Returns an error if any phase fails.

### `Query()`

Queries the semantic index stored in Memories.

```go
results, err := carto.Query(ctx, carto.QueryOptions{
    Project: "myapp",
    Query:   "How does authentication work?",
    Tier:    "standard",
    K:       5,
})
```

**Parameters:**
- `ctx` — Context for cancellation.
- `QueryOptions` — Project name, natural language query, retrieval tier, and result count.

**Returns:** A slice of result objects containing the matched context from the 7-layer index.

### `Sources()`

Retrieves or updates the external signal source configuration for a project.

```go
sources, err := carto.Sources(ctx, "myapp")
```

**Behavior:** Returns the current source configuration. To update sources, use the corresponding update function or configure via the API/CLI.

---

## Usage Patterns

### Basic Indexing and Querying

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/user/carto/pkg/carto"
)

func main() {
    ctx := context.Background()

    // Index
    err := carto.Index(ctx, carto.IndexOptions{
        Project: "myapp",
        Path:    "/projects/myapp",
    })
    if err != nil {
        log.Fatal(err)
    }

    // Query
    results, err := carto.Query(ctx, carto.QueryOptions{
        Project: "myapp",
        Query:   "What are the main API endpoints?",
        Tier:    "standard",
        K:       5,
    })
    if err != nil {
        log.Fatal(err)
    }

    for _, r := range results {
        fmt.Println(r)
    }
}
```

### Batch Indexing Multiple Projects

```go
projects := []string{"auth-service", "api-gateway", "frontend"}
for _, p := range projects {
    err := carto.Index(ctx, carto.IndexOptions{
        Project: p,
        Path:    filepath.Join("/projects", p),
    })
    if err != nil {
        log.Printf("Failed to index %s: %v", p, err)
    }
}
```

### Context-Based Cancellation

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
defer cancel()

err := carto.Index(ctx, carto.IndexOptions{
    Project: "large-monorepo",
    Path:    "/projects/monorepo",
})
if errors.Is(err, context.DeadlineExceeded) {
    log.Println("Indexing timed out")
}
```

---

## Configuration

The SDK reads configuration from the same environment variables as the CLI:

| Variable | Required | Description |
|----------|----------|-------------|
| `LLM_API_KEY` / `ANTHROPIC_API_KEY` | Yes (for indexing) | LLM provider API key. |
| `MEMORIES_URL` | Yes | Memories server URL. |
| `MEMORIES_API_KEY` | No | Memories API key. |

If these variables are not set, `Index()` fails with a configuration error. `Query()` requires only `MEMORIES_URL`.

---

## Edge Cases

- **Missing configuration:** If required environment variables (`LLM_API_KEY`, `MEMORIES_URL`) are not set, the SDK returns a descriptive error immediately rather than panicking.
- **Concurrent calls:** The SDK does not serialize calls internally. Calling `Index()` concurrently for the same project may cause data races in Memories storage. Callers should serialize index operations per project.
- **Nil context:** Passing a `nil` context to any SDK function causes a panic (standard Go behavior). Always pass at least `context.Background()`.
- **Large codebases:** `Index()` for very large projects can be long-running. Use `context.WithTimeout()` or `context.WithCancel()` to bound execution time.

---

## Common Questions

**Q1: When should I use the SDK vs. the CLI?**
Use the SDK when embedding Carto into a Go program (e.g., a custom CI tool, a code review bot, or a larger platform). Use the CLI for interactive use, scripts, and CI pipelines where shell commands are preferred.

**Q2: Is the SDK thread-safe?**
The SDK functions are safe to call from multiple goroutines, provided that concurrent index calls target different projects. Indexing the same project from multiple goroutines concurrently is not safe.

**Q3: How do I handle errors from the SDK?**
All SDK functions return Go errors. Wrap or inspect errors using `errors.Is()` and `errors.As()` for specific error types (e.g., `context.Canceled`, `context.DeadlineExceeded`). Configuration errors include the missing variable name.

**Q4: Can I use the SDK without a running Memories server?**
No. The SDK requires a Memories server for storing and retrieving the semantic index. Start Memories before calling SDK functions.

---

## Troubleshooting

| Symptom | Likely Cause | Resolution |
|---------|-------------|------------|
| Import error: package not found | Incorrect import path or module not downloaded. | Run `go mod tidy` and verify the import path matches the module's `go.mod`. |
| `nil pointer dereference` panic | Nil context or uninitialized options struct. | Always pass a valid context (e.g., `context.Background()`). Initialize all required fields in options structs. |
| Configuration error on `Index()` | `LLM_API_KEY` or `MEMORIES_URL` not set. | Set the required environment variables before calling SDK functions. |
| `Query()` returns empty results | Project not indexed, or wrong project name. | Index the project first. Verify the project name matches exactly. |
| `Index()` hangs | LLM or Memories server unresponsive. | Use `context.WithTimeout()` to bound execution. Check LLM and Memories availability. |

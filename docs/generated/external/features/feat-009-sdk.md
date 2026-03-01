# Go SDK

Carto provides a thin Go SDK in the `pkg/carto` package that lets you embed indexing and querying directly in your own Go programs. If you are building a tool, service, or plugin that needs codebase intelligence, the SDK gives you programmatic access without shelling out to the CLI or calling the REST API.

## Installation

Add Carto as a dependency:

```bash
go get github.com/divyekant/carto
```

Then import the SDK package:

```go
import "github.com/divyekant/carto/pkg/carto"
```

> **Note:** Carto uses tree-sitter for code parsing, so CGO must be enabled in your build environment. You need a C compiler (gcc) available.

## API Overview

The SDK exposes three main functions:

| Function | Description |
|----------|-------------|
| `Index()` | Scan and index a codebase, storing results in Memories |
| `Query()` | Search the semantic index with a natural-language question |
| `Sources()` | Get or update the external signal sources for a project |

## Examples

### Index a Codebase

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/divyekant/carto/pkg/carto"
)

func main() {
    ctx := context.Background()

    result, err := carto.Index(ctx, carto.IndexOptions{
        Name: "my-service",
        Path: "/repos/my-service",
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Indexed %d files in %s\n", result.FileCount, result.Duration)
}
```

To force a full re-index (skipping the incremental manifest):

```go
result, err := carto.Index(ctx, carto.IndexOptions{
    Name:      "my-service",
    Path:      "/repos/my-service",
    FullIndex: true,
})
```

### Query the Index

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/divyekant/carto/pkg/carto"
)

func main() {
    ctx := context.Background()

    answer, err := carto.Query(ctx, carto.QueryOptions{
        Project: "my-service",
        Question: "How does the rate limiter work?",
        Tier:     "standard", // "mini", "standard", or "full"
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(answer.Text)
    for _, src := range answer.Sources {
        fmt.Printf("  - %s\n", src.File)
    }
}
```

### Manage Sources

```go
// Get current sources
sources, err := carto.Sources(ctx, "my-service")
if err != nil {
    log.Fatal(err)
}
fmt.Printf("CI: %s\n", sources.CI)

// Update sources
err = carto.SetSources(ctx, "my-service", carto.SourcesConfig{
    CI:     "https://github.com/org/repo/actions",
    Issues: "https://github.com/org/repo/issues",
})
```

### Embedding in a Web Service

A common pattern is exposing Carto queries through your own API:

```go
http.HandleFunc("/ask", func(w http.ResponseWriter, r *http.Request) {
    q := r.URL.Query().Get("q")
    project := r.URL.Query().Get("project")

    answer, err := carto.Query(r.Context(), carto.QueryOptions{
        Project:  project,
        Question: q,
        Tier:     "standard",
    })
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }

    json.NewEncoder(w).Encode(answer)
})
```

## Configuration

The SDK reads the same environment variables as the CLI. You need to have these set before calling any SDK function:

| Variable | Required | Description |
|----------|----------|-------------|
| `LLM_API_KEY` or `ANTHROPIC_API_KEY` | Yes | API key for the LLM provider |
| `MEMORIES_URL` | Yes | URL of the Memories server |
| `MEMORIES_API_KEY` | If configured | API key for Memories |
| `LLM_PROVIDER` | No | `anthropic`, `openai`, or `ollama` (default: `anthropic`) |

See the `.env.example` file in the Carto repository for the full list.

## Limitations

- **Go only** — The SDK is a native Go package. For other languages, use the [REST API](feat-007-rest-api.md).
- **CGO required** — Because of the tree-sitter dependency, you must build with `CGO_ENABLED=1` and have a C compiler available.
- **Same dependencies** — The SDK requires a running Memories server and a valid LLM API key, just like the CLI.
- **No streaming** — Unlike the REST API's SSE-based indexing endpoint, the SDK's `Index()` function blocks until indexing is complete. For progress updates, use the REST API or CLI.

## Related

- [CLI Reference](feat-006-cli.md) — the command-line interface built on top of these same internals
- [REST API](feat-007-rest-api.md) — HTTP access for non-Go integrations
- [Docker Deployment](feat-010-docker-deployment.md) — run the dependencies (Memories) in containers

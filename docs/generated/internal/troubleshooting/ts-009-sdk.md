# Troubleshooting: SDK Issues

**ID:** ts-009
**Topic:** SDK Issues
**Components:** `pkg/carto/` — `carto.go`, `carto_test.go`

---

## Symptom: Import Error — Package Not Found

**Cause:** The import path does not match the module path in `go.mod`, or the module has not been downloaded.

**Resolution:**

1. Verify the import path matches the module's `go.mod` declaration:
   ```go
   import "github.com/user/carto/pkg/carto"
   ```

2. Download the module:
   ```bash
   go mod tidy
   ```

3. If the package is in a local workspace, ensure `go.work` or `replace` directives are correctly configured in `go.mod`.

4. If using a private repository, verify Go module proxy and authentication settings:
   ```bash
   export GONOSUMCHECK="github.com/user/carto"
   export GOPRIVATE="github.com/user/carto"
   ```

---

## Symptom: Nil Pointer Dereference / Nil Client Panic

**Cause:** A nil context was passed to an SDK function, or an options struct was not properly initialized.

**Resolution:**

1. Always pass a valid context:
   ```go
   // Correct
   ctx := context.Background()
   err := carto.Index(ctx, opts)

   // Wrong — will panic
   err := carto.Index(nil, opts)
   ```

2. Initialize all required fields in options structs:
   ```go
   opts := carto.IndexOptions{
       Project: "myapp",    // Required
       Path:    "/path/to", // Required
   }
   ```

3. If wrapping SDK calls in a function, ensure the context is propagated from the caller, not created as a zero value.

---

## Symptom: Configuration Error — Missing Environment Variables

**Cause:** The SDK requires `LLM_API_KEY` (or `ANTHROPIC_API_KEY`) and `MEMORIES_URL` to be set. `Index()` fails immediately if these are not present.

**Resolution:**

1. Set the required environment variables before running the program:
   ```bash
   export LLM_API_KEY="sk-ant-..."
   export MEMORIES_URL="http://localhost:8900"
   ```

2. Alternatively, set them programmatically before calling SDK functions:
   ```go
   os.Setenv("LLM_API_KEY", "sk-ant-...")
   os.Setenv("MEMORIES_URL", "http://localhost:8900")
   ```

3. The error message from the SDK includes the name of the missing variable. Check the error string for specifics.

4. `Query()` requires only `MEMORIES_URL`. It does not need an LLM API key.

---

## Symptom: `Query()` Returns Empty Results

**Cause:** The project has not been indexed, the project name is misspelled, or the Memories server has no data for that project.

**Resolution:**

1. Index the project before querying:
   ```go
   err := carto.Index(ctx, carto.IndexOptions{
       Project: "myapp",
       Path:    "/path/to/myapp",
   })
   ```

2. Verify the project name is exactly correct (case-sensitive).

3. Check that Memories is running and contains data:
   ```bash
   curl "$MEMORIES_URL/memory/search" -d '{"query":"project:myapp","k":1}'
   ```

4. Try a broader query or increase `K`:
   ```go
   results, err := carto.Query(ctx, carto.QueryOptions{
       Project: "myapp",
       Query:   "main",
       Tier:    "full",
       K:       20,
   })
   ```

---

## Symptom: `Index()` Hangs Indefinitely

**Cause:** The LLM provider or Memories server is unresponsive, and no timeout is set on the context.

**Resolution:**

1. Use a timeout-bounded context:
   ```go
   ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
   defer cancel()
   err := carto.Index(ctx, opts)
   ```

2. Check that the LLM provider is reachable:
   ```bash
   curl https://api.anthropic.com/v1/messages -H "x-api-key: $LLM_API_KEY" -H "anthropic-version: 2023-06-01"
   ```

3. Check that Memories is reachable:
   ```bash
   curl $MEMORIES_URL/health
   ```

4. If the hang persists, cancel the context and check the returned error for details.

---

## Symptom: Data Race When Indexing Concurrently

**Cause:** Two goroutines are calling `Index()` for the same project simultaneously. The SDK does not serialize per-project operations.

**Resolution:**

1. Serialize index calls per project using a mutex or channel:
   ```go
   var mu sync.Mutex
   mu.Lock()
   err := carto.Index(ctx, opts)
   mu.Unlock()
   ```

2. Concurrent indexing of different projects is safe:
   ```go
   // Safe — different projects
   go carto.Index(ctx, carto.IndexOptions{Project: "project-a", Path: "..."})
   go carto.Index(ctx, carto.IndexOptions{Project: "project-b", Path: "..."})
   ```

---

## Quick Reference

| Symptom | First Check |
|---------|-------------|
| Import error | `go mod tidy` and verify import path |
| Nil pointer panic | Pass `context.Background()`, not `nil` |
| Missing config error | `echo $LLM_API_KEY $MEMORIES_URL` |
| Empty query results | Has the project been indexed? |
| Index hangs | Use `context.WithTimeout()` and check LLM/Memories health |
| Data race | Serialize Index() calls per project |

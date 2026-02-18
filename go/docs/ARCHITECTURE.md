# Carto Architecture

## 1. System Overview

Carto is an intent-aware codebase intelligence tool. It scans a codebase
end-to-end, extracts semantic understanding using a two-tier LLM strategy
(Haiku for high-volume atom analysis, Opus for deep architectural analysis),
and stores layered context in [Memories](https://github.com/divyekant/memories) for tiered retrieval. The output is a
multi-layer knowledge graph that captures everything from individual function
summaries to system-wide architectural blueprints.

Carto is written in pure Go (module `github.com/anthropic/indexer`). The only
CGO dependency is Tree-sitter, which embeds C parsers for AST-based code
chunking. The system communicates with two external services over HTTP: the
Anthropic Messages API for LLM inference and a [Memories](https://github.com/divyekant/memories) server for
vector storage and retrieval.

### Core Capabilities

- **Automatic module detection** -- discovers project boundaries via manifest
  files (go.mod, package.json, Cargo.toml, pom.xml, etc.)
- **AST-based code chunking** -- Tree-sitter splits source files into logical
  units (functions, classes, methods, types) rather than arbitrary line ranges
- **Two-tier LLM analysis** -- Haiku summarizes individual code chunks cheaply;
  Opus performs expensive cross-module architectural analysis
- **Incremental indexing** -- SHA-256 manifest tracking ensures only changed
  files are re-indexed on subsequent runs
- **Tiered retrieval** -- mini (~5KB), standard (~50KB), and full (~500KB)
  tiers serve different query needs
- **Plugin-based signal system** -- extensible external context sources
  (git commits, PRs, tickets)

---

## 2. Pipeline Architecture

The pipeline is orchestrated by `internal/pipeline/pipeline.go` in the `Run()`
function. It accepts a `pipeline.Config` struct containing all dependencies
(LLM client, Memories client, signal registry, worker count, progress callback)
and returns a `pipeline.Result` with module counts, atom counts, analyses,
synthesis, and any non-fatal errors collected during execution.

### Phase 1: Scan

```
scanner.Scan(rootPath) --> ScanResult{Root, Files, Modules}
```

`scanner.Scan()` walks the file tree using `filepath.WalkDir`. It:

- Resolves the root path to an absolute path
- Loads `.gitignore` patterns from the root directory and respects them
  during traversal (supporting `*`, `**`, `?` globs, negation with `!`,
  directory-only patterns with trailing `/`, and anchored patterns)
- Skips hardcoded non-code directories: `node_modules`, `.git`,
  `__pycache__`, `vendor`, `dist`, `build`, `.carto`, `target`, `.next`,
  `.cache`
- Skips lock files: `package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`,
  `Gemfile.lock`, `Cargo.lock`, `go.sum`, `composer.lock`
- Detects language from file extension (40+ extensions mapped) and special
  filenames (`Dockerfile`, `Makefile`, `Jenkinsfile`, `Vagrantfile`, etc.)
- Calls `DetectModules()` to find module boundaries

**Module detection** scans the discovered files for manifest files and groups
all files under their nearest ancestor module:

| Manifest File       | Module Type     | Name Parser              |
|---------------------|-----------------|--------------------------|
| `go.mod`            | `go`            | Parses `module` directive |
| `package.json`      | `node`          | Parses `"name"` field     |
| `Cargo.toml`        | `rust`          | Parses `[package] name`   |
| `pom.xml`           | `java-maven`    | Directory name            |
| `build.gradle`      | `java-gradle`   | Directory name            |
| `build.gradle.kts`  | `java-gradle`   | Directory name            |
| `pyproject.toml`    | `python`        | Directory name            |
| `setup.py`          | `python`        | Directory name            |

Modules are sorted by path depth (deepest first) so that files are assigned
to the most specific enclosing module. If no manifest files are found, the
entire root directory is treated as a single module of type `"unknown"`.

An optional `ModuleFilter` in the pipeline config restricts indexing to a
single named module.

**Error handling**: Scan failure is the only fatal error in the pipeline --
if `scanner.Scan()` returns an error, the entire run aborts. All subsequent
phases collect errors into `Result.Errors` without halting execution.

### Phase 2: Chunk + Atoms

```
Files --> chunker.ChunkFile() --> []Chunk --> atoms.AnalyzeBatch() --> []*Atom
```

For each module's files (filtered for incremental changes if applicable):

1. **Chunking**: `chunker.ChunkFile()` uses Tree-sitter to parse the source
   file into an AST and extracts top-level declarations as logical chunks.
   Each chunk has a name, kind, language, file path, line range, and raw
   source code.

   Tree-sitter grammars are supported for six languages:
   - **Go**: `function_declaration`, `method_declaration`, `type_declaration`
   - **JavaScript/TypeScript**: `function_declaration`, `class_declaration`,
     `method_definition`, `export_statement`, `lexical_declaration`
   - **Python**: `function_definition`, `class_definition`
   - **Java**: `class_declaration`, `method_declaration`,
     `interface_declaration`
   - **Rust**: `function_item`, `impl_item`, `struct_item`, `enum_item`

   For unsupported languages, the entire file is returned as a single
   `"module"` chunk. If a supported language produces no extractable nodes,
   the whole file is also returned as a single chunk.

2. **Atom analysis**: `atoms.AnalyzeBatch()` sends each chunk to Haiku in
   parallel (controlled by a buffered channel semaphore with `MaxWorkers`
   slots). Each Haiku call produces an `Atom` containing:
   - `name` / `kind` -- carried from the chunk
   - `summary` -- 1-3 sentence description of purpose
   - `clarified_code` -- the code with cryptic variables renamed and inline
     comments added
   - `imports` / `exports` -- external dependencies and exposed symbols
   - `start_line` / `end_line` -- location in the source file

   Failed chunks are logged and skipped; nil entries are compacted out of
   the result slice.

This phase runs in parallel across modules. Each module spawns a goroutine
(rate-limited by the semaphore) that chunks all its files sequentially, then
passes the chunks to `AnalyzeBatch` which itself runs parallel goroutines
for the LLM calls.

### Phase 3: History + Signals

```
Files --> history.ExtractBulkHistory() --> []*FileHistory
Module --> signals.Registry.FetchAll() --> []Signal
```

Both operations run in parallel per module (same semaphore as Phase 2):

1. **Git history**: `history.ExtractBulkHistory()` runs `git log --follow`
   for each file (up to `MaxCommits=50`, `Since="6 months ago"`) and parses
   the output into structured `CommitInfo` records. Each `FileHistory`
   contains the file path, commit list, unique author list, and a churn
   score (number of commits as a complexity proxy). PR references are
   extracted from commit messages using regex matching for patterns like
   `#123`, `PR-456`, `PR 789`.

2. **Signals**: If a `SignalRegistry` is configured, `FetchAll()` queries
   every registered `SignalSource` for the module. Signals are typed
   (`"commit"`, `"pr"`, `"ticket"`, `"doc"`) and carry an ID, title, body,
   URL, linked files, date, and author. Individual source errors are logged
   but do not fail the pipeline.

### Phase 4: Deep Analysis

```
ModuleInputs --> analyzer.AnalyzeModules() --> []ModuleAnalysis
ModuleAnalyses --> analyzer.SynthesizeSystem() --> SystemSynthesis
```

The deep analyzer uses Opus for two stages:

1. **Per-module analysis**: `AnalyzeModules()` sends each module's atoms,
   history, and signals to Opus in parallel (same semaphore pattern). The
   prompt includes formatted atom summaries with imports/exports, file
   history with churn scores and authorship, and external signals. Opus
   returns a `ModuleAnalysis` containing:
   - `wiring` -- array of `Dependency{From, To, Reason}` describing
     cross-component connections
   - `zones` -- array of `Zone{Name, Intent, Files}` describing business
     domain groupings
   - `module_intent` -- 1-3 sentence summary of the module's purpose

2. **System synthesis**: `SynthesizeSystem()` takes all successful module
   analyses and sends them to Opus in a single call. The prompt includes
   each module's intent, zones, and wiring. Opus returns a
   `SystemSynthesis` containing:
   - `blueprint` -- narrative description of the overall system architecture
   - `patterns` -- array of coding conventions and architectural patterns

### Phase 5: Store

```
Layers --> storage.Store.StoreLayer() --> Memories
Manifest --> manifest.Save() --> .carto/manifest.json
```

The store phase persists all data to Memories and updates the manifest:

For each module, 5 layers are stored:
- `atoms` -- JSON-serialized atom array
- `history` -- JSON-serialized file history array
- `signals` -- JSON-serialized signal array
- `wiring` -- JSON-serialized dependency array (from module analysis)
- `zones` -- JSON-serialized zone array (from module analysis)

For the system as a whole (stored under module name `_system`), 2 layers:
- `blueprint` -- the synthesis blueprint string
- `patterns` -- JSON-serialized pattern array

Each layer is stored with a source tag formatted as:
```
carto/{project}/{module}/layer:{layer}
```

Content exceeding 49,000 characters is truncated at the last newline boundary
before the limit.

After storing all layers, the manifest is updated with SHA-256 hashes, file
sizes, and timestamps for every indexed file, then saved to
`{projectRoot}/.carto/manifest.json`.

---

## 3. Layered Context Graph

Carto organizes indexed knowledge into 7 distinct layers, each serving a
specific purpose in the context hierarchy:

### Layer 0: Map (File Tree)

- **Content**: File paths, modules, detected languages, file sizes
- **Source**: `scanner.Scan()` output
- **LLM cost**: None -- pure filesystem traversal
- **Purpose**: Structural skeleton of the codebase; enables module-aware
  queries and file filtering

### Layer 1a: Atoms

- **Content**: Per-chunk summaries, clarified code, imports, exports
- **Source**: `atoms.Analyzer.AnalyzeBatch()` via Haiku
- **LLM cost**: One Haiku call per code chunk (high volume, low cost)
- **Schema**: `atoms.Atom` -- `name`, `kind`, `file_path`, `summary`,
  `clarified_code`, `imports`, `exports`, `start_line`, `end_line`
- **Memories tag**: `carto/{project}/{module}/layer:atoms`
- **Purpose**: Semantic understanding of individual code units; the primary
  building block for all higher-level analysis

### Layer 1b: History

- **Content**: Git commits, churn scores, authorship patterns per file
- **Source**: `history.ExtractBulkHistory()` via `git log --follow`
- **LLM cost**: None -- direct git CLI output parsing
- **Schema**: `history.FileHistory` -- `FilePath`, `Commits[]` (hash,
  author, date, message, PR reference), `Authors[]`, `ChurnScore`
- **Memories tag**: `carto/{project}/{module}/layer:history`
- **Purpose**: Temporal context; identifies hot spots (high churn files),
  recent activity, and contributor patterns

### Layer 1c: Signals

- **Content**: External context from plugin sources (commits, PRs, tickets,
  docs)
- **Source**: `signals.Registry.FetchAll()` via registered `SignalSource`
  implementations
- **LLM cost**: None -- plugin-provided data
- **Schema**: `signals.Signal` -- `Type`, `ID`, `Title`, `Body`, `URL`,
  `Files`, `Date`, `Author`
- **Memories tag**: `carto/{project}/{module}/layer:signals`
- **Purpose**: Links code to external project context; connects files to
  tickets, pull requests, and documentation

### Layer 2: Wiring

- **Content**: Cross-component dependency graph with intent annotations
- **Source**: `analyzer.DeepAnalyzer.AnalyzeModule()` via Opus
- **LLM cost**: One Opus call per module (low volume, high cost)
- **Schema**: `analyzer.Dependency` -- `From` (source unit), `To` (target
  unit), `Reason` (why they are connected)
- **Memories tag**: `carto/{project}/{module}/layer:wiring`
- **Purpose**: Architectural connectivity; answers "what depends on what
  and why"

### Layer 3: Zones

- **Content**: Business domain groupings with purpose statements
- **Source**: `analyzer.DeepAnalyzer.AnalyzeModule()` via Opus (same call
  as wiring)
- **LLM cost**: Included in the per-module Opus call
- **Schema**: `analyzer.Zone` -- `Name` (domain name), `Intent` (purpose
  statement), `Files` (file paths belonging to this domain)
- **Memories tag**: `carto/{project}/{module}/layer:zones`
- **Purpose**: Business-domain understanding; groups technical files into
  logical functional areas

### Layer 4: Blueprint + Patterns

- **Content**: System-level architectural narrative and discovered coding
  conventions
- **Source**: `analyzer.DeepAnalyzer.SynthesizeSystem()` via Opus
- **LLM cost**: One Opus call for the entire project
- **Schema**: `analyzer.SystemSynthesis` -- `Blueprint` (narrative string),
  `Patterns` (array of pattern descriptions)
- **Memories tags**: `carto/{project}/_system/layer:blueprint` and
  `carto/{project}/_system/layer:patterns`
- **Purpose**: Highest-level understanding; provides the "executive summary"
  of the codebase and identifies recurring architectural patterns

---

## 4. Two-Tier LLM Strategy

Carto uses two model tiers to balance cost, speed, and analytical depth:

### Haiku Tier (Fast, Cheap)

- **Default model**: `claude-haiku-4-5-20251001`
- **Configurable via**: `CARTO_HAIKU_MODEL` environment variable
- **Used for**: Atom analysis (Layer 1a)
- **Call pattern**: One call per code chunk -- high volume
- **Max tokens**: 4,096 per call
- **System prompt**: `"You are a code analysis assistant. Respond only with valid JSON."`
- **Output**: Structured JSON with clarified code, summary, imports, exports

### Opus Tier (Powerful, Expensive)

- **Default model**: `claude-opus-4-6`
- **Configurable via**: `CARTO_OPUS_MODEL` environment variable
- **Used for**: Per-module deep analysis (Layer 2+3) and system synthesis
  (Layer 4)
- **Call pattern**: One call per module + one synthesis call -- low volume
- **Max tokens**: 8,192 per call
- **System prompts**:
  - Module analysis: `"You are a software architecture analyst. Analyze this module and respond with JSON."`
  - System synthesis: `"You are a senior software architect. Synthesize these module analyses into a system-level understanding. Respond with JSON."`
- **Output**: Structured JSON with wiring, zones, intent (module) or
  blueprint, patterns (synthesis)

### Concurrency Control

All LLM calls are gated by a buffered channel semaphore pattern. The
`llm.Client` maintains its own semaphore (`MaxConcurrent` slots, default 10)
that limits total in-flight API requests. On top of that, the pipeline and
each batch analyzer use their own semaphores (`MaxWorkers` slots, default 4)
to limit goroutine fan-out.

The semaphore pattern used throughout:

```go
sem := make(chan struct{}, maxWorkers)
// ...
sem <- struct{}{}        // acquire slot (blocks if full)
go func() {
    defer func() { <-sem }()  // release slot
    // do work
}()
```

### JSON Extraction

The LLM client's `CompleteJSON()` method extracts JSON from model responses
by:
1. Stripping markdown code fences (` ```json ... ``` `)
2. Finding the first `{` character
3. Walking forward to find the matching `}` while tracking brace depth
   and string escaping
4. Validating the extracted JSON with `json.Valid()`

This makes the system resilient to models wrapping JSON in prose or markdown.

---

## 5. Tiered Retrieval

The `storage.Store` supports three retrieval tiers, each including
progressively more context:

### Mini (~5KB)

- **Layers**: `zones`, `blueprint`
- **Use case**: Quick overview for simple questions ("What does this project
  do?", "What are the main domains?")
- **Content**: Business domain groupings and the system-level architectural
  narrative

### Standard (~50KB)

- **Layers**: `zones`, `blueprint`, `atoms`, `wiring`
- **Use case**: Most coding tasks ("How does authentication work?", "Where
  is the database layer?")
- **Content**: Everything in mini plus individual code unit summaries and
  the dependency graph

### Full (~500KB)

- **Layers**: `zones`, `blueprint`, `atoms`, `wiring`, `history`, `signals`
- **Use case**: Architectural decisions, deep investigation ("Why was this
  pattern chosen?", "Who has been working on this area?")
- **Content**: Everything in standard plus git history, churn scores,
  authorship data, and external signals

### Retrieval Mechanism

`Store.RetrieveByTier()` takes a module name and tier, then calls
`Store.RetrieveLayer()` for each layer in the tier. Each layer retrieval
uses `MemoriesClient.ListBySource()` to fetch all entries matching the
source tag `carto/{project}/{module}/layer:{layer}`.

Free-form queries bypass the tier system and use `MemoriesClient.Search()`
with hybrid (BM25 + vector) search across all stored memories.

---

## 6. Incremental Indexing

### Manifest Structure

The manifest is stored at `{projectRoot}/.carto/manifest.json` and tracks:

```go
type Manifest struct {
    Version   string                   // "1.0"
    Project   string                   // project name
    IndexedAt time.Time                // last indexing timestamp
    Files     map[string]FileEntry     // keyed by relative path
}

type FileEntry struct {
    Hash      string      // SHA-256 hex digest of file contents
    Size      int64       // file size in bytes
    IndexedAt time.Time   // when this file was last indexed
}
```

### Change Detection Flow

When `--incremental` is enabled:

1. **Load manifest**: `manifest.Load()` reads from `.carto/manifest.json`.
   If the file does not exist, a new empty manifest is created (first run
   proceeds as a full index).

2. **Detect changes**: For each module, `manifest.DetectChanges()` compares
   current files against the manifest:
   - **Added**: files present on disk but absent from the manifest
   - **Modified**: files whose SHA-256 hash differs from the manifest entry
   - **Removed**: files in the manifest but no longer on disk

3. **Process changes**:
   - Only `Added` and `Modified` files are sent through Phase 2-4
   - `Removed` files trigger `Store.ClearModule()` to delete their entries
     from Memories, and `Manifest.RemoveFile()` to remove them from the manifest

4. **Update manifest**: After successful indexing, each processed file's
   hash, size, and timestamp are updated via `Manifest.UpdateFile()`. The
   manifest is saved to disk at the end of Phase 5.

### Design Choice: Manifest-Based vs Git-Diff-Based

Carto uses manifest-based change detection rather than `git diff` because:
- It works for any version control system (or no VCS at all)
- It tracks the state of the index, not the state of the repository
- It handles the case where the index is older than the most recent commit
- SHA-256 content hashing is deterministic regardless of git state

---

## 7. Signal Plugin System

### Interface

```go
type SignalSource interface {
    Name() string
    Configure(cfg map[string]string) error
    FetchSignals(module Module) ([]Signal, error)
}
```

- `Name()` returns a unique identifier for the signal source (e.g., `"git"`)
- `Configure()` accepts key-value settings for runtime configuration
- `FetchSignals()` receives a `Module` (name, path, relative path, file list)
  and returns signals relevant to that module

### Signal Data Model

```go
type Signal struct {
    Type   string      // "commit", "pr", "ticket", "doc"
    ID     string      // "abc123", "#247", "JIRA-1892"
    Title  string
    Body   string
    URL    string
    Files  []string    // linked file paths
    Date   time.Time
    Author string
}
```

### Registry

The `signals.Registry` holds all configured sources and provides
`FetchAll(module)` which queries every registered source. Individual source
errors are silently skipped -- plugins are treated as optional enrichment.

### Built-in: GitSignalSource

`signals.GitSignalSource` is the built-in signal source. It:

1. Runs `git -C {repoRoot} log --pretty=format:%H|%an|%aI|%s -n20` for
   the module path
2. Parses each line into a `"commit"` signal
3. Extracts PR references from commit subjects using regex
   (`PR #42`, `#123`, `pull #7`) and creates additional `"pr"` signals
4. Sorts all signals newest-first
5. Returns an empty slice (not an error) if the directory is not inside a
   git repository

### Extensibility

To add a new signal source:

1. Implement the `SignalSource` interface
2. Register it in the `signals.Registry` before pipeline execution:
   ```go
   registry := signals.NewRegistry()
   registry.Register(signals.NewGitSignalSource(rootPath))
   registry.Register(myCustomSource)  // your implementation
   ```
3. The pipeline will call `FetchSignals()` for every module during Phase 3

---

## 8. Key Design Decisions

### Pure Go with Selective CGO

The project is written in pure Go with no CGO dependencies except Tree-sitter.
Tree-sitter requires CGO because its parser grammars are C libraries. This is
an acceptable tradeoff because:
- Tree-sitter provides production-grade AST parsing for 6 languages
- The alternative (regex-based chunking) would produce significantly worse
  chunks
- CGO is isolated to a single package (`internal/chunker`)

### HTTP-Based LLM Client (Not SDK)

The `llm.Client` communicates with the Anthropic API via raw HTTP requests
rather than using an SDK. This provides:
- Full control over OAuth token refresh flow (double-checked locking pattern)
- Custom header management (OAuth beta headers, User-Agent)
- Direct control over the request/response JSON schema
- No dependency on SDK release cycles
- Support for both API key and OAuth authentication modes

The client supports the `sk-ant-oat01-` prefix detection for automatic OAuth
mode switching.

### Memories as External Service

Memories is accessed via a REST API (`storage.MemoriesClient`) rather than embedded
as a library. This:
- Decouples storage from the indexing process
- Allows the Memories index to be shared across tools (CLI, IDE plugins, etc.)
- Avoids embedding a large C++ dependency
- Enables scaling the storage layer independently
- Uses a simple REST interface: `/memory/add`, `/memory/add-batch`,
  `/search`, `/memories`, `/memory/{id}` (DELETE)

Batch writes are chunked into groups of 500 for reliability.

### Manifest-Based Incremental Indexing

As described in Section 6, the manifest approach was chosen over git-diff-based
detection for VCS-agnostic operation and deterministic content-based tracking.

### Error Resilience

The pipeline follows a "collect errors, don't halt" philosophy:
- **Fatal**: Only `scanner.Scan()` failure aborts the run
- **Non-fatal**: Chunk failures, LLM call failures, Memories write failures,
  history extraction failures, and signal fetch failures are all collected
  in `Result.Errors` with logged warnings
- Modules or chunks that fail analysis are skipped; successfully processed
  items are still stored
- This ensures partial indexing is always better than no indexing

### Module-First Architecture

Every phase operates on a per-module basis:
- Files are grouped by module before any processing
- Chunks and atoms are scoped to modules
- History and signals are fetched per module
- Deep analysis runs per module
- Storage is tagged per module

This enables:
- Natural parallelism (modules are independent work units)
- Targeted re-indexing (`--module` flag)
- Module-scoped retrieval queries
- Incremental indexing at the module granularity

---

## 9. Data Flow Diagram

```
                         +------------------+
                         |   carto index    |
                         |   (CLI entry)    |
                         +--------+---------+
                                  |
                                  v
                    +-------------+-------------+
                    |     pipeline.Run(cfg)      |
                    +-------------+-------------+
                                  |
              +-------------------+-------------------+
              |                                       |
              v                                       |
    +---------+---------+                             |
    |   Phase 1: Scan   |                             |
    |  scanner.Scan()   |                             |
    +---------+---------+                             |
              |                                       |
              v                                       |
    +----+----+----+----+                             |
    | Module A | Module B | ...                       |
    +----+-----+----+----+                            |
         |          |                                 |
         v          v                                 |
    +----+----------+----+                            |
    |  Phase 2: Chunk +  |  (parallel per module)     |
    |      Atoms         |                            |
    |                    |                            |
    | chunker.ChunkFile  |                            |
    |        |           |                            |
    |        v           |                            |
    | atoms.AnalyzeBatch |---> Haiku LLM (per chunk)  |
    +----+----------+----+                            |
         |          |                                 |
         v          v                                 |
    +----+----------+----+                            |
    | Phase 3: History + |  (parallel per module)     |
    |     Signals        |                            |
    |                    |                            |
    | history.Extract    |---> git log                |
    | signals.FetchAll   |---> plugin sources         |
    +----+----------+----+                            |
         |          |                                 |
         v          v                                 |
    +----+----------+----+                            |
    | Phase 4: Deep      |                            |
    |   Analysis         |                            |
    |                    |                            |
    | AnalyzeModules     |---> Opus LLM (per module)  |
    |        |           |                            |
    |        v           |                            |
    | SynthesizeSystem   |---> Opus LLM (one call)    |
    +----+----------+----+                            |
         |          |                                 |
         v          v                                 v
    +----+----------+----+----+-----------+-----------+
    |          Phase 5: Store                         |
    |                                                 |
    |  store.StoreLayer(module, "atoms", ...)         |
    |  store.StoreLayer(module, "history", ...)       |
    |  store.StoreLayer(module, "signals", ...)       |
    |  store.StoreLayer(module, "wiring", ...)        |
    |  store.StoreLayer(module, "zones", ...)         |
    |  store.StoreLayer("_system", "blueprint", ...)  |
    |  store.StoreLayer("_system", "patterns", ...)   |
    |  manifest.Save()                                |
    +-------------------------------------------------+
              |
              v
    +---------+---------+
    |   Memories Server    |  (external HTTP service)
    |  + manifest.json  |  (local .carto/ directory)
    +-------------------+
```

---

## 10. Package Dependency Graph

```
cmd/carto/main.go
  |
  +-- internal/config         (environment variable loading)
  +-- internal/llm            (Anthropic API client)
  +-- internal/scanner        (file tree walking, module detection)
  +-- internal/manifest       (SHA-256 tracking, change detection)
  +-- internal/signals        (signal plugin registry)
  +-- internal/storage        (Memories client + Store abstraction)
  +-- internal/pipeline       (orchestrator)
  |     |
  |     +-- internal/scanner
  |     +-- internal/chunker  (Tree-sitter AST chunking)
  |     +-- internal/atoms    (Haiku-based atom analysis)
  |     +-- internal/analyzer (Opus-based deep analysis)
  |     +-- internal/history  (git log extraction)
  |     +-- internal/signals
  |     +-- internal/storage
  |     +-- internal/manifest
  |     +-- internal/llm
  |
  +-- internal/patterns       (CLAUDE.md / .cursorrules generation)


internal/atoms
  +-- internal/llm            (LLMClient interface, Tier constants)

internal/analyzer
  +-- internal/atoms           (Atom type for ModuleInput)
  +-- internal/history         (FileHistory type for ModuleInput)
  +-- internal/signals         (Signal type for ModuleInput)
  +-- internal/llm             (LLMClient interface, Tier constants)

internal/chunker
  +-- tree-sitter/go-tree-sitter          (core parser, CGO)
  +-- tree-sitter/tree-sitter-go          (Go grammar)
  +-- tree-sitter/tree-sitter-javascript  (JavaScript grammar)
  +-- tree-sitter/tree-sitter-typescript  (TypeScript grammar)
  +-- tree-sitter/tree-sitter-python      (Python grammar)
  +-- tree-sitter/tree-sitter-java        (Java grammar)
  +-- tree-sitter/tree-sitter-rust        (Rust grammar)

internal/storage
  +-- net/http                (Memories REST client)

internal/history
  +-- os/exec                 (git CLI subprocess)

internal/signals
  +-- os/exec                 (git CLI subprocess, GitSignalSource)

internal/scanner
  (no internal dependencies -- leaf package)

internal/config
  (no internal dependencies -- leaf package)

internal/manifest
  +-- crypto/sha256           (file hashing)

internal/patterns
  (no internal dependencies -- leaf package)
```

### Dependency Rules

- `internal/pipeline` is the only package that imports nearly everything.
  It is the orchestration layer.
- `internal/scanner`, `internal/config`, `internal/manifest`, and
  `internal/patterns` are leaf packages with no internal dependencies.
- `internal/atoms` and `internal/analyzer` depend on `internal/llm` only
  for the `Tier` type and `LLMClient` interface -- they do not import the
  concrete client.
- `internal/chunker` is the only package with external CGO dependencies
  (Tree-sitter).
- `cmd/carto` depends on `pipeline` for the `Run()` entry point, plus
  `scanner`, `storage`, `manifest`, and `config` for the other CLI commands
  (`modules`, `query`, `status`, `patterns`).

### External Dependencies

| Dependency                    | Purpose                        |
|-------------------------------|--------------------------------|
| `github.com/spf13/cobra`     | CLI framework                  |
| `github.com/tree-sitter/*`   | AST parsing (6 language grammars) |

All other functionality uses the Go standard library.

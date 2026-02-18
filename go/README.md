# Carto

**Intent-aware codebase intelligence for AI assistants.**

Carto scans your codebase, builds a layered semantic index using LLMs, and stores it in [Memories](https://github.com/divyekant/memories) for fast retrieval. It produces skill files (`CLAUDE.md`, `.cursorrules`) that give AI coding assistants instant, structured context about your project.

```
carto index .
# Scans 847 files across 3 modules in ~90 seconds
# Produces a 7-layer context graph stored in Memories
# Generates CLAUDE.md with architecture, patterns, and conventions
```

---

## Table of Contents

- [Quick Start](#quick-start)
- [How It Works](#how-it-works)
- [CLI Reference](#cli-reference)
- [Configuration](#configuration)
- [Architecture](#architecture)
- [Supported Languages](#supported-languages)
- [Contributing](#contributing)
- [License](#license)

---

## Quick Start

### Prerequisites

- Go 1.25 or later
- An [Anthropic API key](https://console.anthropic.com/) (standard `sk-ant-api03-` keys or OAuth `sk-ant-oat01-` tokens)
- A running [Memories](https://github.com/divyekant/memories) server (default: `http://localhost:8900`)

### Build

```bash
git clone https://github.com/divyekant/carto.git
cd carto
go build -o carto ./cmd/carto
```

### Configure

```bash
export ANTHROPIC_API_KEY="sk-ant-api03-..."
# Memories server defaults to http://localhost:8900 -- override if needed:
# export MEMORIES_URL="http://your-memories-server:8900"
```

### Run

```bash
# Index a codebase
carto index /path/to/your/project

# Query the index
carto query "How does authentication work?"

# Generate skill files for AI assistants
carto patterns /path/to/your/project --format all
```

---

## How It Works

Carto builds understanding through a **5-phase pipeline** that progressively layers meaning on top of raw code.

### The Pipeline

```
Phase 1: Scan        Walks the directory tree, respects .gitignore,
                     detects module boundaries (go.mod, package.json, etc.)

Phase 2: Chunk       Tree-sitter AST parsing splits files into semantic chunks.
         + Atoms     fast-tier LLM produces structured atom summaries for each chunk.

Phase 3: History     Extracts git history (commits, churn, ownership).
         + Signals   Plugin-based external signals (tickets, PRs, docs).

Phase 4: Deep        deep-tier LLM analyzes cross-component wiring, identifies
         Analysis    business domain zones, and produces an architecture narrative.

Phase 5: Store       Serializes all 7 layers into Memories with source tags.
                     Saves a manifest for incremental re-indexing.
```

### Layered Context Graph

Each layer captures a different dimension of understanding. Higher layers depend on lower ones.

| Layer | Name | LLM | Description |
|-------|------|-----|-------------|
| 0 | Map | None | Files, modules, detected languages |
| 1a | Atoms | Fast | Per-chunk summaries with intent and role annotations |
| 1b | History | None | Git commits, file churn, ownership patterns |
| 1c | Signals | None | External context from tickets, PRs, and other sources |
| 2 | Wiring | Deep | Cross-component dependency analysis |
| 3 | Zones | Deep | Business domain groupings and boundaries |
| 4 | Blueprint | Deep | System architecture narrative and design patterns |

### Tiered Retrieval

When querying, Carto returns context at three granularity levels:

| Tier | Layers Included | Approximate Size |
|------|----------------|-----------------|
| `mini` | Zones + Blueprint | ~5 KB |
| `standard` | + Atoms + Wiring | ~50 KB |
| `full` | + History + Signals | ~500 KB |

This lets AI assistants request just enough context for the task at hand -- a quick question needs `mini`, a refactoring task needs `full`.

---

## CLI Reference

### `carto index <path>`

Run the full indexing pipeline on a codebase.

```bash
carto index .                          # Index current directory
carto index /path/to/project           # Index a specific path
carto index . --incremental            # Only process changed files
carto index . --module my-service      # Index a single module
carto index . --project my-project     # Override the project name
carto index . --full                   # Force full re-index (ignore manifest)
```

| Flag | Description |
|------|-------------|
| `--incremental` | Only re-index files that changed since the last run |
| `--module <name>` | Restrict indexing to a single detected module |
| `--project <name>` | Set the project name (defaults to directory name) |
| `--full` | Force a complete re-index, ignoring the manifest |

### `carto query <text>`

Search the indexed codebase using natural language.

```bash
carto query "How does the payment flow work?"
carto query "error handling" --project my-api --tier full
carto query "database migrations" -k 20
```

| Flag | Description |
|------|-------------|
| `--project <name>` | Search within a specific project (enables tiered retrieval) |
| `--tier mini\|standard\|full` | Context tier for project-scoped queries (default: `standard`) |
| `-k <count>` | Number of results to return (default: `10`) |

### `carto modules <path>`

List all detected modules and their file counts.

```bash
carto modules .
```

Output shows each module's name, type (go, node, rust, etc.), path, and file count.

### `carto patterns <path>`

Generate skill files that give AI assistants structured context about your codebase.

```bash
carto patterns .                       # Generate all formats
carto patterns . --format claude       # Generate CLAUDE.md only
carto patterns . --format cursor       # Generate .cursorrules only
carto patterns . --format all          # Generate both (default)
```

| Flag | Description |
|------|-------------|
| `--format claude\|cursor\|all` | Output format (default: `all`) |

### `carto status <path>`

Show the current index status for a codebase.

```bash
carto status .
```

Displays the project name, last indexed timestamp, file count, and total indexed size.

### Global Flags

```bash
carto --version                        # Print version
carto --help                           # Print help
carto <command> --help                 # Print help for a command
```

---

## Configuration

Carto is configured entirely through environment variables.

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ANTHROPIC_API_KEY` | Yes | -- | Anthropic API key or OAuth token |
| `MEMORIES_URL` | No | `http://localhost:8900` | [Memories](https://github.com/divyekant/memories) server URL |
| `MEMORIES_API_KEY` | No | -- | Memories server API key |
| `CARTO_FAST_MODEL` | No | `claude-haiku-4-5-20251001` | Fast-tier model for atom analysis (Phase 2) |
| `CARTO_DEEP_MODEL` | No | `claude-opus-4-6` | Deep-tier model for deep analysis (Phase 4) |
| `CARTO_MAX_CONCURRENT` | No | `10` | Maximum concurrent LLM requests |
| `LLM_PROVIDER` | No | `anthropic` | LLM provider: `anthropic`, `openai`, `ollama` |
| `LLM_API_KEY` | No | -- | API key for non-Anthropic providers |
| `LLM_BASE_URL` | No | -- | Base URL for non-Anthropic providers |

### Authentication

Carto supports two authentication methods for the Anthropic API:

- **Standard API keys** (`sk-ant-api03-...`) -- used with the `X-Api-Key` header
- **OAuth tokens** (`sk-ant-oat01-...`) -- used with `Authorization: Bearer` header, with automatic token refresh

The authentication method is detected automatically from the key prefix.

---

## Architecture

```
cmd/carto/              CLI entry point (Cobra commands)
internal/
  analyzer/             Deep analysis (wiring, zones, blueprint)
  atoms/                Fast-tier atom summaries for code chunks
  chunker/              Tree-sitter AST chunking engine
  config/               Environment-based configuration loading
  history/              Git history extraction (commits, churn)
  llm/                  Multi-provider LLM client (Anthropic, OpenAI, Ollama)
  manifest/             Incremental indexing manifest (hash-based change detection)
  patterns/             Skill file generation (CLAUDE.md, .cursorrules)
  pipeline/             5-phase orchestrator wiring all components together
  scanner/              File discovery, .gitignore filtering, module detection
  signals/              Plugin-based external signal system (git, tickets, PRs)
  storage/              Memories REST client, layered storage, tiered retrieval
```

### Key Design Decisions

- **Tree-sitter for AST parsing** -- provides language-aware chunking that respects function and class boundaries, rather than naive line-based splitting.
- **Two-tier LLM strategy** -- The fast tier handles high-volume atom summaries (cheap), while the deep tier handles low-volume architectural analysis (thorough).
- **Layered storage with source tags** -- each layer is stored with a structured source tag (`carto/{project}/{module}/layer:{layer}`) enabling precise retrieval and cleanup.
- **Manifest-based incremental indexing** -- SHA-256 hashes track file changes so subsequent runs only process what changed.
- **Semaphore-based concurrency** -- a configurable concurrency limit prevents overwhelming the Anthropic API with parallel requests.

---

## Supported Languages

Carto recognizes and can parse files in the following languages. Tree-sitter grammars are bundled for the six primary languages marked below; all others are detected for file classification and included in the index as raw content.

### Tree-Sitter AST Parsing

| Language | Extensions |
|----------|-----------|
| Go | `.go` |
| JavaScript | `.js`, `.jsx`, `.mjs`, `.cjs` |
| TypeScript | `.ts`, `.tsx`, `.mts`, `.cts` |
| Python | `.py`, `.pyi` |
| Java | `.java` |
| Rust | `.rs` |

### Language Detection (30+ languages)

Carto detects and classifies files across a broad set of languages including C, C++, C#, Kotlin, Ruby, Swift, Scala, PHP, Dart, Elixir, Erlang, Haskell, OCaml, Clojure, Lua, Zig, R, and more. It also recognizes configuration formats (JSON, YAML, TOML, XML, Protobuf, Terraform), web languages (HTML, CSS, SCSS, Vue, Svelte, GraphQL), documentation (Markdown, reStructuredText), SQL, and shell scripts.

### Module Detection

Carto automatically identifies project boundaries by looking for manifest files:

| Manifest | Module Type |
|----------|-------------|
| `go.mod` | Go |
| `package.json` | Node.js |
| `Cargo.toml` | Rust |
| `pom.xml` | Java (Maven) |
| `build.gradle` / `build.gradle.kts` | Java (Gradle) |
| `pyproject.toml` / `setup.py` | Python |

If no manifest files are found, the entire directory is treated as a single module.

---

## Contributing

Contributions are welcome. Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on submitting issues and pull requests.

### Running Tests

```bash
go test ./...
```

### Building

```bash
go build -o carto ./cmd/carto
```

## Web UI

Carto includes a built-in web dashboard for browsing indexed projects, exploring modules, and querying the index visually.

```bash
carto serve --port 8950 --projects-dir /path/to/projects
```

Open `http://localhost:8950` in your browser.

---

## Docker

```bash
docker build -t carto .
docker run -p 8950:8950 \
  -e ANTHROPIC_API_KEY="sk-ant-api03-..." \
  -e MEMORIES_URL="http://host.docker.internal:8900" \
  -e MEMORIES_API_KEY="your-key" \
  -v /path/to/projects:/projects \
  carto
```

See [docker-compose.yml](docker-compose.yml) for a complete multi-service setup.

---

## Integrations

- **[QUICKSTART-LLM.md](integrations/QUICKSTART-LLM.md)** -- LLM-friendly quickstart guide for AI assistants
- **[Agent Write-Back](integrations/agent-writeback.md)** -- How to keep the index current from Claude Code, Codex, Cursor, and OpenClaw

---

## License

MIT License. See [LICENSE](LICENSE) for details.

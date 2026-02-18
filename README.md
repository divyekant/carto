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

## Quick Start

### Prerequisites

- Go 1.25+ (with CGO support for Tree-sitter)
- An LLM API key (Anthropic, OpenAI-compatible, or Ollama)
- A running [Memories](https://github.com/divyekant/memories) server (default: `http://localhost:8900`)

### Build & Run

```bash
git clone https://github.com/divyekant/carto.git
cd carto/go
go build -o carto ./cmd/carto

export ANTHROPIC_API_KEY="sk-ant-api03-..."
./carto index /path/to/your/project
```

### Docker

```bash
cd go
cp .env.example .env   # edit with your keys
docker compose up -d
# UI at http://localhost:8908
```

---

## How It Works

Carto runs a 5-phase pipeline:

| Phase | What | LLM Tier |
|-------|------|----------|
| **Scan** | Walk file tree, detect modules, apply .gitignore | None |
| **Chunk + Atoms** | Tree-sitter AST chunking → per-chunk summaries | Fast (high-volume, low-cost) |
| **History + Signals** | Git log extraction + plugin signals | None |
| **Deep Analysis** | Per-module wiring/zones + system synthesis | Deep (low-volume, high-cost) |
| **Store** | Persist 7 layers to Memories + update manifest | None |

The result is a **layered context graph** with 7 layers — from individual function summaries (atoms) up to system-wide architectural blueprints.

---

## Documentation

| Document | Description |
|----------|-------------|
| [Full README](go/README.md) | Complete docs — CLI reference, configuration, architecture |
| [Architecture](go/docs/ARCHITECTURE.md) | Deep technical architecture guide |
| [Contributing](go/CONTRIBUTING.md) | Development setup, code style, PR guidelines |
| [LLM Quickstart](go/integrations/QUICKSTART-LLM.md) | Machine-readable project summary for AI assistants |
| [Agent Write-back](go/integrations/agent-writeback.md) | How agents can write back to the index |

---

## Web UI

Carto includes a built-in React dashboard for managing indexing jobs:

```bash
cd go && ./carto serve
# Open http://localhost:8908
```

Features: real-time indexing progress, module browser, settings management, query interface.

---

## License

MIT

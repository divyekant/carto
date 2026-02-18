# Carto — Codebase Intelligence Setup

> **This document is designed to be fed directly to an LLM (Claude Code, Codex, OpenClaw, or any AI coding assistant) so it can set up and use Carto for codebase indexing.**

## What This Does

Carto scans your codebase, builds a layered semantic index using LLMs, and stores it in [Memories](https://github.com/divyekant/memories) for fast retrieval. After indexing, any AI assistant can query the index for structured context about your codebase.

After setup:
1. **Index** your codebase with `carto index .`
2. **Query** with natural language: `carto query "how does auth work?"`
3. **Generate** skill files: `carto patterns . --format all`
4. **Browse** via web UI: `carto serve`

---

## Prerequisites

```bash
# 1. Go 1.25+ installed
go version

# 2. Memories service running
curl -s http://localhost:8900/health | jq .

# 3. LLM API key set
echo $LLM_API_KEY  # or $ANTHROPIC_API_KEY
```

If Memories isn't running:
```bash
cd ~/projects/memories
docker compose up -d memories
```

---

## Install from Source

```bash
git clone https://github.com/divyekant/carto.git
cd carto/go
go build -o carto ./cmd/carto

# Verify
./carto --version
# carto v0.3.0
```

Note: Requires CGO (tree-sitter). On Alpine: `apk add gcc musl-dev`. On Ubuntu: `apt install gcc`.

---

## Install via Docker

```bash
# Clone and build
git clone https://github.com/divyekant/carto.git
cd carto/go
docker compose up -d

# Verify
curl -s http://localhost:8950/api/health | jq .
```

Docker requires a `.env` file or environment variables. See `.env.example` for all options.

---

## Configure

```bash
# Required: LLM API key (Anthropic, OpenAI, or Ollama)
export LLM_API_KEY="sk-ant-api03-..."

# Required: Memories server
export MEMORIES_URL="http://localhost:8900"
export MEMORIES_API_KEY="your-key-here"

# Optional: Change provider (default: anthropic)
# export LLM_PROVIDER="openai"
# export LLM_BASE_URL="https://api.openai.com"

# Optional: Override models
# export CARTO_FAST_MODEL="gpt-4.1-mini"
# export CARTO_DEEP_MODEL="gpt-4.1"
```

---

## Index a Codebase

```bash
# Full index
carto index /path/to/project

# Incremental (only changed files)
carto index /path/to/project --incremental

# Index a single module
carto index /path/to/project --module my-service

# Force full re-index
carto index /path/to/project --full
```

Indexing runs a 5-phase pipeline:
1. **Scan** — file discovery, .gitignore, module detection
2. **Chunk + Atoms** — tree-sitter AST splitting, fast-tier LLM summaries
3. **History + Signals** — git history, external signals
4. **Deep Analysis** — deep-tier cross-component analysis
5. **Store** — write all 7 layers to Memories

---

## Query the Index

```bash
# Basic search
carto query "how does authentication work?"

# Project-scoped with tier
carto query "error handling" --project my-api --tier standard

# More results
carto query "database migrations" -k 20
```

Tiers control how much context is returned:
- `mini` — zones + blueprint (~5KB)
- `standard` — + atoms + wiring (~50KB)
- `full` — + history + signals (~500KB)

---

## Generate Skill Files

```bash
# Generate CLAUDE.md and .cursorrules
carto patterns /path/to/project --format all

# CLAUDE.md only
carto patterns /path/to/project --format claude

# .cursorrules only
carto patterns /path/to/project --format cursor
```

Generated files include architecture overview, module descriptions, business domains, and coding patterns discovered during indexing.

---

## Web UI

```bash
carto serve --port 8950
# Open http://localhost:8950
```

The web UI provides:
- **Dashboard** — project overview, health status
- **Index** — trigger indexing with progress streaming
- **Query** — search with tier picker
- **Settings** — configure provider, models, Memories connection

---

## CLI Reference

| Command | Description |
|---------|-------------|
| `carto index <path>` | Run indexing pipeline |
| `carto query <text>` | Search the index |
| `carto modules <path>` | List detected modules |
| `carto patterns <path>` | Generate skill files |
| `carto status <path>` | Show index status |
| `carto serve` | Start web UI |
| `carto --version` | Print version |

---

## Environment Variables Reference

| Variable | Default | Description |
|----------|---------|-------------|
| `LLM_PROVIDER` | `anthropic` | LLM backend: anthropic, openai, openrouter, ollama |
| `LLM_API_KEY` | — | API key (takes priority over ANTHROPIC_API_KEY) |
| `ANTHROPIC_API_KEY` | — | Anthropic API key (fallback) |
| `LLM_BASE_URL` | — | Override API endpoint |
| `CARTO_FAST_MODEL` | `claude-haiku-4-5-20251001` | Model for fast/cheap analysis |
| `CARTO_DEEP_MODEL` | `claude-opus-4-6` | Model for deep/expensive analysis |
| `CARTO_MAX_CONCURRENT` | `10` | Max concurrent LLM requests |
| `MEMORIES_URL` | `http://localhost:8900` | Memories server URL |
| `MEMORIES_API_KEY` | — | Memories API key |

---

## Keeping the Index Current (Agent Write-Back)

After indexing, any AI agent can write discoveries back to Memories to keep the index fresh without re-running `carto index`.

**Source tag convention:** `carto/{project}/{module}/layer:atoms`

**Write a discovery:**
```bash
curl -s -X POST "$MEMORIES_URL/memory/add" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $MEMORIES_API_KEY" \
  -d '{
    "text": "The auth module now uses JWT tokens instead of session cookies. Updated in PR #42.",
    "source": "carto/my-project/auth-service/layer:atoms"
  }'
```

**When to write back:**
- After implementing a new feature or pattern
- After fixing a bug (document the root cause)
- After refactoring (document what changed and why)
- After discovering an undocumented convention

See [agent-writeback.md](agent-writeback.md) for integration guides for Claude Code, Codex, OpenClaw, and Cursor.

---

## Troubleshooting

### Build fails with CGO errors

```bash
# Tree-sitter requires CGO. Install a C compiler:
# macOS: xcode-select --install
# Ubuntu: sudo apt install gcc
# Alpine: apk add gcc musl-dev
```

### "connection refused" when indexing

```bash
# Check Memories server is running
curl -s http://localhost:8900/health

# Check your MEMORIES_URL is correct
echo $MEMORIES_URL
```

### Indexing is slow

```bash
# Increase concurrency (default: 10)
export CARTO_MAX_CONCURRENT=20

# Use incremental mode after first index
carto index . --incremental
```

### No results from query

```bash
# Check index exists
carto status /path/to/project

# Try full tier for broader results
carto query "your search" --project my-project --tier full
```

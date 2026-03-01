---
id: ra-v1.0.0
type: release-announcement
audience: marketing
version: 1.0.0
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Carto v1.0.0: Your AI Coding Assistants Finally Understand Your Codebase

AI coding assistants are brilliant — until they write code that ignores your architecture, breaks your conventions, and misses the patterns your team has spent years building. Today, Carto v1.0.0 changes that.

Carto automatically builds a deep, structured understanding of your codebase and delivers it to your AI assistants so they write code that belongs in your project from the first line.

## What's New

### Instant Codebase Understanding

Point Carto at any repository and it builds a complete semantic index in 90 seconds. No manual tagging. No configuration sprawl. Carto reads your code the way a senior engineer would — understanding not just what the code does, but how it connects, why it's structured that way, and what conventions matter.

### Context That Fits the Task

Not every question needs your entire codebase. Carto's tiered retrieval delivers exactly the right amount of context — a lightweight 5KB summary for quick fixes, a standard 50KB package for feature work, or a comprehensive 500KB deep-dive for architectural decisions. Your AI assistants stay fast and focused.

### Skill Files That Just Work

Carto automatically generates CLAUDE.md and .cursorrules files that plug directly into your existing AI workflow. The moment these files land in your repo, Claude and Cursor immediately know your project's patterns, API conventions, testing approach, and architecture. Zero manual setup.

### Code Meets Context

Your codebase doesn't exist in a vacuum. Carto connects to GitHub, Jira, Linear, Notion, and Slack to surface the decisions, discussions, and requirements behind your code. When an AI assistant sees a function, it also sees the ticket that requested it and the design doc that shaped it.

### One Tool, Four Interfaces

Whether your team prefers the command line, a REST API, a visual dashboard, or a Go SDK — Carto meets you where you work. Nine CLI commands for automation. Real-time streaming for programmatic access. A web dashboard for the whole team. An SDK for custom integrations.

## Improvements

- **Reliable large-codebase indexing** — Battle-tested against repositories with thousands of files; robust error recovery keeps indexing running even when individual files fail.
- **Deep cancellation support** — Cancel any in-progress operation cleanly, from the CLI, API, or dashboard. No orphaned processes, no locked resources.
- **Incremental indexing** — Only re-index files that have changed. Second runs complete in seconds, not minutes.
- **Multi-provider LLM flexibility** — Switch between Anthropic, OpenAI, OpenRouter, or local Ollama models without changing your workflow. Run fully air-gapped when compliance demands it.
- **Real-time progress streaming** — Watch indexing progress live via Server-Sent Events. Know exactly where things stand at every moment.

## Getting Started

### Build from Source

```
git clone https://github.com/anthropic/carto.git
cd carto
go build -o carto ./cmd/carto
./carto index /path/to/your/project
```

### Docker (Recommended)

```
docker compose up -d
```

One command brings up Carto and its Memories storage backend. Start indexing immediately.

### Generate Your First Skill Files

```
./carto index /path/to/your/project
```

Carto scans your code, builds the semantic index, and generates CLAUDE.md and .cursorrules files — ready to drop into your repository.

## What's Next

Carto v1.0.0 is the foundation. Here's where we're headed:

- **Expanded language support** — Deeper AST parsing for more languages, starting with C#, PHP, and Swift.
- **IDE plugins** — Native integrations for VS Code and JetBrains IDEs that deliver codebase intelligence without leaving your editor.
- **Team features** — Shared indexes, role-based access, and collaborative dashboards so your entire engineering organization benefits from a single source of codebase truth.
- **Continuous indexing** — Automatic re-indexing on every commit, keeping your AI assistants perpetually up to date.

---

Carto v1.0.0 is available now. Give your AI assistants the context they've been missing.

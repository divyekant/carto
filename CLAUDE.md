<!-- APOLLO:START - Do not edit this section manually -->
## Project Conventions (managed by Apollo)
- Language: go, package manager: go modules
- Commits: conventional style (feat:, fix:, chore:, etc.)
- Never auto-commit — always ask before committing
- Branch strategy: feature branches
- Code style: concise, comments: minimal
- Testing: TDD — write tests before implementation
- Test framework: go-test
- Run tests before every commit
- Product testing: use Delphi for ui, api, cli surfaces
- Design before code: always run brainstorming/design phase first
- Design entry: invoke conductor skill for all design/brainstorm work
- Code review required before merging
- Maintain README.md
- Maintain CHANGELOG.md
- Maintain a Quick Start guide
- Maintain architecture documentation
- Track decisions in docs/decisions/
- Update docs on: feature
- Versioning: semver
- Check for secrets before committing
<!-- APOLLO:END -->

# Carto Development Guide

## What This Is

Carto is an intent-aware codebase intelligence tool. It scans codebases, builds a 7-layer semantic index using LLMs, stores it in Memories, and generates skill files (CLAUDE.md, .cursorrules) for AI assistants.

## Build & Test

```bash
# Build (requires CGO for tree-sitter)
go build -o carto ./cmd/carto

# Run all tests
go test ./...

# Run tests with race detector
go test -race ./...

# Run only unit tests (no Memories server needed)
go test -short ./...
```

## Architecture

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the full architecture guide.

**Key concepts:**
- **6-phase pipeline:** Scan → Chunk+Atoms → History+Signals → Deep Analysis → Store → Skill Files
- **Two-tier LLM strategy:** Fast tier (high-volume atom summaries) + Deep tier (expensive cross-component analysis)
- **7-layer context graph:** Map → Atoms → History → Signals → Wiring → Zones → Blueprint
- **Tiered retrieval:** mini (~5KB), standard (~50KB), full (~500KB)

## Package Structure

All application code lives in `internal/`. CLI entry point is `cmd/carto/`.

| Package | Purpose |
|---------|---------|
| `analyzer` | Deep-tier analysis (layers 2-4: wiring, zones, blueprint) |
| `atoms` | Fast-tier atom extraction (layer 1a) |
| `chunker` | Tree-sitter AST-based code splitting |
| `config` | Environment variable loading |
| `history` | Git history extraction (layer 1b) |
| `llm` | LLM client with multi-provider support (Anthropic, OpenAI, Ollama) |
| `manifest` | Incremental indexing via SHA-256 file hashing |
| `patterns` | Skill file generation (CLAUDE.md, .cursorrules) |
| `pipeline` | 6-phase orchestrator with cancellation support |
| `scanner` | File discovery, .gitignore, module detection |
| `signals` | Plugin-based external signal collection (layer 1c) |
| `storage` | Memories REST client, layered storage, tiered retrieval |
| `server` | Web UI backend with embedded React SPA |

## Coding Standards

- **Format:** `gofmt` (or `goimports`). Run `go vet ./...` before committing.
- **Testing:** TDD. Write tests first. All tests must pass with `-race`. Unit tests use mocks for external deps (LLM, Memories).
- **Errors:** Return errors, don't panic. Wrap with context: `fmt.Errorf("package: %w", err)`.
- **CGO:** Required for tree-sitter. Build needs `gcc` and `musl-dev` (Alpine) or equivalent.
- **Commits:** Conventional Commits style (`feat:`, `fix:`, `test:`, `docs:`, `refactor:`, `chore:`).

## Environment

See [.env.example](.env.example) for all configuration variables.

Required: `LLM_API_KEY` or `ANTHROPIC_API_KEY`, plus a running Memories server for integration tests.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full contributor guide, including how to add new languages and signal sources.

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

<!-- BEGIN CARTO INDEX -->
# carto

## Architecture

CARTO is a codebase intelligence platform designed to index, analyze, and semantically query software projects. The system follows a self-contained, single-binary deployment architecture where a Go backend serves as the primary runtime, embedding a React single-page application (the 'web' module) directly into its compiled binary via Go's embed package. The frontend provides a comprehensive management interface for creating and managing projects, configuring data sources (local folders, potentially remote repositories), running indexing operations against codebases, performing semantic queries over indexed code, configuring LLM providers and integrations, and monitoring system health. The React SPA communicates with the Go backend exclusively through a centralized API client (`api.ts`) that handles authentication, RESTful data fetching, and Server-Sent Events (SSE) for real-time progress streaming during indexing operations. The root Go module (`github.com/divyekant/carto`) serves as the main application package that orchestrates the backend services—likely including an HTTP API server, indexing pipeline, vector/semantic search engine, LLM integration layer, and configuration management. The 'codex' module likely represents the core intelligence engine responsible for code parsing, embedding generation, and semantic indexing logic that powers the platform's code understanding capabilities. The overall business purpose is to provide developers and teams with an intelligent, self-hosted tool that can deeply index codebases and answer natural-language queries about code structure, patterns, and relationships using LLM-powered semantic search.

## Modules

### web ()
This module is the React-based single-page application frontend for the CARTO codebase intelligence platform. It provides a complete web UI for managing projects, running indexing operations, querying indexed codebases, configuring LLM providers and integrations, and monitoring system health. The compiled SPA is embedded into the Go binary via embed.go for self-contained deployment.

### github.com/divyekant/carto ()


### codex ()


## Business Domains

### Application Shell
Provides the top-level application structure including routing, authentication, theming, error boundaries, and the main navigation layout

Files:
- go/web/src/main.tsx
- go/web/src/App.tsx
- go/web/src/components/Layout.tsx
- go/web/src/components/AuthGuard.tsx
- go/web/src/components/ErrorBoundary.tsx
- go/web/src/components/ThemeProvider.tsx
- go/web/index.html
- go/web/src/index.css

### Pages
Implements the main application views: dashboard overview, project indexing, semantic querying, project details with source management, application settings, and about/help information

Files:
- go/web/src/pages/Dashboard.tsx
- go/web/src/pages/IndexRun.tsx
- go/web/src/pages/Query.tsx
- go/web/src/pages/ProjectDetail.tsx
- go/web/src/pages/Settings.tsx
- go/web/src/pages/About.tsx

### Feature Components
Reusable domain-specific components that encapsulate business logic for folder browsing, progress display, project cards, query results, data source editing, and content sections

Files:
- go/web/src/components/FolderPicker.tsx
- go/web/src/components/ProgressBar.tsx
- go/web/src/components/ProjectCard.tsx
- go/web/src/components/QueryResult.tsx
- go/web/src/components/SourcesEditor.tsx
- go/web/src/components/Section.tsx

### UI Primitives
Shadcn/ui-based design system components providing consistent, accessible, and themeable building blocks (buttons, cards, inputs, selects, tables, tabs, tooltips, etc.)

Files:
- go/web/src/components/ui/badge.tsx
- go/web/src/components/ui/button.tsx
- go/web/src/components/ui/card.tsx
- go/web/src/components/ui/input.tsx
- go/web/src/components/ui/label.tsx
- go/web/src/components/ui/progress.tsx
- go/web/src/components/ui/select.tsx
- go/web/src/components/ui/separator.tsx
- go/web/src/components/ui/switch.tsx
- go/web/src/components/ui/table.tsx
- go/web/src/components/ui/tabs.tsx
- go/web/src/components/ui/tooltip.tsx

### Infrastructure & Utilities
Shared API client with authentication handling, utility functions for class name merging, and the Go embed bridge that compiles the SPA into the server binary

Files:
- go/web/src/lib/api.ts
- go/web/src/lib/utils.ts
- go/web/embed.go

### Build & Configuration
Project configuration for Vite bundling, TypeScript compilation, ESLint linting, shadcn component library, and package dependency management

Files:
- go/web/vite.config.ts
- go/web/tsconfig.json
- go/web/tsconfig.app.json
- go/web/tsconfig.node.json
- go/web/eslint.config.js
- go/web/components.json
- go/web/package.json
- go/web/.gitignore
- go/web/README.md

## Coding Patterns

- Single-binary deployment: The React SPA is compiled via Vite and embedded into the Go binary using Go's embed package, eliminating the need for separate frontend hosting
- Centralized API client pattern: All frontend-backend communication flows through a single api.ts module that encapsulates authentication headers, base URL resolution, error handling, and both REST and SSE protocols
- Authentication guard pattern: An AuthGuard component wraps all routes to validate tokens against the backend API before rendering protected content
- Shell/Layout composition: The app follows a hierarchical composition of ThemeProvider → AuthGuard → ErrorBoundary → Layout → Page routes, establishing clear cross-cutting concern boundaries
- Shadcn/ui design system: UI primitives are built on the shadcn/ui component library pattern with Tailwind CSS, using a cn() utility (clsx + tailwind-merge) for conditional class name composition
- Page-level data fetching: Each page component is responsible for its own data fetching lifecycle via the shared API client, rather than using a global state management library
- Server-Sent Events (SSE) for real-time progress: Long-running indexing operations stream progress updates from backend to frontend via SSE, displayed through ProgressBar components
- Feature component encapsulation: Domain-specific components (FolderPicker, SourcesEditor, QueryResult) encapsulate both UI rendering and API interaction logic for their respective domains
- React Router SPA routing: Client-side routing maps URL paths to page components within a shared Layout shell
- Go module organization: The project uses Go modules with the root module serving as the main application entry point and sub-packages/modules for distinct capabilities (web frontend, codex intelligence engine)
- Theme management via React Context: A ThemeProvider component with useTheme hook provides application-wide dark/light theme toggling using React context
- Error boundary pattern: React error boundaries wrap route content to catch and gracefully handle rendering errors without crashing the entire application
- Card-based dashboard layout: The Dashboard and other pages use a Section/Card compositional pattern for organizing content into visually distinct, scannable regions

## Working with the Carto Index

This project is indexed by Carto. The index is stored in the Memories MCP server and provides semantic understanding of every code unit, cross-component wiring, and architectural patterns. **You MUST query it before editing and update it after changes.**

### Before Editing: Query for Context

Before modifying any file, search the index for existing knowledge about that code. This prevents regressions, respects existing patterns, and surfaces hidden dependencies.

**Using Memories MCP** (preferred):
```
memory_search({ query: "functionName OR fileName", hybrid: true, k: 5 })
```

**Using curl** (fallback):
```bash
curl -s -X POST "$MEMORIES_URL/search" \
  -H "Content-Type: application/json" -H "X-API-Key: $MEMORIES_API_KEY" \
  -d '{"query": "functionName OR fileName", "k": 5, "hybrid": true, "source_prefix": "carto/carto/"}'
```

**What to search for:** the function/class you are changing, the file path, and related component names to check wiring dependencies.

### After Changes: Write Back

After completing a feature, fix, or refactor, write the change back so the index stays current without a full re-index.

**Source tag convention:** `carto/carto/{module}/layer:{layer}`

Use `layer:atoms` for code-level changes. Use `layer:wiring` for new cross-component dependencies.

**Atom format** (match this exactly):
```
name (kind) in path/to/file.ext:startLine-endLine
Summary: What it does and why it exists
Imports: dep1, dep2
Exports: exportedSymbol
```

**Using Memories MCP** (preferred):
```
memory_add({
  text: "handleAuth (function) in src/auth/handler.go:15-42\nSummary: Validates JWT tokens and extracts user claims.\nImports: jwt, context\nExports: handleAuth",
  source: "carto/carto/MODULE_NAME/layer:atoms"
})
```

**Using curl** (fallback):
```bash
curl -s -X POST "$MEMORIES_URL/memory/add" \
  -H "Content-Type: application/json" -H "X-API-Key: $MEMORIES_API_KEY" \
  -d '{"text": "SUMMARY", "source": "carto/carto/MODULE_NAME/layer:atoms"}'
```

Replace `MODULE_NAME` with the relevant module.

**When to write back:** new functions/types, changed signatures, new dependencies, bug fixes that alter behavior, deleted code (note the deletion).

### Follow Discovered Patterns

The Coding Patterns section above reflects conventions discovered across this codebase. Follow them when writing new code. When you discover a new pattern, add it:

```
memory_add({ text: "Pattern: description", source: "carto/carto/_system/layer:patterns" })
```

---
*Generated by Carto v1.1.0*
<!-- END CARTO INDEX -->

# Contributing to Carto

Thank you for your interest in contributing to Carto. This guide covers everything
you need to get started, from setting up your development environment to submitting
a pull request.

## Prerequisites

- **Go 1.25+** -- Carto uses features from the latest Go release.
- **A running [Memories](https://github.com/divyekant/memories) server** -- The storage layer talks to a Memories. You can run one locally at `http://localhost:8900` (the default in
  `internal/config`). Unit tests mock this dependency, but integration tests in
  `internal/pipeline` expect a reachable server.
- **Anthropic API key** -- Required only for integration tests that exercise the
  LLM client (`internal/llm`, `internal/analyzer`, `internal/atoms`). Set the
  `ANTHROPIC_API_KEY` environment variable. Unit tests use mocks and do not require
  a key.

## Getting Started

```bash
# Clone the repository
git clone https://github.com/divyekant/indexer.git
cd indexer/go

# Build the CLI
go build -o carto ./cmd/carto

# Run all tests
go test ./...
```

Verify the build produces a `carto` binary and all tests pass before making changes.

## Development Workflow

1. Create a feature branch off `master`:
   ```bash
   git checkout -b feat/my-change master
   ```
2. Write tests first (TDD). Add or update test files alongside the code you are
   changing.
3. Run the full test suite frequently:
   ```bash
   go test ./...
   ```
4. When satisfied, push your branch and open a pull request against `master`.

## Code Style

Carto follows standard Go conventions. No external linters are required.

- Format all code with `gofmt` (or `goimports`).
- Run `go vet ./...` before committing to catch common issues.
- Keep exported identifiers well-documented with Go doc comments.
- Prefer returning errors over panicking.

## Package Overview

All application packages live under `internal/`.

| Package | Description |
|---------|-------------|
| `internal/analyzer` | Deep analysis phase (Layer 2-4). Uses Opus-tier LLM calls to produce wiring graphs, business zones, and system blueprints from atoms, history, and signals. |
| `internal/atoms` | Atom extraction phase (Layer 1a). Sends code chunks to Haiku-tier LLM to produce clarified, summarized code units with imports and dependencies. |
| `internal/chunker` | AST-based code splitting. Uses Tree-sitter grammars to break source files into logical chunks (functions, classes, types). |
| `internal/config` | Configuration loading from environment variables (Memories URL, API keys, model names, concurrency). |
| `internal/history` | Git history extraction (Layer 1b). Extracts per-file commit history, authorship, churn scores, and PR references. |
| `internal/llm` | Anthropic API client. Handles both API-key and OAuth authentication, model tiering (Haiku/Opus), and structured JSON responses. |
| `internal/manifest` | Incremental indexing manifest. Tracks file hashes and timestamps to detect changed, added, and deleted files between runs. |
| `internal/patterns` | Skill file generation (Layer 5). Produces CLAUDE.md and .cursorrules files from discovered architectural patterns, zones, and blueprints. |
| `internal/pipeline` | Pipeline orchestrator. Wires together scanning, chunking, atom analysis, history extraction, signal collection, deep analysis, and storage into a single indexing flow. |
| `internal/scanner` | File discovery and language detection. Walks the project tree respecting gitignore rules, detects languages by extension, and discovers module boundaries. |
| `internal/signals` | External signal collection (Layer 1c). Plugin-based system for fetching contextual signals (commits, PRs, tickets) via the `SignalSource` interface. |
| `internal/storage` | Memories storage layer. Stores and retrieves indexed data across tiered layers (atoms, history, signals, wiring, zones, blueprint, patterns). |

The CLI entry point lives in `cmd/carto`.

## Testing

- **Unit tests** use mocks and fakes for all external dependencies (LLM client,
  Memories server, git). They run without network access or API keys.
- **Integration tests** in `internal/pipeline` create temporary directories and
  exercise the full indexing flow. They require a Memories server and, for LLM-backed
  tests, an `ANTHROPIC_API_KEY`.
- All tests must pass with the race detector enabled:
  ```bash
  go test -race ./...
  ```
- Name test files with the `_test.go` suffix in the same package as the code
  under test.

## Commit Messages

Use [Conventional Commits](https://www.conventionalcommits.org/) style:

```
feat: add Kotlin tree-sitter grammar for chunker
fix: handle nil atoms slice in deep analyzer
test: add manifest round-trip test for deleted files
docs: update package overview in CONTRIBUTING.md
refactor: extract truncation helper in storage
chore: bump tree-sitter-go to v0.26.0
```

Keep the subject line under 72 characters. Use the body for additional context
when the change is non-obvious.

## How to Add a New Signal Source

Signal sources are plugins that fetch external context (PRs, tickets, docs) for a
module. To add one:

1. Create a new file in `internal/signals/` (e.g., `jira.go`).
2. Define a struct that implements the `SignalSource` interface from
   `internal/signals/source.go`:
   ```go
   type SignalSource interface {
       Name() string
       Configure(cfg map[string]string) error
       FetchSignals(module Module) ([]Signal, error)
   }
   ```
3. `Name()` returns a unique identifier (e.g., `"jira"`).
4. `Configure()` accepts provider-specific settings (API URLs, tokens).
5. `FetchSignals()` returns a slice of `Signal` structs for the given module.
6. Register your source in the pipeline setup so it is included in the signal
   registry.
7. Add tests. Use the `mockSignalSource` pattern from `git_test.go` as a
   reference for testing registry integration.

## How to Add a New Language

Carto uses Tree-sitter for AST-based chunking. To add support for a new language:

1. **Add the Tree-sitter grammar** to `go.mod`:
   ```bash
   go get github.com/tree-sitter/tree-sitter-ruby@latest
   ```
2. **Register the grammar in the chunker.** In `internal/chunker/chunker.go`,
   import the new grammar's Go bindings and add a case to the language-to-parser
   mapping so the chunker can parse files in that language.
3. **Register file extensions in the scanner.** In
   `internal/scanner/languages.go`, add entries to the `extToLanguage` map for
   each file extension the language uses (e.g., `".rb": "ruby"`). If the language
   has extensionless special files (like `Rakefile`), add a case to the
   `DetectLanguage` switch statement.
4. **Add chunking tests.** Create a small test fixture in `test-data/` and add a
   test case in `internal/chunker/chunker_test.go` that verifies chunks are
   extracted correctly for the new language.
5. **Run the full test suite** to make sure nothing breaks:
   ```bash
   go test ./...
   ```

## Code of Conduct

This project follows the [Contributor Covenant Code of Conduct](https://www.contributor-covenant.org/version/2/1/code_of_conduct/).
We are committed to providing a welcoming and inclusive environment for everyone.
Please report unacceptable behavior to the project maintainers.

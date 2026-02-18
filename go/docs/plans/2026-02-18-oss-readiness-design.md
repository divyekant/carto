# Carto OSS Readiness — Design

**Date:** 2026-02-18
**Status:** Approved

## Goals

Make Carto ready for open-source release by:
1. Removing Anthropic-specific naming from the provider-agnostic tier system
2. Removing hardcoded secrets and stale defaults
3. Enabling any AI agent to write back to the Carto index (avoiding costly re-indexing)
4. Adding proper documentation, CI, and contributor tooling

## A. Provider-Agnostic Tier Rename

The two-tier model concept (fast/cheap for volume work, powerful for deep analysis) is universal across all LLM providers. Rename to remove Anthropic-specific branding:

| Current | New | Scope |
|---------|-----|-------|
| `TierHaiku` | `TierFast` | Go const |
| `TierOpus` | `TierDeep` | Go const |
| `HaikuModel` / `OpusModel` | `FastModel` / `DeepModel` | Config struct, server handlers, JSON keys |
| `CARTO_HAIKU_MODEL` / `CARTO_OPUS_MODEL` | `CARTO_FAST_MODEL` / `CARTO_DEEP_MODEL` | Env vars |
| UI: "Haiku Model" / "Opus Model" | UI: "Fast Model" / "Deep Model" | Settings.tsx |

Defaults remain Anthropic model names when `LLM_PROVIDER=anthropic` — that's appropriate since they're the default provider.

**Files affected:** `internal/llm/client.go`, `internal/llm/anthropic.go`, `internal/llm/openai.go`, `internal/llm/ollama.go`, `internal/config/config.go`, `internal/atoms/analyzer.go`, `internal/analyzer/deep.go`, `internal/server/handlers.go`, `internal/pipeline/pipeline.go`, all `*_test.go` files, `web/src/pages/Settings.tsx`, `docker-compose.yml`, `README.md`, `docs/ARCHITECTURE.md`.

## B. Module Path Rename

Change Go module from `github.com/anthropic/indexer` to `github.com/divyekant/carto`.

Update: `go.mod`, all Go import paths, clone URLs in README and CONTRIBUTING.

## C. Hardcoded Cleanup

- Remove `god-is-an-astronaut` as default `MEMORIES_API_KEY` — set to empty string, require user to configure
- Remove hardcoded `MEMORIES_API_KEY` ENV from Dockerfile (rely on docker-compose or .env)
- Fix Settings.tsx placeholder model names to match actual defaults
- Fix Memories URL placeholder (`8951` → `8900`)

## D. Agent Write-Back Integration

### Problem
After `carto index` runs, AI agents working on the codebase make discoveries (new patterns, bug fixes, architectural decisions) that should update the index. Currently, the only way to update is re-running `carto index`, which is expensive (LLM calls).

### Solution
Agents can write directly to Memories using Carto's source tag convention:
- Source tag format: `carto/{project}/{module}/layer:{layer}`
- Agents POST to `{MEMORIES_URL}/memory/add` with the appropriate source tag
- This works for any agent that can make HTTP calls or run shell commands

### Implementation
1. **Extend `carto patterns`** — Generated CLAUDE.md and .cursorrules files include a "Keeping the Index Current" section with:
   - The project name and Memories URL
   - Source tag convention
   - Example curl/shell command for writing back
   - Which layer to use (`atoms` for code-level facts, `wiring` for cross-component discoveries)

2. **Create `integrations/agent-writeback.md`** — Universal guide any agent can follow, covering Claude Code hooks, Codex, OpenClaw skills, and Cursor instructions.

## E. Documentation

### `integrations/QUICKSTART-LLM.md`
LLM-friendly quickstart modelled on Memories' equivalent. Structured for direct copy-paste into any AI assistant's context. Covers: prerequisites, install, configure, index, query, patterns, Docker, env var reference, troubleshooting.

### `.env.example`
Document all environment variables with descriptions and example values.

### README Overhaul
- Update clone URL to `github.com/divyekant/carto`
- Remove `god-is-an-astronaut` from config table
- Update env var names (`CARTO_FAST_MODEL`, `CARTO_DEEP_MODEL`)
- Add `serve` command to CLI reference
- Add "Web UI" section
- Link to QUICKSTART-LLM.md

### Project `CLAUDE.md`
For AI assistants working ON Carto itself. Points to ARCHITECTURE.md and CONTRIBUTING.md with key rules.

## F. CI

`.github/workflows/ci.yml`:
- Trigger: push to master, PRs
- Jobs: `go build`, `go vet`, `go test -race ./...`
- Go version: 1.25

## G. Cleanup

Remove `docs/plans/` session artifacts (4 files). Keep `docs/ARCHITECTURE.md`.

---
*Design approved 2026-02-18*

# Carto OSS Readiness Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make Carto release-ready by removing Anthropic-specific naming, hardcoded secrets, and adding proper docs/CI/agent-writeback.

**Architecture:** Rename internal tier system from Haiku/Opus to Fast/Deep. Change Go module to github.com/divyekant/carto. Extend patterns generator for agent write-back. Add LLM quickstart and CI.

**Tech Stack:** Go 1.25, React/TypeScript, GitHub Actions, Docker

---

### Task 1: Tier Rename — Core LLM Package

**Files:**
- Modify: `internal/llm/client.go` (lines 18-19, 35-36, 68-72, 176-178, 235)
- Modify: `internal/llm/provider.go` (lines 41, 47)
- Modify: `internal/llm/anthropic.go` (lines 21, 23)
- Modify: `internal/llm/openai.go` (lines 17-18, 23, 27-28, 35, 37)
- Modify: `internal/llm/ollama.go` (lines 15-16, 21, 24-25, 32, 34)

**Step 1: Rename tier constants in client.go**

In `internal/llm/client.go`, replace:
- `TierHaiku Tier = "haiku"` → `TierFast Tier = "fast"`
- `TierOpus  Tier = "opus"` → `TierDeep Tier = "deep"`

In the `Options` struct, replace:
- `HaikuModel string` → `FastModel string`
- `OpusModel  string` → `DeepModel string`

In the `Complete` method, replace:
- `c.opts.HaikuModel` → `c.opts.FastModel`
- `c.opts.OpusModel` → `c.opts.DeepModel`

In the default model assignments:
- `opts.HaikuModel` → `opts.FastModel`
- `opts.OpusModel` → `opts.DeepModel`

In the OAuth thinking check:
- `TierOpus` → `TierDeep`

**Step 2: Rename in provider.go**

In `internal/llm/provider.go`, replace:
- `opts.HaikuModel, opts.OpusModel` → `opts.FastModel, opts.DeepModel` (2 occurrences at lines 41, 47)

**Step 3: Rename in anthropic.go**

In `internal/llm/anthropic.go`, replace:
- `TierHaiku` → `TierFast`
- `TierOpus` → `TierDeep`

**Step 4: Rename in openai.go**

In `internal/llm/openai.go`, replace all `haikuModel` → `fastModel` and `opusModel` → `deepModel` (struct fields, constructor params, usage in Complete method).

**Step 5: Rename in ollama.go**

In `internal/llm/ollama.go`, same as openai.go — replace all `haikuModel` → `fastModel` and `opusModel` → `deepModel`.

**Step 6: Run tests**

Run: `go test ./internal/llm/... 2>&1`
Expected: Compilation errors (tests still use old names). That's fine — we fix tests in a later step.

**Step 7: Commit**

```bash
git add internal/llm/
git commit -m "refactor: rename Haiku/Opus tiers to Fast/Deep in LLM package"
```

---

### Task 2: Tier Rename — Config, Atoms, Analyzer, Pipeline

**Files:**
- Modify: `internal/config/config.go` (lines 13-14, 26-27)
- Modify: `internal/atoms/analyzer.go` (line 84 + comments)
- Modify: `internal/analyzer/deep.go` (lines 132, 193 + comments)
- Modify: `internal/server/handlers.go` (lines 188-189, 206-207, 238-244, 334-335)
- Modify: `internal/pipeline/pipeline.go` (line 57 comment)
- Modify: `cmd/carto/main.go` (lines 114-115)

**Step 1: Rename config fields**

In `internal/config/config.go`, replace:
- `HaikuModel string` → `FastModel string`
- `OpusModel  string` → `DeepModel string`
- `envOr("CARTO_HAIKU_MODEL"` → `envOr("CARTO_FAST_MODEL"`
- `envOr("CARTO_OPUS_MODEL"` → `envOr("CARTO_DEEP_MODEL"`

**Step 2: Rename in atoms/analyzer.go**

Replace:
- `llm.TierHaiku` → `llm.TierFast`
- Comments: "Haiku analysis" → "fast-tier analysis", "through Haiku" → "through the fast tier", "sent to Haiku" → "sent to the fast tier"

**Step 3: Rename in analyzer/deep.go**

Replace:
- `llm.TierOpus` → `llm.TierDeep` (2 occurrences)
- Comments: "Opus calls" → "deep-tier calls", "runs Opus" → "runs deep-tier analysis", "to Opus" → "to the deep tier"

**Step 4: Rename in server/handlers.go**

Replace:
- `HaikuModel string` → `FastModel string` (configResponse + JSON tag `haiku_model` → `fast_model`)
- `OpusModel  string` → `DeepModel string` (configResponse + JSON tag `opus_model` → `deep_model`)
- All `cfg.HaikuModel` → `cfg.FastModel`
- All `cfg.OpusModel` → `cfg.DeepModel`
- PATCH handler cases: `"haiku_model"` → `"fast_model"`, `"opus_model"` → `"deep_model"`

**Step 5: Rename in pipeline/pipeline.go**

Replace comment "analyze with Haiku" → "analyze with fast-tier LLM"

**Step 6: Rename in cmd/carto/main.go**

Replace:
- `cfg.HaikuModel` → `cfg.FastModel`
- `cfg.OpusModel` → `cfg.DeepModel`

**Step 7: Commit**

```bash
git add internal/ cmd/
git commit -m "refactor: rename Haiku/Opus to Fast/Deep across config, pipeline, server"
```

---

### Task 3: Tier Rename — All Test Files

**Files:**
- Modify: `internal/config/config_test.go`
- Modify: `internal/llm/client_test.go`
- Modify: `internal/llm/provider_test.go`
- Modify: `internal/llm/anthropic_test.go`
- Modify: `internal/analyzer/deep_test.go`
- Modify: `internal/pipeline/pipeline_test.go`
- Modify: `internal/pipeline/integration_test.go`
- Modify: `internal/server/server_test.go`

**Step 1: Mechanical rename across all test files**

Global find-replace in all `*_test.go` files:
- `TierHaiku` → `TierFast`
- `TierOpus` → `TierDeep`
- `HaikuModel` → `FastModel`
- `OpusModel` → `DeepModel`
- `haiku_model` → `fast_model`
- `opus_model` → `deep_model`
- `haikuCnt` → `fastCnt`
- `opusCnt` → `deepCnt`
- `haiku, opus` → `fast, deep` (in getCounts return comments)
- Comment references: "Haiku" → "fast tier", "Opus" → "deep tier" where appropriate

**Step 2: Run full test suite**

Run: `go test ./... 2>&1`
Expected: ALL PASS

**Step 3: Commit**

```bash
git add internal/ cmd/
git commit -m "test: update all tests for Fast/Deep tier naming"
```

---

### Task 4: Tier Rename — Frontend Settings UI

**Files:**
- Modify: `web/src/pages/Settings.tsx`

**Step 1: Update Settings.tsx**

Replace:
- `haiku_model` → `fast_model` (interface field + all references)
- `opus_model` → `deep_model` (interface field + all references)
- Label "Haiku Model" → "Fast Model"
- Label "Opus Model" → "Deep Model"
- Placeholder `claude-3-5-haiku-latest` → `claude-haiku-4-5-20251001`
- Placeholder `claude-3-5-sonnet-latest` → `claude-opus-4-6`
- Placeholder `http://localhost:8951` → `http://localhost:8900`

**Step 2: Build frontend**

Run: `cd web && npm run build 2>&1`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add web/
git commit -m "refactor: rename Haiku/Opus to Fast/Deep in Settings UI, fix placeholders"
```

---

### Task 5: Module Path Rename

**Files:**
- Modify: `go.mod` (line 1)
- Modify: All .go files importing `github.com/anthropic/indexer`

**Step 1: Update go.mod**

Change: `module github.com/anthropic/indexer` → `module github.com/divyekant/carto`

**Step 2: Replace all import paths**

In every .go file, replace:
- `"github.com/anthropic/indexer/` → `"github.com/divyekant/carto/`

Files affected (11 files):
- `cmd/carto/main.go`
- `internal/analyzer/deep.go`
- `internal/analyzer/deep_test.go`
- `internal/atoms/analyzer.go`
- `internal/atoms/analyzer_test.go`
- `internal/pipeline/pipeline.go`
- `internal/pipeline/pipeline_test.go`
- `internal/pipeline/integration_test.go`
- `internal/server/server.go`
- `internal/server/server_test.go`
- `internal/server/handlers.go`

**Step 3: Verify build**

Run: `go build ./cmd/carto 2>&1`
Expected: Builds successfully

**Step 4: Run tests**

Run: `go test ./... 2>&1`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add go.mod internal/ cmd/
git commit -m "refactor: rename module to github.com/divyekant/carto"
```

---

### Task 6: Remove Hardcoded Secrets

**Files:**
- Modify: `internal/config/config.go` (line 24)
- Modify: `Dockerfile` (lines 49-50)
- Modify: `docker-compose.yml` (line 10)

**Step 1: Remove default API key in config.go**

Change: `envOr("MEMORIES_API_KEY", "god-is-an-astronaut")` → `os.Getenv("MEMORIES_API_KEY")`

**Step 2: Remove hardcoded ENV from Dockerfile**

Remove these two lines:
```
ENV MEMORIES_URL=http://host.docker.internal:8900
ENV MEMORIES_API_KEY=god-is-an-astronaut
```

**Step 3: Remove default in docker-compose.yml**

Change: `MEMORIES_API_KEY=${MEMORIES_API_KEY:-god-is-an-astronaut}` → `MEMORIES_API_KEY=${MEMORIES_API_KEY}`

Also update model env vars:
- `CARTO_HAIKU_MODEL` → `CARTO_FAST_MODEL`
- `CARTO_OPUS_MODEL` → `CARTO_DEEP_MODEL`

**Step 4: Commit**

```bash
git add internal/config/config.go Dockerfile docker-compose.yml
git commit -m "fix: remove hardcoded secrets, rename model env vars in Docker"
```

---

### Task 7: Create .env.example

**Files:**
- Create: `.env.example`

**Step 1: Write .env.example**

```env
# Carto — Environment Variables
# Copy this file to .env and fill in your values.

# LLM Provider: anthropic (default), openai, openrouter, ollama
LLM_PROVIDER=anthropic

# API key for your LLM provider
# For Anthropic: sk-ant-api03-... or OAuth token sk-ant-oat01-...
# For OpenAI: sk-...
# Takes priority over ANTHROPIC_API_KEY if both are set
LLM_API_KEY=

# Legacy Anthropic key (used if LLM_API_KEY is empty)
ANTHROPIC_API_KEY=

# Base URL override (required for OpenRouter, Ollama, self-hosted)
# OpenAI default: https://api.openai.com
# Ollama default: http://localhost:11434
LLM_BASE_URL=

# Model for fast/cheap analysis (atoms, summaries)
# Anthropic default: claude-haiku-4-5-20251001
# OpenAI example: gpt-4.1-mini
# Ollama example: llama3:8b
CARTO_FAST_MODEL=

# Model for deep/expensive analysis (wiring, zones, blueprint)
# Anthropic default: claude-opus-4-6
# OpenAI example: gpt-4.1
# Ollama example: llama3:70b
CARTO_DEEP_MODEL=

# Maximum concurrent LLM requests (default: 10)
CARTO_MAX_CONCURRENT=10

# Memories server URL
MEMORIES_URL=http://localhost:8900

# Memories server API key
MEMORIES_API_KEY=

# For Docker: directory to mount as /projects (read-only)
PROJECTS_DIR=~/projects
```

**Step 2: Commit**

```bash
git add .env.example
git commit -m "docs: add .env.example with all configuration variables"
```

---

### Task 8: Agent Write-Back — Extend Patterns Generator

**Files:**
- Modify: `internal/patterns/generator.go`

**Step 1: Add write-back section to GenerateCLAUDE**

After the "Coding Patterns" section and before the footer, add a "Keeping the Index Current" section that includes:
- The Memories source tag convention: `carto/{project}/{module}/layer:atoms`
- A curl command template for writing back discoveries
- Instructions: when the agent discovers new patterns, refactors, or fixes bugs, write a summary to the atoms layer

**Step 2: Add write-back section to GenerateCursorRules**

Same content adapted for .cursorrules format.

**Step 3: Run tests**

Run: `go test ./internal/patterns/... 2>&1`
Expected: PASS (existing tests should still pass; new section is additive)

**Step 4: Commit**

```bash
git add internal/patterns/
git commit -m "feat: add agent write-back instructions to generated skill files"
```

---

### Task 9: Create integrations/QUICKSTART-LLM.md

**Files:**
- Create: `integrations/QUICKSTART-LLM.md`

**Step 1: Write the LLM quickstart**

Model on the Memories QUICKSTART-LLM.md format. Include:
- Header explaining this is designed for direct LLM context injection
- Prerequisites (Go 1.25, Memories server, API key)
- Install from source (git clone, go build)
- Install via Docker (docker compose up)
- Configure (env vars)
- Index a codebase (`carto index .`)
- Query the index (`carto query "..."`)
- Generate skill files (`carto patterns .`)
- Web UI (`carto serve`)
- Env var reference table
- Agent write-back section (how any agent can update the index)
- Troubleshooting

**Step 2: Commit**

```bash
git add integrations/
git commit -m "docs: add LLM-friendly quickstart guide"
```

---

### Task 10: Create integrations/agent-writeback.md

**Files:**
- Create: `integrations/agent-writeback.md`

**Step 1: Write the universal agent write-back guide**

Cover all 4 agent types:
- **Claude Code**: Hook-based — shell script in `~/.claude/hooks/` that POSTs to Memories on Stop
- **Codex**: Same hooks format, symlink or copy to `~/.codex/hooks/`
- **OpenClaw**: Skill-based — skill file that calls Memories API
- **Cursor**: .cursorrules instruction — tell the agent to curl to Memories

Each section includes:
- The source tag convention
- Exact curl/shell commands
- Which layer to write to (atoms for code facts, wiring for cross-component)
- Example payloads

**Step 2: Commit**

```bash
git add integrations/
git commit -m "docs: add universal agent write-back integration guide"
```

---

### Task 11: README Overhaul

**Files:**
- Modify: `README.md`

**Step 1: Update README**

Changes:
- Clone URL: `github.com/divyekant/indexer` → `github.com/divyekant/carto`
- Config table: Remove `god-is-an-astronaut` default, rename `CARTO_HAIKU_MODEL`/`CARTO_OPUS_MODEL` to `CARTO_FAST_MODEL`/`CARTO_DEEP_MODEL`
- Add `serve` command to CLI reference section
- Add "Web UI" section describing the SPA and how to access it
- Add link to `integrations/QUICKSTART-LLM.md` in ToC
- Add link to `integrations/agent-writeback.md`
- Fix double-bracket link for Memories in Prerequisites
- Update architecture section package descriptions if any mention Haiku/Opus

**Step 2: Commit**

```bash
git add README.md
git commit -m "docs: overhaul README for OSS release"
```

---

### Task 12: Update CONTRIBUTING.md and ARCHITECTURE.md

**Files:**
- Modify: `CONTRIBUTING.md`
- Modify: `docs/ARCHITECTURE.md`

**Step 1: Update CONTRIBUTING.md**

- Clone URL: `github.com/divyekant/indexer` → `github.com/divyekant/carto`
- Any references to Haiku/Opus in package descriptions → Fast/Deep

**Step 2: Update ARCHITECTURE.md**

- Rename "Haiku Tier" → "Fast Tier", "Opus Tier" → "Deep Tier"
- Update env var names in examples
- Update model name defaults if shown

**Step 3: Commit**

```bash
git add CONTRIBUTING.md docs/ARCHITECTURE.md
git commit -m "docs: update CONTRIBUTING and ARCHITECTURE for tier rename"
```

---

### Task 13: Add GitHub Actions CI

**Files:**
- Create: `.github/workflows/ci.yml`

**Step 1: Write CI workflow**

```yaml
name: CI

on:
  push:
    branches: [master]
  pull_request:
    branches: [master]

jobs:
  build-and-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'

      - name: Install C dependencies
        run: sudo apt-get update && sudo apt-get install -y gcc

      - name: Build
        run: go build -o carto ./cmd/carto

      - name: Vet
        run: go vet ./...

      - name: Test
        run: go test -race -short ./...
```

Note: `-short` flag skips integration tests that require Memories server.

**Step 2: Commit**

```bash
git add .github/
git commit -m "ci: add GitHub Actions build, lint, and test workflow"
```

---

### Task 14: Create Project CLAUDE.md

**Files:**
- Create: `CLAUDE.md`

**Step 1: Write CLAUDE.md**

Brief project-level instructions for AI assistants working on Carto:
- Point to ARCHITECTURE.md and CONTRIBUTING.md
- Key rules: Go conventions (gofmt, go vet), TDD, CGO required (tree-sitter), conventional commits
- Package structure overview
- How to build and test
- Environment setup requirements

**Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add project CLAUDE.md for AI-assisted development"
```

---

### Task 15: Clean Up docs/plans/

**Files:**
- Delete: `docs/plans/2026-02-18-carto-ui-design.md`
- Delete: `docs/plans/2026-02-18-carto-ui-plan.md`
- Delete: `docs/plans/2026-02-18-v3-enhancements-design.md`
- Delete: `docs/plans/2026-02-18-v3-enhancements-plan.md`
- Keep: `docs/plans/2026-02-18-oss-readiness-design.md`
- Keep: `docs/plans/2026-02-18-oss-readiness-plan.md` (this file)

**Step 1: Remove session artifacts**

```bash
git rm docs/plans/2026-02-18-carto-ui-design.md docs/plans/2026-02-18-carto-ui-plan.md docs/plans/2026-02-18-v3-enhancements-design.md docs/plans/2026-02-18-v3-enhancements-plan.md
```

**Step 2: Commit**

```bash
git commit -m "chore: remove session planning artifacts from docs/plans"
```

---

### Task 16: Final Build + Verify + Rebuild Docker

**Step 1: Rebuild frontend**

Run: `cd web && npm run build`
Expected: Success

**Step 2: Rebuild Go binary**

Run: `go build -o carto ./cmd/carto`
Expected: Success

**Step 3: Run full test suite**

Run: `go test -race ./...`
Expected: ALL PASS

**Step 4: Verify Docker builds**

Run: `docker build -t carto:latest .`
Expected: Builds successfully

**Step 5: Bump version to 0.3.0**

In `cmd/carto/main.go`, change `var version = "0.2.0"` → `var version = "0.3.0"`
In `internal/patterns/generator.go`, update footer version.
In `internal/llm/client.go`, update UserAgent.

**Step 6: Final commit**

```bash
git add .
git commit -m "chore: bump version to 0.3.0 for OSS release"
```

**Step 7: Push to master**

```bash
git push origin master
```

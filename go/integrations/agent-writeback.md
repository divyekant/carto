# Carto Agent Write-Back Guide

> **Keep your codebase index fresh without re-running `carto index`.** This guide shows how any AI coding assistant can write discoveries back to Memories using Carto's source tag convention.

## How It Works

When Carto indexes a codebase, it stores structured context in Memories with source tags like:

```
carto/{project}/{module}/layer:{layer}
```

Any agent that can make HTTP requests (or run shell commands) can write new facts to the same tag structure. Memories' semantic search then includes these facts alongside the original index.

## Source Tag Convention

| Layer | Tag Pattern | When to Write |
|-------|-------------|---------------|
| `atoms` | `carto/{project}/{module}/layer:atoms` | New code patterns, function summaries, bug fixes |
| `wiring` | `carto/{project}/{module}/layer:wiring` | Cross-component dependencies, API contracts |
| `zones` | `carto/{project}/_system/layer:zones` | New business domain boundaries |
| `blueprint` | `carto/{project}/_system/layer:blueprint` | Architecture changes, design decisions |

**Most agent write-backs should use the `atoms` layer** — it's the most granular and gets included in `standard` and `full` tier queries.

## Write-Back API

**Endpoint:** `POST {MEMORIES_URL}/memory/add`

**Headers:**
- `Content-Type: application/json`
- `X-API-Key: {MEMORIES_API_KEY}`

**Body:**
```json
{
  "text": "Description of the discovery or change",
  "source": "carto/{project}/{module}/layer:atoms"
}
```

**Shell command:**
```bash
curl -s -X POST "$MEMORIES_URL/memory/add" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $MEMORIES_API_KEY" \
  -d '{
    "text": "The UserService.authenticate() method now validates JWT tokens using RS256. Moved from session-based auth in PR #87.",
    "source": "carto/my-project/api-server/layer:atoms"
  }'
```

---

## Setup by Agent Type

### Claude Code

Add a hook that writes back after each coding session.

**1. Create the hook script at `~/.claude/hooks/carto/writeback.sh`:**

```bash
#!/bin/bash
# Carto write-back hook for Claude Code
# Triggered on Stop (after each assistant turn with code changes)

MEMORIES_URL="${MEMORIES_URL:-http://localhost:8900}"
MEMORIES_API_KEY="${MEMORIES_API_KEY}"
PROJECT_NAME="${CARTO_PROJECT:-$(basename "$(pwd)")}"

# Skip if no API key configured
[ -z "$MEMORIES_API_KEY" ] && exit 0

# Read the conversation context from stdin
CONTEXT=$(cat)

# Only write back if there were code changes (check for file modification signals)
echo "$CONTEXT" | grep -q '"tool_use"' || exit 0

# Extract a summary and write to atoms layer
# The hook receives the full exchange; we POST it for extraction
curl -s -X POST "$MEMORIES_URL/memory/extract" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $MEMORIES_API_KEY" \
  -d "{
    \"messages\": $(echo "$CONTEXT" | jq -Rs .),
    \"source\": \"carto/$PROJECT_NAME/_system/layer:atoms\",
    \"context\": \"stop\"
  }" > /dev/null 2>&1
```

```bash
chmod +x ~/.claude/hooks/carto/writeback.sh
```

**2. Add to `~/.claude/settings.json`:**

```json
{
  "hooks": {
    "Stop": [{
      "matcher": "",
      "hooks": [{
        "type": "command",
        "command": "~/.claude/hooks/carto/writeback.sh",
        "timeout": 10
      }]
    }]
  }
}
```

If you already have Stop hooks, add this entry to the existing array.

**3. Set environment variables in `~/.zshrc`:**

```bash
export CARTO_PROJECT="my-project"  # or leave unset to use directory name
```

---

### Codex

Codex uses the same hooks format as Claude Code.

**Option A: Symlink**
```bash
mkdir -p ~/.codex/hooks/carto
ln -s ~/.claude/hooks/carto/writeback.sh ~/.codex/hooks/carto/writeback.sh
```

**Option B: Copy the hook and settings**

Copy `~/.claude/hooks/carto/` to `~/.codex/hooks/carto/` and merge hook config into `~/.codex/settings.json`.

---

### OpenClaw

OpenClaw uses skills instead of hooks. Add write-back instructions to your skill file:

**Add to your OpenClaw skill directory (`carto-writeback.md`):**

```markdown
# Carto Index Write-Back

When you make significant code changes, write a summary to the Carto index.

## When to Write Back
- After implementing a new feature
- After fixing a bug (include root cause)
- After refactoring (document what changed)
- After discovering an undocumented pattern

## How to Write Back

Use curl to POST to Memories:

\`\`\`bash
curl -s -X POST "$MEMORIES_URL/memory/add" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $MEMORIES_API_KEY" \
  -d '{
    "text": "YOUR DISCOVERY HERE",
    "source": "carto/PROJECT_NAME/MODULE_NAME/layer:atoms"
  }'
\`\`\`

Replace PROJECT_NAME with the current project and MODULE_NAME with the relevant module.
```

---

### Cursor

Cursor reads `.cursorrules` for project-specific instructions. Carto's `carto patterns` command generates `.cursorrules` with write-back instructions included automatically.

If you want to add write-back manually, append to your `.cursorrules`:

```
Carto Index Write-Back:
When making significant code changes, update the Carto index by running:
  curl -s -X POST "$MEMORIES_URL/memory/add" -H "Content-Type: application/json" -H "X-API-Key: $MEMORIES_API_KEY" -d '{"text": "SUMMARY", "source": "carto/PROJECT/MODULE/layer:atoms"}'
Replace SUMMARY with a concise description of what changed and why.
Write back after: new features, bug fixes, refactors, or pattern discoveries.
```

---

## What to Write Back

**Good write-backs:**
- "UserService.authenticate() now validates JWT tokens with RS256, replacing session cookies. See PR #87."
- "The payment module depends on the notification service for async receipt emails. Wired via event bus in events/payment.go."
- "Rate limiting is implemented as middleware in middleware/ratelimit.go using a token bucket algorithm with 100 req/min default."

**Skip these (not useful for the index):**
- Minor typo fixes
- Import reordering
- Formatting-only changes
- Dependency version bumps without behavioral changes

## Verifying Write-Backs

```bash
# Search for your write-backs
curl -s -X POST "$MEMORIES_URL/search" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $MEMORIES_API_KEY" \
  -d '{"query": "JWT authentication", "k": 5, "hybrid": true}' | jq '.results[].source'

# List all entries for a project
curl -s "$MEMORIES_URL/memory/list?source=carto/my-project&limit=20" \
  -H "X-API-Key: $MEMORIES_API_KEY" | jq '.memories[].source'
```

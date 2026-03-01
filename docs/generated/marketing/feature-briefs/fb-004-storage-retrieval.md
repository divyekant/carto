---
id: fb-004
type: feature-brief
audience: marketing
topic: Tiered Retrieval
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Feature Brief: Tiered Retrieval

## One-Liner

Get exactly the right amount of codebase context -- from a quick overview to a deep architectural analysis -- in milliseconds.

## What It Is

Carto's tiered retrieval system delivers context matched to task complexity. Need a quick overview for a simple fix? Get a concise summary. Working on a major refactor that touches multiple systems? Get the full architectural analysis. Every response is sized to be useful without overwhelming the AI assistant.

## Who It's For

**Primary:** Developers and AI coding assistants that need the right level of context for each task. A one-line bug fix doesn't need 500KB of architectural detail. A system redesign does.

**Secondary:** Platform and DevEx teams building internal tools that need structured access to codebase knowledge.

## The Problem

Context is a double-edged sword for AI assistants. Too much context overwhelms them -- they lose focus, hallucinate connections, and produce unfocused code. Too little context leaves them guessing -- they miss patterns, violate conventions, and ignore architectural boundaries.

Most tools offer one fixed level of detail. That's the wrong answer for every task except one.

## Key Benefits

- **Three tiers, perfectly sized.** Mini (~5KB) for quick lookups. Standard (~50KB) for everyday development. Full (~500KB) for deep architectural work. Each tier is curated, not just truncated.
- **Instant retrieval.** Every query returns in milliseconds. Context is pre-built and ready, not generated on demand.
- **Natural language queries.** Ask for what you need in plain language. Carto finds the relevant context across all seven layers of its semantic index.
- **Semantic, not keyword.** Carto understands what you're asking about, not just the words you use. Ask about "authentication flow" and get results even if your code calls it "session management."

## How It Works (Simplified)

Carto pre-builds its understanding across seven semantic layers -- from individual function summaries to system-wide architectural blueprints. When a query arrives:

1. **Match** -- Identify the most relevant parts of the codebase for the query.
2. **Size** -- Select the appropriate tier based on what's needed.
3. **Deliver** -- Return structured, curated context in milliseconds.

The result is context that's always relevant and always the right size for the task.

## Competitive Context

Competing tools offer flat, unstructured code search results. Carto is the only product that delivers curated, pre-analyzed context in tiers specifically designed for AI assistant consumption. The seven-layer semantic model ensures depth and relevance that keyword search cannot match.

## Suggested Messaging

**Announcement:** "Carto now delivers codebase context in three tiers -- from quick overviews to deep architectural analysis -- so AI assistants always get exactly the right amount of information for the task."

**Sales Pitch:** "Too much context overwhelms AI assistants. Too little makes them guess. Carto delivers exactly the right amount -- three curated tiers that match context to task complexity, retrieved in milliseconds."

**One-Liner:** "The right context, the right size, every time."

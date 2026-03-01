---
id: fb-002
type: feature-brief
audience: marketing
topic: Multi-Provider LLM Support
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Feature Brief: Multi-Provider LLM Support

## One-Liner

Use Carto with your preferred AI provider -- Anthropic, OpenAI, or run fully local with Ollama.

## What It Is

Carto's multi-provider architecture lets you choose which AI models power your codebase analysis. Switch providers with a single configuration change. Mix providers for cost optimization. Or run entirely on local models for complete data privacy.

## Who It's For

**Primary:** Engineering teams that need flexibility in AI provider selection -- whether for cost, performance, compliance, or strategic reasons.

**Secondary:** Organizations in regulated industries or air-gapped environments that require local-only AI processing.

## The Problem

Most AI-powered developer tools lock you into a single provider. When pricing changes, performance degrades, or compliance requirements shift, you're stuck. Migrating means rewriting integrations and retraining workflows.

Teams also face a cost problem: using the most capable model for every task is expensive and unnecessary. Simple analysis tasks don't need the same model as deep architectural reasoning.

## Key Benefits

- **No vendor lock-in.** Anthropic, OpenAI, and Ollama supported out of the box. Switch with a config change, not a migration.
- **Smart cost optimization.** Carto's two-tier strategy automatically uses fast, affordable models for high-volume work and reserves capable models for deep analysis. You get the best results at the lowest cost.
- **Local-first option.** Run Carto entirely on local models with Ollama. Your code never leaves your network. Perfect for air-gapped environments, regulated industries, or teams with strict data policies.
- **Future-proof.** As new providers and models emerge, Carto's architecture makes adding support straightforward. Your investment in Carto grows with the AI ecosystem.

## How It Works (Simplified)

Carto separates the intelligence layer from the provider layer. You configure which provider handles which tier of analysis:

- **Fast tier** -- High-volume tasks like summarizing individual functions. Uses affordable, fast models.
- **Deep tier** -- Complex tasks like mapping cross-component relationships. Uses the most capable models available.

You can use the same provider for both tiers, or mix providers to optimize cost and performance.

## Competitive Context

Most competing tools are built exclusively on one provider's API, creating hard dependencies. Carto is provider-agnostic by design, with a two-tier cost optimization strategy that no competitor offers.

## Suggested Messaging

**Announcement:** "Carto now supports Anthropic, OpenAI, and Ollama -- use any AI provider to power your codebase understanding, or run fully local for complete data privacy."

**Sales Pitch:** "Don't let your code intelligence tool lock you into one AI vendor. Carto works with Anthropic, OpenAI, or local models -- and its two-tier strategy cuts costs by using the right model for each task."

**One-Liner:** "Your AI provider, your choice. Carto works with all of them."

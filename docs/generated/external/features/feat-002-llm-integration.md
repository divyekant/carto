---
id: feat-002
type: feature-doc
audience: external
topic: LLM Providers
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# LLM Providers

Carto uses large language models to understand your code. It sends code chunks to an LLM for summarization, cross-component analysis, and architectural reasoning. You choose which LLM provider to use -- Anthropic (the default), OpenAI, or Ollama for fully local processing.

Under the hood, Carto uses a two-tier LLM strategy. A **fast tier** handles high-volume work like summarizing individual functions and types. A **deep tier** handles expensive, cross-cutting analysis like mapping how modules connect and identifying architectural patterns. You can configure different models for each tier.

## How to Use It

For most users, all you need is an API key:

```bash
export LLM_API_KEY="your-api-key"
carto index .
```

That's it. Carto defaults to Anthropic's Claude and selects appropriate models for each tier automatically.

## Configuration

All LLM settings are configured through environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `LLM_PROVIDER` | Which provider to use: `anthropic`, `openai`, or `ollama` | `anthropic` |
| `LLM_API_KEY` | API key for your chosen provider | -- |
| `ANTHROPIC_API_KEY` | Anthropic-specific API key (used if `LLM_API_KEY` is not set) | -- |
| `LLM_BASE_URL` | Custom API endpoint (useful for proxies or Ollama) | Provider default |
| `CARTO_FAST_MODEL` | Model for fast-tier work (atom summaries) | Provider default |
| `CARTO_DEEP_MODEL` | Model for deep-tier analysis (architecture, wiring) | Provider default |
| `CARTO_MAX_CONCURRENT` | Maximum concurrent LLM requests | `10` |

## Examples

**Anthropic (default):**

```bash
export LLM_API_KEY="sk-ant-..."
carto index .
```

No additional configuration needed. Carto picks appropriate Claude models for both tiers.

**OpenAI:**

```bash
export LLM_PROVIDER="openai"
export LLM_API_KEY="sk-..."
carto index .
```

**Ollama (fully local, no API key needed):**

```bash
export LLM_PROVIDER="ollama"
export LLM_BASE_URL="http://localhost:11434"
export CARTO_FAST_MODEL="llama3"
export CARTO_DEEP_MODEL="llama3"
carto index .
```

With Ollama, everything runs on your machine. No data leaves your network. You'll need to have Ollama installed and running with your chosen model pulled.

**Custom model selection:**

```bash
export LLM_API_KEY="sk-ant-..."
export CARTO_FAST_MODEL="claude-haiku-4-5"
export CARTO_DEEP_MODEL="claude-sonnet-4"
carto index .
```

You can mix and match. Use a smaller, cheaper model for the high-volume fast tier and a more capable model for deep analysis.

**Controlling concurrency:**

```bash
export CARTO_MAX_CONCURRENT=5
carto index .
```

Lower this if you're hitting rate limits. Raise it if your provider supports higher throughput and you want faster indexing.

## Limitations

- **API access required:** You need a valid API key for Anthropic or OpenAI. Ollama requires a local installation.
- **Costs vary:** Indexing a codebase makes many LLM calls. Costs depend on your provider, model selection, and codebase size. The fast tier generates the most calls (one per code chunk). Use `--incremental` (the default) to minimize re-processing.
- **Ollama performance:** Local models are free but slower and may produce lower-quality analysis compared to cloud providers, depending on the model you choose.

## Related

- [Indexing Pipeline](feat-001-indexing-pipeline.md) -- how indexing works end to end
- [Environment Configuration](.env.example) -- full list of environment variables

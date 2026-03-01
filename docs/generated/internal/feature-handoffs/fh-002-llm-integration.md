---
id: fh-002
type: feature-handoff
audience: internal
topic: LLM Integration
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Feature Handoff: LLM Integration

## What It Does

The `llm/` package provides a unified HTTP-based client for making LLM calls across multiple providers. It supports Anthropic, OpenAI-compatible APIs, OpenRouter, and Ollama. The system uses a two-tier model strategy: a fast tier for high-volume, low-cost operations (atom extraction) and a deep tier for expensive, high-quality analysis (wiring, zones, blueprint synthesis).

The client is HTTP-based rather than SDK-based, giving full control over request construction, retry logic, and response parsing.

## How It Works

### Multi-Provider Architecture

The LLM client detects the configured provider via `LLM_PROVIDER` (or infers it from the API key prefix) and adjusts its HTTP request format accordingly:

| Provider | API Format | Auth Header | Base URL (default) |
|---|---|---|---|
| Anthropic | Anthropic Messages API | `X-Api-Key` | `https://api.anthropic.com` |
| OpenAI | OpenAI Chat Completions | `Authorization: Bearer` | `https://api.openai.com` |
| OpenRouter | OpenAI-compatible | `Authorization: Bearer` | `https://openrouter.ai/api` |
| Ollama | OpenAI-compatible | None | `http://localhost:11434` |

The provider selection is transparent to the rest of the codebase. Pipeline code calls the same `Complete()` or `CompleteJSON()` methods regardless of the underlying provider.

### Two-Tier Model Strategy

| Tier | Purpose | Default Model | Usage |
|---|---|---|---|
| Fast | High-volume atom extraction | `claude-haiku-4-5-20251001` | Phase 2: one call per code chunk |
| Deep | Cross-component analysis, synthesis | `claude-opus-4-6` | Phase 4: per-module + system-level |

The fast tier optimizes for throughput and cost. It processes hundreds or thousands of chunks per indexing run. The deep tier optimizes for reasoning quality. It handles complex analysis tasks that require understanding relationships across an entire module or system.

### Authentication

**API Key Mode (default):**
The system reads `ANTHROPIC_API_KEY` or `LLM_API_KEY` and sends it in the appropriate header for the configured provider.

If both `LLM_API_KEY` and `ANTHROPIC_API_KEY` are set, `LLM_API_KEY` takes precedence.

**OAuth Mode:**
If the API key starts with `sk-ant-oat01-`, the client switches to OAuth mode automatically. In OAuth mode:

- The key is treated as a Bearer token sent in the `Authorization` header.
- Token refresh is handled automatically when the token expires.
- Double-checked locking prevents race conditions during concurrent refresh: a mutex guards the refresh check, but only one goroutine performs the actual refresh while others wait.

### JSON Extraction

The `CompleteJSON()` method is a convenience wrapper that:

1. Sends the prompt to the LLM.
2. Strips markdown code fences (` ```json ... ``` `) from the response if present.
3. Extracts valid JSON by brace/bracket matching -- finds the first `{` or `[` and matches it to the corresponding closing delimiter.
4. Returns the extracted JSON string.

This handles the common case where LLMs wrap JSON output in markdown formatting or include preamble text before the actual JSON.

### Concurrency Control

A semaphore limits the number of concurrent LLM calls to `CARTO_MAX_CONCURRENT` (default: 10). This prevents overwhelming the provider's rate limits and controls local resource usage. The semaphore is shared across both tiers.

## Configuration

| Variable | Default | Description |
|---|---|---|
| `LLM_PROVIDER` | `anthropic` | Provider name: `anthropic`, `openai`, `openrouter`, `ollama` |
| `LLM_API_KEY` | -- | API key (takes precedence over `ANTHROPIC_API_KEY`) |
| `ANTHROPIC_API_KEY` | -- | Anthropic-specific API key |
| `LLM_BASE_URL` | Provider-specific | Override the base URL for any provider |
| `CARTO_FAST_MODEL` | `claude-haiku-4-5-20251001` | Model for fast-tier (atom extraction) |
| `CARTO_DEEP_MODEL` | `claude-opus-4-6` | Model for deep-tier (analysis, synthesis) |
| `CARTO_MAX_CONCURRENT` | `10` | Max concurrent LLM calls (semaphore size) |

### Provider-Specific Notes

**Anthropic:** Uses the Anthropic Messages API (`/v1/messages`). Requires `anthropic-version` header. Supports both API key and OAuth authentication.

**OpenAI / OpenRouter:** Uses the Chat Completions API (`/v1/chat/completions`). API key sent as `Authorization: Bearer`. Model names must match the provider's catalog.

**Ollama:** Uses the OpenAI-compatible endpoint (`/v1/chat/completions`). No authentication required by default. Models must be pulled locally first (`ollama pull <model>`). Set `LLM_BASE_URL=http://localhost:11434` if not using the default port.

## Edge Cases

| Scenario | Behavior |
|---|---|
| OAuth token expires mid-indexing | The client detects the 401 response, acquires the refresh mutex, checks if another goroutine already refreshed (double-checked locking), refreshes if needed, and retries the request. |
| Both `LLM_API_KEY` and `ANTHROPIC_API_KEY` set | `LLM_API_KEY` takes precedence. `ANTHROPIC_API_KEY` is ignored. |
| Model name not available on the provider | The provider returns an error (usually 404 or 400). The error propagates to the pipeline as a non-fatal failure for that specific call. |
| LLM returns non-JSON when `CompleteJSON()` is used | The brace-matching extraction fails. An error is returned indicating that no valid JSON was found in the response. |
| LLM returns truncated JSON (hit token limit) | The brace-matching extraction fails because the closing delimiter is missing. An error is returned. |
| Network timeout | The HTTP client has a configurable timeout. Timeouts are surfaced as errors to the caller. |
| Rate limit (429) response | The error is propagated to the caller. The pipeline collects it as a non-fatal error. No automatic retry with backoff is performed at the LLM client level. |
| Empty API key | The client initializes but all calls will fail with authentication errors. Detected early during config validation. |

## Common Questions

**Q1: How do I switch from Anthropic to OpenAI?**
Set `LLM_PROVIDER=openai`, `LLM_API_KEY=sk-...` (your OpenAI key), and update `CARTO_FAST_MODEL` and `CARTO_DEEP_MODEL` to OpenAI model names (e.g., `gpt-4o-mini` for fast, `gpt-4o` for deep). The client automatically adjusts its request format.

**Q2: What are the cost implications of the two-tier strategy?**
The fast tier handles the bulk of LLM calls (one per code chunk, potentially hundreds per run). Using a cheaper, faster model here keeps costs low. The deep tier makes far fewer calls (one per module + one for system synthesis) but uses a more capable (and expensive) model. For a 200-file project, expect roughly 200 fast-tier calls and 5-10 deep-tier calls.

**Q3: What happens when the LLM provider is completely down?**
The scan phase completes normally (no LLM dependency). Phases 2 and 4 will produce zero results. History and signals (Phase 3) also complete normally. The pipeline finishes with partial results and many errors in `Result.Errors`. Re-index when the provider recovers.

**Q4: When should I use OAuth vs API key?**
API key authentication is simpler and suitable for most deployments. OAuth (`sk-ant-oat01-` prefix) is used when the key comes from an OAuth flow, such as integration with the Anthropic Console or third-party auth providers. The client detects the mode automatically from the key prefix -- no configuration change needed.

**Q5: Can I use custom or fine-tuned model names?**
Yes. Set `CARTO_FAST_MODEL` and `CARTO_DEEP_MODEL` to any model identifier that the configured provider accepts. The client passes the model name through to the API without validation. If the provider rejects the model name, the error will surface during the first LLM call.

## Troubleshooting

| Symptom | Likely Cause | Resolution |
|---|---|---|
| 401 Unauthorized on all LLM calls | API key is invalid, expired, or for the wrong provider | Verify the key. Test it with a direct `curl` call to the provider's API. |
| 429 Rate Limit Exceeded | Too many concurrent requests or account-level rate limit hit | Reduce `CARTO_MAX_CONCURRENT`. Check your account's rate limits on the provider dashboard. Wait and retry. |
| JSON parse errors from `CompleteJSON()` | LLM returned non-JSON, truncated JSON, or unexpected format | Check the raw LLM response. The model may need a stronger prompt to produce valid JSON. Try a different model. |
| Timeout errors | Network latency, large prompts, or slow model | Check network connectivity. Ensure `LLM_BASE_URL` is correct. Try a faster model for the affected tier. |
| OAuth refresh fails repeatedly | Refresh token expired or revoked | Re-authenticate to obtain a new OAuth token. Check token expiration settings. |
| Ollama calls fail with "connection refused" | Ollama server is not running | Start Ollama: `ollama serve`. Verify the port matches `LLM_BASE_URL`. |

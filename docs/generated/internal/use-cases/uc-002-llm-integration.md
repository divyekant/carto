---
id: uc-002
type: use-case
audience: internal
topic: LLM Provider Configuration
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Use Case: LLM Provider Configuration

## Trigger

An administrator or developer configures the LLM provider by setting environment variables before running Carto.

## Preconditions

1. A valid API key for the target provider is available.
2. The target provider is reachable from the network (relevant for cloud providers; Ollama must be running locally).
3. The desired model names are known and available on the provider's platform.

## Primary Flow: Anthropic (Default)

### Step 1: Set Environment Variables

```bash
export ANTHROPIC_API_KEY="sk-ant-api03-..."
# Optional overrides:
# export CARTO_FAST_MODEL="claude-haiku-4-5-20251001"
# export CARTO_DEEP_MODEL="claude-opus-4-6"
# export CARTO_MAX_CONCURRENT=10
```

### Step 2: Client Initialization

When the pipeline starts, the LLM client reads the configuration:

1. Detects `LLM_PROVIDER` (defaults to `anthropic` if not set).
2. Reads the API key from `LLM_API_KEY` first; if not set, falls back to `ANTHROPIC_API_KEY`.
3. Checks the key prefix: `sk-ant-oat01-` triggers OAuth mode; all other prefixes use API key mode.
4. Sets the base URL to `https://api.anthropic.com` (unless `LLM_BASE_URL` overrides it).
5. Initializes the concurrency semaphore with `CARTO_MAX_CONCURRENT` slots.

### Step 3: Request Construction

For each LLM call, the client:

1. Acquires a semaphore slot (blocks if all slots are in use).
2. Constructs an HTTP POST to the Anthropic Messages API endpoint.
3. Sets headers: `X-Api-Key`, `anthropic-version`, `Content-Type: application/json`.
4. Sends the request with the configured model name and prompt.
5. Parses the response. For `CompleteJSON()`, strips markdown fences and extracts JSON by brace matching.
6. Releases the semaphore slot.

### Step 4: Pipeline Execution

The pipeline makes LLM calls in two phases:
- **Phase 2 (Atoms):** Many concurrent calls to the fast-tier model.
- **Phase 4 (Deep Analysis):** Fewer calls to the deep-tier model.

Both phases share the same semaphore pool.

## Variation: OpenAI-Compatible Provider

### Step 1: Set Environment Variables

```bash
export LLM_PROVIDER="openai"
export LLM_API_KEY="sk-..."
export CARTO_FAST_MODEL="gpt-4o-mini"
export CARTO_DEEP_MODEL="gpt-4o"
```

### Differences from Primary Flow

- Request format changes to OpenAI Chat Completions API (`/v1/chat/completions`).
- Auth header changes to `Authorization: Bearer <key>`.
- The `anthropic-version` header is not sent.
- Response parsing adjusts to the OpenAI response structure (choices array).

## Variation: OpenRouter

### Step 1: Set Environment Variables

```bash
export LLM_PROVIDER="openrouter"
export LLM_API_KEY="sk-or-..."
export CARTO_FAST_MODEL="anthropic/claude-haiku-4-5-20251001"
export CARTO_DEEP_MODEL="anthropic/claude-opus-4-6"
```

### Differences from Primary Flow

- Uses OpenAI-compatible request format.
- Base URL defaults to `https://openrouter.ai/api`.
- Model names use the provider-prefixed format (e.g., `anthropic/claude-haiku-4-5-20251001`).
- OpenRouter-specific headers may be included for attribution.

## Variation: Ollama (Local)

### Step 1: Start Ollama and Pull Models

```bash
ollama serve
ollama pull llama3.2
ollama pull deepseek-coder-v2
```

### Step 2: Set Environment Variables

```bash
export LLM_PROVIDER="ollama"
export LLM_BASE_URL="http://localhost:11434"
export CARTO_FAST_MODEL="llama3.2"
export CARTO_DEEP_MODEL="deepseek-coder-v2"
```

### Differences from Primary Flow

- No API key required (authentication is not used by default).
- Models must be pulled locally before use.
- Performance depends on local hardware. Concurrency may need to be reduced (`CARTO_MAX_CONCURRENT=2`) to avoid overloading the local machine.
- Quality of atom extraction and deep analysis depends on the chosen models' capabilities.

## Edge Cases

| Scenario | Behavior |
|---|---|
| Both `LLM_API_KEY` and `ANTHROPIC_API_KEY` are set | `LLM_API_KEY` takes precedence. `ANTHROPIC_API_KEY` is ignored. |
| Invalid API key format | The client initializes without error. The first LLM call returns a 401, which propagates as a non-fatal pipeline error. |
| `LLM_PROVIDER` is set but `LLM_API_KEY` is missing | For Anthropic, falls back to `ANTHROPIC_API_KEY`. For other providers, all calls fail with auth errors. |
| Ollama is not running | All LLM calls fail with "connection refused". The pipeline completes with zero atoms and zero analysis. |
| Model name does not exist on the provider | The provider returns an error (400 or 404). The error surfaces per-call as a non-fatal failure. |
| OAuth key (`sk-ant-oat01-`) used with a non-Anthropic provider | The OAuth prefix detection is Anthropic-specific. The key is sent as a Bearer token, which the non-Anthropic provider will likely reject. |

## Data Impact

The LLM integration itself does not persist data. It produces in-memory results (atom structures, analysis objects) that are consumed by downstream pipeline phases. The LLM provider may log requests on their side according to their data retention policies.

## Post-Conditions

1. The LLM client is configured and ready for the pipeline to use.
2. Both fast-tier and deep-tier models are accessible (or errors are surfaced on first call).
3. Concurrency is bounded by the configured semaphore size.

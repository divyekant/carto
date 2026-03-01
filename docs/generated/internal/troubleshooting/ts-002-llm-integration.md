---
id: ts-002
type: troubleshooting
audience: internal
topic: LLM Connection & Authentication Issues
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Troubleshooting: LLM Connection & Authentication Issues

## Quick Check

1. **API key is set:** `echo $ANTHROPIC_API_KEY` or `echo $LLM_API_KEY`. At least one must be non-empty.
2. **Provider is reachable:** `curl -s -o /dev/null -w "%{http_code}" https://api.anthropic.com/v1/messages` (expect 401 without a body, which confirms connectivity).
3. **Model name is valid:** `echo $CARTO_FAST_MODEL` and `echo $CARTO_DEEP_MODEL`. If unset, defaults are used.

---

## Symptom: 401 Unauthorized Errors

All or most LLM calls return HTTP 401.

### Diagnostic Steps

1. Verify the API key is set:
   ```bash
   echo "LLM_API_KEY: ${LLM_API_KEY:-(not set)}"
   echo "ANTHROPIC_API_KEY: ${ANTHROPIC_API_KEY:-(not set)}"
   ```

2. Test the key directly:
   ```bash
   curl -s https://api.anthropic.com/v1/messages \
     -H "x-api-key: $ANTHROPIC_API_KEY" \
     -H "anthropic-version: 2023-06-01" \
     -H "content-type: application/json" \
     -d '{"model":"claude-haiku-4-5-20251001","max_tokens":10,"messages":[{"role":"user","content":"test"}]}'
   ```

3. If using OAuth (`sk-ant-oat01-` prefix), check if the token has expired:
   - OAuth tokens have a limited lifetime.
   - The client attempts automatic refresh, but the refresh token itself may have expired.

4. If using a non-Anthropic provider, verify the key format matches the provider's expectations.

### Root Causes & Resolutions

| Root Cause | Resolution |
|---|---|
| API key is not set | Export `ANTHROPIC_API_KEY` or `LLM_API_KEY` in your environment or `.env` file. |
| API key is expired or revoked | Generate a new key from the provider's dashboard. |
| API key is for the wrong provider | Ensure `LLM_PROVIDER` matches the key's provider. An Anthropic key will not work with OpenAI. |
| OAuth token expired and refresh failed | Re-authenticate to obtain a new OAuth token. |
| `LLM_API_KEY` overrides `ANTHROPIC_API_KEY` with a wrong value | Check both variables. `LLM_API_KEY` takes precedence. Unset it if `ANTHROPIC_API_KEY` is the correct one. |
| Key has trailing whitespace or newline | Trim the key: `export ANTHROPIC_API_KEY=$(echo "$ANTHROPIC_API_KEY" | tr -d '[:space:]')` |

---

## Symptom: Timeout Errors

LLM calls hang and eventually time out.

### Diagnostic Steps

1. Test basic connectivity to the provider:
   ```bash
   curl -s -o /dev/null -w "HTTP %{http_code}, time %{time_total}s\n" https://api.anthropic.com/v1/messages
   ```

2. Check if the issue is specific to the deep-tier model (larger prompts, longer processing):
   - If only Phase 4 times out but Phase 2 works, the deep model may be slow or the prompts too large.

3. Check local network conditions (corporate proxy, VPN, firewall rules).

4. If using Ollama, check local resource usage (`top`, `nvidia-smi` for GPU).

### Root Causes & Resolutions

| Root Cause | Resolution |
|---|---|
| Network connectivity issues | Check firewall rules, proxy settings, and VPN configuration. Ensure HTTPS to the provider is allowed. |
| Corporate proxy blocks or slows requests | Configure `HTTPS_PROXY` if needed. Check with IT about API access policies. |
| Cloudflare WARP or similar always-on VPN | Usually works fine, but can add latency. Test with WARP disabled if possible. |
| Provider is experiencing high latency | Check the provider's status page. Retry later. |
| Deep-tier prompt is too large | Large modules produce massive prompts. Use `--module` to index smaller units. |
| Ollama running out of memory | Reduce `CARTO_MAX_CONCURRENT` to 1-2 for local models. Use smaller models. |

---

## Symptom: JSON Parse Failures

`CompleteJSON()` returns errors about invalid JSON or no JSON found in the response.

### Diagnostic Steps

1. Check if the LLM is returning non-JSON content:
   - The model may be returning explanatory text instead of structured JSON.
   - The prompt may not be clear enough about the expected output format.

2. Check if the response is being truncated:
   - If the model hits its `max_tokens` limit, the JSON output may be cut off mid-structure.
   - Look for error messages mentioning "stop reason: max_tokens".

3. Check if the model is wrapping JSON in unexpected formatting:
   - `CompleteJSON()` handles ` ```json ``` ` fences, but other wrappers (e.g., XML tags, custom delimiters) are not stripped.

### Root Causes & Resolutions

| Root Cause | Resolution |
|---|---|
| Model returned prose instead of JSON | This is usually a model behavior issue. Try a different model or check if the prompt template changed. |
| Response truncated at `max_tokens` | The token limit may be too low for the expected output. Check the configuration for max token settings. |
| Model returned JSON with non-standard wrapping | The brace-matching extraction should handle most cases. If a specific wrapper is consistently used, the prompt may need adjustment. |
| Malformed JSON (missing quotes, trailing commas) | Some models (especially smaller Ollama models) produce invalid JSON. Use a more capable model. |
| Response contains multiple JSON objects | The extraction takes the first valid JSON object. If the model produces multiple, only the first is used. |

---

## Symptom: 429 Rate Limit Errors

The LLM provider returns HTTP 429 Too Many Requests.

### Diagnostic Steps

1. Check the current concurrency setting: `echo $CARTO_MAX_CONCURRENT` (default: 10).
2. Check the provider's rate limit headers in the error response (if available in logs).
3. Determine if the rate limit is per-minute, per-hour, or daily.
4. Check if other applications or users are sharing the same API key.

### Root Causes & Resolutions

| Root Cause | Resolution |
|---|---|
| `CARTO_MAX_CONCURRENT` too high | Reduce to 3-5 for rate-limited accounts. |
| Account-level rate limit exceeded | Check your plan's rate limits on the provider dashboard. Upgrade the plan if needed. |
| Multiple Carto instances sharing the same key | Stagger indexing runs or use separate API keys. |
| Provider is throttling during peak hours | Retry during off-peak hours. |

---

## Symptom: Ollama-Specific Failures

### "Connection refused" to Ollama

```
dial tcp 127.0.0.1:11434: connect: connection refused
```

**Resolution:** Start the Ollama server with `ollama serve`. Verify the port matches `LLM_BASE_URL`.

### "Model not found" from Ollama

**Resolution:** Pull the model first: `ollama pull <model-name>`. List available models with `ollama list`.

### Very slow responses from Ollama

**Resolution:** Reduce `CARTO_MAX_CONCURRENT` to 1-2. Use smaller models. Ensure adequate RAM/GPU memory for the chosen model.

---

## Escalation

Escalate when:

- **Provider returns 500 errors consistently:** Check the provider's status page (status.anthropic.com, status.openai.com). If the outage is confirmed, wait for resolution.
- **OAuth refresh loop:** The client refreshes the token but immediately gets a 401 again. This may indicate a backend issue with the OAuth provider. Re-authenticate from scratch.
- **JSON extraction succeeds but produces wrong structure:** The model may have changed its output format. Check if the Carto version matches the expected model behavior. File a bug if the prompt templates need updating.
- **Persistent rate limits despite low concurrency:** The account may have been flagged or restricted. Contact the provider's support.

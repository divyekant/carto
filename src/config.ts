import type { CodexConfig } from './types.js';

export function loadConfig(): CodexConfig {
  return {
    faissUrl: process.env.FAISS_URL || 'http://localhost:8900',
    faissApiKey: process.env.FAISS_API_KEY || 'god-is-an-astronaut',
    anthropicApiKey: process.env.ANTHROPIC_API_KEY || '',
    haikuModel: process.env.CODEX_HAIKU_MODEL || 'claude-haiku-4-5-20251001',
    opusModel: process.env.CODEX_OPUS_MODEL || 'claude-opus-4-6',
    maxConcurrentLlmCalls: parseInt(process.env.CODEX_MAX_CONCURRENT || '5', 10),
  };
}

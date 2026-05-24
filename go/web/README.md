# Carto Web UI

React, TypeScript, Vite, Tailwind, and shadcn/ui power the Carto dashboard that is embedded into the Go server binary.

## Development

```bash
npm install
npm run dev
npm run lint
npm run build
```

The production SPA is embedded by the Go service. For product validation, prefer running the Go server so the UI exercises the same API, Memories, and LLM configuration as the shipped binary:

```bash
cd ..
LLM_PROVIDER=codex \
MEMORIES_URL=http://localhost:8901 \
MEMORIES_API_KEY=god-is-an-astronaut \
go run ./cmd/carto serve --projects-dir /path/to/projects --port 8950
```

The default model provider is `codex`, which uses the local Codex ChatGPT session at `~/.codex/auth.json` and does not require provider API keys. The Settings page still supports Anthropic, OpenAI-compatible providers, and Ollama for explicit alternate deployments.

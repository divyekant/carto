# Docker Deployment

Carto ships with a multi-stage Dockerfile and a Docker Compose configuration that runs both Carto and its Memories dependency together. This is the fastest way to get a fully working setup with zero local build requirements.

## Quick Start

```bash
docker compose up
```

That is it. Carto is now available at **http://localhost:8950** and Memories is running at **http://localhost:8900**.

## What Gets Started

| Service | Port | Description |
|---------|------|-------------|
| **Carto** | `8950` | Web UI, REST API, and CLI server |
| **Memories** | `8900` | Vector storage backend for the semantic index |

## Configuration

All configuration is done through environment variables. Create a `.env` file in the same directory as `docker-compose.yml`:

```env
# Required — your LLM API key
LLM_API_KEY=your-api-key-here

# Optional — LLM provider (default: anthropic)
LLM_PROVIDER=anthropic

# Optional — directory containing your codebases (mounted read-only)
PROJECTS_DIR=/path/to/your/repos
```

The `PROJECTS_DIR` variable tells Carto where to find your codebases. This directory is mounted read-only into the container so Carto can scan but never modify your code.

### Port Mapping

To change the default ports, update your `docker-compose.yml` or pass them as environment variables:

```yaml
services:
  carto:
    ports:
      - "9000:8950"   # Carto on port 9000
  memories:
    ports:
      - "9100:8900"   # Memories on port 9100
```

## Docker Compose

The provided `docker-compose.yml` is the recommended way to run Carto:

```bash
# Start both services in the foreground
docker compose up

# Start in the background
docker compose up -d

# Stop
docker compose down

# Rebuild after updating
docker compose up --build
```

### Mounting Your Projects

By default, `PROJECTS_DIR` is mapped to `/projects` inside the container as a read-only volume. You can point it at any directory on your host:

```bash
PROJECTS_DIR=/home/dev/repos docker compose up
```

Or set it in your `.env` file as shown above.

## Standalone Docker Run

If you prefer to run Carto without Compose (for example, if you already have a Memories instance running), you can use `docker run` directly:

```bash
docker run -d \
  --name carto \
  -p 8950:8950 \
  -e LLM_API_KEY=your-api-key \
  -e MEMORIES_URL=http://your-memories-host:8900 \
  -v /path/to/repos:/projects:ro \
  carto
```

Key flags:

| Flag | Purpose |
|------|---------|
| `-p 8950:8950` | Expose the Carto web UI and API |
| `-e LLM_API_KEY=...` | Pass your LLM API key |
| `-e MEMORIES_URL=...` | Point at your Memories server |
| `-v ...:/projects:ro` | Mount codebases read-only |

## Building the Image

The Dockerfile uses a multi-stage build:

1. **Build stage** — Alpine with Go, gcc, and musl-dev. Compiles Carto with CGO enabled for tree-sitter support.
2. **Runtime stage** — Minimal Alpine image with just the compiled binary.

To build manually:

```bash
docker build -t carto .
```

## Custom Projects Directory

You can mount multiple directories or a parent directory containing all your repos:

```bash
# Single project
docker run -v /repos/my-app:/projects/my-app:ro ...

# All repos under one parent
docker run -v /home/dev/repos:/projects:ro ...
```

Inside Carto, projects will appear with paths relative to `/projects`.

## Examples

### Development Setup

Run Carto alongside your local development workflow:

```bash
# .env
LLM_API_KEY=sk-ant-...
PROJECTS_DIR=/Users/you/Projects

# Start
docker compose up -d

# Index a project via the API
curl -X POST http://localhost:8950/api/projects \
  -H "Content-Type: application/json" \
  -d '{"name": "my-app", "path": "/projects/my-app"}'

curl -N -X POST http://localhost:8950/api/projects/my-app/index
```

### Team Server

Set up a shared Carto instance for your team:

```bash
# docker-compose.override.yml
services:
  carto:
    restart: always
    ports:
      - "0.0.0.0:8950:8950"
    volumes:
      - /srv/repos:/projects:ro
```

Put this behind a reverse proxy with authentication for secure access (Carto does not include built-in auth).

## Limitations

- **CGO build** — The Docker build stage requires Alpine with gcc and musl-dev to compile the tree-sitter C bindings. This makes the build image larger, but the final runtime image stays small.
- **Volume permissions** — The container runs as a non-root user. Make sure the mounted project directories are readable by the container user.
- **Single instance** — Carto is designed as a single-server tool. The Docker setup does not include clustering or horizontal scaling.
- **LLM connectivity** — The container needs outbound HTTPS access to reach your LLM provider's API (unless you are using a local Ollama instance).

## Related

- [CLI Reference](feat-006-cli.md) — use Carto from the command line inside the container
- [REST API](feat-007-rest-api.md) — the API available at port 8950
- [Web Dashboard](feat-008-web-ui.md) — open the browser to port 8950 after starting

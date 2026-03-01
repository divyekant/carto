# Use Case: Docker Deployment

**ID:** uc-010
**Topic:** Docker Deployment
**Trigger:** A user runs `docker compose up` to deploy Carto and Memories as containerized services.

---

## Primary Flow

### 1. Configure Environment

Create a `.env` file in the Carto project root:

```env
LLM_API_KEY=sk-ant-...
```

Docker Compose automatically loads this file. The `MEMORIES_URL` is pre-configured in `docker-compose.yml` to point to the Memories service.

### 2. Start Services

```bash
docker compose up -d
```

This builds the Carto image (if not already built) and starts both services:
- **Memories** starts first (Carto `depends_on` Memories).
- **Carto** starts after Memories is running, serving the Web UI and API on port `8950`.

### 3. Verify Services

```bash
# Check service status
docker compose ps

# Verify Carto is responding
curl http://localhost:8950/api/projects

# Verify Memories is responding
curl http://localhost:8900/health
```

### 4. Access the Web UI

Open `http://localhost:8950` in a browser. The Dashboard loads, showing any previously registered projects.

### 5. Register and Index a Project

Projects reference paths inside the container. The `docker-compose.yml` mounts the host's project directory at `/projects`:

```bash
# Register a project (path must be inside the container mount)
curl -X POST http://localhost:8950/api/projects \
  -H "Content-Type: application/json" \
  -d '{"name": "myapp", "path": "/projects/myapp"}'
```

Trigger indexing via the Web UI or API:

```bash
curl -N -X POST http://localhost:8950/api/projects/myapp/index
```

### 6. Query

```bash
curl "http://localhost:8950/api/query?q=how+does+auth+work&project=myapp&tier=standard"
```

---

## Variation: Custom `.env` Configuration

Override LLM provider settings in the `.env` file:

```env
LLM_API_KEY=sk-ant-...
LLM_PROVIDER=anthropic
LLM_MODEL=claude-sonnet-4-20250514
LLM_FAST_MODEL=claude-haiku-4-20250414
LLM_DEEP_MODEL=claude-sonnet-4-20250514
```

Restart services to pick up changes:

```bash
docker compose down && docker compose up -d
```

---

## Variation: Additional Volume Mounts

To index codebases from multiple directories on the host, add volume mounts:

```yaml
# docker-compose.yml
carto:
  volumes:
    - /home/user/work:/projects/work:ro
    - /home/user/oss:/projects/oss:ro
```

Inside the container, projects are at `/projects/work/myapp`, `/projects/oss/library`, etc.

---

## Variation: Standalone Docker Run (No Compose)

Run Carto without Docker Compose. This requires a separately running Memories instance:

```bash
# Start Memories separately
docker run -d -p 8900:8900 -v memories-data:/data memories:latest

# Build and run Carto
docker build -t carto .
docker run -d \
  -p 8950:8950 \
  -v /path/to/projects:/projects:ro \
  -e LLM_API_KEY=sk-ant-... \
  -e MEMORIES_URL=http://host.docker.internal:8900 \
  carto
```

Note: Use `host.docker.internal` (Docker Desktop) or the host's IP address to reach Memories from the Carto container when not using compose networking.

---

## Variation: Rebuilding After Code Changes

After modifying Carto source code:

```bash
# Rebuild only the Carto image
docker compose build carto

# Restart with the new image
docker compose up -d carto
```

The Memories data volume is preserved. Previously indexed data remains available.

---

## Postconditions

- Both Carto and Memories are running as containers.
- The Web UI is accessible at `http://localhost:8950`.
- The API is available for project management, indexing, and querying.
- Memories data is persisted in a named Docker volume.
- Host codebases are accessible inside the container via volume mounts.

# Feature Handoff: Docker & Deployment

**ID:** fh-010
**Feature:** Docker & Deployment
**Components:** `Dockerfile`, `docker-compose.yml`

---

## Overview

Carto provides a Docker-based deployment path using a multi-stage Dockerfile and a `docker-compose.yml` that orchestrates the Carto service alongside its Memories dependency. The Docker setup handles the CGO requirement for tree-sitter by building on Alpine with `gcc` and `musl-dev`. The compose file maps volumes for codebase access and passes environment variables for LLM and Memories configuration.

---

## Multi-Stage Dockerfile

The Dockerfile uses two stages:

### Stage 1: Builder

- **Base image:** `golang:1.22-alpine` (or equivalent).
- **System packages:** `gcc`, `musl-dev` (required for CGO / tree-sitter).
- **Build step:** `CGO_ENABLED=1 go build -o carto ./cmd/carto`.
- **Web UI:** The React SPA is built and embedded during this stage (or pre-built and copied in).

### Stage 2: Runtime

- **Base image:** `alpine:3.19` (or equivalent minimal image).
- **Copies:** The compiled `carto` binary from the builder stage.
- **Entrypoint:** `carto serve --port 8950`.
- **Exposes:** Port `8950`.

The resulting image is small (typically under 50MB) since it only contains the static binary and Alpine base.

---

## docker-compose.yml

The compose file defines two services:

### `carto` Service

```yaml
carto:
  build: .
  ports:
    - "8950:8950"
  volumes:
    - /path/to/projects:/projects:ro
  environment:
    - LLM_API_KEY=${LLM_API_KEY}
    - MEMORIES_URL=http://memories:8900
  depends_on:
    - memories
```

- **Port mapping:** Host `8950` to container `8950`.
- **Volume:** The host's project directory is mounted read-only at `/projects` inside the container. This gives Carto access to codebases without copying them into the image.
- **Environment:** `LLM_API_KEY` is passed through from the host. `MEMORIES_URL` points to the Memories service by its compose service name.

### `memories` Service

```yaml
memories:
  image: memories:latest
  ports:
    - "8900:8900"
  volumes:
    - memories-data:/data
```

- **Port mapping:** Host `8900` to container `8900`.
- **Volume:** Persistent storage for the Memories data directory.

---

## Volume Mounts

| Mount | Container Path | Mode | Purpose |
|-------|---------------|------|---------|
| Host project directory | `/projects` | Read-only (`ro`) | Codebases to be indexed. |
| `memories-data` volume | `/data` | Read-write | Persistent Memories storage. |

To index a project inside the container, reference its path relative to the container mount:
```bash
carto index --project myapp
# Project path should be configured as /projects/myapp
```

---

## Port Mapping

| Service | Container Port | Default Host Port |
|---------|---------------|-------------------|
| Carto | 8950 | 8950 |
| Memories | 8900 | 8900 |

---

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `LLM_API_KEY` | Yes | LLM provider API key. Passed through from the host environment or a `.env` file. |
| `ANTHROPIC_API_KEY` | Alternative | Alternative to `LLM_API_KEY` for Anthropic provider. |
| `MEMORIES_URL` | Set in compose | Points to `http://memories:8900` (the compose service). |
| `MEMORIES_API_KEY` | No | Memories authentication, if enabled. |
| `LLM_PROVIDER` | No | `anthropic`, `openai`, or `ollama`. |
| `LLM_MODEL` | No | Model name override. |

Create a `.env` file in the project root for convenience:
```env
LLM_API_KEY=sk-ant-...
```

Docker Compose automatically loads `.env` from the working directory.

---

## Edge Cases

- **CGO build failures:** The most common Docker build issue. If the builder stage fails with linker errors, verify that `gcc` and `musl-dev` are installed in the builder image and that `CGO_ENABLED=1` is set. Tree-sitter's C dependencies require a working C compiler.
- **Volume permissions:** If the container cannot read files from the `/projects` mount, check the host directory permissions. The container runs as a non-root user (if configured), which may not have read access to the mounted directory. On Linux, ensure the UID inside the container can read the files.
- **Memories service not starting:** If the Memories container fails to start, check its logs (`docker compose logs memories`). Common causes: port conflict on the host, corrupted data volume, missing image.
- **Network resolution:** The `carto` service references Memories as `http://memories:8900` using Docker's internal DNS. If the Memories service is renamed in the compose file, update `MEMORIES_URL` accordingly.
- **Large images:** If the Docker image is unexpectedly large, ensure the multi-stage build is working correctly. The runtime stage should not include Go toolchain, source code, or build artifacts.

---

## Common Questions

**Q1: How do I change the Carto port?**
Modify the port mapping in `docker-compose.yml`:
```yaml
ports:
  - "9000:8950"
```
Access the UI at `http://localhost:9000`. The container still listens on `8950` internally.

**Q2: How do I add projects to the Docker setup?**
Mount additional directories as volumes, or mount a parent directory:
```yaml
volumes:
  - /home/user/code:/projects:ro
```
All subdirectories of `/projects` are then accessible. Register projects with paths like `/projects/backend-api`.

**Q3: Can I build and run Carto without Docker?**
Yes. Build natively with `go build -o carto ./cmd/carto` (requires CGO and a C compiler). Run `carto serve` directly. The Docker setup is optional and primarily useful for consistent, reproducible deployments.

**Q4: How do I persist Memories data across container restarts?**
The `docker-compose.yml` defines a named volume (`memories-data`) for Memories storage. This volume persists across `docker compose down` and `docker compose up` cycles. To fully remove it, run `docker compose down -v`.

**Q5: How do I update Carto in Docker?**
Rebuild the image:
```bash
docker compose build carto
docker compose up -d
```
The Memories data volume is preserved. No re-indexing is needed unless the index format changed between versions.

---

## Troubleshooting

| Symptom | Likely Cause | Resolution |
|---------|-------------|------------|
| CGO build error (`gcc not found`, linker errors) | Missing `gcc` or `musl-dev` in the builder stage. | Add `RUN apk add --no-cache gcc musl-dev` to the builder stage of the Dockerfile. |
| `connection refused` to Memories from Carto | Memories service is not running or `MEMORIES_URL` is wrong. | Check `docker compose logs memories`. Verify `MEMORIES_URL=http://memories:8900` in the Carto service config. |
| `permission denied` on `/projects` | Container user lacks read access to the mounted volume. | Check host directory permissions. Try `chmod -R a+r /path/to/projects` on the host, or adjust the container user. |
| `port already in use` on startup | Host ports 8950 or 8900 are occupied. | Change the host-side port in `docker-compose.yml` (e.g., `"9000:8950"`). Stop the conflicting process. |
| Image is very large (>500MB) | Multi-stage build is not working; runtime stage includes build tools. | Verify the Dockerfile uses `COPY --from=builder` to copy only the binary. Check that the runtime stage uses a minimal base image. |
| Container exits immediately | Entrypoint command failed. | Check logs: `docker compose logs carto`. Common causes: missing environment variables, binary crash on startup. |

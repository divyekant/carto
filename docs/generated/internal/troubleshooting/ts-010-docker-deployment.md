# Troubleshooting: Docker Issues

**ID:** ts-010
**Topic:** Docker Issues
**Components:** `Dockerfile`, `docker-compose.yml`

---

## Symptom: CGO Build Error During `docker build`

**Cause:** The builder stage is missing the C compiler (`gcc`) or C library headers (`musl-dev`) required by tree-sitter's CGO dependencies.

**Resolution:**

1. Verify the Dockerfile installs build tools in the builder stage:
   ```dockerfile
   RUN apk add --no-cache gcc musl-dev
   ```

2. Ensure `CGO_ENABLED=1` is set during the Go build:
   ```dockerfile
   ENV CGO_ENABLED=1
   RUN go build -o carto ./cmd/carto
   ```

3. If the error references specific tree-sitter grammars, ensure their C source files are included in the build context. Check `.dockerignore` to confirm these files are not excluded.

4. For cross-compilation issues (e.g., building on ARM for AMD64), ensure the builder image matches the target architecture, or use Docker's `--platform` flag:
   ```bash
   docker build --platform linux/amd64 -t carto .
   ```

---

## Symptom: `connection refused` to Memories from Carto Container

**Cause:** The Memories service is not running, has not finished starting, or the `MEMORIES_URL` is misconfigured.

**Resolution:**

1. Check Memories service status:
   ```bash
   docker compose ps
   docker compose logs memories
   ```

2. Verify `MEMORIES_URL` in the Carto service configuration points to the correct compose service name:
   ```yaml
   environment:
     - MEMORIES_URL=http://memories:8900
   ```
   The hostname `memories` resolves to the Memories container via Docker's internal DNS, but only when both services are in the same compose network.

3. If Memories is slow to start, Carto may attempt to connect before Memories is ready. Add a health check to the Memories service:
   ```yaml
   memories:
     healthcheck:
       test: ["CMD", "curl", "-f", "http://localhost:8900/health"]
       interval: 5s
       timeout: 3s
       retries: 5
   ```
   And update Carto's dependency:
   ```yaml
   carto:
     depends_on:
       memories:
         condition: service_healthy
   ```

4. If running Carto standalone (without compose), use the host's IP or `host.docker.internal` instead of the service name:
   ```bash
   -e MEMORIES_URL=http://host.docker.internal:8900
   ```

---

## Symptom: `permission denied` on `/projects` Volume

**Cause:** The container process does not have read permission on the mounted host directory. This is common on Linux where the container's UID does not match the host file owner.

**Resolution:**

1. Check the host directory permissions:
   ```bash
   ls -la /path/to/projects/
   ```

2. Ensure the directory is world-readable, or adjust permissions:
   ```bash
   chmod -R a+rX /path/to/projects/
   ```

3. If the Dockerfile specifies a non-root user, verify that user's UID can access the mounted files. On Linux, you can run the container with the host user's UID:
   ```bash
   docker run --user $(id -u):$(id -g) ...
   ```

4. On macOS with Docker Desktop, file sharing permissions are managed by Docker Desktop settings. Verify that the host directory is in the list of shared paths.

5. The volume should be mounted read-only (`:ro`). Write access to the project directory is not needed for indexing.

---

## Symptom: `port already in use` on `docker compose up`

**Cause:** The host ports 8950 (Carto) or 8900 (Memories) are already bound by another process.

**Resolution:**

1. Identify what is using the port:
   ```bash
   lsof -i :8950
   lsof -i :8900
   ```

2. Stop the conflicting process, or change the host port in `docker-compose.yml`:
   ```yaml
   carto:
     ports:
       - "9000:8950"  # Host port 9000 maps to container port 8950
   ```

3. Access the UI on the new port (`http://localhost:9000`).

---

## Symptom: Container Exits Immediately After Starting

**Cause:** The entrypoint command failed. Common causes include missing environment variables, a binary crash, or a configuration error.

**Resolution:**

1. Check the container logs:
   ```bash
   docker compose logs carto
   ```

2. Look for error messages. Typical causes:
   - Missing `LLM_API_KEY`: The serve command itself does not require it, but if the startup validation checks for it, set it in `.env`.
   - Binary incompatibility: If the image was built for a different architecture, the binary fails to execute. Rebuild with the correct platform.

3. Try running the container interactively to debug:
   ```bash
   docker compose run --rm carto sh
   # Inside the container:
   ./carto serve --port 8950
   ```

---

## Symptom: Docker Image Is Unexpectedly Large

**Cause:** The multi-stage build is not correctly isolating the builder stage from the runtime stage, or large files are included in the build context.

**Resolution:**

1. Verify the Dockerfile uses a multi-stage build with a minimal runtime base:
   ```dockerfile
   FROM alpine:3.19
   COPY --from=builder /app/carto /usr/local/bin/carto
   ```

2. Check `.dockerignore` to ensure build artifacts, `.git`, `node_modules`, and other large directories are excluded:
   ```
   .git
   node_modules
   web/node_modules
   *.test
   ```

3. Inspect image layers:
   ```bash
   docker history carto
   ```
   Look for unexpectedly large layers in the runtime stage.

4. The final image should be under 50-100MB. If it exceeds this, the runtime stage is likely pulling in unnecessary files.

---

## Symptom: Memories Data Lost After `docker compose down`

**Cause:** Running `docker compose down -v` removes named volumes, including the Memories data volume.

**Resolution:**

1. Use `docker compose down` (without `-v`) to stop services while preserving volumes.

2. To verify the volume exists:
   ```bash
   docker volume ls | grep memories
   ```

3. Back up the volume data before destructive operations:
   ```bash
   docker run --rm -v memories-data:/data -v $(pwd):/backup alpine tar czf /backup/memories-backup.tar.gz -C /data .
   ```

---

## Quick Reference

| Symptom | First Check |
|---------|-------------|
| CGO build error | `apk add gcc musl-dev` in builder stage |
| Connection refused to Memories | `docker compose logs memories` |
| Permission denied on /projects | Host directory permissions (`ls -la`) |
| Port already in use | `lsof -i :8950` |
| Container exits immediately | `docker compose logs carto` |
| Image too large | Check `.dockerignore` and multi-stage build |
| Data lost after restart | Was `-v` used with `docker compose down`? |

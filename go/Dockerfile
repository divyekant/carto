# ============================================================
# Stage 1: Build frontend
# ============================================================
FROM node:22-alpine AS frontend

WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --ignore-scripts
COPY web/ ./
RUN npm run build

# ============================================================
# Stage 2: Build Go binary
# ============================================================
FROM golang:1.25-alpine AS builder

# tree-sitter requires CGO
RUN apk add --no-cache gcc musl-dev git

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY cmd/ cmd/
COPY internal/ internal/
COPY web/embed.go web/embed.go

# Copy built frontend into the embed directory
COPY --from=frontend /app/web/dist web/dist/

# Build static binary
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /carto ./cmd/carto

# ============================================================
# Stage 3: Runtime
# ============================================================
FROM alpine:3.21

RUN apk add --no-cache ca-certificates git

COPY --from=builder /carto /usr/local/bin/carto

# Default projects directory (mount your projects here)
RUN mkdir -p /projects

# Run as non-root user for defence-in-depth security.
# carto only needs access to /projects (volume mount) and network egress.
RUN addgroup -S carto && adduser -S carto -G carto && chown carto:carto /projects
USER carto

EXPOSE 8950

# Container health check for Docker Compose and Kubernetes readiness probes.
# The /api/health endpoint bypasses authentication so orchestrators can probe
# liveness without a token, even when CARTO_SERVER_TOKEN is configured.
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD wget -qO- http://localhost:8950/api/health || exit 1

# Configure via docker-compose.yml, .env, or docker run -e flags.
# See .env.example for all available variables.

ENTRYPOINT ["carto"]
CMD ["serve", "--port", "8950", "--projects-dir", "/projects"]

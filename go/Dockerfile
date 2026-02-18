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

EXPOSE 8950

ENV MEMORIES_URL=http://host.docker.internal:8900
ENV MEMORIES_API_KEY=god-is-an-astronaut

ENTRYPOINT ["carto"]
CMD ["serve", "--port", "8950", "--projects-dir", "/projects"]

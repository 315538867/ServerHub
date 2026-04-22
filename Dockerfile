# syntax=docker/dockerfile:1.7
#
# Multi-stage build for ServerHub.
#   1. frontend  — bun build Vue assets (Vite outputs to ../backend/web/dist)
#   2. backend   — go build with CGO=1 (mattn/go-sqlite3 needs libc)
#   3. runtime   — distroless/base-debian12:nonroot (~25MB, no shell)
#
# Vite's config writes the bundle into the sibling backend/web/dist folder,
# so both `frontend/` and `backend/` must be laid out at the same level
# inside the builder for `bun run build` to succeed.

# ── stage 1: frontend ─────────────────────────────────────────
FROM node:20-bookworm-slim AS frontend
WORKDIR /src
RUN npm install -g bun@1.2.18
# Mirror the repo layout so Vite's ../backend/web/dist outDir resolves.
COPY frontend/package.json frontend/bun.lock frontend/
RUN cd frontend && bun install --frozen-lockfile
COPY frontend/ frontend/
RUN cd frontend && bun run build
# Result: /src/backend/web/dist/<assets>

# ── stage 2: backend ──────────────────────────────────────────
FROM golang:1.25-bookworm AS backend
ARG TARGETARCH
ARG VERSION=dev
WORKDIR /src
RUN apt-get update && \
    apt-get install -y --no-install-recommends gcc-aarch64-linux-gnu && \
    rm -rf /var/lib/apt/lists/*
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# Pull the built frontend into the expected embed path.
COPY --from=frontend /src/backend/web/dist ./web/dist
RUN set -eux; \
    case "${TARGETARCH}" in \
      amd64) CC=gcc GOARCH=amd64 ;; \
      arm64) CC=aarch64-linux-gnu-gcc GOARCH=arm64 ;; \
      *) echo "unsupported arch: ${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    export CC GOARCH; \
    CGO_ENABLED=1 GOOS=linux \
    go build -trimpath \
      -ldflags="-s -w -X main.Version=${VERSION}" \
      -o /out/serverhub .

# ── stage 3: runtime ──────────────────────────────────────────
# BASE_RUNTIME lets users swap the runtime base (e.g. through a mirror) when
# gcr.io is unreachable:
#   docker buildx build --build-arg BASE_RUNTIME=<mirror>/distroless/base-debian12:nonroot ...
ARG BASE_RUNTIME=gcr.io/distroless/base-debian12:nonroot
FROM ${BASE_RUNTIME}
COPY --from=backend /out/serverhub /serverhub
COPY backend/config.example.yaml /etc/serverhub/config.example.yaml
ENV SERVERHUB_DATA_DIR=/data \
    SERVERHUB_CONFIG=/data/config.yaml \
    SERVERHUB_PORT=9999
VOLUME ["/data"]
EXPOSE 9999
USER nonroot:nonroot
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD ["/serverhub", "--healthcheck"]
ENTRYPOINT ["/serverhub"]

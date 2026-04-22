# syntax=docker/dockerfile:1.7
#
# Multi-stage build for ServerHub.
#   1. frontend  — bun build Vue assets (Vite outputs to ../backend/web/dist)
#   2. backend   — go build with CGO=1 (mattn/go-sqlite3 needs libc)
#   3. runtime   — debian:bookworm-slim + bash + docker-cli (~180MB)
#
# 历史版本曾用 distroless/nonroot；但本机模式的 runner 通过 `bash -lc` 调用
# `docker ps`/`systemctl ...`，distroless 既无 shell 也无 docker CLI，导致
# "本机服务器" 类型的 Docker / systemd 面板全部 5xx。改用 debian-slim
# 并预装 bash + docker.io CLI（仅 client，daemon 走宿主 socket）。
#
# Vite's config writes the bundle into the sibling backend/web/dist folder,
# so both `frontend/` and `backend/` must be laid out at the same level
# inside the builder for `bun run build` to succeed.

# ── stage 1: frontend ─────────────────────────────────────────
# Pin to BUILDPLATFORM: bun/node under QEMU emulation is slow and occasionally
# crashes; the produced web/dist is arch-independent so native build is fine.
FROM --platform=$BUILDPLATFORM node:20-bookworm-slim AS frontend
WORKDIR /src
RUN npm install -g bun@1.3.10
# Mirror the repo layout so Vite's ../backend/web/dist outDir resolves.
COPY frontend/package.json frontend/bun.lock frontend/
RUN cd frontend && bun install --frozen-lockfile
COPY frontend/ frontend/
RUN cd frontend && bun run build
# Result: /src/backend/web/dist/<assets>

# ── stage 2: backend ──────────────────────────────────────────
# Run natively on BUILDPLATFORM and cross-compile to TARGETARCH via Go's
# toolchain + a matching CC. This mirrors the tar.gz release path exactly
# and avoids QEMU emulation bugs that previously produced broken multi-arch
# Docker binaries while the tar.gz build worked.
FROM --platform=$BUILDPLATFORM golang:1.25-bookworm AS backend
ARG TARGETARCH
ARG VERSION=dev
WORKDIR /src
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      gcc gcc-aarch64-linux-gnu libc6-dev-arm64-cross && \
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
# BASE_RUNTIME 默认 debian:bookworm-slim；可通过 mirror 镜像替换：
#   docker buildx build --build-arg BASE_RUNTIME=<mirror>/debian:bookworm-slim ...
# 已包含：bash, ca-certificates, tini, docker-cli (docker.io), curl
# 用户：serverhub (uid 65532)，与旧 distroless nonroot 兼容
ARG BASE_RUNTIME=debian:bookworm-slim
FROM ${BASE_RUNTIME}
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      bash ca-certificates tini curl docker.io && \
    rm -rf /var/lib/apt/lists/* && \
    groupadd -g 65532 serverhub && \
    useradd -u 65532 -g 65532 -m -s /bin/bash serverhub
COPY --from=backend /out/serverhub /serverhub
COPY backend/config.example.yaml /etc/serverhub/config.example.yaml
ENV SERVERHUB_DATA_DIR=/data \
    SERVERHUB_CONFIG=/data/config.yaml \
    SERVERHUB_PORT=9999
VOLUME ["/data"]
EXPOSE 9999
USER serverhub:serverhub
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD ["/serverhub", "--healthcheck"]
ENTRYPOINT ["/usr/bin/tini", "--", "/serverhub"]

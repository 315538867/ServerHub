#!/usr/bin/env bash
# ServerHub — local one-shot builder.
#
# Usage:
#   bash scripts/build.sh                # default: binary
#   bash scripts/build.sh binary         # make build (host arch, embeds frontend)
#   bash scripts/build.sh docker         # docker image (buildx if available)
#   bash scripts/build.sh all            # binary + docker
#
# Image tags applied: serverhub:local + serverhub:<git-describe>

set -euo pipefail

cd "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

TARGET="${1:-binary}"
VERSION="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"

log() { printf '\033[0;36m[build]\033[0m %s\n' "$*"; }
die() { printf '\033[0;31m[build]\033[0m %s\n' "$*" >&2; exit 1; }

build_binary() {
  command -v go >/dev/null   || die "go not installed"
  command -v node >/dev/null || command -v bun >/dev/null || die "node or bun required for frontend build"
  log "make build (version=$VERSION)"
  make build
}

build_docker() {
  command -v docker >/dev/null || die "docker not installed"
  local tag_local="serverhub:local"
  local tag_ver="serverhub:${VERSION}"
  if docker buildx version >/dev/null 2>&1; then
    log "docker buildx → $tag_local + $tag_ver"
    docker buildx build \
      --build-arg "VERSION=$VERSION" \
      -t "$tag_local" -t "$tag_ver" \
      --load .
  else
    log "buildx not found, using legacy builder"
    DOCKER_BUILDKIT=1 docker build \
      --build-arg "VERSION=$VERSION" \
      -t "$tag_local" -t "$tag_ver" .
  fi
  log "✓ images: $tag_local, $tag_ver"
}

case "$TARGET" in
  binary) build_binary ;;
  docker) build_docker ;;
  all)    build_binary; build_docker ;;
  *)      die "unknown target: $TARGET (binary|docker|all)" ;;
esac

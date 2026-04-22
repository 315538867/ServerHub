#!/usr/bin/env bash
# ServerHub — in-place upgrade.
#
# Backs up the current binary, downloads the requested version, swaps it in
# atomically, and restarts the service. On failure the previous binary is
# restored so the panel keeps running.
#
# Usage:
#   sudo bash upgrade.sh                 # upgrade to latest GitHub release
#   sudo bash upgrade.sh v1.2.3          # upgrade to a specific tag
#   sudo SH_DOWNLOAD_URL=https://... bash upgrade.sh   # custom tarball

set -euo pipefail

REPO="serverhub/serverhub"
BIN_PATH="/usr/local/bin/serverhub"
BACKUP_PATH="/usr/local/bin/serverhub.prev"
SERVICE="serverhub.service"

log()  { printf '\033[0;36m[upgrade]\033[0m %s\n' "$*"; }
warn() { printf '\033[0;33m[upgrade]\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[0;31m[upgrade]\033[0m %s\n' "$*" >&2; exit 1; }

[[ $EUID -eq 0 ]] || die "must be run as root (try: sudo bash $0)"
[[ -x "$BIN_PATH" ]] || die "$BIN_PATH not found — run install.sh first"
command -v systemctl >/dev/null || die "systemd not found"
command -v curl >/dev/null      || die "curl is required"
command -v tar >/dev/null       || die "tar is required"

case "$(uname -m)" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  *) die "unsupported arch: $(uname -m)" ;;
esac

VERSION="${1:-}"
if [[ -n "${SH_DOWNLOAD_URL:-}" ]]; then
  URL="$SH_DOWNLOAD_URL"
  VERSION="${VERSION:-custom}"
else
  if [[ -z "$VERSION" ]]; then
    log "resolving latest release"
    VERSION="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
      | grep -oE '"tag_name":\s*"[^"]+"' | head -1 | cut -d'"' -f4)"
    [[ -n "$VERSION" ]] || die "failed to resolve latest version"
  fi
  ARCHIVE="serverhub_linux_${ARCH}"
  URL="https://github.com/$REPO/releases/download/$VERSION/${ARCHIVE}.tar.gz"
fi

CURRENT_VER="$("$BIN_PATH" --version 2>/dev/null || echo unknown)"
log "current: $CURRENT_VER → target: $VERSION ($ARCH)"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
cd "$TMP"

log "downloading $URL"
curl -fSL --retry 3 -o bundle.tar.gz "$URL"
if curl -fsS -o bundle.tar.gz.sha256 "$URL.sha256" 2>/dev/null; then
  log "verifying sha256"
  sha256sum -c <(awk '{print $1"  bundle.tar.gz"}' bundle.tar.gz.sha256)
else
  warn "no .sha256 sidecar — skipping checksum"
fi
tar -xzf bundle.tar.gz
NEW_BIN="$(find . -type f -name serverhub -perm -u+x | head -1)"
[[ -n "$NEW_BIN" ]] || die "serverhub binary not found inside archive"

log "stopping $SERVICE"
systemctl stop "$SERVICE" || true

log "backing up current binary → $BACKUP_PATH"
cp -p "$BIN_PATH" "$BACKUP_PATH"

log "installing new binary"
install -m 0755 "$NEW_BIN" "$BIN_PATH"

log "starting $SERVICE"
systemctl start "$SERVICE"
sleep 2

if systemctl is-active --quiet "$SERVICE"; then
  log "✓ upgraded to $VERSION"
  log "previous binary kept at $BACKUP_PATH (delete when satisfied)"
else
  warn "service failed to start — rolling back"
  install -m 0755 "$BACKUP_PATH" "$BIN_PATH"
  systemctl start "$SERVICE" || true
  if systemctl is-active --quiet "$SERVICE"; then
    die "rolled back to previous binary; inspect: journalctl -u $SERVICE -n 100"
  else
    die "rollback also failed; manual recovery needed: journalctl -u $SERVICE -n 100"
  fi
fi

#!/usr/bin/env bash
# ServerHub — one-shot installer.
#
# Usage:
#   sudo bash install.sh                      # latest release from GitHub
#   sudo bash install.sh v1.2.3               # specific version
#   sudo SH_DOWNLOAD_URL=https://... bash install.sh   # custom tarball source
#
# Idempotent: re-running upgrades the binary, preserves config and data.
# On first install, random jwt_secret/aes_key are generated. DO NOT LOSE
# /etc/serverhub/config.yaml — aes_key is required to decrypt stored
# passwords; losing it means all encrypted fields are unrecoverable.

set -euo pipefail

REPO="serverhub/serverhub"
BIN_PATH="/usr/local/bin/serverhub"
CONFIG_DIR="/etc/serverhub"
DATA_DIR="/var/lib/serverhub"
SERVICE_PATH="/etc/systemd/system/serverhub.service"
SH_USER="serverhub"

log()  { printf '\033[0;36m[install]\033[0m %s\n' "$*"; }
warn() { printf '\033[0;33m[install]\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[0;31m[install]\033[0m %s\n' "$*" >&2; exit 1; }

[[ $EUID -eq 0 ]] || die "must be run as root (try: sudo bash $0)"
command -v systemctl >/dev/null || die "systemd not found; see docs/deployment.md for manual steps"
command -v openssl >/dev/null   || die "openssl is required to generate secrets"
command -v tar >/dev/null       || die "tar is required"

# ── 1. resolve download URL ───────────────────────────────────
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
[[ "$OS" == "linux" ]] || die "only linux is supported (got: $OS)"

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
    log "resolving latest release from github.com/$REPO …"
    VERSION="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
      | grep -oE '"tag_name":\s*"[^"]+"' | head -1 | cut -d'"' -f4)"
    [[ -n "$VERSION" ]] || die "failed to resolve latest version"
  fi
  ARCHIVE="serverhub_linux_${ARCH}"
  URL="https://github.com/$REPO/releases/download/$VERSION/${ARCHIVE}.tar.gz"
fi
log "installing $VERSION ($ARCH)"

# ── 2. download + verify ──────────────────────────────────────
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
cd "$TMP"

log "downloading $URL"
curl -fSL --retry 3 -o bundle.tar.gz "$URL"
if curl -fsS -o bundle.tar.gz.sha256 "$URL.sha256" 2>/dev/null; then
  log "verifying sha256"
  sha256sum -c <(awk '{print $1"  bundle.tar.gz"}' bundle.tar.gz.sha256)
else
  warn "no .sha256 sidecar at $URL.sha256 — skipping checksum"
fi
tar -xzf bundle.tar.gz
BIN_SRC="$(find . -type f -name serverhub -perm -u+x | head -1)"
[[ -n "$BIN_SRC" ]] || die "serverhub binary not found inside archive"

# ── 3. system user, dirs ──────────────────────────────────────
if ! id -u "$SH_USER" >/dev/null 2>&1; then
  log "creating system user $SH_USER"
  useradd --system --no-create-home --shell /usr/sbin/nologin "$SH_USER"
fi
install -d -m 0755 "$(dirname "$BIN_PATH")"
install -d -o root -g "$SH_USER" -m 0750 "$CONFIG_DIR"
install -d -o "$SH_USER" -g "$SH_USER" -m 0750 "$DATA_DIR"

# ── 4. binary (atomic replace so a running service restarts cleanly) ───
log "installing binary to $BIN_PATH"
install -m 0755 "$BIN_SRC" "$BIN_PATH"

# ── 5. config (only if missing) ───────────────────────────────
CONF="$CONFIG_DIR/config.yaml"
if [[ ! -f "$CONF" ]]; then
  log "generating $CONF with random secrets"
  JWT="$(openssl rand -base64 48 | tr -d '\n=+/' | cut -c1-64)"
  AES="$(openssl rand -hex 32)"
  umask 077
  cat > "$CONF" <<EOF
server:
  port: 9999
  data_dir: $DATA_DIR

security:
  jwt_secret: "$JWT"
  aes_key: "$AES"
  allow_register: false
  login_max_attempts: 5
  login_lockout_min: 15

certbot:
  email: "admin@example.com"
  webroot: $DATA_DIR/acme

nginx:
  conf_dir: /etc/nginx/conf.d
  reload_cmd: "nginx -s reload"
  test_cmd: "nginx -t"

log:
  level: info
  file: ""
  max_size_mb: 100
  max_days: 30

scheduler:
  metrics_interval_sec: 5
  cert_check_hour: 2
  deploy_log_keep_days: 30
EOF
  chown root:"$SH_USER" "$CONF"
  chmod 640 "$CONF"
else
  log "$CONF already exists — keeping current config"
fi

# ── 6. systemd unit ───────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ -f "$SCRIPT_DIR/serverhub.service" ]]; then
  install -m 0644 "$SCRIPT_DIR/serverhub.service" "$SERVICE_PATH"
else
  # Fallback: write an inline unit matching scripts/serverhub.service.
  cat > "$SERVICE_PATH" <<'EOF'
[Unit]
Description=ServerHub Panel
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=serverhub
Group=serverhub
ExecStart=/usr/local/bin/serverhub --config /etc/serverhub/config.yaml
Restart=on-failure
RestartSec=5
StateDirectory=serverhub
StateDirectoryMode=0750
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectControlGroups=yes
RestrictSUIDSGID=yes
LockPersonality=yes
ReadWritePaths=/var/lib/serverhub /etc/serverhub
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF
fi

systemctl daemon-reload
systemctl enable serverhub.service >/dev/null
systemctl restart serverhub.service
sleep 1
if systemctl is-active --quiet serverhub.service; then
  log "✓ serverhub is running"
else
  warn "service not active — inspect with: journalctl -u serverhub -n 100"
  exit 1
fi

# ── 7. summary ────────────────────────────────────────────────
cat <<EOF

──────────────────────────────────────────────────────────────
 ServerHub $VERSION installed.
 Panel:   http://$(hostname -I 2>/dev/null | awk '{print $1}' ):9999/panel/
 Config:  $CONF           (aes_key is IRREVERSIBLE — back this up)
 Data:    $DATA_DIR           (SQLite + ssh_keys + deploy-logs)
 Service: systemctl {status,restart,stop} serverhub
 Logs:    journalctl -u serverhub -f
 Login:   admin / admin   (change immediately)
──────────────────────────────────────────────────────────────
EOF

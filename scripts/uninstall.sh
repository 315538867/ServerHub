#!/usr/bin/env bash
# ServerHub — uninstaller.
#
# Stops the service, removes the systemd unit and binary. Data and config are
# intentionally kept unless --purge is passed, because losing config.yaml
# permanently destroys every AES-encrypted field stored in the DB.

set -euo pipefail

BIN_PATH="/usr/local/bin/serverhub"
CONFIG_DIR="/etc/serverhub"
DATA_DIR="/var/lib/serverhub"
SERVICE_PATH="/etc/systemd/system/serverhub.service"
SH_USER="serverhub"

log()  { printf '\033[0;36m[uninstall]\033[0m %s\n' "$*"; }
warn() { printf '\033[0;33m[uninstall]\033[0m %s\n' "$*" >&2; }

[[ $EUID -eq 0 ]] || { echo "must be run as root"; exit 1; }

PURGE=0
for a in "$@"; do
  [[ "$a" == "--purge" ]] && PURGE=1
done

if systemctl list-unit-files serverhub.service >/dev/null 2>&1; then
  log "stopping serverhub"
  systemctl disable --now serverhub.service 2>/dev/null || true
fi
rm -f "$SERVICE_PATH"
systemctl daemon-reload

rm -f "$BIN_PATH"
log "binary removed"

if [[ $PURGE -eq 1 ]]; then
  warn "--purge: removing $CONFIG_DIR and $DATA_DIR (IRREVERSIBLE)"
  rm -rf "$CONFIG_DIR" "$DATA_DIR"
  if id -u "$SH_USER" >/dev/null 2>&1; then
    userdel "$SH_USER" || true
  fi
  log "purge complete"
else
  cat <<EOF

Config and data kept:
  $CONFIG_DIR
  $DATA_DIR

Re-run with --purge to delete them (aes_key cannot be recovered; all
encrypted secrets in the DB become permanently unreadable).
EOF
fi

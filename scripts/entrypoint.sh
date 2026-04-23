#!/bin/bash
# entrypoint.sh — 容器启动胶水脚本，负责：
#   1. 把挂入容器的 /var/run/docker.sock 的宿主 GID 同步到容器内 "docker" 组，
#      并将 serverhub 用户加入该组——这样用户 `docker run` 就不用再写
#      `--group-add <gid>`，部署命令大幅简化。
#   2. 启动阶段一律以 root 起，初始化完降权到非特权 serverhub (uid 65532)。
#
# 本脚本仅处理"容器零配置本机纳管"的胶水，业务能力探测在 Go 侧由
# sysinfo.LocalCapability() 完成（启动时一次）。因此这里不判断 --pid=host /
# /host 是否存在——那些仅影响 Capability=full|docker|none 的结论。
set -eu

SOCK=/var/run/docker.sock
USER_NAME=serverhub

if [ -S "$SOCK" ]; then
  SOCK_GID=$(stat -c '%g' "$SOCK")
  if [ "$SOCK_GID" != "0" ]; then
    # Avoid clashing with an existing group that happens to own that GID
    # (common on Debian: GID 999 is "systemd-journal"). groupmod rewrites
    # the existing "docker" group if one was baked into the image, or
    # reuses whatever group already holds this GID.
    EXISTING=$(getent group "$SOCK_GID" | cut -d: -f1 || true)
    if [ -n "$EXISTING" ]; then
      GROUP_NAME="$EXISTING"
    elif getent group docker >/dev/null; then
      groupmod -g "$SOCK_GID" docker
      GROUP_NAME=docker
    else
      groupadd -g "$SOCK_GID" docker
      GROUP_NAME=docker
    fi
    usermod -aG "$GROUP_NAME" "$USER_NAME"
  fi
fi

# Ensure data dir is writable by serverhub (useful on bind mounts owned by root).
# Skip the chown when ownership already matches — recursive chown on a large
# bind-mounted volume (10k+ files) noticeably delays container boot on every
# restart.
if [ -d /data ]; then
  TARGET_UID=$(id -u "$USER_NAME")
  CUR_UID=$(stat -c '%u' /data 2>/dev/null || echo 0)
  if [ "$CUR_UID" != "$TARGET_UID" ]; then
    chown -R "$USER_NAME:$USER_NAME" /data 2>/dev/null || true
  fi
fi

exec gosu "$USER_NAME" /serverhub "$@"

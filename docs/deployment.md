# 部署

三种部署方式，按使用场景任选其一：

| 方式 | 适合 | 特点 |
|---|---|---|
| [1. install.sh](#1-installsh-裸机单机) | 裸机 / VPS 单机 | 幂等脚本，systemd 托管，加固沙箱 |
| [2. Docker / Compose](#2-docker--compose) | 已有容器化环境 | distroless 镜像，nonroot，单容器 |
| [3. 手动构建](#3-手动构建) | 离线 / 定制 | 自编译 + 自写 systemd |

> **aes_key 警告**：`security.aes_key` 用于加密服务器密码 / 私钥 / 部署环境变量。**丢失后 DB 中所有加密字段不可恢复**。任何方式部署都必须妥善保存 `config.yaml`。

---

## 1. install.sh（裸机单机）

### 安装

```bash
# 最新版本
curl -fsSL https://raw.githubusercontent.com/serverhub/serverhub/main/scripts/install.sh \
  | sudo bash

# 指定版本
curl -fsSL https://raw.githubusercontent.com/serverhub/serverhub/main/scripts/install.sh \
  | sudo bash -s -- v1.2.3

# 自定义下载源（内网镜像、离线包）
sudo SH_DOWNLOAD_URL=https://mirror.example.com/serverhub_linux_amd64.tar.gz bash install.sh
```

脚本会：

- 校验 OS/arch（仅支持 linux amd64/arm64）
- 下载 tar.gz 并验证 `.sha256` sidecar（若 release 附带）
- 创建 `serverhub` 系统用户（nologin）
- 布置目录：
  - `/usr/local/bin/serverhub` — 二进制
  - `/etc/serverhub/` — 配置（`root:serverhub` `0750`）
  - `/var/lib/serverhub/` — SQLite + ssh_keys + 部署日志（`serverhub:serverhub` `0750`）
- **首次**生成随机 `jwt_secret`（64 字符）与 `aes_key`（32 字节 hex）写入 `/etc/serverhub/config.yaml`（`0640`）
- 安装加固版 `systemd` unit，`enable --now`

再次运行 = 原地升级（二进制被 `install(1)` 原子替换，config / data 保留）。

### 日常操作

```bash
systemctl status  serverhub
systemctl restart serverhub
journalctl -u serverhub -f
```

### 升级

```bash
curl -fsSL https://raw.githubusercontent.com/serverhub/serverhub/main/scripts/upgrade.sh \
  | sudo bash                            # 最新版
# 或 sudo bash upgrade.sh v1.2.3
```

`upgrade.sh` 会备份旧二进制到 `/usr/local/bin/serverhub.prev`，启动失败自动回滚。

### 卸载

```bash
sudo bash scripts/uninstall.sh           # 仅移除服务与二进制，保留数据
sudo bash scripts/uninstall.sh --purge   # 连同 /etc/serverhub 和 /var/lib/serverhub 全部删除
```

> `--purge` 会永久删除 `aes_key`，**DB 中所有加密字段将无法解密**。再次安装等同于全新部署。

### systemd 加固摘要

`scripts/serverhub.service` 启用了：

- `User/Group=serverhub`、`NoNewPrivileges`、`RestrictSUIDSGID`
- `ProtectSystem=strict` + `ReadWritePaths=/var/lib/serverhub /etc/serverhub`
- `ProtectHome`、`PrivateTmp`、`ProtectKernelTunables/Modules`、`ProtectControlGroups`
- `LockPersonality`、`LimitNOFILE=65535`

如果把 `nginx.conf_dir` 或 `certbot.webroot` 指向默认路径之外，记得在 `ReadWritePaths` 里加上对应目录，否则 unit 无法写回。

---

## 2. Docker / Compose

仓库不推送镜像到任何公共 registry —— 分发方式由使用者自行决定（本地构建 / 公司内网 registry / 离线 tar）。

### 2.1 本地构建

```bash
docker build -t serverhub:local .
```

Dockerfile 为多阶段构建：`node:20` 构前端 → `golang:1.25` 构后端（CGO=1，支持 amd64/arm64 交叉编译） → `gcr.io/distroless/base-debian12:nonroot` 运行。

### 2.2 使用 GitHub Release 的 OCI tar（离线）

每个 release 附带 `serverhub_linux_<arch>.image.tar`：

```bash
curl -fLO https://github.com/serverhub/serverhub/releases/download/v1.2.3/serverhub_linux_amd64.image.tar
docker load -i serverhub_linux_amd64.image.tar
# 自行 tag / push 到你的 registry
```

### 2.3 运行（Compose）

仓库提供最小 `docker-compose.yml`：

```bash
docker compose up -d
docker compose logs -f serverhub
```

首次启动会在卷 `serverhub-data`（挂载到容器 `/data`）下生成 `config.yaml`。**修改其中的 `security.*` 后重启**：

```bash
docker compose exec serverhub /serverhub --help      # 容器内无 shell
docker compose restart serverhub
```

容器暴露 `/healthz`，`HEALTHCHECK` 走二进制自带的 `--healthcheck` 子命令（distroless 无 shell，无法用 curl）。

### 2.4 运行（docker run）

```bash
docker volume create serverhub-data
docker run -d \
  --name serverhub \
  --restart unless-stopped \
  -p 9999:9999 \
  -v serverhub-data:/data \
  -e SERVERHUB_DATA_DIR=/data \
  -e SERVERHUB_CONFIG=/data/config.yaml \
  serverhub:local
```

### 2.5 环境变量

容器默认读取这三个（flag 仍然可覆盖）：

| 变量 | 默认 | 作用 |
|---|---|---|
| `SERVERHUB_DATA_DIR` | `/data` | SQLite、ssh_keys、部署日志目录 |
| `SERVERHUB_CONFIG`   | `/data/config.yaml` | 配置文件路径 |
| `SERVERHUB_PORT`     | `9999` | 监听端口（与 `config.yaml` 的 `server.port` 等价，flag > env > config） |

### 2.6 SSH 客户端密钥与容器

面板首启会在 `$DATA_DIR/ssh_keys/` 生成本机 SSH 密钥对，用于连接被管服务器。若从旧 host 迁移，把旧 `ssh_keys/` 拷进新卷即可沿用公钥。

---

## 3. 手动构建

### 前置

Go 1.25+、Node 18+（推荐 Bun 1.2+ 构前端）、CGO 工具链（`gcc` / `clang`，SQLite 需要）。跨平台交叉编译需自带 C 交叉编译器，通常直接在目标平台编译更省事。

### 构建

```bash
make build              # → backend/serverhub（前端静态资源已 embed）
```

### 最小 systemd unit

```ini
[Unit]
Description=ServerHub Panel
After=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/serverhub --config /etc/serverhub/config.yaml
Restart=on-failure
RestartSec=5
User=root
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```

生产建议直接拿 `scripts/serverhub.service` 作为起点（带完整沙箱化设置），再按 `nginx.conf_dir` 等实际路径调整 `ReadWritePaths`。

### 反向代理（可选）

WebSocket（终端、日志流、部署日志）走 `Upgrade` 头，Nginx 必须包含如下两行：

```nginx
server {
  listen 443 ssl http2;
  server_name panel.example.com;

  ssl_certificate     /etc/letsencrypt/live/panel.example.com/fullchain.pem;
  ssl_certificate_key /etc/letsencrypt/live/panel.example.com/privkey.pem;

  location /panel/ {
    proxy_pass http://127.0.0.1:9999;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_read_timeout 3600;
    proxy_send_timeout 3600;
  }
}
```

---

## config.yaml 参考

```yaml
server:
  port: 9999
  data_dir: /var/lib/serverhub

security:
  jwt_secret: "<64 字符随机串>"
  aes_key:    "<64 hex 字符 = 32 字节>"       # 丢失 = 加密字段不可恢复
  allow_register: false
  login_max_attempts: 5
  login_lockout_min: 15

certbot:
  email: admin@example.com
  webroot: /var/lib/serverhub/acme

nginx:
  conf_dir: /etc/nginx/conf.d
  reload_cmd: "nginx -s reload"
  test_cmd:   "nginx -t"

log:
  level: info
  file: ""
  max_size_mb: 100
  max_days: 30

scheduler:
  metrics_interval_sec: 5
  cert_check_hour: 2
  deploy_log_keep_days: 30
```

生成密钥：

```bash
openssl rand -hex 32                                              # aes_key
openssl rand -base64 48 | tr -d '\n=+/' | head -c 64 ; echo       # jwt_secret
```

`install.sh` 首装时会自动生成，无需手工准备。

---

## 目录布局

`data_dir`（install.sh 下是 `/var/lib/serverhub`，容器下是 `/data`）：

```
<data_dir>/
├── serverhub.db            # SQLite 主库
├── serverhub.db-wal        # WAL
├── serverhub.db-shm
├── ssh_keys/               # 本机 SSH 客户端密钥
├── logs/                   # 自身日志（可选）
├── deploy-logs/            # 历史部署日志归档
└── acme/                   # certbot webroot（如启用）
```

---

## 健康检查

```
GET /healthz  → 200 {"status":"ok","version":"v1.2.3"}
```

容器内部无 shell，改用二进制自带子命令（install.sh / systemd 外也可用）：

```bash
/usr/local/bin/serverhub --healthcheck           # exit 0 ok / 1 fail
```

---

## 备份与恢复

每天备份 `data_dir` 与 `/etc/serverhub/config.yaml` 即可。SQLite 推荐用 `.backup` 而非直接 cp，避免 WAL 未 checkpoint：

```bash
sqlite3 /var/lib/serverhub/serverhub.db ".backup /tmp/serverhub-$(date +%F).db"
tar -czf /tmp/serverhub-$(date +%F).tar.gz \
  /etc/serverhub/config.yaml \
  /var/lib/serverhub/ssh_keys \
  /tmp/serverhub-$(date +%F).db
```

恢复：停服 → 替换 `serverhub.db` / `config.yaml` / `ssh_keys/` → 启服。`aes_key` 必须与备份源一致，否则 DB 里的加密字段无法解密。

---

## 升级

- **install.sh 路径**：`scripts/upgrade.sh [version]`（失败自动回滚）
- **Docker 路径**：`docker compose pull`（或重新 `docker build` / `docker load`）→ `docker compose up -d`
- **手动路径**：`systemctl stop serverhub` → 备份 DB → 替换二进制 → `systemctl start serverhub`

数据库启动时 AutoMigrate，新增字段/索引由 `pkg/database.Init` 自动处理，无需手工 SQL。

---

## 容量参考

| 项 | 默认 | 影响 |
|---|---|---|
| metrics 采集间隔 | 5s | N 台 × 17280 行/天/台 |
| metrics 保留 | 30d | 30 台 ≈ 15M 行，SQLite 仍轻松 |
| audit 保留 | 90d | 写入异步，查询走 path/username 索引 |
| SSH 并发 | 8 | 大于 30 台时保持默认，避免远端连接风暴 |
| 日志搜索并发 | 8 | 超出立即 429，前端提示稍后重试 |

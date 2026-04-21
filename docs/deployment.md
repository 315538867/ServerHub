# 部署

## 构建产物

```
make build
# → backend/serverhub  （单文件，已嵌入前端）
```

要求：Go 1.25+、Node 18+、CGO 工具链（gcc/clang，SQLite 需要）。

跨平台交叉编译需自带 C 交叉编译器，建议直接在目标平台上编译。

## 目录布局

默认 `--data /opt/serverhub/`：

```
/opt/serverhub/
├── config.yaml          # 配置（首次启动自动生成默认值）
├── serverhub.db         # SQLite 主库
├── serverhub.db-wal     # WAL
├── serverhub.db-shm
├── ssh_keys/            # 生成的会话密钥
├── logs/                # 自身日志（可选）
└── deploy-logs/         # 历史部署日志归档
```

二进制位置不限。建议放 `/usr/local/bin/serverhub`。

## config.yaml

```yaml
server:
  port: 9999
  data_dir: /opt/serverhub

security:
  jwt_secret: "<64 字符随机串，请替换>"
  aes_key:    "<64 hex 字符 = 32 字节，请替换>"
  allow_register: false
  login_max_attempts: 5
  login_lockout_min: 15

certbot:
  email: admin@example.com
  webroot: /var/www/html

nginx:
  conf_dir: /etc/nginx
  reload_cmd: nginx -s reload
  test_cmd:   nginx -t

log:
  level: info
  file: ""              # 空 = stderr
  max_size_mb: 100
  max_days: 30

scheduler:
  metrics_interval_sec: 5
  cert_check_hour: 2
  deploy_log_keep_days: 30
```

**生产必须修改** `jwt_secret` 与 `aes_key`，否则启动 stderr 会有大段告警。

生成示例：

```bash
openssl rand -hex 32   # → aes_key
openssl rand -base64 48 | tr -d '\n' | head -c 64 ; echo   # → jwt_secret
```

> 注：`aes_key` 用于加密服务器密码/私钥/部署 env 等，**线上变更后旧数据无法解密**。仅在初次部署设置。

## systemd

`/etc/systemd/system/serverhub.service`：

```ini
[Unit]
Description=ServerHub Panel
After=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/serverhub --config /opt/serverhub/config.yaml
Restart=on-failure
RestartSec=5
User=root
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```

```bash
systemctl daemon-reload
systemctl enable --now serverhub
journalctl -u serverhub -f
```

## 反向代理（可选）

如果通过 Nginx 暴露 HTTPS：

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

WebSocket（终端、日志流、部署日志）走 `Upgrade` 头，必须配置上述两行。

## 升级

1. `systemctl stop serverhub`
2. 备份 `/opt/serverhub/serverhub.db*`
3. 替换二进制
4. `systemctl start serverhub`

数据库会在启动时自动 AutoMigrate；新版本若加了字段或索引，由 `database.Init` 自动处理。

## 备份建议

仅需备份 `/opt/serverhub/`：

```bash
sqlite3 /opt/serverhub/serverhub.db ".backup /tmp/serverhub-$(date +%F).db"
```

部署时定期 rsync 上述目录到对象存储即可。

## 容量规划

| 项 | 默认 | 影响 |
|---|---|---|
| metrics 采集间隔 | 5s | N 台 × 17280 行/天/台 |
| metrics 保留 | 30d | 30 台约 ~15M 行 → SQLite 仍轻松 |
| audit 保留 | 90d | 写入异步，查询走 path/username 索引 |
| SSH 并发 | 8 | 大于 30 台时建议保持，避免远端瞬时连接风暴 |
| 日志搜索并发 | 8 | 超出立即 429，前端提示稍后重试 |

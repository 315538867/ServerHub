# 部署方案

## 一、本地桌面 App 运行（推荐）

> 无需公网服务器，下载即用。适合个人开发者在 Mac / Linux / Windows 本机管理远程服务器。

### 安装

**macOS：**
```bash
# 下载 macOS 版本
curl -L https://github.com/xxx/serverhub/releases/latest/download/serverhub-darwin-amd64 \
  -o /usr/local/bin/serverhub && chmod +x /usr/local/bin/serverhub

# 首次运行（自动初始化配置 + 打开浏览器）
serverhub
# 访问：http://localhost:9999/panel/
# 默认账号：admin / admin123（首次运行提示修改密码）
```

或直接下载 `ServerHub.app`，拖入 `/Applications` 双击启动。

**Linux：**
```bash
curl -L https://github.com/xxx/serverhub/releases/latest/download/serverhub-linux-amd64 \
  -o /usr/local/bin/serverhub && chmod +x /usr/local/bin/serverhub
serverhub
```

**Windows：**
下载 `serverhub-windows-amd64.exe`，双击运行，系统托盘出现图标后浏览器自动打开。

---

### 系统托盘功能

```
托盘图标（常驻菜单栏）：
  🟢  全部在线        → 无告警
  🟠  2 条未处理告警  → 点击打开告警列表
  🔴  Server 2 离线   → 右键菜单显示离线服务器

右键菜单：
  ┌─────────────────────────────┐
  │  打开面板                    │
  │  ─────────────────────────  │
  │  Server 1  🟢  82.156.x.x   │
  │  Server 2  🔴  10.0.0.3     │
  │  ─────────────────────────  │
  │  2 条未读告警                │
  │  ─────────────────────────  │
  │  退出                        │
  └─────────────────────────────┘
```

**原生系统通知示例：**
```
[ServerHub] Server 2 已离线
[ServerHub] CPU 持续 > 90%（Server 1，当前 94%）
[ServerHub] SSL 证书即将到期：api.xxx.com 剩 5 天
[ServerHub] 部署完成：myapp @ main (a1b2c3d)
```

---

### 开机自启（可选）

**macOS（launchd）：**
```bash
serverhub --install-autostart   # 写入 ~/Library/LaunchAgents/com.serverhub.plist
serverhub --remove-autostart    # 移除
```

**Linux（systemd --user）：**
```bash
serverhub --install-autostart   # 写入 ~/.config/systemd/user/serverhub.service
systemctl --user enable --now serverhub
```

---

### 桌面模式功能对比

| 功能 | 桌面模式 | 服务器模式 |
|------|---------|-----------|
| SSH 终端 / 文件管理 | ✅ | ✅ |
| Docker / Nginx / 部署（手动） | ✅ | ✅ |
| SSL 证书申请续期 | ✅ | ✅ |
| 系统托盘 + 原生通知 | ✅ | ❌ |
| 实时监控（面板运行时） | ✅ | ✅ 持续 |
| 调度任务（面板运行时） | ✅ | ✅ 24/7 |
| Push-to-Deploy Webhook | ❌ | ✅ |
| 多用户共用 | ❌（localhost 访问） | ✅ |

---

## 二、开发环境

### 依赖

```bash
go version        # Go 1.22+
node -v           # Node.js 20+
```

### 启动

```bash
# 终端 1：后端（--dev 模式：跳过前端 embed，允许跨域）
cd backend
go run main.go --dev --port 9999 --data ./dev-data

# 终端 2：前端（Vite HMR + 代理到 :9999）
cd frontend
npm install
npm run dev
# 访问：http://localhost:5173/panel/
```

**Vite 开发代理（vite.config.ts）：**
```typescript
server: {
  proxy: {
    '/panel/api':      { target: 'http://localhost:9999', changeOrigin: true, ws: true },
    '/panel/webhooks': { target: 'http://localhost:9999', changeOrigin: true },
  }
}
```

首次运行，自动初始化：
```bash
# dev-data 目录不存在时，自动创建 SQLite + 默认 admin 用户（密码: admin123）
# 生产环境用 `serverhub init` 重新设置密码
```

---

## 三、生产构建

### Makefile

```makefile
.PHONY: build clean

build:
	@echo "==> 构建前端..."
	cd frontend && npm ci && npm run build
	@echo "==> 构建后端（go embed 打包前端）..."
	cd backend && CGO_ENABLED=1 go build \
		-ldflags="-s -w -X main.Version=$(shell git describe --tags --always)" \
		-o ../serverhub .
	@echo "==> 构建完成：./serverhub"

build-linux:
	cd frontend && npm ci && npm run build
	cd backend && CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
		-ldflags="-s -w -X main.Version=$(shell git describe --tags --always)" \
		-o ../serverhub-linux-amd64 .

build-linux-arm64:
	cd frontend && npm ci && npm run build
	cd backend && CGO_ENABLED=1 GOOS=linux GOARCH=arm64 CC=aarch64-linux-gnu-gcc go build \
		-ldflags="-s -w" -o ../serverhub-linux-arm64 .

clean:
	rm -f serverhub serverhub-linux-amd64 serverhub-linux-arm64
	rm -rf frontend/dist backend/web/dist
```

> **注意：** SQLite 需要 CGO（`CGO_ENABLED=1`），交叉编译时需要对应平台的 gcc 工具链。
> 替代方案：使用 `modernc.org/sqlite`（纯 Go，无 CGO），可直接交叉编译。

### go embed 配置

```go
// backend/main.go 或 backend/static.go
//go:embed web/dist
var staticFiles embed.FS

// Gin 注册（前端 history mode 路由）
r.NoRoute(func(c *gin.Context) {
    path := c.Request.URL.Path
    if strings.HasPrefix(path, "/panel/api") || strings.HasPrefix(path, "/panel/webhooks") {
        c.JSON(404, gin.H{"code": 404, "msg": "not found"})
        return
    }
    c.FileFromFS("web/dist/index.html", http.FS(staticFiles))
})
```

---

## 四、生产部署

### 安装

```bash
# 下载 release 二进制（或本地 make build-linux 后上传）
sudo wget -O /usr/local/bin/serverhub \
    https://github.com/xxx/serverhub/releases/latest/download/serverhub-linux-amd64
sudo chmod +x /usr/local/bin/serverhub

# 创建数据目录
sudo mkdir -p /opt/serverhub/{data,ssh_keys,logs,deploy-logs,acme}
sudo chown -R root:root /opt/serverhub

# 生成配置文件
sudo serverhub init
# 交互式设置：管理员密码、JWT 密钥（自动生成）、AES 密钥（自动生成）
# 输出 config.yaml 到 /opt/serverhub/config.yaml
```

### 配置文件

```yaml
# /opt/serverhub/config.yaml

server:
  port: 9999
  data_dir: /opt/serverhub

security:
  jwt_secret: "<由 serverhub init 自动生成，64 字节随机字符串>"
  aes_key: "<由 serverhub init 自动生成，32 字节 hex>"
  allow_register: false          # 设为 true 开放注册（多用户升级）
  login_max_attempts: 5          # 最大失败次数
  login_lockout_min: 15          # 锁定时间（分钟）

certbot:
  email: "admin@yourdomain.com"  # Let's Encrypt 通知邮箱
  webroot: /opt/serverhub/acme

nginx:
  conf_dir: /etc/nginx/conf.d
  # 如果使用 1Panel 的 OpenResty Docker：
  # conf_dir: /opt/1panel/apps/openresty/openresty/conf/conf.d
  # reload_cmd: "docker exec 1Panel-openresty-7Xsf nginx -s reload"
  # test_cmd: "docker exec 1Panel-openresty-7Xsf nginx -t"

log:
  level: info                    # debug | info | warn | error
  file: /opt/serverhub/logs/serverhub.log
  max_size_mb: 100
  max_days: 30

scheduler:
  metrics_interval_sec: 5        # 指标采集间隔
  cert_check_hour: 2             # 证书检查时间（每天凌晨 2 点）
  deploy_log_keep_days: 30       # 部署日志保留天数
```

### systemd 服务

```ini
# /etc/systemd/system/serverhub.service
[Unit]
Description=ServerHub - SSH-native Server Console
Documentation=https://github.com/xxx/serverhub
After=network.target
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/serverhub
ExecStart=/usr/local/bin/serverhub --config /opt/serverhub/config.yaml
ExecReload=/bin/kill -HUP $MAINPID
Restart=always
RestartSec=5
LimitNOFILE=65536
StandardOutput=journal
StandardError=journal
SyslogIdentifier=serverhub

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now serverhub
sudo systemctl status serverhub

# 查看日志
sudo journalctl -u serverhub -f
```

---

## 五、OpenResty / Nginx 反代配置

### 标准 Nginx 配置

```nginx
# /etc/nginx/conf.d/serverhub.conf

server {
    listen 443 ssl;
    server_name your-domain.com;

    ssl_certificate     /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;
    ssl_protocols       TLSv1.2 TLSv1.3;
    ssl_ciphers         ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256;

    # 面板静态页面 + REST API
    location /panel/ {
        proxy_pass         http://127.0.0.1:9999/panel/;
        proxy_http_version 1.1;
        proxy_set_header   Host              $host;
        proxy_set_header   X-Real-IP         $remote_addr;
        proxy_set_header   X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto $scheme;
        proxy_read_timeout 300s;
        proxy_send_timeout 300s;
        proxy_buffering    off;
    }

    # WebSocket（终端 / 实时数据 / 部署日志）
    location /panel/api/ws/ {
        proxy_pass             http://127.0.0.1:9999/panel/api/ws/;
        proxy_http_version     1.1;
        proxy_set_header       Upgrade    $http_upgrade;
        proxy_set_header       Connection "upgrade";
        proxy_set_header       Host       $host;
        proxy_read_timeout     3600s;    # 终端保持 1 小时
        proxy_send_timeout     3600s;
    }

    # Push-to-Deploy Webhook
    location /panel/webhooks/ {
        proxy_pass         http://127.0.0.1:9999/panel/webhooks/;
        proxy_http_version 1.1;
        proxy_set_header   Host              $host;
        proxy_set_header   X-Real-IP         $remote_addr;
        proxy_read_timeout 30s;
    }
}

# HTTP → HTTPS 重定向
server {
    listen 80;
    server_name your-domain.com;
    return 301 https://$host$request_uri;
}
```

### 1Panel OpenResty 配置（Docker 环境）

```nginx
# /opt/1panel/apps/openresty/openresty/conf/conf.d/serverhub.conf
# 同上，reload 命令：docker exec 1Panel-openresty-7Xsf nginx -s reload
```

访问地址：`https://your-domain.com/panel/`

---

## 六、Docker 部署方案（可选）

```dockerfile
# Dockerfile（多阶段构建）

# 阶段 1：构建前端
FROM node:20-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ .
RUN npm run build

# 阶段 2：构建后端
FROM golang:1.22-alpine AS backend-builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
COPY --from=frontend-builder /app/frontend/dist ./web/dist
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /serverhub .

# 阶段 3：最终镜像
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata certbot openssh-client
COPY --from=backend-builder /serverhub /usr/local/bin/serverhub
VOLUME ["/opt/serverhub/data"]
EXPOSE 9999
ENTRYPOINT ["serverhub", "--config", "/opt/serverhub/config.yaml"]
```

```yaml
# compose.yml（生产 Docker 部署）
services:
  serverhub:
    image: serverhub:latest
    container_name: serverhub
    restart: unless-stopped
    ports:
      - "9999:9999"
    volumes:
      - ./data:/opt/serverhub/data
      - ./config.yaml:/opt/serverhub/config.yaml:ro
      - ./logs:/opt/serverhub/logs
      - ./deploy-logs:/opt/serverhub/deploy-logs
      - ./acme:/opt/serverhub/acme
    environment:
      - TZ=Asia/Shanghai
```

---

## 七、健康检查

```bash
# 检查服务是否正常运行（无需认证）
curl http://localhost:9999/panel/api/v1/health

# 预期响应
{
  "code": 0,
  "data": {
    "version": "1.0.0",
    "uptime": 3600,
    "db_status": "ok",
    "servers_connected": 3,
    "servers_total": 5
  }
}
```

systemd 中配置健康检查（可选）：
```ini
ExecStartPost=/bin/sh -c 'sleep 3 && curl -sf http://localhost:9999/panel/api/v1/health > /dev/null'
```

---

## 八、升级流程

```bash
# 1. 停止服务
sudo systemctl stop serverhub

# 2. 备份数据库（重要！）
sudo cp /opt/serverhub/data/serverhub.db \
        /opt/serverhub/data/serverhub.db.$(date +%Y%m%d)

# 3. 替换二进制
sudo wget -O /usr/local/bin/serverhub \
    https://github.com/xxx/serverhub/releases/latest/download/serverhub-linux-amd64
sudo chmod +x /usr/local/bin/serverhub

# 4. 启动（自动执行数据库迁移）
sudo systemctl start serverhub
sudo journalctl -u serverhub -f --no-tail   # 确认启动无报错
```

---

## 九、备份建议

```bash
# 每日备份脚本（加入 cron: 0 3 * * *）
#!/bin/bash
BACKUP_DIR=/backup/serverhub/$(date +%Y-%m-%d)
mkdir -p $BACKUP_DIR

# 备份数据库
cp /opt/serverhub/data/serverhub.db $BACKUP_DIR/

# 备份加密 SSH 密钥目录
cp -r /opt/serverhub/data/ssh_keys $BACKUP_DIR/

# 备份配置（不含密钥明文）
cp /opt/serverhub/config.yaml $BACKUP_DIR/

# 保留最近 7 天
find /backup/serverhub/ -maxdepth 1 -type d -mtime +7 -exec rm -rf {} \;

echo "备份完成：$BACKUP_DIR"
```

---

## 十、环境变量（替代 config.yaml，适合容器部署）

```bash
SERVERHUB_PORT=9999
SERVERHUB_DATA_DIR=/opt/serverhub
SERVERHUB_JWT_SECRET=your-secret
SERVERHUB_AES_KEY=your-aes-key-hex
SERVERHUB_LOG_LEVEL=info
SERVERHUB_ALLOW_REGISTER=false
SERVERHUB_CERTBOT_EMAIL=admin@domain.com
TZ=Asia/Shanghai
```

配置优先级：`环境变量 > config.yaml > 默认值`

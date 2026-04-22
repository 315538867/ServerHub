# ServerHub

轻量级服务器面板 —— 单二进制、SSH 拉模型、Web 管理。面向 10~30 台规模的小团队与个人运维。

## 特性

- **服务器管理**：SSH/SFTP 连接池、实时资源指标（CPU/内存/磁盘/负载）、Web 终端（xterm.js + WS）
- **应用编排**：Docker / docker-compose / 原生脚本三类部署，env 加密存储，Webhook 触发，部署日志流式推送
- **Nginx 反代**：站点配置编辑、配置校验、热重载；应用级路由拼装写回
- **SSL**：证书列表、Let's Encrypt 申请、手动导入、到期巡检
- **系统运维**：防火墙规则、cron、服务、进程、文件管理（SFTP）
- **远端日志搜索**：docker / journalctl / nginx access|error 一次性 grep + .txt 导出
- **告警通知**：CPU/内存/磁盘/离线阈值触发；企业微信、钉钉、Telegram、自定义 Webhook
- **审计与保留策略**：审计日志异步入库，定时 retention 清理（audit 90d / metrics 30d）

## 技术栈

后端 Go 1.25 · Gin · GORM + SQLite · gorilla/websocket · gopsutil · golang.org/x/crypto/ssh
前端 Vue 3 · Vite · TypeScript · Naive UI · Pinia · ECharts · xterm.js · CodeMirror 6
单二进制（前端 `embed.FS` 嵌入），CGO=1 编译。

## 快速开始

### 生产部署（三选一）

**① install.sh — 裸机 / VPS**

```bash
curl -fsSL https://raw.githubusercontent.com/315538867/ServerHub/main/scripts/install.sh \
  | sudo bash
```

幂等脚本：下载对应架构二进制 → 创建 `serverhub` 系统用户 → 写加固版 systemd unit → 首装生成随机 `jwt_secret` / `aes_key`。再次运行即原地升级；配套 `scripts/upgrade.sh`（失败自动回滚）与 `scripts/uninstall.sh --purge`。

**② Docker / Compose**

```bash
docker build -t serverhub:local .
docker compose up -d
```

debian-slim 镜像（含 bash + docker-cli），nonroot uid 65532；或从 GitHub Release 下载 `serverhub_linux_<arch>.image.tar` 离线 `docker load`。

**③ 手动构建**

```bash
make build              # → backend/serverhub（单文件，前端已 embed）
```

要求 Go 1.25+、Node 18+（或 Bun 1.2+）、CGO 工具链（SQLite 需要）。

> **重要**：`security.aes_key` 用于加密服务器密码 / 私钥 / 部署 env —— **丢失后 DB 中所有加密字段不可恢复**，务必保存 `/etc/serverhub/config.yaml`（或容器卷里的 `config.yaml`）。

### 开发

```bash
make dev-backend        # cd backend && go run . --dev    → :9999
make dev-frontend       # cd frontend && bun run dev      → :5173
```

### 访问

首次访问 `http://<host>:9999/panel/` 会进入初始化向导：填写管理员账号 → 容器部署还会引导一键纳管宿主机（生成 SSH 密钥，宿主上粘贴一行命令即可）。完成后跳到面板首页。

## 文档

- [架构概览](docs/architecture.md) — 模块分层、数据流、并发与限流
- [技术栈](docs/tech-stack.md) — 后端/前端依赖清单
- [模块说明](docs/modules.md) — backend/api、backend/pkg 每包职责
- [数据模型](docs/database-design.md) — GORM 模型与索引
- [API 参考](docs/api-design.md) — 路由分组、请求/响应约定
- [前端设计](docs/frontend-design.md) — 路由、状态、UI 组件约定
- [部署](docs/deployment.md) — install.sh / Docker / 手动三种方式、systemd 加固、升级与备份
- [功能清单](docs/features.md) — 按模块枚举

## 协议

[MIT](LICENSE)

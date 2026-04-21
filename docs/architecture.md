# 架构概览

## 总体形态

```
┌─────────────────────────────────────────────────────────┐
│  浏览器 (Vue 3 SPA)                                     │
│   - axios → REST                                        │
│   - WebSocket → 终端 / 部署日志 / 服务日志              │
└────────────────────┬────────────────────────────────────┘
                     │  /panel/api/v1/*
┌────────────────────▼────────────────────────────────────┐
│  ServerHub 单二进制 (Go + Gin)                          │
│   ┌──────────────────────────────────────────────────┐  │
│   │ HTTP 路由层 (api/*)                              │  │
│   │  auth | servers | docker | files | system | ...  │  │
│   └──────────────────────────────────────────────────┘  │
│   ┌──────────────────────────────────────────────────┐  │
│   │ 中间件: Auth(JWT) · Audit(异步) · RateLimit      │  │
│   └──────────────────────────────────────────────────┘  │
│   ┌──────────────────────────────────────────────────┐  │
│   │ 服务层 (pkg/*)                                   │  │
│   │  runner · fsclient · sshpool · sftppool          │  │
│   │  wsstream · auditq · scheduler · deployer        │  │
│   │  retention · notify · crypto · totp · resp       │  │
│   └──────────────────────────────────────────────────┘  │
│   ┌──────────────────────────────────────────────────┐  │
│   │ 持久化: SQLite (WAL) — GORM models               │  │
│   └──────────────────────────────────────────────────┘  │
└──────┬───────────────────────────────────┬──────────────┘
       │ os/exec · pty · gopsutil          │ SSH (22) / SFTP
┌──────▼──────────┐        ┌───────────────▼──────────────┐
│  本机 (type=local) │        │  远端服务器 N 台 (type=ssh)    │
│  主服务器自身      │        │  sshd · docker · nginx ...   │
└──────────────────┘        └──────────────────────────────┘
```

定位：**主控 + 本机直采 + SSH 远端**，无 Agent。运行主机即 `id=1` 本机服务器。规模 10~30 台舒适，>50 台需评估。

## 请求生命周期

1. 浏览器请求 `/panel/api/v1/...`，axios 自动带 `Authorization: Bearer <jwt>`
2. `middleware.Auth` 解析 JWT，注入 `userID`、`username` 到 gin.Context
3. `middleware.Audit` 复制请求/响应元数据，**异步**投递到 `auditq.Default`，不阻塞请求
4. 业务 handler：
   - 普通查询：直接读 SQLite
   - 远程操作：通过 `runner.For(&server, cfg)` 取执行器；`local` 走 `os/exec` + creack/pty，`ssh` 走 `sshpool` 池化连接
   - 文件：`fsclient.For(&server, cfg)`，`local` 直操 `os`，`ssh` 走 `sftppool`
   - 流式：`wsstream.Stream(ws, runner, cmd, opts)` 推 WebSocket
5. 响应统一走 `pkg/resp` → `{code, msg, data}`

## 后台任务

| 模块 | 触发 | 作用 |
|---|---|---|
| `scheduler.Start` | 每 N 秒（默认 5s） | 全量服务器并发采集指标 → `metrics` 表，更新在线状态 |
| `scheduler` 内 `checkAlerts` | 每次采集后 | 比对 `alert_rules` 触发条件，写 `alert_events` + 发通知 |
| `scheduler.StartReconciler` | 周期性 | 对账 deploy `desired_version` vs `actual_version`，自动同步 |
| `auditq` worker | 实时 | 缓冲 channel 满 500 条或 1s 批量 `CreateInBatches` 入库 |
| `retention.Start` | 每日 02:00 | 清理 audit_logs(90d) / metrics(30d) / deploy_logs(配置)；月初 VACUUM |

## 并发与限流

- **SSH 采集**：`scheduler.collectSem` 信号量上限 8，避免并发 SSH 风暴
- **日志搜索**：`logsearch.searchSem` 上限 8，超出立即 `429`
- **登录**：`middleware.RateLimit` 基于 IP 滑窗，超限锁定
- **WebSocket 流**：`wsstream` 内部 256 缓冲 channel + 丢最旧策略，长行 4MB 缓冲，30s ping / 90s pong；对端断开向远端发 `SIGTERM`
- **SQLite**：开启 WAL + `synchronous=NORMAL`，写并发友好

## 安全模型

| 项 | 实现 |
|---|---|
| 认证 | JWT（HS256，密钥来自 `security.jwt_secret`），WebSocket 走 `?token=` query |
| 鉴权 | 单一 admin 角色（暂未实现细粒度 RBAC） |
| MFA | TOTP（`pkg/totp`），可选启用 |
| 敏感字段 | AES-256-GCM 加密入库：服务器密码/私钥、deploy 环境变量、DB 连接密码、Webhook 渠道 URL |
| 命令注入 | 远端命令统一经 `sq()`/`shellQuote()` 单引号转义；日志搜索源/since 走白名单，target 走正则白名单 |
| 默认凭据 | 启动检测 `dev-jwt-secret` / `devkey...` 默认值，命中则 stderr 告警 |

## 错误处理与可观测

- `middleware.Recover`：panic → 5xx + `log.Printf` 堆栈
- 业务统一 `resp.Fail(c, http, code, msg)`，code 与 http 解耦
- 自身指标：`/system/self`（gopsutil 取本进程 CPU/内存/goroutines/uptime/连接数）→ Dashboard 展示
- 没有引入 OTel/Prometheus；轻量级面板，依赖系统 journald 即可

## 静态资源

生产模式下 `web/dist` 通过 `embed.FS` 嵌入，`r.NoRoute` 兜底返回 `index.html` 实现 SPA history 路由。开发模式下后端只起 API，前端由 Vite dev server 反代。

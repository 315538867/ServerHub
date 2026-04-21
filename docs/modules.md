# 模块说明

## backend/api/* — HTTP 路由层

每个子包提供 `RegisterRoutes(group *gin.RouterGroup, db *gorm.DB, cfg *config.Config)`，由 `main.go` 装配到 `/panel/api/v1` 下。

| 子包 | 路由前缀 | 职责 |
|---|---|---|
| `auth` | `/auth` | 登录、登出、`/me`、MFA 启用/校验；带 RateLimit |
| `health` | `/health` | 健康检查（公开） |
| `servers` | `/servers` | 服务器 CRUD、连通性测试、即时采集 |
| `docker` | `/servers/:id/docker/*` | 容器/镜像列表、启停、日志（WS）、镜像拉取（WS） |
| `files` | `/servers/:id/files/*` | SFTP 浏览/上传/下载/删/改名 |
| `system` | `/servers/:id/system/*` | 防火墙、cron、进程、systemd 服务（含日志 WS） |
| `system` | `/system/self` | ServerHub 自身资源（gopsutil） |
| `nginx` | `/servers/:id/nginx/*` | 站点列表、读写、enable/disable、reload、access/error 日志（WS） |
| `ssl` | `/servers/:id/ssl/*` | 证书列表/上传/续期/删除/扫描；Let's Encrypt 申请（WS） |
| `logsearch` | `/servers/:id/logs/search` | 一次性 grep（journalctl/docker/nginx）+ 并发 429 |
| `application` | `/apps` | 应用 CRUD（聚合 server/deploy/db） |
| `approutes` | `/apps/:id/routes` | 应用 Nginx 反代路由 CRUD，写回 nginx 配置 |
| `deploy` | `/deploys` | 部署配置 CRUD、手动触发、日志流 WS |
| `deploy` (webhook) | `/webhooks/*` | Webhook 触发部署（公开 + 密钥校验） |
| `database` | `/database` | DBConn CRUD（MySQL/Redis） |
| `alerts` | `/alerts` | AlertRule / NotifyChannel / AlertEvent |
| `metrics` | `/metrics` | 历史指标查询 |
| `audit` | `/audit` | 审计日志查询（前缀匹配走索引） |
| `terminal` | `/servers/:id/terminal` | xterm WS（`?token=` 鉴权，绕过 Audit） |
| `settings` | `/settings` | KV 系统设置 |

## backend/pkg/* — 服务层

| 包 | 关键 API | 职责 |
|---|---|---|
| `sshpool` | `Connect(id, host, port, user, type, cred)` `Run(client, cmd)` `Dial(...)` `CollectMetrics(...)` `HumanizeErr` | SSH 连接池：以 serverID 复用 TCP；`Dial` 给长连接（终端、长流）；连接级 MaxSessions 共享 |
| `sftppool` | `Get(serverID, sshClient)` | SFTP 子系统连接池，复用 ssh client |
| `wsstream` | `Stream(ws, client, cmd, Opts)` `OptsFromQuery` | 统一 WS 推送：4MB 行缓冲、include/exclude/regex 过滤、有界 channel、ping/pong、断开发 SIGTERM |
| `auditq` | `New(db)` `Submit(log)` `Close()` | 审计日志异步队列：cap=2000 channel，500 批/1s flush，满则丢并 atomic 计数 |
| `scheduler` | `Start(db, cfg)` `StartReconciler(db, cfg)` | 周期采集指标 + 触发告警；部署对账 reconciler |
| `retention` | `Start(db)` | 每日 02:00 清理 audit/metrics/deploy_logs；月初 VACUUM |
| `deployer` | `Run(db, cfg, deploy, trigger)` | 三类部署执行（docker/docker-compose/native）+ 日志写库 |
| `notify` | `Send(channel, event)` | 告警通知发送：企业微信、钉钉、Telegram、自定义 Webhook |
| `crypto` | `Encrypt(plain, key)` `Decrypt(cipher, key)` `HashPassword` `CheckPassword` | AES-256-GCM + bcrypt |
| `totp` | `GenerateSecret` `Verify` | TOTP MFA |
| `resp` | `OK` `BadRequest` `NotFound` `InternalError` `Fail(c, http, code, msg)` | 统一 JSON 响应 `{code, msg, data}` |

## backend/middleware

| 中间件 | 作用 |
|---|---|
| `Recover` | panic → 500 + 记录堆栈 |
| `Auth(cfg)` | 校验 JWT → 注入 `userID`/`username` |
| `Audit(db)` | 复制元数据 → `auditq.Submit`；含 user-agent、ip、status、duration |
| `RateLimit(cfg)` | 登录端口 IP 限流 |

## backend/database

`Init(cfg)` 打开 SQLite，启用 WAL + `synchronous=NORMAL`，AutoMigrate 全部模型，并通过 `ensureIndexes` 建立非默认索引：

```
idx_audit_created  (audit_logs.created_at DESC)
idx_audit_username (audit_logs.username)
idx_audit_path     (audit_logs.path)
idx_metrics_server_created (metrics(server_id, created_at DESC))
```

并初始化默认 `settings` 行（如 `deploy_log_keep_days=30`）。

## backend/config

`Load(path)` 读取 YAML；不存在则写入默认值。字段见 [架构概览 — 配置](./architecture.md) 与 `backend/config/config.go`。

## backend/tray

可选系统托盘集成（macOS/Windows）。`tray.Run(serve, port)` 包装 `r.Run`，托盘菜单可打开浏览器、退出。

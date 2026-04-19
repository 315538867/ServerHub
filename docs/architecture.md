# 架构设计

## 一、运行模式

ServerHub 支持两种运行模式，共用同一套代码，通过 build tags 区分：

### 桌面模式（Desktop Mode）— 推荐个人开发者

```
┌─────────────────────────────────────────────────────────────┐
│              开发者本机（Mac / Linux）                         │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  系统托盘（menubar）                                   │   │
│  │  ● 图标状态：🟢在线 / 🔴离线 / 🟠告警                 │   │
│  │  ● 原生通知：CPU告警 / 服务器离线 / SSL到期            │   │
│  └──────────────────────────────────────────────────────┘   │
│                          │                                  │
│  ┌──────────────────────────────────────────────────────┐   │
│  │           ServerHub 主程序（Go 单二进制）              │   │
│  │  REST API + WebSocket → localhost:9999               │   │
│  │  调度器（指标采集 / 证书续签 / 告警检查）               │   │
│  │  SSH 连接池 + SQLite                                  │   │
│  └──────────────────────────────────────────────────────┘   │
│                          │                                  │
│         浏览器访问 http://localhost:9999/panel/              │
└─────────────────────────────┬───────────────────────────────┘
                              │ SSH（直连 / SSH over WireGuard）
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
        公网服务器A      公网服务器B      内网服务器C
        (10.0.0.2)      (10.0.0.3)      (10.0.0.4)
```

**优势：** 无需公网服务器，零配置，下载即用，告警通过系统通知直达，数据仅在本机。

**限制：** 不支持 Push-to-Deploy Webhook（无公网端点），机器关闭时调度器停止。

---

### 服务器模式（Server Mode）— 适合团队共用

```
┌─────────────────────────────────────────────────────────────┐
│                      浏览器（用户）                            │
│              Vue 3 + TypeScript + Element Plus               │
└──────────────────────────┬──────────────────────────────────┘
                           │ HTTPS / WSS
                           ▼
┌─────────────────────────────────────────────────────────────┐
│               主控服务器（公网，运行面板）                      │
│  ┌───────────────────────────────────────────────────────┐  │
│  │              OpenResty（80 / 443）                     │  │
│  │   /panel/          → 前端静态文件（Go embed）          │  │
│  │   /panel/api/      → 后端 REST API（:9999）            │  │
│  │   /panel/api/ws/   → WebSocket（:9999）                │  │
│  │   /panel/webhooks/ → Webhook 接收端（:9999）           │  │
│  └───────────────────────────────────────────────────────┘  │
│                           │                                  │
│  ┌───────────────────────────────────────────────────────┐  │
│  │           ServerHub 主程序（Go 单二进制）               │  │
│  │  REST API + WebSocket + 调度器 + SSH 连接池 + SQLite   │  │
│  └───────────────────────────────────────────────────────┘  │
└──────────────────────────┬──────────────────────────────────┘
                           │ SSH over WireGuard
                           ▼
                     内网服务器（10.0.0.x）
```

**优势：** 支持 Webhook，24/7 持续监控，多用户共用一个面板。

---

## 二、整体架构（两种模式共用核心）

---

## 三、后端目录结构

```
backend/
├── main.go                        # 入口：初始化 Gin、数据库、连接池、调度器
├── tray/
│   ├── tray_desktop.go            # +build desktop：systray 初始化、菜单、图标状态
│   ├── tray_server.go             # +build server：空实现
│   └── notify.go                  # 原生通知封装（beeep）
├── config/
│   └── config.go                  # 配置加载（YAML），含 allow_register 开关
├── middleware/
│   ├── auth.go                    # JWT 认证中间件（验证 user_id + exp）
│   ├── audit.go                   # 操作审计：写操作记录到 audit_logs
│   ├── ratelimit.go               # IP 限流（登录接口防暴力破解）
│   └── recover.go                 # Panic 恢复，返回 500
├── api/
│   ├── auth/                      # 登录 / 登出 / MFA
│   ├── server/                    # 服务器 CRUD + 指标
│   ├── terminal/                  # WebSocket SSH 终端
│   ├── website/                   # Nginx 站点管理
│   ├── ssl/                       # SSL 证书
│   ├── docker/                    # Docker 容器 / 镜像
│   ├── deploy/                    # 应用部署 + 模板
│   ├── webhook/                   # Webhook 接收端（GitHub/Gitlab push）
│   ├── database/                  # MySQL / Redis 管理
│   ├── files/                     # 文件管理（SFTP）
│   ├── system/                    # 防火墙 / Cron / 进程 / systemd
│   ├── notification/              # 通知渠道管理
│   └── health/                    # 健康检查（无需认证）
├── service/
│   ├── ssh_pool.go                # SSH 连接池（核心）
│   ├── metrics.go                 # 指标采集调度
│   ├── certbot.go                 # Let's Encrypt 证书
│   ├── nginx.go                   # Nginx 配置 / reload / test
│   ├── docker.go                  # Docker SDK 封装
│   ├── deploy.go                  # 部署执行引擎（nohup 模式）
│   ├── notification.go            # 通知分发：桌面模式→原生通知，服务器模式→Webhook
│   └── scheduler.go               # cron 调度器（续签/指标/告警）
├── model/
│   ├── server.go
│   ├── website.go
│   ├── certificate.go
│   ├── deploy_app.go
│   ├── deploy_history.go
│   ├── service_template.go
│   ├── cron_task.go
│   ├── alert_rule.go
│   ├── notification_channel.go
│   ├── alert_record.go
│   ├── audit_log.go
│   └── user.go
└── pkg/
    ├── ssh/                       # SSH 封装（命令执行 / PTY / 上传 / 下载）
    ├── executor/                  # 统一执行器接口
    ├── crypto/                    # AES-256-GCM 加密工具
    └── ws/                        # WebSocket 帮助函数
```

---

## 四、前端目录结构

```
frontend/src/
├── main.ts
├── types/                         # 全局 TypeScript 类型定义
│   ├── api.ts                     # API 响应类型
│   ├── server.ts
│   └── ...
├── router/
│   └── index.ts                   # 路由配置 + 导航守卫（JWT 检查）
├── stores/                        # Pinia stores
│   ├── auth.ts                    # 登录状态 / user_id / token
│   ├── servers.ts                 # 服务器列表 / 在线状态
│   └── notifications.ts           # 未读告警数
├── api/                           # Axios 封装（对应后端每个模块）
│   ├── request.ts                 # 拦截器：token 注入 + 统一错误处理
│   ├── auth.ts
│   ├── servers.ts
│   └── ...
├── composables/                   # 可复用逻辑
│   ├── useWebSocket.ts            # WS 连接（含重连逻辑）
│   ├── useMetrics.ts              # 实时指标订阅
│   └── useTerminal.ts             # Xterm.js 初始化
├── components/
│   ├── Terminal.vue               # Xterm.js 终端（可复用）
│   ├── MetricChart.vue            # ECharts 实时折线图
│   ├── FileEditor.vue             # CodeMirror 代码编辑器
│   ├── DeployPipeline.vue         # 部署流水线步骤视图
│   ├── EnvEditor.vue              # 环境变量 KV 编辑器
│   └── ServerCard.vue             # 服务器状态卡片
├── views/
│   ├── Login/
│   ├── Dashboard/                 # 概览：服务器卡片 + 告警
│   ├── Servers/
│   ├── Terminal/
│   ├── Websites/
│   ├── SSL/
│   ├── Docker/
│   ├── Deploy/
│   ├── Database/
│   ├── Files/
│   ├── System/
│   ├── Notifications/
│   └── Settings/
└── utils/
    ├── format.ts                  # bytes / duration / 时间格式化
    └── crypto.ts
```

---

## 五、SSH 连接池设计

```
┌──────────────────────────────────────────────────────────────┐
│                      SSH 连接池                               │
│                                                              │
│  map[serverID] → PoolEntry {                                 │
│      client    *ssh.Client                                   │
│      status    Connected | Reconnecting | Offline            │
│      lastSeen  time.Time                                     │
│      mu        sync.RWMutex                                  │
│  }                                                           │
│                                                              │
│  ① 短命令    → session.Run()（每次新 Session，复用 Client）   │
│  ② 交互终端  → session.RequestPty() + WebSocket 双向桥接     │
│  ③ 文件传输  → sftp.NewClient(sshClient)                     │
│  ④ 长命令    → RunBackground: nohup + tail -f 流式日志        │
│                                                              │
│  本机操作    → os/exec（不走 SSH）                            │
└──────────────────────────────────────────────────────────────┘
```

### 连接稳定性设计

**Keepalive（防 NAT 空闲断开）：**
```go
// 建立连接后启动 keepalive goroutine
go func() {
    t := time.NewTicker(30 * time.Second)
    for range t.C {
        _, _, err := client.SendRequest("keepalive@serverhub", true, nil)
        if err != nil {
            pool.triggerReconnect(serverID)
            return
        }
    }
}()
```

**断线自动重连（指数退避）：**
```
首次断线  → 1s 后重连
再次失败  → 2s → 4s → 8s → ... → max 30s
连续失败 3 次 → 标记 Offline → 触发告警通知
```

**健康检查分层：**
```
快速检查（每 30s）：TCP 端口 22 连通性
                   失败 → 立即标记 Offline
慢速确认（每 5min）：SSH 握手 + 执行 "echo ok"
                   验证认证仍然有效
```

### 统一执行器接口

```go
type Executor interface {
    Run(ctx context.Context, cmd string) (stdout, stderr string, err error)
    RunStream(ctx context.Context, cmd string, out io.Writer) error
    // 长命令后台化，返回 PID；日志写入 logFile
    RunBackground(ctx context.Context, cmd string, logFile string) (pid int, err error)
    OpenTerminal(ws *websocket.Conn, rows, cols int) error
    Upload(localPath, remotePath string) error
    Download(remotePath string) (io.ReadCloser, error)
}

// LocalExecutor  → 本机 os/exec
// RemoteExecutor → ssh.Session（复用 ssh.Client）
```

`RunBackground` 实现：
```bash
nohup sh -c '<cmd>' > /tmp/serverhub-deploy-<id>.log 2>&1 & echo $!
```

---

## 六、实时数据流设计

### 指标采集（每 5 秒）
```
调度器 → 遍历 Connected 服务器
      → Executor.Run(采集脚本)
      → 解析 JSON → 内存缓存（最近 60 点）
      → 每 5 分钟聚合写 SQLite（保留 24h）
      → 推送到订阅该服务器的 WebSocket 连接

前端订阅：
ws://panel/api/ws/metrics?serverIds=1,2,3
← { "serverId": 1, "cpu": 23.5, "mem": 65.2, ... }
```

### WebSocket 终端
```
浏览器 xterm.js ←── WSS ──→ 后端 WS Handler
                                    │
               输入（字节流）──→  SSH PTY stdin
               输出（字节流）←──  SSH PTY stdout
               resize（JSON）──→  Session.WindowChange()
```

### 部署日志流
```
前端 DeployPipeline ←── WSS ──→ 后端 WS Handler
                                      │
                RunStream("tail -f -n 0 <logFile>")
                    │ 单个长连接，实时输出每行日志
                    ├─ 普通行 ──→ WS 推送 {type:"log", line:"..."}
                    └─ [STAGE:xxx] ──→ WS 推送 {type:"stage", stage:"build"}
                部署进程结束时（kill -0 <pid> 失败）→ 推送 {type:"done"}
```

> **实现要点：** 使用 `executor.RunStream("tail -f -n 0 ...")` 而非周期性轮询。
> 单个长连接替代每 500ms 一次的短命令，日志实时延迟从 0-500ms 降至 <10ms，
> 且 SSH 命令数减少 60 倍。WebSocket 关闭时 cancel context 即终止 tail。

### 告警通知流（双模式）

```
调度器检测到告警触发
    │
    ├─ 桌面模式（build tag: desktop）
    │      └─ tray.Notify(title, body)
    │             → beeep → macOS 通知中心 / Linux libnotify / Windows Toast
    │             → systray 图标更新（🟠 或 🔴）
    │             → 托盘菜单实时刷新告警数
    │
    └─ 服务器模式（build tag: server）
           └─ notification.Send(channel, event)
                  → 企业微信 / 钉钉 / Telegram / 自定义 Webhook
                  → 写入 notification_history 表
```

---

## 七、性能关键配置

### SQLite WAL 模式（必须配置）

默认 SQLite 使用 rollback journal，写操作会阻塞所有读取。
面板启动时**必须**执行：

```go
// 初始化数据库后立即执行
db.Exec("PRAGMA journal_mode=WAL")       // 读写并发，消除读阻塞
db.Exec("PRAGMA synchronous=NORMAL")     // WAL 下安全降低 fsync 频率
db.Exec("PRAGMA cache_size=-32000")      // 32MB 页缓存
db.Exec("PRAGMA temp_store=MEMORY")      // 临时表存内存
```

不配置 WAL 的后果：审计日志写入时，前端列表接口会被阻塞 10-50ms。

### 文件下载必须流式传输

`/files/download` 和 `/files/read` 接口**禁止**将文件全量读入内存，
必须使用 `io.Copy` 流式传输：

```go
// 正确：SFTP → HTTP Response 直接流式，不缓冲
rc, err := remoteFS.Read(path)
defer rc.Close()
c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
io.Copy(c.Writer, rc)   // 内存消耗 = 单次 buffer（约 32KB），与文件大小无关

// 禁止：
data, _ := io.ReadAll(rc)   // 下载 1GB 文件 = 占用 1GB 内存
```

---

## 八、安全设计

| 方面 | 方案 |
|------|------|
| 面板认证 | JWT（HS256），7 天有效期，可选 TOTP 二次验证 |
| SSH 凭据存储 | AES-256-GCM 加密，密钥来自 config.yaml |
| 登录防暴力 | IP 限流：5 次失败 → 锁定 15 分钟 |
| 操作审计 | 所有 POST/PUT/DELETE：记录 userID / IP / 路径 / 请求体（脱敏） |
| Nginx 配置 | 保存前强制 `nginx -t`，失败自动恢复 .bak |
| 部署环境变量 | AES-256-GCM 加密存储，API 返回时脱敏 |
| HTTPS | OpenResty 反代处理 TLS |
| WebSocket 认证 | Query 参数传 Token：`?token=<jwt>` |
| Webhook 签名 | HMAC-SHA256 验证（GitHub X-Hub-Signature-256 规范） |

---

## 九、用户模型解耦设计

**原则：单用户模式下完整保留多用户数据结构，仅关闭注册入口。**

```
当前：allow_register: false  →  单 admin 用户
升级：allow_register: true   →  开放注册 + 用户管理页面
```

所有 API 处理层接收 `userID`，所有资源含 `created_by`，无需重构业务逻辑。

---

## 十、部署目录

```
/opt/serverhub/
├── serverhub               # Go 单二进制（含前端，go embed）
├── config.yaml             # 面板配置
├── data/
│   ├── serverhub.db        # SQLite
│   ├── ssh_keys/           # AES 加密存储的 SSH 私钥
│   ├── logs/               # 面板运行日志
│   └── deploy-logs/        # 部署任务日志（自动清理 30 天前）
└── acme/                   # Let's Encrypt webroot 目录
```

# API 接口设计

## 规范

```
Base URL:    /panel/api/v1
认证方式:    Header: Authorization: Bearer <jwt_token>
             或 WebSocket Query: ?token=<jwt_token>
响应格式:    { "code": 0, "msg": "success", "data": {...} }
错误格式:    { "code": <错误码>, "msg": "描述", "data": null }
分页格式:    { "code": 0, "data": { "list": [...], "total": 100, "page": 1, "size": 20 } }
```

**分页查询参数：** `?page=1&size=20`（默认 size=20，最大 100）

---

## 健康检查（无需认证）

```
GET    /health
```

响应示例：
```json
{
  "code": 0,
  "data": {
    "version": "1.0.0",
    "uptime": 3600,
    "db_status": "ok",
    "servers_connected": 3
  }
}
```

---

## 认证模块

```
POST   /auth/login              # 登录
       Body: { "username": "", "password": "" }
       返回: { "token": "<jwt>", "mfa_required": false }

POST   /auth/login/mfa          # TOTP 二次验证
       Body: { "token": "<pre_auth_token>", "code": "123456" }
       返回: { "token": "<final_jwt>" }

POST   /auth/logout             # 登出（客户端删除 token）
GET    /auth/info               # 当前用户信息
PUT    /auth/password           # 修改密码
PUT    /auth/mfa/enable         # 开启 TOTP（返回二维码）
DELETE /auth/mfa                # 关闭 TOTP
```

登录频率限制：同 IP 5 次失败 → 锁定 15 分钟，返回 `{ "code": 1004, "retry_after": 900 }`

---

## 服务器模块

```
GET    /servers                         # 服务器列表（含实时在线状态）
POST   /servers                         # 添加服务器
GET    /servers/:id                     # 服务器详情
PUT    /servers/:id                     # 修改服务器
DELETE /servers/:id                     # 删除服务器
POST   /servers/:id/test                # 测试 SSH 连接

GET    /servers/:id/metrics             # 当前指标（一次性）
GET    /servers/:id/metrics/history     # 历史指标（最近 24h）
                                        # Query: ?interval=5m（聚合间隔）
```

**WebSocket — 实时指标（多服务器订阅）：**
```
WS  /ws/metrics?serverIds=1,2,3&token=<jwt>

← 推送消息：
{
  "serverId": 1,
  "cpu": 23.5,
  "mem": { "used": 2048000000, "total": 8192000000, "percent": 25.0 },
  "disk": { "used": 10000000000, "total": 100000000000, "percent": 10.0 },
  "net": { "bytes_sent": 1024, "bytes_recv": 2048 },
  "load": [0.5, 0.3, 0.2],
  "uptime": 86400,
  "timestamp": 1713312000
}

← 心跳（每 30s）：{ "type": "ping" }
→ 客户端回复：{ "type": "pong" }
```

---

## Web 终端

```
WS  /ws/terminal?serverId=1&rows=24&cols=80&token=<jwt>

→ 输入：原始字节流（键盘输入）
→ Resize：{ "type": "resize", "rows": 50, "cols": 220 }
← 输出：原始字节流（终端输出）
← 心跳：{ "type": "ping" } / → { "type": "pong" }
```

---

## 网站管理

```
GET    /websites                        # 站点列表（分页）
POST   /websites                        # 创建站点（向导完成后提交）
GET    /websites/:id                    # 站点详情
PUT    /websites/:id                    # 修改站点基本配置
DELETE /websites/:id                    # 删除站点（含 nginx 配置）

GET    /websites/:id/config             # 获取 nginx 配置原文
PUT    /websites/:id/config             # 保存配置（含 nginx -t 验证）
       Body: { "content": "server {...}" }
       错误: { "code": 2003, "msg": "nginx -t 错误详情" }
POST   /websites/:id/reload             # nginx reload
GET    /websites/:id/log                # 日志内容
       Query: ?type=access|error&lines=100
```

**WebSocket — 实时日志：**
```
WS  /ws/websites/:id/log?type=access&token=<jwt>
← 日志行（字符串，含换行）
```

---

## SSL 证书

```
GET    /ssl/certs                       # 证书列表（含到期天数）
POST   /ssl/certs/apply                 # 申请 Let's Encrypt
       Body: { "domain": "a.com", "email": "x@x.com", "serverId": 1 }
POST   /ssl/certs/upload                # 上传自有证书
       Body: { "domain": "a.com", "cert": "...", "key": "...", "serverId": 1 }
PUT    /ssl/certs/:id/renew             # 手动续期
DELETE /ssl/certs/:id                   # 删除证书
```

**SSE — 申请进度：**
```
GET  /ssl/certs/apply/progress/:taskId
Content-Type: text/event-stream
← data: { "step": "dns_check", "status": "ok" }
← data: { "step": "certbot", "status": "running", "output": "..." }
← data: { "step": "done", "status": "ok", "expire_at": "2026-04-17" }
```

---

## Docker 容器管理

```
GET    /docker/containers               # 容器列表
       Query: ?serverId=1
POST   /docker/containers/:id/start    # 启动（Query: ?serverId=1）
POST   /docker/containers/:id/stop     # 停止
POST   /docker/containers/:id/restart  # 重启
DELETE /docker/containers/:id          # 删除（Query: ?serverId=1）
GET    /docker/containers/:id/inspect  # 详情（环境变量脱敏）
GET    /docker/containers/:id/env      # 查看环境变量明文（需二次确认）

GET    /docker/images                  # 镜像列表（Query: ?serverId=1）
POST   /docker/images/pull             # 拉取镜像
DELETE /docker/images/:id              # 删除镜像
```

**WebSocket — 容器实时日志：**
```
WS  /ws/docker/logs?serverId=1&containerId=abc&tail=100&token=<jwt>
← 日志行
```

---

## 应用部署

```
GET    /deploy/apps                     # 部署应用列表
POST   /deploy/apps                     # 创建部署应用
GET    /deploy/apps/:id                 # 应用详情
PUT    /deploy/apps/:id                 # 修改配置
DELETE /deploy/apps/:id                 # 删除

POST   /deploy/apps/:id/deploy          # 触发部署（返回 deployId）
       Body: { "branch": "main" }（可选，覆盖默认分支）
POST   /deploy/apps/:id/rollback        # 回滚（到上一个成功版本）
GET    /deploy/apps/:id/history         # 部署历史（分页）

GET    /deploy/templates                # 服务模板列表
       Query: ?category=database|storage|monitoring
POST   /deploy/apps/from-template       # 从模板创建部署

GET    /deploy/apps/:id/env             # 环境变量列表（值脱敏）
PUT    /deploy/apps/:id/env             # 更新环境变量
```

**WebSocket — 部署实时日志：**
```
WS  /ws/deploy/:deployId/logs?token=<jwt>
← { "type": "log", "line": "..." }
← { "type": "stage", "stage": "build", "status": "running" }
← { "type": "done", "status": "success"|"failed" }
```

---

## Webhook 接收端（Push-to-Deploy）

```
POST   /webhooks/github/:appId          # GitHub push 事件
POST   /webhooks/gitlab/:appId          # GitLab push 事件
POST   /webhooks/gitea/:appId           # Gitea / Gitee push 事件
```

验证方式：
- GitHub：`X-Hub-Signature-256: sha256=<HMAC>`
- GitLab：`X-Gitlab-Token: <token>`
- Gitea：`X-Gitea-Signature: <HMAC>`

响应：`202 Accepted`（不等待部署完成）

---

## 文件管理

```
GET    /files/list                      # 目录列表
       Query: ?serverId=1&path=/var/www
GET    /files/read                      # 读取文件内容（文本，最大 2MB）
       Query: ?serverId=1&path=/etc/nginx/nginx.conf
POST   /files/write                     # 写入文件
       Body: { "serverId": 1, "path": "...", "content": "..." }
POST   /files/upload                    # 上传文件（multipart/form-data）
GET    /files/download                  # 下载文件（流式）
       Query: ?serverId=1&path=...
POST   /files/mkdir                     # 创建目录
POST   /files/delete                    # 删除文件/目录（二次确认由前端完成）
POST   /files/rename                    # 重命名/移动
POST   /files/chmod                     # 修改权限
       Body: { "serverId": 1, "path": "...", "mode": "0644" }
POST   /files/compress                  # 压缩
       Body: { "serverId": 1, "paths": [...], "dest": "..." }
POST   /files/extract                   # 解压
       Body: { "serverId": 1, "src": "...", "dest": "..." }
```

---

## 数据库管理

```
GET    /databases                       # 连接列表
POST   /databases                       # 添加连接
DELETE /databases/:id                   # 删除连接
POST   /databases/:id/test              # 测试连接

# MySQL
GET    /databases/:id/dbs              # 数据库列表
POST   /databases/:id/dbs              # 建库
DELETE /databases/:id/dbs/:dbname      # 删库

GET    /databases/:id/users            # 用户列表
POST   /databases/:id/users            # 建用户并授权

POST   /databases/:id/query            # 执行 SQL
       Body: { "db": "mydb", "sql": "SELECT 1", "write_mode": false }

POST   /databases/:id/export           # 导出（流式下载）
       Body: { "db": "mydb" }

# Redis
GET    /databases/:id/redis/info       # Redis INFO
GET    /databases/:id/redis/keys       # Key 浏览
       Query: ?pattern=user:*&cursor=0&count=100
POST   /databases/:id/redis/flushdb    # Flushdb（需 Body: { "confirm": true }）
```

---

## 系统工具

```
# 防火墙
GET    /system/firewall/rules          # 规则列表
       Query: ?serverId=1
POST   /system/firewall/rules          # 添加规则
DELETE /system/firewall/rules/:id      # 删除规则

# 计划任务
GET    /system/cron                    # 任务列表（Query: ?serverId=1）
POST   /system/cron                    # 添加任务
PUT    /system/cron/:id                # 修改任务
DELETE /system/cron/:id                # 删除任务

# systemd 服务
GET    /system/services                # 服务列表（Query: ?serverId=1）
POST   /system/services/:name/:action  # 操作（start/stop/restart/enable/disable）
       Query: ?serverId=1
GET    /system/services/:name/logs     # 服务日志（Query: ?serverId=1&lines=100）

# 进程
GET    /system/processes               # 进程列表 Top 20 by CPU（Query: ?serverId=1）
POST   /system/processes/:pid/kill     # Kill 进程
       Body: { "serverId": 1, "signal": 9 }
```

---

## 通知模块

```
GET    /notifications/channels         # 渠道列表
POST   /notifications/channels         # 添加渠道
PUT    /notifications/channels/:id     # 修改渠道
DELETE /notifications/channels/:id     # 删除渠道
POST   /notifications/channels/:id/test # 发送测试消息

GET    /notifications/rules            # 告警路由规则列表
POST   /notifications/rules            # 添加路由规则
PUT    /notifications/rules/:id        # 修改
DELETE /notifications/rules/:id        # 删除

GET    /notifications/history          # 通知发送历史（分页）
```

---

## 面板设置

```
GET    /settings                       # 获取所有配置
PUT    /settings                       # 批量更新配置
       Body: { "panel_name": "...", "alert_cpu_threshold": 90, ... }

GET    /settings/audit                 # 审计日志（分页）
       Query: ?page=1&size=20&user=admin
```

---

## 错误码

```
0     成功
1001  未登录 / Token 无效 / Token 已过期
1002  无权限
1003  参数错误（含字段详情在 msg 中）
1004  登录频率限制（含 retry_after 秒数）
1005  操作需要 MFA 验证
2001  SSH 连接失败（含原因）
2002  命令执行失败（含 stderr）
2003  nginx -t 验证失败（含错误输出）
2004  SFTP 操作失败
3001  证书申请失败（含 certbot 输出）
3002  域名 DNS 未解析到本机
3003  Webhook 签名验证失败
4001  数据库连接失败
4002  SQL 执行失败（含错误信息）
4003  只读模式，禁止写操作
5001  文件不存在
5002  文件过大（超过编辑限制 2MB）
5003  非文本文件，无法在线编辑
```

# API 参考

所有路由前缀：`/panel/api/v1`。

## 通用约定

- 鉴权：`Authorization: Bearer <jwt>`，WS 鉴权用 `?token=<jwt>` query
- 请求体：`Content-Type: application/json`
- 响应封装：

```json
{ "code": 0, "msg": "ok", "data": <payload> }
```

- 失败响应：HTTP 状态码 + `code` 业务码 + `msg`。常见：
  - `400 / 4000` 参数错误
  - `401` 未授权 / token 过期
  - `404 / 4040` 资源不存在
  - `429 / 4290` 并发已满（日志搜索）
  - `503 / 5003` 远端 SSH 不可达
  - `500 / 5000` 内部错误

## 公开路由

| Method | Path | 说明 |
|---|---|---|
| GET | `/health` | 健康检查 |
| POST | `/auth/login` | 登录（支持 MFA 第二步） |
| POST | `/auth/logout` | 登出 |
| POST | `/webhooks/deploy/:id` | 触发部署（带 `secret` 校验） |

## 鉴权后路由（需 JWT）

### 用户

| Method | Path |
|---|---|
| GET | `/auth/me` |
| POST | `/auth/mfa/setup` |
| POST | `/auth/mfa/verify` |
| POST | `/auth/password` |

### 服务器 `/servers`

| Method | Path | 说明 |
|---|---|---|
| GET | `/servers` | 列表 |
| POST | `/servers` | 新增 |
| GET | `/servers/:id` | 详情 |
| PUT | `/servers/:id` | 编辑 |
| DELETE | `/servers/:id` | 删除 |
| POST | `/servers/:id/test` | 连通性测试 |
| POST | `/servers/:id/collect` | 即时采集 |

### Docker（远端）

`/servers/:id/docker/...`：containers GET、`/containers/:cid/action` POST、`/containers/:cid/logs` WS、`/containers/:cid/inspect` GET、images GET、`/images/pull` WS、`/images/:iid` DELETE。

### 文件 `/servers/:id/files`

`GET /list?path=` `POST /upload` `GET /download?path=` `DELETE` `POST /rename`。

### 系统 `/servers/:id/system`

防火墙：`/firewall/rules` GET/POST/DELETE
Cron：`/cron/jobs` GET/POST/PUT/DELETE
进程：`/processes` GET、`/processes/:pid` DELETE
服务：`/services` GET、`/services/:name/action` POST、`/services/:name/logs` WS

ServerHub 自身指标：`GET /system/self`

### Nginx `/servers/:id/nginx`

`/sites` GET/POST、`/sites/:name/config` GET/PUT、`/sites/:name` DELETE、`/sites/:name/enable|disable` POST、`/reload` `/restart` POST、`/logs/access` `/logs/error` WS。

### SSL `/servers/:id/ssl`

`/certs` GET、`/certs/request` WS（Let's Encrypt）、`/certs/upload` POST、`/certs/:cid/renew` WS、`/certs/:cid` DELETE、`/certs/scan` POST。

### 日志搜索 `/servers/:id/logs/search`

```
POST /servers/:id/logs/search
{
  "source": "docker | journalctl | nginx-access | nginx-error",
  "target": "<container | service.unit>",
  "query":  "ERROR",
  "regex":  false,
  "case_sensitive": false,
  "since":  "1h",       // 30m|1h|2h|6h|1d|2d|7d，仅 docker/journalctl
  "context": 0,         // 前后行数 0~10
  "limit":   500        // 最大命中行
}
→ { lines: [{raw}], truncated: bool, error?: string }
```

并发上限 8，超出返回 `429`。target 与 since 走白名单，命令注入零风险。

### 应用 `/apps`

`GET /apps` `POST /apps` `GET /apps/:id` `PUT /apps/:id` `DELETE /apps/:id`
路由：`GET|POST /apps/:id/routes`、`PUT|DELETE /apps/:id/routes/:rid`
应用日志/部署/数据库等 Tab 复用 servers/deploys/database 接口。

### 部署 `/deploys`

CRUD + `POST /deploys/:id/run`（手动触发）+ `GET /deploys/:id/logs/stream` WS + `GET /deploys/:id/logs` 列表查询。

### 数据库 `/database`

DBConn CRUD + `POST /database/:id/test`。

### 告警 `/alerts`

`/alerts/rules` CRUD、`/alerts/channels` CRUD、`/alerts/events` 查询。

### 指标 `/metrics`

`GET /metrics?server_id=&from=&to=&limit=` 返回时序点。

### 审计 `/audit`

`GET /audit?username=&path=&from=&to=&page=&size=`。`username`/`path` 为前缀匹配（走索引）。

### 设置 `/settings`

`GET /settings/:key` `PUT /settings/:key`。

### 终端

`WS /servers/:id/terminal?token=<jwt>` —— xterm.js 协议（resize / 输入 / 输出）。

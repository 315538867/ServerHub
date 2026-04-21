# 数据模型

引擎：SQLite（WAL，`synchronous=NORMAL`）。模型在 `backend/model/`，由 `database.Init` AutoMigrate；非默认索引在 `database.ensureIndexes` 中建。

## 表汇总

| 模型 | 表名 | 说明 |
|---|---|---|
| `User` | `users` | 面板用户 |
| `Server` | `servers` | 受管远端服务器 |
| `Metric` | `metrics` | 时序指标点 |
| `Application` | `applications` | 业务应用 |
| `AppNginxRoute` | `app_nginx_routes` | 应用反代路由 |
| `Deploy` | `deploys` | 部署配置 |
| `DeployLog` | `deploy_logs` | 部署执行日志 |
| `DBConn` | `db_conns` | 数据库连接 |
| `SSLCert` | `ssl_certs` | SSL 证书清单 |
| `AlertRule` | `alert_rules` | 告警规则 |
| `AlertEvent` | `alert_events` | 告警事件 |
| `NotifyChannel` | `notify_channels` | 通知渠道 |
| `AuditLog` | `audit_logs` | 操作审计 |
| `Setting` | `settings` | KV 系统设置 |

## 关键字段

### users
`id, username (UQ), password (bcrypt), role(admin), mfa_secret, mfa_enabled, last_login, last_ip, created_at, updated_at`

### servers
`id, name, host, port(默认22), username, auth_type(password|key), password (AES), private_key (AES), status(online|offline|unknown), last_check_at, remark, created_at, updated_at`

### metrics
`id, server_id(idx), cpu, mem, disk, load1, uptime, created_at`
索引：`(server_id, created_at DESC)`

### deploys
`id, name, server_id, type(docker|docker-compose|native), work_dir, env_vars (AES), webhook_secret, desired_version, actual_version, auto_sync, sync_status, created_at`

### deploy_logs
`id, deploy_id(idx), output(text), status(success|failed), duration(s), trigger_source(manual|webhook|schedule|api), created_at`

### applications
`id, name(UQ), server_id, domain, container_name, deploy_id, db_conn_id, expose_mode(path|domain), status, created_at`

### app_nginx_routes
`id, app_id, path, upstream, extra(text), sort`

### db_conns
`id, server_id, name, type(mysql|redis), host, port, username, password (AES), database`

### ssl_certs
`id, server_id, domain (UQ within server), cert_path, key_path, issuer, expires_at, auto_renew`

### alert_rules
`id, name, metric(cpu|mem|disk|offline), operator(gt|lt|eq), threshold, duration(s), enabled, channel_ids(json)`

### alert_events
`id, rule_id, server_id, value, message, status(firing|resolved), created_at, resolved_at`

### notify_channels
`id, name, type(webhook_wechat|dingtalk|telegram|custom), url (AES), enabled`

### audit_logs
`id, user_id, username, ip, method, path, body(text), status, duration_ms, created_at`
索引：
- `created_at DESC`
- `username`
- `path`（前缀匹配查询）

### settings
`key (PK), value, updated_at`
默认行：`deploy_log_keep_days=30`

## 加密字段

下列字段写入前经 `pkg/crypto.Encrypt`（AES-256-GCM，密钥来自 `security.aes_key`），读取时解密：

- `servers.password`、`servers.private_key`
- `deploys.env_vars`
- `db_conns.password`
- `notify_channels.url`

## 保留策略

由 `pkg/retention` 每日 02:00 执行：

| 表 | 保留窗口 |
|---|---|
| `audit_logs` | 90 天 |
| `metrics` | 30 天 |
| `deploy_logs` | `settings.deploy_log_keep_days`（默认 30） |

每月 1 号执行 `VACUUM` 收缩文件。

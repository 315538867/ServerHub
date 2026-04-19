# 数据库设计（SQLite）

所有表使用 SQLite，通过 GORM 管理迁移。
字段命名：snake_case，时间字段统一使用 `DATETIME DEFAULT CURRENT_TIMESTAMP`。

---

## servers — 被管服务器

```sql
CREATE TABLE servers (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    name         TEXT NOT NULL,
    host         TEXT NOT NULL,              -- IP 或域名
    port         INTEGER NOT NULL DEFAULT 22,
    user         TEXT NOT NULL,
    auth_type    TEXT NOT NULL,              -- 'key' | 'password'
    auth_data    TEXT NOT NULL,              -- AES-256-GCM 加密后的 Key 或密码
    description  TEXT DEFAULT '',
    status       TEXT DEFAULT 'unknown',     -- 'online' | 'offline' | 'unknown'
    last_seen    DATETIME,
    created_by   INTEGER REFERENCES users(id),
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_servers_status ON servers(status);
```

---

## server_metrics — 历史指标（每 5 分钟聚合）

```sql
CREATE TABLE server_metrics (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id    INTEGER NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    cpu_percent  REAL,
    mem_used     INTEGER,                    -- bytes
    mem_total    INTEGER,                    -- bytes
    disk_used    INTEGER,                    -- bytes
    disk_total   INTEGER,                    -- bytes
    net_sent     INTEGER,                    -- bytes 累计
    net_recv     INTEGER,                    -- bytes 累计
    load1        REAL,
    collected_at DATETIME NOT NULL
);
CREATE INDEX idx_metrics_server_time ON server_metrics(server_id, collected_at);
-- 自动清理超过 24 小时的数据（由调度器定时执行）
```

---

## websites — nginx 站点

```sql
CREATE TABLE websites (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id    INTEGER NOT NULL REFERENCES servers(id),
    name         TEXT NOT NULL,              -- 站点名称（内部标识）
    domain       TEXT NOT NULL,              -- 主域名
    domains      TEXT DEFAULT '[]',          -- 全部域名 JSON 数组
    type         TEXT NOT NULL,              -- 'static' | 'proxy' | 'php'
    root         TEXT DEFAULT '',            -- 静态站点根目录
    upstream     TEXT DEFAULT '',            -- 反代上游地址
    php_version  TEXT DEFAULT '',            -- PHP 版本
    nginx_conf   TEXT DEFAULT '',            -- nginx 配置文件路径
    ssl_cert_id  INTEGER REFERENCES certificates(id),
    status       TEXT DEFAULT 'active',      -- 'active' | 'disabled'
    created_by   INTEGER REFERENCES users(id),
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

---

## certificates — SSL 证书

```sql
CREATE TABLE certificates (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id    INTEGER NOT NULL REFERENCES servers(id),
    domain       TEXT NOT NULL,
    cert_path    TEXT NOT NULL,              -- 证书文件路径（目标服务器上）
    key_path     TEXT NOT NULL,              -- 私钥文件路径
    issuer       TEXT DEFAULT '',            -- 颁发机构（CN）
    expire_at    DATETIME NOT NULL,
    auto_renew   BOOLEAN DEFAULT TRUE,
    source       TEXT DEFAULT 'letsencrypt', -- 'letsencrypt' | 'manual'
    created_by   INTEGER REFERENCES users(id),
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_certs_expire ON certificates(expire_at);
```

---

## service_templates — 服务模板库

```sql
CREATE TABLE service_templates (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    name         TEXT NOT NULL,              -- 显示名称，如 "PostgreSQL 16"
    slug         TEXT UNIQUE NOT NULL,       -- 标识符，如 "postgres16"
    category     TEXT NOT NULL,              -- 'database' | 'storage' | 'monitoring' | 'web'
    description  TEXT DEFAULT '',
    icon         TEXT DEFAULT '',            -- 图标名（Element Plus icon 或 URL）
    compose      TEXT NOT NULL,              -- Docker Compose YAML 内容
    variables    TEXT DEFAULT '[]',          -- JSON：[{key, label, default, required, sensitive}]
    sort_order   INTEGER DEFAULT 0,
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

---

## deploy_apps — 部署应用配置

```sql
CREATE TABLE deploy_apps (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT NOT NULL,
    server_id       INTEGER NOT NULL REFERENCES servers(id),
    deploy_type     TEXT NOT NULL,           -- 'git' | 'compose' | 'template'
    template_id     INTEGER REFERENCES service_templates(id),
    git_url         TEXT DEFAULT '',
    git_branch      TEXT DEFAULT 'main',
    work_dir        TEXT DEFAULT '',         -- 目标服务器上的工作目录
    build_cmd       TEXT DEFAULT '',         -- 构建命令
    start_cmd       TEXT DEFAULT '',         -- 启动命令（可选，默认 docker compose up -d）
    webhook_secret  TEXT DEFAULT '',         -- Push-to-Deploy HMAC 密钥
    auto_deploy     BOOLEAN DEFAULT FALSE,   -- 是否自动部署（webhook 触发）
    created_by      INTEGER REFERENCES users(id),
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

---

## deploy_env_vars — 部署环境变量

```sql
CREATE TABLE deploy_env_vars (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    app_id       INTEGER NOT NULL REFERENCES deploy_apps(id) ON DELETE CASCADE,
    key          TEXT NOT NULL,
    value        TEXT NOT NULL,              -- AES-256-GCM 加密存储
    sensitive    BOOLEAN DEFAULT FALSE,      -- 是否脱敏显示
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(app_id, key)
);
```

---

## deploy_history — 部署历史

```sql
CREATE TABLE deploy_history (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    app_id       INTEGER NOT NULL REFERENCES deploy_apps(id),
    deploy_id    TEXT UNIQUE NOT NULL,       -- UUID，用于关联日志文件
    trigger      TEXT DEFAULT 'manual',      -- 'manual' | 'webhook' | 'auto'
    commit_hash  TEXT DEFAULT '',
    commit_msg   TEXT DEFAULT '',
    branch       TEXT DEFAULT '',
    status       TEXT NOT NULL,              -- 'running' | 'success' | 'failed'
    started_at   DATETIME NOT NULL,
    finished_at  DATETIME,
    duration_ms  INTEGER,                    -- 耗时（毫秒）
    log_path     TEXT DEFAULT '',            -- 日志文件路径
    error_msg    TEXT DEFAULT '',
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_deploy_history_app ON deploy_history(app_id, started_at DESC);
```

---

## database_conns — 数据库连接配置

```sql
CREATE TABLE database_conns (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    name         TEXT NOT NULL,
    server_id    INTEGER REFERENCES servers(id), -- NULL 表示本机
    db_type      TEXT NOT NULL,              -- 'mysql' | 'redis'
    host         TEXT NOT NULL DEFAULT '127.0.0.1',
    port         INTEGER NOT NULL,
    user         TEXT DEFAULT '',
    password     TEXT DEFAULT '',            -- AES 加密
    created_by   INTEGER REFERENCES users(id),
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

---

## cron_tasks — 计划任务

```sql
CREATE TABLE cron_tasks (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id    INTEGER NOT NULL REFERENCES servers(id),
    name         TEXT NOT NULL,              -- 任务描述（内部标识）
    cron_expr    TEXT NOT NULL,              -- crontab 表达式，如 "0 2 * * *"
    command      TEXT NOT NULL,
    enabled      BOOLEAN DEFAULT TRUE,
    last_run     DATETIME,
    last_status  TEXT DEFAULT '',            -- 'success' | 'failed'
    created_by   INTEGER REFERENCES users(id),
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

---

## alert_rules — 告警规则

```sql
CREATE TABLE alert_rules (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id    INTEGER REFERENCES servers(id), -- NULL 表示全局规则
    metric       TEXT NOT NULL,              -- 'cpu' | 'mem' | 'disk' | 'offline' | 'ssl_expiry'
    operator     TEXT NOT NULL DEFAULT '>',  -- '>' | '<' | '>='
    threshold    REAL NOT NULL,              -- 阈值（百分比或天数）
    duration_min INTEGER DEFAULT 0,          -- 持续时间（分钟）才触发，0 表示立即
    enabled      BOOLEAN DEFAULT TRUE,
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

---

## alert_records — 告警记录

```sql
CREATE TABLE alert_records (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id      INTEGER NOT NULL REFERENCES alert_rules(id),
    server_id    INTEGER REFERENCES servers(id),
    metric       TEXT NOT NULL,
    value        REAL NOT NULL,              -- 触发时的实际值
    threshold    REAL NOT NULL,
    message      TEXT NOT NULL,
    notified     BOOLEAN DEFAULT FALSE,      -- 是否已发送通知
    resolved_at  DATETIME,                   -- 恢复时间（NULL 表示未恢复）
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_alert_records_time ON alert_records(created_at DESC);
```

---

## notification_channels — 通知渠道

```sql
CREATE TABLE notification_channels (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    name         TEXT NOT NULL,              -- 渠道名称（显示用）
    type         TEXT NOT NULL,              -- 'wechat' | 'dingtalk' | 'telegram' | 'webhook'
    config       TEXT NOT NULL DEFAULT '{}', -- JSON：各渠道特有配置（URL/Token/密钥等，加密存储）
    template     TEXT DEFAULT '',            -- 消息模板（Go template 语法）
    enabled      BOOLEAN DEFAULT TRUE,
    created_by   INTEGER REFERENCES users(id),
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

---

## notification_rules — 告警路由规则

```sql
CREATE TABLE notification_rules (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_id   INTEGER NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    event_type   TEXT NOT NULL,              -- 'cpu_alert' | 'mem_alert' | 'disk_alert' |
                                             -- 'server_offline' | 'ssl_expiry' | 'deploy_failed'
    server_id    INTEGER REFERENCES servers(id), -- NULL 表示所有服务器
    enabled      BOOLEAN DEFAULT TRUE,
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

---

## notification_history — 通知发送历史

```sql
CREATE TABLE notification_history (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_id   INTEGER NOT NULL REFERENCES notification_channels(id),
    alert_id     INTEGER REFERENCES alert_records(id),
    event_type   TEXT NOT NULL,
    message      TEXT NOT NULL,
    status       TEXT NOT NULL,              -- 'success' | 'failed'
    error        TEXT DEFAULT '',
    sent_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_notif_history_time ON notification_history(sent_at DESC);
```

---

## users — 面板用户

```sql
CREATE TABLE users (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    username     TEXT UNIQUE NOT NULL,
    password     TEXT NOT NULL,              -- bcrypt hash（cost=12）
    role         TEXT DEFAULT 'admin',       -- 'admin'（当前仅此一种）
    mfa_secret   TEXT DEFAULT '',            -- TOTP 密钥（AES 加密）
    mfa_enabled  BOOLEAN DEFAULT FALSE,
    last_login   DATETIME,
    last_ip      TEXT DEFAULT '',
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);
-- 初始化时插入默认 admin 用户（密码由 `serverhub init` 设置）
```

---

## audit_logs — 操作审计

```sql
CREATE TABLE audit_logs (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id      INTEGER REFERENCES users(id),
    username     TEXT NOT NULL,
    ip           TEXT NOT NULL,
    method       TEXT NOT NULL,              -- HTTP 方法
    path         TEXT NOT NULL,              -- API 路径
    body         TEXT DEFAULT '',            -- 请求体（脱敏处理）
    status       INTEGER NOT NULL,           -- HTTP 状态码
    duration_ms  INTEGER DEFAULT 0,
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_audit_logs_time ON audit_logs(created_at DESC);
CREATE INDEX idx_audit_logs_user ON audit_logs(user_id, created_at DESC);
```

---

## settings — 面板配置

```sql
CREATE TABLE settings (
    key          TEXT PRIMARY KEY,
    value        TEXT NOT NULL,
    updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO settings VALUES
('panel_name',            'ServerHub',       CURRENT_TIMESTAMP),
('allow_register',        'false',           CURRENT_TIMESTAMP),
('alert_cpu_threshold',   '90',              CURRENT_TIMESTAMP),
('alert_mem_threshold',   '85',              CURRENT_TIMESTAMP),
('alert_disk_threshold',  '80',              CURRENT_TIMESTAMP),
('alert_ssl_days',        '30',              CURRENT_TIMESTAMP),
('cert_renew_days',       '30',              CURRENT_TIMESTAMP),
('metrics_interval',      '5',               CURRENT_TIMESTAMP),
('alert_cooldown_min',    '30',              CURRENT_TIMESTAMP),
('deploy_log_keep_days',  '30',              CURRENT_TIMESTAMP),
('timezone',              'Asia/Shanghai',   CURRENT_TIMESTAMP);
```

---

## 索引汇总

```sql
-- 高频查询索引
CREATE INDEX idx_servers_status       ON servers(status);
CREATE INDEX idx_metrics_server_time  ON server_metrics(server_id, collected_at);
CREATE INDEX idx_certs_expire         ON certificates(expire_at);
CREATE INDEX idx_deploy_history_app   ON deploy_history(app_id, started_at DESC);
CREATE INDEX idx_alert_records_time   ON alert_records(created_at DESC);
CREATE INDEX idx_notif_history_time   ON notification_history(sent_at DESC);
CREATE INDEX idx_audit_logs_time      ON audit_logs(created_at DESC);
CREATE INDEX idx_audit_logs_user      ON audit_logs(user_id, created_at DESC);
```

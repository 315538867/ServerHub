# ServerHub 交互架构重设计

> 创建时间: 2026-04-19
> 状态: 已确认方案，待实施

## 一、问题诊断

### 1.1 现有架构：工具优先（Tool-first）

当前侧边栏是 12 个平级工具入口：

```
概览 | 服务器 | 终端 | 网站 | SSL | Docker | 部署 | 数据库 | 文件 | 系统 | 通知 | 设置
```

每个页面顶部有独立的 `[选择服务器 ▼]` 下拉框，页面间不共享服务器上下文。

### 1.2 核心痛点

**场景：凌晨 3 点收到告警 "my-blog 502"，排障路径：**

1. 概览 → 找 prod-01 → 看指标
2. Docker → **重新选 prod-01** → 找容器 → 看日志
3. 终端 → **又选 prod-01** → 连上手动查
4. 网站 → **又选 prod-01** → 看 Nginx 错误日志
5. 文件 → **又选 prod-01** → 看应用日志
6. 部署 → 找 my-blog → 回滚

**6 次页面跳转，5 次重复选择同一台服务器，无法聚焦"my-blog 这个应用怎么了"。**

### 1.3 根因

工具优先视角按**工具类型**切割信息，而运维者的思维是按**对象**聚合的。运维者有两种思维模式：

| 场景 | 思维方式 | 问的问题 |
|---|---|---|
| "博客 502 了" | **应用视角** | "my-blog 所有组件哪个出问题？" |
| "服务器 CPU 100%" | **服务器视角** | "prod-01 上什么在吃资源？" |
| "SSL 快过期了" | **应用视角** | "my-blog 证书什么时候续？" |
| "加防火墙规则" | **服务器视角** | "prod-01 端口怎么配？" |

---

## 二、真实运维模型

### 2.1 物理架构

```
一台服务器
  └─ 一个 Nginx（总网关，唯一入口）
       ├─ blog.example.com → proxy_pass :3000 (Docker 容器)
       ├─ api.example.com  → proxy_pass :8080 (Docker 容器)
       └─ admin.example.com → /var/www/admin  (静态文件)
```

- 每台服务器只有一个 Nginx 实例作为流量总入口
- Nginx 根据域名将请求转发给不同的后端服务
- 后端服务通常是 Docker 容器

### 2.2 逻辑架构

一个"应用"是一个业务单元，可能跨服务器组合资源：

```
应用: my-blog
  ├─ 域名入口: blog.example.com (Nginx on prod-01)
  ├─ SSL 证书: Let's Encrypt (prod-01)
  ├─ 后端服务: Docker 容器 my-blog-web (prod-01:3000)
  ├─ 数据库: myblog_db MySQL (db-01)        ← 可能跨服务器
  ├─ 部署配置: docker-compose + webhook
  └─ 环境变量
```

### 2.3 三种运维视角

1. **应用视角**（逻辑聚合）："我的博客怎么了？" → 聚合跨服务器的所有关联资源
2. **服务器视角**（物理聚合）："这台机器怎么了？" → 查看单机上的所有资源
3. **任务视角**（运维工作台）："今天要干什么？" → 待处理事项聚合

---

## 三、目标架构：双视角统一

### 3.1 信息架构

```
工作台（Dashboard）
  ├─ 待处理事项（SSL到期、版本漂移、服务器离线、部署失败）
  ├─ 服务器 + 应用概览
  └─ 最近活动流

应用（Applications）— 业务逻辑聚合，一等公民
  └─ my-blog
      ├─ 概览（状态聚合 + 快捷操作 + 部署历史）
      ├─ 域名（Nginx 配置 + SSL 证书管理）
      ├─ 服务（关联 Docker 容器管理）
      ├─ 部署（版本管理 + 同步 + 回滚 + Webhook）
      ├─ 日志（Nginx 日志 + 容器日志聚合）
      ├─ 数据库（关联 DB 实例管理）
      └─ 环境变量

服务器（Servers）— 基础设施层
  └─ prod-01
      ├─ 概览（指标 + 承载应用列表）
      ├─ Nginx 网关（路由表 + 全局日志 + reload/restart）
      ├─ Docker（全部容器，标注应用归属）
      ├─ 系统（防火墙 / Cron / 进程 / systemd）
      ├─ 文件
      └─ 终端

通知 & 告警（全局）
设置（全局）
```

### 3.2 核心原则

1. **应用和服务器是平级的两个导航入口**，看的是同一组底层数据的不同切面
2. **双向穿透**：应用页面可点击跳转到服务器，服务器页面可点击跳转到应用
3. **Application 是聚合层**，不替换底层模型（Site、Container、Deploy、DBConn 仍独立存在）
4. **Nginx 是服务器级别的网关概念**，每台服务器一个，下面挂多个站点路由
5. **没有任何页面需要"选择服务器"下拉框** — 服务器上下文从路由参数获取

### 3.3 导航结构

```
侧边栏（树形，可展开/折叠）:
  📊 工作台
  ── 分隔线 ──
  📦 应用
    → my-blog
    → api-service
    → admin-panel
    → + 新建应用
  ── 分隔线 ──
  🖥 服务器
    → prod-01 (可展开)
       概览 | Nginx 网关 | Docker | 系统 | 文件 | 终端
    → db-01
    → + 添加服务器
  ── 分隔线 ──
  🔔 通知
  ⚙ 设置
```

### 3.4 路由设计

```
/dashboard                        → 工作台

/apps                             → 应用列表
/apps/create                      → 新建应用向导
/apps/:appId/overview             → 应用概览
/apps/:appId/domain               → 域名 & SSL
/apps/:appId/service              → 后端服务（Docker容器）
/apps/:appId/deploy               → 部署管理
/apps/:appId/logs                 → 聚合日志
/apps/:appId/database             → 数据库
/apps/:appId/env                  → 环境变量

/servers                          → 服务器列表
/servers/:serverId/overview       → 服务器概览
/servers/:serverId/nginx          → Nginx 网关
/servers/:serverId/docker         → Docker 容器管理
/servers/:serverId/system         → 系统工具
/servers/:serverId/files          → 文件管理
/servers/:serverId/terminal       → 终端

/notifications                    → 通知 & 告警
/settings                         → 设置
```

---

## 四、数据模型变化

### 4.1 新增 Application 表

```sql
CREATE TABLE applications (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  name          TEXT    NOT NULL UNIQUE,
  description   TEXT    DEFAULT '',
  server_id     INTEGER NOT NULL,        -- 主服务器 FK → servers.id
  site_name     TEXT    DEFAULT '',       -- 关联 Nginx 站点名（conf.d 文件名）
  domain        TEXT    DEFAULT '',       -- 域名（冗余存储便于查询）
  container_name TEXT   DEFAULT '',       -- 关联 Docker 容器名
  deploy_id     INTEGER,                 -- FK → deploys.id (nullable)
  db_conn_id    INTEGER,                 -- FK → db_conns.id (nullable, 可跨服务器)
  status        TEXT    DEFAULT 'unknown', -- online/offline/unknown/error
  created_at    DATETIME,
  updated_at    DATETIME
);
```

### 4.2 现有模型不变

Server、Deploy、SSLCert、DBConn、Metric 等保持原样。Application 只是在上面加一层聚合关联。

### 4.3 新增 API

```
GET    /panel/api/v1/apps              列表（?server_id 可选过滤）
POST   /panel/api/v1/apps              创建
GET    /panel/api/v1/apps/:id          详情（含聚合状态）
PUT    /panel/api/v1/apps/:id          更新
DELETE /panel/api/v1/apps/:id          删除（仅解除关联）
GET    /panel/api/v1/apps/:id/status   实时状态聚合
POST   /panel/api/v1/apps/scan         扫描 Nginx conf.d 自动导入
```

---

## 五、页面迁移映射

### 现有页面 → 新架构

| 旧页面 | 拆解到 |
|--------|--------|
| Dashboard | → 新 Dashboard（工作台） |
| Servers | → ServerList（保留 + 增强） |
| Terminal | → ServerTerminal（从路由参数获取 serverId） |
| Websites | → AppDomain（单应用维度）+ ServerNginx（服务器网关视图） |
| SSL | → AppDomain（合并到域名 Tab） |
| Docker | → AppService（单应用维度）+ ServerDocker（服务器全量视图） |
| Deploy | → AppDeploy + AppEnv（拆分部署和环境变量） |
| Database | → AppDatabase（单应用维度） |
| Files | → ServerFiles（从路由参数获取 serverId） |
| System | → ServerSystem（从路由参数获取 serverId） |
| Notifications | → 保留 |
| Settings | → 保留 |

### 新增页面

| 页面 | 功能 |
|------|------|
| AppList | 应用列表/卡片 + 筛选 + 扫描导入 |
| AppCreate | 新建应用向导（分步表单） |
| AppOverview | 应用概览（状态聚合 + 快捷操作 + 部署历史） |
| AppLogs | 聚合日志（Nginx + 容器统一查看） |
| ServerOverview | 服务器概览（指标 + 承载应用 + 容器概况） |
| ServerNginx | Nginx 网关（路由表 + 全局日志） |

---

## 六、双向穿透设计

### 应用 → 服务器
- 应用概览页的"服务器"字段可点击 → `/servers/:serverId/overview`
- "打开终端"按钮 → `/servers/:serverId/terminal`

### 服务器 → 应用
- Nginx 网关路由表中"关联应用"列可点击 → `/apps/:appId/overview`
- Docker 容器列表中"关联应用"列可点击 → `/apps/:appId/overview`
- 服务器概览的"承载应用"列表可点击 → `/apps/:appId/overview`

### 工作台 → 任意
- "SSL 到期"待处理项 → `/apps/:appId/domain`
- "版本漂移"待处理项 → `/apps/:appId/deploy`
- "服务器离线" → `/servers/:serverId/overview`

---

## 七、与竞品对比

| 特性 | ServerHub (新) | Coolify | 1Panel | 宝塔 |
|------|---------------|---------|--------|------|
| 应用视角 | ✅ 一等公民 | ✅ Project→Resource | ❌ | ❌ |
| 服务器视角 | ✅ 完整 | ⚠ 弱 | ✅ 单机 | ✅ 单机 |
| 双视角穿透 | ✅ | ❌ | ❌ | ❌ |
| Nginx 原生管理 | ✅ 网关+站点 | ❌ 自动生成不可编辑 | ✅ | ✅ |
| 运维工作台 | ✅ 待办+活动流 | ❌ | ❌ | ⚠ 简单 |
| 零 Agent | ✅ | ❌ 需要 Agent | ❌ 需要 Agent | ❌ 需要 Agent |
| 多服务器 | ✅ | ✅ | ❌ | ❌ |

---

## 八、实施阶段与时间线

| 阶段 | 内容 | 预估 | 状态 |
|------|------|------|------|
| 1 | 后端：Application 模型 + CRUD API + 扫描导入 | 2天 | 待开始 |
| 2 | 前端：树形侧边栏 + 嵌套路由 + 全局 Store | 3天 | 待开始 |
| 3 | 前端：应用视角 9 个页面 | 5天 | 待开始 |
| 4 | 前端：服务器视角 7 个页面 | 4天 | 待开始 |
| 5 | 前端：工作台 Dashboard | 2天 | 待开始 |
| 6 | 交叉链接 + 危险操作确认 + 表单验证 + Auth修复 | 2天 | 待开始 |
| **合计** | | **~18天** | |

详细实施步骤见: `.zcf/plan/current/双视角交互架构重构.md`

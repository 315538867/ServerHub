# 01 — 核心功能清单

> 基线: ServerHub 当前版本(commit `845021e` 起)所有线上功能
> 用途: 重构必须 100% 覆盖此清单。任一功能丢失视为重构失败。

---

## 功能矩阵

按"对用户的价值"分组,每条标注:**域(Domain)→ 子系统 → 是否核心**。

### A. 服务器管理(Server)

| # | 功能 | 域 | 核心 |
|---|---|---|---|
| A1 | 添加/编辑/删除服务器(local/ssh) | Server | ✓ |
| A2 | 多 Network 入口管理(loopback/private/vpn/tunnel/public) | Server.Network | ✓ |
| A3 | SSH 密钥/密码认证、密钥加密存储 | Server | ✓ |
| A4 | 服务器在线探活、状态展示 | Server | ✓ |
| A5 | Metric 采集(CPU/Mem/Disk/Load/Uptime) | Observability | ✓ |
| A6 | 远程目录浏览/上传/下载(SFTP) | Files | ✓ |
| A7 | Web Terminal(WebSocket → SSH) | Terminal | ✓ |
| A8 | Resolver(跨机选 upstream IP) | Server.Network | ✓ |

### B. 服务发现与接管(Discovery + Takeover)

| # | 功能 | 域 | 核心 |
|---|---|---|---|
| B1 | Docker 容器发现 | Discovery | ✓ |
| B2 | Docker Compose 项目发现 | Discovery | ✓ |
| B3 | Nginx 站点发现(从 conf 反推) | Discovery | ✓ |
| B4 | systemd 服务发现 | Discovery | ✓ |
| B5 | Fingerprint 去重(SHA1) | Discovery | ✓ |
| B6 | Docker 容器接管(→ docker-compose 物化) | Takeover | ✓ |
| B7 | Docker Compose 项目接管 | Takeover | ✓ |
| B8 | Native 进程接管 | Takeover | ✓ |
| B9 | Static 站点接管 | Takeover | ✓ |
| B10 | 接管步骤引擎(Step + Undo 原子回滚) | Takeover | ✓ |

### C. 服务与发布(Service + Release)

| # | 功能 | 域 | 核心 |
|---|---|---|---|
| C1 | Service CRUD(命名/类型/绑应用) | Service | ✓ |
| C2 | Floating Service(未绑 Application) | Service | ✓ |
| C3 | Release 三维模型(Artifact/EnvVarSet/ConfigFileSet) | Release | ✓ |
| C4 | StartSpec 4 类(docker/compose/native/static) | Release | ✓ |
| C5 | Apply Release(部署一次) | Deploy | ✓ |
| C6 | DeployRun 审计(每次部署写一行) | Deploy | ✓ |
| C7 | 回滚到上一条 active Release | Deploy | ✓ |
| C8 | AutoRollbackOnFail 策略 | Deploy | ✓ |
| C9 | 自动 sync(reconciler 周期重放) | Reconciler | ✓ |
| C10 | SyncStatus 状态机(''/synced/syncing/error) | Reconciler | ✓ |
| C11 | EnvVarSet 加密存储(AES-GCM) | Release | ✓ |
| C12 | ConfigFileSet 多文件版本化 | Release | ✓ |
| C13 | Webhook 触发部署(WebhookSecret) | Deploy | 二级 |

### D. 应用与路由(Application + Ingress)

| # | 功能 | 域 | 核心 |
|---|---|---|---|
| D1 | Application CRUD | Application | ✓ |
| D2 | App ↔ Service 绑定(N:1 或独立) | Application | ✓ |
| D3 | App ↔ Database 绑定 | Application | 二级 |
| D4 | ExposeMode: none/path/site | Application | ✓ |
| D5 | Nginx Site 渲染(完整 server 块) | Ingress | ✓ |
| D6 | Nginx Path 路由(挂在 default site) | Ingress | ✓ |
| D7 | 多 upstream(跨机用 Resolver 选 IP) | Ingress | ✓ |
| D8 | Nginx -t 校验 + reload | Ingress | ✓ |
| D9 | SSL 证书管理(ACME 自动签发) | Ingress.SSL | ✓ |
| D10 | App 状态聚合(从下属 Service) | Application | ✓ |
| D11 | App 目录规范(base_dir + entries) | Application | ✓ |

### E. 数据库管理(Database)

| # | 功能 | 域 | 核心 |
|---|---|---|---|
| E1 | DB 连接 CRUD(MySQL/PG) | Database | 二级 |
| E2 | 备份策略(定时 + 保留) | Database | 二级 |
| E3 | 备份历史 + 下载 | Database | 二级 |

### F. 观测与告警(Observability)

| # | 功能 | 域 | 核心 |
|---|---|---|---|
| F1 | Dashboard 总览 | Obs | ✓ |
| F2 | 日志聚合 + 全文搜索 | LogSearch | ✓ |
| F3 | Alert 规则 + 触发 | Alerts | ✓ |
| F4 | Notify 渠道(Webhook/Email) | Notify | ✓ |
| F5 | Audit Queue(审计流) | Audit | ✓ |
| F6 | 健康检查端点 | Health | ✓ |

### G. 平台与运维(Platform)

| # | 功能 | 域 | 核心 |
|---|---|---|---|
| G1 | 用户登录(密码 + MFA TOTP) | Identity | ✓ |
| G2 | 会话管理 + JWT | Identity | ✓ |
| G3 | Setup 向导(首次启动) | Setup | ✓ |
| G4 | Settings(系统配置) | Settings | ✓ |
| G5 | Migration 框架(版本化 + 单次执行) | Platform | ✓ |
| G6 | SSH Pool / SFTP Pool(连接复用) | Infra | ✓ |
| G7 | Runner 抽象(local + ssh 双后端) | Infra | ✓ |
| G8 | SSE / WebSocket 推送框架 | Infra | ✓ |

---

## 功能 → 子系统 → 重构后归属

| 子系统 | 现位置(v1) | 目标位置(v2) |
|---|---|---|
| Identity | api/auth + middleware | usecase/auth + repo/user + handler |
| Server | api/servers + model | usecase/server + repo/server + domain/server |
| Discovery | pkg/discovery (4 文件) | adapters/source/{docker,compose,nginx,systemd}/ |
| Takeover | pkg/takeover (4 文件) | adapters/source/<kind>/ 同包(Discover + Takeover 合一) |
| Service | model/service + handler 散写 | domain/service + repo/service + usecase/service |
| Release | api/release + pkg/deployer | domain/release + usecase/deploy + adapters/runtime/<kind>/ |
| Application | api/application + handler 聚合 | usecase/application + derive/application |
| Ingress | pkg/nginxops/render/presets | adapters/ingress/nginx/ + usecase/ingress |
| Reconciler | pkg/scheduler | usecase/reconcile(用 RuntimeAdapter.Probe) |
| Notify | pkg/notify | adapters/notify/{webhook,email,...}/ |
| Metric/Log/Alert | api/{metrics,logsearch,alerts} | usecase/<name> + repo + derive |

---

## 功能完整性自检清单(每 Phase commit 前必跑)

```
□ A1-A8 服务器全部 CRUD 通过
□ B1-B10 4×4 发现 + 接管 e2e 跑通
□ C5/C7 Apply Release + Rollback e2e 跑通
□ C9 reconciler 周期触发 + 状态正确
□ D5/D6/D8 Nginx site 渲染 + reload 成功
□ D9 ACME 证书签发(用 staging endpoint)
□ F1 Dashboard 数据展示正常
□ G1/G2 登录 + MFA + JWT 全流程
□ go test -count=1 ./... 全绿
□ vue-tsc --noEmit 全绿
```

任一项失败 → 当前 phase 不准 commit。

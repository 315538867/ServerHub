# 00 — 架构总览

> 版本: v2 (Hexagonal + Plugin Registry)
> 状态: Draft / 待批准
> 适用范围: ServerHub 全栈(backend/frontend)
> 取代: docs/architecture.md / docs/architecture-deploy.md(归为 v1)

---

## 1. 项目定位

ServerHub 是面向中小团队的"自有服务器统一管理面板",在一台主控机上集中管理多台 Linux/macOS 服务器,把"服务发现 → 接管 → 部署 → 路由 → 观测 → 告警"整链路的运维平面收敛进单一二进制 + Web UI。

定位关键词:
- **统一面板**:Server / Service / Application / Ingress 同一抽象集
- **接管而不替代**:对已有 docker / compose / systemd / nginx 残留服务做 Discovery + Takeover,不要求用户先把现状洗一遍
- **可拓展运行时**:docker / compose / native / static 是当前 4 类 runtime,但架构上不绑死任一种 —— 加 k8s/podman 是写一个新 adapter 的工作量

## 2. 项目使命

把"运维知识"从命令行/笔记/聊天群迁移到一个有版本化、有审计、可回滚的系统里。**Release 三维模型(Artifact + EnvVarSet + ConfigFileSet + StartSpec)是这套系统的最高契约**:任何一次部署都是一次不可变 Release 的应用,出问题秒级回滚到上一条 active Release。

## 3. 架构核心理念(v2)

| 理念 | 落地 |
|---|---|
| **Hexagonal**(端口适配器) | core/ 定义接口,adapters/ 实现,业务核心不依赖具体后端 |
| **Plugin Registry** | 每个 Runtime/Source/Ingress/Notify 后端 = 一个 adapter 包,init() 自注册 |
| **真值 vs 派生分离** | model/ 仅持有真值,摘要(LastStatus/Image/AppStatus)由 derive/ 计算 |
| **状态机显式化** | 所有 enum 字段配 `CanTransitionTo()` + GORM hook 强制 |
| **Repository 收口** | 所有 GORM 调用集中到 repo/,handler 不直 db |
| **Usecase 编排层** | handler 只解析参数 + 调 usecase,业务逻辑在 usecase/ 一层 |

## 4. C4 模型 - Level 1(系统上下文)

```
                        ┌──────────────────────────────┐
                        │           User               │
                        │ (Operator / Developer)       │
                        └──────────────┬───────────────┘
                                       │ HTTPS + WebSocket
                                       ▼
                        ┌──────────────────────────────┐
              ┌────────▶│      ServerHub Backend       │◀──────┐
              │         │  (Go single binary + SQLite) │       │
              │         └──────────────┬───────────────┘       │
              │                        │                       │
        Web UI│              SSH/exec  │      ACME / Let's     │
       (Vue3) │                        │      Encrypt          │
              │                        ▼                       │
              │             ┌────────────────────┐             │
              │             │  Managed Servers    │            │
              └─────────────│  (1..N hosts)       │────────────┘
                            │  - docker/compose   │
                            │  - systemd          │
                            │  - nginx            │
                            │  - native processes │
                            └─────────────────────┘
```

外部依赖:
- **被管服务器**:SSH 通道执行命令(本机走 local runner,远程走 ssh runner)
- **ACME**:用于自动签发 SSL 证书(Let's Encrypt / ZeroSSL)
- **可选 Notify**:Webhook / Email / 钉钉 / 飞书 等

## 5. 核心子系统

| 子系统 | 职责 | 关键概念 |
|---|---|---|
| **Identity** | 用户/会话/MFA | User, Session, TOTP |
| **Server** | 主机注册 + 网络 + 探活 | Server, Network, Metric |
| **Discovery** | 在 Server 上扫描候选 Service | Candidate, Fingerprint |
| **Takeover** | 把候选物化为 Service | Step 引擎, Undo 链 |
| **Service** | 运行单元(=部署目标) | Service, Type(runtime), CurrentRelease |
| **Release** | 不可变部署描述 | Release(三维), StartSpec, DeployRun |
| **Application** | 业务聚合体(N×Service+Ingress) | Application, ExposeMode |
| **Ingress** | 流量接入(Nginx/SSL) | IngressRoute, NginxConfig |
| **Reconciler** | 周期重放 CurrentRelease | Scheduler, SyncStatus 状态机 |
| **Observability** | 指标/日志/告警/审计 | Metric, Audit, Alert, Notify |

## 6. 关键拓展点(对外承诺)

| 想加什么 | 改文件数 |
|---|---|
| 新 Runtime(k8s/podman/...) | 1 (adapters/runtime/<kind>/) |
| 新 Source(k3s/portainer/...) | 1 (adapters/source/<kind>/) |
| 新 Ingress 后端(caddy/traefik) | 1 (adapters/ingress/<kind>/) |
| 新 Notify 渠道(飞书/Slack) | 1 (adapters/notify/<kind>/) |
| 新派生字段 | 1 (derive/<entity>.go) |
| 新业务用例 | 1 (usecase/<name>.go) + handler 一行接入 |

## 7. 文档导航

- [01-features.md](./01-features.md) — 核心功能清单与分类
- [02-architecture.md](./02-architecture.md) — Hexagonal 分层 + C4-L2/L3
- [03-domain-model.md](./03-domain-model.md) — 领域模型 + ER + 状态机
- [04-core-flows.md](./04-core-flows.md) — 关键流程时序图
- [05-extension-points.md](./05-extension-points.md) — 接口契约 + 拓展场景
- [06-quality-gates.md](./06-quality-gates.md) — 性能/测试/执行标准

# 04 — 核心流程时序图

> 范围: 5 条关键业务流程
> 表达: ASCII 时序图(每条都标层次)

---

## 流程 1:Service Discovery + Takeover(发现 + 接管)

**入口**: `POST /servers/:id/discover` → 列候选 → 用户选 → `POST /takeover`

```
User    api/discovery   usecase/discovery    SourceScanner       Runner       repo
 │            │                  │                  │                │           │
 │ POST       │                  │                  │                │           │
 ├──────────▶ │                  │                  │                │           │
 │            │ Discover(svrID)  │                  │                │           │
 │            ├────────────────▶ │                  │                │           │
 │            │                  │ source.All().each(scanner):      │           │
 │            │                  │   Scanner.Discover(ctx, runner)  │           │
 │            │                  ├────────────────▶ │                │           │
 │            │                  │                  │ docker ps/inspect           │
 │            │                  │                  ├──────────────▶ │           │
 │            │                  │                  │ ◀── output ────│           │
 │            │                  │ ◀── candidates ──│                │           │
 │            │                  │                                              │
 │            │                  │ Fingerprint dedup vs repo.Service:           │
 │            │                  ├────── repo.Service.ListByFingerprint ──────▶ │
 │            │                  │ ◀────── existing fingerprints ─────────────  │
 │            │ ◀── candidates ──│                                              │
 │ ◀── 200 ── │                                                                 │

[用户选择 candidate, 提交 takeover]

User    api/takeover   usecase/takeover    SourceScanner    StepEngine    repo
 │            │                  │                  │              │          │
 │ POST       │                  │                  │              │          │
 ├──────────▶ │                  │                  │              │          │
 │            │ Takeover(req)    │                  │              │          │
 │            ├────────────────▶ │                                              │
 │            │                  │ source.MustGet(req.Kind)                    │
 │            │                  │      .Takeover(ctx, tc):                    │
 │            │                  ├────────────────▶ │                          │
 │            │                  │                  │ PlanSteps()              │
 │            │                  │                  │ Run via StepEngine       │
 │            │                  │                  ├──────────▶  │            │
 │            │                  │                  │             Step1.Do()   │
 │            │                  │                  │             Step2.Do()   │
 │            │                  │                  │             ...          │
 │            │                  │                  │             [失败 → Undo 链] │
 │            │                  │ ◀── ok ──────────│                          │
 │            │                  │ factory.NewService(domain.Service{...})    │
 │            │                  │ repo.Service.Create ────────────────────▶  │
 │            │                  │ repo.DeployRun.Create(success, takeover) ▶ │
 │            │                  │ if c.Suggested.Env: repo.EnvVarSet.Create  │
 │            │ ◀── service.id ──│                                            │
 │ ◀── 200 ── │                                                               │
```

**不变量保证**:
- INV-8:takeover 末尾必写一条 `DeployRun{Status=success, Source=takeover}`
- 失败:Step.Undo 链回滚远端副作用,Service 不入库

## 流程 2:Apply Release(部署一次)

**入口**: `POST /services/:id/apply` body: `{release_id}`

```
User   api/service   usecase/deploy    runtime.Adapter    StepEngine    repo
 │          │              │                 │                │            │
 │ POST     │              │                 │                │            │
 ├────────▶ │              │                 │                │            │
 │          │ ApplyRelease(svcID, relID)                                   │
 │          ├────────────▶ │                                                │
 │          │              │ svc := repo.Service.Get(svcID)                │
 │          │              │ rel := repo.Release.Get(relID)                │
 │          │              │ assert(rel.ServiceID == svcID)                │
 │          │              │ svc.State.CanTransitionTo(Syncing) ─ check    │
 │          │              │ repo.Service.UpdateState(svcID, Syncing)      │
 │          │              │   [原子守卫: WHERE state != 'syncing']        │
 │          │              │                                                │
 │          │              │ ad := runtime.MustGet(svc.Type)                │
 │          │              │ steps := ad.PlanStart(svc, rel)               │
 │          │              ├──────────────▶ │                              │
 │          │              │ ◀── steps ──── │                              │
 │          │              │                                                │
 │          │              │ defer: 兜底 sync_status (永远走到 Synced/Error)│
 │          │              │ run := repo.DeployRun.Create(running, relID)  │
 │          │              │ stepEngine.Run(steps) ───────────▶ │          │
 │          │              │                                    │ Do()...  │
 │          │              │ ◀── result ────────────────────── │          │
 │          │              │                                                │
 │          │              │ if ok:                                         │
 │          │              │   repo.Release.MarkActive(relID)              │
 │          │              │     [事务: 旧 active → archived]              │
 │          │              │   repo.Service.UpdateCurrentRelease(svcID, relID) │
 │          │              │   repo.DeployRun.Finish(run.id, success)      │
 │          │              │   repo.Service.UpdateState(svcID, Synced)     │
 │          │              │ else:                                          │
 │          │              │   repo.DeployRun.Finish(run.id, failed)       │
 │          │              │   repo.Service.UpdateState(svcID, Error)      │
 │          │              │   if svc.AutoRollbackOnFail:                   │
 │          │              │     prev := repo.Release.PrevActive(svcID)    │
 │          │              │     ApplyRelease(svcID, prev.id) [递归]       │
 │          │ ◀── result ──│                                                │
 │ ◀── 200/SSE ──┤                                                          │
```

**性能保证**:不进 N+1 —— 每次 apply 是 4-6 条 SQL,与 release 数量无关。

## 流程 3:Reconcile(周期重放)

**入口**: scheduler 定时器 → 对每个 AutoSync=true 的 Service

```
Scheduler   usecase/reconcile    runtime.Adapter      repo
    │              │                    │                │
    │ tick         │                    │                │
    ├────────────▶ │                    │                │
    │              │ svcs := repo.Service.ListAutoSync() │
    │              │ for svc in svcs (并行有限并发):      │
    │              │   if svc.State == Syncing: skip     │
    │              │   ad := runtime.MustGet(svc.Type)   │
    │              │   probe := ad.Probe(ctx, runner, svc) │
    │              │   if probe.Running: continue        │
    │              │   else: ApplyRelease(svc, svc.CurrentRelease)
    │              │     [复用流程 2]                    │
```

**SyncStatus 卡死防护**:
- usecase 入口 `defer` 必走最终态(Syncing → Synced/Error)
- 启动期扫描:`UPDATE services SET state='Error' WHERE state='Syncing' AND updated_at < now()-5m`(R6 加)

## 流程 4:Application Ingress Apply(渲染 + reload)

**入口**: 用户改 App 路由 → `POST /applications/:id/ingress/apply`

```
User   api/app   usecase/ingress     ingress.Backend     Runner      repo
 │        │             │                  │                │           │
 │ POST   │             │                                              │
 ├──────▶ │             │                                              │
 │        │ ApplyIngress(appID)                                         │
 │        ├───────────▶ │                                              │
 │        │             │ app  := repo.Application.Get(appID)          │
 │        │             │ rts  := repo.IngressRoute.ListByApp(appID)   │
 │        │             │ svr  := repo.Server.Get(app.RunServerID)     │
 │        │             │                                              │
 │        │             │ resolve upstream IPs:                        │
 │        │             │   for r in rts:                              │
 │        │             │     r.UpstreamIP := resolver.Pick(r.SvcServer, svr) │
 │        │             │                                              │
 │        │             │ ib := ingress.MustGet("nginx")               │
 │        │             │ conf := ib.Render(rts)                       │
 │        │             ├────────────────▶ │                            │
 │        │             │ ◀── yaml/conf ── │                            │
 │        │             │ runner.WriteFile(/etc/nginx/conf.d/<app>.conf)│
 │        │             │ ib.Validate(ctx, runner)                     │
 │        │             ├────────────────▶ │ nginx -t                   │
 │        │             │ ◀── ok/err ──── │                            │
 │        │             │ ib.Reload(ctx, runner, svr) → systemctl reload nginx│
 │        │             │ repo.Application.SetIngressApplied(appID)    │
 │        │ ◀── 200 ────│                                              │
```

## 流程 5:Discovery → Service 列表(读路径,派生展示)

**入口**: `GET /servers/:id/services`

```
User    api/server   usecase/server    repo                derive
 │           │              │            │                     │
 │ GET       │              │            │                     │
 ├─────────▶ │              │            │                     │
 │           │ ListSvcsByServer(srvID)                          │
 │           ├────────────▶ │            │                     │
 │           │              │ svcs := repo.Service.ListByServer(srvID)
 │           │              │ ids  := [s.ID for s in svcs]    │
 │           │              │ entries := derive.ServiceLatest(repo, ids)
 │           │              ├────────────────────────────────▶ │
 │           │              │                                  │
 │           │              │   段一: deploy_runs 子查 latest_started_at
 │           │              │   段二: services JOIN releases 取 StartSpec
 │           │              │   合并 → map[id]Entry{Status, Image, RuntimeStatus}
 │           │              │ ◀────── entries ───────────────  │
 │           │              │ for s in svcs: 拼 SvcView(s, entries[s.ID])
 │           │ ◀── views ── │                                  │
 │ ◀── 200 ──│                                                 │
```

**性能**:无论 N 多大,SQL 数 = 1 主查 + 2 派生查 = O(常数)。

## 6. 错误传播与超时

| 层 | 错误类型 | 处理 |
|---|---|---|
| api/ | 参数错误 | 400,直接 resp.Bad |
| usecase/ | 业务/状态机错误 | 422,resp.Bad with code |
| usecase/ | adapter/runner 错误 | 500,resp.Err + audit log |
| adapters/ | 远端命令失败 | 包装成 RemoteError 上抛 |
| repo/ | DB 错误 | 直接上抛,usecase 顶层兜 |

所有 usecase 入口:`ctx, cancel := context.WithTimeout(parent, 60s)`,可被取消。

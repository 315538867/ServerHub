# R6 改造目标清单(R0 基线快照)

> 生成时间: 2026-04-26 20:38(R0 基线)
> 来源命令:
>   - handler 直 db: `git grep -nE "(DB|db)\.(Find|Where|Create|Save|Update|Delete|First|Model|Order|Count|Preload|Begin)" -- 'api/**/*.go' ':!**/*_test.go'`
>   - model 派生字段: `grep -rEn "^\s+(Status|LastStatus|ImageName|DesiredVersion|ActualVersion|LastRunAt|SyncStatus)" backend/model/`
> 用途: R3 / R6 / R7 改造 input

---

## 1. handler 直 db 调用统计(R6 输入)

**总计**: 166 行,分布在 26 个文件。

| 文件 | 行数 | R6 拆分子域 |
|---|---|---|
| api/application/handler.go | 23 | application |
| api/ingresses/handler.go | 19 | ingressroute |
| api/servers/handler.go | 18 | server |
| api/release/handler.go | 15 | release |
| api/alerts/handler.go | 14 | metric/audit/alert |
| api/nginx/profile_handler.go | 10 | ingress(R5 已迁出大头) |
| api/database/handler.go | 8 | database |
| api/apprelease/service.go | 8 | application(子域内 service helper) |
| api/ssl/handler.go | 7 | ingress.ssl |
| api/auth/handler.go | 7 | identity |
| api/ingresses/import_handler.go | 6 | ingressroute |
| api/auth/totp.go | 5 | identity |
| api/servers/networks.go | 4 | server.network |
| api/setup/handler.go | 3 | identity(setup) |
| api/metrics/handler.go | 3 | metric |
| api/apprelease/handler.go | 3 | application |
| api/system/handler.go | 2 | platform |
| api/nginx/handler.go | 2 | ingress |
| api/discovery/handler.go | 2 | discovery(R4 已收) |
| api/terminal/handler.go | 1 | platform |
| api/settings/handler.go | 1 | platform |
| api/release/webhook.go | 1 | release |
| api/logsearch/handler.go | 1 | observability |
| api/files/handler.go | 1 | platform |
| api/docker/handler.go | 1 | runtime(R2 已收大头) |
| api/audit/handler.go | 1 | metric/audit |

**R6.1 子任务划分(对齐主计划 T6.1)**:

- T6.1.a server: api/servers/handler.go + api/servers/networks.go = 22 行
- T6.1.b service: 含 api/apprelease/service.go = 8 行(命名易混,实为 service helper)
- T6.1.c application: api/application/handler.go + api/apprelease/handler.go = 26 行
- T6.1.d release: api/release/handler.go + api/release/webhook.go = 16 行
- T6.1.e deployrun: 在 release/apply 路径(已属 R2,但 db.Create 可能漏)
- T6.1.f ingressroute: api/ingresses/* + api/nginx/* = 37 行
- T6.1.g database: api/database/handler.go = 8 行
- T6.1.h metric/audit/alert/observability: api/alerts + api/metrics + api/audit + api/logsearch = 19 行
- T6.1.i identity: api/auth/* + api/setup = 15 行
- T6.1.j platform 杂项: api/system + api/settings + api/files + api/terminal = 5 行

**全部清单原始数据**: `/tmp/handler-direct-db.txt`(166 行)

## 2. model 派生字段清单(R3 / R7 输入)

| 文件 | 字段 | 类型 | 派生入口 | 处理 Phase |
|---|---|---|---|---|
| model/application.go:43 | Status string `default:unknown` | enum | derive/application.go | **R3 删字段** |
| model/server.go:21 | Status string `default:unknown` | enum | derive/server.go | **R3 删字段** |
| model/service.go:77 | SyncStatus string `default:''` | enum | derive/service.go(由 DeployRun 派生) | **R3 删字段** |
| model/ingress.go:25 | Status string `default:pending` | enum | 派生自 IngressRoute apply 状态 | **R5 内删** |
| model/release.go:33 | Status string `default:draft` | enum | **真值,保留**(状态机本身) | 保留 |
| model/deploy_run.go:28 | Status string `default:running` | enum | **真值,保留**(审计日志) | 保留 |
| model/deploy_log.go:9 | Status string | enum | 旧表,可能与 deploy_run 重复 | **R6 评估清理** |
| model/audit_log.go:13 | Status int | int | **真值,保留**(HTTP 状态码) | 保留 |
| model/app_release_set.go:26 | Status string `default:draft` | enum | **真值,保留**(集发布状态机) | 保留 |
| model/app_release_set.go:47 | Status string | enum | **真值,保留**(子项执行结果) | 保留 |

**R3 必删 4 字段**:
1. `model.Application.Status`
2. `model.Server.Status`
3. `model.Service.SyncStatus`
4. `model.Ingress.Status`(R5 阶段顺手清,如时间紧可推迟)

## 3. 其它 R0 阶段观察

### 3.1 旧 pkg/ 体量(R2/R4/R5 删除目标)

```
pkg/deployer/        R2 全删
pkg/discovery/       R4 全删
pkg/takeover/        R4 全删
pkg/nginxops/        R5 全删
pkg/nginxpresets/    R5 评估并入 adapters/ingress/nginx/
pkg/nginxrender/     R5 评估并入 adapters/ingress/nginx/render
pkg/scheduler/       R6 业务逻辑搬到 usecase/reconcile,只留定时器骨架
pkg/svcstatus/       R3 / R6 评估并入 derive/(服务状态/镜像派生)
```

### 3.2 已就位的好基础(沿用)

```
pkg/runner/          → backend/infra/runner/(R6 整体迁移,含 sshpool/sftppool/safeshell/safehttp)
pkg/resp/            → backend/infra/resp/
pkg/sse/ + wsstream/ → backend/infra/sse/  backend/infra/wsstream/
pkg/crypto/          → backend/infra/crypto/(EnvVarSet 加密)
pkg/totp/            → backend/infra/totp/
migration/           → 保留架构,加新版本号
```

## 4. 测试基线

- 全包 `go test -count=1 ./...` 全绿(R0 验证)
- coverage: 详见 `baseline/cover-v1.txt`(671 行 func 级)
- bench: 详见 `baseline/bench-v1.txt`(svcstatus / nginxrender 当前无 bench,R3/R5 阶段必须新增)

## 5. R0 退出后下一步

R1 起步:`backend/core/` 5 端口 + 4 registry,空实现保编译。

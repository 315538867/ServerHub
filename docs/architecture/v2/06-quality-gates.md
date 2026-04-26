# 06 — 质量门禁

> 范围: 性能基准 / 测试策略 / 每 Phase 准入准出 / CI 红绿规则
> 用途: 重构每一步都有可量化的"过/不过",杜绝"差不多就行"

---

## 1. 性能基准(SLO)

| 路径 | P50 | P99 | SQL 数 | 备注 |
|---|---|---|---|---|
| `GET /servers/:id/services` (N=20) | < 30ms | < 80ms | 3 | 1 主 + 2 派生 |
| `GET /applications` (N=50) | < 50ms | < 150ms | 4 | 主 + 派生 + ingress |
| `GET /dashboard` | < 100ms | < 300ms | ≤ 8 | 总览页 |
| `POST /services/:id/apply` (本地) | < 2s | < 5s | ≤ 6 | 不含远端时间 |
| `POST /servers/:id/discover` | < 3s | < 10s | ≤ 4 | 远端 docker ps 时间为主 |
| Reconcile 单 service | < 500ms | < 2s | ≤ 3 | 不含远端 probe 时间 |
| Login + JWT 签发 | < 200ms | < 500ms | 3 | 含 bcrypt |

**红线**:
- 任意接口 SQL 数 ≥ 10 → CI 拒
- 任意接口 P99 ≥ SLO 2 倍 → CI 拒
- 任意 list 接口出现 N+1(SQL 数随 N 增长)→ PR 拒

**测量手段**:
- benchmark: `go test -bench` 跑 list/apply 主路径,GitHub Action 跑出 baseline
- SQL 计数: 用 GORM `Logger.LogMode(silent)` + 自研 hook 计数,跑测试时断言
- 慢日志: SQLite `PRAGMA query_only_threshold` 配 100ms,超阈日志 → CI 检查

## 2. 测试金字塔

```
       ┌─────────┐
       │  E2E    │  10%   关键用户流(login, takeover, apply, rollback)
       ├─────────┤
       │integration│ 30%  usecase + repo + sqlite 真实落库
       ├─────────┤
       │  unit    │ 60%   adapter / derive / domain 状态机
       └─────────┘
```

### 2.1 Unit Test 覆盖要求

| 包 | 覆盖率 | 必测内容 |
|---|---|---|
| domain/ | ≥ 90% | 全部状态机 transition + 不变量 hook |
| derive/ | ≥ 95% | 派生函数边界(空/单/多/混合) |
| adapters/runtime/* | ≥ 80% | PlanStart 的 4 类 StartSpec / Probe 输出解析 |
| adapters/source/* | ≥ 80% | Discover 解析 / Fingerprint 稳定性 / Takeover step 链 |
| adapters/ingress/* | ≥ 85% | Render 输出 vs 黄金 fixture / Validate 错误传播 |
| infra/runner/* | ≥ 70% | local + ssh 双后端契约一致 |
| repo/ | ≥ 75% | ListByIDs 批量 / 状态机原子守卫 |

### 2.2 Integration Test

每个 usecase 必须有一条 happy path + 至少一条失败路径:

```go
// usecase/deploy_test.go
func TestApplyRelease_Success(t *testing.T)            // happy
func TestApplyRelease_StateGuardConflict(t *testing.T) // 并发 apply
func TestApplyRelease_AdapterError_Rollback(t *testing.T)
func TestApplyRelease_AutoRollback(t *testing.T)
```

启动: `go test -tags=integration -count=1 ./usecase/...`

### 2.3 E2E (黑盒)

写在 `tests/e2e/` 目录,用 docker-compose 起一台真实被管 ubuntu + ServerHub backend,跑:

| 场景 | 步骤 |
|---|---|
| 完整 docker takeover | add server → discover → takeover → list services |
| 完整 apply + rollback | create release → apply → 改 release → apply → rollback |
| Application + ingress | create app → bind service → set route → apply ingress |
| MFA login | setup TOTP → logout → login w/ TOTP |
| Auto reconcile | service down → 等 30s → 自动恢复 |

跑: `make e2e`(在 CI 单独 job,15 分钟超时)

## 3. CI 门禁(每 PR)

```yaml
# .github/workflows/ci.yml 概念示意
jobs:
  lint:
    - go vet ./...
    - golangci-lint run
    - vue-tsc --noEmit
  unit:
    - go test -race -coverprofile=cover.out ./...
    - 覆盖率 < 阈值 → fail
  integration:
    - go test -tags=integration -count=1 ./usecase/... ./repo/...
  benchmark:
    - go test -bench=. -benchmem ./derive/... ./repo/...
    - 对比 main 分支 baseline,回退 > 20% → fail
  arch-lint:
    - 自研脚本: 检查 import 反向(api ← usecase 是禁止)
    - 检查 adapter 互相 import
    - 检查 model 是否在 repo/migration 之外被 import
  e2e: (nightly,不阻塞 PR)
    - make e2e
```

## 4. 重构 Phase 准入准出

### 4.1 通用准入准出模板

每个 Phase 都遵循:

**准入**:
- [ ] 上一 Phase 全部 commit 通过
- [ ] `go test ./...` 全绿
- [ ] `vue-tsc --noEmit` 全绿
- [ ] 当前 Phase 设计 doc 评审通过

**准出**:
- [ ] 本 Phase 计划清单全部 ✓
- [ ] 单测覆盖率达标(见 §2.1)
- [ ] 性能 SLO 不退化(对比 baseline)
- [ ] 功能自检清单全过(见 01-features.md §自检)
- [ ] 中文 commit 信息,phase 标签清晰

### 4.2 8 Phase 路线图

| Phase | 名称 | 关键产出 | 风险 |
|---|---|---|---|
| **R0** | 基线冻结 | 当前 main commit 标 `v1-final` tag,补缺测试到 60% | 低 |
| **R1** | core/ 接口建立 | 4 个 Registry + 5 个 Port interface 落地,空实现 | 低 |
| **R2** | adapters/runtime 迁出 | docker/compose/native/static 4 个 adapter 全跑通,旧 deployer 删除 | 中 |
| **R3** | derive/ 真值派生 | Application/Server.Status 派生化,model 删字段 | 中 |
| **R4** | adapters/source 迁出 | discovery/takeover 4 套 scanner 落 adapters/ | 中 |
| **R5** | adapters/ingress 迁出 | nginxops 拆 adapters/ingress/nginx + ssl | 高(影响线上路由) |
| **R6** | usecase + repo 收口 | handler 全瘦身,db.Find 全部入 repo | 高(改动面广) |
| **R7** | domain/ 提纯 | model 与 domain 分裂,GORM tag 留 model,domain 纯领域 | 中 |
| **R8** | StartSpec typed + 文档 | typed builder + 6 篇 v2 文档定稿 + v1 归档 | 低 |

预计周期: 3 周(每周 ~2.5 phase),每 Phase 一个 PR,可独立 revert。

### 4.3 强制门禁项(全 Phase 共用)

```
□ 不引入 N+1
□ 不引入 adapter 互 import
□ 不写 model 业务方法(只允许 GORM hook)
□ 不在 handler 写 SQL
□ 不在 usecase 直接 db.xxx
□ 不在 domain 引 gorm/infra
□ 中文 commit + phase 标签
□ 每 commit 后 go test ./... 全绿
```

## 5. 监控与可观测(自洽)

ServerHub 自身的可观测,作为质量门禁的延伸:

- **请求日志**: gin middleware 输出 JSON 行,含 trace_id / latency / sql_count
- **慢请求**: > 500ms 自动告警(走 NotifyChannel,默认 webhook)
- **错误率**: 5 分钟窗口 > 1% → 告警
- **DB 指标**: SQLite size / WAL size / 慢查 / 锁等待
- **Goroutine 泄漏**: `expvar` 暴露,Grafana 看板

## 6. 文档质量门禁

| 文档 | 必备段落 | 评审人 |
|---|---|---|
| 00-overview | C4-L1 + 子系统表 + 拓展承诺 | 架构 + PM |
| 01-features | 全功能矩阵 + 自检清单 | PM + QA |
| 02-architecture | C4-L2 + 端口接口 + 依赖方向 | 架构 |
| 03-domain-model | ER + 状态机 + 不变量 | 架构 + DBA |
| 04-core-flows | 至少 5 个时序图 | 架构 + 业务 |
| 05-extension-points | 接口契约 + 拓展 diff 清单 | 架构 |
| 06-quality-gates | 性能 + 测试 + 准入准出 | QA + 架构 |

任一文档缺段 → 视为重构未完成。

## 7. 回归 - 重构期间的"灯"

每天结束前手动跑(后续 CI 化):

```bash
# 1. 测试
go test -race -count=1 ./...
vue-tsc --noEmit

# 2. SQL 数检测
go test -run=TestSQLCount ./derive/...

# 3. 5 个核心 e2e
make e2e-smoke    # docker takeover + apply + rollback + ingress + login

# 4. 性能对比
go test -bench=. ./derive/... > /tmp/bench.now
benchstat baseline /tmp/bench.now    # 看回退 %
```

任何一条红 → 当天不准 commit 到 main。

## 8. 退出标准(全部 Phase 完成判断)

- [ ] 6 篇 v2 文档全部签字
- [ ] R0-R8 全部 PR merged
- [ ] 旧 architecture.md / architecture-deploy.md 标记 deprecated 移入 docs/v1/
- [ ] CI 8 个 job 全部加上(lint/unit/integration/bench/arch-lint/...)
- [ ] e2e 5 大场景全过
- [ ] 性能基准全部达标
- [ ] 至少接入 1 个第三方插件证明拓展性(eg podman runtime,1 个文件搞定)

达成后,v2 架构 GA,启动后续业务迭代。

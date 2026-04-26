# 02 — 架构分层(Hexagonal + Plugin Registry)

> 模型: Hexagonal (Ports & Adapters) + Plugin Registry
> 适用范围: backend/ 全部代码

---

## 1. 分层总图(C4-L2 容器视角)

```
┌──────────────────────────────────────────────────────────────────────┐
│                        ServerHub Backend (Go)                         │
│                                                                       │
│  ┌──────────────┐                                                     │
│  │   api/       │   HTTP/WS 入口,只编排 usecase + 解析参数             │
│  │  (Gin)       │   零业务、零 SQL、零 IO                              │
│  └──────┬───────┘                                                     │
│         │ calls                                                       │
│         ▼                                                             │
│  ┌──────────────┐    ┌──────────────────┐   ┌─────────────────────┐  │
│  │  usecase/    │───▶│      core/       │◀──│      adapters/      │  │
│  │  (业务编排)   │    │  (端口/接口)       │   │   (插件实现 init   │  │
│  │              │    │                  │   │     自注册)         │  │
│  │ 跨 repo 事务 │    │ runtime / source │   │ runtime/{docker,    │  │
│  │ 跨 adapter   │    │ ingress / notify │   │   compose,native,   │  │
│  │ 状态机推进   │    │ probe            │   │   static}/          │  │
│  └──┬───────┬───┘    └──────────────────┘   │ source/{docker,     │  │
│     │       │                               │   compose,nginx,    │  │
│     │       │                               │   systemd}/         │  │
│     ▼       ▼                               │ ingress/nginx/      │  │
│  ┌──────┐ ┌────────┐                        │ notify/{webhook,    │  │
│  │ repo/│ │derive/ │  派生层(纯计算,        │   email,...}/       │  │
│  │ GORM │ │ 摘要   │   不存数据)             └─────────────────────┘  │
│  │ 收口 │ │        │                                                  │
│  └──┬───┘ └────────┘                                                 │
│     │                                                                 │
│     ▼                                                                 │
│  ┌──────────────┐    ┌──────────────────┐   ┌─────────────────────┐  │
│  │  domain/     │    │      model/      │   │      infra/         │  │
│  │  纯领域      │◀──▶│  GORM tag        │   │ runner/sshpool/     │  │
│  │  + 状态机    │    │  持久化结构       │   │ sftppool/safeshell/ │  │
│  │  + 不变量    │    │  (DB schema)     │   │ resp/sse/...        │  │
│  └──────────────┘    └──────────────────┘   └─────────────────────┘  │
│                            │                                          │
│                            ▼                                          │
│                     ┌──────────────┐                                  │
│                     │   SQLite     │                                  │
│                     │  (WAL,单写)  │                                  │
│                     └──────────────┘                                  │
└──────────────────────────────────────────────────────────────────────┘
```

## 2. 各层职责与禁忌

| 层 | 职责 | 不允许 |
|---|---|---|
| **api/** | 解析 HTTP 参数、调 usecase、resp.JSON | 写 SQL、调 adapter、跨 repo 事务 |
| **usecase/** | 业务编排、跨 repo 事务、推进状态机、调用 adapter | 直接 db.xxx、构造 HTTP 响应、知道 Gin |
| **domain/** | 实体定义、状态机 transition 表、不变量校验 | import gorm、import 任何 infra |
| **repo/** | GORM 调用、SQL 拼接、批量 ListByIDs | 业务逻辑、状态推进、调 adapter |
| **derive/** | 从底表派生摘要(批量 + 单次 SQL) | 写入操作、状态推进 |
| **core/** | 端口接口、Registry、Step/Status 等共享类型 | 任何具体实现、import adapters |
| **adapters/** | 实现 core 接口、init() 自注册 | 跨 adapter 调用、写 repo(事务由 usecase 持有) |
| **infra/** | 通用基础设施(SSH/SFTP/runner/resp 等) | 业务逻辑、领域知识 |
| **model/** | GORM tag 持久化结构 + TableName | 业务方法(只允许 BeforeSave/AfterFind 钩子) |
| **migration/** | 版本化迁移函数 | 调 model 当前结构(用 snapshot struct) |

## 3. 依赖方向(单向)

```
api ─▶ usecase ─▶ {repo, derive, core, domain}
                              │
                              ▼
                          adapters (init 注册到 core/Registry)
                              │
                              ▼
                          infra
```

铁律:
- 任何 import 反向都是 PR 拒绝项
- adapters 之间互不 import
- domain 不 import 任何下游(纯净)
- model 仅允许在 repo/migration 内 import

## 4. 端口(Port)接口冻结

> 只列接口,实现见 [05-extension-points.md](./05-extension-points.md)

### 4.1 RuntimeAdapter

```go
// core/runtime/adapter.go
package runtime

type Adapter interface {
    Kind() string
    PlanStart(svc *domain.Service, rel *domain.Release) ([]Step, error)
    BuildStartCmd(svc *domain.Service, rel *domain.Release) (string, error)
    Probe(ctx context.Context, r infra.Runner, svc *domain.Service) (Status, error)
    Stop(ctx context.Context, r infra.Runner, svc *domain.Service) error
}
```

### 4.2 SourceScanner

```go
// core/source/scanner.go
type Scanner interface {
    Kind() string
    Discover(ctx context.Context, r infra.Runner) ([]Candidate, error)
    Fingerprint(c Candidate) string
    Takeover(ctx context.Context, tc TakeoverContext) error
}
```

### 4.3 IngressBackend

```go
// core/ingress/backend.go
type Backend interface {
    Kind() string
    Render(routes []domain.IngressRoute) (string, error)
    Validate(ctx context.Context, r infra.Runner) error
    Reload(ctx context.Context, r infra.Runner, server *domain.Server) error
}
```

### 4.4 NotifyChannel

```go
// core/notify/channel.go
type Channel interface {
    Kind() string
    Send(ctx context.Context, msg Message) error
}
```

### 4.5 HealthProber(可选,Phase R5)

```go
// core/probe/prober.go
type Prober interface {
    Kind() string
    Check(ctx context.Context, target Target) (Status, error)
}
```

## 5. Registry 模式

```go
// core/runtime/registry.go
type Registry struct {
    mu sync.RWMutex
    m  map[string]Adapter
}

var Default = &Registry{m: map[string]Adapter{}}

func (r *Registry) Register(a Adapter)            // 重复注册 panic
func (r *Registry) Get(kind string) (Adapter, error)
func (r *Registry) MustGet(kind string) Adapter   // 找不到 panic
func (r *Registry) All() []Adapter                // discovery 并行扫用

// adapter 自注册:
// adapters/runtime/docker/init.go
func init() { runtime.Default.Register(&Adapter{}) }

// 主入口 import 触发 init:
// cmd/serverhub/main.go
import _ "github.com/.../adapters/runtime/docker"
import _ "github.com/.../adapters/runtime/compose"
// ...
```

设计要点:
- **fail-fast**:重复 Kind 注册直接 panic,启动期暴露,杜绝运行时歧义
- **零反射**:Registry 是 map[string]Adapter,O(1) 查找
- **可测试**:每个 adapter 可独立 _test.go,mock Registry 注入

## 6. 数据流模板(典型业务)

以 `POST /services/:id/apply` 为例:

```
api/service/handler.go::ApplyHandler
    │ 解析 service_id, release_id
    ▼
usecase/deploy.go::ApplyRelease(svcID, relID)
    │
    ├─▶ repo.Service.Get(svcID)            // 取 domain.Service
    ├─▶ repo.Release.Get(relID)            // 取 domain.Release
    ├─▶ svc.State.CanTransitionTo(Syncing) // 状态机校验
    │
    ├─▶ adapter := runtime.MustGet(svc.Type)
    ├─▶ steps := adapter.PlanStart(svc, rel)
    ├─▶ stepEngine.Run(steps)              // infra/stepengine
    │
    ├─▶ repo.DeployRun.Create(...)         // 写审计
    ├─▶ repo.Service.UpdateCurrentRelease(...)
    └─▶ repo.Service.UpdateState(svcID, Synced)
```

## 7. C4 - Level 3(组件视角:usecase/deploy.go)

```
usecase/deploy.go
├── ApplyRelease()      ── 主入口
├── prepareApply()      ── 取 svc/rel,校验状态
├── executeSteps()      ── 跑 adapter 出的 steps
├── recordDeployRun()   ── 写审计 + 状态推进
└── handleFailure()     ── 失败时检查 AutoRollbackOnFail,触发上一条 active release 重放
```

## 8. 性能边界

| 关键路径 | 性能保证 |
|---|---|
| listServices(N 个) | 1 主查 + 2 派生查,O(N log N) |
| reconcile 单次 | 1 adapter dispatch + 1 probe + ≤2 SQL |
| takeover 步骤执行 | Step 引擎不变,N 步骤 = N 命令 |
| Discovery 并行扫描 | Registry.All() goroutine fanout,error 聚合 |
| 派生字段批量 | 必须有 ListByIDs 接口,严禁 N+1 |

红线:任何 PR 不得引入 N+1 查询。CI 加 sqlite 慢日志门禁(后续)。

# 05 — 拓展点契约

> 范围: core/ 全部端口接口 + Registry 实现 + 5 类拓展场景的逐文件改动清单
> 承诺: "加一个 runtime/source/ingress/notify = 改 1 个文件" 不是口号,本文给出 PR diff 级证据

---

## 1. 端口接口完整契约

### 1.1 RuntimeAdapter (`core/runtime/adapter.go`)

```go
package runtime

import (
    "context"
    "github.com/.../domain"
    "github.com/.../infra"
)

// Status 是 Probe 返回的运行时状态快照(派生自远端真实状态)
type Status struct {
    Running   bool      // 进程/容器是否在跑
    Healthy   bool      // 健康检查通过
    StartedAt time.Time // 起算时间(可选)
    Image     string    // 当前实际镜像(docker/compose 才有)
    PID       int       // 进程 ID(native/systemd 才有)
    Extra     map[string]string // adapter 自定义 KV
}

// Step 由 PlanStart 产出,交给 infra/stepengine 顺序执行
type Step interface {
    Name() string
    Do(ctx context.Context) error
    Undo(ctx context.Context) error  // 失败时反向执行
}

// Adapter 是运行时适配器的核心契约
//
// 实现要求:
// 1. 必须线程安全(Probe 可能并发调用)
// 2. PlanStart 是纯函数:同 svc+rel 必须产出等价 steps
// 3. Probe 不能有副作用(只读远端)
// 4. Stop 应幂等:对已停止的服务不报错
type Adapter interface {
    // Kind 返回唯一标识,用于 Registry 注册
    // 例: "docker" / "compose" / "native" / "static"
    Kind() string

    // PlanStart 根据 Service+Release 产出启动步骤链
    // err: 配置不合法 / StartSpec.Kind 不匹配
    PlanStart(svc *domain.Service, rel *domain.Release) ([]Step, error)

    // BuildStartCmd 用于 takeover 物化(把候选写成可执行命令)
    BuildStartCmd(svc *domain.Service, rel *domain.Release) (string, error)

    // Probe 探活,reconciler 周期调用
    // 远端不可达应返回 Status{Running:false} + nil err
    // 仅当本地参数错误时返回 err
    Probe(ctx context.Context, r infra.Runner, svc *domain.Service) (Status, error)

    // Stop 优雅停止
    Stop(ctx context.Context, r infra.Runner, svc *domain.Service) error
}
```

**错误语义**:
- 返回 `*RemoteError`: 远端命令失败(usecase 层包装为 500)
- 返回 `*ConfigError`: StartSpec/Service 配置不合法(usecase 层包装为 422)
- 返回 `context.Canceled`: 调用方取消,直接上抛

### 1.2 SourceScanner (`core/source/scanner.go`)

```go
package source

import (
    "context"
    "github.com/.../domain"
    "github.com/.../infra"
)

// Candidate 是发现的候选服务(尚未物化为 domain.Service)
type Candidate struct {
    Kind        string            // = Scanner.Kind()
    Name        string            // 候选名(用户可改)
    Image       string            // docker 才有
    Cmd         string            // native/systemd 才有
    ConfigFiles []string          // 远端绝对路径
    Suggested   SuggestedFields   // 建议填充到 Service 的字段
    Raw         map[string]string // adapter 自定义元数据
}

type SuggestedFields struct {
    EnvVars  map[string]string
    Ports    []string
    Volumes  []string
    Workdir  string
}

// TakeoverContext 由 usecase 组装,传给 Takeover
type TakeoverContext struct {
    Server   *domain.Server
    Runner   infra.Runner
    Cand     Candidate
    AppID    *uint  // 若用户接管时绑定 application
    SvcName  string // 用户最终选定的名字
}

type Scanner interface {
    Kind() string

    // Discover 在 server 上扫所有候选
    // 远端不可达返回 (nil, err); 没有候选返回 (nil, nil)
    Discover(ctx context.Context, r infra.Runner) ([]Candidate, error)

    // Fingerprint 用于去重:同物理服务在不同 server 上 SHA1 必须一致
    // 必须是纯函数,不调远端
    Fingerprint(c Candidate) string

    // Takeover 把候选物化为 Service(写远端配置 + 调 stepEngine)
    // 失败时 stepEngine 会回滚远端副作用,Scanner 不需关心 repo 写入
    Takeover(ctx context.Context, tc TakeoverContext) error
}
```

### 1.3 IngressBackend (`core/ingress/backend.go`)

```go
package ingress

import (
    "context"
    "github.com/.../domain"
    "github.com/.../infra"
)

type Backend interface {
    Kind() string

    // Render 把路由列表渲染为 backend 配置文件文本
    // 纯函数,不调远端
    Render(routes []domain.IngressRoute) (string, error)

    // Validate 在远端检查配置合法性(eg `nginx -t`)
    Validate(ctx context.Context, r infra.Runner) error

    // Reload 让 backend 重载(eg `systemctl reload nginx`)
    Reload(ctx context.Context, r infra.Runner, server *domain.Server) error
}
```

### 1.4 NotifyChannel (`core/notify/channel.go`)

```go
package notify

import "context"

type Severity string
const (
    SeverityInfo  Severity = "info"
    SeverityWarn  Severity = "warn"
    SeverityError Severity = "error"
)

type Message struct {
    Severity Severity
    Title    string
    Body     string
    Tags     map[string]string
}

type Channel interface {
    Kind() string

    // Send 发送一次通知
    // 实现必须 context-aware,超时返回 err
    // 不应 retry(由 usecase 层决定重试策略)
    Send(ctx context.Context, msg Message) error
}
```

### 1.5 HealthProber (`core/probe/prober.go`,Phase R5)

```go
package probe

import "context"

type Target struct {
    URL     string            // http(s)://...
    Method  string            // GET/POST
    Headers map[string]string
    Body    []byte
    Expect  ExpectRule
}

type ExpectRule struct {
    StatusCode int    // 0 = 任意 2xx
    BodyRegex  string // 空 = 不校验
}

type Status struct {
    Healthy  bool
    Latency  time.Duration
    HTTPCode int
    Error    string
}

type Prober interface {
    Kind() string  // "http" / "tcp" / "exec"
    Check(ctx context.Context, target Target) (Status, error)
}
```

## 2. Registry 实现规范

```go
// core/<port>/registry.go (4 个端口同模板)

package runtime  // or source / ingress / notify

import (
    "fmt"
    "sync"
)

type Registry struct {
    mu sync.RWMutex
    m  map[string]Adapter  // 4 个 registry 改 Adapter 即可
}

var Default = &Registry{m: map[string]Adapter{}}

// Register 重复注册 Kind 直接 panic(启动期暴露)
func (r *Registry) Register(a Adapter) {
    r.mu.Lock()
    defer r.mu.Unlock()
    kind := a.Kind()
    if kind == "" {
        panic(fmt.Sprintf("%T: Kind() returns empty", a))
    }
    if _, dup := r.m[kind]; dup {
        panic(fmt.Sprintf("runtime: duplicate Kind %q registered", kind))
    }
    r.m[kind] = a
}

func (r *Registry) Get(kind string) (Adapter, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    a, ok := r.m[kind]
    if !ok {
        return nil, fmt.Errorf("runtime: kind %q not registered", kind)
    }
    return a, nil
}

func (r *Registry) MustGet(kind string) Adapter {
    a, err := r.Get(kind)
    if err != nil {
        panic(err)  // usecase 已在外层校验过 svc.Type 合法性,这里 panic 表示 bug
    }
    return a
}

func (r *Registry) All() []Adapter {
    r.mu.RLock()
    defer r.mu.RUnlock()
    out := make([]Adapter, 0, len(r.m))
    for _, a := range r.m {
        out = append(out, a)
    }
    return out
}

func (r *Registry) Kinds() []string {
    r.mu.RLock()
    defer r.mu.RUnlock()
    ks := make([]string, 0, len(r.m))
    for k := range r.m {
        ks = append(ks, k)
    }
    sort.Strings(ks)
    return ks
}
```

**注册时机**:`adapters/<port>/<kind>/init.go`

```go
package docker

import "github.com/.../core/runtime"

func init() {
    runtime.Default.Register(&Adapter{})
}
```

**触发 init**:`cmd/serverhub/main.go` 顶部 blank import

```go
import (
    _ "github.com/.../adapters/runtime/docker"
    _ "github.com/.../adapters/runtime/compose"
    _ "github.com/.../adapters/runtime/native"
    _ "github.com/.../adapters/runtime/static"

    _ "github.com/.../adapters/source/docker"
    _ "github.com/.../adapters/source/compose"
    _ "github.com/.../adapters/source/nginx"
    _ "github.com/.../adapters/source/systemd"

    _ "github.com/.../adapters/ingress/nginx"

    _ "github.com/.../adapters/notify/webhook"
    _ "github.com/.../adapters/notify/email"
)
```

## 3. 拓展场景 — 逐文件 diff 清单

### 3.1 加 Runtime: Kubernetes

新增 1 个目录 `adapters/runtime/k8s/`:

```
adapters/runtime/k8s/
├── init.go         (5 行: func init() { runtime.Default.Register(&Adapter{}) })
├── adapter.go      (实现 5 个接口方法)
├── steps.go        (定义 ApplyManifestStep / DeleteStep 等)
└── adapter_test.go (table-driven 单测)
```

**main.go 改动**: 加一行 blank import
```go
import _ "github.com/.../adapters/runtime/k8s"
```

**domain 改动**: `domain/service.go` 的 `ServiceType` 加常量
```go
const ServiceTypeK8s ServiceType = "k8s"
```

**StartSpec 改动**: `domain/startspec.go` 加 `K8sSpec` + 在 `UnmarshalStartSpec` 加 case

**总计**: 4 个新文件 + 3 行存量改动。**零 handler / repo / usecase 改动**。

### 3.2 加 Source: k3s/portainer

```
adapters/source/k3s/
├── init.go
├── scanner.go      (Discover/Fingerprint/Takeover)
└── scanner_test.go
```

**main.go**: 一行 blank import

**触发**: usecase/discovery 已经 `for _, s := range source.Default.All()` 并行扫,无需改 usecase。

### 3.3 加 Ingress: Caddy

```
adapters/ingress/caddy/
├── init.go
├── backend.go      (Render/Validate/Reload)
└── backend_test.go
```

**Application 配置**: `domain/application.go` 的 `IngressBackend string` 加合法值 `"caddy"`,usecase/ingress 通过 `ingress.MustGet(app.IngressBackend)` 拿后端,零改动。

### 3.4 加 Notify: 飞书

```
adapters/notify/feishu/
├── init.go
├── channel.go      (Send 实现)
└── channel_test.go
```

**Settings UI**: 前端加一个 channel 类型选项,后端 `model/notify_config.go` 已经是 `Type string + Config JSON` 的多态结构,无需改。

### 3.5 加派生字段: Service.LastFailureReason

```
derive/service.go::LastFailureReason(repo, svcIDs []uint) map[uint]string
    └─ 1 条 SQL: SELECT service_id, log FROM deploy_runs
                 WHERE status='failed' AND id IN (SELECT max(id) ... GROUP BY service_id)
```

**展示**: `usecase/service.go::ListWithDetails` 加一行 `merge(views, derive.LastFailureReason(repo, ids))`

**总计**: 1 个新文件 + 1 行存量改动。

### 3.6 加业务用例: 灰度发布(Canary)

```
usecase/canary.go
├── StartCanary(svcID, newRelID, percent uint)
├── PromoteCanary(svcID)
└── AbortCanary(svcID)

api/canary/handler.go (3 个 handler 各 5 行)
```

**Service 改动**: 加一个 `CanaryReleaseID *uint` 字段(走 migration),usecase/deploy 在 reconcile 时优先取 canary。

## 4. 反模式(PR 拒绝项)

| 反模式 | 为什么拒绝 |
|---|---|
| adapter 内 import 另一个 adapter | 破坏插件独立性,Registry 失效 |
| adapter 内 import repo/ | adapter 应纯函数,业务编排在 usecase |
| usecase 直接 import gorm.io/gorm | 必须走 repo/ 收口 |
| handler 直接 import adapters/ | 必须走 usecase → core/Registry |
| domain/ import 任何下游(repo/usecase/adapters) | domain 必须纯净,只 import 标准库 |
| Registry.Register 在非 init() 内调用 | 启动后注册有竞态,必须 init |
| 新增 model 字段不写 migration | 启动期 AutoMigrate 不靠谱(P-X 教训) |
| 派生字段写回 model 表 | 真值/派生分离铁律(R3 之后) |

## 5. 接口冻结承诺

本文档列出的 5 个端口接口(Adapter/Scanner/Backend/Channel/Prober)**签名冻结**。任何后续修改:

- 加方法 → 必须有默认空实现 mixin,杜绝破坏存量 adapter
- 改签名 → 走 RFC,所有 adapter 同步迁移,**不允许半迁移状态**

这是对外承诺的核心:**插件作者一次写完,平台升级不踩雷**。

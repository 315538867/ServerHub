// Package runtime 是 RuntimeAdapter 端口包。
//
// 端口契约见 docs/architecture/v2/05-extension-points.md §1.1。
// adapter 实现入驻 backend/adapters/runtime/<kind>/(R2 起)。
package runtime

import (
	"context"
	"time"

	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/infra"
)

// Status 是 Probe 返回的运行时状态快照(派生自远端真实状态)。
type Status struct {
	Running   bool
	Healthy   bool
	StartedAt time.Time
	Image     string            // docker/compose 才有
	PID       int               // native/systemd 才有
	Extra     map[string]string // adapter 自定义 KV
}

// Step 由 PlanStart 产出,交给 infra/stepengine 顺序执行。
type Step interface {
	Name() string
	Do(ctx context.Context) error
	Undo(ctx context.Context) error // 失败时反向执行
}

// Adapter 是运行时适配器的核心契约。
//
// 实现要求:
//  1. 必须线程安全(Probe 可能并发调用)
//  2. PlanStart 是纯函数:同 svc+rel 必须产出等价 steps
//  3. Probe 不能有副作用(只读远端)
//  4. Stop 应幂等:对已停止的服务不报错
//
// 错误语义:
//   - *RemoteError: 远端命令失败(usecase 包装 500)
//   - *ConfigError: StartSpec/Service 配置不合法(usecase 包装 422)
//   - context.Canceled: 调用方取消,直接上抛
type Adapter interface {
	// Kind 返回唯一标识,用于 Registry 注册。
	// 例: "docker" / "compose" / "native" / "static"
	Kind() string

	// PlanStart 根据 Service+Release 产出启动步骤链。
	// err: 配置不合法 / StartSpec.Kind 不匹配
	PlanStart(svc *domain.Service, rel *domain.Release) ([]Step, error)

	// BuildStartCmd 用于 takeover 物化(把候选写成可执行命令)。
	BuildStartCmd(svc *domain.Service, rel *domain.Release) (string, error)

	// Probe 探活,reconciler 周期调用。
	// 远端不可达应返回 Status{Running:false} + nil err;
	// 仅当本地参数错误时返回 err。
	Probe(ctx context.Context, r infra.Runner, svc *domain.Service) (Status, error)

	// Stop 优雅停止。
	Stop(ctx context.Context, r infra.Runner, svc *domain.Service) error
}

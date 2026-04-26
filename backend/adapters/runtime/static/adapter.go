// Package static 实现 RuntimeAdapter 端口的 static(纯静态资源)运行时。
//
// 命令拼装行为与 v1 pkg/deployer.buildStartPart(case ServiceTypeStatic) 字节级等价:
// 不启动独立进程,由上层 nginx 指向 workdir 提供服务,启动阶段仅打印一行确认。
package static

import (
	"context"
	"fmt"

	"github.com/serverhub/serverhub/core/runtime"
	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/infra"
)

// Adapter 是 static 运行时适配器。
type Adapter struct{}

// Kind 返回 "static"。
func (Adapter) Kind() string { return string(domain.ServiceTypeStatic) }

// PlanStart 返回单步 BashCmdStep。
func (a Adapter) PlanStart(svc *domain.Service, rel *domain.Release) ([]runtime.Step, error) {
	cmd, err := a.BuildStartCmd(svc, rel)
	if err != nil {
		return nil, err
	}
	return []runtime.Step{&runtime.BashCmdStep{StepName: "static-start", Command: cmd}}, nil
}

// BuildStartCmd 返回固定 "echo 'static release prepared'"。
func (Adapter) BuildStartCmd(_ *domain.Service, _ *domain.Release) (string, error) {
	return "echo 'static release prepared'", nil
}

// Probe 总是返回 Running=true(static 由 nginx 接管,不存在独立进程探活语义)。
func (Adapter) Probe(_ context.Context, _ infra.Runner, svc *domain.Service) (runtime.Status, error) {
	if svc == nil {
		return runtime.Status{}, fmt.Errorf("static adapter: nil svc")
	}
	return runtime.Status{Running: true, Healthy: true}, nil
}

// Stop 是 no-op(static 没有独立进程可停止)。
func (Adapter) Stop(_ context.Context, _ infra.Runner, _ *domain.Service) error {
	return nil
}

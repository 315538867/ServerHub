// Package native 实现 RuntimeAdapter 端口的 native(裸进程)运行时。
//
// 命令拼装行为与 v1 pkg/deployer.buildStartPart(case ServiceTypeNative) 字节级等价:
// 直接执行 StartSpec.cmd 并附加 "2>&1"。
package native

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/serverhub/serverhub/core/runtime"
	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/infra"
	"github.com/serverhub/serverhub/internal/cmdbuild"
)

// Adapter 是 native 运行时适配器。
type Adapter struct{}

// Kind 返回 "native"。
func (Adapter) Kind() string { return string(domain.ServiceTypeNative) }

// PlanStart 返回单步 BashCmdStep。
func (a Adapter) PlanStart(svc *domain.Service, rel *domain.Release) ([]runtime.Step, error) {
	cmd, err := a.BuildStartCmd(svc, rel)
	if err != nil {
		return nil, err
	}
	return []runtime.Step{&runtime.BashCmdStep{StepName: "native-start", Command: cmd}}, nil
}

// BuildStartCmd 返回 "<cmd> 2>&1"。StartSpec.cmd 必填。
func (Adapter) BuildStartCmd(_ *domain.Service, rel *domain.Release) (string, error) {
	if rel == nil {
		return "", fmt.Errorf("native adapter: nil rel")
	}
	cmd := parseStartSpec(rel.StartSpec)["cmd"]
	if cmd == "" {
		return "", errors.New("native start_spec.cmd required")
	}
	return cmd + " 2>&1", nil
}

// Probe 通过 pgrep 命中包含 "serverhub-svc-<id>" 关键字的进程判断运行。
// R2 简化:无强制约束启动命令一定带该关键字,误判时退化为 Running=false。
func (Adapter) Probe(ctx context.Context, r infra.Runner, svc *domain.Service) (runtime.Status, error) {
	if svc == nil {
		return runtime.Status{}, fmt.Errorf("native adapter: nil svc")
	}
	tag := fmt.Sprintf("serverhub-svc-%d", svc.ID)
	stdout, _, err := r.Run(ctx, fmt.Sprintf("pgrep -f %s | head -1", cmdbuild.ShellQuote(tag)))
	if err != nil {
		return runtime.Status{Running: false}, nil
	}
	st := runtime.Status{}
	if pidStr := strings.TrimSpace(stdout); pidStr != "" {
		st.Running = true
		fmt.Sscanf(pidStr, "%d", &st.PID)
	}
	return st, nil
}

// Stop 用 pkill 终结匹配进程(幂等;无匹配进程时退出码 1 不视为错误)。
func (Adapter) Stop(ctx context.Context, r infra.Runner, svc *domain.Service) error {
	if svc == nil {
		return fmt.Errorf("native adapter: nil svc")
	}
	tag := fmt.Sprintf("serverhub-svc-%d", svc.ID)
	_, _, _ = r.Run(ctx, fmt.Sprintf("pkill -f %s || true", cmdbuild.ShellQuote(tag)))
	return nil
}

func parseStartSpec(raw string) map[string]string {
	out := map[string]string{}
	if raw == "" {
		return out
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return out
	}
	for k, v := range m {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}

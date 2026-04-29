// Package podman 实现 RuntimeAdapter 端口的 podman(无 root 容器)运行时。
//
// podman CLI 与 docker 高度兼容,本 adapter 是 docker adapter 的 podman 化变体。
// 注册:由 init.go 自注册到 core/runtime.Default。
//
// 此 adapter 作为 R8 GA 验收的"第三方拓展验证",证明仅需实现 RuntimeAdapter
// port + 注册即可新增运行时支持,无需修改任何 core/usecase/api 代码。
package podman

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/serverhub/serverhub/core/runtime"
	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/infra"
	"github.com/serverhub/serverhub/internal/cmdbuild"
)

// Adapter 是 podman 运行时适配器。
type Adapter struct{}

// Kind 返回 "podman"。
func (Adapter) Kind() string { return string(domain.ServiceTypePodman) }

// PlanStart 返回单步 BashCmdStep。
func (a Adapter) PlanStart(svc *domain.Service, rel *domain.Release) ([]runtime.Step, error) {
	cmd, err := a.BuildStartCmd(svc, rel)
	if err != nil {
		return nil, err
	}
	return []runtime.Step{&runtime.BashCmdStep{StepName: "podman-start", Command: cmd}}, nil
}

// BuildStartCmd 拼装 podman run 命令(与 docker adapter 等价,替换 podman 命令)。
func (Adapter) BuildStartCmd(svc *domain.Service, rel *domain.Release) (string, error) {
	if svc == nil || rel == nil {
		return "", fmt.Errorf("podman adapter: nil svc/rel")
	}
	spec, _ := rel.StartSpec.(*domain.DockerSpec)
	image := ""
	extra := ""
	if spec != nil {
		image = spec.Image
		if spec.Cmd != "" {
			extra = " " + spec.Cmd
		}
	}
	if image == "" {
		image = rel.ArtifactRef
	}
	if image == "" {
		return "", fmt.Errorf("podman adapter: image empty (StartSpec.image and ArtifactRef both blank)")
	}
	name := "serverhub-svc-" + fmt.Sprint(svc.ID)
	return fmt.Sprintf(
		"podman rm -f %s 2>/dev/null || true; podman run -d --name %s %s%s 2>&1",
		cmdbuild.ShellQuote(name), cmdbuild.ShellQuote(name), cmdbuild.ShellQuote(image), extra,
	), nil
}

// Probe 通过 podman inspect 探活。
func (Adapter) Probe(ctx context.Context, r infra.Runner, svc *domain.Service) (runtime.Status, error) {
	if svc == nil {
		return runtime.Status{}, fmt.Errorf("podman adapter: nil svc")
	}
	name := "serverhub-svc-" + fmt.Sprint(svc.ID)
	probe := fmt.Sprintf(
		"podman inspect %s --format '{{.State.Running}}|{{.Config.Image}}|{{.State.StartedAt}}'",
		cmdbuild.ShellQuote(name),
	)
	stdout, _, err := r.Run(ctx, probe)
	if err != nil {
		return runtime.Status{Running: false}, nil
	}
	parts := strings.SplitN(strings.TrimSpace(stdout), "|", 3)
	st := runtime.Status{}
	if len(parts) >= 1 {
		st.Running = parts[0] == "true"
	}
	if len(parts) >= 2 {
		st.Image = parts[1]
	}
	if len(parts) >= 3 {
		if t, perr := time.Parse(time.RFC3339Nano, parts[2]); perr == nil {
			st.StartedAt = t
		}
	}
	return st, nil
}

// Stop 强删容器(幂等)。
func (Adapter) Stop(ctx context.Context, r infra.Runner, svc *domain.Service) error {
	if svc == nil {
		return fmt.Errorf("podman adapter: nil svc")
	}
	name := "serverhub-svc-" + fmt.Sprint(svc.ID)
	_, _, err := r.Run(ctx, fmt.Sprintf("podman rm -f %s", cmdbuild.ShellQuote(name)))
	return err
}

// Package docker 实现 RuntimeAdapter 端口的 docker(单容器)运行时。
//
// 注册:由 init.go 自注册到 core/runtime.Default。
// 命令拼装行为与 v1 pkg/deployer.buildStartPart(case ServiceTypeDocker) 字节级等价。
package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/serverhub/serverhub/core/runtime"
	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/infra"
	"github.com/serverhub/serverhub/internal/cmdbuild"
)

// Adapter 是 docker 单容器运行时适配器。
type Adapter struct{}

// Kind 返回 "docker",对应 model.ServiceType="docker"。
func (Adapter) Kind() string { return string(domain.ServiceTypeDocker) }

// PlanStart 返回单步 BashCmdStep,Cmd 即 BuildStartCmd 输出。
// R6 起拆为 PullImageStep / RunContainerStep。
func (a Adapter) PlanStart(svc *domain.Service, rel *domain.Release) ([]runtime.Step, error) {
	cmd, err := a.BuildStartCmd(svc, rel)
	if err != nil {
		return nil, err
	}
	return []runtime.Step{&runtime.BashCmdStep{StepName: "docker-start", Command: cmd}}, nil
}

// BuildStartCmd 拼装单条 "docker rm -f X || true; docker run -d --name X IMG [extra]"。
//
// 与 v1 行为差异:无。包括:
//   - StartSpec.image 为空时回退 rel.ArtifactRef
//   - StartSpec.cmd 非空时空格前缀拼接到镜像名后
//   - 容器名固定 "serverhub-svc-<svc.ID>"
//   - 末尾 "2>&1"
func (Adapter) BuildStartCmd(svc *domain.Service, rel *domain.Release) (string, error) {
	if svc == nil || rel == nil {
		return "", fmt.Errorf("docker adapter: nil svc/rel")
	}
	spec := parseStartSpec(rel.StartSpec)
	image := spec["image"]
	if image == "" {
		image = rel.ArtifactRef
	}
	if image == "" {
		return "", fmt.Errorf("docker adapter: image empty (StartSpec.image and ArtifactRef both blank)")
	}
	name := "serverhub-svc-" + fmt.Sprint(svc.ID)
	extra := ""
	if c := spec["cmd"]; c != "" {
		extra = " " + c
	}
	return fmt.Sprintf(
		"docker rm -f %s 2>/dev/null || true; docker run -d --name %s %s%s 2>&1",
		cmdbuild.ShellQuote(name), cmdbuild.ShellQuote(name), cmdbuild.ShellQuote(image), extra,
	), nil
}

// Probe 通过 docker inspect 拿运行状态。远端不可达返回 Status{Running:false} + nil。
func (Adapter) Probe(ctx context.Context, r infra.Runner, svc *domain.Service) (runtime.Status, error) {
	if svc == nil {
		return runtime.Status{}, fmt.Errorf("docker adapter: nil svc")
	}
	name := "serverhub-svc-" + fmt.Sprint(svc.ID)
	probe := fmt.Sprintf(
		"docker inspect %s --format '{{.State.Running}}|{{.Config.Image}}|{{.State.StartedAt}}'",
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
		return fmt.Errorf("docker adapter: nil svc")
	}
	name := "serverhub-svc-" + fmt.Sprint(svc.ID)
	_, _, err := r.Run(ctx, fmt.Sprintf("docker rm -f %s", cmdbuild.ShellQuote(name)))
	return err
}

// parseStartSpec 解析 JSON 字符串为 map[string]string,空 / 解析失败返回空 map。
// 仅取 string 类型字段,其它类型字段忽略(与 v1 getStr 等价)。
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

// Package compose 实现 RuntimeAdapter 端口的 docker-compose 运行时。
//
// 注册:由 init.go 自注册到 core/runtime.Default。
// 命令拼装行为与 v1 pkg/deployer.buildStartPart(case ServiceTypeDockerCompose) 字节级等价。
package compose

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/serverhub/serverhub/core/runtime"
	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/infra"
	"github.com/serverhub/serverhub/internal/cmdbuild"
)

// Adapter 是 docker-compose 运行时适配器。
type Adapter struct{}

// Kind 返回 "compose"。注意:model.Service.Type 仍为 "docker-compose";
// usecase 层负责把 svc.Type → adapter Kind 的映射。
func (Adapter) Kind() string { return string(domain.ServiceTypeCompose) }

// PlanStart 返回单步 BashCmdStep,Cmd 即 BuildStartCmd 输出。
func (a Adapter) PlanStart(svc *domain.Service, rel *domain.Release) ([]runtime.Step, error) {
	cmd, err := a.BuildStartCmd(svc, rel)
	if err != nil {
		return nil, err
	}
	return []runtime.Step{&runtime.BashCmdStep{StepName: "compose-start", Command: cmd}}, nil
}

// BuildStartCmd 拼装 "docker compose -f X up -d --build 2>&1"。file_name 缺省 "docker-compose.yml"。
func (Adapter) BuildStartCmd(_ *domain.Service, rel *domain.Release) (string, error) {
	if rel == nil {
		return "", fmt.Errorf("compose adapter: nil rel")
	}
	file := parseStartSpec(rel.StartSpec)["file_name"]
	if file == "" {
		file = "docker-compose.yml"
	}
	return fmt.Sprintf("docker compose -f %s up -d --build 2>&1", cmdbuild.ShellQuote(file)), nil
}

// Probe 用 `docker compose -f X ps -q | wc -l` 判断有无运行中容器。
// file_name 取自 svc.WorkDir / 默认 docker-compose.yml(R2 简化:不解析 rel.StartSpec)。
func (Adapter) Probe(ctx context.Context, r infra.Runner, svc *domain.Service) (runtime.Status, error) {
	if svc == nil {
		return runtime.Status{}, fmt.Errorf("compose adapter: nil svc")
	}
	file := "docker-compose.yml"
	probe := fmt.Sprintf("docker compose -f %s ps -q | wc -l", cmdbuild.ShellQuote(file))
	stdout, _, err := r.Run(ctx, probe)
	if err != nil {
		return runtime.Status{Running: false}, nil
	}
	st := runtime.Status{}
	if n := strings.TrimSpace(stdout); n != "" && n != "0" {
		st.Running = true
	}
	return st, nil
}

// Stop 调用 docker compose down(幂等)。
func (Adapter) Stop(ctx context.Context, r infra.Runner, svc *domain.Service) error {
	if svc == nil {
		return fmt.Errorf("compose adapter: nil svc")
	}
	file := "docker-compose.yml"
	_, _, err := r.Run(ctx, fmt.Sprintf("docker compose -f %s down", cmdbuild.ShellQuote(file)))
	return err
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

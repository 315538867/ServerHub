package docker

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/serverhub/serverhub/core/source"
	"github.com/serverhub/serverhub/infra"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/safeshell"
)

const Kind = "docker"

// Scanner 实现 source.Scanner: 发现独立 docker 容器(有
// com.docker.compose.project label 的归 adapters/source/compose)。
type Scanner struct{}

func (Scanner) Kind() string { return Kind }

// dockerPS 是 `docker ps --format '{{json .}}'` 的一行。
type dockerPS struct {
	ID     string `json:"ID"`
	Names  string `json:"Names"`
	Image  string `json:"Image"`
	Labels string `json:"Labels"`
	Status string `json:"Status"`
}

// Discover 列出所有运行中的独立容器(过滤 compose 项目)。远端不可达
// 或无候选返回 (nil, nil/err)。Discover 不做 fingerprint 命中比对——
// 那是 usecase 在 AlreadyManaged 回填阶段的事。
func (s Scanner) Discover(ctx context.Context, r infra.Runner) ([]source.Candidate, error) {
	stdout, _, err := r.Run(ctx, `docker ps --format '{{json .}}' 2>/dev/null`)
	if err != nil {
		// docker 未安装/未启动等价于"无候选",不污染上层 errors 列表。
		return nil, nil
	}
	stdout = strings.TrimSpace(stdout)
	if stdout == "" {
		return nil, nil
	}

	var out []source.Candidate
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var row dockerPS
		if e := json.Unmarshal([]byte(line), &row); e != nil {
			continue
		}
		labels := parseLabels(row.Labels)
		// compose 子容器跳过——交给 adapters/source/compose 按 project 聚合。
		if labels["com.docker.compose.project"] != "" {
			continue
		}
		env := s.inspectContainerEnv(ctx, r, row.ID)
		binds, ports := s.inspectBindsAndPorts(ctx, r, row.ID)
		envVars := make(map[string]string, len(env))
		envSecrets := make(map[string]bool, len(env))
		for _, kv := range env {
			envVars[kv.Key] = kv.Value
			if kv.Secret {
				envSecrets[kv.Key] = true
			}
		}
		out = append(out, source.Candidate{
			Kind:     Kind,
			SourceID: row.ID,
			Name:     row.Names,
			Image:    row.Image,
			Summary:  row.Image + " (" + row.Status + ")",
			Suggested: source.SuggestedFields{
				Type:       model.ServiceTypeDocker,
				Image:      row.Image,
				EnvVars:    envVars,
				EnvSecrets: envSecrets,
			},
			Raw: map[string]string{
				"binds": binds,
				"ports": ports,
			},
		})
	}
	return out, nil
}

// inspectContainerEnv 读 .Config.Env,过滤 docker 注入的默认变量。
func (Scanner) inspectContainerEnv(ctx context.Context, r infra.Runner, id string) []envKV {
	if id == "" {
		return nil
	}
	stdout, _, err := r.Run(ctx, `docker inspect --format '{{json .Config.Env}}' `+safeshell.Quote(id)+` 2>/dev/null`)
	if err != nil || strings.TrimSpace(stdout) == "" {
		return nil
	}
	var raw []string
	if e := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &raw); e != nil {
		return nil
	}
	pairs := parseKVPairs(raw)
	filtered := pairs[:0]
	for _, kv := range pairs {
		if dockerEnvSkip[kv.Key] {
			continue
		}
		filtered = append(filtered, kv)
	}
	return filtered
}

// inspectBindsAndPorts 抽 fingerprint 需要的 binds + ports 字符串(逗号分隔,
// normalizeList 会再排序)。失败返回空串——指纹仍可计算,只是少两维稳定性。
func (Scanner) inspectBindsAndPorts(ctx context.Context, r infra.Runner, id string) (binds, ports string) {
	if id == "" {
		return "", ""
	}
	bOut, _, _ := r.Run(ctx,
		`docker inspect --format '{{range .Mounts}}{{.Source}}:{{.Destination}},{{end}}' `+
			safeshell.Quote(id)+` 2>/dev/null`)
	pOut, _, _ := r.Run(ctx,
		`docker inspect --format '{{range $k,$v := .HostConfig.PortBindings}}{{range $v}}{{.HostPort}}:{{$k}},{{end}}{{end}}' `+
			safeshell.Quote(id)+` 2>/dev/null`)
	return strings.TrimRight(strings.TrimSpace(bOut), ","),
		strings.TrimRight(strings.TrimSpace(pOut), ",")
}

// Fingerprint: 与 v1 pkg/discovery.Fingerprint(KindDocker) 字节一致,保证
// R3→R4 平移后已落库的 source_fingerprint 仍能命中"已接管"。
func (Scanner) Fingerprint(c source.Candidate) string {
	key := strings.Join([]string{
		"docker",
		c.Suggested.Image,
		c.Suggested.Workdir,
		normalizeList(c.Raw["binds"]),
		normalizeList(c.Raw["ports"]),
	}, "|")
	sum := sha1.Sum([]byte(key))
	return hex.EncodeToString(sum[:])
}

// Takeover 实现见 adapters/source/docker/takeover.go(平移 v1
// pkg/takeover/docker.go 的 step 链到 stepkit.RunSteps)。Scanner 不写
// repo——usecase 在 Takeover 返回 nil 后构造 model.Service。

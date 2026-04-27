package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/serverhub/serverhub/core/source"
	"github.com/serverhub/serverhub/infra"
	"github.com/serverhub/serverhub/internal/stepkit"
	"github.com/serverhub/serverhub/pkg/safeshell"
	"gopkg.in/yaml.v3"
)

// Takeover 把单容器物化为 docker-compose 项目,落到 /opt/serverhub/apps/<name>/。
// 平移自 v1 pkg/takeover/docker.go runDocker,差异:
//   - 用 stepkit.RunSteps 替换 takeover.RunSteps(签名等价)
//   - infra.Runner.Run(ctx, cmd) 替换 pkg/runner.Runner.Run(cmd),context 全程穿透
//   - **不**写 model.Service / Application 绑定——那是 usecase 的事(端口契约
//     scanner.go §3:"Takeover 失败时副作用回滚由 stepEngine 负责,Scanner 不写 repo")
//
// Flow:
//  1. inspect      - 读容器配置快照
//  2. precheck     - 目标目录不存在; bind 源可读
//  3. materialize  - mkdir target + target/data; cp -a binds; 写 compose.yml
//  4. stop+rename  - docker stop <id>; docker rename <id> <name>-pre-takeover-<ts>
//  5. compose up   - cd target && docker compose up -d
//  6. probe        - 所有 service running
//
// 失败任意一步立即 rollback(stepkit 自动逆序调 Undo)。
func (Scanner) Takeover(ctx context.Context, tc source.TakeoverContext) error {
	cand := tc.Cand
	containerID := cand.SourceID
	if containerID == "" {
		return fmt.Errorf("候选缺少 source_id (container id)")
	}
	if err := safeshell.ValidName(tc.SvcName, 64); err != nil {
		return fmt.Errorf("svc_name 非法: %w", err)
	}

	target := stepkit.TargetDir(tc.SvcName)
	ts := stepkit.Timestamp()
	dataDir := target + "/data"
	composePath := target + "/docker-compose.yml"
	preTakeoverName := tc.SvcName + "-pre-takeover-" + ts

	log := &stepkit.Log{}

	var (
		spec      containerSpec
		bindCopy  []bindRewrite
		composeYM string
		oldName   string
	)

	steps := []stepkit.Step{
		{
			Name: "docker inspect 源容器",
			Do: func() error {
				out, err := stepkit.MustRun(ctx, tc.Runner, log,
					"sudo -n docker inspect "+safeshell.Quote(containerID))
				if err != nil {
					return err
				}
				s, err := parseContainerSpec(out)
				if err != nil {
					return err
				}
				spec = s
				oldName = strings.TrimPrefix(spec.Name, "/")
				return nil
			},
		},
		{
			Name: "precheck: 目标不存在 + bind 源可读",
			Do: func() error {
				if err := stepkit.EnsureAbsent(ctx, tc.Runner, target); err != nil {
					return err
				}
				for _, m := range spec.Mounts {
					if m.Type != "bind" {
						continue
					}
					if err := stepkit.EnsureReadable(ctx, tc.Runner, m.Source); err != nil {
						return fmt.Errorf("bind 源不可读 %s: %w", m.Source, err)
					}
				}
				return nil
			},
		},
		{
			Name: "建立目标目录并复制 bind 数据",
			Do: func() error {
				if _, err := stepkit.MustRun(ctx, tc.Runner, log,
					"sudo -n mkdir -p "+safeshell.Quote(dataDir)); err != nil {
					return err
				}
				used := map[string]int{}
				for _, m := range spec.Mounts {
					if m.Type != "bind" {
						continue
					}
					base := uniqueBindName(path.Base(strings.TrimRight(m.Source, "/")), used)
					dst := dataDir + "/" + base
					_, err := stepkit.MustRun(ctx, tc.Runner, log,
						"sudo -n cp -a "+safeshell.Quote(m.Source+"/.")+" "+safeshell.Quote(dst+"/"))
					if err != nil {
						// 文件 vs 目录:先按目录复制失败时回退按文件复制
						_, err = stepkit.MustRun(ctx, tc.Runner, log,
							"sudo -n cp -a "+safeshell.Quote(m.Source)+" "+safeshell.Quote(dst))
					}
					if err != nil {
						return err
					}
					bindCopy = append(bindCopy, bindRewrite{
						Original:    m.Source,
						RelInTarget: "./data/" + base,
						Destination: m.Destination,
						ReadOnly:    !m.RW,
					})
				}
				return nil
			},
			Undo: func() error {
				_, err := stepkit.MustRun(ctx, tc.Runner, log, "sudo -n rm -rf "+safeshell.Quote(target))
				return err
			},
		},
		{
			Name: "生成 docker-compose.yml",
			Do: func() error {
				ym, err := buildComposeYAML(tc.SvcName, spec, bindCopy)
				if err != nil {
					return err
				}
				composeYM = ym
				return stepkit.WriteRemoteFile(ctx, tc.Runner, log, composePath, composeYM)
			},
		},
		{
			Name: "停止并重命名旧容器为 " + preTakeoverName,
			Do: func() error {
				if _, err := stepkit.MustRun(ctx, tc.Runner, log,
					"sudo -n docker stop "+safeshell.Quote(containerID)); err != nil {
					return err
				}
				if _, err := stepkit.MustRun(ctx, tc.Runner, log,
					"sudo -n docker rename "+safeshell.Quote(containerID)+" "+safeshell.Quote(preTakeoverName)); err != nil {
					_, _ = stepkit.MustRun(ctx, tc.Runner, log,
						"sudo -n docker start "+safeshell.Quote(containerID))
					return err
				}
				return nil
			},
			Undo: func() error {
				_, _ = stepkit.MustRun(ctx, tc.Runner, log,
					"sudo -n docker rename "+safeshell.Quote(preTakeoverName)+" "+safeshell.Quote(oldName))
				_, err := stepkit.MustRun(ctx, tc.Runner, log,
					"sudo -n docker start "+safeshell.Quote(oldName))
				return err
			},
		},
		{
			Name: "启动新 compose 栈",
			Do: func() error {
				cmd := fmt.Sprintf("cd %s && sudo -n docker compose up -d", safeshell.Quote(target))
				_, err := stepkit.MustRun(ctx, tc.Runner, log, cmd)
				return err
			},
			Undo: func() error {
				cmd := fmt.Sprintf("cd %s && sudo -n docker compose down", safeshell.Quote(target))
				_, err := stepkit.MustRun(ctx, tc.Runner, log, cmd)
				return err
			},
		},
		{
			Name: "等待新栈就绪",
			Do: func() error {
				return waitComposeRunning(ctx, tc.Runner, log, target, "docker-compose.yml")
			},
		},
	}
	return stepkit.RunSteps(log, steps)
}

// containerSpec 是 takeover 需要的 docker inspect 子集,字段名严格对齐 docker
// JSON 以便直接 json.Unmarshal。
type containerSpec struct {
	Name   string
	Image  string
	Config struct {
		Env          []string
		ExposedPorts map[string]struct{}
		Cmd          []string
		Entrypoint   []string
	}
	HostConfig struct {
		PortBindings map[string][]struct {
			HostIP   string `json:"HostIp"`
			HostPort string `json:"HostPort"`
		}
		RestartPolicy struct {
			Name              string
			MaximumRetryCount int
		}
		NetworkMode string
	}
	Mounts []struct {
		Type        string
		Source      string
		Destination string
		Mode        string
		RW          bool
		Name        string
	}
}

func parseContainerSpec(raw string) (containerSpec, error) {
	var arr []containerSpec
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&arr); err != nil {
		return containerSpec{}, fmt.Errorf("inspect JSON 解析失败: %w", err)
	}
	if len(arr) == 0 {
		return containerSpec{}, fmt.Errorf("inspect 返回空数组")
	}
	return arr[0], nil
}

type bindRewrite struct {
	Original    string
	RelInTarget string
	Destination string
	ReadOnly    bool
}

// uniqueBindName 解决 basename 撞名(两个 bind 源末尾都是 "config" 之类),
// 后撞的追加 "-N"。
func uniqueBindName(base string, used map[string]int) string {
	if base == "" || base == "/" {
		base = "data"
	}
	if used[base] == 0 {
		used[base] = 1
		return base
	}
	used[base]++
	return fmt.Sprintf("%s-%d", base, used[base])
}

// buildComposeYAML 渲染最小 compose v3 文件,基于 typed map 输出确定性 YAML
// 以便人审 + 任意 compose 工具回写。
func buildComposeYAML(serviceName string, s containerSpec, binds []bindRewrite) (string, error) {
	svc := map[string]any{
		"image":          s.Image,
		"container_name": serviceName,
	}
	if len(s.Config.Env) > 0 {
		envs := make([]string, 0, len(s.Config.Env))
		envs = append(envs, s.Config.Env...)
		sort.Strings(envs)
		svc["environment"] = envs
	}
	if len(s.Config.Cmd) > 0 {
		svc["command"] = s.Config.Cmd
	}
	if len(s.Config.Entrypoint) > 0 {
		svc["entrypoint"] = s.Config.Entrypoint
	}
	if rp := s.HostConfig.RestartPolicy.Name; rp != "" && rp != "no" {
		svc["restart"] = rp
	}
	if s.HostConfig.NetworkMode == "host" {
		svc["network_mode"] = "host"
	}
	if pbs := s.HostConfig.PortBindings; len(pbs) > 0 {
		var ports []string
		for cport, bs := range pbs {
			c := cport
			proto := ""
			if i := strings.IndexByte(cport, '/'); i >= 0 {
				c = cport[:i]
				proto = cport[i+1:]
			}
			for _, b := range bs {
				host := b.HostPort
				if host == "" {
					continue
				}
				p := host + ":" + c
				if proto != "" && proto != "tcp" {
					p += "/" + proto
				}
				ports = append(ports, p)
			}
		}
		sort.Strings(ports)
		if len(ports) > 0 {
			svc["ports"] = ports
		}
	}
	if len(binds) > 0 {
		var vols []string
		for _, b := range binds {
			line := b.RelInTarget + ":" + b.Destination
			if b.ReadOnly {
				line += ":ro"
			}
			vols = append(vols, line)
		}
		for _, m := range s.Mounts {
			if m.Type == "volume" && m.Name != "" {
				ln := m.Name + ":" + m.Destination
				if !m.RW {
					ln += ":ro"
				}
				vols = append(vols, ln)
			}
		}
		svc["volumes"] = vols
	}
	root := map[string]any{
		"services": map[string]any{serviceName: svc},
	}
	named := map[string]any{}
	for _, m := range s.Mounts {
		if m.Type == "volume" && m.Name != "" {
			named[m.Name] = map[string]any{"external": true}
		}
	}
	if len(named) > 0 {
		root["volumes"] = named
	}
	out, err := yaml.Marshal(root)
	if err != nil {
		return "", err
	}
	return "# generated by serverhub takeover\n" + string(out), nil
}

// waitComposeRunning 轮询 `docker compose ps --format json` 至 30s 截止;每个
// service 状态为 running(或 one-shot exited 0)即视为就绪。Compose v2 输出
// NDJSON,extractJSONField 用 substring 截 "State"/"Service" 字段以兼容
// v2.20± 的"NDJSON / JSON 数组"两种格式。
//
// 注:本函数与 adapters/source/compose 的 wait 逻辑功能等价,后续 compose
// adapter 落地时会抽到 stepkit 共享。
func waitComposeRunning(ctx context.Context, r infra.Runner, log *stepkit.Log, dir, file string) error {
	deadline := time.Now().Add(30 * time.Second)
	cmd := fmt.Sprintf("cd %s && sudo -n docker compose -f %s ps --format json",
		safeshell.Quote(dir), safeshell.Quote(file))
	for {
		out, err := stepkit.MustRun(ctx, r, log, cmd)
		if err != nil {
			return err
		}
		bad := composeNotReady(out)
		if len(bad) == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("compose 服务未就绪: %s", strings.Join(bad, ", "))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func composeNotReady(jsonOut string) []string {
	jsonOut = strings.TrimSpace(jsonOut)
	if jsonOut == "" {
		return []string{"<no services>"}
	}
	var notReady []string
	for _, line := range strings.Split(jsonOut, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "[" || line == "]" {
			continue
		}
		state := extractJSONField(line, "State")
		name := extractJSONField(line, "Service")
		if name == "" {
			name = extractJSONField(line, "Name")
		}
		if state == "" {
			continue
		}
		if state == "running" || state == "exited" {
			continue
		}
		notReady = append(notReady, name+"="+state)
	}
	return notReady
}

func extractJSONField(line, key string) string {
	needle := `"` + key + `":"`
	i := strings.Index(line, needle)
	if i < 0 {
		return ""
	}
	rest := line[i+len(needle):]
	end := strings.IndexByte(rest, '"')
	if end < 0 {
		return ""
	}
	return rest[:end]
}

package compose

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/serverhub/serverhub/core/source"
	"github.com/serverhub/serverhub/infra"
	"github.com/serverhub/serverhub/internal/stepkit"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/safeshell"
)

const Kind = "compose"

type Scanner struct{}

func (Scanner) Kind() string { return Kind }

type dockerPS struct {
	ID     string `json:"ID"`
	Image  string `json:"Image"`
	Labels string `json:"Labels"`
}

// Discover 列 docker ps,按 com.docker.compose.project 聚合;每个 project
// 一个 source.Candidate。Suggested.Workdir / ComposeFile 来��� compose
// project labels(working_dir / config_files)。
//
// Fingerprint 输入只取 ComposeFile 绝对路径(v1 算法),所以不必 inspect
// 容器细节——这层 Discover 比 docker adapter 轻得多。
func (s Scanner) Discover(ctx context.Context, r infra.Runner) ([]source.Candidate, error) {
	stdout, _, err := r.Run(ctx, `docker ps --format '{{json .}}' 2>/dev/null`)
	if err != nil {
		return nil, nil
	}
	stdout = strings.TrimSpace(stdout)
	if stdout == "" {
		return nil, nil
	}
	type group struct {
		project    string
		workingDir string
		file       string
		images     []string
		services   []string
	}
	groups := map[string]*group{}
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
		project := labels["com.docker.compose.project"]
		if project == "" {
			continue
		}
		g, ok := groups[project]
		if !ok {
			g = &group{
				project:    project,
				workingDir: labels["com.docker.compose.project.working_dir"],
				file:       labels["com.docker.compose.project.config_files"],
			}
			groups[project] = g
		}
		g.images = append(g.images, row.Image)
		if svc := labels["com.docker.compose.service"]; svc != "" {
			g.services = append(g.services, svc)
		}
	}
	out := make([]source.Candidate, 0, len(groups))
	for _, g := range groups {
		// v1 fingerprint 用 basename;保留同行为
		cfBasename := g.file
		if idx := strings.LastIndex(cfBasename, "/"); idx >= 0 {
			cfBasename = cfBasename[idx+1:]
		}
		if cfBasename == "" {
			cfBasename = "docker-compose.yml"
		}
		out = append(out, source.Candidate{
			Kind:     Kind,
			SourceID: g.project,
			Name:     g.project,
			Summary:  strings.Join(dedup(g.services), ", "),
			Suggested: source.SuggestedFields{
				Type:        model.ServiceTypeDockerCompose,
				Workdir:     g.workingDir,
				ComposeFile: cfBasename,
			},
		})
	}
	return out, nil
}

// Fingerprint: sha1("compose|<ComposeFile basename>"),与 v1 字节一致。
func (Scanner) Fingerprint(c source.Candidate) string {
	key := "compose|" + c.Suggested.ComposeFile
	sum := sha1.Sum([]byte(key))
	return hex.EncodeToString(sum[:])
}

// Takeover 把 compose 项目从 src work_dir 拷贝到 /opt/serverhub/apps/<svc>/,
// 平移自 v1 pkg/takeover/compose.go runCompose。Scanner 不写 repo。
//
// Flow:
//  1. precheck         - 源目录 + compose 文件可读
//  2. compose down 源  - 保留 volumes
//  3. cp -a            - source/. → target/
//  4. compose up 目标
//  5. probe            - 30s 内所有 service running
//  6. rename src       - mv source source.serverhub-takeover-<ts>
func (Scanner) Takeover(ctx context.Context, tc source.TakeoverContext) error {
	cand := tc.Cand
	srcDir := strings.TrimRight(cand.Suggested.Workdir, "/")
	composeFile := cand.Suggested.ComposeFile
	if composeFile == "" {
		composeFile = "docker-compose.yml"
	}
	if err := safeshell.AbsPath(srcDir); err != nil {
		return fmt.Errorf("source work_dir 非法: %w", err)
	}
	if strings.ContainsAny(composeFile, "/\\") {
		return fmt.Errorf("compose_file 必须是文件名而非路径: %s", composeFile)
	}
	if err := safeshell.ValidName(tc.SvcName, 64); err != nil {
		return fmt.Errorf("svc_name 非法: %w", err)
	}

	target := stepkit.TargetDir(tc.SvcName)
	ts := stepkit.Timestamp()
	srcCompose := srcDir + "/" + composeFile
	srcBak := srcDir + ".serverhub-takeover-" + ts
	log := &stepkit.Log{}

	steps := []stepkit.Step{
		{
			Name: "precheck: 源目录 + compose 文件可读",
			Do: func() error {
				if err := stepkit.EnsureReadable(ctx, tc.Runner, srcDir); err != nil {
					return err
				}
				return stepkit.EnsureReadable(ctx, tc.Runner, srcCompose)
			},
		},
		{
			Name: "停止源 compose 项目(保留 volumes)",
			Do: func() error {
				cmd := fmt.Sprintf("cd %s && sudo -n docker compose -f %s down",
					safeshell.Quote(srcDir), safeshell.Quote(composeFile))
				_, err := stepkit.MustRun(ctx, tc.Runner, log, cmd)
				return err
			},
			Undo: func() error {
				cmd := fmt.Sprintf("cd %s && sudo -n docker compose -f %s up -d",
					safeshell.Quote(srcDir), safeshell.Quote(composeFile))
				_, err := stepkit.MustRun(ctx, tc.Runner, log, cmd)
				return err
			},
		},
		{
			Name: "复制项目目录到 " + target,
			Do: func() error {
				cmds := []string{
					"sudo -n mkdir -p " + safeshell.Quote(target),
					"sudo -n cp -a " + safeshell.Quote(srcDir+"/.") + " " + safeshell.Quote(target+"/"),
				}
				for _, c := range cmds {
					if _, err := stepkit.MustRun(ctx, tc.Runner, log, c); err != nil {
						return err
					}
				}
				return nil
			},
			Undo: func() error {
				_, err := stepkit.MustRun(ctx, tc.Runner, log, "sudo -n rm -rf "+safeshell.Quote(target))
				return err
			},
		},
		{
			Name: "在新位置启动 compose",
			Do: func() error {
				cmd := fmt.Sprintf("cd %s && sudo -n docker compose -f %s up -d",
					safeshell.Quote(target), safeshell.Quote(composeFile))
				_, err := stepkit.MustRun(ctx, tc.Runner, log, cmd)
				return err
			},
			Undo: func() error {
				cmd := fmt.Sprintf("cd %s && sudo -n docker compose -f %s down",
					safeshell.Quote(target), safeshell.Quote(composeFile))
				_, err := stepkit.MustRun(ctx, tc.Runner, log, cmd)
				return err
			},
		},
		{
			Name: "等待 compose 服务就绪",
			Do: func() error {
				return waitComposeRunning(ctx, tc.Runner, log, target, composeFile)
			},
		},
		{
			Name: "改名源目录为 " + srcBak,
			Do: func() error {
				_, err := stepkit.MustRun(ctx, tc.Runner, log,
					"sudo -n mv "+safeshell.Quote(srcDir)+" "+safeshell.Quote(srcBak))
				return err
			},
			Undo: func() error {
				_, err := stepkit.MustRun(ctx, tc.Runner, log,
					"sudo -n mv "+safeshell.Quote(srcBak)+" "+safeshell.Quote(srcDir))
				return err
			},
		},
	}
	return stepkit.RunSteps(log, steps)
}

func parseLabels(s string) map[string]string {
	m := map[string]string{}
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if eq := strings.IndexByte(p, '='); eq > 0 {
			m[p[:eq]] = p[eq+1:]
		}
	}
	return m
}

func dedup(xs []string) []string {
	seen := make(map[string]struct{}, len(xs))
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		if _, ok := seen[x]; ok {
			continue
		}
		seen[x] = struct{}{}
		out = append(out, x)
	}
	return out
}

// waitComposeRunning / composeNotReady / extractJSONField 与 docker adapter
// 完全一致(逻辑等价)。R4 收尾时会抽到 stepkit 共享。
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

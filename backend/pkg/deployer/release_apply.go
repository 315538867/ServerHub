// Package deployer 的 release_apply.go 实现 Phase M1 的 Release 三维正交模型部署入口。
//
// 与同包 runner.go 的 Run() 并存：旧链路继续服务 model.Service.Type/StartCmd 等
// 直接字段；新链路 ApplyRelease() 走 Release+Artifact+EnvVarSet+ConfigFileSet。
//
// M1 优先支持 docker provider 的完整链路；其他 provider 给出最小 shell 拼装，
// 不做超出文档承诺的能力（M2 再补 git/upload SFTP 推送/script 校验等）。
package deployer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/runner"
	"gorm.io/gorm"
)

// ApplyRelease 对指定 Service 应用指定 Release。
// triggerSource 取值：manual | webhook | schedule | api | auto_rollback。
// onLine 为流式日志回调，可为 nil。
//
// 失败 + Service.AutoRollbackOnFail=true + 存在上一条 active Release 时，
// 自动以 trigger_source=auto_rollback 再 Apply 一次，递归深度上限 1。
func ApplyRelease(db *gorm.DB, cfg *config.Config, serviceID, releaseID uint,
	triggerSource string, onLine func(string)) (*model.DeployRun, error) {
	return applyReleaseDepth(db, cfg, serviceID, releaseID, triggerSource, onLine, 0)
}

func applyReleaseDepth(db *gorm.DB, cfg *config.Config, serviceID, releaseID uint,
	triggerSource string, onLine func(string), depth int) (*model.DeployRun, error) {

	var svc model.Service
	if err := db.First(&svc, serviceID).Error; err != nil {
		return nil, fmt.Errorf("service not found: %w", err)
	}
	var rel model.Release
	if err := db.Where("id = ? AND service_id = ?", releaseID, serviceID).First(&rel).Error; err != nil {
		return nil, fmt.Errorf("release not found: %w", err)
	}
	var art model.Artifact
	if err := db.Where("id = ? AND service_id = ?", rel.ArtifactID, serviceID).First(&art).Error; err != nil {
		return nil, fmt.Errorf("artifact not found: %w", err)
	}
	if art.Provider == model.ArtifactProviderImported {
		return nil, errors.New("imported artifact is read-only and cannot be applied")
	}

	var srv model.Server
	if err := db.First(&srv, svc.ServerID).Error; err != nil {
		return nil, fmt.Errorf("server not found: %w", err)
	}

	// 创建 DeployRun（running）
	run := model.DeployRun{
		ServiceID:     serviceID,
		ReleaseID:     releaseID,
		Status:        model.DeployRunStatusRunning,
		TriggerSource: triggerSource,
		StartedAt:     time.Now(),
	}
	if err := db.Create(&run).Error; err != nil {
		return nil, err
	}

	// 装配 shell 命令
	cmd, buildErr := buildReleaseCmd(svc, rel, art, db, cfg.Security.AESKey)
	if buildErr != nil {
		finishRun(db, &run, false, "build command failed: "+buildErr.Error())
		return &run, buildErr
	}
	if onLine != nil {
		onLine("$ " + cmd)
	}

	rn, err := runner.For(&srv, cfg)
	if err != nil {
		finishRun(db, &run, false, "runner: "+err.Error())
		return &run, err
	}

	output, runErr := execStreaming(rn, cmd, onLine)
	success := runErr == nil
	finishRun(db, &run, success, output)

	if success {
		// 切指针：last_run_at/last_status 已下线，状态由最近一条 DeployRun 派生
		db.Model(&svc).Update("current_release_id", rel.ID)
		// 老 active 标记 archived（避免历史累积太多 active）
		db.Model(&model.Release{}).
			Where("service_id = ? AND id <> ? AND status = ?", serviceID, rel.ID, model.ReleaseStatusActive).
			Update("status", model.ReleaseStatusArchived)
		db.Model(&rel).Update("status", model.ReleaseStatusActive)
		return &run, nil
	}

	if svc.AutoRollbackOnFail && depth < 1 {
		prev := findPrevActiveRelease(db, serviceID, rel.ID)
		if prev != nil {
			rbRun, _ := applyReleaseDepth(db, cfg, serviceID, prev.ID,
				model.TriggerSourceAutoRollback, onLine, depth+1)
			if rbRun != nil {
				// 将本次失败 run 标记 rolled_back，并指向回滚 run
				db.Model(&run).Updates(map[string]any{
					"status":               model.DeployRunStatusRolledBack,
					"rollback_from_run_id": &rbRun.ID,
				})
			}
		}
	}
	return &run, runErr
}

// findPrevActiveRelease 找到该 Service 上一条 active Release（排除 currentReleaseID 本身）。
// 用于自动回滚选目标。
func findPrevActiveRelease(db *gorm.DB, serviceID, excludeID uint) *model.Release {
	var prev model.Release
	err := db.Where("service_id = ? AND id <> ? AND status IN ?",
		serviceID, excludeID, []string{model.ReleaseStatusActive, model.ReleaseStatusArchived}).
		Order("id desc").First(&prev).Error
	if err != nil {
		return nil
	}
	return &prev
}

func finishRun(db *gorm.DB, run *model.DeployRun, success bool, output string) {
	now := time.Now()
	dur := int(now.Sub(run.StartedAt).Seconds())
	status := model.DeployRunStatusSuccess
	if !success {
		status = model.DeployRunStatusFailed
	}
	db.Model(run).Updates(map[string]any{
		"status":       status,
		"finished_at":  &now,
		"duration_sec": dur,
		"output":       output,
	})
	run.Status = status
	run.FinishedAt = &now
	run.DurationSec = dur
	run.Output = output
}

// execStreaming 执行 cmd 并把每行 stdout 通过 onLine 推出，最终聚合为 output。
func execStreaming(rn runner.Runner, cmd string, onLine func(string)) (string, error) {
	out, err := rn.Run(cmd)
	if onLine != nil && out != "" {
		// runner.Run 已聚合输出；流式回调按行简化
		for _, line := range strings.Split(out, "\n") {
			onLine(line)
		}
	}
	_ = context.TODO() // 保留 context 以便 M2 引入超时控制
	return out, err
}

// ── 命令装配 ─────────────────────────────────────────────────────────────

// buildReleaseCmd 把 Release 三维 + Service.Type + Artifact.Provider 装成 bash 命令。
//
// 整体结构（按 set -e 串行）：
//
//	mkdir -p workdir && cd workdir
//	[写 ConfigFileSet 文件]
//	[Artifact 拉取/准备]
//	[启动命令]
//
// EnvVarSet 以 "export K=V" 前缀注入。
func buildReleaseCmd(svc model.Service, rel model.Release, art model.Artifact,
	db *gorm.DB, aesKey string) (string, error) {

	envPrefix, err := buildEnvPrefixFromSet(db, rel.EnvSetID, aesKey)
	if err != nil {
		return "", err
	}

	var parts []string
	wd := svc.WorkDir
	if wd == "" {
		wd = "/tmp/serverhub-svc-" + fmt.Sprint(svc.ID)
	}
	parts = append(parts,
		fmt.Sprintf("mkdir -p %s", shellQuote(wd)),
		fmt.Sprintf("cd %s", shellQuote(wd)),
	)

	// 写配置文件
	if rel.ConfigSetID != nil {
		var cs model.ConfigFileSet
		if err := db.First(&cs, *rel.ConfigSetID).Error; err == nil && cs.Files != "" {
			var files []struct {
				Name       string `json:"name"`
				ContentB64 string `json:"content_b64"`
				Mode       int    `json:"mode"`
			}
			if err := json.Unmarshal([]byte(cs.Files), &files); err == nil {
				for _, f := range files {
					// content_b64 留空时退化为空文件
					content := f.ContentB64
					if content == "" {
						content = base64.StdEncoding.EncodeToString(nil)
					}
					parts = append(parts, fmt.Sprintf(
						"mkdir -p %s && echo %s | base64 -d > %s",
						shellQuote(dirOf(f.Name)),
						shellQuote(content),
						shellQuote(f.Name),
					))
					if f.Mode > 0 {
						parts = append(parts, fmt.Sprintf("chmod %o %s", f.Mode, shellQuote(f.Name)))
					}
				}
			}
		}
	}

	// 制品拉取
	fetchPart, err := buildFetchPart(art, wd)
	if err != nil {
		return "", err
	}
	if fetchPart != "" {
		parts = append(parts, fetchPart)
	}

	// 启动
	startPart, err := buildStartPart(svc, rel, art)
	if err != nil {
		return "", err
	}
	if startPart != "" {
		parts = append(parts, startPart)
	}

	full := envPrefix + "set -e; " + strings.Join(parts, " && ")
	return "bash -c " + shellQuote(full), nil
}

func buildEnvPrefixFromSet(db *gorm.DB, envSetID *uint, aesKey string) (string, error) {
	if envSetID == nil {
		return "", nil
	}
	var es model.EnvVarSet
	if err := db.First(&es, *envSetID).Error; err != nil {
		return "", nil
	}
	if es.Content == "" {
		return "", nil
	}
	dec, err := crypto.Decrypt(es.Content, aesKey)
	if err != nil {
		return "", fmt.Errorf("env set decrypt: %w", err)
	}
	var vars []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal([]byte(dec), &vars); err != nil {
		return "", nil
	}
	var out []string
	for _, v := range vars {
		if v.Key != "" {
			out = append(out, fmt.Sprintf("export %s=%s", shellQuote(v.Key), shellQuote(v.Value)))
		}
	}
	if len(out) == 0 {
		return "", nil
	}
	return strings.Join(out, "; ") + "; ", nil
}

// buildFetchPart 按 provider 生成"把制品弄到 workdir"的 shell 段。
// M1 范围：docker / http / script / upload；upload 暂以"占位 echo"提示用户由
// 面板 SFTP 推送（M2 真正实现 SFTP 推送）。
func buildFetchPart(art model.Artifact, workdir string) (string, error) {
	switch art.Provider {
	case model.ArtifactProviderDocker:
		if art.Ref == "" {
			return "", errors.New("docker artifact ref empty")
		}
		return fmt.Sprintf("docker pull %s 2>&1", shellQuote(art.Ref)), nil
	case model.ArtifactProviderHTTP:
		if art.Ref == "" {
			return "", errors.New("http artifact ref empty")
		}
		dst := "artifact.bin"
		return fmt.Sprintf("curl -fsSL -o %s %s", shellQuote(dst), shellQuote(art.Ref)), nil
	case model.ArtifactProviderScript:
		if art.PullScript == "" {
			return "", errors.New("script artifact pull_script empty")
		}
		return art.PullScript, nil
	case model.ArtifactProviderUpload:
		// M1：依赖外部 SFTP 推送；这里只校验目标文件存在
		if art.Ref == "" {
			return "", errors.New("upload artifact ref empty")
		}
		return fmt.Sprintf("test -f %s || (echo 'upload artifact missing on target; SFTP push not implemented in M1'; exit 1)",
			shellQuote(art.Ref)), nil
	case model.ArtifactProviderGit:
		if art.Ref == "" {
			return "", errors.New("git artifact ref empty (expect 'repo_url' or 'repo_url@ref')")
		}
		repo, ref := parseGitRef(art.Ref)
		// 幂等：目录存在则 fetch+checkout+reset，否则 clone。ref 为空默认跟踪远端 HEAD。
		dir := "src"
		if ref == "" {
			return fmt.Sprintf(
				"if [ -d %s/.git ]; then cd %s && git fetch --all --prune && git reset --hard @{u} && cd ..; else rm -rf %s && git clone --depth 1 %s %s; fi",
				shellQuote(dir), shellQuote(dir), shellQuote(dir), shellQuote(repo), shellQuote(dir),
			), nil
		}
		return fmt.Sprintf(
			"if [ -d %s/.git ]; then cd %s && git fetch --all --prune && git checkout %s && git reset --hard %s && cd ..; else rm -rf %s && git clone %s %s && cd %s && git checkout %s && cd ..; fi",
			shellQuote(dir), shellQuote(dir), shellQuote(ref), shellQuote(ref),
			shellQuote(dir), shellQuote(repo), shellQuote(dir), shellQuote(dir), shellQuote(ref),
		), nil
	}
	return "", fmt.Errorf("unsupported provider: %s", art.Provider)
}

// buildStartPart 按 Service.Type + StartSpec 生成启动命令。
func buildStartPart(svc model.Service, rel model.Release, art model.Artifact) (string, error) {
	var spec map[string]any
	if rel.StartSpec != "" {
		_ = json.Unmarshal([]byte(rel.StartSpec), &spec)
	}
	getStr := func(k string) string {
		if spec == nil {
			return ""
		}
		if v, ok := spec[k].(string); ok {
			return v
		}
		return ""
	}

	switch svc.Type {
	case model.ServiceTypeDocker:
		image := getStr("image")
		if image == "" {
			image = art.Ref
		}
		name := "serverhub-svc-" + fmt.Sprint(svc.ID)
		cmd := getStr("cmd")
		extra := ""
		if cmd != "" {
			extra = " " + cmd
		}
		return fmt.Sprintf(
			"docker rm -f %s 2>/dev/null || true; docker run -d --name %s %s%s 2>&1",
			shellQuote(name), shellQuote(name), shellQuote(image), extra), nil
	case model.ServiceTypeDockerCompose:
		file := getStr("file_name")
		if file == "" {
			file = "docker-compose.yml"
		}
		return fmt.Sprintf("docker compose -f %s up -d --build 2>&1", shellQuote(file)), nil
	case model.ServiceTypeNative:
		cmd := getStr("cmd")
		if cmd == "" {
			return "", errors.New("native start_spec.cmd required")
		}
		return cmd + " 2>&1", nil
	case model.ServiceTypeStatic:
		// static 类型由 nginx 指向 workdir，无独立启动进程
		return "echo 'static release prepared'", nil
	}
	return "", fmt.Errorf("unsupported service type: %s", svc.Type)
}

// parseGitRef 把 Artifact.Ref 切成 (repo, ref)。约定：最后一个 '@' 之后是 ref；
// 若 repo 本身带 '@'（如 git@github.com:foo/bar.git），需显式在 ref 前再加 '@'。
// 兼容：repo_url 不含 '@' 时，ref 为空，表示跟踪远端默认分支。
func parseGitRef(s string) (repo, ref string) {
	// scp 式 SSH 地址不以 ssh:// 开头，形如 user@host:path；此时 '@' 属于 repo 本身。
	// 约定：用户要锁 ref 时，必须把 ref 写在 "#" 之后避免歧义。
	// 为了简单起见：优先按最后一个 '#' 拆分；再按最后一个 '@' 拆分但排除 scp 式地址。
	if i := strings.LastIndex(s, "#"); i >= 0 {
		return s[:i], s[i+1:]
	}
	if !strings.Contains(s, "://") && strings.Contains(s, "@") && strings.Contains(s, ":") {
		// scp 形式，视为整体
		return s, ""
	}
	if i := strings.LastIndex(s, "@"); i >= 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

// dirOf 返回路径的目录部分（兼容相对路径）。
func dirOf(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[:i]
	}
	return "."
}

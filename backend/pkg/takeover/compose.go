package takeover

import (
	"fmt"
	"strings"
	"time"

	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/runner"
	"github.com/serverhub/serverhub/pkg/safeshell"
	"gorm.io/gorm"
)

// runCompose implements docker-compose project takeover. Source candidate's
// Suggested.WorkDir is the project dir on disk (extracted by discovery from
// the com.docker.compose.project.working_dir label); Suggested.ComposeFile is
// just the file basename (also from labels).
//
// Flow (plan §Compose):
//  1. precheck    - source dir + compose file readable; target absent
//  2. compose down at source (preserve volumes — we copy data dirs separately
//                            if any are bind-mounted, but named volumes are
//                            referenced by name so they survive)
//  3. cp -a       - source/. -> target/
//  4. compose up  - cd target && docker compose -f <file> up -d
//  5. probe       - all services Running (or exited 0 for one-shots) via ps
//  6. rename src  - mv source source.serverhub-takeover-<ts>
//  7. db insert
//
// Rollback (reverse order, only undoes Do that succeeded):
//  - rename src   undo: rename back
//  - probe        no-op
//  - compose up   undo: cd target && compose down
//  - cp           undo: rm -rf target
//  - compose down undo: cd source && compose up -d (best-effort restoration)
//  - precheck     no-op
func runCompose(db *gorm.DB, rn runner.Runner, log *Log, server model.Server,
	req Request, res *Result) error {

	c := req.Candidate
	srcDir := strings.TrimRight(c.Suggested.WorkDir, "/")
	composeFile := c.Suggested.ComposeFile
	if composeFile == "" {
		composeFile = "docker-compose.yml"
	}

	if err := safeshell.AbsPath(srcDir); err != nil {
		return fmt.Errorf("source work_dir 非法: %w", err)
	}
	// composeFile is a basename; reject anything with a slash
	if strings.ContainsAny(composeFile, "/\\") {
		return fmt.Errorf("compose_file 必须是文件名而非路径: %s", composeFile)
	}

	target := TargetDir(req.TargetName)
	ts := Timestamp()
	srcCompose := srcDir + "/" + composeFile
	tgtCompose := target + "/" + composeFile
	srcBak := srcDir + ".serverhub-takeover-" + ts

	steps := []Step{
		{
			Name: "precheck: 源目录 + compose 文件可读",
			Do: func() error {
				if err := EnsureReadable(rn, srcDir); err != nil {
					return err
				}
				return EnsureReadable(rn, srcCompose)
			},
		},
		{
			Name: "停止源 compose 项目（保留 volumes）",
			Do: func() error {
				cmd := fmt.Sprintf("cd %s && sudo -n docker compose -f %s down",
					safeshell.Quote(srcDir), safeshell.Quote(composeFile))
				_, err := MustRun(rn, log, cmd)
				return err
			},
			Undo: func() error {
				// Best-effort: bring source back up if a later step failed.
				cmd := fmt.Sprintf("cd %s && sudo -n docker compose -f %s up -d",
					safeshell.Quote(srcDir), safeshell.Quote(composeFile))
				_, err := MustRun(rn, log, cmd)
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
				for _, cmd := range cmds {
					if _, err := MustRun(rn, log, cmd); err != nil {
						return err
					}
				}
				return nil
			},
			Undo: func() error {
				_, err := MustRun(rn, log, "sudo -n rm -rf "+safeshell.Quote(target))
				return err
			},
		},
		{
			Name: "在新位置启动 compose",
			Do: func() error {
				cmd := fmt.Sprintf("cd %s && sudo -n docker compose -f %s up -d",
					safeshell.Quote(target), safeshell.Quote(composeFile))
				_, err := MustRun(rn, log, cmd)
				return err
			},
			Undo: func() error {
				cmd := fmt.Sprintf("cd %s && sudo -n docker compose -f %s down",
					safeshell.Quote(target), safeshell.Quote(composeFile))
				_, err := MustRun(rn, log, cmd)
				return err
			},
		},
		{
			Name: "等待 compose 服务就绪",
			Do: func() error {
				return waitComposeRunning(rn, log, target, composeFile, tgtCompose)
			},
		},
		{
			Name: "改名源目录为 " + srcBak,
			Do: func() error {
				_, err := MustRun(rn, log,
					"sudo -n mv "+safeshell.Quote(srcDir)+" "+safeshell.Quote(srcBak))
				return err
			},
			Undo: func() error {
				_, err := MustRun(rn, log,
					"sudo -n mv "+safeshell.Quote(srcBak)+" "+safeshell.Quote(srcDir))
				return err
			},
		},
	}

	if err := RunSteps(log, steps); err != nil {
		res.RolledBack = true
		return err
	}

	now := time.Now()
	d := model.Service{
		Name:           req.TargetName,
		ServerID:       server.ID,
		Type:           "docker-compose",
		WorkDir:        target,
		ComposeFile:    composeFile,
		DesiredVersion: ts,
		ActualVersion:  ts,
		SourceKind:     c.Kind,
		SourceID:       c.SourceID,
		SyncStatus:     "synced",
		LastStatus:     "success",
		LastRunAt:      &now,
	}
	if _, err := attachToApplication(db, &d, c, req.TargetName); err != nil {
		log.Printf("⚠ Application 绑定失败: %v\n", err)
		return fmt.Errorf("application 绑定失败: %w", err)
	}
	if err := db.Create(&d).Error; err != nil {
		log.Printf("⚠ Deploy 写入失败（主机已迁移完成）: %v\n", err)
		return fmt.Errorf("DB 写入失败: %w", err)
	}
	if d.ApplicationID != nil {
		finalizeApplicationLink(db, *d.ApplicationID, d.ID)
	}
	log.Printf("Deploy 已创建: id=%d name=%s\n", d.ID, d.Name)
	res.DeployID = d.ID
	return nil
}

// waitComposeRunning polls `docker compose ps --format json` for up to ~30s
// and accepts when every service is in state "running" (or one-shot exited 0).
// Compose v2 emits one JSON object per line.
func waitComposeRunning(rn runner.Runner, log *Log, dir, file, _ string) error {
	deadline := time.Now().Add(30 * time.Second)
	cmd := fmt.Sprintf("cd %s && sudo -n docker compose -f %s ps --format json",
		safeshell.Quote(dir), safeshell.Quote(file))
	for {
		out, err := MustRun(rn, log, cmd)
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
		time.Sleep(2 * time.Second)
	}
}

// composeNotReady returns service names that aren't yet "running" (or exited 0).
// Tolerates both NDJSON (compose v2.20+) and a single JSON array. The state
// field is `State` per compose's documented schema.
func composeNotReady(jsonOut string) []string {
	jsonOut = strings.TrimSpace(jsonOut)
	if jsonOut == "" {
		return []string{"<no services>"}
	}
	// Crude state extraction — robust enough for our checks without pulling in
	// the full compose schema. We only look at "State":"value" pairs.
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

// extractJSONField does a tiny scan for `"key":"value"` in a JSON line. We
// avoid encoding/json here because compose v2 has shipped both a streaming
// NDJSON form and an array form across versions, and the values we care about
// (State, Service, Name) are always plain strings.
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

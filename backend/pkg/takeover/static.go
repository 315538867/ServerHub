package takeover

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/runner"
	"github.com/serverhub/serverhub/pkg/safeshell"
	"gorm.io/gorm"
)

// runStatic implements the nginx static-site takeover flow per Phase B plan.
// On success it inserts a Deploy row and sets res.DeployID; on any forward-step
// failure RunSteps walks back the already-completed undos and we mark
// res.RolledBack=true so the caller can present "已自动回滚" status.
//
// The flow (see plan):
//  1. precheck  - target absent, oldRoot readable + has index.html, conf readable
//  2. mkdir     - target/{releases/<ts>,} ; ln -sfn releases/<ts> target/current
//  3. cp -a     - oldRoot/. -> target/releases/<ts>/
//  4. backup    - copy original conf to /opt/serverhub/backups/nginx/<ts>-<base>.conf
//  5. rewrite   - replace root/alias pointing at oldRoot with root <target>/current
//  6. nginx -t  - syntax check (any failure restores conf in undo)
//  7. reload    - nginx -s reload
//  8. probe     - HTTP fetch via --resolve <server_name>:<port>:127.0.0.1
//  9. rename    - mv oldRoot oldRoot.serverhub-takeover-<ts>  (NOT rm — operator audits)
// 10. db        - insert Deploy row
//
// Each mutation registers an Undo; RunSteps invokes them in reverse on failure.
func runStatic(db *gorm.DB, rn runner.Runner, log *Log, server model.Server,
	req Request, res *Result) error {

	c := req.Candidate
	oldRoot := strings.TrimRight(c.Suggested.WorkDir, "/")
	confFile := c.ExtraLabels["config_file"]
	serverName := c.ExtraLabels["server_name"]
	listen := c.ExtraLabels["listen"]

	if err := safeshell.AbsPath(oldRoot); err != nil {
		return fmt.Errorf("oldRoot 非法: %w", err)
	}
	if err := safeshell.AbsPath(confFile); err != nil {
		return fmt.Errorf("config_file 非法: %w", err)
	}
	target := TargetDir(req.TargetName)
	ts := Timestamp()
	releaseDir := target + "/releases/" + ts
	backupDir := BackupDir("nginx")
	confBase := nginxConfBase(confFile)
	backupConf := backupDir + "/" + ts + "-" + confBase

	port := parseListenPort(listen)
	probeName := serverName
	if probeName == "" || probeName == "_" {
		probeName = "localhost"
	}

	// Cached state shared across step closures.
	var (
		origConfBody string
		oldRootBak   string // post-rename path of original root
	)

	steps := []Step{
		{
			Name: "precheck: oldRoot 可读 + index.html 存在",
			Do: func() error {
				if err := EnsureReadable(rn, oldRoot); err != nil {
					return err
				}
				out, _ := rn.Run("test -f " + safeshell.Quote(oldRoot+"/index.html") + " && echo ok")
				if !strings.Contains(out, "ok") {
					return fmt.Errorf("%s/index.html 不存在", oldRoot)
				}
				return EnsureReadable(rn, confFile)
			},
		},
		{
			Name: "读取并备份 nginx 配置",
			Do: func() error {
				out, err := MustRun(rn, log, "cat "+safeshell.Quote(confFile))
				if err != nil {
					return err
				}
				origConfBody = out
				if _, err := MustRun(rn, log, "sudo -n mkdir -p "+safeshell.Quote(backupDir)); err != nil {
					return err
				}
				cmd := safeshell.WriteRemoteFile(backupConf, origConfBody, true)
				if _, err := MustRun(rn, log, cmd); err != nil {
					return err
				}
				return nil
			},
			Undo: func() error {
				_, err := MustRun(rn, log, "sudo -n rm -f "+safeshell.Quote(backupConf))
				return err
			},
		},
		{
			Name: "创建标准目录并复制内容",
			Do: func() error {
				cmds := []string{
					"sudo -n mkdir -p " + safeshell.Quote(releaseDir),
					"sudo -n cp -a " + safeshell.Quote(oldRoot+"/.") + " " + safeshell.Quote(releaseDir+"/"),
					"sudo -n ln -sfn " + safeshell.Quote("releases/"+ts) + " " + safeshell.Quote(target+"/current"),
					// nginx (www-data/nginx) needs read access; reuse oldRoot owner if possible,
					// but at minimum make it world-readable.
					"sudo -n chmod -R a+rX " + safeshell.Quote(target),
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
			Name: "改写 nginx 配置: root -> " + target + "/current",
			Do: func() error {
				newBody, hits, err := NginxRewrite(origConfBody, oldRoot, target+"/current")
				if err != nil {
					return err
				}
				log.Printf("替换 %d 处 root/alias 指令\n", hits)
				cmd := safeshell.WriteRemoteFile(confFile, newBody, true)
				if _, err := MustRun(rn, log, cmd); err != nil {
					return err
				}
				return nil
			},
			Undo: func() error {
				cmd := safeshell.WriteRemoteFile(confFile, origConfBody, true)
				if _, err := MustRun(rn, log, cmd); err != nil {
					return err
				}
				// Reload (or start, if nginx is stopped) to actually un-do the
				// change in the running nginx.
				_ = nginxReloadOrStart(rn, log)
				return nil
			},
		},
		{
			Name: "nginx -t 校验",
			Do: func() error {
				_, err := MustRun(rn, log, "sudo -n nginx -t")
				return err
			},
		},
		{
			Name: "nginx reload (停机则 systemctl start)",
			Do: func() error {
				return nginxReloadOrStart(rn, log)
			},
			Undo: func() error {
				// Conf already restored by the rewrite step's undo; just reload.
				return nginxReloadOrStart(rn, log)
			},
		},
		{
			Name: "HTTP 探活 " + probeName,
			Do: func() error {
				return ProbeHTTP(rn, log, probeName, port)
			},
		},
		{
			Name: "改名原目录为 " + oldRoot + ".serverhub-takeover-<ts>",
			Do: func() error {
				oldRootBak = oldRoot + ".serverhub-takeover-" + ts
				_, err := MustRun(rn, log,
					"sudo -n mv "+safeshell.Quote(oldRoot)+" "+safeshell.Quote(oldRootBak))
				return err
			},
			Undo: func() error {
				if oldRootBak == "" {
					return nil
				}
				_, err := MustRun(rn, log,
					"sudo -n mv "+safeshell.Quote(oldRootBak)+" "+safeshell.Quote(oldRoot))
				return err
			},
		},
	}

	if err := RunSteps(log, steps); err != nil {
		res.RolledBack = true
		return err
	}

	// All host-side mutations succeeded; create the DB row. If this fails we do
	// NOT roll back the host changes — the files are correct, the operator can
	// retry the import from the UI without needing another takeover dance.
	now := time.Now()
	d := model.Service{
		Name:           req.TargetName,
		ServerID:       server.ID,
		Type:           "static",
		WorkDir:        target,
		SourceKind:     c.Kind,
		SourceID:       c.SourceID,
		SyncStatus:     "synced",
		LastStatus:     "success",
		LastRunAt:      &now,
	}
	if _, err := attachToApplication(db, &d, c, req); err != nil {
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

// nginxConfBase returns the basename for use in backup file names. We strip the
// directory portion but keep the extension so the file looks familiar to an
// operator opening the backups dir.
func nginxConfBase(path string) string {
	i := strings.LastIndexByte(path, '/')
	if i < 0 {
		return path
	}
	return path[i+1:]
}

// parseListenPort extracts a numeric port from an nginx `listen` directive
// value. Handles plain "80", "443 ssl", "[::]:8080", "0.0.0.0:8443 ssl".
// Returns 80 as a sane default when nothing parseable is found.
func parseListenPort(listen string) int {
	listen = strings.TrimSpace(listen)
	if listen == "" {
		return 80
	}
	first := strings.Fields(listen)[0]
	// strip [::] / 0.0.0.0 prefix
	if i := strings.LastIndexByte(first, ':'); i >= 0 {
		first = first[i+1:]
	}
	if n, err := strconv.Atoi(first); err == nil && n > 0 && n < 65536 {
		return n
	}
	if strings.Contains(listen, "ssl") || strings.Contains(listen, "443") {
		return 443
	}
	return 80
}

// nginxReloadOrStart reloads nginx when it's running, otherwise boots it via
// systemctl. Covers the case where the host's nginx is currently stopped —
// `nginx -s reload` fails with "invalid PID number" because there's no master
// process to signal; starting is the right move then.
func nginxReloadOrStart(rn runner.Runner, log *Log) error {
	out, _ := rn.Run("pgrep -x nginx || true")
	if strings.TrimSpace(out) != "" {
		_, err := MustRun(rn, log, "sudo -n nginx -s reload")
		return err
	}
	log.Printf("nginx 未在运行，改用 systemctl start nginx\n")
	_, err := MustRun(rn, log, "sudo -n systemctl start nginx")
	return err
}

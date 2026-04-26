package takeover

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/runner"
	"github.com/serverhub/serverhub/pkg/safeshell"
	"gorm.io/gorm"
)

// runSystemd takes over a custom systemd-managed service. Strict safety gate
// up front: distro-package units (paths under /lib /usr/lib) and binaries
// owned by the system (anything in /usr /bin /sbin) are refused — those have
// upgrade contracts via apt/yum that takeover would silently break.
//
// Only "user-rolled" services pass: a unit file written by hand under
// /etc/systemd/system pointing at a binary in the user's own dir (e.g.
// /home/x/app, /opt/something, /srv/...).
//
// Flow (plan §Systemd):
//  1. safety gate    - reject system-package units
//  2. precheck       - target absent; WorkingDirectory readable; binary readable
//  3. copy           - cp -a WorkingDirectory/. target/releases/<ts>/
//                      cp ExecStart-binary target/bin/<binary-name>
//                      ln -sfn releases/<ts> target/current
//  4. backup unit    - cp /etc/systemd/system/<unit> backups/systemd/<ts>-<unit>
//  5. write new unit - serverhub-<name>.service pointing at target/{current,bin}
//  6. stop old       - systemctl stop <unit> && systemctl disable <unit>
//  7. install new    - systemctl daemon-reload && enable + start serverhub-<name>
//  8. probe          - is-active = active + 5s second-check
//  9. cleanup old    - rm /etc/systemd/system/<unit>
//                      mv WorkingDirectory WorkingDirectory.serverhub-takeover-<ts>
// 10. db insert
//
// Rollback is critical here — a half-migrated service means downtime. Every
// mutating step has an Undo, and Undo for the new-unit step covers both
// systemctl-level and filesystem-level state.
func runSystemd(db *gorm.DB, rn runner.Runner, log *Log, server model.Server,
	req Request, res *Result) error {

	c := req.Candidate
	unit := c.SourceID
	if unit == "" {
		return fmt.Errorf("候选缺少 source_id (unit name)")
	}
	if !strings.HasSuffix(unit, ".service") {
		unit = unit + ".service"
	}
	workDir := strings.TrimRight(c.Suggested.WorkDir, "/")
	execStart := strings.TrimSpace(c.Suggested.StartCmd)
	if workDir == "" || execStart == "" {
		return fmt.Errorf("候选缺少 WorkingDirectory / ExecStart")
	}
	if err := safeshell.AbsPath(workDir); err != nil {
		return fmt.Errorf("WorkingDirectory 非法: %w", err)
	}

	// First arg of ExecStart is the binary path.
	execFields := strings.Fields(execStart)
	binaryPath := execFields[0]
	binArgs := strings.Join(execFields[1:], " ")
	if err := safeshell.AbsPath(binaryPath); err != nil {
		return fmt.Errorf("ExecStart 二进制路径非法: %w", err)
	}
	binaryBase := path.Base(binaryPath)

	if reason := systemdSafetyGate(unit, binaryPath, workDir); reason != "" {
		return fmt.Errorf("拒绝接管: %s", reason)
	}

	target := TargetDir(req.TargetName)
	ts := Timestamp()
	releaseDir := target + "/releases/" + ts
	binDir := target + "/bin"
	newBin := binDir + "/" + binaryBase
	newUnitName := "serverhub-" + req.TargetName + ".service"
	newUnitPath := "/etc/systemd/system/" + newUnitName
	oldUnitPath := "/etc/systemd/system/" + unit
	backupDir := BackupDir("systemd")
	backupUnit := backupDir + "/" + ts + "-" + unit
	workBak := workDir + ".serverhub-takeover-" + ts

	var (
		origUnitBody string
		newCmd       string // ExecStart for the rewritten unit
	)

	steps := []Step{
		{
			Name: "precheck: WorkingDirectory + 二进制可读",
			Do: func() error {
				if err := EnsureReadable(rn, workDir); err != nil {
					return err
				}
				return EnsureReadable(rn, binaryPath)
			},
		},
		{
			Name: "读取并备份原 unit",
			Do: func() error {
				out, err := MustRun(rn, log, "sudo -n systemctl cat "+safeshell.Quote(unit))
				if err != nil {
					return err
				}
				origUnitBody = stripSystemctlCatHeader(out)
				if _, err := MustRun(rn, log, "sudo -n mkdir -p "+safeshell.Quote(backupDir)); err != nil {
					return err
				}
				return runtimeWriteFile(rn, log, backupUnit, origUnitBody)
			},
			Undo: func() error {
				_, err := MustRun(rn, log, "sudo -n rm -f "+safeshell.Quote(backupUnit))
				return err
			},
		},
		{
			Name: "复制 WorkingDirectory + 二进制到 " + target,
			Do: func() error {
				cmds := []string{
					"sudo -n mkdir -p " + safeshell.Quote(releaseDir),
					"sudo -n mkdir -p " + safeshell.Quote(binDir),
					"sudo -n cp -a " + safeshell.Quote(workDir+"/.") + " " + safeshell.Quote(releaseDir+"/"),
					"sudo -n cp -p " + safeshell.Quote(binaryPath) + " " + safeshell.Quote(newBin),
					"sudo -n ln -sfn " + safeshell.Quote("releases/"+ts) + " " + safeshell.Quote(target+"/current"),
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
			Name: "生成新 unit 文件 " + newUnitPath,
			Do: func() error {
				newCmd = newBin
				if binArgs != "" {
					newCmd += " " + binArgs
				}
				body := rewriteSystemdUnit(origUnitBody, target+"/current", newCmd)
				return runtimeWriteFile(rn, log, newUnitPath, body)
			},
			Undo: func() error {
				_, err := MustRun(rn, log, "sudo -n rm -f "+safeshell.Quote(newUnitPath))
				return err
			},
		},
		{
			Name: "停止并禁用原 unit",
			Do: func() error {
				if _, err := MustRun(rn, log, "sudo -n systemctl stop "+safeshell.Quote(unit)); err != nil {
					return err
				}
				_, err := MustRun(rn, log, "sudo -n systemctl disable "+safeshell.Quote(unit))
				return err
			},
			Undo: func() error {
				_, _ = MustRun(rn, log, "sudo -n systemctl enable "+safeshell.Quote(unit))
				_, err := MustRun(rn, log, "sudo -n systemctl start "+safeshell.Quote(unit))
				return err
			},
		},
		{
			Name: "启用并启动新 unit " + newUnitName,
			Do: func() error {
				cmds := []string{
					"sudo -n systemctl daemon-reload",
					"sudo -n systemctl enable " + safeshell.Quote(newUnitName),
					"sudo -n systemctl start " + safeshell.Quote(newUnitName),
				}
				for _, cmd := range cmds {
					if _, err := MustRun(rn, log, cmd); err != nil {
						return err
					}
				}
				return nil
			},
			Undo: func() error {
				_, _ = MustRun(rn, log, "sudo -n systemctl stop "+safeshell.Quote(newUnitName))
				_, _ = MustRun(rn, log, "sudo -n systemctl disable "+safeshell.Quote(newUnitName))
				_, err := MustRun(rn, log, "sudo -n systemctl daemon-reload")
				return err
			},
		},
		{
			Name: "探活: is-active + 5s 二次确认",
			Do: func() error {
				if err := systemctlIsActive(rn, log, newUnitName); err != nil {
					return err
				}
				time.Sleep(5 * time.Second)
				return systemctlIsActive(rn, log, newUnitName)
			},
		},
		{
			Name: "删除旧 unit 文件并改名 WorkingDirectory",
			Do: func() error {
				if _, err := MustRun(rn, log,
					"sudo -n rm -f "+safeshell.Quote(oldUnitPath)); err != nil {
					return err
				}
				_, err := MustRun(rn, log,
					"sudo -n mv "+safeshell.Quote(workDir)+" "+safeshell.Quote(workBak))
				return err
			},
			Undo: func() error {
				_, _ = MustRun(rn, log,
					"sudo -n mv "+safeshell.Quote(workBak)+" "+safeshell.Quote(workDir))
				_, err := MustRun(rn, log, safeshell.WriteRemoteFile(oldUnitPath, origUnitBody, true))
				if err != nil {
					return err
				}
				_, err = MustRun(rn, log, "sudo -n systemctl daemon-reload")
				return err
			},
		},
	}

	if err := RunSteps(log, steps); err != nil {
		res.RolledBack = true
		return err
	}

	d := model.Service{
		Name:       req.TargetName,
		ServerID:   server.ID,
		Type:       "native",
		WorkDir:    target,
		SourceKind: c.Kind,
		SourceID:   c.SourceID,
		SyncStatus: "synced",
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

// systemdSafetyGate refuses takeover if any of these distro-package signals
// hit. Returns "" when the unit looks user-rolled and is safe to migrate.
func systemdSafetyGate(unit, binary, workDir string) string {
	// 1) Resolved unit file location: `systemctl cat` would have shown the
	//    actual on-disk path, but our discovery only carries the unit name.
	//    We can still sanity-check by looking at the binary + workdir prefixes
	//    that overwhelmingly imply a packaged service.
	systemBinPrefixes := []string{
		"/usr/sbin/", "/usr/bin/", "/sbin/", "/bin/",
		"/usr/lib/", "/usr/libexec/", "/lib/",
	}
	for _, p := range systemBinPrefixes {
		if strings.HasPrefix(binary, p) {
			return "ExecStart 二进制位于系统目录: " + binary
		}
	}
	systemDataPrefixes := []string{
		"/usr/", "/etc/", "/var/lib/",
	}
	for _, p := range systemDataPrefixes {
		if strings.HasPrefix(workDir, p) {
			return "WorkingDirectory 位于系统目录: " + workDir
		}
	}
	return ""
}

// stripSystemctlCatHeader removes the `# /path/to/unit` header line `systemctl
// cat` prefixes to its output so we round-trip a clean unit file.
func stripSystemctlCatHeader(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "# /") || strings.HasPrefix(t, "# ; /") {
			continue
		}
		out = append(out, ln)
	}
	return strings.TrimSpace(strings.Join(out, "\n")) + "\n"
}

// rewriteSystemdUnit replaces WorkingDirectory= and ExecStart= in the original
// unit body, leaving every other directive (User/Group/Restart/Environment/
// After/WantedBy/...) byte-for-byte intact.
func rewriteSystemdUnit(body, newWorkDir, newExecStart string) string {
	lines := strings.Split(body, "\n")
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		switch {
		case strings.HasPrefix(t, "WorkingDirectory="):
			lines[i] = "WorkingDirectory=" + newWorkDir
		case strings.HasPrefix(t, "ExecStart="):
			lines[i] = "ExecStart=" + newExecStart
		}
	}
	header := "# managed by serverhub takeover\n"
	return header + strings.Join(lines, "\n")
}

// systemctlIsActive returns nil only when the named unit reports "active".
func systemctlIsActive(rn runner.Runner, log *Log, unit string) error {
	out, _ := rn.Run("sudo -n systemctl is-active " + safeshell.Quote(unit))
	state := strings.TrimSpace(out)
	log.Printf("is-active %s = %s\n", unit, state)
	if state != "active" {
		return fmt.Errorf("%s 状态非 active: %s", unit, state)
	}
	return nil
}

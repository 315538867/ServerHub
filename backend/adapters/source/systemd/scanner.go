package systemd

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/serverhub/serverhub/core/source"
	"github.com/serverhub/serverhub/infra"
	"github.com/serverhub/serverhub/internal/stepkit"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/safeshell"
)

const Kind = "systemd"

type Scanner struct{}

func (Scanner) Kind() string { return Kind }

// systemUnitSkip 过滤永远不该出现在候选列表的系统级 unit。
var systemUnitSkip = []string{
	"systemd-", "dbus", "NetworkManager", "ssh.service", "sshd.service",
	"cron.service", "rsyslog", "snapd", "udev", "polkit", "getty",
	"user@", "networkd", "resolved", "logind", "journald", "timesyncd",
	"apt-daily", "accounts-daemon", "cloud-", "multipathd", "unattended-",
}

// Discover 列 active running services,提取 WorkingDirectory + 第一条
// ExecStart + Environment / EnvironmentFile。无 ExecStart 的丢弃。
func (s Scanner) Discover(ctx context.Context, r infra.Runner) ([]source.Candidate, error) {
	listOut, _, err := r.Run(ctx,
		`systemctl list-units --type=service --state=running --no-legend --no-pager 2>/dev/null | awk '{print $1}'`)
	if err != nil {
		return nil, nil
	}
	listOut = strings.TrimSpace(listOut)
	if listOut == "" {
		return nil, nil
	}

	var out []source.Candidate
	for _, unit := range strings.Split(listOut, "\n") {
		unit = strings.TrimSpace(unit)
		if unit == "" || shouldSkipUnit(unit) {
			continue
		}
		detailOut, _, derr := r.Run(ctx, "systemctl cat "+safeshell.Quote(unit)+" 2>/dev/null")
		if derr != nil || detailOut == "" {
			continue
		}
		workDir, execStart, envInline, envFiles := parseUnit(detailOut)
		if execStart == "" {
			continue
		}
		env := envInline
		for _, p := range envFiles {
			env = mergeEnv(env, readEnvFile(ctx, r, p))
		}
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
			SourceID: unit,
			Name:     strings.TrimSuffix(unit, ".service"),
			Cmd:      execStart,
			Summary:  truncate(execStart, 120),
			Suggested: source.SuggestedFields{
				Type:       model.ServiceTypeNative,
				Workdir:    workDir,
				StartCmd:   execStart,
				EnvVars:    envVars,
				EnvSecrets: envSecrets,
			},
			Raw: map[string]string{
				"exec_start": execStart,
			},
		})
	}
	return out, nil
}

// Fingerprint: sha1("systemd|<unit>|<exec_start>"),与 v1 字节一致。
func (Scanner) Fingerprint(c source.Candidate) string {
	key := strings.Join([]string{
		"systemd",
		c.SourceID,
		c.Raw["exec_start"],
	}, "|")
	sum := sha1.Sum([]byte(key))
	return hex.EncodeToString(sum[:])
}

// Takeover 平移 v1 pkg/takeover/systemd.go runSystemd。安全门拒绝系统包 unit。
//
// Flow:
//  1. precheck      - WorkingDirectory + 二进制可读
//  2. backup unit   - 读 + 备份原 unit 到 /opt/serverhub/backups/systemd/
//  3. copy          - cp -a workdir → target/releases/<ts>/; cp 二进制 → target/bin/;
//                     ln -sfn releases/<ts> target/current
//  4. write new unit
//  5. stop+disable old
//  6. enable+start new
//  7. probe         - is-active + 5s 二次确认
//  8. cleanup old   - rm 旧 unit 文件; mv workdir workdir.serverhub-takeover-<ts>
func (Scanner) Takeover(ctx context.Context, tc source.TakeoverContext) error {
	cand := tc.Cand
	unit := cand.SourceID
	if unit == "" {
		return fmt.Errorf("候选缺少 source_id (unit name)")
	}
	if !strings.HasSuffix(unit, ".service") {
		unit = unit + ".service"
	}
	workDir := strings.TrimRight(cand.Suggested.Workdir, "/")
	execStart := strings.TrimSpace(cand.Suggested.StartCmd)
	if workDir == "" || execStart == "" {
		return fmt.Errorf("候选缺少 WorkingDirectory / ExecStart")
	}
	if err := safeshell.AbsPath(workDir); err != nil {
		return fmt.Errorf("WorkingDirectory 非法: %w", err)
	}
	execFields := strings.Fields(execStart)
	binaryPath := execFields[0]
	binArgs := strings.Join(execFields[1:], " ")
	if err := safeshell.AbsPath(binaryPath); err != nil {
		return fmt.Errorf("ExecStart 二进制路径非法: %w", err)
	}
	if reason := systemdSafetyGate(unit, binaryPath, workDir); reason != "" {
		return fmt.Errorf("拒绝接管: %s", reason)
	}
	if err := safeshell.ValidName(tc.SvcName, 64); err != nil {
		return fmt.Errorf("svc_name 非法: %w", err)
	}

	binaryBase := path.Base(binaryPath)
	target := stepkit.TargetDir(tc.SvcName)
	ts := stepkit.Timestamp()
	releaseDir := target + "/releases/" + ts
	binDir := target + "/bin"
	newBin := binDir + "/" + binaryBase
	newUnitName := "serverhub-" + tc.SvcName + ".service"
	newUnitPath := "/etc/systemd/system/" + newUnitName
	oldUnitPath := "/etc/systemd/system/" + unit
	backupDir := stepkit.BackupDir("systemd")
	backupUnit := backupDir + "/" + ts + "-" + unit
	workBak := workDir + ".serverhub-takeover-" + ts
	log := &stepkit.Log{}

	var (
		origUnitBody string
		newCmd       string
	)

	steps := []stepkit.Step{
		{
			Name: "precheck: WorkingDirectory + 二进制可读",
			Do: func() error {
				if err := stepkit.EnsureReadable(ctx, tc.Runner, workDir); err != nil {
					return err
				}
				return stepkit.EnsureReadable(ctx, tc.Runner, binaryPath)
			},
		},
		{
			Name: "读取并备份原 unit",
			Do: func() error {
				out, err := stepkit.MustRun(ctx, tc.Runner, log, "sudo -n systemctl cat "+safeshell.Quote(unit))
				if err != nil {
					return err
				}
				origUnitBody = stripSystemctlCatHeader(out)
				if _, err := stepkit.MustRun(ctx, tc.Runner, log, "sudo -n mkdir -p "+safeshell.Quote(backupDir)); err != nil {
					return err
				}
				return stepkit.WriteRemoteFile(ctx, tc.Runner, log, backupUnit, origUnitBody)
			},
			Undo: func() error {
				_, err := stepkit.MustRun(ctx, tc.Runner, log, "sudo -n rm -f "+safeshell.Quote(backupUnit))
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
			Name: "生成新 unit 文件 " + newUnitPath,
			Do: func() error {
				newCmd = newBin
				if binArgs != "" {
					newCmd += " " + binArgs
				}
				body := rewriteSystemdUnit(origUnitBody, target+"/current", newCmd)
				return stepkit.WriteRemoteFile(ctx, tc.Runner, log, newUnitPath, body)
			},
			Undo: func() error {
				_, err := stepkit.MustRun(ctx, tc.Runner, log, "sudo -n rm -f "+safeshell.Quote(newUnitPath))
				return err
			},
		},
		{
			Name: "停止并禁用原 unit",
			Do: func() error {
				if _, err := stepkit.MustRun(ctx, tc.Runner, log, "sudo -n systemctl stop "+safeshell.Quote(unit)); err != nil {
					return err
				}
				_, err := stepkit.MustRun(ctx, tc.Runner, log, "sudo -n systemctl disable "+safeshell.Quote(unit))
				return err
			},
			Undo: func() error {
				_, _ = stepkit.MustRun(ctx, tc.Runner, log, "sudo -n systemctl enable "+safeshell.Quote(unit))
				_, err := stepkit.MustRun(ctx, tc.Runner, log, "sudo -n systemctl start "+safeshell.Quote(unit))
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
				for _, c := range cmds {
					if _, err := stepkit.MustRun(ctx, tc.Runner, log, c); err != nil {
						return err
					}
				}
				return nil
			},
			Undo: func() error {
				_, _ = stepkit.MustRun(ctx, tc.Runner, log, "sudo -n systemctl stop "+safeshell.Quote(newUnitName))
				_, _ = stepkit.MustRun(ctx, tc.Runner, log, "sudo -n systemctl disable "+safeshell.Quote(newUnitName))
				_, err := stepkit.MustRun(ctx, tc.Runner, log, "sudo -n systemctl daemon-reload")
				return err
			},
		},
		{
			Name: "探活: is-active + 5s 二次确认",
			Do: func() error {
				if err := systemctlIsActive(ctx, tc.Runner, log, newUnitName); err != nil {
					return err
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(5 * time.Second):
				}
				return systemctlIsActive(ctx, tc.Runner, log, newUnitName)
			},
		},
		{
			Name: "删除旧 unit 文件并改名 WorkingDirectory",
			Do: func() error {
				if _, err := stepkit.MustRun(ctx, tc.Runner, log,
					"sudo -n rm -f "+safeshell.Quote(oldUnitPath)); err != nil {
					return err
				}
				_, err := stepkit.MustRun(ctx, tc.Runner, log,
					"sudo -n mv "+safeshell.Quote(workDir)+" "+safeshell.Quote(workBak))
				return err
			},
			Undo: func() error {
				_, _ = stepkit.MustRun(ctx, tc.Runner, log,
					"sudo -n mv "+safeshell.Quote(workBak)+" "+safeshell.Quote(workDir))
				_, err := stepkit.MustRun(ctx, tc.Runner, log, safeshell.WriteRemoteFile(oldUnitPath, origUnitBody, true))
				if err != nil {
					return err
				}
				_, err = stepkit.MustRun(ctx, tc.Runner, log, "sudo -n systemctl daemon-reload")
				return err
			},
		},
	}
	return stepkit.RunSteps(log, steps)
}

func systemctlIsActive(ctx context.Context, r infra.Runner, log *stepkit.Log, unit string) error {
	out, _, _ := r.Run(ctx, "sudo -n systemctl is-active "+safeshell.Quote(unit))
	state := strings.TrimSpace(out)
	log.Printf("is-active %s = %s\n", unit, state)
	if state != "active" {
		return fmt.Errorf("%s 状态非 active: %s", unit, state)
	}
	return nil
}

package discovery

import (
	"strings"

	"github.com/serverhub/serverhub/pkg/runner"
)

// systemUnitSkip filters out system-level units we never want to surface.
var systemUnitSkip = []string{
	"systemd-", "dbus", "NetworkManager", "ssh.service", "sshd.service",
	"cron.service", "rsyslog", "snapd", "udev", "polkit", "getty",
	"user@", "networkd", "resolved", "logind", "journald", "timesyncd",
	"apt-daily", "accounts-daemon", "cloud-", "multipathd", "unattended-",
}

// ScanSystemd enumerates active system services and extracts their WorkingDirectory
// and ExecStart. Returns empty slice if systemd is not available (e.g. containers).
func ScanSystemd(rn runner.Runner) ([]Candidate, error) {
	list, err := rn.Run(`systemctl list-units --type=service --state=running --no-legend --no-pager 2>/dev/null | awk '{print $1}'`)
	if err != nil || strings.TrimSpace(list) == "" {
		return nil, err
	}

	var out []Candidate
	for _, unit := range strings.Split(strings.TrimSpace(list), "\n") {
		unit = strings.TrimSpace(unit)
		if unit == "" || shouldSkipUnit(unit) {
			continue
		}
		detail, derr := rn.Run("systemctl cat " + shellQuote(unit) + " 2>/dev/null")
		if derr != nil || detail == "" {
			continue
		}
		workDir, execStart := parseUnit(detail)
		if execStart == "" {
			continue
		}
		out = append(out, Candidate{
			Kind:     KindSystemd,
			SourceID: unit,
			Name:     strings.TrimSuffix(unit, ".service"),
			Summary:  truncate(execStart, 120),
			Suggested: SuggestedDeploy{
				Type:     "native",
				WorkDir:  workDir,
				StartCmd: execStart,
			},
		})
	}
	return out, nil
}

func shouldSkipUnit(u string) bool {
	for _, p := range systemUnitSkip {
		if strings.HasPrefix(u, p) || strings.Contains(u, p) {
			return true
		}
	}
	return false
}

// parseUnit extracts WorkingDirectory= and ExecStart= from `systemctl cat` output.
// ExecStart may have a leading `-` or `@`; those prefixes are stripped.
func parseUnit(body string) (workDir, execStart string) {
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(line, "WorkingDirectory="):
			workDir = strings.TrimSpace(strings.TrimPrefix(line, "WorkingDirectory="))
		case strings.HasPrefix(line, "ExecStart=") && execStart == "":
			v := strings.TrimPrefix(line, "ExecStart=")
			v = strings.TrimLeft(v, "-@+:!")
			execStart = strings.TrimSpace(v)
		}
	}
	return
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

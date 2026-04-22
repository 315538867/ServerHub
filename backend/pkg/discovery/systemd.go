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
		workDir, execStart, envInline, envFiles := parseUnit(detail)
		if execStart == "" {
			continue
		}
		// Merge inline `Environment=` first, then any `EnvironmentFile=`.
		// Inline takes precedence per systemd's own semantics.
		env := envInline
		for _, p := range envFiles {
			env = mergeEnv(env, readEnvFile(rn, p))
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
				Env:      env,
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

// parseUnit extracts the fields we care about from `systemctl cat` output:
//   - WorkingDirectory=
//   - first ExecStart= (leading `-@+:!` prefixes are stripped)
//   - all Environment= entries (each line may contain multiple K=V pairs,
//     possibly quoted)
//   - all EnvironmentFile= entries (leading `-` marks optional files; we
//     accept either form and let the reader silently ignore missing files)
func parseUnit(body string) (workDir, execStart string, envInline []EnvKV, envFiles []string) {
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(line, "WorkingDirectory="):
			workDir = strings.TrimSpace(strings.TrimPrefix(line, "WorkingDirectory="))
		case strings.HasPrefix(line, "ExecStart=") && execStart == "":
			v := strings.TrimPrefix(line, "ExecStart=")
			v = strings.TrimLeft(v, "-@+:!")
			execStart = strings.TrimSpace(v)
		case strings.HasPrefix(line, "Environment="):
			v := strings.TrimSpace(strings.TrimPrefix(line, "Environment="))
			envInline = mergeEnv(envInline, parseKVPairs(splitEnvLine(v)))
		case strings.HasPrefix(line, "EnvironmentFile="):
			v := strings.TrimSpace(strings.TrimPrefix(line, "EnvironmentFile="))
			v = strings.TrimPrefix(v, "-") // optional-file marker
			v = strings.TrimSpace(v)
			if v != "" {
				envFiles = append(envFiles, v)
			}
		}
	}
	return
}

// splitEnvLine splits a systemd Environment= value into individual K=V tokens.
// Per systemd semantics, multiple pairs on one line are space-separated and
// quoted values may contain spaces, e.g. `FOO=1 BAR="hello world" BAZ=qux`.
func splitEnvLine(s string) []string {
	var out []string
	var cur strings.Builder
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			} else {
				cur.WriteByte(c)
			}
		case c == '"' || c == '\'':
			quote = c
		case c == ' ' || c == '\t':
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
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

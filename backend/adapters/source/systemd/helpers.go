package systemd

import (
	"context"
	"strings"

	"github.com/serverhub/serverhub/infra"
	"github.com/serverhub/serverhub/pkg/safeshell"
)

// envKV 单条发现的环境变量。Secret 由 keyLooksSecret 启发判定;在 Discover
// 回填阶段折叠到 source.SuggestedFields.EnvSecrets[k]=true。
type envKV struct {
	Key    string
	Value  string
	Secret bool
}

// secretKeyHints — substring 大小写不敏感命中,UI 默认掩码。
// 与 v1 pkg/discovery/env.go 保持完全一致以维持行为兼容。
var secretKeyHints = []string{
	"password", "passwd", "secret", "token", "apikey", "api_key",
	"private", "credential", "jdbc", "dsn", "auth", "session",
	"jwt", "encrypt", "salt", "cookie",
}

func keyLooksSecret(k string) bool {
	lower := strings.ToLower(k)
	if strings.HasSuffix(lower, "_key") || (strings.HasSuffix(lower, "key") && len(lower) > 3) {
		return true
	}
	for _, h := range secretKeyHints {
		if strings.Contains(lower, h) {
			return true
		}
	}
	return false
}

func parseKVPairs(pairs []string) []envKV {
	out := make([]envKV, 0, len(pairs))
	for _, p := range pairs {
		p = strings.TrimSpace(p)
		if p == "" || strings.HasPrefix(p, "#") {
			continue
		}
		eq := strings.IndexByte(p, '=')
		if eq <= 0 {
			continue
		}
		k := strings.TrimSpace(p[:eq])
		v := strings.TrimSpace(p[eq+1:])
		v = unquote(v)
		if k == "" {
			continue
		}
		out = append(out, envKV{Key: k, Value: v, Secret: keyLooksSecret(k)})
	}
	return out
}

func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// shouldSkipUnit 判断 unit 是否落在 systemUnitSkip 黑名单。
func shouldSkipUnit(u string) bool {
	for _, p := range systemUnitSkip {
		if strings.HasPrefix(u, p) || strings.Contains(u, p) {
			return true
		}
	}
	return false
}

// parseUnit 从 `systemctl cat` 输出抽取我们关心的字段:
//   - WorkingDirectory=
//   - 第一条 ExecStart=(剥离 `-@+:!` 前缀)
//   - 全部 Environment= 行(每行可含多对 K=V,可带引号)
//   - 全部 EnvironmentFile= 行(`-` 前缀 = 可选文件)
func parseUnit(body string) (workDir, execStart string, envInline []envKV, envFiles []string) {
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
			v = strings.TrimPrefix(v, "-")
			v = strings.TrimSpace(v)
			if v != "" {
				envFiles = append(envFiles, v)
			}
		}
	}
	return
}

// splitEnvLine 拆 systemd Environment= 值为 K=V token,支持引号包含空格的值。
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

// readEnvFile 通过 runner 拉 .env 文件并解析。文件缺失/读失败安静返回 nil。
func readEnvFile(ctx context.Context, r infra.Runner, path string) []envKV {
	if path == "" {
		return nil
	}
	out, _, err := r.Run(ctx, "cat "+safeshell.Quote(path)+" 2>/dev/null")
	if err != nil || strings.TrimSpace(out) == "" {
		return nil
	}
	return parseKVPairs(strings.Split(out, "\n"))
}

// mergeEnv 把 extra 追加到 base,跳过 base 已有的 key(base 优先)。
func mergeEnv(base, extra []envKV) []envKV {
	if len(extra) == 0 {
		return base
	}
	seen := make(map[string]struct{}, len(base))
	for _, kv := range base {
		seen[kv.Key] = struct{}{}
	}
	for _, kv := range extra {
		if _, ok := seen[kv.Key]; ok {
			continue
		}
		seen[kv.Key] = struct{}{}
		base = append(base, kv)
	}
	return base
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// systemdSafetyGate 拒绝接管发行版包提供的 unit:
// /usr/lib /lib 下的二进制 + /usr /etc /var/lib 下的 WorkingDirectory
// 都意味着 apt/yum 升级契约,接管会破坏。返回 "" 表示放行。
func systemdSafetyGate(unit, binary, workDir string) string {
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

// stripSystemctlCatHeader 去掉 `systemctl cat` 输出顶部的 `# /path` 注释行,
// 让我们能 round-trip 一份干净的 unit 文件。
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

// rewriteSystemdUnit 替换原 unit 的 WorkingDirectory= 与 ExecStart=,其余指令
// (User/Group/Restart/Environment/After/WantedBy/...)字节级保留。
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

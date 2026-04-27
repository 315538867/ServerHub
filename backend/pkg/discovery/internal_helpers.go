// internal_helpers.go 是 R4 后 pkg/discovery 唯一保留的辅助文件,只服务于
// 同包的 ingress_proxy.go (反向代理 vhost 扫描器,R5 ingress 适配器接管前
// 还需要它)。
//
// 这些 helper 在 R4 之前散落在 nginx.go / systemd.go,删原文件后必须把它们
// 收拢到这里,避免 ingress_proxy.go 反向依赖 adapter 包(adapter→discovery
// 是允许的,反方向就是循环了)。R5 把 ingress_proxy 平移到
// adapters/source/ingress_proxy 时本文件随同删掉。
package discovery

import (
	"regexp"
	"strings"
)

var (
	nginxServerBlockRe = regexp.MustCompile(`(?s)server\s*\{`)
	nginxServerNameRe  = regexp.MustCompile(`(?m)^\s*server_name\s+([^;]+);`)
	nginxListenRe      = regexp.MustCompile(`(?m)^\s*listen\s+([^;]+);`)
)

// parseLocationPrefix 抽取 `location [modifier] URI {` 行里的 URI。
// 仅 prefix-style match (无修饰 / `=` / `^~`) 返回非空;regex 修饰返回空串。
func parseLocationPrefix(line string) string {
	rest := strings.TrimPrefix(line, "location")
	if rest == line {
		return ""
	}
	if len(rest) == 0 || (rest[0] != ' ' && rest[0] != '\t') {
		return ""
	}
	rest = strings.TrimSpace(rest)
	if i := strings.IndexByte(rest, '{'); i >= 0 {
		rest = strings.TrimSpace(rest[:i])
	}
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return ""
	}
	mod := fields[0]
	uri := ""
	switch mod {
	case "=", "^~":
		if len(fields) >= 2 {
			uri = fields[1]
		}
	case "~", "~*":
		return ""
	default:
		uri = mod
	}
	if uri == "" || !strings.HasPrefix(uri, "/") {
		return ""
	}
	return uri
}

func firstField(s string) string {
	for _, f := range strings.Fields(s) {
		return f
	}
	return ""
}

// shellQuote 将参数包成单引号字符串,内部单引号按 POSIX 习惯 `'\''` 转义。
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

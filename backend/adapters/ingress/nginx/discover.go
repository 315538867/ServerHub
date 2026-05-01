package nginx

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"regexp"
	"sort"
	"strings"

	"github.com/serverhub/serverhub/core/ingress"
	"github.com/serverhub/serverhub/infra"
)

// discover 扫描 sites-enabled / conf.d 下所有反代 vhost,按 server{} 块切片,
// 每个块抽取 server_name / listen 与各个 location 内的 proxy_pass。无 proxy_pass
// 的 vhost 会被跳过(那是静态站点的活儿,归 source/nginx 管)。
//
// AlreadyManaged 与 Route.CrossServerID/Name 由 usecase 层填,这里只信 nginx
// conf 字面量,不查 DB、不解析跨机。本函数平移自 pkg/discovery/ingress_proxy.go::
// ScanNginxIngressProxy,仅把 Runner 接口从 pkg/runner.Runner (Run(cmd)) 适配成
// infra.Runner (Run(ctx, cmd) → stdout, stderr, err) — 共两个调用点。
func discover(ctx context.Context, r infra.Runner) ([]ingress.IngressCandidate, error) {
	list, _, err := r.Run(ctx,
		`( ls /etc/nginx/sites-enabled/ 2>/dev/null | sed 's|^|/etc/nginx/sites-enabled/|'; `+
			`ls /etc/nginx/conf.d/*.conf 2>/dev/null ) | sort -u`)
	if err != nil || strings.TrimSpace(list) == "" {
		return nil, nil
	}
	var out []ingress.IngressCandidate
	for _, path := range strings.Split(strings.TrimSpace(list), "\n") {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		body, _, berr := r.Run(ctx, "cat "+shellQuote(path)+" 2>/dev/null")
		if berr != nil || body == "" {
			continue
		}
		blocks := splitServerBlocks(body)
		for _, block := range blocks {
			cand, ok := extractIngressProxyCandidate(block)
			if !ok {
				continue
			}
			cand.ConfigFile = path
			cand.Fingerprint = ingressProxyFingerprint(path, cand.ServerName)
			out = append(out, cand)
		}
	}
	// 同名 server_name 多次出现(http+https 双站点)时只保留第一条;指纹按
	// (path, server_name) 计算保证稳定,不受 routes 顺序变化影响。
	seen := map[string]bool{}
	uniq := make([]ingress.IngressCandidate, 0, len(out))
	for _, c := range out {
		if seen[c.Fingerprint] {
			continue
		}
		seen[c.Fingerprint] = true
		uniq = append(uniq, c)
	}
	sort.Slice(uniq, func(i, j int) bool {
		if uniq[i].ServerName != uniq[j].ServerName {
			return uniq[i].ServerName < uniq[j].ServerName
		}
		return uniq[i].ConfigFile < uniq[j].ConfigFile
	})
	return uniq, nil
}

var (
	nginxServerBlockRe = regexp.MustCompile(`(?s)server\s*\{`)
	nginxServerNameRe  = regexp.MustCompile(`(?m)^\s*server_name\s+([^;]+);`)
	nginxListenRe      = regexp.MustCompile(`(?m)^\s*listen\s+([^;]+);`)

	ingressProxyPassRe     = regexp.MustCompile(`^\s*proxy_pass\s+([^;]+);`)
	ingressUpgradeHeaderRe = regexp.MustCompile(`(?i)proxy_set_header\s+Upgrade\s+\$http_upgrade`)
	ingressConnHeaderRe    = regexp.MustCompile(`(?i)proxy_set_header\s+Connection\s+["']?upgrade["']?`)
)

// splitServerBlocks 把 nginx vhost 文本按 `server { ... }` 切片,返回每个块的
// 内部文本(不含外层花括号)。先剥 # 注释避免 location 内的 # 干扰花括号配对。
func splitServerBlocks(body string) []string {
	var clean strings.Builder
	for _, line := range strings.Split(body, "\n") {
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		clean.WriteString(line)
		clean.WriteByte('\n')
	}
	text := clean.String()
	var blocks []string
	for {
		loc := nginxServerBlockRe.FindStringIndex(text)
		if loc == nil {
			break
		}
		start := loc[1] - 1
		depth := 0
		end := -1
		for i := start; i < len(text); i++ {
			switch text[i] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					end = i
				}
			}
			if end >= 0 {
				break
			}
		}
		if end < 0 {
			break
		}
		blocks = append(blocks, text[start+1:end])
		text = text[end+1:]
	}
	return blocks
}

// extractIngressProxyCandidate 把一个 server{} 块解析成 IngressCandidate。规则:
//   - 只采第一条 server_name / listen
//   - 顶层 proxy_pass:合成一条 path="/" 的 route,Extra 为整个 server 块去掉
//     listen/server_name/proxy_pass 的剩余行
//   - 每个 location { ... }:path = location 的 prefix,Extra 为 location 内除
//     proxy_pass 之外的剩余行
//   - 检测 Upgrade/Connection upgrade 头 → WebSocket=true
//   - 整个块没找到任何 proxy_pass → 返回 (_, false)
func extractIngressProxyCandidate(block string) (ingress.IngressCandidate, bool) {
	cand := ingress.IngressCandidate{}
	if m := nginxServerNameRe.FindStringSubmatch(block); m != nil {
		cand.ServerName = strings.TrimSpace(firstField(m[1]))
	}
	if m := nginxListenRe.FindStringSubmatch(block); m != nil {
		cand.Listen = strings.TrimSpace(m[1])
	}

	type loc struct {
		prefix    string
		body      string
		proxyPass string
	}
	var locs []loc
	var topLines []string
	var topProxyPass string
	var topWS bool

	lines := strings.Split(block, "\n")
	depth := 0
	type frame struct {
		prefix    string
		body      strings.Builder
		proxyPass string
		started   bool
	}
	var stack []*frame
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		opens := strings.Count(line, "{")
		closes := strings.Count(line, "}")
		var cur *frame
		if depth > 0 && len(stack) > 0 {
			cur = stack[len(stack)-1]
		}
		if depth == 0 {
			if m := ingressProxyPassRe.FindStringSubmatch(line); m != nil {
				topProxyPass = strings.TrimSpace(m[1])
			} else if !isHeaderDirective(trimmed, "listen") &&
				!isHeaderDirective(trimmed, "server_name") &&
				!strings.HasPrefix(trimmed, "location") &&
				trimmed != "" && trimmed != "{" && trimmed != "}" {
				topLines = append(topLines, line)
			}
			if ingressUpgradeHeaderRe.MatchString(line) || ingressConnHeaderRe.MatchString(line) {
				topWS = true
			}
		} else if cur != nil {
			if m := ingressProxyPassRe.FindStringSubmatch(line); m != nil {
				cur.proxyPass = strings.TrimSpace(m[1])
			} else if !strings.HasPrefix(trimmed, "location") || cur.started {
				if trimmed != "" && trimmed != "{" && trimmed != "}" {
					cur.body.WriteString(line + "\n")
				}
			}
			cur.started = true
		}

		for i := 0; i < opens; i++ {
			if depth == 0 && strings.HasPrefix(trimmed, "location") {
				stack = append(stack, &frame{prefix: parseLocationPrefix(trimmed)})
			} else {
				stack = append(stack, nil)
			}
			depth++
		}
		for i := 0; i < closes; i++ {
			if depth > 0 {
				depth--
				if len(stack) > 0 {
					top := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					if top != nil && depth == 0 {
						locs = append(locs, loc{
							prefix:    top.prefix,
							body:      top.body.String(),
							proxyPass: top.proxyPass,
						})
					}
				}
			}
		}
	}

	for _, l := range locs {
		if l.proxyPass == "" {
			continue
		}
		path := l.prefix
		if path == "" {
			path = "/"
		}
		ws := ingressUpgradeHeaderRe.MatchString(l.body) ||
			ingressConnHeaderRe.MatchString(l.body)
		cand.Routes = append(cand.Routes, ingress.Route{
			Path:      path,
			ProxyPass: l.proxyPass,
			WebSocket: ws,
			Extra:     strings.TrimRight(l.body, "\n"),
		})
	}

	if topProxyPass != "" {
		cand.Routes = append([]ingress.Route{{
			Path:      "/",
			ProxyPass: topProxyPass,
			WebSocket: topWS,
			Extra:     strings.TrimSpace(strings.Join(topLines, "\n")),
		}}, cand.Routes...)
	}

	if len(cand.Routes) == 0 {
		return cand, false
	}
	return cand, true
}

func isHeaderDirective(line, name string) bool {
	rest := strings.TrimPrefix(line, name)
	if rest == line {
		return false
	}
	return len(rest) > 0 && (rest[0] == ' ' || rest[0] == '\t' || rest[0] == ';')
}

func ingressProxyFingerprint(path, serverName string) string {
	h := sha1.Sum([]byte(path + "|" + serverName))
	return hex.EncodeToString(h[:8])
}

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

// ProxyPassHost 从 proxy_pass URL 字面量里抽出 host(不含端口)。usecase 层用它
// 做跨机检测;同 edge 自己 / unix sock / 域名 / upstream{} 名字一律返回 ("", false)。
func ProxyPassHost(proxyPass string) (string, bool) {
	pp := strings.TrimSpace(proxyPass)
	if pp == "" {
		return "", false
	}
	if strings.HasPrefix(pp, "unix:") {
		return "", false
	}
	idx := strings.Index(pp, "://")
	if idx < 0 {
		return "", false
	}
	rest := pp[idx+3:]
	for _, sep := range []string{"/", "?", "#"} {
		if i := strings.Index(rest, sep); i >= 0 {
			rest = rest[:i]
		}
	}
	if i := strings.LastIndex(rest, ":"); i >= 0 {
		if strings.HasPrefix(rest, "[") {
			return "", false
		}
		rest = rest[:i]
	}
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return "", false
	}
	return rest, true
}

package nginx

import (
	"regexp"
	"strings"
)

// 与 v1 pkg/discovery/nginx.go 一致的 server{} 块切分 + 字段抽取。
// ingress_proxy adapter (R5) 也需要类似 parser,本包导出 Parse* 之外的
// helper 暂时仅在包内复用,待 R5 扩抽到 internal/nginxparse。

var (
	serverBlockRe = regexp.MustCompile(`(?s)server\s*\{`)
	serverNameRe  = regexp.MustCompile(`(?m)^\s*server_name\s+([^;]+);`)
	listenRe      = regexp.MustCompile(`(?m)^\s*listen\s+([^;]+);`)
	proxyPassRe   = regexp.MustCompile(`(?m)^\s*proxy_pass\s+[^;]+;`)
)

type nginxSite struct {
	ServerName string
	Listen     string
	RootDir    string
	Aliases    []string
	NestedRoot []string
	HasProxy   bool
}

// Roots 汇总 server 块所有静态根:top-level root + 每条 location 的 root
// 重组(prefix 合并)+ alias 原文。去重保序。
func (s nginxSite) Roots() []string {
	seen := map[string]bool{}
	var out []string
	push := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
	}
	push(s.RootDir)
	for _, p := range s.NestedRoot {
		push(p)
	}
	for _, p := range s.Aliases {
		push(p)
	}
	return out
}

// parseSites 把 conf 拆成 server{} 块,逐个抽字段。注释先剥再走花括号配对。
func parseSites(body string) []nginxSite {
	var clean strings.Builder
	for _, line := range strings.Split(body, "\n") {
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		clean.WriteString(line)
		clean.WriteByte('\n')
	}
	text := clean.String()

	var sites []nginxSite
	for {
		loc := serverBlockRe.FindStringIndex(text)
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
		sites = append(sites, extractSite(text[start+1:end]))
		text = text[end+1:]
	}
	return sites
}

func extractSite(block string) nginxSite {
	s := nginxSite{}
	if m := serverNameRe.FindStringSubmatch(block); m != nil {
		s.ServerName = strings.TrimSpace(firstField(m[1]))
	}
	if m := listenRe.FindStringSubmatch(block); m != nil {
		s.Listen = strings.TrimSpace(firstField(m[1]))
	}
	if proxyPassRe.MatchString(block) {
		s.HasProxy = true
	}
	type frame struct{ prefix string }
	var stack []frame
	depth := 0
	for _, line := range strings.Split(block, "\n") {
		trimmed := strings.TrimSpace(line)
		if depth == 0 {
			if m := directiveValue("root", trimmed); m != "" {
				s.RootDir = unquoteNginx(m)
			}
		} else {
			prefix := ""
			if len(stack) > 0 {
				prefix = stack[len(stack)-1].prefix
			}
			if m := directiveValue("root", trimmed); m != "" {
				dir := unquoteNginx(m)
				if prefix != "" {
					dir = joinNginxPath(dir, prefix)
				}
				s.NestedRoot = append(s.NestedRoot, dir)
			}
			if m := directiveValue("alias", trimmed); m != "" {
				s.Aliases = append(s.Aliases, unquoteNginx(m))
			}
		}
		if open := strings.Count(line, "{"); open > 0 {
			for i := 0; i < open; i++ {
				stack = append(stack, frame{prefix: parseLocationPrefix(trimmed)})
			}
		}
		for _, c := range line {
			switch c {
			case '{':
				depth++
			case '}':
				depth--
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
			}
		}
	}
	return s
}

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

func joinNginxPath(root, prefix string) string {
	root = strings.TrimRight(root, "/")
	prefix = strings.TrimLeft(prefix, "/")
	out := root + "/" + prefix
	return strings.TrimRight(out, "/")
}

func directiveValue(name, line string) string {
	rest := strings.TrimPrefix(line, name)
	if rest == line {
		return ""
	}
	if len(rest) == 0 || (rest[0] != ' ' && rest[0] != '\t') {
		return ""
	}
	rest = strings.TrimSpace(rest)
	semi := strings.IndexByte(rest, ';')
	if semi < 0 {
		return ""
	}
	val := strings.TrimSpace(rest[:semi])
	return firstField(val)
}

func unquoteNginx(s string) string {
	return strings.Trim(strings.TrimSpace(s), `"'`)
}

func firstField(s string) string {
	for _, f := range strings.Fields(s) {
		return f
	}
	return ""
}

func baseName(path string) string {
	i := strings.LastIndexByte(path, '/')
	name := path
	if i >= 0 {
		name = path[i+1:]
	}
	return strings.TrimSuffix(name, ".conf")
}

func confBase(path string) string {
	i := strings.LastIndexByte(path, '/')
	if i < 0 {
		return path
	}
	return path[i+1:]
}

func slugPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.Trim(p, "/")
	if p == "" {
		return "root"
	}
	return strings.ReplaceAll(p, "/", "_")
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func fallbackStr(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

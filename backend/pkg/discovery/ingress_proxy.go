package discovery

import (
	"crypto/sha1"
	"encoding/hex"
	"regexp"
	"sort"
	"strings"

	"github.com/serverhub/serverhub/pkg/runner"
)

// IngressProxyRoute 描述一个反向代理 vhost 内的一条 location（或 server 顶层
// proxy_pass）转译成 IngressRoute 所需的最小字段。
//
// Extra 是 location 块内除 proxy_pass 之外的所有 body 行原样保留（trim 掉前导空格）；
// 用户接管入库后这部分会落到 IngressRoute.Extra，由渲染器照贴回 location，确保
// 接管后第一次 apply 不丢失原配置语义。
type IngressProxyRoute struct {
	Path      string `json:"path"`       // location prefix（无 location 包裹时为 "/"）
	ProxyPass string `json:"proxy_pass"` // proxy_pass 后面的 URL
	WebSocket bool   `json:"websocket"`  // 检测到 Upgrade/Connection: upgrade 头
	Extra     string `json:"extra"`      // 剩余 body 行，verbatim
}

// IngressProxyCandidate 是"sites-available 里某个反代 vhost 可被接管"的一条建议。
// 一个 server{} 块映射成一个 candidate；候选对应的 routes 通常 ≥1 条。
type IngressProxyCandidate struct {
	ConfigFile     string              `json:"config_file"`     // 来源 conf 路径
	ServerName     string              `json:"server_name"`     // 第一个 server_name
	Listen         string              `json:"listen"`          // 第一个 listen 值（含可能的 ssl/http2 等修饰）
	Routes         []IngressProxyRoute `json:"routes"`
	Fingerprint    string              `json:"fingerprint"`     // 稳定指纹（conf path + server_name）
	AlreadyManaged bool                `json:"already_managed"` // 同 edge 同 domain 已有 Ingress
}

// ScanNginxIngressProxy 扫描 sites-enabled / conf.d 下所有反代 vhost，按 server{}
// 块切片，每个块抽取 server_name / listen 与各个 location 内的 proxy_pass。无
// proxy_pass 的 vhost 会被跳过（那是静态站点的活儿，归 ScanNginx 管）。
//
// 与 ScanNginx 不同，这里不读 listing/fs 上的 index.html — 我们只信 nginx conf
// 的字面量；接管后用户可以再补 OverrideHost/Port。
func ScanNginxIngressProxy(rn runner.Runner) ([]IngressProxyCandidate, error) {
	list, err := rn.Run(
		`( ls /etc/nginx/sites-enabled/ 2>/dev/null | sed 's|^|/etc/nginx/sites-enabled/|'; ` +
			`ls /etc/nginx/conf.d/*.conf 2>/dev/null ) | sort -u`)
	if err != nil || strings.TrimSpace(list) == "" {
		return nil, nil
	}
	var out []IngressProxyCandidate
	for _, path := range strings.Split(strings.TrimSpace(list), "\n") {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		body, berr := rn.Run("cat " + shellQuote(path) + " 2>/dev/null")
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
	// 同名 server_name 多次出现（http+https 双站点）时只保留第一条；指纹按
	// (path, server_name) 计算保证稳定，不受 routes 顺序变化影响。
	seen := map[string]bool{}
	uniq := make([]IngressProxyCandidate, 0, len(out))
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

// splitServerBlocks 复用 nginx.go 的 server{} 拆分逻辑，但返回 block 文本而不是
// 解析后的结构体——这里我们要在块内做更细粒度的 location 走查。
func splitServerBlocks(body string) []string {
	// 与 parseNginxSites 一样先剥掉 # 注释，避免 location 内 # 干扰花括号配对。
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

var (
	ingressProxyPassRe       = regexp.MustCompile(`^\s*proxy_pass\s+([^;]+);`)
	ingressUpgradeHeaderRe   = regexp.MustCompile(`(?i)proxy_set_header\s+Upgrade\s+\$http_upgrade`)
	ingressConnHeaderRe      = regexp.MustCompile(`(?i)proxy_set_header\s+Connection\s+["']?upgrade["']?`)
)

// extractIngressProxyCandidate 把一个 server{} 块（不含外层花括号）解析成
// IngressProxyCandidate。规则：
//   - 只采第一条 server_name / listen（多个 listen 取首条；ipv6 行通常作为第二条）
//   - 顶层 proxy_pass：合成一条 path="/" 的 route，body 为整个 server 块去掉
//     listen/server_name/proxy_pass 的剩余行
//   - 每个 location { ... }：path = location 的 prefix，body 为 location 内除
//     proxy_pass 之外的剩余行
//   - 检测 Upgrade/Connection upgrade 头 → WebSocket=true
//   - 整个块没找到任何 proxy_pass → 返回 (_, false)
func extractIngressProxyCandidate(block string) (IngressProxyCandidate, bool) {
	cand := IngressProxyCandidate{}
	if m := nginxServerNameRe.FindStringSubmatch(block); m != nil {
		cand.ServerName = strings.TrimSpace(firstField(m[1]))
	}
	if m := nginxListenRe.FindStringSubmatch(block); m != nil {
		cand.Listen = strings.TrimSpace(m[1])
	}

	// 一次 pass 同时收集顶层杂项行 + 每个 location 块的范围。
	type loc struct {
		prefix string
		body   string
	}
	var locs []loc
	var topLines []string
	var topProxyPass string
	var topWS bool

	lines := strings.Split(block, "\n")
	depth := 0
	type frame struct {
		prefix  string
		body    strings.Builder
		started bool
	}
	var stack []*frame
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// 计算这一行后将进入的深度变化。先处理 `{`：location 的 prefix 必须在
		// `{` 之前的字串里读出来（同一行）。
		opens := strings.Count(line, "{")
		closes := strings.Count(line, "}")
		// 当前所在帧（处理本行内容时——还没考虑这一行的 brace 变化）
		var cur *frame
		if depth > 0 && len(stack) > 0 {
			cur = stack[len(stack)-1]
		}
		// 顶层处理：抽出 proxy_pass / 跳过 listen/server_name / 其它都收进 topLines
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
			// 在 location 内：body 累积；proxy_pass 单独抓出，不写进 body。
			if m := ingressProxyPassRe.FindStringSubmatch(line); m != nil {
				// proxy_pass 不入 body，但要记到该帧（取最后一条作为 ProxyPass）
				cur.body.WriteString("__PROXY_PASS__=" + strings.TrimSpace(m[1]) + "\n")
			} else if !strings.HasPrefix(trimmed, "location") || cur.started {
				// 第一条 `location ... {` 也别写进 body（已经被作为帧 prefix 消化）
				if trimmed != "" {
					cur.body.WriteString(line + "\n")
				}
			}
			cur.started = true
		}

		// 处理 brace：先开后关（同一行 `}` 比 `{` 多的情况是单行 location；
		// 但 nginx 单行 location 在 sites-available 极少见，简化为先 open 后 close）
		for i := 0; i < opens; i++ {
			if depth == 0 && strings.HasPrefix(trimmed, "location") {
				stack = append(stack, &frame{prefix: parseLocationPrefix(trimmed)})
			} else {
				// 嵌套（if/limit_except/...）：当前帧继续，深度+1，但不新开 location 帧
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
						locs = append(locs, loc{prefix: top.prefix, body: top.body.String()})
					}
				}
			}
		}
	}

	for _, l := range locs {
		path := l.prefix
		if path == "" {
			path = "/"
		}
		body := l.body
		// 抽出 location 帧里我们临时编码的 __PROXY_PASS__ 行
		var pp string
		var rest strings.Builder
		for _, ln := range strings.Split(body, "\n") {
			if strings.HasPrefix(ln, "__PROXY_PASS__=") {
				pp = strings.TrimPrefix(ln, "__PROXY_PASS__=")
				continue
			}
			rest.WriteString(ln + "\n")
		}
		if pp == "" {
			continue
		}
		ws := ingressUpgradeHeaderRe.MatchString(rest.String()) ||
			ingressConnHeaderRe.MatchString(rest.String())
		cand.Routes = append(cand.Routes, IngressProxyRoute{
			Path:      path,
			ProxyPass: pp,
			WebSocket: ws,
			Extra:     strings.TrimRight(rest.String(), "\n"),
		})
	}

	if topProxyPass != "" {
		// 顶层 proxy_pass：把所有顶层杂项行都塞进 Extra，路径用 "/"
		cand.Routes = append([]IngressProxyRoute{{
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

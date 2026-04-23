package discovery

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/serverhub/serverhub/pkg/runner"
)

// ScanNginx enumerates enabled nginx vhost files and produces Candidates for
// static sites (server blocks that declare a `root` directive and do not
// forward to an upstream via `proxy_pass`). Reverse-proxy vhosts are ignored
// here — their backends show up through docker/systemd detectors.
//
// When nginx is not installed or sites-enabled is empty, returns (nil, nil):
// absence of nginx is not an error in this context.
func ScanNginx(rn runner.Runner) ([]Candidate, error) {
	// sites-enabled is the conventional discovery root on Debian/Ubuntu. On
	// RHEL-family systems confs live under /etc/nginx/conf.d; we scan both.
	list, err := rn.Run(
		`( ls /etc/nginx/sites-enabled/ 2>/dev/null | sed 's|^|/etc/nginx/sites-enabled/|'; ` +
			`ls /etc/nginx/conf.d/*.conf 2>/dev/null ) | sort -u`)
	if err != nil || strings.TrimSpace(list) == "" {
		return nil, nil
	}

	var out []Candidate
	seen := map[string]bool{}
	for _, path := range strings.Split(strings.TrimSpace(list), "\n") {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		body, berr := rn.Run("cat " + shellQuote(path) + " 2>/dev/null")
		if berr != nil || body == "" {
			continue
		}
		sites := parseNginxSites(body)
		name := nginxBaseName(path)
		for i, s := range sites {
			// Need at least one filesystem-backed location to qualify as a
			// static-site candidate. A pure reverse proxy (no root, no alias)
			// is uninteresting here — it shows up via docker / systemd.
			roots := s.Roots()
			if len(roots) == 0 {
				continue
			}
			sid := name
			if len(sites) > 1 {
				sid = name + "#" + strconv.Itoa(i)
			}
			if seen[sid] {
				continue
			}
			seen[sid] = true
			primary := roots[0]
			sum := strings.TrimSpace(s.ServerName)
			if sum == "" {
				sum = "static site"
			}
			sum += "  root=" + primary
			if len(roots) > 1 {
				sum += "  (+" + strconv.Itoa(len(roots)-1) + " path)"
			}
			if s.HasProxy {
				sum += "  +reverse-proxy"
			}
			out = append(out, Candidate{
				Kind:     KindNginx,
				SourceID: sid,
				Name:     fallbackStr(s.ServerName, name),
				Summary:  truncate(sum, 200),
				Suggested: SuggestedDeploy{
					Type:    "static",
					WorkDir: primary,
				},
				ExtraLabels: map[string]string{
					"config_file": path,
					"server_name": s.ServerName,
					"listen":      s.Listen,
					"all_roots":   strings.Join(roots, ","),
					"has_proxy":   boolStr(s.HasProxy),
				},
			})
		}
	}
	return out, nil
}

type nginxSite struct {
	ServerName string
	Listen     string
	RootDir    string   // top-level `root` directive
	Aliases    []string // per-location `alias` paths (each is a static path)
	NestedRoot []string // per-location `root` overrides (also static)
	HasProxy   bool
}

// Roots returns all filesystem paths this server block serves files from,
// ordered: top-level root first, then per-location roots, then aliases.
// Duplicates removed.
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

var (
	nginxServerBlockRe = regexp.MustCompile(`(?s)server\s*\{`)
	nginxServerNameRe  = regexp.MustCompile(`(?m)^\s*server_name\s+([^;]+);`)
	nginxListenRe      = regexp.MustCompile(`(?m)^\s*listen\s+([^;]+);`)
	nginxProxyPassRe   = regexp.MustCompile(`(?m)^\s*proxy_pass\s+[^;]+;`)
)

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// parseNginxSites splits the conf into server {} blocks by brace-depth
// tracking (regex cannot match nested braces) and extracts the fields we
// care about from each block. Comments are stripped first.
func parseNginxSites(body string) []nginxSite {
	// strip # comments (conservative: drop from # to end-of-line; ignores the
	// extremely rare "#" inside a quoted string, which nginx configs almost
	// never have at directive level)
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
		loc := nginxServerBlockRe.FindStringIndex(text)
		if loc == nil {
			break
		}
		// walk from the `{` and find matching `}`
		start := loc[1] - 1 // index of '{'
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
		block := text[start+1 : end]
		sites = append(sites, extractSite(block))
		text = text[end+1:]
	}
	return sites
}

func extractSite(block string) nginxSite {
	s := nginxSite{}
	if m := nginxServerNameRe.FindStringSubmatch(block); m != nil {
		s.ServerName = strings.TrimSpace(firstField(m[1]))
	}
	if m := nginxListenRe.FindStringSubmatch(block); m != nil {
		s.Listen = strings.TrimSpace(firstField(m[1]))
	}
	if nginxProxyPassRe.MatchString(block) {
		s.HasProxy = true
	}
	// Walk the block line-by-line, tracking brace depth so we can tell a
	// top-level `root` from one scoped inside a location {}. Both are real
	// static-site paths, but only the top-level one is the default doc root.
	depth := 0
	for _, line := range strings.Split(block, "\n") {
		trimmed := strings.TrimSpace(line)
		// Directive parsing must happen BEFORE depth adjustment for `{` on
		// the same line (e.g. `location / { root /foo; }` on one line) so we
		// don't misclassify. Nginx conf rarely does that, but handle it
		// anyway: we check directive matches, then adjust depth for the line.
		if depth == 0 {
			if m := nginxDirectiveRe("root", trimmed); m != "" {
				s.RootDir = unquoteNginx(m)
			}
		} else {
			if m := nginxDirectiveRe("root", trimmed); m != "" {
				s.NestedRoot = append(s.NestedRoot, unquoteNginx(m))
			}
			if m := nginxDirectiveRe("alias", trimmed); m != "" {
				s.Aliases = append(s.Aliases, unquoteNginx(m))
			}
		}
		for _, c := range line {
			switch c {
			case '{':
				depth++
			case '}':
				depth--
			}
		}
	}
	return s
}

// nginxDirectiveRe returns the single-argument value of `name arg;` on this
// line, or "" if not matched. Handles leading whitespace. Does not support
// multi-line directives (nginx conventions rarely require it for our uses).
func nginxDirectiveRe(name, line string) string {
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

func nginxBaseName(path string) string {
	i := strings.LastIndexByte(path, '/')
	name := path
	if i >= 0 {
		name = path[i+1:]
	}
	return strings.TrimSuffix(name, ".conf")
}

func fallbackStr(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

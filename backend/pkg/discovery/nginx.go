package discovery

import (
	"regexp"
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
			if s.Root == "" || s.HasProxy {
				continue
			}
			sid := name
			if len(sites) > 1 {
				sid = name + "#" + itoa(i)
			}
			if seen[sid] {
				continue
			}
			seen[sid] = true
			sum := strings.TrimSpace(s.ServerName)
			if sum == "" {
				sum = "static site"
			}
			sum += "  root=" + s.Root
			out = append(out, Candidate{
				Kind:     KindNginx,
				SourceID: sid,
				Name:     fallbackStr(s.ServerName, name),
				Summary:  truncate(sum, 160),
				Suggested: SuggestedDeploy{
					Type:    "static",
					WorkDir: s.Root,
				},
				ExtraLabels: map[string]string{
					"config_file": path,
					"server_name": s.ServerName,
					"listen":      s.Listen,
				},
			})
		}
	}
	return out, nil
}

type nginxSite struct {
	ServerName string
	Listen     string
	Root       string
	HasProxy   bool
}

var (
	nginxServerBlockRe = regexp.MustCompile(`(?s)server\s*\{`)
	nginxServerNameRe  = regexp.MustCompile(`(?m)^\s*server_name\s+([^;]+);`)
	nginxListenRe      = regexp.MustCompile(`(?m)^\s*listen\s+([^;]+);`)
	nginxRootRe        = regexp.MustCompile(`(?m)^\s*root\s+([^;]+);`)
	nginxProxyPassRe   = regexp.MustCompile(`(?m)^\s*proxy_pass\s+[^;]+;`)
)

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
	if m := nginxRootRe.FindStringSubmatch(block); m != nil {
		s.Root = strings.Trim(strings.TrimSpace(m[1]), `"'`)
	}
	if nginxProxyPassRe.MatchString(block) {
		s.HasProxy = true
	}
	return s
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

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

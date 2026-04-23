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
		// Skip the Debian/Ubuntu fallback vhost — it's an OS welcome page, not a
		// real deployed site. Actual sites live in their own named conf files.
		isDefaultFile := name == "default"
		for i, s := range sites {
			// A pure reverse proxy (no root, no alias) is uninteresting here —
			// it shows up via docker / systemd.
			roots := s.Roots()
			if len(roots) == 0 {
				continue
			}
			// Drop the catch-all fallback vhost: `server_name _` or the Debian
			// default file with an empty/default server_name is the welcome
			// page, not a user-deployed frontend.
			sn := strings.TrimSpace(s.ServerName)
			if sn == "_" || (isDefaultFile && (sn == "" || sn == "_")) {
				continue
			}
			// Filter each root path: drop OS-default dirs and paths that don't
			// actually look like a frontend build (no index.html on disk).
			roots = filterStaticRoots(rn, roots)
			if len(roots) == 0 {
				continue
			}
			sidBase := name
			if len(sites) > 1 {
				sidBase = name + "#" + strconv.Itoa(i)
			}
			// Emit one candidate per static root path: each is independently
			// deployable (takeover flow needs a single WorkDir) and the user
			// should be able to pick them individually. Same vhost may back
			// several paths (an admin panel aliased + a Vite app under /lxy/).
			for _, rootPath := range roots {
				sid := sidBase + "|" + slugPath(rootPath)
				if seen[sid] {
					continue
				}
				seen[sid] = true
				sum := strings.TrimSpace(s.ServerName)
				if sum == "" {
					sum = "static site"
				}
				sum += "  root=" + rootPath
				if s.HasProxy {
					sum += "  +reverse-proxy"
				}
				dispName := fallbackStr(s.ServerName, name)
				if len(roots) > 1 {
					dispName += " [" + rootPath + "]"
				}
				out = append(out, Candidate{
					Kind:     KindNginx,
					SourceID: sid,
					Name:     dispName,
					Summary:  truncate(sum, 200),
					Suggested: SuggestedDeploy{
						Type:    "static",
						WorkDir: rootPath,
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

// slugPath turns a filesystem path into a compact identifier fragment that's
// stable across scans. We keep it readable (not hashed) so SourceIDs stay
// debuggable and diff-friendly in the UI.
func slugPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.Trim(p, "/")
	if p == "" {
		return "root"
	}
	return strings.ReplaceAll(p, "/", "_")
}

// osDefaultWebRoots are paths that ship with the distro's nginx/apache package
// as a welcome page. They virtually never host a real user-deployed frontend,
// so we exclude them from discovery results.
var osDefaultWebRoots = map[string]bool{
	"/var/www":            true,
	"/var/www/html":       true,
	"/usr/share/nginx/html": true,
	"/usr/share/nginx":    true,
}

// filterStaticRoots keeps only those paths that (a) are not OS-default welcome
// directories and (b) actually contain an index.html on the target host. The
// index.html check is a simple proxy for "a real frontend was deployed here"
// — SPAs, Vite/Webpack/Next-export builds all produce one at the root.
func filterStaticRoots(rn runner.Runner, roots []string) []string {
	var out []string
	for _, p := range roots {
		clean := strings.TrimRight(p, "/")
		if clean == "" || osDefaultWebRoots[clean] {
			continue
		}
		// `test -f` returns 0 only if the file exists and is regular. We don't
		// care about stderr; a non-zero exit just means "skip this path".
		if _, err := rn.Run("test -f " + shellQuote(clean+"/index.html") + " && echo ok"); err != nil {
			continue
		}
		out = append(out, p)
	}
	return out
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
	// Walk the block line-by-line, tracking brace depth and a stack of the
	// enclosing `location` URIs. nginx semantics:
	//   - top-level `root X`         → serves files from X/<url>
	//   - `location /p/ { root X; }` → serves files from X/p/<rest>  (the
	//                                   location prefix is *prepended* to root)
	//   - `location /p/ { alias X; }`→ serves files from X/<rest>    (alias
	//                                   replaces the location prefix entirely)
	// So nested-root effective dir = root + location-prefix, while alias
	// effective dir = alias as-is. We can only resolve prefix-style locations
	// (no modifier, `=`, or `^~`); regex locations (`~` / `~*`) we skip.
	type frame struct{ prefix string } // empty for non-prefix or non-location frames
	var stack []frame
	depth := 0
	for _, line := range strings.Split(block, "\n") {
		trimmed := strings.TrimSpace(line)
		if depth == 0 {
			if m := nginxDirectiveRe("root", trimmed); m != "" {
				s.RootDir = unquoteNginx(m)
			}
		} else {
			prefix := ""
			if len(stack) > 0 {
				prefix = stack[len(stack)-1].prefix
			}
			if m := nginxDirectiveRe("root", trimmed); m != "" {
				dir := unquoteNginx(m)
				if prefix != "" {
					dir = joinNginxPath(dir, prefix)
				}
				s.NestedRoot = append(s.NestedRoot, dir)
			}
			if m := nginxDirectiveRe("alias", trimmed); m != "" {
				s.Aliases = append(s.Aliases, unquoteNginx(m))
			}
		}
		// Update the location stack BEFORE depth-tracking, so a `{` on this
		// line opens a frame whose prefix is the URI we just parsed. We only
		// react to `{` here; `}` pops the matching frame below alongside depth.
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

// parseLocationPrefix returns the URI of a `location` directive on this line
// if it's a prefix-style match we can use to compose paths (no modifier, `=`,
// or `^~`). Returns "" for regex modifiers or non-location lines — which
// causes the caller to treat the frame as "no useful prefix".
func parseLocationPrefix(line string) string {
	rest := strings.TrimPrefix(line, "location")
	if rest == line {
		return ""
	}
	if len(rest) == 0 || (rest[0] != ' ' && rest[0] != '\t') {
		return ""
	}
	rest = strings.TrimSpace(rest)
	// strip trailing `{` and anything after (a one-line `location / { ... }`)
	if i := strings.IndexByte(rest, '{'); i >= 0 {
		rest = strings.TrimSpace(rest[:i])
	}
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return ""
	}
	// optional modifier
	mod := fields[0]
	uri := ""
	switch mod {
	case "=", "^~":
		if len(fields) >= 2 {
			uri = fields[1]
		}
	case "~", "~*":
		return "" // regex location — we can't compose a directory path
	default:
		uri = mod
	}
	if uri == "" || !strings.HasPrefix(uri, "/") {
		return ""
	}
	return uri
}

// joinNginxPath composes <root>/<location-prefix> with one slash between
// them, normalizing any trailing slash on root and leading/trailing slashes
// on the prefix. Trailing slash on the result is preserved if the prefix
// had one — that helps the index.html probe match the served directory.
func joinNginxPath(root, prefix string) string {
	root = strings.TrimRight(root, "/")
	prefix = strings.TrimLeft(prefix, "/")
	out := root + "/" + prefix
	return strings.TrimRight(out, "/")
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

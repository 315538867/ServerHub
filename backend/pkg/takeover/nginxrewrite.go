package takeover

import (
	"fmt"
	"regexp"
	"strings"
)

// NginxRewrite transforms an existing nginx config so the server blocks that
// reference oldRoot (either as top-level root, nested root within a location,
// or alias) point at newRoot instead. Other directives (ssl, proxy_pass,
// add_header, ...) are left byte-for-byte unchanged.
//
// Strategy: parse line-by-line tracking brace depth and the current location
// prefix stack (same approach as discovery/nginx.go). When we hit a matching
// root/alias directive, rewrite just that line. Lines we don't touch flow
// through verbatim. This preserves comments, ordering, spacing.
//
// Matching rules:
//   - top-level `root X;` matches if X == oldRoot
//   - nested `root X;` inside `location P {…}` matches if X+P == oldRoot
//     (same join semantics as the discovery module)
//   - `alias X;` matches if X (trimmed of trailing slash) equals oldRoot
//     trimmed likewise
//
// The rewrite always emits the replacement as `root <newRoot>;` (even where
// the original used alias) because newRoot is a serverhub-owned symlink whose
// contents mirror oldRoot exactly — root semantics serve files from newRoot
// directly without needing alias' prefix-replacement trick.
func NginxRewrite(body, oldRoot, newRoot string) (string, int, error) {
	old := strings.TrimRight(oldRoot, "/")
	if old == "" {
		return body, 0, fmt.Errorf("oldRoot 为空")
	}
	new_ := strings.TrimRight(newRoot, "/")
	if new_ == "" {
		return body, 0, fmt.Errorf("newRoot 为空")
	}

	lines := strings.Split(body, "\n")
	depth := 0
	type frame struct{ prefix string }
	var stack []frame
	hits := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		prefix := ""
		if depth > 0 && len(stack) > 0 {
			prefix = stack[len(stack)-1].prefix
		}

		if val := directiveValue("root", trimmed); val != "" {
			effective := strings.TrimRight(val, "/")
			if depth > 0 && prefix != "" {
				effective = joinNginxPath(effective, prefix)
			}
			if effective == old {
				lines[i] = replaceDirectiveValue(line, "root", new_)
				hits++
			}
		} else if val := directiveValue("alias", trimmed); val != "" {
			if strings.TrimRight(val, "/") == old {
				// Replace the alias directive with a root pointing to newRoot.
				// We keep the original indentation via replaceDirective.
				lines[i] = replaceDirective(line, "alias", "root", new_)
				hits++
			}
		}

		// Push/pop location stack to keep prefix accurate. `location` directives
		// always open a block with `{` on the same or following line; we only
		// push when we actually see the `{`.
		for _, c := range line {
			switch c {
			case '{':
				depth++
				stack = append(stack, frame{prefix: parseLocPrefix(trimmed)})
			case '}':
				depth--
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
			}
		}
	}

	if hits == 0 {
		return body, 0, fmt.Errorf("未在配置中找到指向 %q 的 root/alias 指令", oldRoot)
	}
	return strings.Join(lines, "\n"), hits, nil
}

var directiveLineRe = regexp.MustCompile(`^(\s*)(root|alias)\s+([^;]+);(.*)$`)

// directiveValue extracts the argument of a `name arg;` directive from a
// single trimmed line. Returns "" if the line isn't that directive.
func directiveValue(name, line string) string {
	if !strings.HasPrefix(line, name) {
		return ""
	}
	rest := line[len(name):]
	if rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
		return ""
	}
	rest = strings.TrimSpace(rest)
	semi := strings.IndexByte(rest, ';')
	if semi < 0 {
		return ""
	}
	val := strings.TrimSpace(rest[:semi])
	// Strip surrounding quotes (rare in practice).
	return strings.Trim(val, `"'`)
}

// replaceDirectiveValue swaps the value of a matched `root` directive while
// keeping the line's indentation, any trailing comment, and the semicolon.
func replaceDirectiveValue(origLine, name, newVal string) string {
	m := directiveLineRe.FindStringSubmatch(origLine)
	if m == nil || m[2] != name {
		// Shouldn't happen given the caller already matched, but fall back.
		return origLine
	}
	return m[1] + name + " " + newVal + ";" + m[4] + "  # managed by serverhub"
}

// replaceDirective swaps both the name and value (e.g. alias → root).
func replaceDirective(origLine, fromName, toName, newVal string) string {
	m := directiveLineRe.FindStringSubmatch(origLine)
	if m == nil || m[2] != fromName {
		return origLine
	}
	return m[1] + toName + " " + newVal + ";" + m[4] + "  # managed by serverhub (was " + fromName + ")"
}

// parseLocPrefix extracts the URI from a `location [modifier] URI {` line.
// Returns "" for regex modifiers (`~`/`~*`) or non-location lines.
func parseLocPrefix(line string) string {
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
	f := strings.Fields(rest)
	if len(f) == 0 {
		return ""
	}
	switch f[0] {
	case "=", "^~":
		if len(f) >= 2 {
			return f[1]
		}
		return ""
	case "~", "~*":
		return ""
	default:
		if strings.HasPrefix(f[0], "/") {
			return f[0]
		}
		return ""
	}
}

// joinNginxPath composes root + location-prefix the way nginx does when the
// prefix is a simple string match. Mirrors discovery/nginx.go.
func joinNginxPath(root, prefix string) string {
	root = strings.TrimRight(root, "/")
	prefix = strings.TrimLeft(prefix, "/")
	return strings.TrimRight(root+"/"+prefix, "/")
}

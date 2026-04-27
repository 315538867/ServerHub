package nginx

import (
	"fmt"
	"regexp"
	"strings"
)

// NginxRewrite 把 server 块里指向 oldRoot 的 root/alias 指令改写到 newRoot,
// 其余字节不动。规则与 v1 pkg/takeover.NginxRewrite 等价:
//   - 顶层 `root X;`：X==old → root → newRoot
//   - location 内 `root X;`：X+prefix==old → 改成 alias newRoot/(去 prefix 叠加)
//   - `alias X;`：X==old → alias newRoot/
//
// 没匹配到任何指令视为失败 (返回 hits=0 + err),由 caller 决定是否回滚。
func NginxRewrite(body, oldRoot, newRoot string) (string, int, error) {
	old := strings.TrimRight(oldRoot, "/")
	if old == "" {
		return body, 0, fmt.Errorf("oldRoot 为空")
	}
	newR := strings.TrimRight(newRoot, "/")
	if newR == "" {
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

		if val := rawDirectiveValue("root", trimmed); val != "" {
			effective := strings.TrimRight(val, "/")
			nested := depth > 0 && prefix != ""
			if nested {
				effective = joinNginxPath(effective, prefix)
			}
			if effective == old {
				if nested {
					lines[i] = replaceDirective(line, "root", "alias", newR+"/")
				} else {
					lines[i] = replaceDirectiveValue(line, "root", newR)
				}
				hits++
			}
		} else if val := rawDirectiveValue("alias", trimmed); val != "" {
			if strings.TrimRight(val, "/") == old {
				lines[i] = replaceDirectiveValue(line, "alias", newR+"/")
				hits++
			}
		}

		for _, c := range line {
			switch c {
			case '{':
				depth++
				stack = append(stack, frame{prefix: parseLocationPrefix(trimmed)})
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

// rawDirectiveValue 与 parser.directiveValue 区别在于:不取 firstField,而是
// 把整段 trim quotes,以便处理 `root "/path with spaces";` 之类边角。
func rawDirectiveValue(name, line string) string {
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
	return strings.Trim(val, `"'`)
}

func replaceDirectiveValue(origLine, name, newVal string) string {
	m := directiveLineRe.FindStringSubmatch(origLine)
	if m == nil || m[2] != name {
		return origLine
	}
	return m[1] + name + " " + newVal + ";" + m[4] + "  # managed by serverhub"
}

func replaceDirective(origLine, fromName, toName, newVal string) string {
	m := directiveLineRe.FindStringSubmatch(origLine)
	if m == nil || m[2] != fromName {
		return origLine
	}
	return m[1] + toName + " " + newVal + ";" + m[4] + "  # managed by serverhub (was " + fromName + ")"
}

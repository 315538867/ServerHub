// Package safeshell centralises shell-string handling for commands sent to
// remote hosts. Use it to (a) single-quote arbitrary values for `bash -c`
// contexts and (b) validate user-controlled identifiers that are interpolated
// into file paths.
package safeshell

import (
	"encoding/base64"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// Quote wraps s in single quotes and escapes any embedded single quotes so the
// result is safe to splice into a shell command. Always prefer Quote over
// ad-hoc concatenation.
func Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

// nameRe matches identifiers used as filenames or symbol names.
var nameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// ValidName returns nil when s is a non-empty identifier of [A-Za-z0-9._-]
// not exceeding maxLen characters and not equal to "." or "..".
func ValidName(s string, maxLen int) error {
	if s == "" {
		return errors.New("名称不能为空")
	}
	if len(s) > maxLen {
		return fmt.Errorf("名称过长（>%d）", maxLen)
	}
	if s == "." || s == ".." {
		return errors.New("名称非法")
	}
	if !nameRe.MatchString(s) {
		return errors.New("名称只能包含字母、数字、点、下划线和短横线")
	}
	return nil
}

// ValidVersion is a stricter ValidName tailored for deploy version labels.
func ValidVersion(s string) error {
	if s == "" {
		return errors.New("版本不能为空")
	}
	return ValidName(s, 64)
}

// AbsPath returns nil when p is a clean, absolute filesystem path that does
// not contain shell metacharacters or newlines. It is intended for values that
// are spliced into shell commands as path roots (e.g. application BaseDir).
func AbsPath(p string) error {
	if p == "" {
		return errors.New("路径不能为空")
	}
	if !path.IsAbs(p) {
		return errors.New("必须是绝对路径")
	}
	if filepath.Clean(p) != p {
		return errors.New("路径包含 .. 或多余分隔符")
	}
	if strings.ContainsAny(p, "\n\r\t\x00`$;&|<>*?\"'\\") {
		return errors.New("路径包含非法字符")
	}
	return nil
}

// WriteRemoteFile builds a shell command that pipes content into `tee path`
// via base64. Compared to heredoc this immune to terminator-injection because
// the base64 alphabet cannot represent newlines or shell metacharacters.
//
// Set sudo=true to prepend `sudo -n` to tee so the command can write
// root-owned files.
func WriteRemoteFile(path, content string, sudo bool) string {
	enc := base64.StdEncoding.EncodeToString([]byte(content))
	tee := "tee " + Quote(path) + " > /dev/null"
	if sudo {
		tee = "sudo -n " + tee
	}
	return "printf '%s' " + Quote(enc) + " | base64 -d | " + tee
}

// nginxValueBad matches characters that have meaning inside an nginx config
// directive value (semicolon, braces, newline, quote, hash) and would let a
// caller break out of the surrounding directive.
var nginxValueBad = regexp.MustCompile(`[;{}\n\r"'#\\]`)

// NginxValue returns nil when v is safe to splice into an nginx directive
// value (e.g. server_name, root, proxy_pass).
func NginxValue(v string) error {
	if v == "" {
		return errors.New("值不能为空")
	}
	if len(v) > 1024 {
		return errors.New("值过长")
	}
	if nginxValueBad.MatchString(v) {
		return errors.New("值包含 nginx 指令分隔符")
	}
	return nil
}

// NginxBlock 校验"多行 nginx 指令片段"——比 NginxValue 宽松得多,允许换行 / 分号
// 与双引号(用户写 add_header X "Y" 这种是合法的),但仍然禁止以下"提权"字符:
//
//   - `\` 反引号或 `$(...)`：location 块本身不会被 shell 解释,但我们的
//     Apply 链路会把整段配置 base64 后写到远端的 tee,这里防的是"渲染时
//     有人在 Extra 里偷塞 shell 元字符再走 base64 解码"——实际不会被执行,
//     纯属减小攻击面。
//   - `{` `}`：用户的 Extra 内容会被插入到 location { ... } 内部。一旦
//     允许花括号,就能提前关掉 location、注入新的 server / location 块。
//   - 注释字符 `#`：单条 # 是 nginx 注释合法字符,但允许它会让"值末尾混
//     注释"导致后续半行被忽略,渲染审计 diff 会变得不可读。直接禁。
//
// 允许 `;` 与 `\n` —— 否则就退化成 NginxValue 没法表达 ratelimit/security
// 这种多指令组合。空字符串放行(空 Extra 是合法的,代表用户没填)。
var nginxBlockBad = regexp.MustCompile("[{}#`\\\\]")

// NginxBlock 限定上限 8 KB——超过就基本是被人塞 payload 而非真用配置。
const nginxBlockMaxLen = 8 * 1024

func NginxBlock(v string) error {
	if v == "" {
		return nil
	}
	if len(v) > nginxBlockMaxLen {
		return fmt.Errorf("Extra 内容超过 %d 字节上限", nginxBlockMaxLen)
	}
	if nginxBlockBad.MatchString(v) {
		return errors.New("Extra 不允许出现 { } # ` \\ 字符（避免提前关闭 location 块或注入 shell 元字符）")
	}
	if strings.Contains(v, "$(") {
		return errors.New("Extra 不允许包含 $( 命令替换序列")
	}
	return nil
}

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

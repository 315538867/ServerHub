package deployer

import "strings"

// shellQuote wraps s in single quotes, escaping embedded single quotes so the
// result is safe to splice into a POSIX shell command.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

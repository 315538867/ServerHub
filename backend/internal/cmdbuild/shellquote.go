// Package cmdbuild 是 release apply 命令拼装的内部共享层。
//
// 仅供 backend/usecase/* 与 backend/adapters/runtime/* 使用(Go internal 规则
// 限制 —— 包外不可 import)。函数行为与 v1 pkg/deployer 字节级等价,后续 R6/R8
// 演进时此包是唯一变更入口。
package cmdbuild

import "strings"

// ShellQuote 把 s 包成单引号字符串,转义内嵌的单引号,使其可以安全拼接到
// POSIX shell 命令里。
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

// DirOf 返回路径的目录部分(兼容相对路径)。
func DirOf(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[:i]
	}
	return "."
}

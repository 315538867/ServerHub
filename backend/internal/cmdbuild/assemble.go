package cmdbuild

import (
	"fmt"
	"strings"

	"github.com/serverhub/serverhub/domain"
)

// WorkdirSetup 返回 [mkdir -p X, cd X] 两条命令。WorkDir 为空时退化为
// /tmp/serverhub-svc-<id>(与 v1 默认一致)。
func WorkdirSetup(svc domain.Service) []string {
	wd := svc.WorkDir
	if wd == "" {
		wd = "/tmp/serverhub-svc-" + fmt.Sprint(svc.ID)
	}
	return []string{
		fmt.Sprintf("mkdir -p %s", ShellQuote(wd)),
		fmt.Sprintf("cd %s", ShellQuote(wd)),
	}
}

// Workdir 返回最终工作目录(供 native pgrep 等场景复用)。
func Workdir(svc domain.Service) string {
	if svc.WorkDir != "" {
		return svc.WorkDir
	}
	return "/tmp/serverhub-svc-" + fmt.Sprint(svc.ID)
}

// Assemble 把 envPrefix 与 parts 串成最终 `bash -c '...'` 命令。
// 行为与 v1 pkg/deployer.buildReleaseCmd 末两行字节级等价。
func Assemble(envPrefix string, parts ...string) string {
	full := envPrefix + "set -e; " + strings.Join(parts, " && ")
	return "bash -c " + ShellQuote(full)
}

package ssl

import (
	"os/exec"
)

// execShell 跑一条本地 shell 命令并合并 stdout/stderr。仅给 parseExpiryFromPEM 用。
func execShell(cmd string) (string, error) {
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	return string(out), err
}

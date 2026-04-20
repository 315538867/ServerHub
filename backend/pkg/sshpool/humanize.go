package sshpool

import "strings"

// HumanizeErr detects common sudo/permission failures from remote command
// output and returns a friendly Chinese message plus actionable remediation.
// Returns the trimmed original output if no known pattern matches.
func HumanizeErr(out string) string {
	s := strings.TrimSpace(out)
	low := strings.ToLower(s)
	switch {
	case strings.Contains(low, "sudo: a password is required"),
		strings.Contains(low, "a terminal is required to read the password"),
		strings.Contains(low, "askpass"):
		return "SSH 用户没有免密 sudo 权限。请在目标服务器执行：\n" +
			"  echo \"$USER ALL=(ALL) NOPASSWD: ALL\" | sudo tee /etc/sudoers.d/serverhub\n" +
			"  sudo chmod 0440 /etc/sudoers.d/serverhub\n" +
			"（或仅授权 nginx/systemctl/tee/rm/ln/mkdir/tail 等最小命令集）"
	case strings.Contains(low, "sudo: command not found"),
		strings.Contains(low, "sudo: not found"):
		return "目标服务器未安装 sudo，请使用 root 账号连接，或先安装 sudo"
	case strings.Contains(low, "is not in the sudoers file"),
		strings.Contains(low, "not allowed to execute"):
		return "SSH 用户不在 sudoers 列表中。请联系管理员将该用户加入 sudoers（建议配置 NOPASSWD）"
	case strings.Contains(low, "permission denied"):
		return "权限不足：" + s + "\n建议配置 SSH 用户的免密 sudo 权限"
	}
	return s
}

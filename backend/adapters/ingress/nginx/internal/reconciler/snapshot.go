package reconciler

import (
	"fmt"
	"strings"
	"time"

	"github.com/serverhub/serverhub/adapters/ingress/nginx/internal/render"
	"github.com/serverhub/serverhub/pkg/runner"
	"github.com/serverhub/serverhub/pkg/safeshell"
)

// BackupDir 是远端备份根目录。Snapshot 会确保其存在。
const BackupDir = "/var/lib/serverhub/nginx-bak"

// Snapshot 把当前 nginx 配置整树（profile.NginxConfDir）打包到 BackupDir/<edge>-<ts>.tar.gz，
// 用于 reload 失败时的 breakglass 还原。返回备份路径。
//
// 旧签名 Snapshot(r, id) 等价于 SnapshotWith(r, id, DefaultProfile())。
func Snapshot(r runner.Runner, edgeID uint) (string, error) {
	return SnapshotWith(r, edgeID, render.DefaultProfile())
}

// SnapshotWith 用 profile.NginxConfDir 作为 tar 起始目录做快照。其它行为同 Snapshot。
//
// 清理 7 天前的旧备份，避免磁盘膨胀。清理失败不返回错误（仅写日志级影响）。
func SnapshotWith(r runner.Runner, edgeID uint, p render.Profile) (string, error) {
	p = render.NormalizeProfile(p)
	ts := time.Now().UTC().Format("20060102T150405Z")
	path := fmt.Sprintf("%s/%d-%s.tar.gz", BackupDir, edgeID, ts)

	cmds := []string{
		"sudo -n mkdir -p " + safeshell.Quote(BackupDir),
		"sudo -n tar -C " + safeshell.Quote(p.NginxConfDir) + " -czf " + safeshell.Quote(path) + " .",
		// 清理 7 天前
		"sudo -n find " + safeshell.Quote(BackupDir) + " -maxdepth 1 -type f -name '*.tar.gz' -mtime +7 -delete 2>/dev/null || true",
	}
	out, err := r.Run(strings.Join(cmds, " && "))
	if err != nil {
		return "", fmt.Errorf("snapshot 失败: %s: %w", strings.TrimSpace(out), err)
	}
	return path, nil
}

// Restore 从 backupPath 还原 profile.NginxConfDir。仅作为 breakglass：
// Reconciler 自身的回滚走 Differ 反向回写，无需本函数。
//
// 旧签名 Restore(r, path) 等价于 RestoreWith(r, path, DefaultProfile())。
func Restore(r runner.Runner, backupPath string) error {
	return RestoreWith(r, backupPath, render.DefaultProfile())
}

// RestoreWith 同 Restore，但允许指定 Profile（多实例必需）。
func RestoreWith(r runner.Runner, backupPath string, p render.Profile) error {
	if backupPath == "" {
		return fmt.Errorf("restore: 备份路径为空")
	}
	p = render.NormalizeProfile(p)
	cmd := "sudo -n tar -C " + safeshell.Quote(p.NginxConfDir) + " -xzf " + safeshell.Quote(backupPath)
	out, err := r.Run(cmd)
	if err != nil {
		return fmt.Errorf("restore 失败: %s: %w", strings.TrimSpace(out), err)
	}
	return nil
}

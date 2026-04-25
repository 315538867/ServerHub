package nginxops

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/nginxrender"
	"github.com/serverhub/serverhub/pkg/runner"
	"github.com/serverhub/serverhub/pkg/safeshell"
	"github.com/serverhub/serverhub/pkg/sysinfo"
)

// runnerFactory 让测试可以注入 fake runner，省去 SSH 依赖。
// 默认实现绑定到 runner.For（线上路径）。
type runnerFactory func(*model.Server, *config.Config) (runner.Runner, error)

var defaultRunnerFactory runnerFactory = runner.For

// SetRunnerFactory 仅供测试覆盖；返回旧 factory 以便恢复。
func SetRunnerFactory(f runnerFactory) runnerFactory {
	old := defaultRunnerFactory
	defaultRunnerFactory = f
	return old
}

// Apply 是 Reconciler 的同步入口。流程：
//
//   1. Acquire(edgeID)
//   2. Capability 守卫：runner.Capability != CapFull → 423 等价错误
//   3. INSERT audit_apply 占位拿到 ID（defer UPDATE 写完整记录）
//   4. LoadDesired：DB → IngressCtx
//   5. nginxrender.Render → desired ConfigFile
//   6. Snapshot → backupPath（breakglass）
//   7. Inspect + Diff → changeset；空 → NoOp
//   8. 写入差异（add/update 用 WriteRemoteFile；delete 用 rm；维护 sites-enabled symlink）
//   9. nginx -t；失败 → 用 Diff 反向回滚（add 回 rm；update/delete 回写旧内容）+ RolledBack
//   10. nginx -s reload
//   11. UPDATE audit + ingress.status
//
// actor 为执行用户 ID（可能为 nil，例如系统触发）。
func Apply(ctx context.Context, db *gorm.DB, cfg *config.Config, edgeID uint, actor *uint) (ApplyResult, error) {
	release := Acquire(edgeID)
	defer release()

	start := time.Now()

	var edge model.Server
	if err := db.First(&edge, edgeID).Error; err != nil {
		return ApplyResult{}, fmt.Errorf("加载 edge server id=%d: %w", edgeID, err)
	}

	r, err := defaultRunnerFactory(&edge, cfg)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("无法连接 edge: %w", err)
	}
	defer r.Close()

	if r.Capability() != sysinfo.CapFull {
		return ApplyResult{}, fmt.Errorf("apply 拒绝：edge 当前 capability=%s，需要 full（裸机或 --pid=host 容器）", r.Capability())
	}

	audit := model.AuditApply{EdgeServerID: edgeID, ActorUserID: actor}
	if err := db.Create(&audit).Error; err != nil {
		return ApplyResult{}, fmt.Errorf("创建 audit 失败: %w", err)
	}

	res := ApplyResult{AuditID: audit.ID}
	defer func() {
		audit.DurationMs = int(time.Since(start).Milliseconds())
		audit.ChangesetDiff = formatChangeset(res.Changes)
		audit.NginxTOutput = res.Output
		audit.RolledBack = res.RolledBack
		audit.BackupPath = res.BackupPath
		_ = db.Save(&audit).Error // 审计写入失败也不掩盖主流程错误
	}()

	desiredCtxs, err := LoadDesired(db, &edge)
	if err != nil {
		return res, err
	}
	desired, err := nginxrender.Render(desiredCtxs)
	if err != nil {
		return res, fmt.Errorf("render 失败: %w", err)
	}

	backupPath, err := Snapshot(r, edgeID)
	if err != nil {
		return res, err
	}
	res.BackupPath = backupPath

	actual, err := Inspect(r)
	if err != nil {
		return res, err
	}
	changes := Diff(desired, actual)
	res.Changes = changes
	if len(changes) == 0 {
		res.NoOp = true
		_ = updateIngressStatus(db, edgeID)
		return res, nil
	}

	if err := writeChanges(r, changes); err != nil {
		return res, err
	}
	if err := syncSitesEnabled(r, desired); err != nil {
		return res, err
	}

	out, terr := r.Run("sudo -n nginx -t 2>&1")
	res.Output = strings.TrimSpace(out)
	if terr != nil {
		// nginx -t 失败：反向回滚 + 不 reload
		if rerr := rollback(r, changes); rerr != nil {
			res.Output += "\n[rollback 也失败] " + rerr.Error()
		}
		res.RolledBack = true
		return res, fmt.Errorf("nginx -t 失败: %s", res.Output)
	}

	rout, rerr := r.Run("sudo -n nginx -s reload 2>&1")
	if rout != "" {
		res.Output += "\n" + strings.TrimSpace(rout)
	}
	if rerr != nil {
		if rb := rollback(r, changes); rb != nil {
			res.Output += "\n[rollback 也失败] " + rb.Error()
		}
		res.RolledBack = true
		return res, fmt.Errorf("nginx -s reload 失败: %w", rerr)
	}

	_ = updateIngressStatus(db, edgeID)
	return res, nil
}

// DryRun 走与 Apply 同样的渲染 / Inspect / Diff 流程，但不写盘也不 reload，
// 仅返回 changeset 给 UI 预览。亦不写 audit。
func DryRun(ctx context.Context, db *gorm.DB, cfg *config.Config, edgeID uint) ([]Change, error) {
	var edge model.Server
	if err := db.First(&edge, edgeID).Error; err != nil {
		return nil, fmt.Errorf("加载 edge server id=%d: %w", edgeID, err)
	}
	r, err := defaultRunnerFactory(&edge, cfg)
	if err != nil {
		return nil, fmt.Errorf("无法连接 edge: %w", err)
	}
	defer r.Close()

	desiredCtxs, err := LoadDesired(db, &edge)
	if err != nil {
		return nil, err
	}
	desired, err := nginxrender.Render(desiredCtxs)
	if err != nil {
		return nil, fmt.Errorf("render 失败: %w", err)
	}
	actual, err := Inspect(r)
	if err != nil {
		return nil, err
	}
	return Diff(desired, actual), nil
}

// writeChanges 把 Diff 的 add/update/delete 落盘到远端。
func writeChanges(r runner.Runner, changes []Change) error {
	for _, c := range changes {
		switch c.Kind {
		case ChangeAdd, ChangeUpdate:
			if err := ensureParentDir(r, c.Path); err != nil {
				return err
			}
			cmd := safeshell.WriteRemoteFile(c.Path, c.NewContent, true)
			if out, err := r.Run(cmd); err != nil {
				return fmt.Errorf("写入 %s 失败: %s: %w", c.Path, strings.TrimSpace(out), err)
			}
		case ChangeDelete:
			if out, err := r.Run("sudo -n rm -f " + safeshell.Quote(c.Path)); err != nil {
				return fmt.Errorf("删除 %s 失败: %s: %w", c.Path, strings.TrimSpace(out), err)
			}
		}
	}
	return nil
}

// rollback 按 Diff 反向把远端还原到 apply 前。失败合并错误返回。
func rollback(r runner.Runner, changes []Change) error {
	var errs []string
	for _, c := range changes {
		switch c.Kind {
		case ChangeAdd:
			if out, err := r.Run("sudo -n rm -f " + safeshell.Quote(c.Path)); err != nil {
				errs = append(errs, fmt.Sprintf("rm %s: %s", c.Path, strings.TrimSpace(out)))
			}
		case ChangeUpdate, ChangeDelete:
			cmd := safeshell.WriteRemoteFile(c.Path, c.OldContent, true)
			if out, err := r.Run(cmd); err != nil {
				errs = append(errs, fmt.Sprintf("restore %s: %s", c.Path, strings.TrimSpace(out)))
			}
		}
	}
	// rollback 后也要修一遍 sites-enabled symlink（因为我们可能新增了 *-sh.conf）
	_ = syncSitesEnabledFromRemote(r)
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

// syncSitesEnabled 根据 desired 中的 SitesAvailable 文件维护 sites-enabled
// 下的 symlink：所有 *-sh.conf 与 hub 文件都启用；其余清理。
func syncSitesEnabled(r runner.Runner, desired []nginxrender.ConfigFile) error {
	var enable []string
	for _, f := range desired {
		if !strings.HasPrefix(f.Path, nginxrender.SitesAvailableDir+"/") {
			continue
		}
		base := strings.TrimPrefix(f.Path, nginxrender.SitesAvailableDir+"/")
		switch {
		case strings.HasSuffix(base, ".conf"):
			// foo-sh.conf → 链接名 foo-sh
			link := strings.TrimSuffix(base, ".conf")
			enable = append(enable, link+":"+base)
		case base == nginxrender.HubSiteName:
			enable = append(enable, base+":"+base)
		}
	}

	// 先清理我们管的旧 link：sites-enabled 下名为 *-sh 或 HubSiteName 的链接
	clearCmd := fmt.Sprintf(
		"sudo -n find %s -maxdepth 1 \\( -name '*-sh' -o -name %s \\) -delete 2>/dev/null || true",
		safeshell.Quote(nginxrender.SitesEnabledDir), safeshell.Quote(nginxrender.HubSiteName),
	)
	if out, err := r.Run(clearCmd); err != nil {
		return fmt.Errorf("清理 sites-enabled 失败: %s: %w", strings.TrimSpace(out), err)
	}

	for _, item := range enable {
		parts := strings.SplitN(item, ":", 2)
		linkName, target := parts[0], parts[1]
		linkPath := nginxrender.SitesEnabledDir + "/" + linkName
		targetPath := nginxrender.SitesAvailableDir + "/" + target
		cmd := fmt.Sprintf("sudo -n ln -sf %s %s",
			safeshell.Quote(targetPath), safeshell.Quote(linkPath))
		if out, err := r.Run(cmd); err != nil {
			return fmt.Errorf("创建 symlink %s → %s 失败: %s: %w", linkPath, targetPath, strings.TrimSpace(out), err)
		}
	}
	return nil
}

// syncSitesEnabledFromRemote 在 rollback 之后再扫一次远端 sites-available 修复 symlink。
func syncSitesEnabledFromRemote(r runner.Runner) error {
	cmd := fmt.Sprintf(`set -eu
sudo -n find %s -maxdepth 1 \( -name '*-sh' -o -name %s \) -delete 2>/dev/null || true
shopt -s nullglob 2>/dev/null || true
for f in %s/*-sh.conf; do
  [ -f "$f" ] || continue
  base=$(basename "$f" .conf)
  sudo -n ln -sf "$f" %s/"$base"
done
if [ -f %s/%s ]; then
  sudo -n ln -sf %s/%s %s/%s
fi
`,
		safeshell.Quote(nginxrender.SitesEnabledDir),
		safeshell.Quote(nginxrender.HubSiteName),
		nginxrender.SitesAvailableDir,
		safeshell.Quote(nginxrender.SitesEnabledDir),
		nginxrender.SitesAvailableDir, nginxrender.HubSiteName,
		nginxrender.SitesAvailableDir, nginxrender.HubSiteName,
		nginxrender.SitesEnabledDir, nginxrender.HubSiteName,
	)
	_, err := r.Run("bash -c " + safeshell.Quote(cmd))
	return err
}

// ensureParentDir 在写文件前确保父目录存在（首次 apply 触发）。
func ensureParentDir(r runner.Runner, path string) error {
	idx := strings.LastIndex(path, "/")
	if idx <= 0 {
		return nil
	}
	dir := path[:idx]
	if out, err := r.Run("sudo -n mkdir -p " + safeshell.Quote(dir)); err != nil {
		return fmt.Errorf("创建目录 %s 失败: %s: %w", dir, strings.TrimSpace(out), err)
	}
	return nil
}

// updateIngressStatus 把 edge 上所有 ingress.status 标记为 applied + last_applied_at=now。
func updateIngressStatus(db *gorm.DB, edgeID uint) error {
	now := time.Now()
	return db.Model(&model.Ingress{}).
		Where("edge_server_id = ?", edgeID).
		Updates(map[string]any{"status": "applied", "last_applied_at": &now}).Error
}

// formatChangeset 把 Change 列表压成一段适合存进 audit 的人类可读 diff。
func formatChangeset(changes []Change) string {
	if len(changes) == 0 {
		return ""
	}
	var b strings.Builder
	for _, c := range changes {
		switch c.Kind {
		case ChangeAdd:
			fmt.Fprintf(&b, "+ %s (%s)\n", c.Path, c.NewHash[:8])
		case ChangeUpdate:
			fmt.Fprintf(&b, "~ %s (%s → %s)\n", c.Path, c.OldHash[:8], c.NewHash[:8])
		case ChangeDelete:
			fmt.Fprintf(&b, "- %s\n", c.Path)
		}
	}
	return b.String()
}

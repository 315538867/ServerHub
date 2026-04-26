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
//   4. LoadProfile：DB → nginxrender.Profile（多实例路径/命令）
//   5. LoadDesired：DB → IngressCtx
//   6. nginxrender.RenderWith → desired ConfigFile
//   7. SnapshotWith → backupPath（breakglass）
//   8. InspectWith + Diff → changeset；空 → NoOp
//   9. 写入差异（add/update 用 WriteRemoteFile；delete 用 rm；维护 sites-enabled symlink）
//   10. profile.TestCmd；失败 → 用 Diff 反向回滚（add 回 rm；update/delete 回写旧内容）+ RolledBack
//   11. profile.ReloadCmd
//   12. UPDATE audit + ingress.status
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

	profile, _, err := LoadProfile(db, edgeID)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("加载 nginx profile 失败: %w", err)
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

	desiredCtxs, err := LoadDesired(db, &edge, cfg.Security.AESKey, profile)
	if err != nil {
		return res, err
	}
	desired, err := nginxrender.RenderWith(desiredCtxs, profile)
	if err != nil {
		return res, fmt.Errorf("render 失败: %w", err)
	}

	backupPath, err := SnapshotWith(r, edgeID, profile)
	if err != nil {
		return res, err
	}
	res.BackupPath = backupPath

	// cert 落盘必须在 Snapshot 之后（保证回滚有快照可用），但在 Inspect/Diff
	// 之前——避免反复 reload。证书内容由 IngressCtx 携带，写盘是幂等的（base64
	// tee），所以即使 PEM 没变也只是 noop 写入，不会扰动 nginx。
	if err := syncCerts(r, desiredCtxs); err != nil {
		return res, fmt.Errorf("证书落盘失败: %w", err)
	}

	actual, err := InspectWith(r, profile)
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
	if err := syncSitesEnabledWith(r, desired, profile); err != nil {
		return res, err
	}

	// stream 路由的 include 不能落在 sites-enabled（被 http{} include），必须挂在
	// nginx.conf 顶层。这步独立于 Diff：直接读 nginx.conf、做带标记的幂等 rewrite，
	// 把变化（如有）作为合成 Change 加进 rollback 列表。
	streamWanted := desiredHasStreams(desired, profile)
	if streamChange, err := ensureStreamIncludeWith(r, profile, streamWanted); err != nil {
		return res, err
	} else if streamChange != nil {
		changes = append(changes, *streamChange)
		res.Changes = changes
	}

	out, terr := r.Run(profile.TestCmd)
	res.Output = strings.TrimSpace(out)
	if terr != nil {
		// nginx -t 失败：反向回滚 + 不 reload
		if rerr := rollbackWith(r, changes, profile); rerr != nil {
			res.Output += "\n[rollback 也失败] " + rerr.Error()
		}
		res.RolledBack = true
		_ = markPendingAsBroken(db, edgeID)
		return res, fmt.Errorf("nginx -t 失败: %s", res.Output)
	}

	rout, rerr := r.Run(profile.ReloadCmd)
	if rout != "" {
		res.Output += "\n" + strings.TrimSpace(rout)
	}
	if rerr != nil {
		if rb := rollbackWith(r, changes, profile); rb != nil {
			res.Output += "\n[rollback 也失败] " + rb.Error()
		}
		res.RolledBack = true
		_ = markPendingAsBroken(db, edgeID)
		return res, fmt.Errorf("nginx -s reload 失败: %w", rerr)
	}

	_ = updateIngressStatus(db, edgeID)
	return res, nil
}

// DryRun 走与 Apply 同样的渲染 / Inspect / Diff 流程，但不写盘也不 reload，
// 仅返回 changeset 给 UI 预览。不写 audit。
//
// 副作用：把比对结果回写到 ingress.status —— 空 changes 表示数据库期望与远端
// 一致，所有 ingress 标 applied；非空 changes 表示存在漂移，所有 ingress 标
// drift。这样"预览变更"按钮顺便完成了漂移扫描，状态字段反映最近一次扫描的
// 真相，而不仅仅是上次 Apply 的结果。
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

	profile, _, err := LoadProfile(db, edgeID)
	if err != nil {
		return nil, fmt.Errorf("加载 nginx profile 失败: %w", err)
	}

	desiredCtxs, err := LoadDesired(db, &edge, cfg.Security.AESKey, profile)
	if err != nil {
		return nil, err
	}
	desired, err := nginxrender.RenderWith(desiredCtxs, profile)
	if err != nil {
		return nil, fmt.Errorf("render 失败: %w", err)
	}
	actual, err := InspectWith(r, profile)
	if err != nil {
		return nil, err
	}
	changes := Diff(desired, actual)
	_ = writeDriftStatus(db, edgeID, len(changes) == 0)
	return changes, nil
}

// syncCerts 把 IngressCtx 携带的 PEM 内容写到 canonical 路径。
//
// 仅处理 TLSCertContent 与 TLSKeyContent 都非空的 ctx——空内容意味着 cert 由
// 外部维护（例如 letsencrypt live 路径），reconciler 不能也不应改写。
//
// 同 (CertPath, KeyPath) 可能被多个 ingress 共享同域名引用，做去重避免重复
// 写盘；底层 tee + base64 已经幂等，去重只是省一次 RTT。
//
// privkey 用 0600 权限（chmod 单独跟一刀），fullchain 默认 0644 即可。
func syncCerts(r runner.Runner, ctxs []nginxrender.IngressCtx) error {
	type item struct{ path, content string; mode string }
	seen := make(map[string]struct{})
	var items []item
	for _, ig := range ctxs {
		if ig.TLSCertContent == "" || ig.TLSKeyContent == "" {
			continue
		}
		if ig.TLSCertPath == "" || ig.TLSKeyPath == "" {
			return fmt.Errorf("ingress edge=%d domain=%q TLS PEM 非空但路径为空", ig.EdgeServerID, ig.Domain)
		}
		if _, ok := seen[ig.TLSCertPath]; !ok {
			seen[ig.TLSCertPath] = struct{}{}
			items = append(items, item{ig.TLSCertPath, ig.TLSCertContent, "0644"})
		}
		if _, ok := seen[ig.TLSKeyPath]; !ok {
			seen[ig.TLSKeyPath] = struct{}{}
			items = append(items, item{ig.TLSKeyPath, ig.TLSKeyContent, "0600"})
		}
	}
	for _, it := range items {
		if err := ensureParentDir(r, it.path); err != nil {
			return err
		}
		cmd := safeshell.WriteRemoteFile(it.path, it.content, true)
		if out, err := r.Run(cmd); err != nil {
			return fmt.Errorf("写入 %s 失败: %s: %w", it.path, strings.TrimSpace(out), err)
		}
		if out, err := r.Run("sudo -n chmod " + it.mode + " " + safeshell.Quote(it.path)); err != nil {
			return fmt.Errorf("chmod %s 失败: %s: %w", it.path, strings.TrimSpace(out), err)
		}
	}
	return nil
}

// writeDriftStatus 把 edge 上所有 ingress 的 status 翻成 applied 或 drift。
//   - inSync=true → applied（同时记录 last_applied_at=now，与 Apply 的语义对齐）
//   - inSync=false → drift（last_applied_at 不动，便于排查"上次成功 apply 后多久开始漂移"）
//
// 错误吞掉：drift 扫描是辅助信息，不能因为状态写失败让用户拿不到 changes。
func writeDriftStatus(db *gorm.DB, edgeID uint, inSync bool) error {
	if inSync {
		return updateIngressStatus(db, edgeID)
	}
	return db.Model(&model.Ingress{}).
		Where("edge_server_id = ?", edgeID).
		Update("status", "drift").Error
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

// rollback 等价于 rollbackWith(r, changes, DefaultProfile())，老调用方可继续使用。
func rollback(r runner.Runner, changes []Change) error {
	return rollbackWith(r, changes, nginxrender.DefaultProfile())
}

// rollbackWith 按 Diff 反向把远端还原到 apply 前。失败合并错误返回。
// Profile 用于 sites-enabled symlink 修复，多实例必需。
func rollbackWith(r runner.Runner, changes []Change, p nginxrender.Profile) error {
	p = nginxrender.NormalizeProfile(p)
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
	_ = syncSitesEnabledFromRemoteWith(r, p)
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

// syncSitesEnabled 等价于 syncSitesEnabledWith(..., DefaultProfile())。
func syncSitesEnabled(r runner.Runner, desired []nginxrender.ConfigFile) error {
	return syncSitesEnabledWith(r, desired, nginxrender.DefaultProfile())
}

// syncSitesEnabledWith 根据 desired 中的 SitesAvailable 文件维护 sites-enabled
// 下的 symlink：所有 *-sh.conf 与 hub 文件都启用；其余清理。
// Profile 决定 SitesAvailableDir / SitesEnabledDir / HubSiteName。
func syncSitesEnabledWith(r runner.Runner, desired []nginxrender.ConfigFile, p nginxrender.Profile) error {
	p = nginxrender.NormalizeProfile(p)
	var enable []string
	for _, f := range desired {
		if !strings.HasPrefix(f.Path, p.SitesAvailableDir+"/") {
			continue
		}
		base := strings.TrimPrefix(f.Path, p.SitesAvailableDir+"/")
		switch {
		case strings.HasSuffix(base, ".conf"):
			// foo-sh.conf → 链接名 foo-sh
			link := strings.TrimSuffix(base, ".conf")
			enable = append(enable, link+":"+base)
		case base == p.HubSiteName:
			enable = append(enable, base+":"+base)
		}
	}

	// 先清理我们管的旧 link：sites-enabled 下名为 *-sh 或 HubSiteName 的链接
	clearCmd := fmt.Sprintf(
		"sudo -n find %s -maxdepth 1 \\( -name '*-sh' -o -name %s \\) -delete 2>/dev/null || true",
		safeshell.Quote(p.SitesEnabledDir), safeshell.Quote(p.HubSiteName),
	)
	if out, err := r.Run(clearCmd); err != nil {
		return fmt.Errorf("清理 sites-enabled 失败: %s: %w", strings.TrimSpace(out), err)
	}

	for _, item := range enable {
		parts := strings.SplitN(item, ":", 2)
		linkName, target := parts[0], parts[1]
		linkPath := p.SitesEnabledDir + "/" + linkName
		targetPath := p.SitesAvailableDir + "/" + target
		cmd := fmt.Sprintf("sudo -n ln -sf %s %s",
			safeshell.Quote(targetPath), safeshell.Quote(linkPath))
		if out, err := r.Run(cmd); err != nil {
			return fmt.Errorf("创建 symlink %s → %s 失败: %s: %w", linkPath, targetPath, strings.TrimSpace(out), err)
		}
	}
	return nil
}

// syncSitesEnabledFromRemote 等价于 syncSitesEnabledFromRemoteWith(..., DefaultProfile())。
func syncSitesEnabledFromRemote(r runner.Runner) error {
	return syncSitesEnabledFromRemoteWith(r, nginxrender.DefaultProfile())
}

// syncSitesEnabledFromRemoteWith 在 rollback 之后再扫一次远端 sites-available 修复 symlink。
func syncSitesEnabledFromRemoteWith(r runner.Runner, p nginxrender.Profile) error {
	p = nginxrender.NormalizeProfile(p)
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
		safeshell.Quote(p.SitesEnabledDir),
		safeshell.Quote(p.HubSiteName),
		p.SitesAvailableDir,
		safeshell.Quote(p.SitesEnabledDir),
		p.SitesAvailableDir, p.HubSiteName,
		p.SitesAvailableDir, p.HubSiteName,
		p.SitesEnabledDir, p.HubSiteName,
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

// markPendingAsBroken 在 Apply 失败 + rollback 之后,把"用户改完还没成功 apply 的"
// (status=pending) 标 broken,让 UI 能看到这次提交确实没生效。已 applied 的不动 ——
// rollback 完远端就回到了 apply 前状态,正常运行的 ingress 不应被波及。
func markPendingAsBroken(db *gorm.DB, edgeID uint) error {
	return db.Model(&model.Ingress{}).
		Where("edge_server_id = ? AND status = ?", edgeID, "pending").
		Update("status", "broken").Error
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

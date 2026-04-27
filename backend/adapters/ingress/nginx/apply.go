// apply.go 是 nginx adapter 的 usecase 入口公共门面。
//
// 设计动机:`internal/reconciler` 实现了完整的 differ + snapshot + reload + 审计闭环,
// 但 Go internal/ 可见性规则禁止 `usecase/` 直接 import;同时 Backend.Render port 入参
// `[]domain.IngressRoute` 与 reconciler 实际需要的 `IngressCtx`(已聚合 EdgeServerID/
// FileStem/MatchKind/TLS/profile)类型不匹配,无法走 port 通路。
//
// R5 折中方案:在 adapter 包根 re-export 4 个公共符号作为 thin wrapper,内部仍 delegate
// 到 internal/reconciler。usecase 直接 import 本包 + 调本文件的 Apply/DryRun 即可。
//
// R6 计划:Backend.Render port 重构成接 IngressCtx 后,本文件可删,usecase 走端口。
package nginx

import (
	"context"

	"github.com/serverhub/serverhub/adapters/ingress/nginx/internal/reconciler"
	"github.com/serverhub/serverhub/config"
	"gorm.io/gorm"
)

// Change 是单条文件级 reconcile 变更,与 internal/reconciler.Change 同型。
type Change = reconciler.Change

// ApplyResult 是 Apply 一次调用的汇总结果,与 internal/reconciler.ApplyResult 同型。
type ApplyResult = reconciler.ApplyResult

// ChangeKind 用于 Change.Kind,与 internal/reconciler.ChangeKind 同型。
type ChangeKind = reconciler.ChangeKind

// Apply 把 DB 中 ingress 路由 reconcile 到远端 nginx:加锁 → 加载 desired/current →
// differ → 写文件 → nginx -t → reload → 审计入库。
//
// db / cfg 入参是 R5 平移期妥协,R6 切 ports.IngressRepo 后由 usecase 在外面装配。
func Apply(ctx context.Context, db *gorm.DB, cfg *config.Config, edgeID uint, actor *uint) (ApplyResult, error) {
	return reconciler.Apply(ctx, db, cfg, edgeID, actor)
}

// DryRun 只做 differ 不落盘,返回预期变更集供 UI 预览。
func DryRun(ctx context.Context, db *gorm.DB, cfg *config.Config, edgeID uint) ([]Change, error) {
	return reconciler.DryRun(ctx, db, cfg, edgeID)
}

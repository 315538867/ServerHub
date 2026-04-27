// Package usecase: ingress_apply.go 是 Ingress reconcile 的 usecase 入口。
//
// R5 平移期定位:thin pass-through 到 adapters/ingress/nginx 包根公共门面
// (Apply / DryRun)。adapter 内部仍用 internal/reconciler 跑 differ + snapshot +
// reload + 审计闭环。
//
// **不**走 ingress.Default.MustGet("nginx").Render port:port 入参 []domain.IngressRoute
// 与 reconciler 实际需要的 IngressCtx(已聚合 EdgeServerID/FileStem/MatchKind/TLS/profile)
// 类型不匹配;同时 internal/reconciler 受 Go internal/ 可见性限制不可被 usecase 直接 import。
// R6 重构 Backend.Render port 后,本文件可改回走端口。
package usecase

import (
	"context"

	nginxingress "github.com/serverhub/serverhub/adapters/ingress/nginx"
	"github.com/serverhub/serverhub/config"
	"gorm.io/gorm"
)

// ApplyIngressResult 是 ApplyIngress 一次调用的汇总结果,与 adapter 的 ApplyResult 同型。
type ApplyIngressResult = nginxingress.ApplyResult

// IngressChange 是单条文件级 reconcile 变更,与 adapter 的 Change 同型。
type IngressChange = nginxingress.Change

// ApplyIngress 把 DB 中 ingress 路由 reconcile 到指定 edge 的远端 nginx。
//
// TODO R6: 切 ports.IngressRepo,移除 db *gorm.DB 入参;Backend.Render port 重构后回归走端口。
func ApplyIngress(ctx context.Context, db *gorm.DB, cfg *config.Config, edgeID uint, actor *uint) (ApplyIngressResult, error) {
	return nginxingress.Apply(ctx, db, cfg, edgeID, actor)
}

// DryRunIngress 只做 differ 不落盘,返回预期变更集供 UI 预览。
//
// TODO R6: 同 ApplyIngress。
func DryRunIngress(ctx context.Context, db *gorm.DB, cfg *config.Config, edgeID uint) ([]IngressChange, error) {
	return nginxingress.DryRun(ctx, db, cfg, edgeID)
}

// Package usecase: reconcile.go 收口后台 reconcile 业务逻辑。
//
// 原先散落在 pkg/scheduler/reconciler.go 的业务编排（CAS sync_status +
// ApplyRelease + 状态回写）迁入此处；pkg/scheduler 仅保留定时器入口与
// lastReconcileAt 节流状态。
//
// TODO R7: 切 ports interface，移除 db *gorm.DB 入参。
package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/repo"
	"gorm.io/gorm"
)

// ListAutoSyncServices 返回 auto_sync=true 且已绑定 CurrentReleaseID 的 Service 列表。
func ListAutoSyncServices(ctx context.Context, db *gorm.DB) ([]domain.Service, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	return repo.ListAutoSyncServices(ctx, db)
}

// ReconcileAll 拉取全部 auto_sync=true 且已绑定 CurrentReleaseID 的 Service，
// 逐个调用 ReconcileOne。调用方负责节流（ticker / 并发控制）。
func ReconcileAll(ctx context.Context, db *gorm.DB, cfg *config.Config) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	svcs, err := repo.ListAutoSyncServices(ctx, db)
	if err != nil {
		fmt.Printf("[reconcile] query auto-sync services: %v\n", err)
		return
	}
	for _, svc := range svcs {
		go ReconcileOne(ctx, db, cfg, svc.ID)
	}
}

// ReconcileOne 对单个 Service 执行 reconcile：CAS 置 syncing → ApplyRelease → 回写 status。
//
// 若 Service 无 CurrentReleaseID 或 CAS 失败（已被另一 goroutine 处理），静默跳过。
func ReconcileOne(ctx context.Context, db *gorm.DB, cfg *config.Config, serviceID uint) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	svc, err := repo.GetServiceByID(ctx, db, serviceID)
	if err != nil {
		return
	}
	if svc.CurrentReleaseID == nil {
		return
	}

	// CAS: 仅在非 syncing 时推进
	if err := repo.CASServiceSyncStatus(ctx, db, serviceID, "syncing"); err != nil {
		return
	}

	fmt.Printf("[reconcile] service %d (%s) re-applying release %d\n",
		svc.ID, svc.Name, *svc.CurrentReleaseID)

	_, err = ApplyRelease(db, cfg, svc.ID, *svc.CurrentReleaseID, "schedule", nil)
	if err != nil {
		fmt.Printf("[reconcile] service %d apply failed: %v\n", svc.ID, err)
		_ = repo.UpdateServiceSyncStatus(ctx, db, serviceID, "error")
		return
	}
	_ = repo.UpdateServiceSyncStatus(ctx, db, serviceID, "synced")
}

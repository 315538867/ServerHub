package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/usecase"
	"gorm.io/gorm"
)

// lastReconcileAt tracks per-service last run time to respect SyncInterval.
// 调度状态保留在 scheduler 包；业务编排已迁至 usecase.ReconcileOne。
var lastReconcileAt sync.Map // key: svc.ID (uint) → time.Time

// StartReconciler 启动后台 reconcile loop,每 30s 触发一次。
// 逐个 Service 的节流（SyncInterval）+ CAS + ApplyRelease + 状态回写
// 由 usecase.ReconcileOne 完成。
func StartReconciler(db *gorm.DB, cfg *config.Config) {
	go func() {
		reconcileAll(db, cfg) // 启动后立刻跑一次
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			reconcileAll(db, cfg)
		}
	}()
}

func reconcileAll(db *gorm.DB, cfg *config.Config) {
	svcs, err := usecase.ListAutoSyncServices(context.Background(), db)
	if err != nil {
		return
	}
	for _, svc := range svcs {
		svc := svc
		// 节流：遵守 per-service SyncInterval
		if svc.SyncInterval > 0 {
			if last, ok := lastReconcileAt.Load(svc.ID); ok {
				if time.Since(last.(time.Time)) < time.Duration(svc.SyncInterval)*time.Second {
					continue
				}
			}
		}
		lastReconcileAt.Store(svc.ID, time.Now())
		go usecase.ReconcileOne(context.Background(), db, cfg, svc.ID)
	}
}

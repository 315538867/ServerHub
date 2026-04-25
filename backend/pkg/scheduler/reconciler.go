package scheduler

import (
	"fmt"
	"sync"
	"time"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/deployer"
	"gorm.io/gorm"
)

// lastReconcileAt tracks per-service last run time to respect SyncInterval
var lastReconcileAt sync.Map // key: svc.ID (uint) → time.Time

// StartReconciler launches the background reconcile loop. In M3 the old
// DesiredVersion ↔ ActualVersion drift check is gone; reconcile now re-applies
// the Service's CurrentReleaseID so any out-of-band drift (manual changes on
// the host, killed containers) is corrected on schedule.
func StartReconciler(db *gorm.DB, cfg *config.Config) {
	go func() {
		reconcileAll(db, cfg) // immediate first pass
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			reconcileAll(db, cfg)
		}
	}()
}

func reconcileAll(db *gorm.DB, cfg *config.Config) {
	var services []model.Service
	db.Where("auto_sync = ? AND current_release_id IS NOT NULL", true).Find(&services)
	for _, svc := range services {
		go reconcileOne(db, cfg, svc)
	}
}

func reconcileOne(db *gorm.DB, cfg *config.Config, svc model.Service) {
	if svc.CurrentReleaseID == nil {
		return
	}

	// Respect per-service SyncInterval
	if svc.SyncInterval > 0 {
		if last, ok := lastReconcileAt.Load(svc.ID); ok {
			if time.Since(last.(time.Time)) < time.Duration(svc.SyncInterval)*time.Second {
				return
			}
		}
	}
	lastReconcileAt.Store(svc.ID, time.Now())

	fmt.Printf("[reconciler] service %d (%s) re-applying release %d\n", svc.ID, svc.Name, *svc.CurrentReleaseID)

	// Atomic guard: skip if another goroutine already started syncing
	tx := db.Model(&svc).Where("sync_status != ?", "syncing").Update("sync_status", "syncing")
	if tx.RowsAffected == 0 {
		return
	}

	_, err := deployer.ApplyRelease(db, cfg, svc.ID, *svc.CurrentReleaseID, "schedule", nil)
	if err != nil {
		fmt.Printf("[reconciler] service %d apply failed: %v\n", svc.ID, err)
		db.Model(&svc).Update("sync_status", "error")
		return
	}
	db.Model(&svc).Update("sync_status", "synced")
}

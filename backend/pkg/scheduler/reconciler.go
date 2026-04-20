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

// lastReconcileAt tracks per-app last run time to respect SyncInterval
var lastReconcileAt sync.Map // key: app.ID (uint) → time.Time

// StartReconciler launches the background reconcile loop.
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
	var apps []model.Deploy
	db.Where("auto_sync = ? AND desired_version != ''", true).Find(&apps)
	for _, app := range apps {
		go reconcileOne(db, cfg, app)
	}
}

func reconcileOne(db *gorm.DB, cfg *config.Config, app model.Deploy) {
	// Already in sync
	if app.DesiredVersion == app.ActualVersion {
		if app.SyncStatus != "synced" {
			db.Model(&app).Update("sync_status", "synced")
		}
		return
	}

	// Respect per-app SyncInterval
	if app.SyncInterval > 0 {
		if last, ok := lastReconcileAt.Load(app.ID); ok {
			if time.Since(last.(time.Time)) < time.Duration(app.SyncInterval)*time.Second {
				return
			}
		}
	}
	lastReconcileAt.Store(app.ID, time.Now())

	fmt.Printf("[reconciler] app %d (%s): %q → %q\n", app.ID, app.Name, app.ActualVersion, app.DesiredVersion)

	// Atomic guard: skip if another goroutine already started syncing
	tx := db.Model(&app).Where("sync_status != ?", "syncing").Update("sync_status", "syncing")
	if tx.RowsAffected == 0 {
		return
	}

	result := deployer.Run(db, cfg, app, "schedule", nil)
	if !result.Success {
		fmt.Printf("[reconciler] app %d failed: %s\n", app.ID, result.Output)
	}
}

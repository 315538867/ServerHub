// Package retention runs daily cleanup of time-series tables (audit_logs,
// metrics, deploy_logs) and a monthly VACUUM. Policies:
//   - audit_logs: 90 days
//   - metrics:    30 days
//   - deploy_logs: from settings key "deploy_log_keep_days" (default 30)
//   - VACUUM: on the 1st of each month
package retention

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/usecase"
	"gorm.io/gorm"
)

const (
	auditKeepDays   = 90
	metricsKeepDays = 30
)

// Start launches a background goroutine that fires once at startup and then
// daily at 02:00 local time.
func Start(db *gorm.DB) {
	go func() {
		runAll(db)
		for {
			next := nextRunAt(time.Now())
			time.Sleep(time.Until(next))
			runAll(db)
		}
	}()
}

// nextRunAt returns the next 02:00 boundary strictly after t.
func nextRunAt(t time.Time) time.Time {
	y, m, d := t.Date()
	next := time.Date(y, m, d, 2, 0, 0, 0, t.Location())
	if !next.After(t) {
		next = next.Add(24 * time.Hour)
	}
	return next
}

func runAll(db *gorm.DB) {
	cleanTable(db, "audit_logs", auditKeepDays)
	cleanTable(db, "metrics", metricsKeepDays)
	cleanTable(db, "deploy_logs", deployLogKeepDays(db))
	// M3：旧 deploy_versions 表已只读，不再 PruneAllVersions；
	// Release 保留 + Artifact 物理文件孤儿清理仍然生效。
	usecase.PruneAllReleases(db, releaseKeepPerService(db))
	usecase.PruneOrphanArtifactFiles(db)
	if shouldVacuum(db) {
		if err := db.Exec("VACUUM").Error; err != nil {
			log.Printf("[retention] VACUUM error: %v", err)
		} else {
			upsertSetting(db, "retention_last_vacuum", time.Now().Format(time.RFC3339))
			log.Printf("[retention] VACUUM done")
		}
	}
	upsertSetting(db, "retention_last_run", time.Now().Format(time.RFC3339))
}

// shouldVacuum returns true if it's the 1st of the month AND we haven't
// already VACUUMed this month — without persistence we'd VACUUM repeatedly
// when the process restarts during the day.
func shouldVacuum(db *gorm.DB) bool {
	if time.Now().Day() != 1 {
		return false
	}
	var s model.Setting
	if err := db.Where("key = ?", "retention_last_vacuum").First(&s).Error; err != nil {
		return true
	}
	last, err := time.Parse(time.RFC3339, s.Value)
	if err != nil {
		return true
	}
	now := time.Now()
	return last.Year() != now.Year() || last.Month() != now.Month()
}

func upsertSetting(db *gorm.DB, key, value string) {
	db.Save(&model.Setting{Key: key, Value: value})
}

func cleanTable(db *gorm.DB, table string, keepDays int) {
	if keepDays <= 0 {
		return
	}
	cutoff := time.Now().Add(-time.Duration(keepDays) * 24 * time.Hour)
	res := db.Exec(fmt.Sprintf("DELETE FROM %s WHERE created_at < ?", table), cutoff)
	if res.Error != nil {
		log.Printf("[retention] %s cleanup error: %v", table, res.Error)
		return
	}
	if res.RowsAffected > 0 {
		log.Printf("[retention] %s: deleted %d rows older than %d days", table, res.RowsAffected, keepDays)
	}
}

// releaseKeepPerService 读 settings.release_keep_per_service；缺省沿用常量。
func releaseKeepPerService(db *gorm.DB) int {
	var s model.Setting
	if err := db.Where("key = ?", "release_keep_per_service").First(&s).Error; err != nil {
		return usecase.MaxReleasesPerService
	}
	n, err := strconv.Atoi(s.Value)
	if err != nil || n <= 0 {
		return usecase.MaxReleasesPerService
	}
	return n
}

func deployLogKeepDays(db *gorm.DB) int {
	var s model.Setting
	if err := db.Where("key = ?", "deploy_log_keep_days").First(&s).Error; err != nil {
		return 30
	}
	n, err := strconv.Atoi(s.Value)
	if err != nil || n <= 0 {
		return 30
	}
	return n
}

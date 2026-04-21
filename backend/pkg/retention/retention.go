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
	"github.com/serverhub/serverhub/pkg/deployer"
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
	deployer.PruneAllVersions(db, deployer.MaxVersionsPerDeploy)
	if time.Now().Day() == 1 {
		if err := db.Exec("VACUUM").Error; err != nil {
			log.Printf("[retention] VACUUM error: %v", err)
		} else {
			log.Printf("[retention] VACUUM done")
		}
	}
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

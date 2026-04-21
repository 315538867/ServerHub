package database

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"time"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/crypto"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Init(cfg *config.Config) *gorm.DB {
	dbPath := filepath.Join(cfg.Server.DataDir, "serverhub.db")

	logLevel := logger.Silent
	if cfg.DevMode {
		logLevel = logger.Info
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		panic(fmt.Sprintf("failed to open database: %v", err))
	}

	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1) // SQLite: single writer

	// WAL mode for concurrent reads during writes
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA synchronous=NORMAL")
	db.Exec("PRAGMA cache_size=-32000")
	db.Exec("PRAGMA temp_store=MEMORY")
	db.Exec("PRAGMA foreign_keys=ON")

	if err := db.AutoMigrate(
		&model.User{},
		&model.AuditLog{},
		&model.Setting{},
		&model.Server{},
		&model.Metric{},
		&model.Deploy{},
		&model.DeployLog{},
		&model.SSLCert{},
		&model.DBConn{},
		&model.AlertRule{},
		&model.AlertEvent{},
		&model.NotifyChannel{},
		&model.Application{},
		&model.AppNginxRoute{},
	); err != nil {
		panic(fmt.Sprintf("migration failed: %v", err))
	}

	ensureIndexes(db)

	seedSettings(db)
	seedAdminUser(db, cfg)
	seedLocalServer(db)

	return db
}

func ensureIndexes(db *gorm.DB) {
	stmts := []string{
		"CREATE INDEX IF NOT EXISTS idx_audit_created  ON audit_logs(created_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_audit_username ON audit_logs(username)",
		"CREATE INDEX IF NOT EXISTS idx_audit_path     ON audit_logs(path)",
		"CREATE INDEX IF NOT EXISTS idx_metrics_server_created ON metrics(server_id, created_at DESC)",
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			fmt.Printf("ensureIndexes: %v (stmt=%q)\n", err, s)
		}
	}
}

func seedSettings(db *gorm.DB) {
	defaults := []model.Setting{
		{Key: "panel_name", Value: "ServerHub"},
		{Key: "allow_register", Value: "false"},
		{Key: "alert_cpu_threshold", Value: "90"},
		{Key: "alert_mem_threshold", Value: "85"},
		{Key: "alert_disk_threshold", Value: "80"},
		{Key: "alert_ssl_days", Value: "30"},
		{Key: "cert_renew_days", Value: "30"},
		{Key: "metrics_interval", Value: "5"},
		{Key: "alert_cooldown_min", Value: "30"},
		{Key: "deploy_log_keep_days", Value: "30"},
		{Key: "timezone", Value: "Asia/Shanghai"},
	}
	for _, s := range defaults {
		db.Where(model.Setting{Key: s.Key}).FirstOrCreate(&s)
	}
}

func seedAdminUser(db *gorm.DB, cfg *config.Config) {
	var count int64
	db.Model(&model.User{}).Count(&count)
	if count > 0 {
		return
	}

	password := "admin123"
	if !cfg.DevMode {
		// production: require serverhub init to set password
		password = generateRandomPassword()
		fmt.Printf("⚠️  Default admin password: %s  (change this immediately)\n", password)
	}

	hash, err := crypto.BcryptHash(password)
	if err != nil {
		panic(fmt.Sprintf("failed to hash password: %v", err))
	}

	now := time.Now()
	user := model.User{
		Username:  "admin",
		Password:  hash,
		Role:      "admin",
		LastLogin: &now,
	}
	if err := db.Create(&user).Error; err != nil {
		panic(fmt.Sprintf("failed to create admin user: %v", err))
	}

	if cfg.DevMode {
		fmt.Println("✓ Dev admin user created: admin / admin123")
	}
}

func generateRandomPassword() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "changeme123!"
	}
	return hex.EncodeToString(b)[:16]
}

// seedLocalServer inserts a single "local" server record representing the host
// machine ServerHub itself runs on. It is created only if no local-type server
// exists. The record lets handlers manage the local host without SSH.
func seedLocalServer(db *gorm.DB) {
	var count int64
	db.Model(&model.Server{}).Where("type = ?", "local").Count(&count)
	if count > 0 {
		return
	}
	now := time.Now()
	local := model.Server{
		Name:        "本机",
		Type:        "local",
		Host:        "127.0.0.1",
		Port:        0,
		Username:    "local",
		AuthType:    "local",
		Status:      "online",
		Remark:      "ServerHub 所在主机（本地执行，无需 SSH）",
		LastCheckAt: &now,
	}
	if err := db.Create(&local).Error; err != nil {
		fmt.Printf("seedLocalServer: %v\n", err)
	}
}

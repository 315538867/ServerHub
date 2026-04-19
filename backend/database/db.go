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
	); err != nil {
		panic(fmt.Sprintf("migration failed: %v", err))
	}

	seedSettings(db)
	seedAdminUser(db, cfg)

	return db
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

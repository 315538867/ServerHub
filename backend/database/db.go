package database

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/sysinfo"
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
		&model.DeployVersion{},
		&model.SSLCert{},
		&model.DBConn{},
		&model.AlertRule{},
		&model.AlertEvent{},
		&model.NotifyChannel{},
		&model.Application{},
		&model.AppNginxRoute{},
		&model.SetupState{},
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
		"CREATE INDEX IF NOT EXISTS idx_deploy_ver_deploy_created ON deploy_versions(deploy_id, created_at DESC)",
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

// seedAdminUser ensures dev mode always has a working admin/admin123 login.
// In production, the user table is left empty on first boot — the setup
// wizard (POST /panel/api/v1/setup/admin) creates the first admin from
// user-supplied credentials. The wizard's safety gate is "user count == 0",
// so this function must NOT seed anything in production.
func seedAdminUser(db *gorm.DB, cfg *config.Config) {
	if !cfg.DevMode {
		return
	}
	var count int64
	db.Model(&model.User{}).Count(&count)
	if count > 0 {
		return
	}

	hash, err := crypto.BcryptHash("admin123")
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
	fmt.Println("✓ Dev admin user created: admin / admin123")
}

// seedLocalServer inserts a single Type="local" server record representing
// the host the binary itself runs on. Skipped under containerized runtimes:
// the local runner (`bash -lc <cmd>`) only sees the container's namespace,
// not the host, so an in-container "local" entry can't manage host nginx /
// systemd / docker. Container deployments instead get a wizard-driven
// SSH-self-loopback record (Type="ssh", Host=<gateway>) created via the
// setup wizard.
func seedLocalServer(db *gorm.DB) {
	if sysinfo.IsContainerized() {
		return
	}
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

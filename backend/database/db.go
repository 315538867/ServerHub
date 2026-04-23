package database

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/discovery"
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

	// Pre-migration: rename legacy "deploys" table to "services" if a rename
	// hasn't happened yet. The Service model declares TableName()="services"
	// but an upgraded binary will find the old "deploys" table from a prior
	// release. AutoMigrate otherwise silently creates an empty "services"
	// table alongside the old one, stranding data.
	if db.Migrator().HasTable("deploys") && !db.Migrator().HasTable("services") {
		if err := db.Migrator().RenameTable("deploys", "services"); err != nil {
			panic(fmt.Sprintf("rename deploys→services failed: %v", err))
		}
	}

	if err := db.AutoMigrate(
		&model.User{},
		&model.AuditLog{},
		&model.Setting{},
		&model.Server{},
		&model.Metric{},
		&model.Service{},
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
	backfillFingerprints(db)

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

// seedLocalServer ensures exactly one Type="local" server record exists,
// representing the host the binary itself runs on. Skipped under containerized
// runtimes (see Type field for rationale).
//
// Migration safety: earlier versions allowed users to manually add servers
// pointing at 127.0.0.1/localhost as Type="ssh". On upgrade those would
// coexist with a freshly seeded Type="local" row, producing two "本机"
// entries. This function now (1) promotes the oldest localhost-like Type="ssh"
// row to Type="local" if no local row exists yet, (2) demotes the rest into
// inactive remarks rather than deleting (user data preservation), (3) only
// creates a new row when no candidate exists.
func seedLocalServer(db *gorm.DB) {
	if sysinfo.IsContainerized() {
		return
	}
	var locals []model.Server
	db.Where("type = ?", "local").Order("id asc").Find(&locals)
	if len(locals) > 1 {
		// Multiple local rows existed (data corruption / older bug). Keep the
		// oldest, mark the rest as ssh + flag in remark for manual review.
		for _, s := range locals[1:] {
			db.Model(&s).Updates(map[string]any{
				"type":   "ssh",
				"remark": s.Remark + " [auto-demoted: duplicate local row]",
			})
		}
	}
	if len(locals) >= 1 {
		return
	}
	// No local row yet. Try to promote an existing localhost-like ssh row.
	var existing model.Server
	err := db.Where("type = ? AND host IN ?", "ssh",
		[]string{"127.0.0.1", "localhost", "::1", "0.0.0.0"}).
		Order("id asc").First(&existing).Error
	now := time.Now()
	if err == nil {
		db.Model(&existing).Updates(map[string]any{
			"type":          "local",
			"name":          "本机",
			"host":          "127.0.0.1",
			"port":          0,
			"username":      "local",
			"auth_type":     "local",
			"password":      "",
			"private_key":   "",
			"status":        "online",
			"remark":        "ServerHub 所在主机（本地执行，无需 SSH）",
			"last_check_at": &now,
		})
		return
	}
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

// backfillFingerprints fills SourceFingerprint for legacy Service rows that
// were imported before discovery.Fingerprint existed. Without this, the
// "已接管" 标记在升级后第一次扫描会全部丢失。
//
// 我们重建一个最小化的 discovery.Candidate（只填 Fingerprint 用得到的字段），
// 仅当 SourceKind/SourceID 非空且 Fingerprint 为空时才回填。
func backfillFingerprints(db *gorm.DB) {
	var rows []model.Service
	if err := db.Where("source_fingerprint = '' AND source_kind != ''").Find(&rows).Error; err != nil {
		return
	}
	for _, s := range rows {
		cand := discovery.Candidate{
			Kind:     s.SourceKind,
			SourceID: s.SourceID,
			Suggested: discovery.SuggestedDeploy{
				ImageName:   s.ImageName,
				WorkDir:     s.WorkDir,
				ComposeFile: s.ComposeFile,
			},
		}
		fp := discovery.Fingerprint(cand)
		db.Model(&model.Service{}).Where("id = ?", s.ID).Update("source_fingerprint", fp)
	}
	if len(rows) > 0 {
		fmt.Printf("backfillFingerprints: %d 条已回填\n", len(rows))
	}
}

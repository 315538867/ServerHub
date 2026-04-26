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

	// Pre-migration: rename legacy "deploys" table to "services" if a rename
	// hasn't happened yet. The Service model declares TableName()="services"
	// but an upgraded binary will find the old "deploys" table from a prior
	// release. AutoMigrate otherwise silently creates an empty "services"
	// table alongside the old one, stranding data.
	//
	// 这条必须发生在 AutoMigrate 之前——否则 GORM 会先建空 services 再让我们
	// 改名,旧数据被孤立。所以这条 schema 操作物理上留在这里,但
	// migration/migrations.go 的 001 版本号也注册了同样的 Fn(幂等),走 runner
	// 仅做"已 applied"审计,真正起效的是这次 RenameTable。
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
		&model.ServerProbe{},
		&model.Metric{},
		&model.Service{},
		&model.DeployLog{},
		&model.SSLCert{},
		&model.DBConn{},
		&model.AlertRule{},
		&model.AlertEvent{},
		&model.NotifyChannel{},
		&model.Application{},
		// Phase M1: Release 三维正交模型（与旧 DeployVersion/DeployLog 并存）
		&model.Artifact{},
		&model.EnvVarSet{},
		&model.ConfigFileSet{},
		&model.Release{},
		&model.DeployRun{},
		// Phase M3: App 级发布集（组合多个 Service 的 Release 做原子应用/回滚）
		&model.AppReleaseSet{},
		// Phase Nginx-P0: Ingress 模型（跨机入口编排），P0 建表 + 数据迁移，
		// Renderer/Reconciler 在 P1 接入；NginxCert/AuditApply 表先占位，
		// P2 (HTTPS) / P1 (审计) 起开始写入。
		&model.Ingress{},
		&model.IngressRoute{},
		&model.NginxCert{},
		&model.AuditApply{},
		// Phase Nginx-P3: 多实例 nginx 部署形态（路径/命令/probe 结果）。1:1 with edge.
		&model.NginxProfile{},
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
		{Key: "server_probe_keep_days", Value: "7"},
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

// seedLocalServer ensures a Type="local" Server row exists when the runtime
// has capability to manage the host, otherwise leaves the row absent so the
// UI doesn't surface a "本机" card we can't act on.
//
// Capability is decided by sysinfo.LocalCapability():
//   - "full":   bare metal, OR container with --pid=host + -v /:/host + sock
//   - "docker": container with only docker.sock mounted
//   - "none":   container without any host bridge → no row created
//
// Migration safety: earlier versions allowed users to manually add servers
// pointing at 127.0.0.1/localhost (or a docker bridge gateway) as Type="ssh",
// often named "本机"/"本机 (SSH)". On upgrade those would coexist with a
// freshly seeded Type="local" row, producing two "本机" entries and splitting
// services across server_ids. mergeLocalAliases collapses such legacy rows
// into the canonical local row regardless of capability — so even in
// docker-only or none mode we still run the merge step.
func seedLocalServer(db *gorm.DB) {
	localHosts := []string{"127.0.0.1", "localhost", "::1", "0.0.0.0"}
	localNames := []string{"本机", "本机 (SSH)"}
	lc := sysinfo.LocalCapability()

	var locals []model.Server
	db.Where("type = ?", "local").Order("id asc").Find(&locals)
	if len(locals) > 1 {
		for _, s := range locals[1:] {
			db.Model(&s).Updates(map[string]any{
				"type":   "ssh",
				"remark": s.Remark + " [auto-demoted: duplicate local row]",
			})
		}
	}
	if len(locals) >= 1 {
		kept := locals[0]
		if kept.Capability != lc && lc != sysinfo.CapNone {
			db.Model(&kept).Update("capability", lc)
		}
		mergeLocalAliases(db, kept.ID, localHosts, localNames)
		return
	}
	if lc == sysinfo.CapNone {
		// No row to host the merge target either, but legacy aliases (if any)
		// will simply remain as ssh records — user can clean up manually.
		return
	}
	var existing model.Server
	err := db.Where("type = ? AND (host IN ? OR name IN ?)", "ssh",
		localHosts, localNames).
		Order("id asc").First(&existing).Error
	now := time.Now()
	remark := localRemarkFor(lc)
	// R3 起 server.status / server.last_check_at 已下线,在线状态由 server_probes
	// 派生。本地服务器 seed 完毕后写一条 online probe,避免首次 scheduler tick 前
	// UI 显示 "unknown"。
	if err == nil {
		db.Model(&existing).Updates(map[string]any{
			"type":        "local",
			"name":        "本机",
			"host":        "127.0.0.1",
			"port":        0,
			"username":    "local",
			"auth_type":   "local",
			"password":    "",
			"private_key": "",
			"capability":  lc,
			"remark":      remark,
		})
		db.Create(&model.ServerProbe{ServerID: existing.ID, Result: "online", CreatedAt: now})
		mergeLocalAliases(db, existing.ID, localHosts, localNames)
		return
	}
	local := model.Server{
		Name:       "本机",
		Type:       "local",
		Host:       "127.0.0.1",
		Port:       0,
		Username:   "local",
		AuthType:   "local",
		Capability: lc,
		Remark:     remark,
	}
	if err := db.Create(&local).Error; err != nil {
		fmt.Printf("seedLocalServer: %v\n", err)
		return
	}
	db.Create(&model.ServerProbe{ServerID: local.ID, Result: "online", CreatedAt: now})
}

func localRemarkFor(cap string) string {
	switch cap {
	case sysinfo.CapFull:
		return "ServerHub 所在主机（本地执行，无需 SSH）"
	case sysinfo.CapDocker:
		return "ServerHub 容器仅挂载 docker.sock，本机仅支持 Docker 操作；如需 systemd/文件管理，请加 --pid=host 与 -v /:/host"
	default:
		return ""
	}
}

// mergeLocalAliases collapses legacy "本机"-shaped ssh Server rows into the
// canonical Type="local" row (keptID), re-pointing any child records so the
// Discover flow can correctly detect already-imported services. Aliases are
// renamed + remarked (not deleted) to preserve user traceability; their host
// is also neutralized so they cannot re-match this function on next boot.
func mergeLocalAliases(db *gorm.DB, keptID uint, hosts, names []string) {
	var aliases []model.Server
	db.Where("id != ? AND type = ? AND (host IN ? OR name IN ?)",
		keptID, "ssh", hosts, names).Find(&aliases)
	if len(aliases) == 0 {
		return
	}
	for _, a := range aliases {
		db.Model(&model.Service{}).Where("server_id = ?", a.ID).Update("server_id", keptID)
		db.Model(&model.Application{}).Where("server_id = ?", a.ID).Update("server_id", keptID)
		db.Model(&model.DBConn{}).Where("server_id = ?", a.ID).Update("server_id", keptID)
		db.Model(&model.SSLCert{}).Where("server_id = ?", a.ID).Update("server_id", keptID)
		db.Model(&model.Metric{}).Where("server_id = ?", a.ID).Update("server_id", keptID)
		db.Model(&a).Updates(map[string]any{
			"name":   a.Name + " [已合并到本机]",
			"host":   "",
			"remark": a.Remark + " [auto-merged into local server]",
		})
	}
	fmt.Printf("mergeLocalAliases: %d 条 ssh 本机别名已合并到 server_id=%d\n", len(aliases), keptID)
}

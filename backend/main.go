package main

// === v2 重构 — adapters blank import 占位块(R1 落地,R2-R5 逐个填实) ===
//
// 触发各 adapter 包的 init() 自注册到 core/<port>.Default。
// 注册时机参见 docs/architecture/v2/05-extension-points.md §2。
//
//   _ "github.com/serverhub/serverhub/adapters/runtime/docker"   // R2
//   _ "github.com/serverhub/serverhub/adapters/runtime/compose"  // R2
//   _ "github.com/serverhub/serverhub/adapters/runtime/native"   // R2
//   _ "github.com/serverhub/serverhub/adapters/runtime/static"   // R2
//   _ "github.com/serverhub/serverhub/adapters/source/docker"    // R4
//   _ "github.com/serverhub/serverhub/adapters/source/compose"   // R4
//   _ "github.com/serverhub/serverhub/adapters/source/nginx"     // R4
//   _ "github.com/serverhub/serverhub/adapters/source/systemd"   // R4
//   _ "github.com/serverhub/serverhub/adapters/ingress/nginx"    // R5
//   _ "github.com/serverhub/serverhub/adapters/notify/webhook"   // R6
//   _ "github.com/serverhub/serverhub/adapters/notify/email"     // R6
//
// === v2 重构占位结束 ===

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	apialerts "github.com/serverhub/serverhub/api/alerts"
	apiapplication "github.com/serverhub/serverhub/api/application"
	apiapprelease "github.com/serverhub/serverhub/api/apprelease"
	apiaudit "github.com/serverhub/serverhub/api/audit"
	apiauth "github.com/serverhub/serverhub/api/auth"
	apidatabase "github.com/serverhub/serverhub/api/database"
	apidiscovery "github.com/serverhub/serverhub/api/discovery"
	apidocker "github.com/serverhub/serverhub/api/docker"
	apifiles "github.com/serverhub/serverhub/api/files"
	"github.com/serverhub/serverhub/api/health"
	apiingresses "github.com/serverhub/serverhub/api/ingresses"
	apimetrics "github.com/serverhub/serverhub/api/metrics"
	apilogsearch "github.com/serverhub/serverhub/api/logsearch"
	apinginx "github.com/serverhub/serverhub/api/nginx"
	apirelease "github.com/serverhub/serverhub/api/release"
	apiservers "github.com/serverhub/serverhub/api/servers"
	apisetup "github.com/serverhub/serverhub/api/setup"
	apissl "github.com/serverhub/serverhub/api/ssl"
	apisystem "github.com/serverhub/serverhub/api/system"
	apiterminal "github.com/serverhub/serverhub/api/terminal"
	apisettings "github.com/serverhub/serverhub/api/settings"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/database"
	"github.com/serverhub/serverhub/middleware"
	"github.com/serverhub/serverhub/migration"
	"github.com/serverhub/serverhub/pkg/scheduler"
	"github.com/serverhub/serverhub/pkg/sshpool"
	"github.com/serverhub/serverhub/pkg/retention"
	"github.com/serverhub/serverhub/pkg/auditq"
	"github.com/serverhub/serverhub/usecase"

	_ "github.com/serverhub/serverhub/adapters/runtime/compose"
	_ "github.com/serverhub/serverhub/adapters/runtime/docker"
	_ "github.com/serverhub/serverhub/adapters/runtime/native"
	_ "github.com/serverhub/serverhub/adapters/runtime/static"
	_ "github.com/serverhub/serverhub/adapters/source/compose"
	_ "github.com/serverhub/serverhub/adapters/source/docker"
	_ "github.com/serverhub/serverhub/adapters/source/nginx"
	_ "github.com/serverhub/serverhub/adapters/source/systemd"
	"gorm.io/gorm"
)

// Version is injected at build time via ldflags.
var Version = "dev"

func main() {
	var (
		configPath  = flag.String("config", envOr("SERVERHUB_CONFIG", "/opt/serverhub/config.yaml"), "config file path (env: SERVERHUB_CONFIG)")
		dataDir     = flag.String("data", os.Getenv("SERVERHUB_DATA_DIR"), "data directory override (env: SERVERHUB_DATA_DIR)")
		port        = flag.Int("port", envInt("SERVERHUB_PORT", 0), "port override (env: SERVERHUB_PORT)")
		devMode     = flag.Bool("dev", false, "development mode (CORS, debug logging)")
		healthcheck = flag.Bool("healthcheck", false, "probe local /healthz and exit 0/1 (for container HEALTHCHECK)")
		showVersion = flag.Bool("version", false, "print version and exit")
		migrateCmd  = flag.String("migrate", "", "run a data migration and exit: m2|m2-dryrun")
	)
	flag.Parse()

	if *showVersion {
		fmt.Println(Version)
		os.Exit(0)
	}
	if *healthcheck {
		os.Exit(runHealthcheck(*configPath, *port))
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}
	if *dataDir != "" {
		cfg.Server.DataDir = *dataDir
	}
	if *port != 0 {
		cfg.Server.Port = *port
	}
	cfg.DevMode = *devMode

	health.Version = Version

	// ensure required directories exist
	for _, d := range []string{
		cfg.Server.DataDir,
		filepath.Join(cfg.Server.DataDir, "ssh_keys"),
		filepath.Join(cfg.Server.DataDir, "logs"),
		filepath.Join(cfg.Server.DataDir, "deploy-logs"),
	} {
		if err := os.MkdirAll(d, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "mkdir %s: %v\n", d, err)
			os.Exit(1)
		}
	}

	db := database.Init(cfg)

	// 版本化数据迁移收口:把 v0.3.7 之前散落在 db.Init 的"启动期一次性补丁"
	// (drop setup_states / backfill fingerprints / backfill run_server_id /
	// M2 deploy_versions→release) 全部交给 migration runner,以
	// schema_migrations 表为单一事实源。每条 migration 在事务里跑且仅跑一次。
	migration.RegisterM2(cfg.Security.AESKey)

	// 一次性迁移命令:跑完立即退出,不起 HTTP 服务。
	// m2-dryrun 走独立 core,不写库不进 schema_migrations;m2 等价于普通启动
	// 的 runner 阶段(会一并跑 001-004 schema 补丁),跑完打印 m2 报告退出。
	if *migrateCmd != "" {
		if err := runMigration(*migrateCmd, db, cfg.Security.AESKey); err != nil {
			fmt.Fprintf(os.Stderr, "migration failed: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if err := migration.Run(db); err != nil {
		fmt.Fprintf(os.Stderr, "schema migration failed: %v\n", err)
		os.Exit(1)
	}

	sshpool.SetHostKeyStore(sshpool.NewGormHostKeyStore(db))

	auditq.Default = auditq.New(db)
	defer auditq.Default.Close()

	// Surface host-key mismatches as security audit events. Wired here so
	// the sshpool package stays free of the auditq dependency.
	sshpool.OnHostKeyMismatch = func(serverID uint, hostname, pinned, got string) {
		auditq.Security("system", hostname, "security:host_key_mismatch", 0, map[string]any{
			"server_id": serverID, "pinned": pinned, "got": got,
		})
	}

	if !cfg.DevMode {
		const devJWT = "serverhub-dev-jwt-secret-change-in-production!!"
		const devAES = "6465766b6579363436343634363436346465766b657936343634363436343634"
		if cfg.Security.JWTSecret == "" || cfg.Security.AESKey == "" ||
			cfg.Security.JWTSecret == devJWT || cfg.Security.AESKey == devAES {
			fmt.Fprintln(os.Stderr, "╔══════════════════════════════════════════════════════════════╗")
			fmt.Fprintln(os.Stderr, "║  FATAL: default or empty JWT/AES secrets in production mode. ║")
			fmt.Fprintln(os.Stderr, "║  Set security.jwt_secret and security.aes_key in config.yaml ║")
			fmt.Fprintln(os.Stderr, "║  (aes_key = 64 hex chars / 32 bytes). Refusing to start.     ║")
			fmt.Fprintln(os.Stderr, "╚══════════════════════════════════════════════════════════════╝")
			os.Exit(1)
		}
		if len(cfg.Security.AESKey) != 64 {
			fmt.Fprintln(os.Stderr, "FATAL: security.aes_key must be 64 hex chars (32 bytes) for AES-256")
			os.Exit(1)
		}
	}

	// Release / Artifact 保留策略需要 data_dir 来删除 upload 物理文件
	usecase.ArtifactsDataDir = cfg.Server.DataDir

	scheduler.Start(db, cfg)
	scheduler.StartReconciler(db, cfg)
	scheduler.StartCertRenewer(db, cfg)
	retention.Start(db)

	if cfg.DevMode {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(middleware.Recover())

	// Liveness/readiness probe for container orchestrators and the --healthcheck
	// subcommand. Kept outside /panel/api so probers don't need to know the prefix.
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "version": Version})
	})

	if cfg.DevMode {
		r.Use(corsMiddleware())
	}

	// ── public routes ──────────────────────────────────────────
	base := r.Group("/panel/api/v1")
	base.GET("/health", health.Handler(cfg, db))

	authGroup := base.Group("/auth")
	authGroup.Use(middleware.RateLimit(cfg))
	apiauth.RegisterRoutes(authGroup, db, cfg)

	// First-run wizard (intentionally public — user table is empty on first
	// boot, so JWT auth isn't available yet). Only /admin and /status remain;
	// /admin enforces user_count==0 server-side. Local-host SSH bootstrap was
	// removed in v0.3.7-beta.16 in favor of capability-probed local server.
	apisetup.RegisterRoutes(base.Group("/setup"), db)

	// ── protected routes ───────────────────────────────────────────
	protected := base.Group("/")
	protected.Use(middleware.Auth(cfg))
	protected.Use(middleware.Audit(db))

	serversGroup := protected.Group("/servers")
	apiservers.RegisterRoutes(serversGroup, db, cfg)
	apidiscovery.RegisterRoutes(serversGroup, db, cfg)
	apidocker.RegisterRoutes(serversGroup, db, cfg)
	apifiles.RegisterRoutes(serversGroup, db, cfg)
	apisystem.RegisterRoutes(serversGroup, db, cfg)
	apisystem.RegisterSelfRoutes(protected.Group("/system/self"))
	apinginx.RegisterRoutes(serversGroup, db, cfg)
	apissl.RegisterRoutes(serversGroup, db, cfg)
	apilogsearch.RegisterRoutes(serversGroup, db, cfg)
	apidatabase.RegisterRoutes(protected, db, cfg)
	apialerts.RegisterRoutes(protected.Group("/alerts"), db, cfg)
	// Phase M3: Release 三维模型是 service 写路径的唯一来源,旧 apideploy 包已退役。
	apirelease.RegisterRoutes(protected.Group("/services"), db, cfg)
	apirelease.RegisterWebhookRoutes(base.Group("/webhooks"), db, cfg)
	apimetrics.RegisterRoutes(protected.Group("/metrics"), db)
	apisettings.RegisterRoutes(protected.Group("/settings"), db, cfg)
	appsGroup := protected.Group("/apps")
	apiapplication.RegisterRoutes(appsGroup, db, cfg)
	// Phase M3: AppReleaseSet（App 级 Release 组合 + SSE Apply/Rollback）
	apiapprelease.RegisterRoutes(appsGroup, db, cfg)
	apiingresses.RegisterRoutes(protected.Group("/ingresses"), db, cfg)
	apiaudit.RegisterRoutes(protected.Group("/audit"), db)

	// Terminal: auth via ?token= query param (WS upgrade), no Audit middleware
	apiterminal.RegisterRoutes(base.Group("/servers"), db, cfg)

	// ── static files (production only) ─────────────────────────
	if !cfg.DevMode {
		dist, _ := fs.Sub(staticFiles, "web/dist")
		// Pre-read index.html so we can serve it via c.Data and avoid
		// http.FileServer's canonical-path redirect on "/index.html".
		indexData, err := fs.ReadFile(dist, "index.html")
		if err != nil {
			indexData = []byte("<!doctype html><title>ServerHub</title><body>missing embedded frontend</body>")
		}
		r.NoRoute(func(c *gin.Context) {
			p := c.Request.URL.Path
			if strings.HasPrefix(p, "/panel/api") || strings.HasPrefix(p, "/panel/webhooks") {
				c.JSON(404, gin.H{"code": 404, "msg": "not found"})
				return
			}
			// Serve real assets from the embedded dist (Vite base = /panel/).
			rel := strings.TrimPrefix(p, "/panel/")
			rel = strings.TrimPrefix(rel, "/")
			if rel != "" && rel != "index.html" {
				if f, err := dist.Open(rel); err == nil {
					defer f.Close()
					if stat, _ := f.Stat(); stat != nil && !stat.IsDir() {
						if ctype := mime.TypeByExtension(filepath.Ext(rel)); ctype != "" {
							c.Writer.Header().Set("Content-Type", ctype)
						}
						// NoRoute presets status=404; set 200 explicitly.
						c.Writer.WriteHeader(http.StatusOK)
						_, _ = io.Copy(c.Writer, f)
						return
					}
				}
			}
			c.Data(http.StatusOK, "text/html; charset=utf-8", indexData)
		})
	}

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	fmt.Printf("ServerHub %s  →  http://localhost%s/panel/\n", Version, addr)
	if err := r.Run(addr); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

// runHealthcheck probes the local /healthz endpoint so distroless containers
// (which have no shell or curl) can declare a Docker HEALTHCHECK by invoking
// the binary itself. Port resolution order: --port flag > SERVERHUB_PORT env >
// config file > 9999.
func runHealthcheck(configPath string, portFlag int) int {
	p := portFlag
	if p == 0 {
		if cfg, err := config.Load(configPath); err == nil && cfg.Server.Port != 0 {
			p = cfg.Server.Port
		} else {
			p = 9999
		}
	}
	client := &http.Client{Timeout: 3 * time.Second}
	r, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/healthz", p))
	if err != nil {
		return 1
	}
	defer r.Body.Close()
	_, _ = io.Copy(io.Discard, r.Body)
	if r.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

// runMigration 分发 -migrate=xxx 命令。成功后打印 JSON 报告。
func runMigration(name string, db interface{}, aesKey string) error {
	gdb, ok := db.(*gorm.DB)
	if !ok {
		return fmt.Errorf("migration: unexpected db type")
	}
	switch name {
	case "m2":
		// 走 runner 主通道 —— 跟普通启动同一条路径,schema_migrations 唯一事实源。
		// 已 applied 时 Fn 不会再跑,lastM2Report 为 nil,提示已完成即可。
		if err := migration.Run(gdb); err != nil {
			return err
		}
		rep := migration.LastM2Report()
		if rep == nil {
			fmt.Println(`{"already_done": true}`)
			return nil
		}
		b, _ := json.MarshalIndent(rep, "", "  ")
		fmt.Println(string(b))
		return nil
	case "m2-dryrun":
		rep, err := migration.RunM2Dryrun(gdb, aesKey)
		if err != nil {
			return err
		}
		b, _ := json.MarshalIndent(rep, "", "  ")
		fmt.Println(string(b))
		return nil
	}
	return fmt.Errorf("unknown migration: %s (want m2|m2-dryrun)", name)
}

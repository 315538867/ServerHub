package main

import (
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
	apiaudit "github.com/serverhub/serverhub/api/audit"
	apiapproutes "github.com/serverhub/serverhub/api/approutes"
	apiauth "github.com/serverhub/serverhub/api/auth"
	apidatabase "github.com/serverhub/serverhub/api/database"
	apideploy "github.com/serverhub/serverhub/api/deploy"
	apidiscovery "github.com/serverhub/serverhub/api/discovery"
	apidocker "github.com/serverhub/serverhub/api/docker"
	apifiles "github.com/serverhub/serverhub/api/files"
	"github.com/serverhub/serverhub/api/health"
	apimetrics "github.com/serverhub/serverhub/api/metrics"
	apilogsearch "github.com/serverhub/serverhub/api/logsearch"
	apinginx "github.com/serverhub/serverhub/api/nginx"
	apiservers "github.com/serverhub/serverhub/api/servers"
	apisetup "github.com/serverhub/serverhub/api/setup"
	apissl "github.com/serverhub/serverhub/api/ssl"
	apisystem "github.com/serverhub/serverhub/api/system"
	apiterminal "github.com/serverhub/serverhub/api/terminal"
	apisettings "github.com/serverhub/serverhub/api/settings"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/database"
	"github.com/serverhub/serverhub/middleware"
	"github.com/serverhub/serverhub/pkg/scheduler"
	"github.com/serverhub/serverhub/pkg/sshpool"
	"github.com/serverhub/serverhub/pkg/retention"
	"github.com/serverhub/serverhub/pkg/auditq"
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

	scheduler.Start(db, cfg)
	scheduler.StartReconciler(db, cfg)
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
	// boot, so JWT auth isn't available yet. Each endpoint enforces its own
	// safety gate server-side: /admin requires user_count==0, /local/* requires
	// containerized runtime + admin already created.)
	apisetup.RegisterRoutes(base.Group("/setup"), db, cfg)

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
	apideploy.RegisterRoutes(protected.Group("/services"), db, cfg)
	apideploy.RegisterWebhookRoutes(base.Group("/webhooks"), db, cfg)
	apimetrics.RegisterRoutes(protected.Group("/metrics"), db)
	apisettings.RegisterRoutes(protected.Group("/settings"), db, cfg)
	appsGroup := protected.Group("/apps")
	apiapplication.RegisterRoutes(appsGroup, db, cfg)
	apiapproutes.RegisterRoutes(appsGroup, db, cfg)
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

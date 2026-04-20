package main

import (
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	apialerts "github.com/serverhub/serverhub/api/alerts"
	apiapplication "github.com/serverhub/serverhub/api/application"
	apiaudit "github.com/serverhub/serverhub/api/audit"
	apiapproutes "github.com/serverhub/serverhub/api/approutes"
	apiauth "github.com/serverhub/serverhub/api/auth"
	apidatabase "github.com/serverhub/serverhub/api/database"
	apideploy "github.com/serverhub/serverhub/api/deploy"
	apidocker "github.com/serverhub/serverhub/api/docker"
	apifiles "github.com/serverhub/serverhub/api/files"
	"github.com/serverhub/serverhub/api/health"
	apimetrics "github.com/serverhub/serverhub/api/metrics"
	apinginx "github.com/serverhub/serverhub/api/nginx"
	apiservers "github.com/serverhub/serverhub/api/servers"
	apissl "github.com/serverhub/serverhub/api/ssl"
	apisystem "github.com/serverhub/serverhub/api/system"
	apiterminal "github.com/serverhub/serverhub/api/terminal"
	apisettings "github.com/serverhub/serverhub/api/settings"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/database"
	"github.com/serverhub/serverhub/middleware"
	"github.com/serverhub/serverhub/pkg/scheduler"
	"github.com/serverhub/serverhub/tray"
)

// Version is injected at build time via ldflags.
var Version = "dev"

func main() {
	var (
		configPath = flag.String("config", "/opt/serverhub/config.yaml", "config file path")
		dataDir    = flag.String("data", "", "data directory override")
		port       = flag.Int("port", 0, "port override")
		devMode    = flag.Bool("dev", false, "development mode (CORS, debug logging)")
	)
	flag.Parse()

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

	if !cfg.DevMode {
		const devJWT = "serverhub-dev-jwt-secret-change-in-production!!"
		const devAES = "6465766b6579363436343634363436346465766b657936343634363436343634"
		if cfg.Security.JWTSecret == devJWT || cfg.Security.AESKey == devAES {
			fmt.Fprintln(os.Stderr, "╔══════════════════════════════════════════════════════════════╗")
			fmt.Fprintln(os.Stderr, "║  CRITICAL: default JWT/AES secrets detected in production!  ║")
			fmt.Fprintln(os.Stderr, "║  Set security.jwt_secret and security.aes_key in config.yaml ║")
			fmt.Fprintln(os.Stderr, "╚══════════════════════════════════════════════════════════════╝")
		}
	}

	scheduler.Start(db, cfg)
	scheduler.StartReconciler(db, cfg)

	if cfg.DevMode {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(middleware.Recover())

	if cfg.DevMode {
		r.Use(corsMiddleware())
	}

	// ── public routes ──────────────────────────────────────────
	base := r.Group("/panel/api/v1")
	base.GET("/health", health.Handler(cfg, db))

	authGroup := base.Group("/auth")
	authGroup.Use(middleware.RateLimit(cfg))
	apiauth.RegisterRoutes(authGroup, db, cfg)

	// ── protected routes ───────────────────────────────────────────
	protected := base.Group("/")
	protected.Use(middleware.Auth(cfg))
	protected.Use(middleware.Audit(db))

	serversGroup := protected.Group("/servers")
	apiservers.RegisterRoutes(serversGroup, db, cfg)
	apidocker.RegisterRoutes(serversGroup, db, cfg)
	apifiles.RegisterRoutes(serversGroup, db, cfg)
	apisystem.RegisterRoutes(serversGroup, db, cfg)
	apinginx.RegisterRoutes(serversGroup, db, cfg)
	apissl.RegisterRoutes(serversGroup, db, cfg)
	apidatabase.RegisterRoutes(protected, db, cfg)
	apialerts.RegisterRoutes(protected.Group("/alerts"), db, cfg)
	apideploy.RegisterRoutes(protected.Group("/deploys"), db, cfg)
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
		r.NoRoute(func(c *gin.Context) {
			p := c.Request.URL.Path
			if strings.HasPrefix(p, "/panel/api") || strings.HasPrefix(p, "/panel/webhooks") {
				c.JSON(404, gin.H{"code": 404, "msg": "not found"})
				return
			}
			c.FileFromFS("index.html", http.FS(dist))
		})
	}

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	fmt.Printf("ServerHub %s  →  http://localhost%s/panel/\n", Version, addr)
	tray.Run(func() {
		if err := r.Run(addr); err != nil {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
			os.Exit(1)
		}
	}, cfg.Server.Port)
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

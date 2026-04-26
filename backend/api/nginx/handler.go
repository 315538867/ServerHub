package nginx

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/middleware"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/pkg/runner"
	"github.com/serverhub/serverhub/pkg/safeshell"
	"github.com/serverhub/serverhub/pkg/sshpool"
	"github.com/serverhub/serverhub/pkg/wsstream"
	"gorm.io/gorm"
)

var upgrader = websocket.Upgrader{ReadBufferSize: 4096, WriteBufferSize: 4096}

func RegisterRoutes(r *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	upgrader.CheckOrigin = middleware.WSCheckOrigin(cfg)
	r.GET("/:id/nginx/sites", listSitesHandler(db, cfg))
	r.POST("/:id/nginx/sites", createSiteHandler(db, cfg))
	r.GET("/:id/nginx/sites/:name/config", getSiteConfigHandler(db, cfg))
	r.PUT("/:id/nginx/sites/:name/config", putSiteConfigHandler(db, cfg))
	r.DELETE("/:id/nginx/sites/:name", deleteSiteHandler(db, cfg))
	r.POST("/:id/nginx/sites/:name/enable", enableSiteHandler(db, cfg))
	r.POST("/:id/nginx/sites/:name/disable", disableSiteHandler(db, cfg))
	r.POST("/:id/nginx/reload", reloadHandler(db, cfg))
	r.POST("/:id/nginx/restart", restartHandler(db, cfg))
	r.GET("/:id/nginx/logs/access", accessLogsHandler(db, cfg))
	r.GET("/:id/nginx/logs/error", errorLogsHandler(db, cfg))
	// Phase Nginx-P3: 多实例 profile 与 nginx -V probe
	RegisterProfileRoutes(r, db, cfg)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func getRunner(c *gin.Context, db *gorm.DB, cfg *config.Config) (runner.Runner, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.BadRequest(c, "服务器 ID 无效")
		return nil, false
	}
	var s model.Server
	if err := db.First(&s, id).Error; err != nil {
		resp.NotFound(c, "服务器不存在")
		return nil, false
	}
	rn, err := runner.For(&s, cfg)
	if err != nil {
		resp.Fail(c, http.StatusServiceUnavailable, 5003, "执行器获取失败: "+err.Error())
		return nil, false
	}
	return rn, true
}

func getDedicatedRunner(c *gin.Context, db *gorm.DB, cfg *config.Config) (runner.Runner, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.BadRequest(c, "服务器 ID 无效")
		return nil, false
	}
	var s model.Server
	if err := db.First(&s, id).Error; err != nil {
		resp.NotFound(c, "服务器不存在")
		return nil, false
	}
	rn, err := runner.ForDedicated(&s, cfg)
	if err != nil {
		resp.Fail(c, http.StatusServiceUnavailable, 5003, "执行器获取失败: "+err.Error())
		return nil, false
	}
	return rn, true
}

func sq(s string) string { return safeshell.Quote(s) }

// siteName returns the :name URL param if it passes whitelist validation,
// otherwise it writes a 400 and returns ("", false). This stops path traversal
// into /etc/nginx/sites-{available,enabled} via values like "../etc/passwd".
func siteName(c *gin.Context) (string, bool) {
	name := c.Param("name")
	if err := safeshell.ValidName(name, 64); err != nil {
		resp.BadRequest(c, "站点名无效："+err.Error())
		return "", false
	}
	return name, true
}

// ── site list ─────────────────────────────────────────────────────────────────

type SiteItem struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Path    string `json:"path"`
}

func listSitesHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getRunner(c, db, cfg)
		if !ok {
			return
		}
		available, _ := client.Run("ls /etc/nginx/sites-available/ 2>/dev/null")
		enabled, _ := client.Run("ls /etc/nginx/sites-enabled/ 2>/dev/null")

		enabledSet := make(map[string]bool)
		for _, name := range strings.Fields(enabled) {
			enabledSet[strings.TrimSpace(name)] = true
		}

		var sites []SiteItem
		for _, name := range strings.Fields(available) {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			sites = append(sites, SiteItem{
				Name:    name,
				Enabled: enabledSet[name],
				Path:    "/etc/nginx/sites-available/" + name,
			})
		}
		if sites == nil {
			sites = []SiteItem{}
		}
		resp.OK(c, sites)
	}
}

// ── create site ───────────────────────────────────────────────────────────────

func createSiteHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getRunner(c, db, cfg)
		if !ok {
			return
		}
		var body struct {
			Name   string `json:"name" binding:"required"`
			Type   string `json:"type" binding:"required"` // static | proxy | php
			Domain string `json:"domain" binding:"required"`
			Port   int    `json:"port"`
			Root   string `json:"root"`
			Proxy  string `json:"proxy"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, "请求体格式错误")
			return
		}
		if err := safeshell.ValidName(body.Name, 64); err != nil {
			resp.BadRequest(c, "站点名无效："+err.Error())
			return
		}
		if err := safeshell.NginxValue(body.Domain); err != nil {
			resp.BadRequest(c, "domain 非法："+err.Error())
			return
		}
		if body.Root != "" {
			if err := safeshell.NginxValue(body.Root); err != nil {
				resp.BadRequest(c, "root 非法："+err.Error())
				return
			}
		}
		if body.Proxy != "" {
			if err := safeshell.NginxValue(body.Proxy); err != nil {
				resp.BadRequest(c, "proxy 非法："+err.Error())
				return
			}
		}
		if body.Port == 0 {
			body.Port = 80
		}
		if body.Port < 1 || body.Port > 65535 {
			resp.BadRequest(c, "port 超出范围")
			return
		}
		cfgText := generateNginxConfig(body.Type, body.Domain, body.Port, body.Root, body.Proxy)
		path := "/etc/nginx/sites-available/" + body.Name

		// write config via base64-piped tee; immune to heredoc terminator injection
		out, err := client.Run(safeshell.WriteRemoteFile(path, cfgText, true))
		if err != nil {
			resp.InternalError(c, "写入配置失败: "+sshpool.HumanizeErr(out))
			return
		}
		// validate
		out, err = client.Run("sudo -n nginx -t 2>&1")
		if err != nil {
			// rollback
			client.Run("sudo -n rm -f "+sq(path)) //nolint:errcheck
			resp.InternalError(c, "Nginx 配置验证失败: "+sshpool.HumanizeErr(out))
			return
		}
		resp.OK(c, gin.H{"name": body.Name, "path": path})
	}
}

func generateNginxConfig(siteType, domain string, port int, root, proxy string) string {
	switch siteType {
	case "proxy":
		if proxy == "" {
			proxy = "http://127.0.0.1:3000"
		}
		return fmt.Sprintf(`server {
    listen %d;
    server_name %s;

    location / {
        proxy_pass %s;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}`, port, domain, proxy)
	case "php":
		if root == "" {
			root = "/var/www/" + domain
		}
		return fmt.Sprintf(`server {
    listen %d;
    server_name %s;
    root %s;
    index index.php index.html;

    location / {
        try_files $uri $uri/ /index.php?$query_string;
    }

    location ~ \.php$ {
        include snippets/fastcgi-php.conf;
        fastcgi_pass unix:/var/run/php/php-fpm.sock;
    }
}`, port, domain, root)
	default: // static
		if root == "" {
			root = "/var/www/" + domain
		}
		return fmt.Sprintf(`server {
    listen %d;
    server_name %s;
    root %s;
    index index.html index.htm;

    location / {
        try_files $uri $uri/ =404;
    }
}`, port, domain, root)
	}
}

// ── get/put config ────────────────────────────────────────────────────────────

func getSiteConfigHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getRunner(c, db, cfg)
		if !ok {
			return
		}
		name, ok2 := siteName(c)
		if !ok2 {
			return
		}
		path := "/etc/nginx/sites-available/" + name
		out, err := client.Run("cat " + sq(path) + " 2>&1")
		if err != nil {
			resp.InternalError(c, sshpool.HumanizeErr(out))
			return
		}
		resp.OK(c, gin.H{"name": name, "path": path, "content": out})
	}
}

func putSiteConfigHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getRunner(c, db, cfg)
		if !ok {
			return
		}
		name, ok2 := siteName(c)
		if !ok2 {
			return
		}
		var body struct {
			Content string `json:"content" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, "内容不能为空")
			return
		}
		path := "/etc/nginx/sites-available/" + name
		// backup
		backup, _ := client.Run("sudo -n cat " + sq(path) + " 2>/dev/null")
		// write new via base64 (no heredoc terminator injection risk)
		if _, err := client.Run(safeshell.WriteRemoteFile(path, body.Content, true)); err != nil {
			resp.InternalError(c, "写入失败: "+err.Error())
			return
		}
		// validate
		out, err := client.Run("sudo -n nginx -t 2>&1")
		if err != nil {
			// restore backup (only if we had one; an empty file is valid though)
			if _, rerr := client.Run(safeshell.WriteRemoteFile(path, backup, true)); rerr != nil {
				resp.InternalError(c, "Nginx 校验失败且回滚失败: "+sshpool.HumanizeErr(out))
				return
			}
			resp.InternalError(c, "Nginx 配置验证失败: "+sshpool.HumanizeErr(out))
			return
		}
		resp.OK(c, nil)
	}
}

// ── delete/enable/disable ─────────────────────────────────────────────────────

func deleteSiteHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getRunner(c, db, cfg)
		if !ok {
			return
		}
		name, ok2 := siteName(c)
		if !ok2 {
			return
		}
		client.Run("sudo -n rm -f " + sq("/etc/nginx/sites-enabled/"+name)) //nolint:errcheck
		out, err := client.Run("sudo -n rm -f " + sq("/etc/nginx/sites-available/"+name))
		if err != nil {
			resp.InternalError(c, sshpool.HumanizeErr(out))
			return
		}
		client.Run("sudo -n nginx -s reload 2>/dev/null") //nolint:errcheck
		resp.OK(c, nil)
	}
}

func enableSiteHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getRunner(c, db, cfg)
		if !ok {
			return
		}
		name, ok2 := siteName(c)
		if !ok2 {
			return
		}
		src := "/etc/nginx/sites-available/" + name
		dst := "/etc/nginx/sites-enabled/" + name
		out, err := client.Run(fmt.Sprintf("sudo -n ln -sf %s %s && sudo -n nginx -s reload 2>&1", sq(src), sq(dst)))
		if err != nil {
			resp.InternalError(c, sshpool.HumanizeErr(out))
			return
		}
		resp.OK(c, nil)
	}
}

func disableSiteHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getRunner(c, db, cfg)
		if !ok {
			return
		}
		name, ok2 := siteName(c)
		if !ok2 {
			return
		}
		out, err := client.Run("sudo -n rm -f " + sq("/etc/nginx/sites-enabled/"+name) + " && sudo -n nginx -s reload 2>&1")
		if err != nil {
			resp.InternalError(c, sshpool.HumanizeErr(out))
			return
		}
		resp.OK(c, nil)
	}
}

// ── reload/restart ────────────────────────────────────────────────────────────

func reloadHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getRunner(c, db, cfg)
		if !ok {
			return
		}
		out, err := client.Run("sudo -n nginx -s reload 2>&1")
		if err != nil {
			resp.InternalError(c, sshpool.HumanizeErr(out))
			return
		}
		resp.OK(c, gin.H{"output": strings.TrimSpace(out)})
	}
}

func restartHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getRunner(c, db, cfg)
		if !ok {
			return
		}
		out, err := client.Run("sudo -n systemctl restart nginx 2>&1")
		if err != nil {
			resp.InternalError(c, sshpool.HumanizeErr(out))
			return
		}
		resp.OK(c, gin.H{"output": strings.TrimSpace(out)})
	}
}

// ── logs ─────────────────────────────────────────────────────────────────────

func accessLogsHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getDedicatedRunner(c, db, cfg)
		if !ok {
			return
		}
		defer client.Close()
		ws, err := middleware.WSUpgrade(upgrader, c)
		if err != nil {
			return
		}
		defer ws.Close()
		go wsstream.Stream(ws, client, "sudo -n tail -f /var/log/nginx/access.log 2>&1", wsstream.Opts{})
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				break
			}
		}
	}
}

func errorLogsHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getDedicatedRunner(c, db, cfg)
		if !ok {
			return
		}
		defer client.Close()
		ws, err := middleware.WSUpgrade(upgrader, c)
		if err != nil {
			return
		}
		defer ws.Close()
		go wsstream.Stream(ws, client, "sudo -n tail -f /var/log/nginx/error.log 2>&1", wsstream.Opts{})
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				break
			}
		}
	}
}

package nginx

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/pkg/sshpool"
	gossh "golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}

func RegisterRoutes(r *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
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
}

// ── helpers ───────────────────────────────────────────────────────────────────

func getSSH(c *gin.Context, db *gorm.DB, cfg *config.Config) (*gossh.Client, bool) {
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
	var cred string
	switch s.AuthType {
	case "key":
		if s.PrivateKey != "" {
			cred, err = crypto.Decrypt(s.PrivateKey, cfg.Security.AESKey)
		}
	default:
		if s.Password != "" {
			cred, err = crypto.Decrypt(s.Password, cfg.Security.AESKey)
		}
	}
	if err != nil {
		resp.InternalError(c, "解密失败")
		return nil, false
	}
	client, err := sshpool.Connect(s.ID, s.Host, s.Port, s.Username, s.AuthType, cred)
	if err != nil {
		resp.Fail(c, http.StatusServiceUnavailable, 5003, "SSH 连接失败: "+err.Error())
		return nil, false
	}
	return client, true
}

func sq(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func streamSSH(ws *websocket.Conn, client *gossh.Client, cmd string) {
	var mu sync.Mutex
	send := func(v any) {
		b, _ := json.Marshal(v)
		mu.Lock()
		ws.SetWriteDeadline(time.Now().Add(10 * time.Second)) //nolint:errcheck
		ws.WriteMessage(websocket.TextMessage, b)              //nolint:errcheck
		mu.Unlock()
	}
	sess, err := client.NewSession()
	if err != nil {
		send(gin.H{"type": "error", "data": err.Error()})
		return
	}
	defer sess.Close()
	stdout, _ := sess.StdoutPipe()
	if err := sess.Start(cmd); err != nil {
		send(gin.H{"type": "error", "data": err.Error()})
		return
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		send(gin.H{"type": "output", "data": scanner.Text()})
	}
	sess.Wait() //nolint:errcheck
	send(gin.H{"type": "done"})
}

// ── site list ─────────────────────────────────────────────────────────────────

type SiteItem struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Path    string `json:"path"`
}

func listSitesHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getSSH(c, db, cfg)
		if !ok {
			return
		}
		available, _ := sshpool.Run(client, "ls /etc/nginx/sites-available/ 2>/dev/null")
		enabled, _ := sshpool.Run(client, "ls /etc/nginx/sites-enabled/ 2>/dev/null")

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
		client, ok := getSSH(c, db, cfg)
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
		if body.Port == 0 {
			body.Port = 80
		}
		config := generateNginxConfig(body.Type, body.Domain, body.Port, body.Root, body.Proxy)
		path := "/etc/nginx/sites-available/" + body.Name

		// write config (sudo tee so root-owned target is writable)
		out, err := sshpool.Run(client, fmt.Sprintf("sudo -n tee %s > /dev/null << 'NGINX_EOF'\n%s\nNGINX_EOF", sq(path), config))
		if err != nil {
			resp.InternalError(c, "写入配置失败: "+strings.TrimSpace(out))
			return
		}
		// validate
		out, err = sshpool.Run(client, "sudo -n nginx -t 2>&1")
		if err != nil {
			// rollback
			sshpool.Run(client, "sudo -n rm -f "+sq(path)) //nolint:errcheck
			resp.InternalError(c, "Nginx 配置验证失败: "+strings.TrimSpace(out))
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
		client, ok := getSSH(c, db, cfg)
		if !ok {
			return
		}
		name := c.Param("name")
		path := "/etc/nginx/sites-available/" + name
		out, err := sshpool.Run(client, "cat "+sq(path)+" 2>&1")
		if err != nil {
			resp.InternalError(c, strings.TrimSpace(out))
			return
		}
		resp.OK(c, gin.H{"name": name, "path": path, "content": out})
	}
}

func putSiteConfigHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getSSH(c, db, cfg)
		if !ok {
			return
		}
		name := c.Param("name")
		var body struct {
			Content string `json:"content" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, "内容不能为空")
			return
		}
		path := "/etc/nginx/sites-available/" + name
		// backup
		backup, _ := sshpool.Run(client, "sudo -n cat "+sq(path)+" 2>/dev/null")
		// write new
		sshpool.Run(client, fmt.Sprintf("sudo -n tee %s > /dev/null << 'NGINX_EOF'\n%s\nNGINX_EOF", sq(path), body.Content)) //nolint:errcheck
		// validate
		out, err := sshpool.Run(client, "sudo -n nginx -t 2>&1")
		if err != nil {
			// restore backup
			sshpool.Run(client, fmt.Sprintf("sudo -n tee %s > /dev/null << 'NGINX_EOF'\n%s\nNGINX_EOF", sq(path), backup)) //nolint:errcheck
			resp.InternalError(c, "Nginx 配置验证失败: "+strings.TrimSpace(out))
			return
		}
		resp.OK(c, nil)
	}
}

// ── delete/enable/disable ─────────────────────────────────────────────────────

func deleteSiteHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getSSH(c, db, cfg)
		if !ok {
			return
		}
		name := c.Param("name")
		sshpool.Run(client, "sudo -n rm -f "+sq("/etc/nginx/sites-enabled/"+name)) //nolint:errcheck
		out, err := sshpool.Run(client, "sudo -n rm -f "+sq("/etc/nginx/sites-available/"+name))
		if err != nil {
			resp.InternalError(c, strings.TrimSpace(out))
			return
		}
		sshpool.Run(client, "sudo -n nginx -s reload 2>/dev/null") //nolint:errcheck
		resp.OK(c, nil)
	}
}

func enableSiteHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getSSH(c, db, cfg)
		if !ok {
			return
		}
		name := c.Param("name")
		src := "/etc/nginx/sites-available/" + name
		dst := "/etc/nginx/sites-enabled/" + name
		out, err := sshpool.Run(client, fmt.Sprintf("sudo -n ln -sf %s %s && sudo -n nginx -s reload 2>&1", sq(src), sq(dst)))
		if err != nil {
			resp.InternalError(c, strings.TrimSpace(out))
			return
		}
		resp.OK(c, nil)
	}
}

func disableSiteHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getSSH(c, db, cfg)
		if !ok {
			return
		}
		name := c.Param("name")
		out, err := sshpool.Run(client, "sudo -n rm -f "+sq("/etc/nginx/sites-enabled/"+name)+" && sudo -n nginx -s reload 2>&1")
		if err != nil {
			resp.InternalError(c, strings.TrimSpace(out))
			return
		}
		resp.OK(c, nil)
	}
}

// ── reload/restart ────────────────────────────────────────────────────────────

func reloadHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getSSH(c, db, cfg)
		if !ok {
			return
		}
		out, err := sshpool.Run(client, "sudo -n nginx -s reload 2>&1")
		if err != nil {
			resp.InternalError(c, strings.TrimSpace(out))
			return
		}
		resp.OK(c, gin.H{"output": strings.TrimSpace(out)})
	}
}

func restartHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getSSH(c, db, cfg)
		if !ok {
			return
		}
		out, err := sshpool.Run(client, "sudo -n systemctl restart nginx 2>&1")
		if err != nil {
			resp.InternalError(c, strings.TrimSpace(out))
			return
		}
		resp.OK(c, gin.H{"output": strings.TrimSpace(out)})
	}
}

// ── logs ─────────────────────────────────────────────────────────────────────

func accessLogsHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getSSH(c, db, cfg)
		if !ok {
			return
		}
		ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer ws.Close()
		go streamSSH(ws, client, "sudo -n tail -f /var/log/nginx/access.log 2>&1")
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				break
			}
		}
	}
}

func errorLogsHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getSSH(c, db, cfg)
		if !ok {
			return
		}
		ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer ws.Close()
		go streamSSH(ws, client, "sudo -n tail -f /var/log/nginx/error.log 2>&1")
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				break
			}
		}
	}
}

package nginx

import (
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
	// Phase Nginx-P3F: legacy 站点 CRUD 全部下架,替换为 410 Gone。
	// Ingress 模型(POST /api/v1/ingresses)已经覆盖反代/静态站点的全部能力,
	// 旧端点保留只会让前端误调用 + 与 Ingress 渲染产物冲突。这里返回 410 而
	// 不是 404,是为了让仍跑老前端的客户端能从响应里直接看到迁移指引,而不
	// 是单纯"找不到"。
	r.GET("/:id/nginx/sites", legacySiteGoneHandler())
	r.POST("/:id/nginx/sites", legacySiteGoneHandler())
	r.GET("/:id/nginx/sites/:name/config", legacySiteGoneHandler())
	r.PUT("/:id/nginx/sites/:name/config", legacySiteGoneHandler())
	r.DELETE("/:id/nginx/sites/:name", legacySiteGoneHandler())
	r.POST("/:id/nginx/sites/:name/enable", legacySiteGoneHandler())
	r.POST("/:id/nginx/sites/:name/disable", legacySiteGoneHandler())

	r.POST("/:id/nginx/reload", reloadHandler(db, cfg))
	r.POST("/:id/nginx/restart", restartHandler(db, cfg))
	r.GET("/:id/nginx/logs/access", accessLogsHandler(db, cfg))
	r.GET("/:id/nginx/logs/error", errorLogsHandler(db, cfg))
	// Phase Nginx-P3: 多实例 profile 与 nginx -V probe
	RegisterProfileRoutes(r, db, cfg)
}

// legacySiteGoneHandler 把 Phase Nginx-P3F 下架的 7 个 site CRUD 路由统一回吐
// 410 Gone。带 RFC 8594 的 Deprecation/Sunset/Link 头,客户端可据此自动切到
// /api/v1/ingresses。Body 里的中文 message 是给人看的迁移提示。
func legacySiteGoneHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Deprecation", "true")
		c.Header("Sunset", "Wed, 01 Jul 2026 00:00:00 GMT")
		c.Header("Link", `</api/v1/ingresses>; rel="successor-version"`)
		resp.Fail(c, http.StatusGone, 4100,
			"该接口已下线,请改用 /api/v1/ingresses(Ingress 模型已覆盖站点反代/静态托管)")
	}
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

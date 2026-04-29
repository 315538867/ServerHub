package nginx

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/middleware"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/pkg/runner"
	"github.com/serverhub/serverhub/repo"
	"github.com/serverhub/serverhub/pkg/safeshell"
	"github.com/serverhub/serverhub/pkg/sshpool"
	"github.com/serverhub/serverhub/pkg/wsstream"
)

var upgrader = websocket.Upgrader{ReadBufferSize: 4096, WriteBufferSize: 4096}

// 站点 CRUD 已由 Ingress 模型(POST /api/v1/ingresses)接管,nginx 包只剩
// reload/restart/日志/profile 四类"实例级"操作。任何 site 路由都不在此注册——
// 客户端命中老路径直接 404,这就是层架构表达"已下架"的写法。
func RegisterRoutes(r *gin.RouterGroup, db repo.DB, cfg *config.Config) {
	upgrader.CheckOrigin = middleware.WSCheckOrigin(cfg)
	r.POST("/:id/nginx/reload", reloadHandler(db, cfg))
	r.POST("/:id/nginx/restart", restartHandler(db, cfg))
	r.GET("/:id/nginx/logs/access", accessLogsHandler(db, cfg))
	r.GET("/:id/nginx/logs/error", errorLogsHandler(db, cfg))
	RegisterProfileRoutes(r, db, cfg)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func getRunner(c *gin.Context, db repo.DB, cfg *config.Config) (runner.Runner, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.BadRequest(c, "服务器 ID 无效")
		return nil, false
	}
	s, err := repo.GetServerByID(c.Request.Context(), db, uint(id))
	if err != nil {
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

func getDedicatedRunner(c *gin.Context, db repo.DB, cfg *config.Config) (runner.Runner, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.BadRequest(c, "服务器 ID 无效")
		return nil, false
	}
	s, err := repo.GetServerByID(c.Request.Context(), db, uint(id))
	if err != nil {
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

func reloadHandler(db repo.DB, cfg *config.Config) gin.HandlerFunc {
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

func restartHandler(db repo.DB, cfg *config.Config) gin.HandlerFunc {
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

func accessLogsHandler(db repo.DB, cfg *config.Config) gin.HandlerFunc {
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

func errorLogsHandler(db repo.DB, cfg *config.Config) gin.HandlerFunc {
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

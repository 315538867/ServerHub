package docker

import (
	"encoding/json"
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
	"github.com/serverhub/serverhub/pkg/wsstream"
	"gorm.io/gorm"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize: 4096, WriteBufferSize: 4096,
}

// ContainerItem mirrors `docker ps --format '{{json .}}'` fields we care about.
type ContainerItem struct {
	ID      string `json:"id"`
	Names   string `json:"names"`
	Image   string `json:"image"`
	Status  string `json:"status"`
	State   string `json:"state"`
	Ports   string `json:"ports"`
	Created string `json:"created_at"`
}

// ImageItem mirrors `docker images --format '{{json .}}'`.
type ImageItem struct {
	ID         string `json:"id"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Size       string `json:"size"`
	Created    string `json:"created_at"`
}

func RegisterRoutes(r *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	upgrader.CheckOrigin = middleware.WSCheckOrigin(cfg)
	r.GET("/:id/docker/containers", listContainersHandler(db, cfg))
	r.POST("/:id/docker/containers/:cid/action", containerActionHandler(db, cfg))
	r.GET("/:id/docker/containers/:cid/logs", containerLogsHandler(db, cfg))
	r.GET("/:id/docker/containers/:cid/inspect", containerInspectHandler(db, cfg))
	r.GET("/:id/docker/images", listImagesHandler(db, cfg))
	r.GET("/:id/docker/images/pull", pullImageHandler(db, cfg))
	r.DELETE("/:id/docker/images/:iid", deleteImageHandler(db, cfg))
}

// ── helpers ──────────────────────────────────────────────────────────────────

// loadServer fetches the server by :id param, writing any error response itself.
func loadServer(c *gin.Context, db *gorm.DB) (*model.Server, bool) {
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
	return &s, true
}

// getRunner returns a pooled Runner for one-shot operations.
func getRunner(c *gin.Context, db *gorm.DB, cfg *config.Config) (runner.Runner, bool) {
	s, ok := loadServer(c, db)
	if !ok {
		return nil, false
	}
	rn, err := runner.For(s, cfg)
	if err != nil {
		resp.Fail(c, http.StatusServiceUnavailable, 5003, "执行器获取失败: "+err.Error())
		return nil, false
	}
	return rn, true
}

// getDedicatedRunner returns a dedicated Runner for long-lived streams.
func getDedicatedRunner(c *gin.Context, db *gorm.DB, cfg *config.Config) (runner.Runner, bool) {
	s, ok := loadServer(c, db)
	if !ok {
		return nil, false
	}
	rn, err := runner.ForDedicated(s, cfg)
	if err != nil {
		resp.Fail(c, http.StatusServiceUnavailable, 5003, "执行器获取失败: "+err.Error())
		return nil, false
	}
	return rn, true
}

// shellQuote wraps s in single quotes safe for bash.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// ── handlers ─────────────────────────────────────────────────────────────────

func listContainersHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		rn, ok := getRunner(c, db, cfg)
		if !ok {
			return
		}
		out, err := rn.Run(
			`docker ps -a --format '{"id":"{{.ID}}","names":"{{.Names}}","image":"{{.Image}}","status":"{{.Status}}","state":"{{.State}}","ports":"{{.Ports}}","created_at":"{{.CreatedAt}}"}'`)
		if err != nil {
			detail := strings.TrimSpace(out)
			if detail == "" {
				detail = err.Error()
			}
			resp.InternalError(c, "获取容器列表失败: "+detail)
			return
		}
		var items []ContainerItem
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var item ContainerItem
			if json.Unmarshal([]byte(line), &item) == nil {
				items = append(items, item)
			}
		}
		if items == nil {
			items = []ContainerItem{}
		}
		resp.OK(c, items)
	}
}

func containerActionHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		rn, ok := getRunner(c, db, cfg)
		if !ok {
			return
		}
		cid := shellQuote(c.Param("cid"))
		var body struct {
			Action string `json:"action"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, "请求体格式错误")
			return
		}
		var cmd string
		switch body.Action {
		case "start":
			cmd = fmt.Sprintf("docker start %s", cid)
		case "stop":
			cmd = fmt.Sprintf("docker stop %s", cid)
		case "restart":
			cmd = fmt.Sprintf("docker restart %s", cid)
		case "remove":
			cmd = fmt.Sprintf("docker rm -f %s", cid)
		default:
			resp.BadRequest(c, "未知操作: "+body.Action)
			return
		}
		out, err := rn.Run(cmd)
		if err != nil {
			resp.InternalError(c, strings.TrimSpace(out))
			return
		}
		resp.OK(c, gin.H{"output": strings.TrimSpace(out)})
	}
}

func containerLogsHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		rn, ok := getDedicatedRunner(c, db, cfg)
		if !ok {
			return
		}
		defer rn.Close()
		ws, err := middleware.WSUpgrade(upgrader, c)
		if err != nil {
			return
		}
		defer ws.Close()
		cid := shellQuote(c.Param("cid"))
		go wsstream.Stream(ws, rn, fmt.Sprintf("docker logs -f --tail=100 %s 2>&1", cid), wsstream.Opts{})
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				break
			}
		}
	}
}

func containerInspectHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		rn, ok := getRunner(c, db, cfg)
		if !ok {
			return
		}
		cid := shellQuote(c.Param("cid"))
		out, err := rn.Run(fmt.Sprintf("docker inspect %s", cid))
		if err != nil {
			resp.InternalError(c, strings.TrimSpace(out))
			return
		}
		var raw json.RawMessage
		if json.Unmarshal([]byte(strings.TrimSpace(out)), &raw) != nil {
			resp.InternalError(c, "解析容器信息失败")
			return
		}
		resp.OK(c, raw)
	}
}

func listImagesHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		rn, ok := getRunner(c, db, cfg)
		if !ok {
			return
		}
		out, err := rn.Run(
			`docker images --format '{"id":"{{.ID}}","repository":"{{.Repository}}","tag":"{{.Tag}}","size":"{{.Size}}","created_at":"{{.CreatedAt}}"}'`)
		if err != nil {
			detail := strings.TrimSpace(out)
			if detail == "" {
				detail = err.Error()
			}
			resp.InternalError(c, "获取镜像列表失败: "+detail)
			return
		}
		var items []ImageItem
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var item ImageItem
			if json.Unmarshal([]byte(line), &item) == nil {
				items = append(items, item)
			}
		}
		if items == nil {
			items = []ImageItem{}
		}
		resp.OK(c, items)
	}
}

func pullImageHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		rn, ok := getDedicatedRunner(c, db, cfg)
		if !ok {
			return
		}
		defer rn.Close()
		image := strings.TrimSpace(c.Query("image"))
		if image == "" {
			resp.BadRequest(c, "镜像名称不能为空")
			return
		}
		ws, err := middleware.WSUpgrade(upgrader, c)
		if err != nil {
			return
		}
		defer ws.Close()
		go wsstream.Stream(ws, rn, fmt.Sprintf("docker pull %s 2>&1", shellQuote(image)), wsstream.Opts{})
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				break
			}
		}
	}
}

func deleteImageHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		rn, ok := getRunner(c, db, cfg)
		if !ok {
			return
		}
		iid := shellQuote(c.Param("iid"))
		out, err := rn.Run(fmt.Sprintf("docker rmi %s", iid))
		if err != nil {
			resp.InternalError(c, strings.TrimSpace(out))
			return
		}
		resp.OK(c, gin.H{"output": strings.TrimSpace(out)})
	}
}

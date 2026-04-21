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
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/pkg/sshpool"
	"github.com/serverhub/serverhub/pkg/wsstream"
	gossh "golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

var upgrader = websocket.Upgrader{
	CheckOrigin:    func(_ *http.Request) bool { return true },
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
	r.GET("/:id/docker/containers", listContainersHandler(db, cfg))
	r.POST("/:id/docker/containers/:cid/action", containerActionHandler(db, cfg))
	r.GET("/:id/docker/containers/:cid/logs", containerLogsHandler(db, cfg))
	r.GET("/:id/docker/containers/:cid/inspect", containerInspectHandler(db, cfg))
	r.GET("/:id/docker/images", listImagesHandler(db, cfg))
	r.GET("/:id/docker/images/pull", pullImageHandler(db, cfg))
	r.DELETE("/:id/docker/images/:iid", deleteImageHandler(db, cfg))
}

// ── helpers ──────────────────────────────────────────────────────────────────

// loadServerCred looks up the server by :id param, decrypts its credential,
// and writes any error response itself. Returns (server, cred, ok).
func loadServerCred(c *gin.Context, db *gorm.DB, cfg *config.Config) (*model.Server, string, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.BadRequest(c, "服务器 ID 无效")
		return nil, "", false
	}
	var s model.Server
	if err := db.First(&s, id).Error; err != nil {
		resp.NotFound(c, "服务器不存在")
		return nil, "", false
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
		return nil, "", false
	}
	return &s, cred, true
}

// getClient returns a pooled SSH client for short-lived, frequent operations
// (list, inspect, action). The pool reuses a single TCP connection and shares
// a MaxSessions budget — do not use this for long-lived streams.
func getClient(c *gin.Context, db *gorm.DB, cfg *config.Config) (*gossh.Client, bool) {
	s, cred, ok := loadServerCred(c, db, cfg)
	if !ok {
		return nil, false
	}
	client, err := sshpool.Connect(s.ID, s.Host, s.Port, s.Username, s.AuthType, cred)
	if err != nil {
		resp.Fail(c, http.StatusServiceUnavailable, 5003, "SSH 连接失败: "+err.Error())
		return nil, false
	}
	return client, true
}

// getDedicatedClient returns a fresh non-pooled SSH client for long-lived
// WebSocket streams (container logs tail, image pull). The caller MUST
// Close() it to release the TCP connection.
func getDedicatedClient(c *gin.Context, db *gorm.DB, cfg *config.Config) (*gossh.Client, bool) {
	s, cred, ok := loadServerCred(c, db, cfg)
	if !ok {
		return nil, false
	}
	client, err := sshpool.Dial(s.Host, s.Port, s.Username, s.AuthType, cred)
	if err != nil {
		resp.Fail(c, http.StatusServiceUnavailable, 5003, "SSH 连接失败: "+err.Error())
		return nil, false
	}
	return client, true
}

// shellQuote wraps s in single quotes safe for bash.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// streamCmd opens an SSH session, runs cmd, and streams each output line to ws.
func streamCmd(ws *websocket.Conn, client *gossh.Client, cmd string) {
	wsstream.Stream(ws, client, cmd, wsstream.Opts{})
}

// ── handlers ─────────────────────────────────────────────────────────────────

func listContainersHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getClient(c, db, cfg)
		if !ok {
			return
		}
		out, err := sshpool.Run(client,
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
		client, ok := getClient(c, db, cfg)
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
		out, err := sshpool.Run(client, cmd)
		if err != nil {
			resp.InternalError(c, strings.TrimSpace(out))
			return
		}
		resp.OK(c, gin.H{"output": strings.TrimSpace(out)})
	}
}

func containerLogsHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getDedicatedClient(c, db, cfg)
		if !ok {
			return
		}
		defer client.Close()
		ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer ws.Close()
		cid := shellQuote(c.Param("cid"))
		go streamCmd(ws, client, fmt.Sprintf("docker logs -f --tail=100 %s 2>&1", cid))
		// keep alive until client disconnects
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				break
			}
		}
	}
}

func containerInspectHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getClient(c, db, cfg)
		if !ok {
			return
		}
		cid := shellQuote(c.Param("cid"))
		out, err := sshpool.Run(client, fmt.Sprintf("docker inspect %s", cid))
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
		client, ok := getClient(c, db, cfg)
		if !ok {
			return
		}
		out, err := sshpool.Run(client,
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
		client, ok := getDedicatedClient(c, db, cfg)
		if !ok {
			return
		}
		defer client.Close()
		image := strings.TrimSpace(c.Query("image"))
		if image == "" {
			resp.BadRequest(c, "镜像名称不能为空")
			return
		}
		ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer ws.Close()
		go streamCmd(ws, client, fmt.Sprintf("docker pull %s 2>&1", shellQuote(image)))
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				break
			}
		}
	}
}

func deleteImageHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getClient(c, db, cfg)
		if !ok {
			return
		}
		iid := shellQuote(c.Param("iid"))
		out, err := sshpool.Run(client, fmt.Sprintf("docker rmi %s", iid))
		if err != nil {
			resp.InternalError(c, strings.TrimSpace(out))
			return
		}
		resp.OK(c, gin.H{"output": strings.TrimSpace(out)})
	}
}

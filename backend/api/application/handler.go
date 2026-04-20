package application

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/pkg/sshpool"
	gossh "golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

func RegisterRoutes(r *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	r.GET("", listHandler(db))
	r.POST("", createHandler(db, cfg))
	r.GET("/:id", getHandler(db))
	r.PUT("/:id", updateHandler(db))
	r.DELETE("/:id", deleteHandler(db))
	r.GET("/:id/dirs", dirsHandler(db, cfg))
	r.POST("/:id/init-dirs", initDirsHandler(db, cfg))
}

type appReq struct {
	Name          string `json:"name" binding:"required"`
	Description   string `json:"description"`
	ServerID      uint   `json:"server_id" binding:"required"`
	SiteName      string `json:"site_name"`
	Domain        string `json:"domain"`
	ContainerName string `json:"container_name"`
	BaseDir       string `json:"base_dir"`
	ExposeMode    string `json:"expose_mode"`
	DeployID      *uint  `json:"deploy_id"`
	DBConnID      *uint  `json:"db_conn_id"`
}

// ── SSH helper ────────────────────────────────────────────────────────────────

func connectSSH(db *gorm.DB, cfg *config.Config, serverID uint) (*gossh.Client, error) {
	var s model.Server
	if err := db.First(&s, serverID).Error; err != nil {
		return nil, fmt.Errorf("服务器不存在")
	}
	var (
		cred string
		err  error
	)
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
		return nil, fmt.Errorf("解密失败: %w", err)
	}
	return sshpool.Connect(s.ID, s.Host, s.Port, s.Username, s.AuthType, cred)
}

func initAppDirs(db *gorm.DB, cfg *config.Config, app *model.Application) error {
	client, err := connectSSH(db, cfg, app.ServerID)
	if err != nil {
		return err
	}
	cmd := fmt.Sprintf("mkdir -p %s/data %s/logs %s/config %s/backup",
		app.BaseDir, app.BaseDir, app.BaseDir, app.BaseDir)
	_, err = sshpool.Run(client, cmd)
	return err
}

// ── list ──────────────────────────────────────────────────────────────────────

func listHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var apps []model.Application
		q := db.Order("id asc")
		if sid := c.Query("server_id"); sid != "" {
			q = q.Where("server_id = ?", sid)
		}
		q.Find(&apps)
		resp.OK(c, apps)
	}
}

// ── create ────────────────────────────────────────────────────────────────────

func createHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req appReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		var server model.Server
		if err := db.First(&server, req.ServerID).Error; err != nil {
			resp.BadRequest(c, "服务器不存在")
			return
		}
		exposeMode := req.ExposeMode
		if exposeMode != "path" && exposeMode != "site" {
			exposeMode = "none"
		}
		baseDir := req.BaseDir
		if baseDir == "" {
			baseDir = "/srv/apps/" + req.Name
		}
		app := model.Application{
			Name:          req.Name,
			Description:   req.Description,
			ServerID:      req.ServerID,
			SiteName:      req.SiteName,
			Domain:        req.Domain,
			ContainerName: req.ContainerName,
			BaseDir:       baseDir,
			ExposeMode:    exposeMode,
			DeployID:      req.DeployID,
			DBConnID:      req.DBConnID,
			Status:        "unknown",
		}
		if err := db.Create(&app).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		go func() {
			_ = initAppDirs(db, cfg, &app)
		}()
		resp.OK(c, app)
	}
}

// ── get ───────────────────────────────────────────────────────────────────────

func getHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			resp.BadRequest(c, "无效 ID")
			return
		}
		var app model.Application
		if err := db.First(&app, id).Error; err != nil {
			resp.NotFound(c, "应用不存在")
			return
		}
		resp.OK(c, app)
	}
}

// ── update ────────────────────────────────────────────────────────────────────

func updateHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			resp.BadRequest(c, "无效 ID")
			return
		}
		var app model.Application
		if err := db.First(&app, id).Error; err != nil {
			resp.NotFound(c, "应用不存在")
			return
		}
		var req appReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		app.Name = req.Name
		app.Description = req.Description
		app.ServerID = req.ServerID
		app.SiteName = req.SiteName
		app.Domain = req.Domain
		app.ContainerName = req.ContainerName
		app.DeployID = req.DeployID
		app.DBConnID = req.DBConnID
		if req.ExposeMode == "path" || req.ExposeMode == "site" || req.ExposeMode == "none" {
			app.ExposeMode = req.ExposeMode
		}
		if err := db.Save(&app).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, app)
	}
}

// ── delete ────────────────────────────────────────────────────────────────────

func deleteHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			resp.BadRequest(c, "无效 ID")
			return
		}
		if err := db.Delete(&model.Application{}, id).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, nil)
	}
}

// ── dirs ──────────────────────────────────────────────────────────────────────

type dirEntry struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Status string `json:"status"` // "ok" | "missing"
	Size   string `json:"size"`
	Mtime  string `json:"mtime"`
}

func dirsHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			resp.BadRequest(c, "无效 ID")
			return
		}
		var app model.Application
		if err := db.First(&app, id).Error; err != nil {
			resp.NotFound(c, "应用不存在")
			return
		}
		if app.BaseDir == "" {
			resp.OK(c, []dirEntry{})
			return
		}
		client, err := connectSSH(db, cfg, app.ServerID)
		if err != nil {
			resp.Fail(c, http.StatusServiceUnavailable, 5003, "SSH 连接失败: "+err.Error())
			return
		}
		cmd := fmt.Sprintf(`for d in data logs config backup; do
  p="%s/$d"
  if [ -d "$p" ]; then
    sz=$(du -sh "$p" 2>/dev/null | cut -f1)
    mt=$(date -r "$p" "+%%Y-%%m-%%d %%H:%%M:%%S" 2>/dev/null || stat -c "%%y" "$p" 2>/dev/null | cut -d'.' -f1)
    echo "$d|$sz|$mt|ok"
  else
    echo "$d|||missing"
  fi
done`, app.BaseDir)
		out, err := sshpool.Run(client, cmd)
		if err != nil {
			resp.InternalError(c, "执行失败: "+err.Error())
			return
		}
		entries := make([]dirEntry, 0, 4)
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, "|", 4)
			if len(parts) != 4 {
				continue
			}
			name := parts[0]
			entries = append(entries, dirEntry{
				Name:   name,
				Path:   app.BaseDir + "/" + name,
				Size:   parts[1],
				Mtime:  parts[2],
				Status: parts[3],
			})
		}
		resp.OK(c, entries)
	}
}

func initDirsHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			resp.BadRequest(c, "无效 ID")
			return
		}
		var app model.Application
		if err := db.First(&app, id).Error; err != nil {
			resp.NotFound(c, "应用不存在")
			return
		}
		if app.BaseDir == "" {
			resp.BadRequest(c, "应用未设置 base_dir")
			return
		}
		if err := initAppDirs(db, cfg, &app); err != nil {
			resp.Fail(c, http.StatusServiceUnavailable, 5003, "初始化失败: "+err.Error())
			return
		}
		resp.OK(c, gin.H{"message": "目录初始化成功"})
	}
}

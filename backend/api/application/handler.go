package application

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/derive"
	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/pkg/runner"
	"github.com/serverhub/serverhub/pkg/safeshell"
	"github.com/serverhub/serverhub/repo"
	"github.com/serverhub/serverhub/usecase"
)

func RegisterRoutes(r *gin.RouterGroup, db repo.DB, cfg *config.Config) {
	r.GET("", listHandler(db))
	r.POST("", createHandler(db, cfg))
	r.GET("/:id", getHandler(db))
	r.PUT("/:id", updateHandler(db))
	r.DELETE("/:id", deleteHandler(db))
	r.GET("/:id/dirs", dirsHandler(db, cfg))
	r.POST("/:id/init-dirs", initDirsHandler(db, cfg))
	r.GET("/:id/metrics", metricsHandler(db, cfg))
	r.GET("/:id/services", listServicesHandler(db))
	r.POST("/:id/services/:sid/attach", attachServiceHandler(db))
	r.DELETE("/:id/services/:sid/attach", detachServiceHandler(db))
	r.GET("/:id/ingresses", listAppIngressesHandler(db))
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

func runnerForServer(ctx *gin.Context, db repo.DB, cfg *config.Config, serverID uint) (runner.Runner, error) {
	s, err := repo.GetServerByID(ctx.Request.Context(), db, serverID)
	if err != nil {
		return nil, fmt.Errorf("服务器不存在")
	}
	return runner.For(&s, cfg)
}

func initAppDirs(c *gin.Context, db repo.DB, cfg *config.Config, app *domain.Application) error {
	if err := safeshell.AbsPath(app.BaseDir); err != nil {
		return fmt.Errorf("base_dir 非法: %w", err)
	}
	r, err := runnerForServer(c, db, cfg, app.ServerID)
	if err != nil {
		return err
	}
	bd := safeshell.Quote(app.BaseDir)
	cmd := fmt.Sprintf("mkdir -p %s/data %s/logs %s/config %s/backup",
		bd, bd, bd, bd)
	_, err = r.Run(cmd)
	return err
}

// validateAppReq enforces shell/path safety on user-controlled fields.
func validateAppReq(req *appReq) error {
	if err := safeshell.ValidName(req.Name, 64); err != nil {
		return fmt.Errorf("name 非法: %w", err)
	}
	if req.BaseDir != "" {
		if err := safeshell.AbsPath(req.BaseDir); err != nil {
			return fmt.Errorf("base_dir 非法: %w", err)
		}
	}
	if req.ContainerName != "" {
		if err := safeshell.ValidName(req.ContainerName, 64); err != nil {
			return fmt.Errorf("container_name 非法: %w", err)
		}
	}
	if req.SiteName != "" {
		if err := safeshell.ValidName(req.SiteName, 64); err != nil {
			return fmt.Errorf("site_name 非法: %w", err)
		}
	}
	if req.Domain != "" {
		if err := safeshell.NginxValue(req.Domain); err != nil {
			return fmt.Errorf("domain 非法: %w", err)
		}
	}
	return nil
}

// ── list ──────────────────────────────────────────────────────────────────────

func listHandler(db repo.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var serverID *uint
		if sid := c.Query("server_id"); sid != "" {
			v, err := strconv.Atoi(sid)
			if err != nil || v <= 0 {
				resp.BadRequest(c, "server_id 无效")
				return
			}
			uid := uint(v)
			serverID = &uid
		}
		out, err := usecase.ListApplications(c.Request.Context(), db, serverID)
		if err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, out)
	}
}

// ── services 子路由 ───────────────────────────────────────────────────────────

func listServicesHandler(db repo.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			resp.BadRequest(c, "无效 ID")
			return
		}
		services, err := repo.ListServicesByApplicationID(c.Request.Context(), db, uint(id))
		if err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, services)
	}
}

func attachServiceHandler(db repo.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		appID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			resp.BadRequest(c, "无效应用 ID")
			return
		}
		sid, err := strconv.Atoi(c.Param("sid"))
		if err != nil {
			resp.BadRequest(c, "无效服务 ID")
			return
		}
		svc, err := usecase.AttachService(c.Request.Context(), db, uint(appID), uint(sid))
		if err != nil {
			if repo.IsNotFound(err) {
				resp.NotFound(c, err.Error())
			} else {
				resp.BadRequest(c, err.Error())
			}
			return
		}
		resp.OK(c, svc)
	}
}

func detachServiceHandler(db repo.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		appID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			resp.BadRequest(c, "无效应用 ID")
			return
		}
		sid, err := strconv.Atoi(c.Param("sid"))
		if err != nil {
			resp.BadRequest(c, "无效服务 ID")
			return
		}
		if err := usecase.DetachService(c.Request.Context(), db, uint(appID), uint(sid)); err != nil {
			if repo.IsNotFound(err) {
				resp.NotFound(c, err.Error())
			} else {
				resp.BadRequest(c, err.Error())
			}
			return
		}
		resp.OK(c, nil)
	}
}

// ── create ────────────────────────────────────────────────────────────────────

func createHandler(db repo.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req appReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		if err := validateAppReq(&req); err != nil {
			resp.BadRequest(c, err.Error())
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
		if err := safeshell.AbsPath(baseDir); err != nil {
			resp.BadRequest(c, "base_dir 非法: "+err.Error())
			return
		}
		app := domain.Application{
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
		}
		if err := usecase.CreateApplication(c.Request.Context(), db, &app); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		go func() {
			_ = initAppDirs(c, db, cfg, &app)
		}()
		resp.OK(c, usecase.AppWithStatus{Application: app, Status: derive.AppStatusUnknown})
	}
}

// ── get ───────────────────────────────────────────────────────────────────────

func getHandler(db repo.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			resp.BadRequest(c, "无效 ID")
			return
		}
		out, err := usecase.GetApplication(c.Request.Context(), db, uint(id))
		if err != nil {
			resp.NotFound(c, "应用不存在")
			return
		}
		resp.OK(c, out)
	}
}

// ── update ────────────────────────────────────────────────────────────────────

func updateHandler(db repo.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			resp.BadRequest(c, "无效 ID")
			return
		}
		ctx := c.Request.Context()
		app, err := repo.GetApplicationByID(ctx, db, uint(id))
		if err != nil {
			resp.NotFound(c, "应用不存在")
			return
		}
		var req appReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		if err := validateAppReq(&req); err != nil {
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
		out, err := usecase.UpdateApplication(ctx, db, &app)
		if err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, out)
	}
}

// ── delete ────────────────────────────────────────────────────────────────────

func deleteHandler(db repo.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			resp.BadRequest(c, "无效 ID")
			return
		}
		if err := usecase.DeleteApplication(c.Request.Context(), db, uint(id)); err != nil {
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

func dirsHandler(db repo.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			resp.BadRequest(c, "无效 ID")
			return
		}
		app, err := repo.GetApplicationByID(c.Request.Context(), db, uint(id))
		if err != nil {
			resp.NotFound(c, "应用不存在")
			return
		}
		if app.BaseDir == "" {
			resp.OK(c, []dirEntry{})
			return
		}
		if err := safeshell.AbsPath(app.BaseDir); err != nil {
			resp.BadRequest(c, "base_dir 非法: "+err.Error())
			return
		}
		client, err := runnerForServer(c, db, cfg, app.ServerID)
		if err != nil {
			resp.Fail(c, http.StatusServiceUnavailable, 5003, "连接失败: "+err.Error())
			return
		}
		bd := safeshell.Quote(app.BaseDir)
		cmd := fmt.Sprintf(`for d in data logs config backup; do
  p=%s/$d
  if [ -d "$p" ]; then
    sz=$(du -sh "$p" 2>/dev/null | cut -f1)
    mt=$(date -r "$p" "+%%Y-%%m-%%d %%H:%%M:%%S" 2>/dev/null || stat -c "%%y" "$p" 2>/dev/null | cut -d'.' -f1)
    echo "$d|$sz|$mt|ok"
  else
    echo "$d|||missing"
  fi
done`, bd)
		out, err := client.Run(cmd)
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

func initDirsHandler(db repo.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			resp.BadRequest(c, "无效 ID")
			return
		}
		app, err := repo.GetApplicationByID(c.Request.Context(), db, uint(id))
		if err != nil {
			resp.NotFound(c, "应用不存在")
			return
		}
		if app.BaseDir == "" {
			resp.BadRequest(c, "应用未设置 base_dir")
			return
		}
		if err := initAppDirs(c, db, cfg, &app); err != nil {
			resp.Fail(c, http.StatusServiceUnavailable, 5003, "初始化失败: "+err.Error())
			return
		}
		resp.OK(c, gin.H{"message": "目录初始化成功"})
	}
}

// ── metrics ───────────────────────────────────────────────────────────────────

type appMetrics struct {
	Available   bool    `json:"available"`
	Reason      string  `json:"reason,omitempty"`
	CPUPercent  float64 `json:"cpu_percent"`
	MemUsage    string  `json:"mem_usage"`
	MemPercent  float64 `json:"mem_percent"`
	NetIO       string  `json:"net_io"`
	BlockIO     string  `json:"block_io"`
	PIDs        int     `json:"pids"`
	ContainerID string  `json:"container_id"`
	Timestamp   int64   `json:"ts"`
}

type dockerStatsLine struct {
	ID       string `json:"ID"`
	CPUPerc  string `json:"CPUPerc"`
	MemUsage string `json:"MemUsage"`
	MemPerc  string `json:"MemPerc"`
	NetIO    string `json:"NetIO"`
	BlockIO  string `json:"BlockIO"`
	PIDs     string `json:"PIDs"`
}

func metricsHandler(db repo.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			resp.BadRequest(c, "无效 ID")
			return
		}
		app, err := repo.GetApplicationByID(c.Request.Context(), db, uint(id))
		if err != nil {
			resp.NotFound(c, "应用不存在")
			return
		}
		if app.ContainerName == "" {
			resp.OK(c, appMetrics{Available: false, Reason: "未关联容器"})
			return
		}
		client, err := runnerForServer(c, db, cfg, app.ServerID)
		if err != nil {
			resp.OK(c, appMetrics{Available: false, Reason: "连接失败: " + err.Error()})
			return
		}
		cmd := fmt.Sprintf(
			"docker stats --no-stream --format '{{json .}}' %s 2>/dev/null",
			shellQuoteSafe(app.ContainerName),
		)
		out, err := client.Run(cmd)
		if err != nil || strings.TrimSpace(out) == "" {
			reason := "容器未运行或不存在"
			if err != nil {
				reason = err.Error()
			}
			resp.OK(c, appMetrics{Available: false, Reason: reason})
			return
		}
		line := strings.TrimSpace(strings.Split(out, "\n")[0])
		var s dockerStatsLine
		if err := json.Unmarshal([]byte(line), &s); err != nil {
			resp.OK(c, appMetrics{Available: false, Reason: "解析 docker stats 失败"})
			return
		}
		m := appMetrics{
			Available:   true,
			CPUPercent:  parsePercent(s.CPUPerc),
			MemUsage:    s.MemUsage,
			MemPercent:  parsePercent(s.MemPerc),
			NetIO:       s.NetIO,
			BlockIO:     s.BlockIO,
			PIDs:        atoiSafe(s.PIDs),
			ContainerID: s.ID,
			Timestamp:   timeNowUnix(),
		}
		resp.OK(c, m)
	}
}

// ── ingresses 反向视图 ───────────────────────────────────────────────────────

func listAppIngressesHandler(db repo.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil || id <= 0 {
			resp.BadRequest(c, "无效 ID")
			return
		}
		out, err := usecase.ListAppIngresses(c.Request.Context(), db, uint(id))
		if err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, out)
	}
}

// ── 工具函数 ─────────────────────────────────────────────────────────────────

func shellQuoteSafe(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func parsePercent(s string) float64 {
	s = strings.TrimSpace(strings.TrimSuffix(s, "%"))
	if s == "" || s == "--" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

func atoiSafe(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}

func timeNowUnix() int64 {
	return timeNowFn()
}

// 允许测试时替换；生产用 time.Now().Unix()
var timeNowFn = func() int64 {
	return time.Now().Unix()
}

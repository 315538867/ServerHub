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
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/pkg/runner"
	"github.com/serverhub/serverhub/pkg/safeshell"
	"gorm.io/gorm"
)

// appResp 是 Application API 的统一响应壳:嵌入 model.Application 把全部基础字段
// 平铺到 JSON,Status 字段从 derive.AppStatus 派生(R3 起 Application.Status 列已下线)。
//
// 历史兼容:JSON 字段名 "status" 与 R2 之前一致,前端 TS 类型不需要改。
type appResp struct {
	model.Application
	Status string `json:"status"`
}

func toAppResp(db *gorm.DB, a model.Application) appResp {
	m := derive.AppStatus(db, []uint{a.ID})
	return appResp{Application: a, Status: m[a.ID].Result}
}

func RegisterRoutes(r *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
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

func runnerForServer(db *gorm.DB, cfg *config.Config, serverID uint) (runner.Runner, error) {
	var s model.Server
	if err := db.First(&s, serverID).Error; err != nil {
		return nil, fmt.Errorf("服务器不存在")
	}
	return runner.For(&s, cfg)
}

func initAppDirs(db *gorm.DB, cfg *config.Config, app *model.Application) error {
	if err := safeshell.AbsPath(app.BaseDir); err != nil {
		return fmt.Errorf("base_dir 非法: %w", err)
	}
	r, err := runnerForServer(db, cfg, app.ServerID)
	if err != nil {
		return err
	}
	bd := safeshell.Quote(app.BaseDir)
	cmd := fmt.Sprintf("mkdir -p %s/data %s/logs %s/config %s/backup",
		bd, bd, bd, bd)
	_, err = r.Run(cmd)
	return err
}

// validateAppReq enforces shell/path safety on user-controlled fields that
// are spliced into remote commands or filesystem paths.
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

func listHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var apps []model.Application
		q := db.Order("id asc")
		if sid := c.Query("server_id"); sid != "" {
			q = q.Where("server_id = ?", sid)
		}
		q.Find(&apps)
		// R3 起 Application.Status 列已下线,改由 derive.AppStatus 派生:
		//   任一 Service.DeployRun.Status=failed → error
		//   任一 syncing → syncing
		//   全 success → running
		//   无 Service / 全 unknown → unknown
		ids := make([]uint, len(apps))
		for i, a := range apps {
			ids[i] = a.ID
		}
		statusMap := derive.AppStatus(db, ids)
		out := make([]appResp, len(apps))
		for i, a := range apps {
			out[i] = appResp{Application: a, Status: statusMap[a.ID].Result}
		}
		resp.OK(c, out)
	}
}

// ── services 子路由 ───────────────────────────────────────────────────────────

func listServicesHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			resp.BadRequest(c, "无效 ID")
			return
		}
		var services []model.Service
		db.Where("application_id = ?", id).Order("id asc").Find(&services)
		resp.OK(c, services)
	}
}

func attachServiceHandler(db *gorm.DB) gin.HandlerFunc {
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
		var app model.Application
		if err := db.First(&app, appID).Error; err != nil {
			resp.NotFound(c, "应用不存在")
			return
		}
		var svc model.Service
		if err := db.First(&svc, sid).Error; err != nil {
			resp.NotFound(c, "服务不存在")
			return
		}
		if svc.ServerID != app.ServerID {
			resp.BadRequest(c, "服务与应用不在同一服务器，不可挂载")
			return
		}
		appIDu := uint(appID)
		svc.ApplicationID = &appIDu
		if err := db.Save(&svc).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		if app.PrimaryServiceID == nil {
			svcID := svc.ID
			db.Model(&app).Update("primary_service_id", svcID)
		}
		resp.OK(c, svc)
	}
}

func detachServiceHandler(db *gorm.DB) gin.HandlerFunc {
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
		var svc model.Service
		if err := db.First(&svc, sid).Error; err != nil {
			resp.NotFound(c, "服务不存在")
			return
		}
		if svc.ApplicationID == nil || *svc.ApplicationID != uint(appID) {
			resp.BadRequest(c, "该服务未挂在此应用下")
			return
		}
		svc.ApplicationID = nil
		if err := db.Save(&svc).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		// 若主服务被卸下，清掉 PrimaryServiceID
		db.Model(&model.Application{}).
			Where("id = ? AND primary_service_id = ?", appID, sid).
			Update("primary_service_id", nil)
		resp.OK(c, nil)
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
		if err := validateAppReq(&req); err != nil {
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
		if err := safeshell.AbsPath(baseDir); err != nil {
			resp.BadRequest(c, "base_dir 非法: "+err.Error())
			return
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
		}
		if err := db.Create(&app).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		go func() {
			_ = initAppDirs(db, cfg, &app)
		}()
		// 刚创建尚无 Service → derive 返回 unknown,无需打 DB
		resp.OK(c, appResp{Application: app, Status: derive.AppStatusUnknown})
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
		resp.OK(c, toAppResp(db, app))
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
		if err := db.Save(&app).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, toAppResp(db, app))
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
		if err := safeshell.AbsPath(app.BaseDir); err != nil {
			resp.BadRequest(c, "base_dir 非法: "+err.Error())
			return
		}
		client, err := runnerForServer(db, cfg, app.ServerID)
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

// ── metrics ───────────────────────────────────────────────────────────────────
//
// GET /api/applications/:id/metrics
// 通过 SSH 调 `docker stats --no-stream --format '{{json .}}'` 取关联容器的实时指标。

type appMetrics struct {
	Available   bool    `json:"available"`
	Reason      string  `json:"reason,omitempty"`
	CPUPercent  float64 `json:"cpu_percent"`
	MemUsage    string  `json:"mem_usage"`    // e.g. "128.5MiB / 2GiB"
	MemPercent  float64 `json:"mem_percent"`
	NetIO       string  `json:"net_io"`       // e.g. "1.2MB / 340kB"
	BlockIO     string  `json:"block_io"`
	PIDs        int     `json:"pids"`
	ContainerID string  `json:"container_id"`
	Timestamp   int64   `json:"ts"`
}

// docker stats JSON 行字段名（Docker CLI 输出格式）
type dockerStatsLine struct {
	ID       string `json:"ID"`
	CPUPerc  string `json:"CPUPerc"`
	MemUsage string `json:"MemUsage"`
	MemPerc  string `json:"MemPerc"`
	NetIO    string `json:"NetIO"`
	BlockIO  string `json:"BlockIO"`
	PIDs     string `json:"PIDs"`
}

func metricsHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
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
		if app.ContainerName == "" {
			resp.OK(c, appMetrics{Available: false, Reason: "未关联容器"})
			return
		}
		client, err := runnerForServer(db, cfg, app.ServerID)
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

// listAppIngressesHandler 反向视图:返回引用了本 app 任一 Service 的所有 Ingress,
// 每条 Ingress 附带 EdgeServerName + MatchingRoutes(只命中本 app 的那些子路由,
// 因为同一 Ingress 可能既路由到本 app 又路由到其它 app/raw,前端反向视图只关心
// 本 app 的部分)。
//
// 实现思路:Upstream 是 JSON text 列,直接 SQL 查 service_id 不可靠。两步走:
//   1) services WHERE application_id=? → service_id 集合
//   2) ingress_routes 全表扫(GORM 反序列化 Upstream),内存过滤 type=service &&
//      service_id ∈ 集合,聚到 ingress_id;再回拉 Ingress + Server.Name。
//
// 数据量评估:Ingress 是低频实体(一台 edge 几十条上限),全表扫可接受;后续若涨到
// 千级,可改为 SQL JSON 函数(SQLite/PG 都支持)或新建反向索引列。
func listAppIngressesHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil || id <= 0 {
			resp.BadRequest(c, "无效 ID")
			return
		}
		var serviceIDs []uint
		if err := db.Model(&model.Service{}).
			Where("application_id = ?", id).
			Pluck("id", &serviceIDs).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		type appIngressDTO struct {
			model.Ingress
			EdgeServerName string               `json:"edge_server_name"`
			MatchingRoutes []model.IngressRoute `json:"matching_routes"`
		}
		if len(serviceIDs) == 0 {
			resp.OK(c, []appIngressDTO{})
			return
		}
		sidSet := make(map[uint]struct{}, len(serviceIDs))
		for _, s := range serviceIDs {
			sidSet[s] = struct{}{}
		}
		var routes []model.IngressRoute
		if err := db.Order("ingress_id, sort, id").Find(&routes).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		routesByIngress := map[uint][]model.IngressRoute{}
		for _, rt := range routes {
			if rt.Upstream.Type != "service" || rt.Upstream.ServiceID == nil {
				continue
			}
			if _, ok := sidSet[*rt.Upstream.ServiceID]; !ok {
				continue
			}
			routesByIngress[rt.IngressID] = append(routesByIngress[rt.IngressID], rt)
		}
		if len(routesByIngress) == 0 {
			resp.OK(c, []appIngressDTO{})
			return
		}
		ingressIDs := make([]uint, 0, len(routesByIngress))
		for id := range routesByIngress {
			ingressIDs = append(ingressIDs, id)
		}
		var ingresses []model.Ingress
		if err := db.Where("id IN ?", ingressIDs).Order("id").Find(&ingresses).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		serverIDSet := map[uint]struct{}{}
		for _, ig := range ingresses {
			serverIDSet[ig.EdgeServerID] = struct{}{}
		}
		nameByID := map[uint]string{}
		if len(serverIDSet) > 0 {
			sids := make([]uint, 0, len(serverIDSet))
			for id := range serverIDSet {
				sids = append(sids, id)
			}
			var servers []model.Server
			db.Select("id, name").Where("id IN ?", sids).Find(&servers)
			for _, s := range servers {
				nameByID[s.ID] = s.Name
			}
		}
		out := make([]appIngressDTO, 0, len(ingresses))
		for _, ig := range ingresses {
			out = append(out, appIngressDTO{
				Ingress:        ig,
				EdgeServerName: nameByID[ig.EdgeServerID],
				MatchingRoutes: routesByIngress[ig.ID],
			})
		}
		resp.OK(c, out)
	}
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

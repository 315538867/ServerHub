package approutes

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/nginxops"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/pkg/safeshell"
	"gorm.io/gorm"
)

func RegisterRoutes(r *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	r.GET("/:id/nginx", getNginxHandler(db))
	r.PUT("/:id/nginx/mode", setModeHandler(db, cfg))
	r.POST("/:id/nginx/routes", addRouteHandler(db))
	r.PUT("/:id/nginx/routes/:rid", updateRouteHandler(db))
	r.DELETE("/:id/nginx/routes/:rid", deleteRouteHandler(db))
	r.POST("/:id/nginx/apply", applyHandler(db, cfg))
}

// ── helpers ───────────────────────────────────────────────────────────────────

// validateRoute rejects route fields that would let a caller break out of the
// nginx directive or inject a shell terminator when written via base64+tee.
func validateRoute(r *routeReq) error {
	if err := safeshell.NginxValue(r.Path); err != nil {
		return fmt.Errorf("path 非法: %w", err)
	}
	if err := safeshell.NginxValue(r.Upstream); err != nil {
		return fmt.Errorf("upstream 非法: %w", err)
	}
	if r.Extra != "" {
		// Extra is spliced as a raw directive line — disallow newlines and
		// braces so callers cannot open/close nested contexts.
		if strings.ContainsAny(r.Extra, "\n\r{}") {
			return fmt.Errorf("extra 包含非法字符")
		}
	}
	return nil
}

// validateAppName ensures an Application.Name is safe to use as the filename
// for its generated nginx location/site include. Called in applyHandler.
func validateAppName(name string) error {
	return safeshell.ValidName(name, 64)
}

func getApp(c *gin.Context, db *gorm.DB) (*model.Application, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.BadRequest(c, "应用 ID 无效")
		return nil, false
	}
	var app model.Application
	if err := db.First(&app, id).Error; err != nil {
		resp.NotFound(c, "应用不存在")
		return nil, false
	}
	return &app, true
}

// ── GET /:id/nginx ────────────────────────────────────────────────────────────

type nginxConfig struct {
	ExposeMode string               `json:"expose_mode"`
	Routes     []model.AppNginxRoute `json:"routes"`
}

func getNginxHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		app, ok := getApp(c, db)
		if !ok {
			return
		}
		var routes []model.AppNginxRoute
		db.Where("app_id = ?", app.ID).Order("sort asc, id asc").Find(&routes)
		if routes == nil {
			routes = []model.AppNginxRoute{}
		}
		resp.OK(c, nginxConfig{ExposeMode: app.ExposeMode, Routes: routes})
	}
}

// ── PUT /:id/nginx/mode ───────────────────────────────────────────────────────

func setModeHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		app, ok := getApp(c, db)
		if !ok {
			return
		}
		var body struct {
			Mode string `json:"mode" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, "mode 字段不能为空")
			return
		}
		if body.Mode != "none" && body.Mode != "path" && body.Mode != "site" {
			resp.BadRequest(c, "mode 取值为 none / path / site")
			return
		}
		if err := db.Model(app).Update("expose_mode", body.Mode).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		app.ExposeMode = body.Mode
		// 桥接：mode 变化时全量重灌该 app 的 IngressRoute（含模式 none 的清理）。
		if err := resyncAppRoutes(db, app); err != nil {
			fmt.Printf("resyncAppRoutes(app=%d mode=%s): %v\n", app.ID, body.Mode, err)
		}
		resp.OK(c, nil)
	}
}

// ── POST /:id/nginx/routes ────────────────────────────────────────────────────

type routeReq struct {
	Path     string `json:"path" binding:"required"`
	Upstream string `json:"upstream" binding:"required"`
	Extra    string `json:"extra"`
	Sort     int    `json:"sort"`
}

func addRouteHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		app, ok := getApp(c, db)
		if !ok {
			return
		}
		var req routeReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		if err := validateRoute(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		route := model.AppNginxRoute{
			AppID:    app.ID,
			Path:     req.Path,
			Upstream: req.Upstream,
			Extra:    req.Extra,
			Sort:     req.Sort,
		}
		if err := db.Create(&route).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		// 桥接：同步写新 Ingress 模型。失败不影响旧链路，仅记日志。
		if err := syncToIngress(db, app, &route); err != nil {
			fmt.Printf("syncToIngress(create app=%d route=%d): %v\n", app.ID, route.ID, err)
		}
		resp.OK(c, route)
	}
}

// ── PUT /:id/nginx/routes/:rid ────────────────────────────────────────────────

func updateRouteHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		app, ok := getApp(c, db)
		if !ok {
			return
		}
		rid, err := strconv.Atoi(c.Param("rid"))
		if err != nil {
			resp.BadRequest(c, "路由 ID 无效")
			return
		}
		var route model.AppNginxRoute
		if err := db.Where("id = ? AND app_id = ?", rid, app.ID).First(&route).Error; err != nil {
			resp.NotFound(c, "路由不存在")
			return
		}
		var req routeReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		if err := validateRoute(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		route.Path = req.Path
		route.Upstream = req.Upstream
		route.Extra = req.Extra
		route.Sort = req.Sort
		if err := db.Save(&route).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		if err := syncToIngress(db, app, &route); err != nil {
			fmt.Printf("syncToIngress(update app=%d route=%d): %v\n", app.ID, route.ID, err)
		}
		resp.OK(c, route)
	}
}

// ── DELETE /:id/nginx/routes/:rid ─────────────────────────────────────────────

func deleteRouteHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		app, ok := getApp(c, db)
		if !ok {
			return
		}
		rid, err := strconv.Atoi(c.Param("rid"))
		if err != nil {
			resp.BadRequest(c, "路由 ID 无效")
			return
		}
		if err := db.Where("id = ? AND app_id = ?", rid, app.ID).Delete(&model.AppNginxRoute{}).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		if err := removeIngressRouteByLegacy(db, uint(rid)); err != nil {
			fmt.Printf("removeIngressRouteByLegacy(app=%d route=%d): %v\n", app.ID, rid, err)
		}
		resp.OK(c, nil)
	}
}

// ── POST /:id/nginx/apply ─────────────────────────────────────────────────────

// applyHandler 兼容旧 NginxRoutes.vue 路径的 apply 入口。新链路下，
// 真正的 nginx 写盘 / reload / 回滚都由 nginxops.Reconciler 完成；
// 这里仅做参数校验与 edge 解析后转发。
func applyHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		app, ok := getApp(c, db)
		if !ok {
			return
		}
		if err := validateAppName(app.Name); err != nil {
			resp.BadRequest(c, "应用名包含不能用作文件名的字符: "+err.Error())
			return
		}
		var routes []model.AppNginxRoute
		db.Where("app_id = ?", app.ID).Order("sort asc, id asc").Find(&routes)
		// Re-validate persisted routes — DB rows from before this validation
		// landed could still contain dangerous values.
		for i := range routes {
			rq := routeReq{Path: routes[i].Path, Upstream: routes[i].Upstream, Extra: routes[i].Extra}
			if err := validateRoute(&rq); err != nil {
				resp.BadRequest(c, fmt.Sprintf("路由 #%d 非法: %s", routes[i].ID, err.Error()))
				return
			}
		}

		edge := app.RunServerID
		if edge == 0 {
			edge = app.ServerID
		}
		if edge == 0 {
			resp.BadRequest(c, "应用未绑定运行节点")
			return
		}

		var actor *uint
		if v, exists := c.Get("userID"); exists {
			if uid, okUID := v.(uint); okUID && uid > 0 {
				actor = &uid
			}
		}

		res, err := nginxops.Apply(context.Background(), db, cfg, edge, actor)
		if err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, gin.H{
			"output":      strings.TrimSpace(res.Output),
			"changes":     res.Changes,
			"no_op":       res.NoOp,
			"rolled_back": res.RolledBack,
			"audit_id":    res.AuditID,
		})
	}
}

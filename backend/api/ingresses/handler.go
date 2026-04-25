// Package ingresses 提供 P1 新链路的 Ingress 编排 API：CRUD、apply、dry-run、
// audit 历史与下拉数据源。前端旧 approutes API 仍保留（其 apply 已桥接到本包
// 同源的 Reconciler），P3 完整下线。
package ingresses

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/middleware"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/nginxops"
	"github.com/serverhub/serverhub/pkg/resp"
)

// RegisterRoutes 把所有 Ingress 相关 API 挂到 group 下。
//
// 资源型路由（CRUD + 路由子资源）使用 :id；apply / dry-run / audit / services
// 是按 edge_server_id / server_id 维度的操作端点，挂在 /edges 与 /services 子组。
func RegisterRoutes(group *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	group.GET("", listHandler(db))
	group.POST("", createHandler(db))
	group.GET(":id", getHandler(db))
	group.PUT(":id", updateHandler(db))
	group.DELETE(":id", deleteHandler(db))
	group.POST(":id/routes", addRouteHandler(db))
	group.PUT(":id/routes/:rid", updateRouteHandler(db))
	group.DELETE(":id/routes/:rid", deleteRouteHandler(db))

	group.POST("edges/:server_id/apply", applyHandler(db, cfg))
	group.POST("edges/:server_id/dry-run", dryRunHandler(db, cfg))
	group.GET("edges/:server_id/audit", auditHandler(db))

	group.GET("services/:server_id", servicesHandler(db))
}

// ── DTO ───────────────────────────────────────────────────────────────────

type ingressDTO struct {
	model.Ingress
	Routes []model.IngressRoute `json:"routes,omitempty"`
}

type createReq struct {
	EdgeServerID uint                  `json:"edge_server_id" binding:"required"`
	MatchKind    string                `json:"match_kind" binding:"required"`
	Domain       string                `json:"domain" binding:"required"`
	DefaultPath  string                `json:"default_path"`
	Routes       []routeReq            `json:"routes"`
}

type updateReq struct {
	MatchKind   string `json:"match_kind"`
	Domain      string `json:"domain"`
	DefaultPath string `json:"default_path"`
}

type routeReq struct {
	Sort      int                    `json:"sort"`
	Path      string                 `json:"path" binding:"required"`
	Protocol  string                 `json:"protocol"`
	Upstream  model.IngressUpstream  `json:"upstream"`
	WebSocket bool                   `json:"websocket"`
	Extra     string                 `json:"extra"`
}

// ── helpers ───────────────────────────────────────────────────────────────

func parseUintParam(c *gin.Context, name string) (uint, bool) {
	v, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil || v == 0 {
		resp.BadRequest(c, name+" 无效")
		return 0, false
	}
	return uint(v), true
}

func validateMatchKind(k string) error {
	if k != "domain" && k != "path" {
		return errors.New("match_kind 必须是 domain 或 path")
	}
	return nil
}

// validateProtocol 限制 RouteCtx.Protocol 的取值。
//
// 当前 Renderer 支持的协议：
//   - http / "" / ws：proxy_pass 链路（ws 等价 http + WebSocket=true）
//   - grpc：grpc_pass + http2 listen
//
// tcp / udp 需要 nginx stream 块，留到 P2-D3。提前在 API 层挡住可避免用户
// 在前端选了 tcp/udp 之后 apply 时才被 nginx -t 拒绝。
func validateProtocol(p string) error {
	switch p {
	case "", "http", "ws", "grpc":
		return nil
	case "tcp", "udp":
		return errors.New("protocol=" + p + " 暂未支持（stream 段渲染计划中）")
	default:
		return errors.New("protocol 取值非法：" + p)
	}
}

func loadIngress(db *gorm.DB, id uint) (*model.Ingress, error) {
	var ig model.Ingress
	if err := db.First(&ig, id).Error; err != nil {
		return nil, err
	}
	return &ig, nil
}

// ── handlers ──────────────────────────────────────────────────────────────

func listHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		q := db.Model(&model.Ingress{})
		if v := c.Query("edge_server_id"); v != "" {
			id, err := strconv.Atoi(v)
			if err != nil || id <= 0 {
				resp.BadRequest(c, "edge_server_id 无效")
				return
			}
			q = q.Where("edge_server_id = ?", id)
		}
		var rows []model.Ingress
		if err := q.Order("id").Find(&rows).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		if rows == nil {
			rows = []model.Ingress{}
		}
		resp.OK(c, rows)
	}
}

func getHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseUintParam(c, "id")
		if !ok {
			return
		}
		ig, err := loadIngress(db, id)
		if err != nil {
			resp.NotFound(c, "ingress 不存在")
			return
		}
		var routes []model.IngressRoute
		db.Where("ingress_id = ?", id).Order("sort, id").Find(&routes)
		resp.OK(c, ingressDTO{Ingress: *ig, Routes: routes})
	}
}

func createHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		if err := validateMatchKind(req.MatchKind); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		// 强一致：同 edge 同 domain 必须 MatchKind 一致
		var existing model.Ingress
		err := db.Where("edge_server_id = ? AND domain = ?", req.EdgeServerID, req.Domain).First(&existing).Error
		if err == nil && existing.MatchKind != req.MatchKind {
			resp.BadRequest(c, "同一 edge+domain 下 match_kind 不允许混用，已存在 "+existing.MatchKind)
			return
		}

		ig := model.Ingress{
			EdgeServerID: req.EdgeServerID,
			MatchKind:    req.MatchKind,
			Domain:       req.Domain,
			DefaultPath:  req.DefaultPath,
			Status:       "pending",
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&ig).Error; err != nil {
				return err
			}
			for _, r := range req.Routes {
				if err := validateProtocol(r.Protocol); err != nil {
					return err
				}
				ir := model.IngressRoute{
					IngressID: ig.ID, Sort: r.Sort, Path: r.Path,
					Protocol: r.Protocol, Upstream: r.Upstream,
					WebSocket: r.WebSocket, Extra: r.Extra,
				}
				if ir.Protocol == "" {
					ir.Protocol = "http"
				}
				if err := tx.Create(&ir).Error; err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		resp.OK(c, ig)
	}
}

func updateHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseUintParam(c, "id")
		if !ok {
			return
		}
		var req updateReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		ig, err := loadIngress(db, id)
		if err != nil {
			resp.NotFound(c, "ingress 不存在")
			return
		}
		updates := map[string]any{}
		if req.MatchKind != "" {
			if err := validateMatchKind(req.MatchKind); err != nil {
				resp.BadRequest(c, err.Error())
				return
			}
			updates["match_kind"] = req.MatchKind
		}
		if req.Domain != "" {
			updates["domain"] = req.Domain
		}
		if req.DefaultPath != "" {
			updates["default_path"] = req.DefaultPath
		}
		if len(updates) == 0 {
			resp.OK(c, ig)
			return
		}
		updates["status"] = "pending"
		if err := db.Model(ig).Updates(updates).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		_ = db.First(ig, id).Error
		resp.OK(c, ig)
	}
}

func deleteHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseUintParam(c, "id")
		if !ok {
			return
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("ingress_id = ?", id).Delete(&model.IngressRoute{}).Error; err != nil {
				return err
			}
			return tx.Delete(&model.Ingress{}, id).Error
		}); err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, nil)
	}
}

func addRouteHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		igID, ok := parseUintParam(c, "id")
		if !ok {
			return
		}
		if _, err := loadIngress(db, igID); err != nil {
			resp.NotFound(c, "ingress 不存在")
			return
		}
		var req routeReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		if err := validateProtocol(req.Protocol); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		ir := model.IngressRoute{
			IngressID: igID, Sort: req.Sort, Path: req.Path,
			Protocol: req.Protocol, Upstream: req.Upstream,
			WebSocket: req.WebSocket, Extra: req.Extra,
		}
		if ir.Protocol == "" {
			ir.Protocol = "http"
		}
		if err := db.Create(&ir).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		_ = db.Model(&model.Ingress{}).Where("id = ?", igID).Update("status", "pending").Error
		resp.OK(c, ir)
	}
}

func updateRouteHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		igID, ok := parseUintParam(c, "id")
		if !ok {
			return
		}
		rid, ok := parseUintParam(c, "rid")
		if !ok {
			return
		}
		var ir model.IngressRoute
		if err := db.Where("id = ? AND ingress_id = ?", rid, igID).First(&ir).Error; err != nil {
			resp.NotFound(c, "route 不存在")
			return
		}
		var req routeReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		if err := validateProtocol(req.Protocol); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		ir.Sort = req.Sort
		ir.Path = req.Path
		if req.Protocol != "" {
			ir.Protocol = req.Protocol
		}
		ir.Upstream = req.Upstream
		ir.WebSocket = req.WebSocket
		ir.Extra = req.Extra
		if err := db.Save(&ir).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		_ = db.Model(&model.Ingress{}).Where("id = ?", igID).Update("status", "pending").Error
		resp.OK(c, ir)
	}
}

func deleteRouteHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		igID, ok := parseUintParam(c, "id")
		if !ok {
			return
		}
		rid, ok := parseUintParam(c, "rid")
		if !ok {
			return
		}
		if err := db.Where("id = ? AND ingress_id = ?", rid, igID).Delete(&model.IngressRoute{}).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		_ = db.Model(&model.Ingress{}).Where("id = ?", igID).Update("status", "pending").Error
		resp.OK(c, nil)
	}
}

// ── apply / dry-run / audit ───────────────────────────────────────────────

func currentUserID(c *gin.Context) *uint {
	if v, exists := c.Get("userID"); exists {
		if uid, ok := v.(uint); ok && uid > 0 {
			return &uid
		}
	}
	return nil
}

func applyHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		edgeID, ok := parseUintParam(c, "server_id")
		if !ok {
			return
		}
		actor := currentUserID(c)
		_ = middleware.GetClaims(c) // 触发 claim 解析（部分中间件依赖）
		res, err := nginxops.Apply(context.Background(), db, cfg, edgeID, actor)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    5000,
				"message": err.Error(),
				"data":    res,
			})
			return
		}
		resp.OK(c, res)
	}
}

func dryRunHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		edgeID, ok := parseUintParam(c, "server_id")
		if !ok {
			return
		}
		changes, err := nginxops.DryRun(context.Background(), db, cfg, edgeID)
		if err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		if changes == nil {
			changes = []nginxops.Change{}
		}
		resp.OK(c, gin.H{"changes": changes})
	}
}

func auditHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		edgeID, ok := parseUintParam(c, "server_id")
		if !ok {
			return
		}
		var rows []model.AuditApply
		limit := 50
		if v := c.Query("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
				limit = n
			}
		}
		if err := db.Where("edge_server_id = ?", edgeID).
			Order("id DESC").Limit(limit).Find(&rows).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		if rows == nil {
			rows = []model.AuditApply{}
		}
		resp.OK(c, rows)
	}
}

// ── upstream 下拉 ──────────────────────────────────────────────────────────

type serviceOpt struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	ExposedPort int    `json:"exposed_port"`
}

func servicesHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		serverID, ok := parseUintParam(c, "server_id")
		if !ok {
			return
		}
		var rows []model.Service
		if err := db.Where("server_id = ?", serverID).Order("name").Find(&rows).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		out := make([]serviceOpt, 0, len(rows))
		for _, s := range rows {
			out = append(out, serviceOpt{ID: s.ID, Name: s.Name, ExposedPort: s.ExposedPort})
		}
		resp.OK(c, out)
	}
}

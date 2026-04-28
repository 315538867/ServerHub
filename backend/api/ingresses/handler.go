// Package ingresses 提供新链路的 Ingress 编排 API：CRUD、apply、dry-run、
// audit 历史与下拉数据源。旧 approutes API 与 AppNginxRoute 表已在 P3 完整下线,
// 历史数据由一次性 m4 迁移搬到 Ingress/IngressRoute,此后所有 nginx 编排走本包。
package ingresses

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/middleware"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/repo"
	"github.com/serverhub/serverhub/usecase"
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

	// Phase Nginx-P3B: discovery → Ingress 反代接管候选
	RegisterImportRoutes(group, db, cfg)
	// Phase Nginx-P3C: ratelimit/cache/security 预设模板 → Extra 文本
	RegisterPresetRoutes(group, db)
}

// ── DTO ───────────────────────────────────────────────────────────────────

type ingressDTO struct {
	model.Ingress
	Routes []model.IngressRoute `json:"routes,omitempty"`
}

type createReq struct {
	EdgeServerID uint       `json:"edge_server_id" binding:"required"`
	MatchKind    string     `json:"match_kind" binding:"required"`
	Domain       string     `json:"domain" binding:"required"`
	DefaultPath  string     `json:"default_path"`
	CertID       *uint      `json:"cert_id"`
	ForceHTTPS   bool       `json:"force_https"`
	Routes       []routeReq `json:"routes"`
}

type updateReq struct {
	MatchKind string `json:"match_kind"`
	Domain    string `json:"domain"`
	// CertID 用 json.RawMessage 实现真三态：
	//   nil           → 字段未传(保持现值)
	//   []byte("null")→ 显式清空
	//   "<uint>"      → 替换
	// 注：Go stdlib JSON 的 **uint 把 "字段缺失" 与 "null" 都解成 nil,无法区分,
	// 因此这里改用 RawMessage,在 handler 里手动二次解。
	CertID      json.RawMessage `json:"cert_id,omitempty"`
	DefaultPath string          `json:"default_path"`
	// ForceHTTPS 同样需要"未传/传 false/传 true"三态,用 *bool。
	ForceHTTPS *bool `json:"force_https,omitempty"`
}

type routeReq struct {
	Sort       int                   `json:"sort"`
	Path       string                `json:"path" binding:"required"`
	Protocol   string                `json:"protocol"`
	Upstream   model.IngressUpstream `json:"upstream"`
	WebSocket  bool                  `json:"websocket"`
	Extra      string                `json:"extra"`
	ListenPort *int                  `json:"listen_port,omitempty"`
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

// toRouteParams 把 handler DTO 映射到 usecase 入参。
func toRouteParams(r routeReq) usecase.RouteParams {
	return usecase.RouteParams{
		Sort: r.Sort, Path: r.Path, Protocol: r.Protocol,
		Upstream: r.Upstream, WebSocket: r.WebSocket,
		Extra: r.Extra, ListenPort: r.ListenPort,
	}
}

// ── handlers ──────────────────────────────────────────────────────────────

func listHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var edgeID *uint
		if v := c.Query("edge_server_id"); v != "" {
			id, err := strconv.Atoi(v)
			if err != nil || id <= 0 {
				resp.BadRequest(c, "edge_server_id 无效")
				return
			}
			uid := uint(id)
			edgeID = &uid
		}
		rows, err := usecase.ListIngresses(c.Request.Context(), db, edgeID)
		if err != nil {
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
		ig, routes, err := usecase.GetIngressWithRoutes(c.Request.Context(), db, id)
		if err != nil {
			resp.NotFound(c, "ingress 不存在")
			return
		}
		resp.OK(c, ingressDTO{Ingress: ig, Routes: routes})
	}
}

func createHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		routes := make([]usecase.RouteParams, 0, len(req.Routes))
		for _, r := range req.Routes {
			routes = append(routes, toRouteParams(r))
		}
		ig, err := usecase.CreateIngress(c.Request.Context(), db, usecase.CreateIngressParams{
			EdgeServerID: req.EdgeServerID,
			MatchKind:    req.MatchKind,
			Domain:       req.Domain,
			DefaultPath:  req.DefaultPath,
			CertID:       req.CertID,
			ForceHTTPS:   req.ForceHTTPS,
			Routes:       routes,
		})
		if err != nil {
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
		// 加载当前 ingress 以计算合并后的 next 值
		ig, _, err := usecase.GetIngressWithRoutes(c.Request.Context(), db, id)
		if err != nil {
			resp.NotFound(c, "ingress 不存在")
			return
		}
		updates := map[string]any{}
		nextMatchKind := ig.MatchKind
		nextCertID := ig.CertID
		nextForceHTTPS := ig.ForceHTTPS

		if req.MatchKind != "" {
			updates["match_kind"] = req.MatchKind
			nextMatchKind = req.MatchKind
		}
		if req.Domain != "" {
			updates["domain"] = req.Domain
		}
		if req.DefaultPath != "" {
			updates["default_path"] = req.DefaultPath
		}
		if len(req.CertID) > 0 {
			if bytes.Equal(bytes.TrimSpace(req.CertID), []byte("null")) {
				updates["cert_id"] = nil
				nextCertID = nil
			} else {
				var v uint
				if err := json.Unmarshal(req.CertID, &v); err != nil {
					resp.BadRequest(c, "cert_id 必须是非负整数或 null")
					return
				}
				updates["cert_id"] = v
				nextCertID = &v
			}
		}
		if req.ForceHTTPS != nil {
			updates["force_https"] = *req.ForceHTTPS
			nextForceHTTPS = *req.ForceHTTPS
		}

		result, err := usecase.UpdateIngress(c.Request.Context(), db, id, usecase.UpdateIngressParams{
			Updates:        updates,
			NextMatchKind:  nextMatchKind,
			NextCertID:     nextCertID,
			NextForceHTTPS: nextForceHTTPS,
		})
		if err != nil {
			if repo.IsNotFound(err) {
				resp.NotFound(c, "ingress 不存在")
			} else {
				resp.BadRequest(c, err.Error())
			}
			return
		}
		resp.OK(c, result)
	}
}

func deleteHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseUintParam(c, "id")
		if !ok {
			return
		}
		if err := usecase.DeleteIngress(c.Request.Context(), db, id); err != nil {
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
		var req routeReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		ir, err := usecase.AddIngressRoute(c.Request.Context(), db, igID, toRouteParams(req))
		if err != nil {
			if repo.IsNotFound(err) {
				resp.NotFound(c, "ingress 不存在")
			} else {
				resp.BadRequest(c, err.Error())
			}
			return
		}
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
		var req routeReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		ir, err := usecase.UpdateIngressRoute(c.Request.Context(), db, igID, rid, toRouteParams(req))
		if err != nil {
			if repo.IsNotFound(err) {
				resp.NotFound(c, "route 不存在")
			} else {
				resp.BadRequest(c, err.Error())
			}
			return
		}
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
		if err := usecase.DeleteIngressRoute(c.Request.Context(), db, igID, rid); err != nil {
			resp.InternalError(c, err.Error())
			return
		}
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
		res, err := usecase.ApplyIngress(context.Background(), db, cfg, edgeID, actor)
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
		changes, err := usecase.DryRunIngress(context.Background(), db, cfg, edgeID)
		if err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		if changes == nil {
			changes = []usecase.IngressChange{}
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
		limit := 50
		if v := c.Query("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
				limit = n
			}
		}
		out, err := usecase.ListAuditWithActors(c.Request.Context(), db, edgeID, limit)
		if err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, out)
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
		rows, err := usecase.ListUpstreamServices(c.Request.Context(), db, serverID)
		if err != nil {
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

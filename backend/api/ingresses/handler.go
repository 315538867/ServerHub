// Package ingresses 提供新链路的 Ingress 编排 API：CRUD、apply、dry-run、
// audit 历史与下拉数据源。旧 approutes API 与 AppNginxRoute 表已在 P3 完整下线,
// 历史数据由一次性 m4 迁移搬到 Ingress/IngressRoute,此后所有 nginx 编排走本包。
package ingresses

import (
	"bytes"
	"context"
	"encoding/json"
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
	CertID       *uint                 `json:"cert_id"`
	ForceHTTPS   bool                  `json:"force_https"`
	Routes       []routeReq            `json:"routes"`
}

type updateReq struct {
	MatchKind    string `json:"match_kind"`
	Domain       string `json:"domain"`
	DefaultPath  string `json:"default_path"`
	// CertID 用 json.RawMessage 实现真三态：
	//   nil           → 字段未传(保持现值)
	//   []byte("null")→ 显式清空
	//   "<uint>"      → 替换
	// 注：Go stdlib JSON 的 **uint 把 "字段缺失" 与 "null" 都解成 nil,无法区分,
	// 因此这里改用 RawMessage,在 handler 里手动二次解。
	CertID       json.RawMessage `json:"cert_id,omitempty"`
	// ForceHTTPS 同样需要"未传/传 false/传 true"三态,用 *bool。
	ForceHTTPS   *bool  `json:"force_https,omitempty"`
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
//   - tcp / udp：聚合到 streams.conf 的 stream{} 顶层块；必须配 listen_port
//
// listenPort 仅在 tcp/udp 协议下被检查（>0 必填）；其它协议为 nil 即可。
func validateProtocol(p string, listenPort *int) error {
	switch p {
	case "", "http", "ws", "grpc":
		return nil
	case "tcp", "udp":
		if listenPort == nil || *listenPort <= 0 {
			return errors.New("protocol=" + p + " 需要 listen_port>0")
		}
		if *listenPort > 65535 {
			return errors.New("listen_port 超出范围")
		}
		return nil
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

// validateTLS 校验 cert_id / force_https / match_kind 的组合一致性。
//   - matchKind=path 时不允许带 CertID（共享 hub 站点暂不支持 per-ingress 证书）
//   - certID==nil 但 forceHTTPS=true → 拒（强制跳转必须有目的端证书）
//   - certID!=nil 时：cert 必须存在 && cert.ServerID == edgeServerID
func validateTLS(db *gorm.DB, edgeServerID uint, matchKind string, certID *uint, forceHTTPS bool) error {
	if certID != nil && matchKind == "path" {
		return errors.New("path 模式暂不支持 TLS（cert_id 必须为空）")
	}
	if certID == nil && forceHTTPS {
		return errors.New("force_https=true 需要先指定 cert_id")
	}
	if certID == nil {
		return nil
	}
	var cert model.SSLCert
	if err := db.First(&cert, *certID).Error; err != nil {
		return errors.New("cert_id 引用的证书不存在")
	}
	if cert.ServerID != edgeServerID {
		return errors.New("cert 不属于该 edge_server")
	}
	return nil
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
		if err := validateTLS(db, req.EdgeServerID, req.MatchKind, req.CertID, req.ForceHTTPS); err != nil {
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
			CertID:       req.CertID,
			ForceHTTPS:   req.ForceHTTPS,
			Status:       "pending",
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&ig).Error; err != nil {
				return err
			}
			for _, r := range req.Routes {
				if err := validateProtocol(r.Protocol, r.ListenPort); err != nil {
					return err
				}
				ir := model.IngressRoute{
					IngressID: ig.ID, Sort: r.Sort, Path: r.Path,
					Protocol: r.Protocol, Upstream: r.Upstream,
					WebSocket: r.WebSocket, Extra: r.Extra,
					ListenPort: r.ListenPort,
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
		// 先把 match_kind/cert_id/force_https 的最终值算出来再走 validateTLS
		nextMatchKind := ig.MatchKind
		nextCertID := ig.CertID
		nextForceHTTPS := ig.ForceHTTPS
		if req.MatchKind != "" {
			if err := validateMatchKind(req.MatchKind); err != nil {
				resp.BadRequest(c, err.Error())
				return
			}
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
			// RawMessage 三态：长度为 0 = 字段未传；"null" = 清空；其它 = 解 uint
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
		if err := validateTLS(db, ig.EdgeServerID, nextMatchKind, nextCertID, nextForceHTTPS); err != nil {
			resp.BadRequest(c, err.Error())
			return
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
		if err := validateProtocol(req.Protocol, req.ListenPort); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		ir := model.IngressRoute{
			IngressID: igID, Sort: req.Sort, Path: req.Path,
			Protocol: req.Protocol, Upstream: req.Upstream,
			WebSocket: req.WebSocket, Extra: req.Extra,
			ListenPort: req.ListenPort,
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
		if err := validateProtocol(req.Protocol, req.ListenPort); err != nil {
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
		ir.ListenPort = req.ListenPort
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

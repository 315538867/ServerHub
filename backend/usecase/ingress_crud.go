// Package usecase: ingress_crud.go 封装 Ingress CRUD、Route 子资源操作、
// audit 查询与 services 下拉等业务逻辑。
//
// handler 只负责 DTO 解析 / 鉴权 / 调 usecase / 回响应四件事,
// 所有涉 DB 的校验与写入全部经 repo/ 层完成。
//
// TODO R7: 切 ports.IngressRepo interface,移除 db *gorm.DB 入参。
package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/safeshell"
	"github.com/serverhub/serverhub/repo"
	"gorm.io/gorm"
)

// ── 入参结构 ────────────────────────────────────────────────────────────────

// CreateIngressParams 是 usecase 层的"创建 Ingress"入参。
type CreateIngressParams struct {
	EdgeServerID uint
	MatchKind    string
	Domain       string
	DefaultPath  string
	CertID       *uint
	ForceHTTPS   bool
	Routes       []RouteParams
}

// RouteParams 是 usecase 层的路由入参,handler 把 routeReq 映射到此。
type RouteParams struct {
	Sort       int
	Path       string
	Protocol   string
	Upstream   model.IngressUpstream
	WebSocket  bool
	Extra      string
	ListenPort *int
}

// UpdateIngressParams 是 usecase 层的"更新 Ingress"入参。
// handler 完成 JSON 三态解析后,把最终值传入。
type UpdateIngressParams struct {
	Updates        map[string]any // handler 已组装好的字段 map
	NextMatchKind  string         // 合并后的最终值(用于 TLS 校验)
	NextCertID     *uint
	NextForceHTTPS bool
}

// AuditApplyWithActor 在 AuditApply 基础上拼 username。
type AuditApplyWithActor struct {
	model.AuditApply
	ActorUsername string `json:"actor_username"`
}

// ── 纯校验(不访问 DB) ──────────────────────────────────────────────────────

func validateMatchKind(k string) error {
	if k != "domain" && k != "path" {
		return errors.New("match_kind 必须是 domain 或 path")
	}
	return nil
}

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

func validateExtra(extra string) error {
	return safeshell.NginxBlock(extra)
}

// ── DB 校验 ─────────────────────────────────────────────────────────────────

// validateTLS 校验 cert_id / force_https / match_kind 的组合一致性。
func validateTLS(ctx context.Context, db *gorm.DB, edgeServerID uint, matchKind string, certID *uint, forceHTTPS bool) error {
	if certID != nil && matchKind == "path" {
		return errors.New("path 模式暂不支持 TLS（cert_id 必须为空）")
	}
	if certID == nil && forceHTTPS {
		return errors.New("force_https=true 需要先指定 cert_id")
	}
	if certID == nil {
		return nil
	}
	cert, err := repo.GetCertByID(ctx, db, *certID)
	if err != nil {
		return errors.New("cert_id 引用的证书不存在")
	}
	if cert.ServerID != edgeServerID {
		return errors.New("cert 不属于该 edge_server")
	}
	return nil
}

// validateRouteUniqueness 预检同 ingress path 重复 + 同 edge stream port 冲突。
func validateRouteUniqueness(
	ctx context.Context, db *gorm.DB, ingressID, edgeServerID uint,
	path, protocol string, listenPort *int, excludeRouteID uint,
) error {
	if path != "" {
		cnt, err := repo.CountRouteConflicts(ctx, db, ingressID, path, excludeRouteID)
		if err != nil {
			return err
		}
		if cnt > 0 {
			return errors.New("同 ingress 下已存在 path=" + path + " 的路由")
		}
	}
	if (protocol == "tcp" || protocol == "udp") && listenPort != nil && *listenPort > 0 {
		cnt, err := repo.CountStreamPortConflicts(ctx, db, edgeServerID, *listenPort, excludeRouteID)
		if err != nil {
			return err
		}
		if cnt > 0 {
			return fmt.Errorf("同 edge 下已存在 listen_port=%d 的 tcp/udp 路由", *listenPort)
		}
	}
	return nil
}

// ── CRUD ────────────────────────────────────────────────────────────────────

// ListIngresses 列出 ingress,可按 edgeServerID 过滤。
func ListIngresses(ctx context.Context, db *gorm.DB, edgeServerID *uint) ([]model.Ingress, error) {
	return repo.ListIngresses(ctx, db, edgeServerID)
}

// GetIngressWithRoutes 返回单条 ingress 及其路由列表。
func GetIngressWithRoutes(ctx context.Context, db *gorm.DB, id uint) (model.Ingress, []model.IngressRoute, error) {
	ig, err := repo.GetIngressByID(ctx, db, id)
	if err != nil {
		return model.Ingress{}, nil, err
	}
	routes, err := repo.ListRoutesByIngressID(ctx, db, id)
	if err != nil {
		return model.Ingress{}, nil, err
	}
	return ig, routes, nil
}

// CreateIngress 校验 + 事务创建 Ingress 及其路由。
func CreateIngress(ctx context.Context, db *gorm.DB, p CreateIngressParams) (model.Ingress, error) {
	if err := validateMatchKind(p.MatchKind); err != nil {
		return model.Ingress{}, err
	}
	if err := validateTLS(ctx, db, p.EdgeServerID, p.MatchKind, p.CertID, p.ForceHTTPS); err != nil {
		return model.Ingress{}, err
	}
	// 同 edge 同 domain 的 match_kind 一致性
	existing, err := repo.FindIngressByEdgeAndDomain(ctx, db, p.EdgeServerID, p.Domain)
	if err == nil && existing.MatchKind != p.MatchKind {
		return model.Ingress{}, errors.New("同一 edge+domain 下 match_kind 不允许混用，已存在 " + existing.MatchKind)
	}

	ig := model.Ingress{
		EdgeServerID: p.EdgeServerID,
		MatchKind:    p.MatchKind,
		Domain:       p.Domain,
		DefaultPath:  p.DefaultPath,
		CertID:       p.CertID,
		ForceHTTPS:   p.ForceHTTPS,
		Status:       "pending",
	}

	// 需要在事务内逐条校验路由唯一性(批内冲突也能前置拦截),
	// 不能直接用 repo.CreateIngressWithRoutes。
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := repo.CreateIngress(ctx, tx, &ig); err != nil {
			return err
		}
		for _, r := range p.Routes {
			if err := validateProtocol(r.Protocol, r.ListenPort); err != nil {
				return err
			}
			if err := validateExtra(r.Extra); err != nil {
				return err
			}
			proto := r.Protocol
			if proto == "" {
				proto = "http"
			}
			if err := validateRouteUniqueness(ctx, tx, ig.ID, ig.EdgeServerID,
				r.Path, proto, r.ListenPort, 0); err != nil {
				return err
			}
			ir := model.IngressRoute{
				IngressID: ig.ID, Sort: r.Sort, Path: r.Path,
				Protocol: proto, Upstream: r.Upstream,
				WebSocket: r.WebSocket, Extra: r.Extra,
				ListenPort: r.ListenPort,
			}
			if err := repo.CreateRoute(ctx, tx, &ir); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return model.Ingress{}, err
	}
	return ig, nil
}

// UpdateIngress 校验 TLS + 更新字段 + 返回刷新后的 ingress。
// handler 负责 DTO 三态解析和 updates map 组装,usecase 负责 TLS 校验与 DB 写入。
func UpdateIngress(ctx context.Context, db *gorm.DB, id uint, p UpdateIngressParams) (model.Ingress, error) {
	ig, err := repo.GetIngressByID(ctx, db, id)
	if err != nil {
		return model.Ingress{}, err
	}
	if err := validateTLS(ctx, db, ig.EdgeServerID, p.NextMatchKind, p.NextCertID, p.NextForceHTTPS); err != nil {
		return model.Ingress{}, err
	}
	if len(p.Updates) == 0 {
		return ig, nil
	}
	p.Updates["status"] = "pending"
	if err := repo.UpdateIngressFields(ctx, db, id, p.Updates); err != nil {
		return model.Ingress{}, err
	}
	ig, _ = repo.GetIngressByID(ctx, db, id)
	return ig, nil
}

// DeleteIngress 级联删除 ingress 及其路由。
func DeleteIngress(ctx context.Context, db *gorm.DB, id uint) error {
	return repo.DeleteIngressCascade(ctx, db, id)
}

// ── Route 子资源 ────────────────────────────────────────────────────────────

// AddIngressRoute 校验 + 创建路由 + 标记 ingress pending。
func AddIngressRoute(ctx context.Context, db *gorm.DB, ingressID uint, p RouteParams) (model.IngressRoute, error) {
	ig, err := repo.GetIngressByID(ctx, db, ingressID)
	if err != nil {
		return model.IngressRoute{}, err
	}
	if err := validateProtocol(p.Protocol, p.ListenPort); err != nil {
		return model.IngressRoute{}, err
	}
	if err := validateExtra(p.Extra); err != nil {
		return model.IngressRoute{}, err
	}
	proto := p.Protocol
	if proto == "" {
		proto = "http"
	}
	if err := validateRouteUniqueness(ctx, db, ingressID, ig.EdgeServerID,
		p.Path, proto, p.ListenPort, 0); err != nil {
		return model.IngressRoute{}, err
	}
	ir := model.IngressRoute{
		IngressID: ingressID, Sort: p.Sort, Path: p.Path,
		Protocol: proto, Upstream: p.Upstream,
		WebSocket: p.WebSocket, Extra: p.Extra,
		ListenPort: p.ListenPort,
	}
	if err := repo.CreateRoute(ctx, db, &ir); err != nil {
		return model.IngressRoute{}, err
	}
	_ = repo.MarkIngressPending(ctx, db, ingressID)
	return ir, nil
}

// UpdateIngressRoute 校验 + 保存路由 + 标记 ingress pending。
func UpdateIngressRoute(ctx context.Context, db *gorm.DB, ingressID, routeID uint, p RouteParams) (model.IngressRoute, error) {
	ir, err := repo.GetRouteByID(ctx, db, ingressID, routeID)
	if err != nil {
		return model.IngressRoute{}, err
	}
	if err := validateProtocol(p.Protocol, p.ListenPort); err != nil {
		return model.IngressRoute{}, err
	}
	if err := validateExtra(p.Extra); err != nil {
		return model.IngressRoute{}, err
	}
	ig, err := repo.GetIngressByID(ctx, db, ingressID)
	if err != nil {
		return model.IngressRoute{}, err
	}
	proto := p.Protocol
	if proto == "" {
		proto = ir.Protocol
	}
	if err := validateRouteUniqueness(ctx, db, ingressID, ig.EdgeServerID,
		p.Path, proto, p.ListenPort, ir.ID); err != nil {
		return model.IngressRoute{}, err
	}
	ir.Sort = p.Sort
	ir.Path = p.Path
	if p.Protocol != "" {
		ir.Protocol = p.Protocol
	}
	ir.Upstream = p.Upstream
	ir.WebSocket = p.WebSocket
	ir.Extra = p.Extra
	ir.ListenPort = p.ListenPort
	if err := repo.SaveRoute(ctx, db, &ir); err != nil {
		return model.IngressRoute{}, err
	}
	_ = repo.MarkIngressPending(ctx, db, ingressID)
	return ir, nil
}

// DeleteIngressRoute 删除路由 + 标记 ingress pending。
func DeleteIngressRoute(ctx context.Context, db *gorm.DB, ingressID, routeID uint) error {
	if err := repo.DeleteRoute(ctx, db, ingressID, routeID); err != nil {
		return err
	}
	_ = repo.MarkIngressPending(ctx, db, ingressID)
	return nil
}

// ── 派生查询 ────────────────────────────────────────────────────────────────

// ListAuditWithActors 列出 apply 审计记录,并批量拼接 actor username。
func ListAuditWithActors(ctx context.Context, db *gorm.DB, edgeID uint, limit int) ([]AuditApplyWithActor, error) {
	rows, err := repo.ListAuditAppliesByEdge(ctx, db, edgeID, limit)
	if err != nil {
		return nil, err
	}
	// 收集需要查的 user ID
	userIDSet := map[uint]struct{}{}
	for _, r := range rows {
		if r.ActorUserID != nil {
			userIDSet[*r.ActorUserID] = struct{}{}
		}
	}
	nameByID := map[uint]string{}
	if len(userIDSet) > 0 {
		ids := make([]uint, 0, len(userIDSet))
		for id := range userIDSet {
			ids = append(ids, id)
		}
		users, err := repo.ListUsersByIDs(ctx, db, ids)
		if err == nil {
			for _, u := range users {
				nameByID[u.ID] = u.Username
			}
		}
	}
	out := make([]AuditApplyWithActor, 0, len(rows))
	for _, r := range rows {
		name := ""
		if r.ActorUserID != nil {
			name = nameByID[*r.ActorUserID]
		}
		out = append(out, AuditApplyWithActor{AuditApply: r, ActorUsername: name})
	}
	return out, nil
}

// ListUpstreamServices 列出指定 server 下的 service 列表(ingress upstream 下拉用)。
func ListUpstreamServices(ctx context.Context, db *gorm.DB, serverID uint) ([]model.Service, error) {
	return repo.ListServicesByServerID(ctx, db, serverID)
}

// ── import 相关(供 import_handler 使用) ─────────────────────────────────────

// CountIngressByEdgeAndDomain 检查同 edge+domain 是否已有 Ingress。
func CountIngressByEdgeAndDomain(ctx context.Context, db *gorm.DB, edgeID uint, domain string) (int64, error) {
	_, err := repo.FindIngressByEdgeAndDomain(ctx, db, edgeID, domain)
	if err != nil {
		if repo.IsNotFound(err) {
			return 0, nil
		}
		return 0, err
	}
	return 1, nil
}

// ImportCreateIngress 事务创建接管来源的 Ingress 及路由(不做路由唯一性校验,
// 因为接管来源的配置已在线上运行)。
func ImportCreateIngress(ctx context.Context, db *gorm.DB, ig *model.Ingress, routes []model.IngressRoute) error {
	return repo.CreateIngressWithRoutes(ctx, db, ig, routes)
}

// ImportDeleteIngress 级联删除接管来源的 Ingress(还原场景)。
func ImportDeleteIngress(ctx context.Context, db *gorm.DB, ingressID uint) error {
	return repo.DeleteIngressCascade(ctx, db, ingressID)
}

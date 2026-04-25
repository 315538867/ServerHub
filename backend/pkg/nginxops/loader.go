package nginxops

import (
	"fmt"
	"strconv"

	"gorm.io/gorm"

	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/netresolve"
	"github.com/serverhub/serverhub/pkg/nginxrender"
)

// LoadDesired 把 DB 里 edge 上的 Ingress + IngressRoute 加载并解析成
// nginxrender 可消费的 IngressCtx 列表。
//
// 步骤：
//   1. SELECT * FROM ingresses WHERE edge_server_id = edge.ID
//   2. 对每条 ingress：SELECT routes ORDER BY sort, id
//   3. 对每条 route：service 类型 → netresolve.Resolve；raw 类型 → 直接用
//   4. 拼装 FileStem（path 模式以 ingress.id 入名避免同 domain 冲突）
//
// 返回 nil error 即使无 ingress（空切片合法，触发"清空 edge"语义）。
func LoadDesired(db *gorm.DB, edge *model.Server) ([]nginxrender.IngressCtx, error) {
	var ingresses []model.Ingress
	if err := db.Where("edge_server_id = ?", edge.ID).Order("id").Find(&ingresses).Error; err != nil {
		return nil, fmt.Errorf("加载 ingress 失败: %w", err)
	}

	out := make([]nginxrender.IngressCtx, 0, len(ingresses))
	for _, ig := range ingresses {
		var routes []model.IngressRoute
		if err := db.Where("ingress_id = ?", ig.ID).
			Order("sort, id").Find(&routes).Error; err != nil {
			return nil, fmt.Errorf("加载 ingress_routes 失败 (ingress=%d): %w", ig.ID, err)
		}

		ctxRoutes := make([]nginxrender.RouteCtx, 0, len(routes))
		for _, rt := range routes {
			url, err := resolveUpstream(db, edge, &rt)
			if err != nil {
				return nil, fmt.Errorf("ingress=%d route=%d upstream 解析失败: %w", ig.ID, rt.ID, err)
			}
			ctxRoutes = append(ctxRoutes, nginxrender.RouteCtx{
				Sort:        rt.Sort,
				Path:        rt.Path,
				Protocol:    rt.Protocol,
				UpstreamURL: url,
				WebSocket:   rt.WebSocket,
				Extra:       rt.Extra,
			})
		}

		out = append(out, nginxrender.IngressCtx{
			EdgeServerID: ig.EdgeServerID,
			FileStem:     fileStem(ig),
			MatchKind:    ig.MatchKind,
			Domain:       ig.Domain,
			Routes:       ctxRoutes,
		})
	}
	return out, nil
}

// fileStem 给 ingress 生成稳定唯一的文件名干。
//   - domain 模式：sanitize(domain)，独占域名一一对应
//   - path 模式  ：sanitize(domain) + "-" + ingress.id；同 domain 多 path ingress 共享 hub 站点
func fileStem(ig model.Ingress) string {
	d := sanitizeStem(ig.Domain)
	if ig.MatchKind == nginxrender.MatchKindPath {
		return d + "-" + strconv.FormatUint(uint64(ig.ID), 10)
	}
	return d
}

// sanitizeStem 把 domain 转成文件名安全的形式。
//   - 空 / "_"  → "default"
//   - 把 "." → "_"，其它非 [A-Za-z0-9_-] → "-"
func sanitizeStem(s string) string {
	if s == "" || s == "_" {
		return "default"
	}
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_', c == '-':
			out = append(out, c)
		case c == '.':
			out = append(out, '_')
		default:
			out = append(out, '-')
		}
	}
	if len(out) == 0 {
		return "default"
	}
	return string(out)
}

// resolveUpstream 根据 IngressUpstream 类型计算最终 URL。
//   - service 类型：加载 Service → 加载其 Server → netresolve.Resolve
//   - raw 类型：直接返回 RawURL（前置已校验）
func resolveUpstream(db *gorm.DB, edge *model.Server, rt *model.IngressRoute) (string, error) {
	up := rt.Upstream
	switch up.Type {
	case "raw":
		if up.RawURL == "" {
			return "", fmt.Errorf("raw upstream 未填 raw_url")
		}
		return up.RawURL, nil

	case "service", "":
		if up.ServiceID == nil {
			return "", fmt.Errorf("service upstream 未填 service_id")
		}
		var svc model.Service
		if err := db.First(&svc, *up.ServiceID).Error; err != nil {
			return "", fmt.Errorf("加载 service id=%d: %w", *up.ServiceID, err)
		}
		port := svc.ExposedPort
		if up.OverridePort > 0 {
			port = up.OverridePort
		}
		if port == 0 {
			return "", fmt.Errorf("service id=%d 未声明 exposed_port，且 upstream 无 override_port", svc.ID)
		}
		var target model.Server
		if err := db.First(&target, svc.ServerID).Error; err != nil {
			return "", fmt.Errorf("加载 service.server id=%d: %w", svc.ServerID, err)
		}
		r, err := netresolve.Resolve(edge, &target, port, up.NetworkPref, up.OverrideHost, up.OverridePort)
		if err != nil {
			return "", err
		}
		return r.URL, nil

	default:
		return "", fmt.Errorf("未知 upstream.type=%q", up.Type)
	}
}

// Package usecase: ingress_discover.go 扫描指定 edge 上的 nginx 反代 vhost,返回
// 可被"接管"的候选列表。
//
// 与 R4 source.Default 注册表风格一致:走 ingress.Default.MustGet(kind).Discover,
// 对返回的 candidates 做两类 best-effort 注解:
//   - AlreadyManaged:同 edge 已有 Ingress 命中相同 server_name 时打上,前端置灰按钮
//   - CrossServer:proxy_pass 主机命中**另一台**已注册 Server 的 Host/Networks[].Address
//     时回填 (id, name),前端提示"建议接管后切 service upstream"
//
// 平移自 api/ingresses/import_handler.go::importCandidatesHandler + annotateCrossServer。
package usecase

import (
	"context"
	"strings"
	"time"

	nginxingress "github.com/serverhub/serverhub/adapters/ingress/nginx"
	"github.com/serverhub/serverhub/core/ingress"
	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/infra"
	"github.com/serverhub/serverhub/repo"
	"gorm.io/gorm"
)

// DiscoverIngressesResult 把候选列表与 best-effort 错误一并回吐。
type DiscoverIngressesResult struct {
	Candidates []ingress.IngressCandidate `json:"candidates"`
	Errors     []string                   `json:"errors,omitempty"`
}

// DiscoverIngresses 在指定 edge 上跑 ingress.Default["nginx"].Discover,并填注。
//
// kind 当前固定 "nginx"(R5 范围);未来扩展 traefik/caddy 时改成接 kind 参数。
func DiscoverIngresses(ctx context.Context, db *gorm.DB, r infra.Runner, edgeID uint) DiscoverIngressesResult {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	out := DiscoverIngressesResult{}
	be, gerr := ingress.Default.Get("nginx")
	if gerr != nil {
		out.Errors = append(out.Errors, gerr.Error())
		return out
	}
	cands, err := be.Discover(ctx, r)
	if err != nil {
		out.Errors = append(out.Errors, err.Error())
	}
	out.Candidates = cands
	if len(out.Candidates) == 0 {
		return out
	}
	annotateAlreadyManaged(cands, db, edgeID)
	annotateCrossServer(cands, db, edgeID)
	return out
}

// annotateAlreadyManaged 把同 edge 下已存在同 domain 的 Ingress 标为 AlreadyManaged,
// 前端据此置灰"导入"按钮,避免重复落库导致 unique(edge_id, domain) 冲突 500。
func annotateAlreadyManaged(cands []ingress.IngressCandidate, db *gorm.DB, edgeID uint) {
	existing, err := repo.ListIngressDomainsByEdgeID(context.Background(), db, edgeID)
	if err != nil || len(existing) == 0 {
		return
	}
	known := make(map[string]struct{}, len(existing))
	for _, d := range existing {
		known[d] = struct{}{}
	}
	for i := range cands {
		if _, hit := known[cands[i].ServerName]; hit {
			cands[i].AlreadyManaged = true
		}
	}
}

// annotateCrossServer 把 proxy_pass 命中**另一台**已注册 Server 的 host/Address 时
// 回填 CrossServerID/Name。同 edge 自身的 loopback / 自家私网命中**不**视作跨机。
//
// 平移自 api/ingresses/import_handler.go::annotateCrossServer。本函数不返回 error
// (best-effort,DB 抖一下不影响主流程)。
func annotateCrossServer(cands []ingress.IngressCandidate, db *gorm.DB, edgeID uint) {
	servers, err := repo.ListAllServers(context.Background(), db)
	if err != nil {
		return
	}
	type srvRef struct {
		id   uint
		name string
	}
	lookup := make(map[string]srvRef, len(servers)*2)
	for _, s := range servers {
		ref := srvRef{id: s.ID, name: s.Name}
		if h := strings.TrimSpace(s.Host); h != "" {
			lookup[h] = ref
		}
		for _, n := range s.Networks {
			// loopback 永远是 127.0.0.1,不能用来跨机匹配 — 任何 edge 上看到
			// proxy_pass http://127.0.0.1 都是"自家进程"。
			if n.Kind == domain.NetworkKindLoopback {
				continue
			}
			if a := strings.TrimSpace(n.Address); a != "" {
				lookup[a] = ref
			}
		}
	}
	for i := range cands {
		for j := range cands[i].Routes {
			host, ok := nginxingress.ProxyPassHost(cands[i].Routes[j].ProxyPass)
			if !ok {
				continue
			}
			ref, hit := lookup[host]
			if !hit || ref.id == edgeID {
				continue
			}
			cands[i].Routes[j].CrossServerID = ref.id
			cands[i].Routes[j].CrossServerName = ref.name
		}
	}
}

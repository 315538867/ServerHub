// Package ingress 是 IngressBackend 端口包(nginx/caddy/traefik 等)。
//
// 端口契约见 docs/architecture/v2/05-extension-points.md §1.3。
package ingress

import (
	"context"

	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/infra"
)

// Backend 是 ingress 后端契约:渲染 + 校验 + 重载。
type Backend interface {
	Kind() string

	// Render 把路由列表渲染为 backend 配置文件文本。
	// 必须是纯函数,不调远端,便于 golden fixture 字节级对比。
	Render(routes []domain.IngressRoute) (string, error)

	// Validate 在远端检查配置合法性(如 `nginx -t`)。
	Validate(ctx context.Context, r infra.Runner) error

	// Reload 让 backend 重载(如 `systemctl reload nginx`)。
	Reload(ctx context.Context, r infra.Runner, server *domain.Server) error
}

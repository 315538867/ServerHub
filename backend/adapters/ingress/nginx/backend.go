package nginx

import (
	"context"

	"github.com/serverhub/serverhub/core/ingress"
	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/infra"
)

// Backend 是 nginx 的 IngressBackend 实现。零值可用、无状态。
type Backend struct{}

func (Backend) Kind() string { return "nginx" }

// Render 委托给 internal/render(纯函数);见 render.go。
func (b Backend) Render(routes []domain.IngressRoute) (string, error) {
	return renderRoutes(routes)
}

// Validate 在远端跑 `nginx -t`;见 validate.go。
func (b Backend) Validate(ctx context.Context, r infra.Runner) error {
	return validate(ctx, r)
}

// Reload 在远端 reload nginx;见 reload.go。
func (b Backend) Reload(ctx context.Context, r infra.Runner, server *domain.Server) error {
	return reload(ctx, r, server)
}

// Discover 扫描远端反代 vhost 候选;见 discover.go。
// commit 2 stub:返回 (nil, nil),完整实现在 commit 3 平移。
func (b Backend) Discover(ctx context.Context, r infra.Runner) ([]ingress.IngressCandidate, error) {
	return discover(ctx, r)
}

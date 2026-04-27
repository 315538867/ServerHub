package nginx

import (
	"github.com/serverhub/serverhub/domain"
)

// renderRoutes 是 Backend.Render 的内部实现。
//
// commit 2 阶段:domain.IngressRoute 的字段集与 internal/render.IngressCtx
// 不一一对应(internal/render 期望已经聚合好的 IngressCtx,而 Backend.Render
// 接到的是单条 IngressRoute 列表)。完整聚合(域名分组、ssl 证书内联、profile
// 路径)由 commit 3 的 internal/reconciler 在 LoadDesired 内做掉,Backend.Render
// 在 commit 4 usecase 接通后用 reconciler 走单次输出。
//
// 当前 stub 返回空串 + nil:commit 3 替换成调 internal/render.Render。
func renderRoutes(routes []domain.IngressRoute) (string, error) {
	_ = routes
	return "", nil
}

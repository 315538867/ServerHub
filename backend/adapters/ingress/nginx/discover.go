package nginx

import (
	"context"

	"github.com/serverhub/serverhub/core/ingress"
	"github.com/serverhub/serverhub/infra"
)

// discover 扫描远端 nginx 反代 vhost 候选。
//
// commit 2 stub:返回 (nil, nil) — 没有候选不视为错误。
// 完整实现在 commit 3 平移自 backend/pkg/discovery/ingress_proxy.go::ScanNginxIngressProxy
// 配套测试由 commit 3 一并搬入 discover_test.go。
func discover(ctx context.Context, r infra.Runner) ([]ingress.IngressCandidate, error) {
	_, _ = ctx, r
	return nil, nil
}

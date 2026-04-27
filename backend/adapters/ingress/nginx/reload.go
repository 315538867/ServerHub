package nginx

import (
	"context"
	"fmt"

	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/infra"
)

// reload 让远端 nginx 重载配置(SIGHUP 等价)。
//
// commit 2 用默认命令串(等价 nginxrender.DefaultProfile().ReloadCmd);
// per-edge 命令由 commit 3 internal/reconciler 接管。server 入参当前未用,
// 留给后续可能的 server-aware 重载策略(如多 nginx 实例选择)。
func reload(ctx context.Context, r infra.Runner, _ *domain.Server) error {
	stdout, stderr, err := r.Run(ctx, "sudo -n nginx -s reload 2>&1")
	if err != nil {
		return fmt.Errorf("nginx reload failed: %s%s: %w", stdout, stderr, err)
	}
	return nil
}

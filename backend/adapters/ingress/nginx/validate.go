package nginx

import (
	"context"
	"fmt"

	"github.com/serverhub/serverhub/infra"
)

// validate 跑 `nginx -t` 校验远端配置合法性。
//
// commit 2 用默认命令串(等价 nginxrender.DefaultProfile().TestCmd);
// 多实例 / 自编译 / 容器化场景的 per-edge 命令在 commit 3 起由
// internal/reconciler 接管(LoadProfile + p.TestCmd)。Backend.Validate
// 走 default 路径,符合 core/ingress 端口"无 profile 入参"的合一接口契约。
func validate(ctx context.Context, r infra.Runner) error {
	stdout, stderr, err := r.Run(ctx, "sudo -n nginx -t 2>&1")
	if err != nil {
		return fmt.Errorf("nginx -t failed: %s%s: %w", stdout, stderr, err)
	}
	return nil
}

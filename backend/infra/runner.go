// Package infra 提供基础设施抽象(SSH/Docker/HTTP runner 等)。
//
// R1 阶段:仅落 Runner 接口骨架,满足 core/ 端口编译需求。
// R2 阶段:把 pkg/sshexec / pkg/dockercli 等迁入并实现 Runner。
package infra

import "context"

// Runner 是远端命令执行抽象。
//
// 实现要求(R2 起):
//   - 必须 context-aware,ctx 取消应中断命令
//   - 输出大小受调用方限制
//   - 不应在内部 retry,由 usecase 决策
type Runner interface {
	// Run 执行命令并返回 stdout / stderr / err。
	// 远端非 0 退出码视为 err(包含 stderr)。
	Run(ctx context.Context, cmd string) (stdout, stderr string, err error)
}

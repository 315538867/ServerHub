package infra

import (
	"context"
	"fmt"
	"strings"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/pkg/runner"
)

// V1 把 v1 pkg/runner.Runner 适配到 infra.Runner。
//
// v1 Run 返回 combined output(stdout+stderr 合并),没有 ctx。这里把同一份输出
// 同时塞进 stdout 与 stderr,err 透传。ctx 取消由 caller 在更外层处理:本封装
// 不开启 goroutine,中断只能等命令自然返回(已有调用方的行为一致)。
type v1Adapter struct{ rn runner.Runner }

func (a v1Adapter) Run(ctx context.Context, cmd string) (string, string, error) {
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	out, err := a.rn.Run(cmd)
	if err != nil {
		return out, out, err
	}
	return out, "", nil
}

// AdaptV1 包装 v1 runner.Runner 为 infra.Runner。
func AdaptV1(rn runner.Runner) Runner { return v1Adapter{rn: rn} }

// For 接 v1 runner.For:发现/接管路径单点入口,避免每个 caller 都 import
// pkg/runner 又包一次。返回 infra.Runner;v1 runner 的 Close/Session 不在此
// 暴露——发现/接管全部走 Run 一条路。
func For(s *domain.Server, cfg *config.Config) (Runner, error) {
	if s == nil {
		return nil, fmt.Errorf("nil server")
	}
	rn, err := runner.For(s, cfg)
	if err != nil {
		return nil, err
	}
	return AdaptV1(rn), nil
}

// SafeError 把 stderr/stdout 合并为单行错误描述,供 caller 在日志中复用。
func SafeError(stdout, stderr string, err error) string {
	if err == nil {
		return ""
	}
	pieces := []string{err.Error()}
	if s := strings.TrimSpace(stderr); s != "" {
		pieces = append(pieces, s)
	} else if s := strings.TrimSpace(stdout); s != "" {
		pieces = append(pieces, s)
	}
	return strings.Join(pieces, ": ")
}

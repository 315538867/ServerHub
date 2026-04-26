// Package source 是 SourceScanner 端口包(发现/接管远端候选服务)。
//
// 端口契约见 docs/architecture/v2/05-extension-points.md §1.2。
package source

import (
	"context"

	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/infra"
)

// TakeoverContext 由 usecase 组装,传给 Takeover。
type TakeoverContext struct {
	Server  *domain.Server
	Runner  infra.Runner
	Cand    Candidate
	AppID   *uint  // 若用户接管时绑定 application
	SvcName string // 用户最终选定的名字
}

// Scanner 是源适配器契约:发现候选、生成指纹、物化为 Service。
//
// 实现要求:
//  1. Discover 必须 context-aware,远端不可达返回 (nil, err)
//  2. Fingerprint 是纯函数,同物理服务在不同 server 上 SHA1 必须一致
//  3. Takeover 失败时副作用回滚由 stepEngine 负责,Scanner 不写 repo
type Scanner interface {
	Kind() string

	// Discover 在 server 上扫所有候选。
	// 远端不可达返回 (nil, err); 没有候选返回 (nil, nil)。
	Discover(ctx context.Context, r infra.Runner) ([]Candidate, error)

	// Fingerprint 用于去重:必须是纯函数,不调远端。
	Fingerprint(c Candidate) string

	// Takeover 把候选物化为 Service(写远端配置 + 调 stepEngine)。
	Takeover(ctx context.Context, tc TakeoverContext) error
}

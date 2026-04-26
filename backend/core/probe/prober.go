// Package probe 是 HealthProber 端口包(http/tcp/exec)。
//
// 端口契约见 docs/architecture/v2/05-extension-points.md §1.5。
// R1 仅放接口骨架,真正实现与 Registry 在 R5 与 ingress/probe 联动时落地。
package probe

import (
	"context"
	"time"
)

// Target 是一次健康检查的目标描述。
type Target struct {
	URL     string // http(s)://...
	Method  string // GET/POST
	Headers map[string]string
	Body    []byte
	Expect  ExpectRule
}

// ExpectRule 描述判定通过的条件。
type ExpectRule struct {
	StatusCode int    // 0 = 任意 2xx
	BodyRegex  string // 空 = 不校验
}

// Status 是一次探测的结果。
type Status struct {
	Healthy  bool
	Latency  time.Duration
	HTTPCode int
	Error    string
}

// Prober 是健康探测器契约。
type Prober interface {
	Kind() string // "http" / "tcp" / "exec"
	Check(ctx context.Context, target Target) (Status, error)
}

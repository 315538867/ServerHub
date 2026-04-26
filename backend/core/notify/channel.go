// Package notify 是 NotifyChannel 端口包(webhook/email/feishu 等)。
//
// 端口契约见 docs/architecture/v2/05-extension-points.md §1.4。
package notify

import "context"

// Channel 是通知渠道契约。
//
// 实现要求:
//  1. 必须 context-aware,超时返回 err
//  2. 不应内部 retry(重试策略由 usecase 层决定)
//  3. Send 应是幂等可重入的(由调用方保证 dedup key)
type Channel interface {
	Kind() string

	// Send 发送一次通知。
	Send(ctx context.Context, msg Message) error
}

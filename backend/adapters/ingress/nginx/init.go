// Package nginx 是 IngressBackend adapter,实现 core/ingress.Backend 契约。
//
// R5 决议 1A：nginxrender 已内部化到 internal/render；reconciler/ssl/profile
// 在 commit 3 平移；Discover 在 commit 3 平移自 pkg/discovery/ingress_proxy。
//
// 本包通过 init() 自注册到 ingress.Default，main.go 用 blank import 触发。
package nginx

import "github.com/serverhub/serverhub/core/ingress"

func init() { ingress.Default.Register(Backend{}) }

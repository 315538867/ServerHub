// Package nginx 是 SourceScanner adapter,负责发现 nginx 静态站点候选并
// 接管成 ServerHub 标准 release 目录。反向代理 vhost 不在本 adapter
// 处理(归属 R5 ingress 适配器)。
package nginx

import "github.com/serverhub/serverhub/core/source"

func init() { source.Default.Register(Scanner{}) }

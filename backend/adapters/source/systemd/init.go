// Package systemd 是 SourceScanner adapter,负责发现并接管自定义 systemd
// service(过滤系统包 unit)。安全门:拒绝接管 /usr/lib /etc /var/lib 等
// 系统目录里的 unit / 二进制,避免破坏 apt/yum 升级契约。
package systemd

import "github.com/serverhub/serverhub/core/source"

func init() { source.Default.Register(Scanner{}) }

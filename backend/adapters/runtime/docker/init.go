package docker

import "github.com/serverhub/serverhub/core/runtime"

// init 自注册到全局 Registry。main.go 用 blank import 触发本 init。
// Kind 重复会 panic(Registry 行为),保证编译期/启动期可见。
func init() {
	runtime.Default.Register(Adapter{})
}

package native

import "github.com/serverhub/serverhub/core/runtime"

func init() {
	runtime.Default.Register(Adapter{})
}

package runtime

import (
	"fmt"
	"sort"
	"sync"
)

// Registry 是 RuntimeAdapter 的注册表。
//
// 使用约定:
//   - adapters/runtime/<kind> 包内 init() 调用 Default.Register
//   - cmd/main.go 顶部 blank import 触发 init
//   - 重复注册 Kind 启动期 panic
type Registry struct {
	mu sync.RWMutex
	m  map[string]Adapter
}

// Default 是全局默认 Registry,业务层从此处取 adapter。
var Default = &Registry{m: map[string]Adapter{}}

// Register 注册一个 Adapter。重复 Kind 直接 panic(启动期暴露)。
func (r *Registry) Register(a Adapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	kind := a.Kind()
	if kind == "" {
		panic(fmt.Sprintf("runtime: %T: Kind() returns empty", a))
	}
	if _, dup := r.m[kind]; dup {
		panic(fmt.Sprintf("runtime: duplicate Kind %q registered", kind))
	}
	r.m[kind] = a
}

// Get 按 Kind 获取 Adapter,未注册返回 error。
func (r *Registry) Get(kind string) (Adapter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.m[kind]
	if !ok {
		return nil, fmt.Errorf("runtime: kind %q not registered", kind)
	}
	return a, nil
}

// MustGet 按 Kind 获取 Adapter,未注册 panic(usecase 层应已校验合法性)。
func (r *Registry) MustGet(kind string) Adapter {
	a, err := r.Get(kind)
	if err != nil {
		panic(err)
	}
	return a
}

// All 返回所有已注册 Adapter,按 Kind 升序排序保证稳定性。
func (r *Registry) All() []Adapter {
	r.mu.RLock()
	defer r.mu.RUnlock()
	kinds := make([]string, 0, len(r.m))
	for k := range r.m {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	out := make([]Adapter, 0, len(kinds))
	for _, k := range kinds {
		out = append(out, r.m[k])
	}
	return out
}

// Kinds 返回所有已注册 Kind,按字典序排序。
func (r *Registry) Kinds() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ks := make([]string, 0, len(r.m))
	for k := range r.m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// Reset 仅供测试使用,清空注册表。
func (r *Registry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m = map[string]Adapter{}
}

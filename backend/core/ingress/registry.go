package ingress

import (
	"fmt"
	"sort"
	"sync"
)

// Registry 是 IngressBackend 的注册表。模板与 core/runtime.Registry 一致。
type Registry struct {
	mu sync.RWMutex
	m  map[string]Backend
}

// Default 是全局默认 Registry。
var Default = &Registry{m: map[string]Backend{}}

// Register 注册一个 Backend。重复 Kind panic。
func (r *Registry) Register(b Backend) {
	r.mu.Lock()
	defer r.mu.Unlock()
	kind := b.Kind()
	if kind == "" {
		panic(fmt.Sprintf("ingress: %T: Kind() returns empty", b))
	}
	if _, dup := r.m[kind]; dup {
		panic(fmt.Sprintf("ingress: duplicate Kind %q registered", kind))
	}
	r.m[kind] = b
}

func (r *Registry) Get(kind string) (Backend, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	b, ok := r.m[kind]
	if !ok {
		return nil, fmt.Errorf("ingress: kind %q not registered", kind)
	}
	return b, nil
}

func (r *Registry) MustGet(kind string) Backend {
	b, err := r.Get(kind)
	if err != nil {
		panic(err)
	}
	return b
}

func (r *Registry) All() []Backend {
	r.mu.RLock()
	defer r.mu.RUnlock()
	kinds := make([]string, 0, len(r.m))
	for k := range r.m {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	out := make([]Backend, 0, len(kinds))
	for _, k := range kinds {
		out = append(out, r.m[k])
	}
	return out
}

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

// Reset 仅供测试使用。
func (r *Registry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m = map[string]Backend{}
}

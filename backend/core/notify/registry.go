package notify

import (
	"fmt"
	"sort"
	"sync"
)

// Registry 是 NotifyChannel 的注册表。模板与 core/runtime.Registry 一致。
type Registry struct {
	mu sync.RWMutex
	m  map[string]Channel
}

// Default 是全局默认 Registry。
var Default = &Registry{m: map[string]Channel{}}

func (r *Registry) Register(c Channel) {
	r.mu.Lock()
	defer r.mu.Unlock()
	kind := c.Kind()
	if kind == "" {
		panic(fmt.Sprintf("notify: %T: Kind() returns empty", c))
	}
	if _, dup := r.m[kind]; dup {
		panic(fmt.Sprintf("notify: duplicate Kind %q registered", kind))
	}
	r.m[kind] = c
}

func (r *Registry) Get(kind string) (Channel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.m[kind]
	if !ok {
		return nil, fmt.Errorf("notify: kind %q not registered", kind)
	}
	return c, nil
}

func (r *Registry) MustGet(kind string) Channel {
	c, err := r.Get(kind)
	if err != nil {
		panic(err)
	}
	return c
}

func (r *Registry) All() []Channel {
	r.mu.RLock()
	defer r.mu.RUnlock()
	kinds := make([]string, 0, len(r.m))
	for k := range r.m {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	out := make([]Channel, 0, len(kinds))
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
	r.m = map[string]Channel{}
}

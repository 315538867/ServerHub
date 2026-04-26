package source

import (
	"fmt"
	"sort"
	"sync"
)

// Registry 是 SourceScanner 的注册表。模板与 core/runtime.Registry 一致。
type Registry struct {
	mu sync.RWMutex
	m  map[string]Scanner
}

// Default 是全局默认 Registry。
var Default = &Registry{m: map[string]Scanner{}}

// Register 注册一个 Scanner。重复 Kind 直接 panic(启动期暴露)。
func (r *Registry) Register(s Scanner) {
	r.mu.Lock()
	defer r.mu.Unlock()
	kind := s.Kind()
	if kind == "" {
		panic(fmt.Sprintf("source: %T: Kind() returns empty", s))
	}
	if _, dup := r.m[kind]; dup {
		panic(fmt.Sprintf("source: duplicate Kind %q registered", kind))
	}
	r.m[kind] = s
}

func (r *Registry) Get(kind string) (Scanner, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.m[kind]
	if !ok {
		return nil, fmt.Errorf("source: kind %q not registered", kind)
	}
	return s, nil
}

func (r *Registry) MustGet(kind string) Scanner {
	s, err := r.Get(kind)
	if err != nil {
		panic(err)
	}
	return s
}

func (r *Registry) All() []Scanner {
	r.mu.RLock()
	defer r.mu.RUnlock()
	kinds := make([]string, 0, len(r.m))
	for k := range r.m {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	out := make([]Scanner, 0, len(kinds))
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
	r.m = map[string]Scanner{}
}

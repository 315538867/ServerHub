package nginxops

import (
	"errors"
	"io"
	"strings"
	"sync"

	"github.com/serverhub/serverhub/pkg/runner"
	"github.com/serverhub/serverhub/pkg/sysinfo"
)

// fakeRunner 是 nginxops 单测用的 runner.Runner 实现：按命令前缀匹配返回
// 预设输出，记录调用顺序，便于断言。
type fakeRunner struct {
	mu       sync.Mutex
	cap      string
	calls    []string
	handlers []handler
	defaults string
	defErr   error
}

type handler struct {
	match func(string) bool
	out   string
	err   error
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{cap: sysinfo.CapFull}
}

// onPrefix 注册命令前缀匹配响应。
func (f *fakeRunner) onPrefix(prefix, out string, err error) *fakeRunner {
	f.handlers = append(f.handlers, handler{
		match: func(s string) bool { return strings.HasPrefix(s, prefix) },
		out:   out, err: err,
	})
	return f
}

// onContains 注册子串匹配响应。
func (f *fakeRunner) onContains(sub, out string, err error) *fakeRunner {
	f.handlers = append(f.handlers, handler{
		match: func(s string) bool { return strings.Contains(s, sub) },
		out:   out, err: err,
	})
	return f
}

func (f *fakeRunner) Run(cmd string) (string, error) {
	f.mu.Lock()
	f.calls = append(f.calls, cmd)
	hs := f.handlers
	f.mu.Unlock()
	for _, h := range hs {
		if h.match(cmd) {
			return h.out, h.err
		}
	}
	return f.defaults, f.defErr
}

func (f *fakeRunner) NewSession() (runner.Session, error) { return nil, errors.New("not impl") }
func (f *fakeRunner) IsLocal() bool                       { return false }
func (f *fakeRunner) Capability() string                  { return f.cap }
func (f *fakeRunner) Close() error                        { return nil }

// 防止 io 未使用告警（部分编译期工具会扫，不影响实际）。
var _ io.Reader = (*strings.Reader)(nil)

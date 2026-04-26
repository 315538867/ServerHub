package ingress

import (
	"context"
	"testing"

	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/infra"
)

type fakeBackend struct{ kind string }

func (f *fakeBackend) Kind() string                                        { return f.kind }
func (f *fakeBackend) Render([]domain.IngressRoute) (string, error)        { return "", nil }
func (f *fakeBackend) Validate(context.Context, infra.Runner) error        { return nil }
func (f *fakeBackend) Reload(context.Context, infra.Runner, *domain.Server) error {
	return nil
}

func newReg() *Registry { return &Registry{m: map[string]Backend{}} }

func TestRegistry_RegisterGetMustGet(t *testing.T) {
	r := newReg()
	b := &fakeBackend{kind: "nginx"}
	r.Register(b)
	if got, _ := r.Get("nginx"); got != b {
		t.Fatalf("Get(nginx) wrong")
	}
	if got := r.MustGet("nginx"); got != b {
		t.Fatalf("MustGet wrong")
	}
}

func TestRegistry_DuplicatePanics(t *testing.T) {
	r := newReg()
	r.Register(&fakeBackend{kind: "nginx"})
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic")
		}
	}()
	r.Register(&fakeBackend{kind: "nginx"})
}

func TestRegistry_EmptyKindPanics(t *testing.T) {
	r := newReg()
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic")
		}
	}()
	r.Register(&fakeBackend{kind: ""})
}

func TestRegistry_MissingGet(t *testing.T) {
	r := newReg()
	if _, err := r.Get("none"); err == nil {
		t.Fatalf("Get(none) should error")
	}
	defer func() {
		if recover() == nil {
			t.Fatalf("expected MustGet panic")
		}
	}()
	r.MustGet("none")
}

func TestRegistry_AllKindsSorted(t *testing.T) {
	r := newReg()
	for _, k := range []string{"traefik", "caddy", "nginx"} {
		r.Register(&fakeBackend{kind: k})
	}
	want := []string{"caddy", "nginx", "traefik"}
	got := r.Kinds()
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Kinds[%d]=%q want %q", i, got[i], want[i])
		}
	}
}

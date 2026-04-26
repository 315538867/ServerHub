package source

import (
	"context"
	"sync"
	"testing"

	"github.com/serverhub/serverhub/infra"
)

type fakeScanner struct{ kind string }

func (f *fakeScanner) Kind() string { return f.kind }
func (f *fakeScanner) Discover(context.Context, infra.Runner) ([]Candidate, error) {
	return nil, nil
}
func (f *fakeScanner) Fingerprint(Candidate) string                 { return "" }
func (f *fakeScanner) Takeover(context.Context, TakeoverContext) error { return nil }

func newReg() *Registry { return &Registry{m: map[string]Scanner{}} }

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := newReg()
	s := &fakeScanner{kind: "docker"}
	r.Register(s)
	if got, err := r.Get("docker"); err != nil || got != s {
		t.Fatalf("Get(docker) = %v, %v", got, err)
	}
	if _, err := r.Get("missing"); err == nil {
		t.Fatalf("Get(missing) should error")
	}
}

func TestRegistry_DuplicatePanics(t *testing.T) {
	r := newReg()
	r.Register(&fakeScanner{kind: "x"})
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic")
		}
	}()
	r.Register(&fakeScanner{kind: "x"})
}

func TestRegistry_EmptyKindPanics(t *testing.T) {
	r := newReg()
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic on empty Kind")
		}
	}()
	r.Register(&fakeScanner{kind: ""})
}

func TestRegistry_AllSorted(t *testing.T) {
	r := newReg()
	for _, k := range []string{"systemd", "docker", "nginx", "compose"} {
		r.Register(&fakeScanner{kind: k})
	}
	want := []string{"compose", "docker", "nginx", "systemd"}
	got := r.Kinds()
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Kinds[%d]=%q want %q", i, got[i], want[i])
		}
	}
}

func TestRegistry_Concurrent(t *testing.T) {
	r := newReg()
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r.Register(&fakeScanner{kind: keyOf(i)})
		}(i)
	}
	wg.Wait()
	if len(r.Kinds()) != 32 {
		t.Fatalf("got %d kinds", len(r.Kinds()))
	}
}

func keyOf(i int) string {
	const hex = "0123456789abcdef"
	return "k" + string([]byte{hex[(i>>4)&0xF], hex[i&0xF]})
}

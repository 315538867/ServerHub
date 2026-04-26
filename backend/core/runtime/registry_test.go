package runtime

import (
	"context"
	"sync"
	"testing"

	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/infra"
)

type fakeAdapter struct{ kind string }

func (f *fakeAdapter) Kind() string { return f.kind }
func (f *fakeAdapter) PlanStart(*domain.Service, *domain.Release) ([]Step, error) {
	return nil, nil
}
func (f *fakeAdapter) BuildStartCmd(*domain.Service, *domain.Release) (string, error) {
	return "", nil
}
func (f *fakeAdapter) Probe(context.Context, infra.Runner, *domain.Service) (Status, error) {
	return Status{}, nil
}
func (f *fakeAdapter) Stop(context.Context, infra.Runner, *domain.Service) error { return nil }

func newReg() *Registry { return &Registry{m: map[string]Adapter{}} }

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := newReg()
	a := &fakeAdapter{kind: "docker"}
	r.Register(a)

	got, err := r.Get("docker")
	if err != nil || got != a {
		t.Fatalf("Get(docker) = %v, %v; want adapter, nil", got, err)
	}
	if _, err := r.Get("missing"); err == nil {
		t.Fatalf("Get(missing) should error")
	}
}

func TestRegistry_DuplicatePanics(t *testing.T) {
	r := newReg()
	r.Register(&fakeAdapter{kind: "docker"})
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic on duplicate Kind")
		}
	}()
	r.Register(&fakeAdapter{kind: "docker"})
}

func TestRegistry_EmptyKindPanics(t *testing.T) {
	r := newReg()
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic on empty Kind")
		}
	}()
	r.Register(&fakeAdapter{kind: ""})
}

func TestRegistry_MustGet(t *testing.T) {
	r := newReg()
	r.Register(&fakeAdapter{kind: "k"})
	if got := r.MustGet("k"); got == nil {
		t.Fatalf("MustGet returned nil")
	}
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic on MustGet missing")
		}
	}()
	r.MustGet("nope")
}

func TestRegistry_AllAndKindsSorted(t *testing.T) {
	r := newReg()
	for _, k := range []string{"static", "docker", "native", "compose"} {
		r.Register(&fakeAdapter{kind: k})
	}
	want := []string{"compose", "docker", "native", "static"}
	got := r.Kinds()
	if len(got) != len(want) {
		t.Fatalf("Kinds len=%d", len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Kinds[%d]=%q want %q", i, got[i], want[i])
		}
	}
	all := r.All()
	for i := range want {
		if all[i].Kind() != want[i] {
			t.Fatalf("All()[%d].Kind=%q want %q", i, all[i].Kind(), want[i])
		}
	}
}

func TestRegistry_ConcurrentRegisterGet(t *testing.T) {
	r := newReg()
	const N = 64
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r.Register(&fakeAdapter{kind: keyOf(i)})
		}(i)
	}
	wg.Wait()
	if got := len(r.Kinds()); got != N {
		t.Fatalf("Kinds len=%d want %d", got, N)
	}

	wg = sync.WaitGroup{}
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := r.Get(keyOf(i)); err != nil {
				t.Errorf("Get(%s): %v", keyOf(i), err)
			}
		}(i)
	}
	wg.Wait()
}

func keyOf(i int) string {
	const hex = "0123456789abcdef"
	return "k" + string([]byte{hex[(i>>4)&0xF], hex[i&0xF]})
}

package notify

import (
	"context"
	"testing"
)

type fakeChannel struct{ kind string }

func (f *fakeChannel) Kind() string                          { return f.kind }
func (f *fakeChannel) Send(context.Context, Message) error   { return nil }

func newReg() *Registry { return &Registry{m: map[string]Channel{}} }

func TestRegistry_RegisterGet(t *testing.T) {
	r := newReg()
	c := &fakeChannel{kind: "webhook"}
	r.Register(c)
	if got, _ := r.Get("webhook"); got != c {
		t.Fatalf("Get wrong")
	}
}

func TestRegistry_DuplicatePanics(t *testing.T) {
	r := newReg()
	r.Register(&fakeChannel{kind: "webhook"})
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic")
		}
	}()
	r.Register(&fakeChannel{kind: "webhook"})
}

func TestRegistry_EmptyKindPanics(t *testing.T) {
	r := newReg()
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic")
		}
	}()
	r.Register(&fakeChannel{kind: ""})
}

func TestRegistry_MissingGet(t *testing.T) {
	r := newReg()
	if _, err := r.Get("none"); err == nil {
		t.Fatalf("Get(none) should error")
	}
	defer func() {
		if recover() == nil {
			t.Fatalf("MustGet should panic")
		}
	}()
	r.MustGet("none")
}

func TestRegistry_AllKindsSorted(t *testing.T) {
	r := newReg()
	for _, k := range []string{"feishu", "email", "webhook"} {
		r.Register(&fakeChannel{kind: k})
	}
	want := []string{"email", "feishu", "webhook"}
	got := r.Kinds()
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Kinds[%d]=%q want %q", i, got[i], want[i])
		}
	}
}

func TestSeverityValues(t *testing.T) {
	for _, s := range []Severity{SeverityInfo, SeverityWarn, SeverityError} {
		if s == "" {
			t.Fatalf("severity empty")
		}
	}
}

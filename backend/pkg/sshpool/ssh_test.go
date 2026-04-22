package sshpool

import (
	"errors"
	"strings"
	"sync"
	"testing"

	gossh "golang.org/x/crypto/ssh"
)

type fakeStore struct {
	mu  sync.Mutex
	got map[uint]string
}

func (f *fakeStore) Get(id uint) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.got[id]
	return v, ok
}
func (f *fakeStore) Set(id uint, fp string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.got == nil {
		f.got = map[uint]string{}
	}
	f.got[id] = fp
	return nil
}

// stubKey implements gossh.PublicKey just enough for FingerprintSHA256.
type stubKey struct{ blob []byte }

func (k *stubKey) Type() string                                       { return "ssh-rsa" }
func (k *stubKey) Marshal() []byte                                    { return k.blob }
func (k *stubKey) Verify(_ []byte, _ *gossh.Signature) error          { return nil }

func TestHostKeyCallback_TOFUSet(t *testing.T) {
	prev := hostKeyStore
	t.Cleanup(func() { hostKeyStore = prev })
	store := &fakeStore{}
	SetHostKeyStore(store)

	cb := hostKeyCallback(42)
	key := &stubKey{blob: []byte("first-key")}
	if err := cb("h", nil, key); err != nil {
		t.Fatalf("first connect (TOFU) should pin, got %v", err)
	}
	if got, _ := store.Get(42); got == "" {
		t.Error("fingerprint not stored on TOFU")
	}
}

func TestHostKeyCallback_MismatchRejected(t *testing.T) {
	prev := hostKeyStore
	prevHook := OnHostKeyMismatch
	t.Cleanup(func() {
		hostKeyStore = prev
		OnHostKeyMismatch = prevHook
	})

	pinned := gossh.FingerprintSHA256(&stubKey{blob: []byte("pinned-key")})
	store := &fakeStore{got: map[uint]string{7: pinned}}
	SetHostKeyStore(store)

	called := 0
	OnHostKeyMismatch = func(_ uint, _, _, _ string) { called++ }

	cb := hostKeyCallback(7)
	err := cb("h", nil, &stubKey{blob: []byte("evil-key")})
	if !errors.Is(err, ErrHostKeyMismatch) {
		t.Errorf("want ErrHostKeyMismatch, got %v", err)
	}
	if called != 1 {
		t.Errorf("OnHostKeyMismatch invocations = %d, want 1", called)
	}
}

func TestHostKeyCallback_FailsClosedWithoutStore(t *testing.T) {
	prev := hostKeyStore
	t.Cleanup(func() { hostKeyStore = prev })
	hostKeyStore = nil

	err := hostKeyCallback(1)("h", nil, &stubKey{blob: []byte("k")})
	if err == nil || !strings.Contains(err.Error(), "store not initialised") {
		t.Errorf("want fail-closed error, got %v", err)
	}
}

func TestIsSessionUnrecoverable(t *testing.T) {
	for _, msg := range []string{
		"open failed: administratively prohibited",
		"resource shortage",
		"use of closed network connection",
		"EOF",
	} {
		if !isSessionUnrecoverable(errors.New(msg)) {
			t.Errorf("%q should be unrecoverable", msg)
		}
	}
	if isSessionUnrecoverable(nil) {
		t.Error("nil should not be unrecoverable")
	}
	if isSessionUnrecoverable(errors.New("permission denied")) {
		t.Error("auth errors should not be classified as session-unrecoverable")
	}
}

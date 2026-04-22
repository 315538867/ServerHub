package totp

import (
	"encoding/base32"
	"testing"
	"time"
)

const testSecret = "JBSWY3DPEHPK3PXP" // RFC 4648 example "Hello!"

func TestGenerateAndVerify(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	key, _ := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(testSecret)
	step := now.Unix() / 30
	code := generate(key, step)

	gotStep, ok := VerifyAt(testSecret, code, now)
	if !ok || gotStep != step {
		t.Fatalf("VerifyAt = (%d,%v), want (%d,true)", gotStep, ok, step)
	}
}

func TestVerifyWindow(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	key, _ := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(testSecret)
	step := now.Unix() / 30

	// ±1 step should be accepted (clock skew tolerance).
	for _, delta := range []int64{-1, 0, 1} {
		code := generate(key, step+delta)
		gotStep, ok := VerifyAt(testSecret, code, now)
		if !ok || gotStep != step+delta {
			t.Errorf("delta=%d: VerifyAt = (%d,%v), want (%d,true)", delta, gotStep, ok, step+delta)
		}
	}
	// ±2 must not be accepted.
	for _, delta := range []int64{-2, 2} {
		code := generate(key, step+delta)
		if _, ok := VerifyAt(testSecret, code, now); ok {
			t.Errorf("delta=%d should not verify", delta)
		}
	}
}

func TestVerifyRejectsBadCode(t *testing.T) {
	if _, ok := VerifyAt(testSecret, "000000", time.Unix(1_700_000_000, 0)); ok {
		t.Error("000000 should not verify against arbitrary now")
	}
	if _, ok := VerifyAt("not!base32", "123456", time.Now()); ok {
		t.Error("malformed secret should not verify")
	}
}

func TestGenerateSecretIsBase32(t *testing.T) {
	s, err := GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(s); err != nil {
		t.Errorf("secret not valid base32: %v", err)
	}
	if len(s) != 32 { // 20 bytes → 32 base32 chars (no padding)
		t.Errorf("secret length = %d, want 32", len(s))
	}
}

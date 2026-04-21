package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"time"
)

// GenerateSecret returns a random 20-byte base32-encoded TOTP secret.
func GenerateSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), nil
}

// OtpAuthURI returns an otpauth:// URI for QR-code display.
func OtpAuthURI(secret, account, issuer string) string {
	return fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s&algorithm=SHA1&digits=6&period=30",
		issuer, account, secret, issuer)
}

// Verify checks code against current ± 1 time window (90s tolerance).
func Verify(secret, code string) bool {
	_, ok := VerifyAt(secret, code, time.Now())
	return ok
}

// VerifyAt is like Verify but takes an explicit "now" and also returns the
// matched time-step (Unix seconds / 30) for replay protection. Callers can
// persist the step per-user and reject any subsequent code whose step is
// less than or equal to the stored value.
func VerifyAt(secret, code string, now time.Time) (int64, bool) {
	secret = strings.ToUpper(strings.ReplaceAll(secret, " ", ""))
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		return 0, false
	}
	t := now.Unix() / 30
	for _, delta := range []int64{-1, 0, 1} {
		step := t + delta
		if subtle.ConstantTimeCompare([]byte(generate(key, step)), []byte(code)) == 1 {
			return step, true
		}
	}
	return 0, false
}

func generate(key []byte, counter int64) string {
	msg := make([]byte, 8)
	binary.BigEndian.PutUint64(msg, uint64(counter))
	mac := hmac.New(sha1.New, key)
	mac.Write(msg)
	h := mac.Sum(nil)
	offset := h[len(h)-1] & 0x0f
	code := binary.BigEndian.Uint32(h[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", int(code)%int(math.Pow10(6)))
}

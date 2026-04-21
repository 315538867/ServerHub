package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/pkg/resp"
)

type ipState struct {
	failures  int
	lockUntil time.Time
	lastSeen  time.Time
}

var (
	rateMu       sync.Mutex
	ipStates     = make(map[string]*ipState)
	acctStates   = make(map[string]*ipState)
	acctMaxFail  = 5
	acctLockMin  = 15
)

// SetAccountLimits adjusts the per-account thresholds (called once at boot
// from RateLimit so we share config). Defaults match RateLimit's defaults.
func setAccountLimits(maxFail, lockMin int) {
	rateMu.Lock()
	defer rateMu.Unlock()
	if maxFail > 0 {
		acctMaxFail = maxFail
	}
	if lockMin > 0 {
		acctLockMin = lockMin
	}
}

// AccountLocked reports whether the given account is currently locked out
// due to too many recent failures. Login handlers should call this before
// validating credentials to avoid leaking timing information about valid
// usernames vs. invalid ones.
func AccountLocked(username string) bool {
	if username == "" {
		return false
	}
	rateMu.Lock()
	defer rateMu.Unlock()
	s, ok := acctStates[username]
	if !ok {
		return false
	}
	return time.Now().Before(s.lockUntil)
}

// RecordAccountFailure increments the failure counter for username and
// applies a lockout when the threshold is reached. Call after any
// authentication attempt that fails for credential reasons.
func RecordAccountFailure(username string) {
	if username == "" {
		return
	}
	rateMu.Lock()
	defer rateMu.Unlock()
	s, ok := acctStates[username]
	if !ok {
		s = &ipState{}
		acctStates[username] = s
	}
	s.lastSeen = time.Now()
	s.failures++
	if s.failures >= acctMaxFail {
		s.lockUntil = time.Now().Add(time.Duration(acctLockMin) * time.Minute)
		s.failures = 0
	}
}

// RecordAccountSuccess resets the failure counter for username. Call after
// successful authentication.
func RecordAccountSuccess(username string) {
	if username == "" {
		return
	}
	rateMu.Lock()
	defer rateMu.Unlock()
	if s, ok := acctStates[username]; ok {
		s.failures = 0
		s.lockUntil = time.Time{}
		s.lastSeen = time.Now()
	}
}

func RateLimit(cfg *config.Config) gin.HandlerFunc {
	maxFail := cfg.Security.LoginMaxAttempts
	lockMin := cfg.Security.LoginLockoutMin
	if maxFail <= 0 {
		maxFail = 5
	}
	if lockMin <= 0 {
		lockMin = 15
	}
	setAccountLimits(maxFail, lockMin)

	// Clean up stale entries periodically.
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			cutoff := time.Now().Add(-time.Duration(lockMin+30) * time.Minute)
			rateMu.Lock()
			for ip, s := range ipStates {
				if s.lastSeen.Before(cutoff) {
					delete(ipStates, ip)
				}
			}
			for u, s := range acctStates {
				if s.lastSeen.Before(cutoff) {
					delete(acctStates, u)
				}
			}
			rateMu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()

		rateMu.Lock()
		state, ok := ipStates[ip]
		if !ok {
			state = &ipState{}
			ipStates[ip] = state
		}
		state.lastSeen = time.Now()
		if time.Now().Before(state.lockUntil) {
			rateMu.Unlock()
			resp.Fail(c, http.StatusTooManyRequests, 1005, "too many attempts, try again later")
			return
		}
		rateMu.Unlock()

		c.Next()

		// track failures on 401 responses
		if c.Writer.Status() == http.StatusUnauthorized {
			rateMu.Lock()
			state.failures++
			if state.failures >= maxFail {
				state.lockUntil = time.Now().Add(time.Duration(lockMin) * time.Minute)
				state.failures = 0
			}
			rateMu.Unlock()
		} else if c.Writer.Status() == http.StatusOK {
			rateMu.Lock()
			state.failures = 0
			rateMu.Unlock()
		}
	}
}

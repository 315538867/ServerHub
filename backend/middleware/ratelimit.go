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
	rateMu   sync.Mutex
	ipStates = make(map[string]*ipState)
)

func RateLimit(cfg *config.Config) gin.HandlerFunc {
	maxFail := cfg.Security.LoginMaxAttempts
	lockMin := cfg.Security.LoginLockoutMin
	if maxFail <= 0 {
		maxFail = 5
	}
	if lockMin <= 0 {
		lockMin = 15
	}

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

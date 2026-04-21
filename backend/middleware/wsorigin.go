package middleware

import (
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/serverhub/serverhub/config"
)

// WSCheckOrigin returns a CheckOrigin function that rejects cross-origin
// WebSocket handshakes. A missing Origin is accepted (non-browser clients
// such as the agent CLI, curl, and some proxies omit it). In dev mode the
// Vite dev server on :5173 is allowed so that `npm run dev` keeps working.
//
// This replaces the previous unconditional `return true` which allowed any
// site to open authenticated WebSockets if a logged-in user visited it —
// classic CSWSH (Cross-Site WebSocket Hijacking).
func WSCheckOrigin(cfg *config.Config) func(r *http.Request) bool {
	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		host := r.Host
		if origin == "http://"+host || origin == "https://"+host {
			return true
		}
		if cfg != nil && cfg.DevMode {
			if origin == "http://localhost:5173" || origin == "http://127.0.0.1:5173" {
				return true
			}
		}
		return false
	}
}

// WSUpgrader builds a gorilla/websocket Upgrader wired to WSCheckOrigin.
func WSUpgrader(cfg *config.Config) websocket.Upgrader {
	return websocket.Upgrader{
		CheckOrigin:     WSCheckOrigin(cfg),
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
	}
}

// ExtractWSToken returns the JWT carried by a WebSocket handshake. Browsers
// can't set Authorization on ws:// connections, so callers support either:
//
//  1. Sec-WebSocket-Protocol: "bearer, <token>" — preferred; not logged
//     in proxy access logs, not stored in browser history.
//  2. ?token=<token> — legacy/fallback; kept for the terminal until
//     the frontend migrates. Appears in server & proxy access logs.
//
// When the subprotocol form is used, the caller should echo "bearer" back
// via websocket.Upgrader.Upgrade responseHeader["Sec-WebSocket-Protocol"]
// so RFC 6455 subprotocol negotiation succeeds.
func ExtractWSToken(r *http.Request) (token string, viaSubprotocol bool) {
	if sp := r.Header.Get("Sec-WebSocket-Protocol"); sp != "" {
		parts := strings.Split(sp, ",")
		if len(parts) >= 2 && strings.TrimSpace(parts[0]) == "bearer" {
			return strings.TrimSpace(parts[1]), true
		}
	}
	return r.URL.Query().Get("token"), false
}

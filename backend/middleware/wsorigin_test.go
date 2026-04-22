package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/serverhub/serverhub/config"
)

func TestExtractWSToken(t *testing.T) {
	cases := []struct {
		name       string
		header     string
		query      string
		wantToken  string
		wantSubpro bool
	}{
		{"subprotocol bearer", "bearer, abc.def.ghi", "", "abc.def.ghi", true},
		{"subprotocol whitespace", "bearer  ,  tok123  ", "", "tok123", true},
		{"non-bearer subprotocol falls back", "graphql-ws", "qtok", "qtok", false},
		{"empty subprotocol uses query", "", "qtok", "qtok", false},
		{"both → subprotocol wins", "bearer, sptok", "qtok", "sptok", true},
		{"nothing", "", "", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/ws?token="+c.query, nil)
			if c.header != "" {
				r.Header.Set("Sec-WebSocket-Protocol", c.header)
			}
			tok, sub := ExtractWSToken(r)
			if tok != c.wantToken || sub != c.wantSubpro {
				t.Errorf("got (%q,%v), want (%q,%v)", tok, sub, c.wantToken, c.wantSubpro)
			}
		})
	}
}

func TestWSCheckOrigin(t *testing.T) {
	cfg := &config.Config{}
	check := WSCheckOrigin(cfg)
	mk := func(origin, host string) *http.Request {
		r := httptest.NewRequest("GET", "/", nil)
		r.Host = host
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		return r
	}
	cases := []struct {
		name   string
		origin string
		host   string
		ok     bool
	}{
		{"missing origin allowed", "", "panel.example.com", true},
		{"same-origin http", "http://panel.example.com", "panel.example.com", true},
		{"same-origin https", "https://panel.example.com", "panel.example.com", true},
		{"cross-origin rejected", "https://evil.com", "panel.example.com", false},
		{"localhost cross rejected without dev", "http://localhost:5173", "panel.example.com", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := check(mk(c.origin, c.host)); got != c.ok {
				t.Errorf("got %v, want %v", got, c.ok)
			}
		})
	}

	devCfg := &config.Config{DevMode: true}
	devCheck := WSCheckOrigin(devCfg)
	if !devCheck(mk("http://localhost:5173", "panel.example.com")) {
		t.Error("dev mode should allow localhost:5173 origin")
	}
}

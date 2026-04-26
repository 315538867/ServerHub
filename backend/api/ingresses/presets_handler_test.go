package ingresses

import (
	"net/http"
	"strings"
	"testing"
)

// presets/render: 三类 kind 的 happy-path + 非法 kind + 非法参数。
//
// 这一层只走 HTTP → handler → nginxpresets,本身不触 DB。setup() 已经
// 在 RegisterRoutes 里挂好这条路由(经由 RegisterPresetRoutes)。
func TestPresetsRender_RateLimit(t *testing.T) {
	r, _ := setup(t)
	w, body := do(t, r, http.MethodPost, "/ingresses/presets/render", map[string]any{
		"kind":   "ratelimit",
		"params": map[string]any{"max_body_kb": 10240, "rate_kbs": 200},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%v", w.Code, body)
	}
	data, _ := body["data"].(map[string]any)
	extra, _ := data["extra"].(string)
	if !strings.Contains(extra, "client_max_body_size 10240k;") {
		t.Errorf("missing client_max_body_size: %s", extra)
	}
	if !strings.Contains(extra, "limit_rate 200k;") {
		t.Errorf("missing limit_rate: %s", extra)
	}
}

func TestPresetsRender_Cache(t *testing.T) {
	r, _ := setup(t)
	w, body := do(t, r, http.MethodPost, "/ingresses/presets/render", map[string]any{
		"kind":   "cache",
		"params": map[string]any{"zone_name": "edge_cache", "valid_200_mins": 5, "use_stale": true},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%v", w.Code, body)
	}
	data, _ := body["data"].(map[string]any)
	extra, _ := data["extra"].(string)
	for _, w := range []string{"proxy_cache edge_cache;", "proxy_cache_valid 200 5m;", "proxy_cache_use_stale"} {
		if !strings.Contains(extra, w) {
			t.Errorf("missing %q in: %s", w, extra)
		}
	}
}

func TestPresetsRender_Security(t *testing.T) {
	r, _ := setup(t)
	w, body := do(t, r, http.MethodPost, "/ingresses/presets/render", map[string]any{
		"kind": "security",
		"params": map[string]any{
			"frame_deny": true, "no_sniff": true, "referrer_strict": true,
			"hsts_max_age_days": 365, "hsts_include_sub": true,
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%v", w.Code, body)
	}
	data, _ := body["data"].(map[string]any)
	extra, _ := data["extra"].(string)
	for _, want := range []string{"X-Frame-Options DENY", "max-age=31536000", "includeSubDomains"} {
		if !strings.Contains(extra, want) {
			t.Errorf("missing %q in: %s", want, extra)
		}
	}
}

func TestPresetsRender_BadKind(t *testing.T) {
	r, _ := setup(t)
	w, _ := do(t, r, http.MethodPost, "/ingresses/presets/render", map[string]any{
		"kind":   "unknown",
		"params": map[string]any{},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPresetsRender_BadParams(t *testing.T) {
	r, _ := setup(t)
	// hsts 档位非法,builder 应回 400
	w, _ := do(t, r, http.MethodPost, "/ingresses/presets/render", map[string]any{
		"kind":   "security",
		"params": map[string]any{"hsts_max_age_days": 7},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for hsts=7, got %d", w.Code)
	}
}

// ── Extra 字段写库前 NginxBlock 收口的回归 ────────────────────────────────────

func TestCreate_RejectsBadExtra(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	w, _ := do(t, r, http.MethodPost, "/ingresses", map[string]any{
		"edge_server_id": edge,
		"match_kind":     "domain",
		"domain":         "demo.example.com",
		"routes": []map[string]any{
			{
				"path":     "/",
				"upstream": map[string]any{"type": "raw", "raw_url": "http://x:1"},
				// 用 } 试图提前关 location
				"extra": "limit_rate 1k; }",
			},
		},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for extra with brace, got %d", w.Code)
	}
}

func TestAddRoute_RejectsBadExtra(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	w, body := do(t, r, http.MethodPost, "/ingresses", map[string]any{
		"edge_server_id": edge,
		"match_kind":     "domain",
		"domain":         "ok.example.com",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create base ingress failed: %d %v", w.Code, body)
	}
	data, _ := body["data"].(map[string]any)
	idF, _ := data["id"].(float64)
	id := int(idF)

	// $() 命令替换序列必须被拒
	w2, _ := do(t, r, http.MethodPost, "/ingresses/"+strFromInt(id)+"/routes", map[string]any{
		"path":     "/api",
		"upstream": map[string]any{"type": "raw", "raw_url": "http://x:1"},
		"extra":    "limit_rate $(id)k;",
	})
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for extra with $(), got %d", w2.Code)
	}
}

func strFromInt(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

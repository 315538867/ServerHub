package nginxpresets

import (
	"strings"
	"testing"

	"github.com/serverhub/serverhub/pkg/safeshell"
)

// 三类预设的产物都必须能通过 safeshell.NginxBlock,这是与 IngressRoute.Extra
// 校验链路的契约——预设输出本身就是 location 块里的合法多行片段。
func mustPassNginxBlock(t *testing.T, label, out string) {
	t.Helper()
	if err := safeshell.NginxBlock(out); err != nil {
		t.Fatalf("%s output rejected by NginxBlock: %v\noutput=\n%s", label, err, out)
	}
}

func TestBuildRateLimit(t *testing.T) {
	t.Run("body+rate+after", func(t *testing.T) {
		out, err := BuildRateLimit(RateLimitOpts{MaxBodyKB: 10240, RateKBs: 200, RateAfterKB: 1024})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		want := []string{"client_max_body_size 10240k;", "limit_rate 200k;", "limit_rate_after 1024k;"}
		for _, w := range want {
			if !strings.Contains(out, w) {
				t.Errorf("missing %q in output:\n%s", w, out)
			}
		}
		mustPassNginxBlock(t, "ratelimit", out)
	})
	t.Run("仅 body 不带 rate", func(t *testing.T) {
		out, err := BuildRateLimit(RateLimitOpts{MaxBodyKB: 10})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if strings.Contains(out, "limit_rate") {
			t.Errorf("应只出 client_max_body_size, got=\n%s", out)
		}
	})
	t.Run("两项都为 0 报错", func(t *testing.T) {
		if _, err := BuildRateLimit(RateLimitOpts{}); err == nil {
			t.Error("expected err for all-zero")
		}
	})
	t.Run("超界报错", func(t *testing.T) {
		if _, err := BuildRateLimit(RateLimitOpts{RateKBs: -1}); err == nil {
			t.Error("expected err for negative rate")
		}
	})
}

func TestBuildCache(t *testing.T) {
	t.Run("zone+200+404+stale", func(t *testing.T) {
		out, err := BuildCache(CacheOpts{ZoneName: "edge_cache", Valid200Mins: 5, Valid404Mins: 1, UseStale: true})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		for _, w := range []string{"proxy_cache edge_cache;", "proxy_cache_valid 200 5m;", "proxy_cache_valid 404 1m;", "proxy_cache_use_stale"} {
			if !strings.Contains(out, w) {
				t.Errorf("missing %q in output:\n%s", w, out)
			}
		}
		mustPassNginxBlock(t, "cache", out)
	})
	t.Run("非法 zone_name", func(t *testing.T) {
		if _, err := BuildCache(CacheOpts{ZoneName: "bad name", Valid200Mins: 1}); err == nil {
			t.Error("expected err for zone with space")
		}
		if _, err := BuildCache(CacheOpts{ZoneName: "bad;name", Valid200Mins: 1}); err == nil {
			t.Error("expected err for zone with ;")
		}
	})
	t.Run("全 0 全 false 报错", func(t *testing.T) {
		if _, err := BuildCache(CacheOpts{ZoneName: "z"}); err == nil {
			t.Error("expected err for nothing-set")
		}
	})
}

func TestBuildSecurity(t *testing.T) {
	t.Run("frame+nosniff+referrer+hsts", func(t *testing.T) {
		out, err := BuildSecurity(SecurityOpts{
			FrameDeny: true, NoSniff: true, ReferrerStrict: true,
			HSTSMaxAgeDays: 365, HSTSIncludeSub: true,
		})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		for _, w := range []string{
			"X-Frame-Options DENY",
			"X-Content-Type-Options nosniff",
			"Referrer-Policy strict-origin-when-cross-origin",
			"Strict-Transport-Security",
			"max-age=31536000",
			"includeSubDomains",
		} {
			if !strings.Contains(out, w) {
				t.Errorf("missing %q in output:\n%s", w, out)
			}
		}
		mustPassNginxBlock(t, "security", out)
	})
	t.Run("hsts 档位非法", func(t *testing.T) {
		if _, err := BuildSecurity(SecurityOpts{HSTSMaxAgeDays: 7}); err == nil {
			t.Error("expected err for hsts=7")
		}
	})
	t.Run("一项都没启用", func(t *testing.T) {
		if _, err := BuildSecurity(SecurityOpts{}); err == nil {
			t.Error("expected err when nothing enabled")
		}
	})
}

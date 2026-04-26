package safeshell

import (
	"strings"
	"testing"
)

func TestNginxBlock_Allows(t *testing.T) {
	cases := []string{
		"",
		"limit_rate 200k;",
		"limit_rate 200k;\nclient_max_body_size 10m;",
		`add_header X-Frame-Options "DENY" always;`,
		"proxy_cache mycache;\nproxy_cache_valid 200 5m;",
	}
	for _, c := range cases {
		if err := NginxBlock(c); err != nil {
			t.Errorf("expected nil for %q, got %v", c, err)
		}
	}
}

func TestNginxBlock_Rejects(t *testing.T) {
	cases := map[string]string{
		"花括号-逃逸 location":       "limit_rate 1k; }\nlocation / { proxy_pass http://x;",
		"反引号":                  "limit_rate 1k; `id`",
		"美元括号":                 "limit_rate 1k; $(id)",
		"反斜杠":                  "limit_rate 1k; \\n",
		"井号注释":                 "limit_rate 1k; # 偷加注释",
		"超长":                   strings.Repeat("a", nginxBlockMaxLen+1),
	}
	for name, in := range cases {
		if err := NginxBlock(in); err == nil {
			t.Errorf("%s: expected error for %q, got nil", name, in)
		}
	}
}

func TestNginxValue_StillStrict(t *testing.T) {
	// 回归:NginxValue 仍然必须比 NginxBlock 严格(不允许 ; / \n / 引号)
	if err := NginxValue("limit_rate 200k;"); err == nil {
		t.Error("NginxValue 应仍拒绝分号")
	}
	if err := NginxValue("a\nb"); err == nil {
		t.Error("NginxValue 应仍拒绝换行")
	}
	if err := NginxValue("ok.example.com"); err != nil {
		t.Errorf("NginxValue 不应拒绝普通域名: %v", err)
	}
}

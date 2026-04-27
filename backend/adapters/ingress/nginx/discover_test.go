package nginx

import (
	"context"
	"strings"
	"testing"
)

// stubRunner 是 infra.Runner 的最小实现,按"先匹配 cat 命令再匹配 ls"的优先级
// 分派。两者子串会相互覆盖(cat 命令里也含 sites-enabled 路径),所以这里显式
// 拆开 listingOut / catOut。
type stubRunner struct {
	listingOut string
	catOut     map[string]string
}

func (s *stubRunner) Run(_ context.Context, cmd string) (string, string, error) {
	if strings.HasPrefix(strings.TrimSpace(cmd), "cat ") {
		for path, out := range s.catOut {
			if strings.Contains(cmd, path) {
				return out, "", nil
			}
		}
		return "", "", nil
	}
	if strings.Contains(cmd, "ls /etc/nginx/sites-enabled") {
		return s.listingOut, "", nil
	}
	return "", "", nil
}

func TestDiscover_LocationsAndWebSocket(t *testing.T) {
	listing := "/etc/nginx/sites-enabled/api\n"
	body := `
server {
    listen 80;
    server_name api.example.com;

    location /ws/ {
        proxy_pass http://127.0.0.1:9000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 3600s;
    }

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
`
	rn := &stubRunner{
		listingOut: listing,
		catOut:     map[string]string{"/etc/nginx/sites-enabled/api": body},
	}
	cands, err := discover(context.Background(), rn)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(cands) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(cands))
	}
	c := cands[0]
	if c.ServerName != "api.example.com" {
		t.Errorf("server_name=%q", c.ServerName)
	}
	if c.Listen == "" {
		t.Errorf("listen empty")
	}
	if len(c.Routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(c.Routes))
	}
	var ws, root int = -1, -1
	for i := range c.Routes {
		if c.Routes[i].Path == "/ws/" {
			ws = i
		}
		if c.Routes[i].Path == "/" {
			root = i
		}
	}
	if ws < 0 || root < 0 {
		t.Fatalf("missing route: %+v", c.Routes)
	}
	if c.Routes[ws].ProxyPass != "http://127.0.0.1:9000" {
		t.Errorf("/ws/ proxy_pass=%q", c.Routes[ws].ProxyPass)
	}
	if !c.Routes[ws].WebSocket {
		t.Errorf("/ws/ should be WebSocket=true")
	}
	if !strings.Contains(c.Routes[ws].Extra, "proxy_read_timeout 3600s") {
		t.Errorf("/ws/ extra 应保留 proxy_read_timeout, got: %q", c.Routes[ws].Extra)
	}
	if strings.Contains(c.Routes[ws].Extra, "proxy_pass") {
		t.Errorf("/ws/ extra 不应再含 proxy_pass: %q", c.Routes[ws].Extra)
	}
	if c.Routes[root].ProxyPass != "http://127.0.0.1:8080" {
		t.Errorf("/ proxy_pass=%q", c.Routes[root].ProxyPass)
	}
	if c.Routes[root].WebSocket {
		t.Errorf("/ should not be WebSocket")
	}
	if !strings.Contains(c.Routes[root].Extra, "Host $host") {
		t.Errorf("/ extra 应保留 Host header, got: %q", c.Routes[root].Extra)
	}
}

func TestDiscover_TopLevelProxyPass(t *testing.T) {
	listing := "/etc/nginx/sites-enabled/legacy\n"
	body := `
server {
    listen 80;
    server_name legacy.example.com;
    proxy_pass http://10.0.0.5:80;
    proxy_set_header Host $host;
}
`
	rn := &stubRunner{
		listingOut: listing,
		catOut:     map[string]string{"/etc/nginx/sites-enabled/legacy": body},
	}
	cands, _ := discover(context.Background(), rn)
	if len(cands) != 1 || len(cands[0].Routes) != 1 {
		t.Fatalf("got %+v", cands)
	}
	r := cands[0].Routes[0]
	if r.Path != "/" || r.ProxyPass != "http://10.0.0.5:80" {
		t.Errorf("route mismatch: %+v", r)
	}
	if !strings.Contains(r.Extra, "Host $host") {
		t.Errorf("extra should contain header, got: %q", r.Extra)
	}
}

func TestDiscover_SkipsStaticOnly(t *testing.T) {
	listing := "/etc/nginx/sites-enabled/static\n"
	body := `
server {
    listen 80;
    server_name static.example.com;
    root /var/www/html;
    index index.html;
    location / { try_files $uri $uri/ =404; }
}
`
	rn := &stubRunner{
		listingOut: listing,
		catOut:     map[string]string{"/etc/nginx/sites-enabled/static": body},
	}
	cands, _ := discover(context.Background(), rn)
	if len(cands) != 0 {
		t.Errorf("纯静态 vhost 不应作为反代候选返回, 得 %+v", cands)
	}
}

func TestIngressProxyFingerprint_Stable(t *testing.T) {
	fp1 := ingressProxyFingerprint("/etc/nginx/sites-enabled/api", "api.example.com")
	fp2 := ingressProxyFingerprint("/etc/nginx/sites-enabled/api", "api.example.com")
	if fp1 != fp2 || fp1 == "" {
		t.Fatalf("fingerprint not stable: %q vs %q", fp1, fp2)
	}
	fp3 := ingressProxyFingerprint("/etc/nginx/sites-enabled/api", "other.example.com")
	if fp1 == fp3 {
		t.Errorf("fingerprint must differ on server_name")
	}
}

func TestProxyPassHost(t *testing.T) {
	cases := []struct {
		in   string
		host string
		ok   bool
	}{
		{"http://127.0.0.1:8080", "127.0.0.1", true},
		{"https://10.0.0.5:8443/foo", "10.0.0.5", true},
		{"http://api.example.com/x", "api.example.com", true},
		{"http://upstream-pool", "upstream-pool", true},
		{"unix:/var/run/sock", "", false},
		{"upstream_only", "", false},
		{"", "", false},
		{"http://[::1]:8080", "", false},
	}
	for _, c := range cases {
		got, ok := ProxyPassHost(c.in)
		if ok != c.ok || got != c.host {
			t.Errorf("ProxyPassHost(%q) = (%q,%v), want (%q,%v)", c.in, got, ok, c.host, c.ok)
		}
	}
}

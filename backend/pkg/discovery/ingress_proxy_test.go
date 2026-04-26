package discovery

import (
	"errors"
	"strings"
	"testing"

	"github.com/serverhub/serverhub/pkg/runner"
)

// stubRunner 是 discovery 包内单测用的 runner.Runner 最小实现——按"先匹配 cat
// 命令再匹配 ls"的优先级分派。两者子串会相互覆盖（cat 命令里也含 sites-enabled
// 路径），所以这里显式拆开 listingOut / catOut。
type stubRunner struct {
	listingOut string            // ls /etc/nginx/sites-enabled/... 的返回
	catOut     map[string]string // 路径 → cat 输出
}

func (s *stubRunner) Run(cmd string) (string, error) {
	if strings.HasPrefix(strings.TrimSpace(cmd), "cat ") {
		for path, out := range s.catOut {
			if strings.Contains(cmd, path) {
				return out, nil
			}
		}
		return "", nil
	}
	if strings.Contains(cmd, "ls /etc/nginx/sites-enabled") {
		return s.listingOut, nil
	}
	return "", nil
}

func (s *stubRunner) NewSession() (runner.Session, error) {
	return nil, errors.New("not impl")
}
func (s *stubRunner) IsLocal() bool      { return false }
func (s *stubRunner) Capability() string { return "full" }
func (s *stubRunner) Close() error       { return nil }

func TestScanNginxIngressProxy_LocationsAndWebSocket(t *testing.T) {
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
	cands, err := ScanNginxIngressProxy(rn)
	if err != nil {
		t.Fatalf("scan: %v", err)
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
	// /ws/ 必须 WebSocket=true 且 ProxyPass 抽出
	var ws, root *IngressProxyRoute
	for i := range c.Routes {
		if c.Routes[i].Path == "/ws/" {
			ws = &c.Routes[i]
		}
		if c.Routes[i].Path == "/" {
			root = &c.Routes[i]
		}
	}
	if ws == nil || root == nil {
		t.Fatalf("missing route: %+v", c.Routes)
	}
	if ws.ProxyPass != "http://127.0.0.1:9000" {
		t.Errorf("/ws/ proxy_pass=%q", ws.ProxyPass)
	}
	if !ws.WebSocket {
		t.Errorf("/ws/ should be WebSocket=true")
	}
	if !strings.Contains(ws.Extra, "proxy_read_timeout 3600s") {
		t.Errorf("/ws/ extra 应保留 proxy_read_timeout, got: %q", ws.Extra)
	}
	if strings.Contains(ws.Extra, "proxy_pass") {
		t.Errorf("/ws/ extra 不应再含 proxy_pass: %q", ws.Extra)
	}
	if root.ProxyPass != "http://127.0.0.1:8080" {
		t.Errorf("/ proxy_pass=%q", root.ProxyPass)
	}
	if root.WebSocket {
		t.Errorf("/ should not be WebSocket")
	}
	if !strings.Contains(root.Extra, "Host $host") {
		t.Errorf("/ extra 应保留 Host header, got: %q", root.Extra)
	}
}

func TestScanNginxIngressProxy_TopLevelProxyPass(t *testing.T) {
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
	cands, _ := ScanNginxIngressProxy(rn)
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

func TestScanNginxIngressProxy_SkipsStaticOnly(t *testing.T) {
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
	cands, _ := ScanNginxIngressProxy(rn)
	if len(cands) != 0 {
		t.Errorf("纯静态 vhost 不应作为反代候选返回，得 %+v", cands)
	}
}

func TestScanNginxIngressProxy_FingerprintStable(t *testing.T) {
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
		in     string
		host   string
		ok     bool
	}{
		{"http://127.0.0.1:8080", "127.0.0.1", true},
		{"https://10.0.0.5:8443/foo", "10.0.0.5", true},
		{"http://api.example.com/x", "api.example.com", true},
		{"http://upstream-pool", "upstream-pool", true},
		{"unix:/var/run/sock", "", false},
		{"upstream_only", "", false},
		{"", "", false},
		{"http://[::1]:8080", "", false}, // IPv6 暂不支持跨机匹配
	}
	for _, c := range cases {
		got, ok := ProxyPassHost(c.in)
		if ok != c.ok || got != c.host {
			t.Errorf("ProxyPassHost(%q) = (%q,%v), want (%q,%v)", c.in, got, ok, c.host, c.ok)
		}
	}
}

package takeover

import (
	"strings"
	"testing"
)

// TestNginxRewrite_TopLevelRoot covers the simplest case: a server block with
// a top-level root pointing at oldRoot.
func TestNginxRewrite_TopLevelRoot(t *testing.T) {
	body := `server {
    listen 80;
    server_name example.com;
    root /var/www/old;
    index index.html;
}
`
	out, hits, err := NginxRewrite(body, "/var/www/old", "/opt/serverhub/apps/x/current")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if hits != 1 {
		t.Fatalf("hits = %d, want 1", hits)
	}
	if !strings.Contains(out, "root /opt/serverhub/apps/x/current;") {
		t.Fatalf("missing rewritten root in:\n%s", out)
	}
	if strings.Contains(out, "root /var/www/old;") {
		t.Fatalf("old root still present:\n%s", out)
	}
}

// TestNginxRewrite_NestedRootJoinsLocationPrefix covers the discovery-aligned
// case: a `location /lxy/ { root /var/www; }` whose effective directory is
// /var/www/lxy must match oldRoot=/var/www/lxy and be rewritten to an alias
// (not root) so nginx strips the /lxy/ prefix instead of appending it to
// newRoot — newRoot holds the contents of /var/www/lxy flat, not under a
// /lxy/ subdir.
func TestNginxRewrite_NestedRootJoinsLocationPrefix(t *testing.T) {
	body := `server {
    listen 80;
    server_name foo.com;
    location ^~ /lxy/ {
        root /var/www;
        try_files $uri $uri/ /lxy/index.html;
    }
}
`
	out, hits, err := NginxRewrite(body, "/var/www/lxy", "/opt/serverhub/apps/lxy/current")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if hits != 1 {
		t.Fatalf("hits = %d, want 1", hits)
	}
	if !strings.Contains(out, "alias /opt/serverhub/apps/lxy/current/;") {
		t.Fatalf("nested root not converted to alias:\n%s", out)
	}
	if strings.Contains(out, "root /var/www;") {
		t.Fatalf("original root still present:\n%s", out)
	}
	// try_files line must remain byte-for-byte intact
	if !strings.Contains(out, "try_files $uri $uri/ /lxy/index.html;") {
		t.Fatalf("try_files line was modified:\n%s", out)
	}
}

// TestNginxRewrite_AliasStaysAlias verifies an alias directive keeps alias
// semantics with its path swapped (alias already strips the location prefix,
// which is what we want when newRoot holds flat contents).
func TestNginxRewrite_AliasStaysAlias(t *testing.T) {
	body := `server {
    location /assets/ {
        alias /srv/assets/;
    }
}
`
	out, hits, err := NginxRewrite(body, "/srv/assets", "/opt/serverhub/apps/assets/current")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if hits != 1 {
		t.Fatalf("hits = %d, want 1", hits)
	}
	if !strings.Contains(out, "alias /opt/serverhub/apps/assets/current/;") {
		t.Fatalf("alias path not swapped:\n%s", out)
	}
	if strings.Contains(out, "alias /srv/assets/;") {
		t.Fatalf("original alias survived:\n%s", out)
	}
}

// TestNginxRewrite_NoMatchReturnsError confirms that asking to rewrite an
// oldRoot the file doesn't reference is a clear error rather than a silent
// no-op.
func TestNginxRewrite_NoMatchReturnsError(t *testing.T) {
	body := `server { root /a/b; }
`
	out, hits, err := NginxRewrite(body, "/c/d", "/x/y")
	if err == nil {
		t.Fatal("expected error for unmatched oldRoot")
	}
	if hits != 0 {
		t.Fatalf("hits = %d, want 0", hits)
	}
	// On error, body must be returned byte-equal so the caller can no-op safely
	if out != body {
		t.Fatalf("body mutated on error path")
	}
}

// TestNginxRewrite_PreservesUnrelatedDirectives makes sure SSL, proxy_pass,
// add_header etc. all flow through unchanged.
func TestNginxRewrite_PreservesUnrelatedDirectives(t *testing.T) {
	body := `server {
    listen 443 ssl;
    ssl_certificate /etc/ssl/foo.pem;
    ssl_certificate_key /etc/ssl/foo.key;
    add_header X-Frame-Options DENY;
    location /api/ {
        proxy_pass http://backend:8080;
        proxy_set_header Host $host;
    }
    location / {
        root /var/www/site;
    }
}
`
	out, _, err := NginxRewrite(body, "/var/www/site", "/opt/serverhub/apps/site/current")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	for _, line := range []string{
		"listen 443 ssl;",
		"ssl_certificate /etc/ssl/foo.pem;",
		"add_header X-Frame-Options DENY;",
		"proxy_pass http://backend:8080;",
		"proxy_set_header Host $host;",
	} {
		if !strings.Contains(out, line) {
			t.Errorf("preserved line missing: %q", line)
		}
	}
}

// TestNginxRewrite_MultipleSites rewrites both server blocks when each
// references a matching root. Verifies hit count and that both rewrites land.
func TestNginxRewrite_MultipleSites(t *testing.T) {
	body := `server {
    server_name a.com;
    root /shared/web;
}
server {
    server_name b.com;
    root /shared/web;
    listen 8080;
}
`
	out, hits, err := NginxRewrite(body, "/shared/web", "/opt/serverhub/apps/web/current")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if hits != 2 {
		t.Fatalf("hits = %d, want 2", hits)
	}
	if strings.Count(out, "root /opt/serverhub/apps/web/current;") != 2 {
		t.Fatalf("expected 2 rewritten roots:\n%s", out)
	}
}

// TestParseLocPrefix exercises the modifier matrix.
func TestParseLocPrefix(t *testing.T) {
	cases := map[string]string{
		"location /lxy/ {":     "/lxy/",
		"location ^~ /api/ {":  "/api/",
		"location = /favicon": "/favicon",
		"location ~ \\.php$ {":  "", // regex — unusable
		"location ~* gif {":    "",
		"location bad {":       "", // missing leading slash
		"server {":             "",
	}
	for in, want := range cases {
		got := parseLocPrefix(in)
		if got != want {
			t.Errorf("parseLocPrefix(%q) = %q, want %q", in, got, want)
		}
	}
}

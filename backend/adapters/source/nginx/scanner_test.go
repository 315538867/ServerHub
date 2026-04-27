package nginx

import (
	"testing"

	"github.com/serverhub/serverhub/core/source"
	"github.com/serverhub/serverhub/model"
)

func TestFingerprintByteCompatV1(t *testing.T) {
	cases := []struct {
		name string
		c    source.Candidate
		want string
	}{
		{
			name: "example.com /var/www/example",
			c: source.Candidate{
				Suggested: source.SuggestedFields{Type: model.ServiceTypeStatic, Workdir: "/var/www/example"},
				Raw:       map[string]string{"server_name": "example.com", "location_prefix": ""},
			},
			want: "5b34fe50a239ff7080bbcf7fb853ca7616d6cf24",
		},
		{
			name: "api.example.com /srv/api/dist",
			c: source.Candidate{
				Suggested: source.SuggestedFields{Type: model.ServiceTypeStatic, Workdir: "/srv/api/dist"},
				Raw:       map[string]string{"server_name": "api.example.com", "location_prefix": ""},
			},
			want: "bb5c1a8a8b335c97d7deace6bc2636ae9363310c",
		},
	}
	s := Scanner{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := s.Fingerprint(tc.c)
			if got != tc.want {
				t.Fatalf("fp=%s, want %s", got, tc.want)
			}
		})
	}
}

func TestKindRegistered(t *testing.T) {
	if got := (Scanner{}).Kind(); got != "nginx" {
		t.Fatalf("Kind=%s, want nginx", got)
	}
	if _, err := source.Default.Get("nginx"); err != nil {
		t.Fatalf("nginx not registered in source.Default: %v", err)
	}
}

func TestNginxRewriteTopLevel(t *testing.T) {
	body := `server {
    listen 80;
    server_name example.com;
    root /var/www/old;
    index index.html;
}
`
	out, hits, err := NginxRewrite(body, "/var/www/old", "/opt/serverhub/sites/example/current")
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if hits != 1 {
		t.Fatalf("hits=%d, want 1", hits)
	}
	if !contains(out, "root /opt/serverhub/sites/example/current;") {
		t.Fatalf("missing rewritten root: %s", out)
	}
}

func TestNginxRewriteNestedBecomesAlias(t *testing.T) {
	body := `server {
    listen 80;
    server_name app.test;
    location /lxy {
        root /var/www;
    }
}
`
	out, hits, err := NginxRewrite(body, "/var/www/lxy", "/opt/serverhub/sites/app/current")
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if hits != 1 {
		t.Fatalf("hits=%d, want 1", hits)
	}
	if !contains(out, "alias /opt/serverhub/sites/app/current/;") {
		t.Fatalf("expected alias rewrite, got: %s", out)
	}
}

func TestNginxRewriteMissThrows(t *testing.T) {
	body := `server { server_name x; root /something/else; }`
	_, _, err := NginxRewrite(body, "/var/www/old", "/opt/new")
	if err == nil {
		t.Fatal("expected error when no directive matches")
	}
}

func TestParseListenPort(t *testing.T) {
	cases := map[string]int{
		"":                 80,
		"80":               80,
		"443 ssl":          443,
		"[::]:8080":        8080,
		"0.0.0.0:8443 ssl": 8443,
		"unix:/run/x.sock": 80,
	}
	for in, want := range cases {
		if got := parseListenPort(in); got != want {
			t.Errorf("parseListenPort(%q)=%d, want %d", in, got, want)
		}
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

package takeover

import (
	"strings"
	"testing"
)

// Most of static.go runs IO; only parseListenPort + nginxConfBase are pure
// helpers we can unit-test. Together they cover the cases that show up in
// real nginx configs.

func TestParseListenPort(t *testing.T) {
	cases := map[string]int{
		"":                       80,
		"80":                     80,
		"443 ssl":                443,
		"443 ssl http2":          443,
		"[::]:8080":              8080,
		"0.0.0.0:8443 ssl http2": 8443,
		"unix:/run/x.sock":       80, // unparseable → default
	}
	for in, want := range cases {
		got := parseListenPort(in)
		if got != want {
			t.Errorf("parseListenPort(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestNginxConfBase(t *testing.T) {
	cases := map[string]string{
		"/etc/nginx/sites-enabled/default": "default",
		"/etc/nginx/conf.d/foo.conf":       "foo.conf",
		"plain.conf":                       "plain.conf",
	}
	for in, want := range cases {
		got := nginxConfBase(in)
		if got != want {
			t.Errorf("nginxConfBase(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSystemdSafetyGate(t *testing.T) {
	type tc struct {
		bin, work string
		refused   bool
	}
	cases := []tc{
		{"/usr/sbin/nginx", "/etc/nginx", true},      // distro nginx
		{"/usr/bin/python3", "/var/lib/x", true},     // distro python + system data
		{"/opt/myapp/bin/srv", "/opt/myapp", false},  // user-rolled
		{"/home/u/app", "/home/u/app", false},        // user-rolled in home
		{"/srv/svc/bin", "/srv/svc", false},          // /srv is fine
		{"/usr/local/bin/x", "/var/lib/x", true},     // /var/lib is system
		{"/opt/x/bin", "/etc/x", true},               // /etc work dir is system
	}
	for _, c := range cases {
		reason := systemdSafetyGate("foo.service", c.bin, c.work)
		got := reason != ""
		if got != c.refused {
			t.Errorf("safetyGate(bin=%q work=%q) refused=%v want %v (reason=%q)",
				c.bin, c.work, got, c.refused, reason)
		}
	}
}

func TestRewriteSystemdUnit(t *testing.T) {
	body := `[Unit]
Description=foo
After=network.target

[Service]
User=app
WorkingDirectory=/old/path
ExecStart=/old/path/bin/foo --flag
Restart=on-failure
Environment=FOO=bar

[Install]
WantedBy=multi-user.target
`
	out := rewriteSystemdUnit(body, "/opt/serverhub/apps/foo/current",
		"/opt/serverhub/apps/foo/bin/foo --flag")
	if !strings.Contains(out, "WorkingDirectory=/opt/serverhub/apps/foo/current") {
		t.Errorf("WorkingDirectory not rewritten:\n%s", out)
	}
	if !strings.Contains(out, "ExecStart=/opt/serverhub/apps/foo/bin/foo --flag") {
		t.Errorf("ExecStart not rewritten:\n%s", out)
	}
	// Untouched directives must survive verbatim.
	for _, want := range []string{
		"After=network.target", "User=app", "Restart=on-failure",
		"Environment=FOO=bar", "WantedBy=multi-user.target",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing preserved line: %q\n%s", want, out)
		}
	}
}

func TestStripSystemctlCatHeader(t *testing.T) {
	in := `# /etc/systemd/system/foo.service
[Unit]
Description=foo
`
	out := stripSystemctlCatHeader(in)
	if strings.Contains(out, "# /etc/systemd/system/foo.service") {
		t.Errorf("header not stripped:\n%s", out)
	}
	if !strings.HasPrefix(out, "[Unit]") {
		t.Errorf("expected to start with [Unit]:\n%s", out)
	}
}

func TestComposeNotReady(t *testing.T) {
	// compose v2 NDJSON form
	jsonOut := `{"Service":"web","State":"running"}
{"Service":"db","State":"starting"}
{"Service":"worker","State":"exited"}
`
	notReady := composeNotReady(jsonOut)
	if len(notReady) != 1 || !strings.Contains(notReady[0], "db") {
		t.Fatalf("notReady = %v, want only [db=starting]", notReady)
	}

	allOK := `{"Service":"a","State":"running"}
{"Service":"b","State":"running"}
`
	if got := composeNotReady(allOK); len(got) != 0 {
		t.Fatalf("notReady = %v, want empty", got)
	}
}

func TestUniqueBindName(t *testing.T) {
	used := map[string]int{}
	if got := uniqueBindName("config", used); got != "config" {
		t.Errorf("first call = %q, want config", got)
	}
	if got := uniqueBindName("config", used); got != "config-2" {
		t.Errorf("second call = %q, want config-2", got)
	}
	if got := uniqueBindName("config", used); got != "config-3" {
		t.Errorf("third call = %q, want config-3", got)
	}
	if got := uniqueBindName("", used); got != "data" {
		t.Errorf("empty base = %q, want data", got)
	}
}

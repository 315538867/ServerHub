package systemd

import (
	"testing"

	"github.com/serverhub/serverhub/core/source"
)

func TestFingerprintByteCompatV1(t *testing.T) {
	cases := []struct {
		name string
		c    source.Candidate
		want string
	}{
		{
			name: "simple unit",
			c: source.Candidate{
				Kind:     Kind,
				SourceID: "myapp.service",
				Raw:      map[string]string{"exec_start": "/opt/myapp/bin/server"},
			},
			// printf '%s' "systemd|myapp.service|/opt/myapp/bin/server" | shasum -a 1
			want: "db79236fd646b7b9e9a8a27363c33db8eac51ab8",
		},
		{
			name: "exec with args",
			c: source.Candidate{
				Kind:     Kind,
				SourceID: "api.service",
				Raw:      map[string]string{"exec_start": "/usr/local/bin/api --port 8080"},
			},
			// printf '%s' "systemd|api.service|/usr/local/bin/api --port 8080" | shasum -a 1
			want: "762ea6a75bcaef9fbf27c4ac15dfdc561e113dc3",
		},
	}
	s := Scanner{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := s.Fingerprint(tc.c)
			if got != tc.want {
				t.Errorf("fingerprint drift: got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestKindRegistered(t *testing.T) {
	if got := (Scanner{}).Kind(); got != "systemd" {
		t.Errorf("Kind=%q, want systemd", got)
	}
	got, err := source.Default.Get("systemd")
	if err != nil {
		t.Fatalf("source.Default.Get(systemd) failed: %v", err)
	}
	if got.Kind() != "systemd" {
		t.Errorf("registered scanner Kind=%q", got.Kind())
	}
}

func TestShouldSkipUnit(t *testing.T) {
	skip := []string{"systemd-resolved.service", "ssh.service", "user@1000.service", "snapd.service"}
	keep := []string{"myapp.service", "api-gateway.service", "worker.service"}
	for _, u := range skip {
		if !shouldSkipUnit(u) {
			t.Errorf("expected skip: %s", u)
		}
	}
	for _, u := range keep {
		if shouldSkipUnit(u) {
			t.Errorf("expected keep: %s", u)
		}
	}
}

func TestSystemdSafetyGate(t *testing.T) {
	if reason := systemdSafetyGate("nginx.service", "/usr/sbin/nginx", "/var/www"); reason == "" {
		t.Error("expected reject for /usr/sbin/ binary")
	}
	if reason := systemdSafetyGate("myapp.service", "/opt/myapp/bin/server", "/etc/myapp"); reason == "" {
		t.Error("expected reject for /etc/ workdir")
	}
	if reason := systemdSafetyGate("myapp.service", "/opt/myapp/bin/server", "/opt/myapp"); reason != "" {
		t.Errorf("expected accept user-rolled service, got: %s", reason)
	}
}

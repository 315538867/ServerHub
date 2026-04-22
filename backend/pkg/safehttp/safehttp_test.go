package safehttp

import (
	"errors"
	"net"
	"strings"
	"testing"
)

func TestIsBlockedIP(t *testing.T) {
	cases := []struct {
		ip      string
		blocked bool
	}{
		{"127.0.0.1", true},
		{"::1", true},
		{"10.1.2.3", true},
		{"192.168.0.5", true},
		{"172.16.0.1", true},
		{"169.254.169.254", true}, // EC2 metadata link-local
		{"100.64.0.1", true},      // CGNAT
		{"100.127.255.255", true}, // CGNAT upper bound
		{"0.0.0.0", true},
		{"224.0.0.1", true}, // multicast
		{"8.8.8.8", false},
		{"100.63.255.255", false}, // just below CGNAT
		{"100.128.0.0", false},    // just above CGNAT
		{"2606:4700:4700::1111", false},
	}
	for _, c := range cases {
		got := IsBlockedIP(net.ParseIP(c.ip))
		if got != c.blocked {
			t.Errorf("IsBlockedIP(%s) = %v, want %v", c.ip, got, c.blocked)
		}
	}
	if !IsBlockedIP(nil) {
		t.Error("IsBlockedIP(nil) should be true (fail-closed)")
	}
}

func TestValidateOutboundURL(t *testing.T) {
	cases := []struct {
		url     string
		wantErr bool
	}{
		{"https://example.com/x", false},
		{"http://example.com", false},
		{"https://1.1.1.1/", false},
		{"file:///etc/passwd", true},
		{"gopher://x", true},
		{"https://", true},
		{"https://127.0.0.1/", true},
		{"https://10.0.0.1/", true},
		{"https://[::1]/", true},
		{"https://169.254.169.254/latest/meta-data/", true},
		{":not a url", true},
	}
	for _, c := range cases {
		err := ValidateOutboundURL(c.url)
		if (err != nil) != c.wantErr {
			t.Errorf("ValidateOutboundURL(%q) err=%v wantErr=%v", c.url, err, c.wantErr)
		}
	}
}

func TestControlRejectsBlockedIP(t *testing.T) {
	if err := control("tcp", "10.0.0.1:80", nil); !errors.Is(err, ErrBlockedAddress) {
		t.Errorf("control should reject 10.0.0.1, got %v", err)
	}
	if err := control("tcp", "8.8.8.8:443", nil); err != nil {
		t.Errorf("control should accept 8.8.8.8, got %v", err)
	}
	if err := control("tcp", "no-port", nil); err == nil || !strings.Contains(err.Error(), "missing port") {
		t.Errorf("control should reject malformed addr, got %v", err)
	}
}

// Package safehttp provides an HTTP client hardened against SSRF: only http
// and https are allowed; the resolved peer must be a public unicast address.
// Both the URL and the actual dialed address are validated, so a name that
// resolves to a private IP (or rebinds between resolution and connect) is
// rejected at the syscall layer.
package safehttp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"syscall"
	"time"
)

var ErrBlockedAddress = errors.New("blocked address")

// IsBlockedIP returns true for loopback, private, link-local, multicast,
// unspecified and CGNAT addresses — anything that should not be reachable
// from a public webhook.
func IsBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() ||
		(len(ip) == 4 && ip[0] == 100 && ip[1] >= 64 && ip[1] <= 127)
}

// ValidateOutboundURL checks scheme and (for literal-IP hosts) the address.
// Hostnames are validated by the dialer Control hook on every resolved IP.
func ValidateOutboundURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("scheme not allowed: %s", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return errors.New("missing host")
	}
	if ip := net.ParseIP(host); ip != nil && IsBlockedIP(ip) {
		return ErrBlockedAddress
	}
	return nil
}

func control(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	if IsBlockedIP(net.ParseIP(host)) {
		return ErrBlockedAddress
	}
	return nil
}

// Client returns an *http.Client that refuses to connect to private/loopback
// targets and re-validates every redirect hop (capped at 3).
func Client(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 0,
		Control:   control,
	}
	tr := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, addr)
		},
		DisableKeepAlives:     true,
		MaxIdleConns:          1,
		IdleConnTimeout:       1 * time.Second,
		ResponseHeaderTimeout: timeout,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: tr,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("too many redirects")
			}
			return ValidateOutboundURL(req.URL.String())
		},
	}
}

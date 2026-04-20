package sshpool

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	gossh "golang.org/x/crypto/ssh"
)

type entry struct {
	client   *gossh.Client
	lastUsed time.Time
}

var (
	mu          sync.Map // key: serverID uint → *entry
	clientToID  sync.Map // key: *gossh.Client → uint (reverse map for self-healing)
)

const idleTimeout = 30 * time.Minute

func init() {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			cutoff := time.Now().Add(-idleTimeout)
			mu.Range(func(k, v any) bool {
				e := v.(*entry)
				if e.lastUsed.Before(cutoff) {
					clientToID.Delete(e.client)
					e.client.Close()
					mu.Delete(k)
				}
				return true
			})
		}
	}()
}

// buildClientConfig constructs an SSH client config from credentials.
func buildClientConfig(user, authType, cred string) (*gossh.ClientConfig, error) {
	var authMethods []gossh.AuthMethod
	switch authType {
	case "key":
		signer, err := gossh.ParsePrivateKey([]byte(cred))
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		authMethods = append(authMethods, gossh.PublicKeys(signer))
	default:
		authMethods = append(authMethods, gossh.Password(cred))
	}
	return &gossh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint:gosec
		Timeout:         10 * time.Second,
	}, nil
}

// Dial creates a fresh SSH client without registering it in the pool.
// The caller is responsible for calling client.Close() when done.
// Use this for long-lived dedicated sessions (e.g. interactive terminals)
// to avoid exhausting the pool client's MaxSessions budget.
func Dial(host string, port int, user, authType, cred string) (*gossh.Client, error) {
	cfg, err := buildClientConfig(user, authType, cred)
	if err != nil {
		return nil, err
	}
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	return gossh.Dial("tcp", addr, cfg)
}

func Connect(id uint, host string, port int, user, authType, cred string) (*gossh.Client, error) {
	// reuse live connection
	if v, ok := mu.Load(id); ok {
		e := v.(*entry)
		if _, _, err := e.client.SendRequest("keepalive@openssh.com", true, nil); err == nil {
			e.lastUsed = time.Now()
			return e.client, nil
		}
		clientToID.Delete(e.client)
		e.client.Close()
		mu.Delete(id)
	}

	cfg, err := buildClientConfig(user, authType, cred)
	if err != nil {
		return nil, err
	}

	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	client, err := gossh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}

	mu.Store(id, &entry{client: client, lastUsed: time.Now()})
	clientToID.Store(client, id)
	return client, nil
}

func Remove(id uint) {
	if v, ok := mu.Load(id); ok {
		e := v.(*entry)
		clientToID.Delete(e.client)
		e.client.Close()
		mu.Delete(id)
	}
}

// isSessionUnrecoverable reports whether an error from NewSession indicates
// the underlying SSH connection is in a state where new channels can't be
// opened (typically MaxSessions exhausted or peer-side limits). When true,
// the pooled client should be evicted so the next call gets a fresh dial.
func isSessionUnrecoverable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "open failed") ||
		strings.Contains(msg, "administratively prohibited") ||
		strings.Contains(msg, "resource shortage") ||
		strings.Contains(msg, "use of closed network connection") ||
		strings.Contains(msg, "EOF")
}

// evictByClient finds the pool entry for the given client and removes it.
func evictByClient(client *gossh.Client) {
	if v, ok := clientToID.Load(client); ok {
		Remove(v.(uint))
	}
}

func Run(client *gossh.Client, cmd string) (string, error) {
	sess, err := client.NewSession()
	if err != nil {
		// Self-heal: evict pool entry so next Connect() opens a fresh TCP
		// connection with a fresh MaxSessions budget. The next caller (or a
		// retry of this caller) will succeed.
		if isSessionUnrecoverable(err) {
			evictByClient(client)
		}
		return "", fmt.Errorf("new session: %w", err)
	}
	defer sess.Close()
	out, err := sess.CombinedOutput(cmd)
	return string(out), err
}

package sshpool

import (
	"fmt"
	"net"
	"sync"
	"time"

	gossh "golang.org/x/crypto/ssh"
)

type entry struct {
	client   *gossh.Client
	lastUsed time.Time
}

var mu sync.Map // key: serverID uint → *entry

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
					e.client.Close()
					mu.Delete(k)
				}
				return true
			})
		}
	}()
}

func Connect(id uint, host string, port int, user, authType, cred string) (*gossh.Client, error) {
	// reuse live connection
	if v, ok := mu.Load(id); ok {
		e := v.(*entry)
		if _, _, err := e.client.SendRequest("keepalive@openssh.com", true, nil); err == nil {
			e.lastUsed = time.Now()
			return e.client, nil
		}
		e.client.Close()
		mu.Delete(id)
	}

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

	cfg := &gossh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint:gosec
		Timeout:         10 * time.Second,
	}

	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	client, err := gossh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}

	mu.Store(id, &entry{client: client, lastUsed: time.Now()})
	return client, nil
}

func Remove(id uint) {
	if v, ok := mu.Load(id); ok {
		v.(*entry).client.Close()
		mu.Delete(id)
	}
}

func Run(client *gossh.Client, cmd string) (string, error) {
	sess, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("new session: %w", err)
	}
	defer sess.Close()
	out, err := sess.CombinedOutput(cmd)
	return string(out), err
}

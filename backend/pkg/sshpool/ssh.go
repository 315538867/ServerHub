package sshpool

import (
	"errors"
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
	mu         sync.Map // key: serverID uint → *entry
	clientToID sync.Map // key: *gossh.Client → uint (reverse map for self-healing)
)

// HostKeyStore is the integration point used by Connect/Dial to enforce TOFU
// host-key pinning. Implementations are typically backed by the Server model.
//
//   - Get returns the pinned fingerprint (e.g. "SHA256:abc...") for serverID
//     and ok=false if nothing is pinned yet.
//   - Set persists the freshly observed fingerprint after a first successful
//     connection (TOFU path). Errors are surfaced — failure to persist must
//     abort the connect, otherwise the pin would be silently lost.
type HostKeyStore interface {
	Get(serverID uint) (fingerprint string, ok bool)
	Set(serverID uint, fingerprint string) error
}

// hostKeyStore is set once at startup via SetHostKeyStore. Until then,
// connections are rejected when no fingerprint can be checked — refusing to
// connect is safer than the previous InsecureIgnoreHostKey behaviour.
var hostKeyStore HostKeyStore

func SetHostKeyStore(s HostKeyStore) { hostKeyStore = s }

// ErrHostKeyMismatch indicates the server presented a key that does not match
// the previously pinned fingerprint — possible MITM or genuine key rotation.
var ErrHostKeyMismatch = errors.New("ssh host key mismatch")

// OnHostKeyMismatch, if non-nil, is invoked when a server's presented key
// does not match the pinned fingerprint. Wired by main.go to push a
// security audit event without making this package depend on auditq.
var OnHostKeyMismatch func(serverID uint, hostname, pinned, got string)

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
					sessSem.Delete(e.client)
					e.client.Close()
					mu.Delete(k)
				}
				return true
			})
		}
	}()
}

// hostKeyCallback returns an ssh.HostKeyCallback that:
//   - rejects the connection if no HostKeyStore is configured (fail-closed),
//   - on first connect for serverID, records the fingerprint via Set (TOFU),
//   - on subsequent connects, requires the fingerprint to match.
//
// serverID==0 means "no pinning" (e.g. ad-hoc Dial without an associated
// Server row); we still require a store but accept any key without pinning.
func hostKeyCallback(serverID uint) gossh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key gossh.PublicKey) error {
		store := hostKeyStore
		if store == nil {
			return errors.New("ssh host key store not initialised; refusing to connect")
		}
		fp := gossh.FingerprintSHA256(key)
		if serverID == 0 {
			return nil
		}
		if pinned, ok := store.Get(serverID); ok && pinned != "" {
			if pinned != fp {
				if OnHostKeyMismatch != nil {
					OnHostKeyMismatch(serverID, hostname, pinned, fp)
				}
				return fmt.Errorf("%w: server=%d host=%s pinned=%s got=%s",
					ErrHostKeyMismatch, serverID, hostname, pinned, fp)
			}
			return nil
		}
		return store.Set(serverID, fp)
	}
}

// buildClientConfig constructs an SSH client config from credentials.
// serverID is used to look up / store the pinned host-key fingerprint.
func buildClientConfig(serverID uint, user, authType, cred string) (*gossh.ClientConfig, error) {
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
		HostKeyCallback: hostKeyCallback(serverID),
		Timeout:         10 * time.Second,
	}, nil
}

// Dial creates a fresh SSH client without registering it in the pool.
// The caller is responsible for calling client.Close() when done.
// Use this for long-lived dedicated sessions (e.g. interactive terminals)
// to avoid exhausting the pool client's MaxSessions budget.
func Dial(host string, port int, user, authType, cred string) (*gossh.Client, error) {
	return DialPinned(0, host, port, user, authType, cred)
}

// DialPinned is the host-key-pinning variant of Dial; pass the Server row's
// ID so the host key is checked / stored. Use Dial (serverID=0) only for
// truly ephemeral dials with no persistent identity.
func DialPinned(serverID uint, host string, port int, user, authType, cred string) (*gossh.Client, error) {
	cfg, err := buildClientConfig(serverID, user, authType, cred)
	if err != nil {
		return nil, err
	}
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	return gossh.Dial("tcp", addr, cfg)
}

func Connect(id uint, host string, port int, user, authType, cred string) (*gossh.Client, error) {
	if v, ok := mu.Load(id); ok {
		e := v.(*entry)
		if _, _, err := e.client.SendRequest("keepalive@openssh.com", true, nil); err == nil {
			e.lastUsed = time.Now()
			return e.client, nil
		}
		clientToID.Delete(e.client)
		sessSem.Delete(e.client)
		e.client.Close()
		mu.Delete(id)
	}

	cfg, err := buildClientConfig(id, user, authType, cred)
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
		sessSem.Delete(e.client)
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
	sessSem.Delete(client)
}

// maxConcurrentSessions caps how many sessions Run will open against a single
// pooled client at once. OpenSSH's default MaxSessions is 10; staying below it
// avoids the "open failed: administratively prohibited" race where many
// goroutines call NewSession concurrently and exhaust the budget — even
// though each session is short-lived.
const maxConcurrentSessions = 8

// sessSem maps *gossh.Client → chan struct{} (a counting semaphore).
// Lazily allocated on first Run for each client; cleared on eviction.
var sessSem sync.Map

func acquireSession(client *gossh.Client) chan struct{} {
	if v, ok := sessSem.Load(client); ok {
		return v.(chan struct{})
	}
	ch := make(chan struct{}, maxConcurrentSessions)
	actual, _ := sessSem.LoadOrStore(client, ch)
	return actual.(chan struct{})
}

func Run(client *gossh.Client, cmd string) (string, error) {
	sem := acquireSession(client)
	sem <- struct{}{}
	defer func() { <-sem }()

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

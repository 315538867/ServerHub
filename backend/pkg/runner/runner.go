// Package runner abstracts command execution over either an SSH connection
// (for remote managed servers) or local os/exec (for the host running
// ServerHub itself, marked as Type="local"). All API handlers should obtain
// a Runner via For()/ForDedicated() and call Run/NewSession instead of
// touching *gossh.Client directly.
package runner

import (
	"errors"
	"fmt"
	"io"
	"os/exec"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/sshpool"
	"github.com/serverhub/serverhub/pkg/sysinfo"
	gossh "golang.org/x/crypto/ssh"
)

// Session is the streaming-execution surface needed by wsstream and other
// callers. Implementations: sshSession (wraps *gossh.Session), localSession
// (wraps *exec.Cmd).
type Session interface {
	// StdoutPipe must be called before Start.
	StdoutPipe() (io.Reader, error)
	Start(cmd string) error
	Wait() error
	// Kill terminates the running command (best-effort).
	Kill() error
	Close() error
}

// Runner executes commands against a server.
type Runner interface {
	// Run executes cmd and returns combined stdout+stderr.
	Run(cmd string) (string, error)
	// NewSession returns a fresh streaming session. Caller must Close it.
	NewSession() (Session, error)
	// IsLocal reports whether commands run via os/exec on this host.
	IsLocal() bool
	// Capability reports what kind of host control this runner offers.
	// SSH runners are always "full". Local runners depend on the boot-time
	// probe (see sysinfo.LocalCapability) — "full" when the binary can drive
	// the host directly (bare metal or container with --pid=host + /host),
	// "docker" when only the docker socket is reachable.
	Capability() string
	// Close releases any dedicated underlying connection (no-op for pooled).
	Close() error
}

// For returns a pooled Runner suitable for one-shot commands.
func For(s *domain.Server, cfg *config.Config) (Runner, error) {
	if s == nil {
		return nil, errors.New("nil server")
	}
	if s.Type == "local" {
		return newLocalRunner(s.Capability), nil
	}
	cred, err := decryptCred(s, cfg)
	if err != nil {
		return nil, err
	}
	port := s.Port
	if port == 0 {
		port = 22
	}
	cli, err := sshpool.Connect(s.ID, s.Host, port, s.Username, s.AuthType, cred)
	if err != nil {
		return nil, err
	}
	return &sshRunner{client: cli}, nil
}

// ForDedicated returns a Runner whose underlying connection is NOT shared
// with the pool. Caller MUST Close. Use for long-lived sessions (terminal,
// streaming logs, certbot) to avoid contending on MaxSessions.
func ForDedicated(s *domain.Server, cfg *config.Config) (Runner, error) {
	if s == nil {
		return nil, errors.New("nil server")
	}
	if s.Type == "local" {
		return newLocalRunner(s.Capability), nil
	}
	cred, err := decryptCred(s, cfg)
	if err != nil {
		return nil, err
	}
	port := s.Port
	if port == 0 {
		port = 22
	}
	cli, err := sshpool.Dial(s.Host, port, s.Username, s.AuthType, cred)
	if err != nil {
		return nil, err
	}
	return &sshRunner{client: cli, dedicated: true}, nil
}

func decryptCred(s *domain.Server, cfg *config.Config) (string, error) {
	switch s.AuthType {
	case "key":
		if s.PrivateKey == "" {
			return "", errors.New("private key empty")
		}
		return crypto.Decrypt(s.PrivateKey, cfg.Security.AESKey)
	case "password":
		if s.Password == "" {
			return "", errors.New("password empty")
		}
		return crypto.Decrypt(s.Password, cfg.Security.AESKey)
	default:
		return "", fmt.Errorf("unsupported auth type %q", s.AuthType)
	}
}

// SSHClient extracts the underlying SSH client from a Runner (if any).
// Returns nil for local runners. Used by code paths that still need raw
// gossh access (e.g. SFTP layering).
func SSHClient(r Runner) *gossh.Client {
	if sr, ok := r.(*sshRunner); ok {
		return sr.client
	}
	return nil
}

// WrapSSH adapts an existing *gossh.Client into a Runner without altering
// pool ownership. Transitional helper for code that already holds a client.
func WrapSSH(client *gossh.Client) Runner {
	return &sshRunner{client: client}
}

// ─── ssh impl ────────────────────────────────────────────────────────────

type sshRunner struct {
	client    *gossh.Client
	dedicated bool
}

func (r *sshRunner) Run(cmd string) (string, error) {
	return sshpool.Run(r.client, cmd)
}

func (r *sshRunner) NewSession() (Session, error) {
	s, err := r.client.NewSession()
	if err != nil {
		return nil, err
	}
	return &sshSession{s: s}, nil
}

func (r *sshRunner) IsLocal() bool     { return false }
func (r *sshRunner) Capability() string { return sysinfo.CapFull }

func (r *sshRunner) Close() error {
	if r.dedicated && r.client != nil {
		return r.client.Close()
	}
	return nil
}

type sshSession struct {
	s *gossh.Session
}

func (s *sshSession) StdoutPipe() (io.Reader, error) {
	s.s.Stderr = nil
	return s.s.StdoutPipe()
}

func (s *sshSession) Start(cmd string) error { return s.s.Start(cmd) }
func (s *sshSession) Wait() error            { return s.s.Wait() }
func (s *sshSession) Kill() error            { return s.s.Signal(gossh.SIGTERM) }
func (s *sshSession) Close() error           { return s.s.Close() }

// ─── local impl ──────────────────────────────────────────────────────────

// newLocalRunner picks between "direct bash -lc" (bare-metal or docker-only
// capability) and "bash -lc wrapped by nsenter into PID 1 namespaces"
// (containerized with full capability: --pid=host + /host + CAP_SYS_ADMIN).
//
// Legacy rows created before the Capability column (empty string) default to
// "full" so a freshly upgraded bare-metal install keeps working without a
// manual DB edit.
func newLocalRunner(capability string) Runner {
	c := capability
	if c == "" {
		c = sysinfo.CapFull
	}
	useNsenter := c == sysinfo.CapFull && sysinfo.IsContainerized()
	return localRunner{capability: c, useNsenter: useNsenter}
}

type localRunner struct {
	capability string
	useNsenter bool
}

// wrap applies the nsenter prefix for host-namespace execution when the
// runner was created inside a container with full capability. nsenter enters
// the mount/uts/ipc/net/pid namespaces of PID 1 (the host init), giving the
// child process the same view as if it were running on the host directly.
// CAP_SYS_ADMIN is required for this and must be granted via --cap-add.
func (r localRunner) wrap(cmd string) (string, []string) {
	if r.useNsenter {
		return "nsenter", []string{
			"-t", "1", "-m", "-u", "-i", "-n", "-p", "--",
			"bash", "-lc", cmd,
		}
	}
	return "bash", []string{"-lc", cmd}
}

func (r localRunner) Run(cmd string) (string, error) {
	name, args := r.wrap(cmd)
	out, err := exec.Command(name, args...).CombinedOutput()
	return string(out), err
}

func (r localRunner) NewSession() (Session, error) {
	return &localSession{runner: r}, nil
}
func (r localRunner) IsLocal() bool       { return true }
func (r localRunner) Capability() string  { return r.capability }
func (r localRunner) Close() error        { return nil }

type localSession struct {
	runner localRunner
	cmd    *exec.Cmd
	stdout io.ReadCloser
	piped  bool
}

func (s *localSession) StdoutPipe() (io.Reader, error) {
	if s.cmd != nil {
		return nil, errors.New("StdoutPipe must be called before Start")
	}
	// Defer command construction until Start, but mark that piping is requested.
	s.piped = true
	return readerFunc(func(p []byte) (int, error) {
		if s.stdout == nil {
			return 0, io.EOF
		}
		return s.stdout.Read(p)
	}), nil
}

func (s *localSession) Start(cmd string) error {
	name, args := s.runner.wrap(cmd)
	s.cmd = exec.Command(name, args...)
	if s.piped {
		pipe, err := s.cmd.StdoutPipe()
		if err != nil {
			return err
		}
		s.stdout = pipe
		s.cmd.Stderr = nil
	}
	return s.cmd.Start()
}

func (s *localSession) Wait() error {
	if s.cmd == nil {
		return nil
	}
	return s.cmd.Wait()
}

func (s *localSession) Kill() error {
	if s.cmd == nil || s.cmd.Process == nil {
		return nil
	}
	return s.cmd.Process.Kill()
}

func (s *localSession) Close() error {
	if s.stdout != nil {
		_ = s.stdout.Close()
	}
	return nil
}

// readerFunc adapts a read function into io.Reader.
type readerFunc func(p []byte) (int, error)

func (f readerFunc) Read(p []byte) (int, error) { return f(p) }

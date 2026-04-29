// Package fsclient abstracts file-system operations over either an SFTP
// connection (for remote managed servers) or the local OS (for the host
// running ServerHub itself, marked Type="local"). API handlers should use
// For() to obtain a Client and avoid touching *sftp.Client directly.
package fsclient

import (
	"errors"
	"io"
	"io/fs"
	"os"

	"github.com/pkg/sftp"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/pkg/runner"
	"github.com/serverhub/serverhub/pkg/sftppool"
)

// File abstracts a remote/local file handle.
type File interface {
	io.ReadWriteCloser
	Stat() (fs.FileInfo, error)
}

// Client unifies SFTP and local FS operations.
type Client interface {
	ReadDir(path string) ([]fs.FileInfo, error)
	Open(path string) (File, error)
	Create(path string) (File, error)
	MkdirAll(path string) error
	Rename(oldPath, newPath string) error
	Remove(path string) error
	Stat(path string) (fs.FileInfo, error)
	Close() error // pooled clients: no-op
}

// For returns an FS client for the given server. Reuses pooled SFTP for SSH
// servers; uses os for local. Caller need not Close pooled clients (no-op).
func For(s *domain.Server, cfg *config.Config) (Client, error) {
	if s == nil {
		return nil, errors.New("nil server")
	}
	if s.Type == "local" {
		return localClient{}, nil
	}
	rn, err := runner.For(s, cfg)
	if err != nil {
		return nil, err
	}
	cli := runner.SSHClient(rn)
	if cli == nil {
		return nil, errors.New("no ssh client for non-local server")
	}
	sc, err := sftppool.Get(s.ID, cli)
	if err != nil {
		return nil, err
	}
	return &sftpClient{c: sc}, nil
}

// ─── sftp impl (pooled, do not Close) ────────────────────────────────────

type sftpClient struct {
	c *sftp.Client
}

func (s *sftpClient) ReadDir(p string) ([]fs.FileInfo, error) { return s.c.ReadDir(p) }
func (s *sftpClient) Open(p string) (File, error)             { return s.c.Open(p) }
func (s *sftpClient) Create(p string) (File, error)           { return s.c.Create(p) }
func (s *sftpClient) MkdirAll(p string) error                 { return s.c.MkdirAll(p) }
func (s *sftpClient) Rename(o, n string) error                { return s.c.Rename(o, n) }
func (s *sftpClient) Remove(p string) error                   { return s.c.Remove(p) }
func (s *sftpClient) Stat(p string) (fs.FileInfo, error)      { return s.c.Stat(p) }
func (s *sftpClient) Close() error                            { return nil }

// ─── local impl ──────────────────────────────────────────────────────────

type localClient struct{}

func (localClient) ReadDir(p string) ([]fs.FileInfo, error) {
	entries, err := os.ReadDir(p)
	if err != nil {
		return nil, err
	}
	out := make([]fs.FileInfo, 0, len(entries))
	for _, e := range entries {
		fi, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, fi)
	}
	return out, nil
}

func (localClient) Open(p string) (File, error)   { return os.Open(p) }
func (localClient) Create(p string) (File, error) { return os.Create(p) }
func (localClient) MkdirAll(p string) error       { return os.MkdirAll(p, 0o755) }
func (localClient) Rename(o, n string) error      { return os.Rename(o, n) }
func (localClient) Remove(p string) error         { return os.RemoveAll(p) }
func (localClient) Stat(p string) (fs.FileInfo, error) {
	return os.Stat(p)
}
func (localClient) Close() error { return nil }

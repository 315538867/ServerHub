package sftppool

import (
	"sync"

	"github.com/pkg/sftp"
	gossh "golang.org/x/crypto/ssh"
)

var mu sync.Map // serverID uint → *sftp.Client

// Get returns a cached or new SFTP client for the given server.
func Get(serverID uint, sshClient *gossh.Client) (*sftp.Client, error) {
	if v, ok := mu.Load(serverID); ok {
		c := v.(*sftp.Client)
		if _, err := c.Getwd(); err == nil {
			return c, nil
		}
		c.Close()
		mu.Delete(serverID)
	}
	c, err := sftp.NewClient(sshClient)
	if err != nil {
		return nil, err
	}
	mu.Store(serverID, c)
	return c, nil
}

func Remove(serverID uint) {
	if v, ok := mu.Load(serverID); ok {
		v.(*sftp.Client).Close()
		mu.Delete(serverID)
	}
}

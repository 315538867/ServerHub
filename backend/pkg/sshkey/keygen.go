// Package sshkey generates ed25519 SSH keypairs in OpenSSH wire format.
package sshkey

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"

	"golang.org/x/crypto/ssh"
)

// Pair represents a generated keypair in OpenSSH text format.
type Pair struct {
	// PrivatePEM is the OpenSSH-format PEM-encoded private key (no passphrase).
	// Suitable to feed straight back into ssh.ParsePrivateKey.
	PrivatePEM string
	// PublicAuthorized is the single-line public key in authorized_keys format,
	// e.g. "ssh-ed25519 AAAA... serverhub@self".
	PublicAuthorized string
}

// GenerateEd25519 creates a fresh ed25519 keypair tagged with the given comment.
func GenerateEd25519(comment string) (*Pair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("ed25519 keygen: %w", err)
	}

	block, err := ssh.MarshalPrivateKey(priv, comment)
	if err != nil {
		return nil, fmt.Errorf("marshal private key: %w", err)
	}
	privPEM := pem.EncodeToMemory(block)

	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return nil, fmt.Errorf("wrap public key: %w", err)
	}
	authLine := string(ssh.MarshalAuthorizedKey(sshPub))
	// Append the comment so authorized_keys is human-readable; MarshalAuthorizedKey
	// only emits "<type> <base64>\n", no comment.
	if comment != "" {
		// strip trailing newline, append comment, restore newline
		authLine = authLine[:len(authLine)-1] + " " + comment + "\n"
	}

	return &Pair{
		PrivatePEM:       string(privPEM),
		PublicAuthorized: authLine,
	}, nil
}

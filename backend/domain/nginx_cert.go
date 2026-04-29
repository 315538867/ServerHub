package domain

import "time"

// NginxCert 表示 serverhub 管理的一份 TLS 证书。
type NginxCert struct {
	ID        uint       `json:"id"`
	Domain    string     `json:"domain"`
	Source    string     `json:"source"` // acme | manual
	CertPEM   string     `json:"-"`
	KeyPEM    string     `json:"-"`
	ExpiresAt *time.Time `json:"expires_at"`
	AutoRenew bool       `json:"auto_renew"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

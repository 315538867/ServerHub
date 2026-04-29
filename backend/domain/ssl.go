package domain

import "time"

// SSLCert 是一台 edge 上的一份证书。
// model 中使用 gorm.Model(含 DeletedAt),domain 扁平化。
type SSLCert struct {
	ID            uint       `json:"id"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	DeletedAt     *time.Time `json:"-"`
	ServerID      uint       `json:"server_id"`
	ApplicationID *uint      `json:"application_id"`
	Domain        string     `json:"domain"`
	CertPath      string     `json:"cert_path"`
	KeyPath       string     `json:"key_path"`
	Issuer        string     `json:"issuer"`
	ExpiresAt     time.Time  `json:"expires_at"`
	AutoRenew     bool       `json:"auto_renew"`
	CertPEM       string     `json:"-"`
	KeyPEM        string     `json:"-"`
	LastRenewedAt *time.Time `json:"last_renewed_at,omitempty"`
}

package model

import "time"

// NginxCert 表示 serverhub 管理的一份 TLS 证书。
//
// Source 决定证书来源：
//   - acme  : 由 P2 集成的 acme.sh / certbot 自动申请，AutoRenew=true 时定期续签
//   - manual: 用户上传的自带证书，不自动续签
//
// CertPEM/KeyPEM 用 AES-GCM 加密存储（复用 pkg/crypto），落盘时由 Reconciler
// 解密后写到目标 server 的 /etc/nginx/cert/<domain>/。
//
// P0 仅建表，不实现申请/续签逻辑（P2 范畴）。
type NginxCert struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	Domain    string     `gorm:"not null;uniqueIndex" json:"domain"`
	Source    string     `gorm:"not null" json:"source"` // acme | manual
	CertPEM   string     `gorm:"type:text" json:"-"`     // AES 加密
	KeyPEM    string     `gorm:"type:text" json:"-"`     // AES 加密
	ExpiresAt *time.Time `json:"expires_at"`
	AutoRenew bool       `gorm:"default:true" json:"auto_renew"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

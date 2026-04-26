package model

import (
	"time"

	"gorm.io/gorm"
)

// SSLCert 一台 edge 上的一份证书。
//
// 历史版本只存远端文件路径（CertPath/KeyPath），P2 起把 PEM 内容也加密入库
// （CertPEM / KeyPEM，AES-GCM），原因有两个：
//   1. 证书机密落 DB 后，reconciler 落盘时不再依赖远端文件存活——重装 nginx 也能恢复；
//   2. 续签调度可以在不开 SSH 的前提下读到当前 PEM、判断到期、决定要不要 renew。
//
// CertPath/KeyPath 仍保留：导入历史 letsencrypt 站点不一定能立刻拷回 PEM，
// loader 优先用 DB PEM，回退到 CertPath。
type SSLCert struct {
	gorm.Model
	ServerID      uint   `gorm:"not null;index"`
	ApplicationID *uint  `gorm:"index"` // 可选：把证书绑定到具体应用域名
	Domain        string `gorm:"not null"`
	CertPath      string
	KeyPath       string
	Issuer        string
	ExpiresAt     time.Time
	AutoRenew     bool       `gorm:"default:true"`
	CertPEM       string     `gorm:"type:text" json:"-"` // AES-GCM 加密后的 fullchain
	KeyPEM        string     `gorm:"type:text" json:"-"` // AES-GCM 加密后的 privkey
	LastRenewedAt *time.Time `json:"last_renewed_at,omitempty"`
}

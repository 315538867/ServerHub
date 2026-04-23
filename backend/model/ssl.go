package model

import (
	"time"

	"gorm.io/gorm"
)

type SSLCert struct {
	gorm.Model
	ServerID      uint   `gorm:"not null;index"`
	ApplicationID *uint  `gorm:"index"` // 可选：把证书绑定到具体应用域名
	Domain        string `gorm:"not null"`
	CertPath      string
	KeyPath       string
	Issuer        string
	ExpiresAt     time.Time
	AutoRenew     bool `gorm:"default:true"`
}

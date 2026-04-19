package model

import (
	"time"

	"gorm.io/gorm"
)

type SSLCert struct {
	gorm.Model
	ServerID  uint      `gorm:"not null;index"`
	Domain    string    `gorm:"not null"`
	CertPath  string
	KeyPath   string
	Issuer    string
	ExpiresAt time.Time
	AutoRenew bool `gorm:"default:true"`
}

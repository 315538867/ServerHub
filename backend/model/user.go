package model

import "time"

type User struct {
	ID         uint       `gorm:"primaryKey"           json:"id"`
	Username   string     `gorm:"uniqueIndex;not null" json:"username"`
	Password   string     `gorm:"not null"             json:"-"`
	Role       string     `gorm:"default:admin"        json:"role"`
	MFASecret  string     `gorm:"default:''"           json:"-"`
	MFAEnabled bool       `gorm:"default:false"        json:"mfa_enabled"`
	// LastTOTPStep is the most recently accepted TOTP time-step for this
	// user (Unix seconds / 30). Verification rejects codes whose step is
	// not strictly greater — stopping replay within the 90s tolerance
	// window after the first use.
	LastTOTPStep int64      `gorm:"default:0"            json:"-"`
	LastLogin    *time.Time `                            json:"last_login"`
	LastIP     string     `gorm:"default:''"           json:"last_ip"`
	CreatedAt  time.Time  `                            json:"created_at"`
	UpdatedAt  time.Time  `                            json:"updated_at"`
}

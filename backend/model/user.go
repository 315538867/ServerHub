package model

import "time"

type User struct {
	ID         uint       `gorm:"primaryKey"           json:"id"`
	Username   string     `gorm:"uniqueIndex;not null" json:"username"`
	Password   string     `gorm:"not null"             json:"-"`
	Role       string     `gorm:"default:admin"        json:"role"`
	MFASecret  string     `gorm:"default:''"           json:"-"`
	MFAEnabled bool       `gorm:"default:false"        json:"mfa_enabled"`
	LastLogin  *time.Time `                            json:"last_login"`
	LastIP     string     `gorm:"default:''"           json:"last_ip"`
	CreatedAt  time.Time  `                            json:"created_at"`
	UpdatedAt  time.Time  `                            json:"updated_at"`
}

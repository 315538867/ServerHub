package domain

import "time"

// User 是领域实体。
type User struct {
	ID           uint       `json:"id"`
	Username     string     `json:"username"`
	Password     string     `json:"-"`
	Role         string     `json:"role"`
	MFASecret    string     `json:"-"`
	MFAEnabled   bool       `json:"mfa_enabled"`
	LastTOTPStep int64      `json:"-"`
	LastLogin    *time.Time `json:"last_login"`
	LastIP       string     `json:"last_ip"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

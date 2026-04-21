package model

import "time"

type Server struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Name        string     `gorm:"not null" json:"name"`
	Type        string     `gorm:"default:ssh" json:"type"` // "ssh" | "local"
	Host        string     `gorm:"not null" json:"host"`
	Port        int        `gorm:"default:22" json:"port"`
	Username    string     `gorm:"not null" json:"username"`
	AuthType    string     `gorm:"default:password" json:"auth_type"` // "password" | "key" | "local"
	Password    string     `gorm:"default:''" json:"-"`               // AES-GCM encrypted
	PrivateKey  string     `gorm:"default:''" json:"-"`               // AES-GCM encrypted
	Remark      string     `gorm:"default:''" json:"remark"`
	Status      string     `gorm:"default:unknown" json:"status"` // "online"|"offline"|"unknown"
	LastCheckAt *time.Time `json:"last_check_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

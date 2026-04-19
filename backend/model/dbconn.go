package model

import "gorm.io/gorm"

type DBConn struct {
	gorm.Model
	ServerID uint   `gorm:"not null;index"`
	Name     string `gorm:"not null"`
	Type     string `gorm:"not null"` // mysql / redis
	Host     string `gorm:"default:'127.0.0.1'"`
	Port     int
	Username string
	Password string // AES encrypted
	Database string
}

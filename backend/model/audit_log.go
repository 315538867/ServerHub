package model

import "time"

type AuditLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     *uint     `                  json:"user_id"`
	Username   string    `gorm:"not null"   json:"username"`
	IP         string    `gorm:"not null"   json:"ip"`
	Method     string    `gorm:"not null"   json:"method"`
	Path       string    `gorm:"not null"   json:"path"`
	Body       string    `gorm:"default:''" json:"body"`
	Status     int       `gorm:"not null"   json:"status"`
	DurationMS int       `gorm:"default:0"  json:"duration_ms"`
	CreatedAt  time.Time `                  json:"created_at"`
}

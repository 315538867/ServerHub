package model

import "time"

type AppNginxRoute struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	AppID     uint      `gorm:"not null;index" json:"app_id"`
	Path      string    `gorm:"not null" json:"path"`
	Upstream  string    `gorm:"not null" json:"upstream"`
	Extra     string    `gorm:"default:''" json:"extra"`
	Sort      int       `gorm:"default:0" json:"sort"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

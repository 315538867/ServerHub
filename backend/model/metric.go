package model

import "time"

type Metric struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ServerID  uint      `gorm:"index;not null" json:"server_id"`
	CPU       float64   `json:"cpu"`    // percentage 0-100
	Mem       float64   `json:"mem"`    // percentage 0-100
	Disk      float64   `json:"disk"`   // percentage 0-100
	Load1     float64   `json:"load1"`
	Uptime    int64     `json:"uptime"` // seconds
	CreatedAt time.Time `json:"created_at"`
}

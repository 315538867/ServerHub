package domain

import "time"

// Metric 系统指标。
type Metric struct {
	ID        uint      `json:"id"`
	ServerID  uint      `json:"server_id"`
	CPU       float64   `json:"cpu"`
	Mem       float64   `json:"mem"`
	Disk      float64   `json:"disk"`
	Load1     float64   `json:"load1"`
	Uptime    int64     `json:"uptime"`
	CreatedAt time.Time `json:"created_at"`
}

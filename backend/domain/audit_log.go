package domain

import "time"

// AuditLog 审计日志。
type AuditLog struct {
	ID         uint      `json:"id"`
	UserID     *uint     `json:"user_id"`
	Username   string    `json:"username"`
	IP         string    `json:"ip"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	Body       string    `json:"body"`
	Status     int       `json:"status"`
	DurationMS int       `json:"duration_ms"`
	CreatedAt  time.Time `json:"created_at"`
}

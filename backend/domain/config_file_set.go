package domain

import "time"

// ConfigFileSet 是配置文件集,独立版本,可被多个 Release 复用。
type ConfigFileSet struct {
	ID        uint      `json:"id"`
	ServiceID uint      `json:"service_id"`
	Label     string    `json:"label"`
	Files     string    `json:"files"` // JSON 数组
	CreatedAt time.Time `json:"created_at"`
}

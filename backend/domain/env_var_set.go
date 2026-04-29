package domain

import "time"

// EnvVarSet 是环境变量集,独立版本,可被多个 Release 复用。
type EnvVarSet struct {
	ID        uint      `json:"id"`
	ServiceID uint      `json:"service_id"`
	Label     string    `json:"label"`
	Content   string    `json:"-"` // AES-GCM 加密内容
	CreatedAt time.Time `json:"created_at"`
}

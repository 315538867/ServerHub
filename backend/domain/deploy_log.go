package domain

import "time"

// DeployLog 是部署日志记录。
type DeployLog struct {
	ID            uint      `json:"id"`
	DeployID      uint      `json:"deploy_id"`
	Output        string    `json:"output"`
	Status        string    `json:"status"` // "success"|"failed"
	Duration      int       `json:"duration"`
	TriggerSource string    `json:"trigger_source"`
	CreatedAt     time.Time `json:"created_at"`
}

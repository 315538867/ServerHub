package model

import "time"

// DeployRunStatus 枚举
const (
	DeployRunStatusRunning    = "running"
	DeployRunStatusSuccess    = "success"
	DeployRunStatusFailed     = "failed"
	DeployRunStatusRolledBack = "rolled_back"
)

// DeployRunTrigger 枚举
const (
	TriggerSourceManual       = "manual"
	TriggerSourceWebhook      = "webhook"
	TriggerSourceSchedule     = "schedule"
	TriggerSourceAPI          = "api"
	TriggerSourceAutoRollback = "auto_rollback"
)

// DeployRun 一次部署执行的记录。每次 Apply 对应一条。
type DeployRun struct {
	ID        uint `gorm:"primaryKey" json:"id"`
	ServiceID uint `gorm:"not null;index:idx_run_svc,priority:1" json:"service_id"`
	ReleaseID uint `gorm:"not null" json:"release_id"`

	Status        string `gorm:"default:running" json:"status"` // running|success|failed|rolled_back
	TriggerSource string `gorm:"default:manual" json:"trigger_source"`

	StartedAt   time.Time  `gorm:"index:idx_run_svc,priority:2,sort:desc" json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at"`
	DurationSec int        `gorm:"default:0" json:"duration_sec"`

	Output string `gorm:"type:text;default:''" json:"output"`

	// 若本次是回滚，指向触发回滚的那次失败 run
	RollbackFromRunID *uint `json:"rollback_from_run_id"`

	CreatedAt time.Time `json:"created_at"`
}

func (DeployRun) TableName() string { return "deploy_runs" }

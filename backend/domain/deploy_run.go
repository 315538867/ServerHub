package domain

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

// DeployRun 一次部署执行的记录。
type DeployRun struct {
	ID                uint       `json:"id"`
	ServiceID         uint       `json:"service_id"`
	ReleaseID         uint       `json:"release_id"`
	Status            string     `json:"status"`
	TriggerSource     string     `json:"trigger_source"`
	StartedAt         time.Time  `json:"started_at"`
	FinishedAt        *time.Time `json:"finished_at"`
	DurationSec       int        `json:"duration_sec"`
	Output            string     `json:"output"`
	RollbackFromRunID *uint      `json:"rollback_from_run_id"`
	CreatedAt         time.Time  `json:"created_at"`
}

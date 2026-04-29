package domain

import "time"

// AppReleaseSetStatus 枚举
const (
	AppReleaseSetStatusDraft      = "draft"
	AppReleaseSetStatusApplying   = "applying"
	AppReleaseSetStatusSuccess    = "success"
	AppReleaseSetStatusPartial    = "partial"
	AppReleaseSetStatusFailed     = "failed"
	AppReleaseSetStatusRolledBack = "rolled_back"
)

// AppReleaseSet 是 App 下多个 Service 在某一时刻的 Release 组合快照。
type AppReleaseSet struct {
	ID            uint       `json:"id"`
	ApplicationID uint       `json:"application_id"`
	Label         string     `json:"label"`
	Items         string     `json:"items"`       // JSON [{service_id,release_id}]
	Note          string     `json:"note"`
	Status        string     `json:"status"`
	CreatedBy     string     `json:"created_by"`
	AppliedAt     *time.Time `json:"applied_at"`
	LastSummary   string     `json:"last_summary"` // JSON
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// AppReleaseSetItem Items JSON 数组元素。
type AppReleaseSetItem struct {
	ServiceID uint `json:"service_id"`
	ReleaseID uint `json:"release_id"`
}

// AppReleaseSummaryItem LastSummary JSON 数组元素。
type AppReleaseSummaryItem struct {
	ServiceID uint   `json:"service_id"`
	RunID     *uint  `json:"run_id,omitempty"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

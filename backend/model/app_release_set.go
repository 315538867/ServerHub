package model

import "time"

// AppReleaseSet 枚举
const (
	AppReleaseSetStatusDraft      = "draft"
	AppReleaseSetStatusApplying   = "applying"
	AppReleaseSetStatusSuccess    = "success"
	AppReleaseSetStatusPartial    = "partial"
	AppReleaseSetStatusFailed     = "failed"
	AppReleaseSetStatusRolledBack = "rolled_back"
)

// AppReleaseSet 是"App 下多个 Service 在某一时刻的 Release 组合"的快照。
// 一次 Apply 串行对 Items 里每个 (service_id, release_id) 调 deployer.ApplyRelease。
//
// 幂等性：同一 set Apply 多次等价——内部按 Service 切 CurrentReleaseID，重复 Apply
// 同一 Release 由 deployer 侧处理成"再 up 一次"。
type AppReleaseSet struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	ApplicationID uint       `gorm:"not null;index" json:"application_id"`
	Label         string     `gorm:"default:''" json:"label"`
	Items         string     `gorm:"type:text;default:''" json:"items"`        // JSON [{service_id,release_id}]
	Note          string     `gorm:"default:''" json:"note"`
	Status        string     `gorm:"default:'draft'" json:"status"`            // draft|applying|success|partial|failed|rolled_back
	CreatedBy     string     `gorm:"default:''" json:"created_by"`
	AppliedAt     *time.Time `json:"applied_at"`
	LastSummary   string     `gorm:"type:text;default:''" json:"last_summary"` // JSON [{service_id,run_id?,status,error?}]

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (AppReleaseSet) TableName() string { return "app_release_sets" }

// AppReleaseSetItem Items JSON 数组元素。
type AppReleaseSetItem struct {
	ServiceID uint `json:"service_id"`
	ReleaseID uint `json:"release_id"`
}

// AppReleaseSummaryItem LastSummary JSON 数组元素。
type AppReleaseSummaryItem struct {
	ServiceID uint   `json:"service_id"`
	RunID     *uint  `json:"run_id,omitempty"`
	Status    string `json:"status"` // success|failed|skipped
	Error     string `json:"error,omitempty"`
}

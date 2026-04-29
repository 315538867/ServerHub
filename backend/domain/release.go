package domain

import "time"

// ReleaseStatus 枚举
const (
	ReleaseStatusDraft      = "draft"
	ReleaseStatusActive     = "active"
	ReleaseStatusRolledBack = "rolled_back"
	ReleaseStatusArchived   = "archived"
)

// Release 是一次可部署的最小完整单位。
// StartSpec 为 typed interface(R8),持久化 JSON string 由 model/converter 处理。
type Release struct {
	ID               uint      `json:"id"`
	ServiceID        uint      `json:"service_id"`
	Label            string    `json:"label"`
	Version          string    `json:"version"` // 兼容旧代码(R8 统一为 Label)
	ArtifactID       uint      `json:"artifact_id"`
	EnvSetID         *uint     `json:"env_set_id"`
	ConfigSetID      *uint     `json:"config_set_id"`
	StartSpec        StartSpec `json:"-"` // typed interface, 持久化由 model/converter 处理
	Note             string    `json:"note"`
	CreatedBy        string    `json:"created_by"`
	Status           string    `json:"status"`
	ArtifactProvider string    `json:"-"`
	ArtifactRef      string    `json:"-"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// ReleaseStatus 枚举
const (
	ReleaseStatusDraft      = "draft"
	ReleaseStatusActive     = "active"
	ReleaseStatusRolledBack = "rolled_back"
	ReleaseStatusArchived   = "archived"
)

// Release 是一次可部署的最小完整单位，由三维组件组合而成（Artifact/EnvVarSet/ConfigFileSet）
// + StartSpec。一旦 Apply 成功就不可变；任何变更都通过生成新 Release 实现。
// 回滚 = 以历史 Release 再次 Apply（复制其三维生成新 Release 后 Apply，或直接 Apply 历史行）。
type Release struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	ServiceID  uint   `gorm:"not null;index:idx_release_svc,priority:1" json:"service_id"`
	Label      string `gorm:"default:''" json:"label"`
	ArtifactID uint   `gorm:"not null" json:"artifact_id"`
	EnvSetID    *uint `json:"env_set_id"`
	ConfigSetID *uint `json:"config_set_id"`

	// StartSpec 按 Service.Type 存不同结构的 JSON：
	//   docker:  {image, cmd, args, ports, volumes, restart}
	//   compose: {file_name, compose_profile}
	//   static:  {index_file}
	//   native:  {cmd, workdir_subpath}
	StartSpec string `gorm:"type:text;default:''" json:"start_spec"`

	Note      string `gorm:"default:''" json:"note"`
	CreatedBy string `gorm:"default:''" json:"created_by"`
	Status    string `gorm:"default:draft" json:"status"` // draft|active|rolled_back|archived

	CreatedAt time.Time `gorm:"index:idx_release_svc,priority:2,sort:desc" json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Release) TableName() string { return "releases" }

// BeforeUpdate 钩子：INV-2 — status≠draft 时不可变字段保护。
// 已非 draft 的 Release（active/rolled_back/archived），其 ArtifactID、
// EnvSetID、ConfigSetID、StartSpec 四个字段不可修改。
func (r *Release) BeforeUpdate(tx *gorm.DB) error {
	if r.Status == ReleaseStatusDraft {
		return nil
	}
	if tx.Statement.Changed("ArtifactID") ||
		tx.Statement.Changed("EnvSetID") ||
		tx.Statement.Changed("ConfigSetID") ||
		tx.Statement.Changed("StartSpec") {
		return errors.New("release: status≠draft 时不可修改 ArtifactID/EnvSetID/ConfigSetID/StartSpec(INV-2)")
	}
	return nil
}

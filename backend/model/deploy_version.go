package model

import "time"

// DeployVersion is a snapshot of a Deploy config captured after a successful run.
//
// Deprecated: 在 M3 阶段该表已转为只读，新链路用 model.Release / model.Artifact /
// model.DeployRun 表达。保留用于历史查看与 M2 迁移脚本；ArchivedAt 非空表示该行
// 已在迁移中被折算成 Release，前端仅展示不再计入活跃数据。
type DeployVersion struct {
	ID            uint   `gorm:"primaryKey" json:"id"`
	DeployID      uint   `gorm:"index:idx_deploy_ver,priority:1;not null" json:"deploy_id"`
	Version       string `gorm:"default:''" json:"version"`
	Status        string `gorm:"default:success" json:"status"` // success|failed (we only snapshot success, reserve failed for future)
	TriggerSource string `gorm:"default:manual" json:"trigger_source"`

	// Full deploy config snapshot (enough to rollback)
	Type        string `gorm:"default:''" json:"type"`
	WorkDir     string `gorm:"default:''" json:"work_dir"`
	ComposeFile string `gorm:"default:''" json:"compose_file"`
	StartCmd    string `gorm:"default:''" json:"start_cmd"`
	ImageName   string `gorm:"default:''" json:"image_name"`
	Runtime     string `gorm:"default:''" json:"runtime"`
	ConfigFiles string `gorm:"type:text;default:''" json:"config_files"`
	EnvVars     string `gorm:"type:text;default:''" json:"-"` // AES ciphertext, mirrors Deploy.EnvVars

	DeployLogID uint      `gorm:"default:0" json:"deploy_log_id"`
	Note        string    `gorm:"default:''" json:"note"`
	CreatedAt   time.Time `gorm:"index:idx_deploy_ver,priority:2,sort:desc" json:"created_at"`

	// ArchivedAt 在 M3 阶段由 M2 迁移脚本回填；非空表示该行已成为只读历史记录，
	// 不再参与任何部署/回滚逻辑。AutoMigrate 会自动为旧表补列（nullable）。
	ArchivedAt *time.Time `json:"archived_at,omitempty"`
}

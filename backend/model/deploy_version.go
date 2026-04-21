package model

import "time"

// DeployVersion is a snapshot of a Deploy config captured after a successful run.
// Up to 7 rows per DeployID are retained (FIFO).
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
}

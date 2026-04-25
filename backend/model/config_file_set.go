package model

import "time"

// ConfigFileSet 是配置文件集，独立版本，可被多个 Release 复用。
// Files 是 JSON 数组：[{"name":"app.yml","content_b64":"...","mode":420}]
type ConfigFileSet struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	ServiceID uint   `gorm:"not null;index" json:"service_id"`
	Label     string `gorm:"default:''" json:"label"`
	Files     string `gorm:"type:text;default:''" json:"files"`

	CreatedAt time.Time `json:"created_at"`
}

func (ConfigFileSet) TableName() string { return "config_file_sets" }

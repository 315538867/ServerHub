package model

import "time"

// EnvVarSet 是环境变量集，独立版本，可被多个 Release 复用。
// Content 是 AES-GCM 加密的 JSON：[{"key":"K","value":"V","secret":true}]
type EnvVarSet struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	ServiceID uint   `gorm:"not null;index" json:"service_id"`
	Label     string `gorm:"default:''" json:"label"`
	Content   string `gorm:"type:text;default:''" json:"-"` // 加密内容不直接出 JSON

	CreatedAt time.Time `json:"created_at"`
}

func (EnvVarSet) TableName() string { return "env_var_sets" }

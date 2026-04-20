package model

import "time"

type DeployLog struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	DeployID      uint      `gorm:"index;not null" json:"deploy_id"`
	Output        string    `gorm:"type:text" json:"output"`
	Status        string    `json:"status"` // "success"|"failed"
	Duration      int       `json:"duration"` // seconds
	TriggerSource string    `gorm:"default:manual" json:"trigger_source"` // manual|webhook|schedule|api
	CreatedAt     time.Time `json:"created_at"`
}

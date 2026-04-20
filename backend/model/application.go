package model

import "time"

type Application struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	Name          string     `gorm:"not null;uniqueIndex" json:"name"`
	Description   string     `gorm:"default:''" json:"description"`
	ServerID      uint       `gorm:"not null;index" json:"server_id"`
	SiteName      string     `gorm:"default:''" json:"site_name"`
	Domain        string     `gorm:"default:''" json:"domain"`
	ContainerName string     `gorm:"default:''" json:"container_name"`
	DeployID      *uint      `gorm:"index" json:"deploy_id"`
	DBConnID      *uint      `gorm:"index" json:"db_conn_id"`
	BaseDir       string     `gorm:"default:''" json:"base_dir"`
	ExposeMode    string     `gorm:"default:none" json:"expose_mode"`
	Status        string     `gorm:"default:unknown" json:"status"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

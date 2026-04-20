package model

import "time"

type Deploy struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	Name        string `gorm:"not null" json:"name"`
	ServerID    uint   `gorm:"not null" json:"server_id"`
	Type        string `gorm:"default:docker-compose" json:"type"` // docker|docker-compose|native

	// Execution config
	WorkDir     string `gorm:"default:''" json:"work_dir"`
	ComposeFile string `gorm:"default:docker-compose.yml" json:"compose_file"`
	StartCmd    string `gorm:"default:''" json:"start_cmd"`
	ImageName   string `gorm:"default:''" json:"image_name"` // used for docker rmi on version update
	Runtime     string `gorm:"default:''" json:"runtime"`     // java|go|node|rust|python|custom (native/docker)
	ConfigFiles string `gorm:"default:''" json:"config_files"` // JSON: [{name,content}]

	// Auth & secrets
	EnvVars       string `gorm:"default:''" json:"-"` // AES-encrypted JSON array
	WebhookSecret string `gorm:"default:''" json:"-"`

	// Version management
	DesiredVersion  string `gorm:"default:''" json:"desired_version"`
	ActualVersion   string `gorm:"default:''" json:"actual_version"`
	PreviousVersion string `gorm:"default:''" json:"previous_version"`

	// Reconcile loop
	AutoSync     bool   `gorm:"default:false" json:"auto_sync"`
	SyncInterval int    `gorm:"default:60" json:"sync_interval"` // seconds, 0 = manual/webhook only
	SyncStatus   string `gorm:"default:''" json:"sync_status"`   // ""|synced|drifted|syncing|error

	// Status
	LastRunAt  *time.Time `json:"last_run_at"`
	LastStatus string     `gorm:"default:''" json:"last_status"` // ""|running|success|failed
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

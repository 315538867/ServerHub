package model

import "time"

// Service represents a managed runtime entity (one start/stop unit) on a
// Server. Replaces the former Deploy model; on-disk table name kept as
// "services" via TableName(). Existing "deploys" table is renamed by the
// AutoMigrate hook in database/db.go.
//
// A Service may belong to an Application (ApplicationID set) or float
// independently (ApplicationID nil) — the latter typically right after
// import-only discovery before takeover.
type Service struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	Name     string `gorm:"not null" json:"name"`
	ServerID uint   `gorm:"not null" json:"server_id"`
	Type     string `gorm:"default:docker-compose" json:"type"` // docker|docker-compose|native|static

	// Application binding (nullable: floating services allowed)
	ApplicationID *uint `gorm:"index" json:"application_id"`

	// Execution config
	WorkDir     string `gorm:"default:''" json:"work_dir"`
	ComposeFile string `gorm:"default:docker-compose.yml" json:"compose_file"`
	StartCmd    string `gorm:"default:''" json:"start_cmd"`
	ImageName   string `gorm:"default:''" json:"image_name"`
	Runtime     string `gorm:"default:''" json:"runtime"`
	ConfigFiles string `gorm:"default:''" json:"config_files"`

	// Auth & secrets
	EnvVars       string `gorm:"default:''" json:"-"`
	WebhookSecret string `gorm:"default:''" json:"-"`

	// Version management
	DesiredVersion string `gorm:"default:''" json:"desired_version"`
	ActualVersion  string `gorm:"default:''" json:"actual_version"`

	// Reconcile loop
	AutoSync     bool   `gorm:"default:false" json:"auto_sync"`
	SyncInterval int    `gorm:"default:60" json:"sync_interval"`
	SyncStatus   string `gorm:"default:''" json:"sync_status"`

	// Status
	LastRunAt  *time.Time `json:"last_run_at"`
	LastStatus string     `gorm:"default:''" json:"last_status"`

	// Discovery source. SourceFingerprint is computed by discovery.Fingerprint
	// from kind-specific stable inputs and used to dedup candidates against
	// already-managed services.
	SourceKind        string `gorm:"default:'';index:idx_svc_source,priority:2" json:"source_kind"`
	SourceID          string `gorm:"default:'';index:idx_svc_source,priority:3" json:"source_id"`
	SourceFingerprint string `gorm:"default:'';size:64;index" json:"source_fingerprint"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Service) TableName() string { return "services" }

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
	// Deprecated: M3 起由 Release.StartSpec + Artifact.Provider 表达；保留供历史读路径与 M2 迁移脚本使用。
	Type string `gorm:"default:docker-compose" json:"type"` // docker|docker-compose|native|static

	// Application binding (nullable: floating services allowed)
	ApplicationID *uint `gorm:"index" json:"application_id"`

	// Execution config
	//
	// Deprecated: 以下 6 字段在 M3 起被 Release 三维模型替代，仅用于历史读路径与迁移脚本：
	//   WorkDir      → 新链路由 Release 上下文与 Service.WorkDir 合并（Service.WorkDir 仍生效，作为默认 cwd）
	//   ComposeFile  → Release.StartSpec["file_name"]
	//   StartCmd     → Release.StartSpec["cmd"]
	//   ImageName    → Artifact.Ref（provider=docker）
	//   Runtime      → Release.StartSpec 自由键
	//   ConfigFiles  → ConfigFileSet.Files
	WorkDir     string `gorm:"default:''" json:"work_dir"`
	ComposeFile string `gorm:"default:docker-compose.yml" json:"compose_file"`
	StartCmd    string `gorm:"default:''" json:"start_cmd"`
	ImageName   string `gorm:"default:''" json:"image_name"`
	Runtime     string `gorm:"default:''" json:"runtime"`
	ConfigFiles string `gorm:"default:''" json:"config_files"`

	// ExposedPort 是 Service 对外提供的主端口（供 Nginx upstream 使用）。
	// 0 表示未暴露或纯静态服务。discovery 阶段会尽量从 docker ports / compose
	// 端口映射 / systemd env 中推断填入；用户也可以在 UI 里手工修改。
	ExposedPort int `gorm:"default:0" json:"exposed_port"`

	// Auth & secrets
	//
	// Deprecated: EnvVars 在 M3 起由 EnvVarSet 替代；保留用于 /panel/api/v1/services/:id/env 只读展示。
	EnvVars       string `gorm:"default:''" json:"-"`
	WebhookSecret string `gorm:"default:''" json:"-"`

	// Version management
	//
	// Deprecated: 版本语义在 M3 起由 Release.Label + Service.CurrentReleaseID 表达；
	// DesiredVersion/ActualVersion 不再参与调度，仅保留供历史读路径与迁移脚本使用。
	DesiredVersion string `gorm:"default:''" json:"desired_version"`
	ActualVersion  string `gorm:"default:''" json:"actual_version"`

	// Release 新模型指针（Phase M1 引入，与旧 DesiredVersion/DeployVersion 并存）。
	// 指向 releases.id；为 nil 表示 Service 还没有创建过 Release（空壳）。
	CurrentReleaseID *uint `gorm:"index" json:"current_release_id"`
	// 部署失败时是否自动回滚到上一条 Status=active 的 Release（默认关闭）。
	AutoRollbackOnFail bool `gorm:"default:false" json:"auto_rollback_on_fail"`

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

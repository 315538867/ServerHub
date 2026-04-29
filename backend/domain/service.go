package domain

import "time"

// ServiceType 标识服务的运行时种类,与 RuntimeAdapter.Kind() 一一对应。
type ServiceType string

const (
	ServiceTypeDocker        ServiceType = "docker"
	ServiceTypeDockerCompose ServiceType = "docker-compose"
	ServiceTypeCompose       ServiceType = "compose" // adapter Kind 值
	ServiceTypeNative        ServiceType = "native"
	ServiceTypeStatic        ServiceType = "static"
)

// Service 是领域实体。
//
// Status 摘要字段已在 R3 下线:运行状态由 deploy_runs 最近一条派生。
type Service struct {
	ID                 uint       `json:"id"`
	Name               string     `json:"name"`
	ServerID           uint       `json:"server_id"`
	Type               ServiceType `json:"type"`
	ApplicationID      *uint      `json:"application_id"`
	WorkDir            string     `json:"work_dir"`
	ExposedPort        int        `json:"exposed_port"`
	WebhookSecret      string     `json:"-"`
	CurrentReleaseID   *uint      `json:"current_release_id"`
	AutoRollbackOnFail bool       `json:"auto_rollback_on_fail"`
	AutoSync           bool       `json:"auto_sync"`
	SyncInterval       int        `json:"sync_interval"`
	SyncStatus         string     `json:"sync_status"`
	SourceKind         string     `json:"source_kind"`
	SourceID           string     `json:"source_id"`
	SourceFingerprint  string     `json:"source_fingerprint"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

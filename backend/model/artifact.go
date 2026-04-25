package model

import "time"

// ArtifactProvider 枚举
const (
	ArtifactProviderUpload   = "upload"
	ArtifactProviderScript   = "script"
	ArtifactProviderGit      = "git"
	ArtifactProviderHTTP     = "http"
	ArtifactProviderDocker   = "docker"
	ArtifactProviderImported = "imported" // 接管老系统时占位，不可再部署
)

// Artifact 是类型化的制品引用。Ref 的语义随 Provider 变化：
//   upload:   面板本地相对路径 artifacts/${sid}/${sha}.ext
//   docker:   image:tag[@sha256:digest]（Apply 后反写 digest 锁定）
//   git:      repo@ref
//   http:     url
//   script:   无 ref（由 PullScript 产生，Apply 时 sha256 回写 Checksum）
//   imported: 占位字符串（接管时用）
type Artifact struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	ServiceID  uint   `gorm:"not null;index" json:"service_id"`
	Provider   string `gorm:"not null" json:"provider"`
	Ref        string `gorm:"default:''" json:"ref"`
	PullScript string `gorm:"type:text;default:''" json:"pull_script"`
	Checksum   string `gorm:"default:''" json:"checksum"` // sha256 or docker digest
	SizeBytes  int64  `gorm:"default:0" json:"size_bytes"`

	CreatedAt time.Time `json:"created_at"`
}

func (Artifact) TableName() string { return "artifacts" }

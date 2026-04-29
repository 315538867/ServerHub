package domain

import "time"

// ArtifactProvider 枚举
const (
	ArtifactProviderUpload   = "upload"
	ArtifactProviderScript   = "script"
	ArtifactProviderGit      = "git"
	ArtifactProviderHTTP     = "http"
	ArtifactProviderDocker   = "docker"
	ArtifactProviderImported = "imported"
)

// Artifact 是类型化的制品引用。
type Artifact struct {
	ID         uint      `json:"id"`
	ServiceID  uint      `json:"service_id"`
	Provider   string    `json:"provider"`
	Ref        string    `json:"ref"`
	PullScript string    `json:"pull_script"`
	Checksum   string    `json:"checksum"`
	SizeBytes  int64     `json:"size_bytes"`
	CreatedAt  time.Time `json:"created_at"`
}

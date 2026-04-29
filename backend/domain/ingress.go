package domain

import "time"

// Ingress 表示一台 edge server 上的一个入口。
type Ingress struct {
	ID                 uint       `json:"id"`
	EdgeServerID       uint       `json:"edge_server_id"`
	MatchKind          string     `json:"match_kind"` // domain | path
	Domain             string     `json:"domain"`
	DefaultPath        string     `json:"default_path"`
	CertID             *uint      `json:"cert_id"`
	ForceHTTPS         bool       `json:"force_https"`
	Status             string     `json:"status"` // pending|applied|drift|broken
	LastAppliedAt      *time.Time `json:"last_applied_at"`
	ArchivePath        string     `json:"archive_path"`
	OriginalConfigPath string     `json:"original_config_path"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

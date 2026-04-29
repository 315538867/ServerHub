package domain

import "time"

// AuditApply 记录每次 nginx apply 操作的审计信息。
type AuditApply struct {
	ID            uint      `json:"id"`
	EdgeServerID  uint      `json:"edge_server_id"`
	ActorUserID   *uint     `json:"actor_user_id"`
	ChangesetDiff string    `json:"changeset_diff"`
	NginxTOutput  string    `json:"nginx_t_output"`
	RolledBack    bool      `json:"rolled_back"`
	BackupPath    string    `json:"backup_path"`
	DurationMs    int       `json:"duration_ms"`
	CreatedAt     time.Time `json:"created_at"`
}

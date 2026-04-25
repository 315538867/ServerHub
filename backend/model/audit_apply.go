package model

import "time"

// AuditApply 记录每次 nginx apply 操作的审计信息。
//
// 设计目标：事故后能在数据库里直接查到「谁在何时改了什么、nginx -t 输出是什么、
// 是否回滚、备份在哪」，不必依赖外部日志系统。
//
// 写入时机：Reconciler.Apply 入口先 Create 一条占位（拿到 ID），结束时 Update
// 完整字段，确保失败/panic 也能留下记录。
//
// P0 仅建表，写入逻辑由 P1 的 nginxops.Reconciler 实现。
type AuditApply struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	EdgeServerID  uint      `gorm:"not null;index" json:"edge_server_id"`
	ActorUserID   *uint     `gorm:"index" json:"actor_user_id"`
	ChangesetDiff string    `gorm:"type:text" json:"changeset_diff"`
	NginxTOutput  string    `gorm:"type:text" json:"nginx_t_output"`
	RolledBack    bool      `gorm:"default:false" json:"rolled_back"`
	BackupPath    string    `gorm:"default:''" json:"backup_path"`
	DurationMs    int       `gorm:"default:0" json:"duration_ms"`
	CreatedAt     time.Time `gorm:"index" json:"created_at"`
}

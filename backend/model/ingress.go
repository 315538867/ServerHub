package model

import "time"

// Ingress 表示一台 edge server 上的一个入口（一个 nginx server block 的逻辑封装）。
//
// match_kind 决定这个 Ingress 的渲染策略：
//   - domain: 独占 server_name <domain>；常用于 https://app.example.com
//   - path  : 与同 edge 同 domain 下的其他 Path Ingress 共享 server block，
//             各自管自己的 location 前缀；用于多个应用共用一个域名分路径暴露
//
// 强一致约束（业务层校验）：同一个 (edge_server_id, domain) 下的 Ingress
// 必须 MatchKind 一致，不允许 domain 与 path 混用。
type Ingress struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	EdgeServerID  uint       `gorm:"not null;index;uniqueIndex:idx_ingress_edge_domain,priority:1" json:"edge_server_id"`
	MatchKind     string     `gorm:"not null" json:"match_kind"` // domain | path
	Domain        string     `gorm:"not null;uniqueIndex:idx_ingress_edge_domain,priority:2" json:"domain"`
	DefaultPath   string     `gorm:"default:'/'" json:"default_path"`
	CertID        *uint      `gorm:"index" json:"cert_id"`
	// ForceHTTPS=true 时渲染额外 server{listen 80; return 301 https://...}
	// 强制 80→443 跳转；仅当 CertID 非空时才允许置 true（API 层校验）。
	// 显式 column 避免 GORM 默认把 HTTPS 拆成 force_http_s。
	ForceHTTPS    bool       `gorm:"default:false;column:force_https" json:"force_https"`
	Status        string     `gorm:"default:'pending'" json:"status"` // pending|applied|drift|broken
	LastAppliedAt *time.Time `json:"last_applied_at"`
	// ArchivePath / OriginalConfigPath 标记"本 Ingress 是从 nginx 老 vhost 接管而来"。
	//   - ArchivePath: 远端归档目录中 mv 后的绝对路径,如
	//                  /etc/nginx/.serverhub-archive/1714119492/api.example.com.conf
	//   - OriginalConfigPath: 接管前的原文件路径,如 /etc/nginx/sites-enabled/api
	//
	// 两者同时为空 = 普通新建,POST /ingresses/:id/restore 端点拒绝处理。
	// 同时非空 = 接管来源,可一键还原(mv 文件回 OriginalConfigPath + 删 Ingress)。
	// 半空状态(只有一个有值) 视为脏数据,restore 端点也拒绝。
	ArchivePath        string    `gorm:"default:''" json:"archive_path"`
	OriginalConfigPath string    `gorm:"default:''" json:"original_config_path"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

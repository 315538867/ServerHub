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
	Status        string     `gorm:"default:'pending'" json:"status"` // pending|applied|drift|broken
	LastAppliedAt *time.Time `json:"last_applied_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

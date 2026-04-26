package model

import "time"

// ServerProbe 是 server 探测结果的 append-only 时序表。R3 起替代
// servers.{status,last_check_at} 作为"在线状态"的真值源:每次探测追加一行,
// derive.ServerStatus 读最近一条 + 阈值判定 online/lagging/offline/unknown。
//
// 命名:CreatedAt 单列同时承担"探测发生时刻"与 retention cleanTable 字段,
// 让 pkg/retention 的 cleanTable("server_probes", keepDays) 可直接复用,
// 不需要再开 checked_at + created_at 两列。语义上探测结果生成即落库,
// 二者同一时刻无歧义。
//
// Result 取值: "online" | "offline"。"unknown" 不入库 —— 没探测就没行,
// derive 层用 "缺行" 表示 unknown,与 "已探但确认离线" 区分开。
type ServerProbe struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ServerID  uint      `gorm:"not null;index:idx_server_probes_sid_created,priority:1" json:"server_id"`
	Result    string    `gorm:"not null" json:"result"`              // "online" | "offline"
	LatencyMs int       `gorm:"default:0" json:"latency_ms"`         // 0 = 未测/失败
	ErrMsg    string    `gorm:"default:''" json:"err_msg"`           // offline 时填错误信息
	CreatedAt time.Time `gorm:"index:idx_server_probes_sid_created,priority:2,sort:desc" json:"created_at"`
}

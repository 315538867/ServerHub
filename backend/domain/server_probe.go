package domain

import "time"

// ServerProbe 是 server 探测结果的领域实体。
// Result 取值: "online" | "offline"。"unknown" 不入库——没探测就没行。
type ServerProbe struct {
	ID        uint      `json:"id"`
	ServerID  uint      `json:"server_id"`
	Result    string    `json:"result"` // "online" | "offline"
	LatencyMs int       `json:"latency_ms"`
	ErrMsg    string    `json:"err_msg"`
	CreatedAt time.Time `json:"created_at"`
}

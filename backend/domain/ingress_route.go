package domain

import "time"

// IngressUpstream 表示一条 IngressRoute 的目标地址。
type IngressUpstream struct {
	Type         string `json:"type"` // service | raw
	ServiceID    *uint  `json:"service_id,omitempty"`
	RawURL       string `json:"raw_url,omitempty"`
	NetworkPref  string `json:"network_pref,omitempty"`
	OverrideHost string `json:"override_host,omitempty"`
	OverridePort int    `json:"override_port,omitempty"`
}

// IngressRoute 是 Ingress 下的一条路由规则。
type IngressRoute struct {
	ID         uint           `json:"id"`
	IngressID  uint           `json:"ingress_id"`
	Sort       int            `json:"sort"`
	Path       string         `json:"path"`
	Protocol   string         `json:"protocol"` // http|grpc|ws|tcp|udp
	Upstream   IngressUpstream `json:"upstream"`
	WebSocket  bool           `json:"websocket"`
	Extra      string         `json:"extra"`
	ListenPort *int           `json:"listen_port,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// IngressUpstream 表示一条 IngressRoute 的目标地址。两类型互斥：
//   - service 类型：引用某个 Service.ID + 端口（端口名或 ExposedPort），
//     由 netresolve.Resolve 在 apply 时算出 edge 视角的 URL，Service 端口变更
//     下次 apply 自动同步。
//   - raw 类型：用户直接填的 URL 字符串（migrate 来的旧数据 / 外部地址）。
//
// network_pref 控制 Resolver 的网络偏好；override_host/override_port 用于在
// 极少数场景下显式覆盖 Resolver 的选择。
type IngressUpstream struct {
	Type         string `json:"type"` // service | raw
	ServiceID    *uint  `json:"service_id,omitempty"`
	PortName     string `json:"port_name,omitempty"`
	RawURL       string `json:"raw_url,omitempty"`
	NetworkPref  string `json:"network_pref,omitempty"` // auto|loopback|private|vpn|tunnel|public（空=auto）
	OverrideHost string `json:"override_host,omitempty"`
	OverridePort int    `json:"override_port,omitempty"`
}

// Value 实现 driver.Valuer：序列化为 JSON 字符串。
func (u IngressUpstream) Value() (driver.Value, error) {
	b, err := json.Marshal(u)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan 实现 sql.Scanner：从 JSON 字符串反序列化。空值容忍。
func (u *IngressUpstream) Scan(src any) error {
	if src == nil {
		*u = IngressUpstream{}
		return nil
	}
	var raw []byte
	switch v := src.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("ingress_upstream scan: 不支持的源类型 %T", src)
	}
	if len(raw) == 0 {
		*u = IngressUpstream{}
		return nil
	}
	return json.Unmarshal(raw, u)
}

// IngressRoute 是 Ingress 下的一条路由规则，对应渲染出的一个 nginx location 块。
//
// LegacyAppRouteID 是 P0 桥接期临时字段：把新 IngressRoute 与旧 AppNginxRoute 一一
// 对应，便于双写双读保证一致性。P3 完全下线旧表后此字段删除。
type IngressRoute struct {
	ID               uint            `gorm:"primaryKey" json:"id"`
	IngressID        uint            `gorm:"not null;index" json:"ingress_id"`
	Sort             int             `gorm:"default:0" json:"sort"`
	Path             string          `gorm:"not null" json:"path"`
	Protocol         string          `gorm:"default:'http'" json:"protocol"` // http|grpc|ws|tcp|udp
	Upstream         IngressUpstream `gorm:"type:text" json:"upstream"`
	WebSocket        bool            `gorm:"default:false" json:"websocket"`
	Extra            string          `gorm:"default:''" json:"extra"`
	LegacyAppRouteID *uint           `gorm:"index" json:"-"` // 桥接期映射，P3 删除
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

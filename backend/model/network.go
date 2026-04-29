package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// Network 描述一台 Server 上的一个可达入口（loopback / 内网 / VPN / 隧道 / 公网）。
// 多个 Server 之间是否互通由 NetworkID 标签决定（不依赖 CIDR 数学，VPN 网段经常
// 重叠或虚标）。Resolver 在选 upstream URL 时会按 (priority, kind) 排序挑最优。
//
// loopback 由系统自动注入（Server.AfterFind），用户不能删除，确保「同机短路」永远可用。
type Network struct {
	Kind          string `json:"kind"`                     // loopback | private | vpn | tunnel | public
	NetworkID     string `json:"network_id"`               // 同 ID 视为互通
	Address       string `json:"address"`                  // IP 或 hostname
	Priority      int    `json:"priority"`                 // 越小越优；默认见 DefaultPriority
	ReachableFrom []uint `json:"reachable_from,omitempty"` // 仅 tunnel：哪些 server.id 能用此地址
	Label         string `json:"label,omitempty"`          // UI 显示名
}

const (
	NetworkKindLoopback = "loopback"
	NetworkKindPrivate  = "private"
	NetworkKindVPN      = "vpn"
	NetworkKindTunnel   = "tunnel"
	NetworkKindPublic   = "public"
)

// DefaultPriority 返回某 kind 的默认 priority。priority 越小越优先。
// 设计目标：内网 < VPN < 隧道 < 公网，公网始终是兜底。
func DefaultPriority(kind string) int {
	switch kind {
	case NetworkKindLoopback:
		return 0
	case NetworkKindPrivate:
		return 10
	case NetworkKindVPN:
		return 20
	case NetworkKindTunnel:
		return 30
	case NetworkKindPublic:
		return 100
	default:
		return 50
	}
}

// Networks 是 []Network 的 GORM 友好包装：序列化为 JSON 文本存 SQLite text 列。
type Networks []Network

// Value 实现 driver.Valuer。空切片落 "[]"，避免 NULL 让 Scan 出错。
func (n Networks) Value() (driver.Value, error) {
	if n == nil {
		return "[]", nil
	}
	b, err := json.Marshal(n)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan 实现 sql.Scanner。允许 NULL / 空字符串 / "[]" / 完整 JSON。
func (n *Networks) Scan(src any) error {
	if src == nil {
		*n = Networks{}
		return nil
	}
	var raw []byte
	switch v := src.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("networks scan: 不支持的源类型 %T", src)
	}
	if len(raw) == 0 {
		*n = Networks{}
		return nil
	}
	var arr []Network
	if err := json.Unmarshal(raw, &arr); err != nil {
		return fmt.Errorf("networks scan: %w", err)
	}
	*n = arr
	return nil
}

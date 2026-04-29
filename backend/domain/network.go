package domain

import (
	"errors"
	"fmt"
	"strings"
)

// NetworkKind 枚举
const (
	NetworkKindLoopback = "loopback"
	NetworkKindPrivate  = "private"
	NetworkKindVPN      = "vpn"
	NetworkKindTunnel   = "tunnel"
	NetworkKindPublic   = "public"
)

// DefaultPriority 返回某 kind 的默认 priority。priority 越小越优先。
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

// Network 描述一台 Server 上的一个可达入口。
type Network struct {
	Kind          string `json:"kind"`
	NetworkID     string `json:"network_id"`
	Address       string `json:"address"`
	Priority      int    `json:"priority"`
	ReachableFrom []uint `json:"reachable_from,omitempty"`
	Label         string `json:"label,omitempty"`
}

// Validate 校验单条 Network 的字段约束。
func (n *Network) Validate() error {
	switch n.Kind {
	case NetworkKindLoopback, NetworkKindPrivate, NetworkKindVPN,
		NetworkKindTunnel, NetworkKindPublic:
	default:
		return fmt.Errorf("network: 未知 kind %q", n.Kind)
	}
	if strings.TrimSpace(n.Address) == "" {
		return errors.New("network: address 不能为空")
	}
	switch n.Kind {
	case NetworkKindPrivate, NetworkKindVPN:
		if strings.TrimSpace(n.NetworkID) == "" {
			return fmt.Errorf("network: kind=%s 必须填 network_id", n.Kind)
		}
	case NetworkKindTunnel:
		if len(n.ReachableFrom) == 0 {
			return errors.New("network: kind=tunnel 必须填 reachable_from")
		}
	case NetworkKindPublic:
		n.NetworkID = "public"
	}
	return nil
}

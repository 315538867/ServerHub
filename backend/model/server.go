package model

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

type Server struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Name        string     `gorm:"not null" json:"name"`
	Type        string     `gorm:"default:ssh" json:"type"` // "ssh" | "local"
	Host        string     `gorm:"not null" json:"host"`
	Port        int        `gorm:"default:22" json:"port"`
	Username    string     `gorm:"not null" json:"username"`
	AuthType    string     `gorm:"default:password" json:"auth_type"` // "password" | "key" | "local"
	Password    string     `gorm:"default:''" json:"-"`               // AES-GCM encrypted
	PrivateKey  string     `gorm:"default:''" json:"-"`               // AES-GCM encrypted
	Remark      string     `gorm:"default:''" json:"remark"`
	Status      string     `gorm:"default:unknown" json:"status"` // "online"|"offline"|"unknown"
	LastCheckAt *time.Time `json:"last_check_at"`
	// HostKeyFP stores the pinned SSH host-key fingerprint (SHA256:base64,
	// matching ssh.FingerprintSHA256). Populated on first successful dial
	// (TOFU). Later dials must match; mismatches abort the connection.
	HostKeyFP string `gorm:"default:''" json:"host_key_fp"`
	// Capability is meaningful only when Type == "local". Stamped at boot by
	// seedLocalServer based on sysinfo.LocalCapability(). Values:
	//   - "full":   exec/docker/systemd/files all available (bare metal, or
	//               container with --pid=host + /host + sock).
	//   - "docker": only the docker socket is reachable (container with sock
	//               but no host root / pid namespace).
	//   - "":       legacy rows or Type != "local" — frontend treats empty as
	//               "full" for backwards compat.
	Capability string    `gorm:"default:''" json:"capability"`
	// Networks 描述这台 server 的可达入口列表（loopback / 内网 / VPN / 隧道 / 公网），
	// 由 netresolve.Resolve 用来给跨机 upstream 选最优 URL。loopback 由 AfterFind
	// 自动注入，用户不能删除。具体语义见 model/network.go。
	Networks   Networks  `gorm:"type:text;default:'[]'" json:"networks"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// loopbackNetwork 返回该 server 的固定 loopback 条目。
//
// NetworkID 用 "lo-{id}" 而非全局 "loopback"，是为了避免不同 server 的 loopback
// 在 Resolver 的 NetworkID 匹配中被误判为「互通」——loopback 永远只对自己有效，
// 同机短路在 Resolver 算法第 1 步独立处理，不走 NetworkID 匹配路径。
func loopbackNetwork(id uint) Network {
	return Network{
		Kind:      NetworkKindLoopback,
		NetworkID: fmt.Sprintf("lo-%d", id),
		Address:   "127.0.0.1",
		Priority:  DefaultPriority(NetworkKindLoopback),
		Label:     "自身",
	}
}

// AfterFind 钩子：保证返回的 Networks 永远包含 loopback。旧行（升级前没填 Networks
// 的）不需要写库，只在内存里补；新行因 BeforeSave 已经补过。
func (s *Server) AfterFind(_ *gorm.DB) error {
	hasLo := false
	for _, n := range s.Networks {
		if n.Kind == NetworkKindLoopback {
			hasLo = true
			break
		}
	}
	if !hasLo {
		s.Networks = append(Networks{loopbackNetwork(s.ID)}, s.Networks...)
	}
	return nil
}

// BeforeSave 钩子：补默认 priority、强制注入 loopback、(kind, address) 去重、跑 Validate。
func (s *Server) BeforeSave(_ *gorm.DB) error {
	if s.Networks == nil {
		s.Networks = Networks{}
	}
	// 1) 补默认 priority（仅当用户没显式填，约定 0 是有效值，但 loopback 之外 0 也被当作未填）
	for i := range s.Networks {
		n := &s.Networks[i]
		if n.Priority == 0 && n.Kind != NetworkKindLoopback {
			n.Priority = DefaultPriority(n.Kind)
		}
	}
	// 2) 校验
	for i := range s.Networks {
		if err := s.Networks[i].Validate(); err != nil {
			return fmt.Errorf("server.networks[%d]: %w", i, err)
		}
	}
	// 3) (kind,address) 去重，保留第一个
	seen := make(map[string]struct{}, len(s.Networks))
	dedup := make(Networks, 0, len(s.Networks))
	for _, n := range s.Networks {
		key := n.Kind + "|" + n.Address
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		dedup = append(dedup, n)
	}
	s.Networks = dedup
	// 4) 注入 loopback（id=0 时（首次 Create）后续 AfterCreate 再修正 NetworkID）
	hasLo := false
	for _, n := range s.Networks {
		if n.Kind == NetworkKindLoopback {
			hasLo = true
			break
		}
	}
	if !hasLo {
		s.Networks = append(Networks{loopbackNetwork(s.ID)}, s.Networks...)
	}
	return nil
}

// AfterCreate 钩子：首次 Create 时 ID 在 BeforeSave 阶段还是 0，loopback 的 NetworkID
// 会被填成 "lo-0"。这里在 ID 生成后修正一次。
func (s *Server) AfterCreate(tx *gorm.DB) error {
	fixed := false
	for i := range s.Networks {
		n := &s.Networks[i]
		if n.Kind == NetworkKindLoopback && n.NetworkID == "lo-0" {
			n.NetworkID = fmt.Sprintf("lo-%d", s.ID)
			fixed = true
		}
	}
	if fixed {
		return tx.Model(s).Update("networks", s.Networks).Error
	}
	return nil
}

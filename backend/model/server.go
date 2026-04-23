package model

import "time"

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
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

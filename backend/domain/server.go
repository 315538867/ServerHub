package domain

import "time"

// Server 是受管主机的领域实体。
// Status/LastCheckAt 已在 R3 下线,在线状态由 ServerProbe 时序表派生。
type Server struct {
	ID         uint      `json:"id"`
	Name       string    `json:"name"`
	Type       string    `json:"type"` // "ssh" | "local"
	Host       string    `json:"host"`
	Port       int       `json:"port"`
	Username   string    `json:"username"`
	AuthType   string    `json:"auth_type"` // "password" | "key" | "local"
	Password   string    `json:"-"`         // AES-GCM encrypted
	PrivateKey string    `json:"-"`         // AES-GCM encrypted
	Remark     string    `json:"remark"`
	HostKeyFP  string    `json:"host_key_fp"`
	Capability string    `json:"capability"`
	Networks   []Network `json:"networks"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

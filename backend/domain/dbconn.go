package domain

import "time"

// DBConn 是数据库连接配置的领域实体。
// model 中使用 gorm.Model(含 DeletedAt),domain 扁平化。
type DBConn struct {
	ID            uint       `json:"id"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	DeletedAt     *time.Time `json:"-"`
	ServerID      uint       `json:"server_id"`
	ApplicationID *uint      `json:"application_id"`
	Name          string     `json:"name"`
	Type          string     `json:"type"` // mysql / redis
	Host          string     `json:"host"`
	Port          int        `json:"port"`
	Username      string     `json:"username"`
	Password      string     `json:"-"` // AES encrypted
	Database      string     `json:"database"`
}

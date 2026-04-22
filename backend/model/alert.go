package model

import "time"

type AlertRule struct {
	ID        uint    `gorm:"primaryKey" json:"id"`
	ServerID  uint    `gorm:"index" json:"server_id"` // 0 = all servers
	Metric    string  `gorm:"not null" json:"metric"` // cpu / mem / disk / offline
	Operator  string  `gorm:"default:'gt'" json:"operator"` // gt / lt
	Threshold float64 `json:"threshold"`
	Duration  int     `gorm:"default:1" json:"duration"` // consecutive hits before alert
	Enabled   bool    `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AlertEvent struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	RuleID    uint      `gorm:"index" json:"rule_id"`
	ServerID  uint      `gorm:"index" json:"server_id"`
	Value     float64   `json:"value"`
	Message   string    `json:"message"`
	SentAt    time.Time `json:"sent_at"`
}

type NotifyChannel struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"not null" json:"name"`
	Type      string    `gorm:"not null" json:"type"` // webhook_wechat / webhook_dingtalk / webhook_slack / webhook_feishu / webhook_telegram / custom
	URL       string    `json:"-"`                    // AES encrypted
	Template  string    `json:"template"`
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

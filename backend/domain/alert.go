package domain

import "time"

// AlertRule 告警规则。
type AlertRule struct {
	ID        uint      `json:"id"`
	ServerID  uint      `json:"server_id"` // 0 = all servers
	Metric    string    `json:"metric"`     // cpu / mem / disk / offline
	Operator  string    `json:"operator"`   // gt / lt
	Threshold float64   `json:"threshold"`
	Duration  int       `json:"duration"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AlertEvent 告警事件。
type AlertEvent struct {
	ID       uint      `json:"id"`
	RuleID   uint      `json:"rule_id"`
	ServerID uint      `json:"server_id"`
	Value    float64   `json:"value"`
	Message  string    `json:"message"`
	SentAt   time.Time `json:"sent_at"`
}

// NotifyChannel 通知渠道。
type NotifyChannel struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"` // webhook_wechat / webhook_dingtalk / webhook_slack / webhook_feishu / webhook_telegram / custom
	URL       string    `json:"-"`
	Template  string    `json:"template"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

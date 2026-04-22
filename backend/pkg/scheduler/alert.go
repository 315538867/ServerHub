package scheduler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/notify"
	"github.com/serverhub/serverhub/pkg/safehttp"
	"gorm.io/gorm"
)

// hitCounters tracks consecutive alert hits per rule+server pair
var hitCounters sync.Map // key: "ruleID:serverID" → int

func checkAlerts(db *gorm.DB, cfg *config.Config, serverID uint, cpu, mem, disk float64, offline bool) {
	var rules []model.AlertRule
	db.Where("enabled = ? AND (server_id = 0 OR server_id = ?)", true, serverID).Find(&rules)
	if len(rules) == 0 {
		return
	}

	var srv model.Server
	if err := db.First(&srv, serverID).Error; err != nil {
		return
	}

	for _, rule := range rules {
		var value float64
		var triggered bool

		switch rule.Metric {
		case "cpu":
			value = cpu
		case "mem":
			value = mem
		case "disk":
			value = disk
		case "offline":
			triggered = offline
			value = 0
		}

		if rule.Metric != "offline" {
			switch rule.Operator {
			case "gt":
				triggered = value > rule.Threshold
			case "lt":
				triggered = value < rule.Threshold
			}
		}

		key := fmt.Sprintf("%d:%d", rule.ID, serverID)
		if triggered {
			raw, _ := hitCounters.LoadOrStore(key, 0)
			count := raw.(int) + 1
			hitCounters.Store(key, count)
			if count >= rule.Duration {
				// fire alert (only once per duration window — reset counter)
				hitCounters.Store(key, 0)
				fireAlert(db, cfg, rule, srv, value)
			}
		} else {
			hitCounters.Store(key, 0)
		}
	}
}

func fireAlert(db *gorm.DB, cfg *config.Config, rule model.AlertRule, srv model.Server, value float64) {
	msg := fmt.Sprintf("[告警] %s - %s %s %.1f (阈值 %.1f)",
		srv.Name, rule.Metric, rule.Operator, value, rule.Threshold)
	if rule.Metric == "offline" {
		msg = fmt.Sprintf("[告警] 服务器 %s 离线", srv.Name)
	}

	// Save event
	event := model.AlertEvent{
		RuleID:   rule.ID,
		ServerID: srv.ID,
		Value:    value,
		Message:  msg,
		SentAt:   time.Now(),
	}
	db.Create(&event)

	// Desktop notification
	notify.Send("ServerHub 告警", msg)

	// Send to all enabled channels
	var channels []model.NotifyChannel
	db.Where("enabled = ?", true).Find(&channels)
	for _, ch := range channels {
		go sendWebhook(cfg, ch, srv, rule, value, msg)
	}
}

func sendWebhook(cfg *config.Config, ch model.NotifyChannel, srv model.Server, rule model.AlertRule, value float64, defaultMsg string) {
	if ch.URL == "" {
		return
	}
	rawURL, err := crypto.Decrypt(ch.URL, cfg.Security.AESKey)
	if err != nil {
		log.Printf("alert: channel %d decrypt URL failed: %v", ch.ID, err)
		return
	}

	// Build message from template or use default
	text := defaultMsg
	if ch.Template != "" {
		text = strings.NewReplacer(
			"{{.Server}}", srv.Name,
			"{{.Metric}}", rule.Metric,
			"{{.Value}}", fmt.Sprintf("%.1f", value),
			"{{.Time}}", time.Now().Format("2006-01-02 15:04:05"),
		).Replace(ch.Template)
	}

	payload := BuildWebhookPayload(ch.Type, text)

	req, err := http.NewRequest("POST", rawURL, strings.NewReader(string(payload)))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if err := safehttp.ValidateOutboundURL(rawURL); err != nil {
		return
	}
	client := safehttp.Client(10 * time.Second)
	resp, err := client.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}

// BuildWebhookPayload returns a JSON body formatted for the given webhook provider.
func BuildWebhookPayload(chType, text string) []byte {
	var payload []byte
	switch chType {
	case "webhook_wechat", "webhook_dingtalk":
		payload, _ = json.Marshal(map[string]any{"msgtype": "text", "text": map[string]string{"content": text}})
	case "webhook_slack":
		payload, _ = json.Marshal(map[string]string{"text": text})
	case "webhook_feishu":
		payload, _ = json.Marshal(map[string]any{"msg_type": "text", "content": map[string]string{"text": text}})
	case "webhook_telegram":
		payload, _ = json.Marshal(map[string]string{"text": text})
	default: // custom
		payload, _ = json.Marshal(map[string]string{"content": text, "message": text})
	}
	return payload
}

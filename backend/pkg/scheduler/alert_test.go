package scheduler

import (
	"encoding/json"
	"testing"
)

func TestBuildWebhookPayload(t *testing.T) {
	cases := []struct {
		chType string
		want   map[string]any
	}{
		{"webhook_wechat", map[string]any{"msgtype": "text", "text": map[string]any{"content": "hi"}}},
		{"webhook_dingtalk", map[string]any{"msgtype": "text", "text": map[string]any{"content": "hi"}}},
		{"webhook_slack", map[string]any{"text": "hi"}},
		{"webhook_feishu", map[string]any{"msg_type": "text", "content": map[string]any{"text": "hi"}}},
		{"webhook_telegram", map[string]any{"text": "hi"}},
		{"custom", map[string]any{"content": "hi", "message": "hi"}},
	}
	for _, c := range cases {
		t.Run(c.chType, func(t *testing.T) {
			var got map[string]any
			if err := json.Unmarshal(BuildWebhookPayload(c.chType, "hi"), &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if !eqJSON(got, c.want) {
				t.Errorf("payload = %v, want %v", got, c.want)
			}
		})
	}
}

func eqJSON(a, b any) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(ab) == string(bb)
}

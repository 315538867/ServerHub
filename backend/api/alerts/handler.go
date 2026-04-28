package alerts

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/pkg/safehttp"
	"github.com/serverhub/serverhub/pkg/scheduler"
	"github.com/serverhub/serverhub/repo"
	"gorm.io/gorm"
)

func RegisterRoutes(r *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	// Rules
	r.GET("/rules", listRules(db))
	r.POST("/rules", createRule(db))
	r.PUT("/rules/:id", updateRule(db))
	r.DELETE("/rules/:id", deleteRule(db))

	// Events
	r.GET("/events", listEvents(db))
	r.DELETE("/events", clearEvents(db))

	// Channels
	r.GET("/channels", listChannels(db))
	r.POST("/channels", createChannel(db, cfg))
	r.PUT("/channels/:id", updateChannel(db, cfg))
	r.DELETE("/channels/:id", deleteChannel(db))
	r.POST("/channels/:id/test", testChannel(db, cfg))
}

// ── Rules ─────────────────────────────────────────────────────────────────────

func listRules(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rules, err := repo.ListAllAlertRules(c.Request.Context(), db)
		if err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, rules)
	}
}

func createRule(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			ServerID  uint    `json:"server_id"`
			Metric    string  `json:"metric"    binding:"required"`
			Operator  string  `json:"operator"`
			Threshold float64 `json:"threshold"`
			Duration  int     `json:"duration"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, "指标类型不能为空")
			return
		}
		if body.Operator == "" {
			body.Operator = "gt"
		}
		if body.Duration < 1 {
			body.Duration = 1
		}
		rule := model.AlertRule{
			ServerID: body.ServerID, Metric: body.Metric, Operator: body.Operator,
			Threshold: body.Threshold, Duration: body.Duration, Enabled: true,
		}
		if err := repo.CreateAlertRule(c.Request.Context(), db, &rule); err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, rule)
	}
}

func updateRule(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))
		rule, err := repo.GetAlertRuleByID(c.Request.Context(), db, uint(id))
		if err != nil {
			resp.NotFound(c, "资源不存在")
			return
		}
		var body struct {
			Operator  *string  `json:"operator"`
			Threshold *float64 `json:"threshold"`
			Duration  *int     `json:"duration"`
			Enabled   *bool    `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, "请求体格式错误")
			return
		}
		if body.Operator != nil {
			rule.Operator = *body.Operator
		}
		if body.Threshold != nil {
			rule.Threshold = *body.Threshold
		}
		if body.Duration != nil {
			rule.Duration = *body.Duration
		}
		if body.Enabled != nil {
			rule.Enabled = *body.Enabled
		}
		if err := repo.SaveAlertRule(c.Request.Context(), db, &rule); err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, rule)
	}
}

func deleteRule(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))
		_ = repo.DeleteAlertRule(c.Request.Context(), db, uint(id))
		resp.OK(c, nil)
	}
}

// ── Events ────────────────────────────────────────────────────────────────────

func listEvents(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		size, _ := strconv.Atoi(c.DefaultQuery("size", "50"))
		if page < 1 {
			page = 1
		}
		if size < 1 || size > 200 {
			size = 50
		}
		events, total, err := repo.ListAlertEventsPaginated(c.Request.Context(), db, (page-1)*size, size)
		if err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, gin.H{"total": total, "events": events})
	}
}

func clearEvents(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_ = repo.PruneAlertEventsBefore(c.Request.Context(), db, time.Now().AddDate(0, 0, -30))
		resp.OK(c, nil)
	}
}

// ── Channels ──────────────────────────────────────────────────────────────────

type channelResp struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Template  string    `json:"template"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

func listChannels(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		channels, err := repo.ListAllNotifyChannels(c.Request.Context(), db)
		if err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		result := make([]channelResp, len(channels))
		for i, ch := range channels {
			result[i] = channelResp{ID: ch.ID, Name: ch.Name, Type: ch.Type,
				Template: ch.Template, Enabled: ch.Enabled, CreatedAt: ch.CreatedAt}
		}
		resp.OK(c, result)
	}
}

func createChannel(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Name     string `json:"name"     binding:"required"`
			Type     string `json:"type"     binding:"required"`
			URL      string `json:"url"      binding:"required"`
			Template string `json:"template"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, "名称、类型和 URL 不能为空")
			return
		}
		if err := safehttp.ValidateOutboundURL(body.URL); err != nil {
			resp.BadRequest(c, "URL 不允许: "+err.Error())
			return
		}
		encURL, err := crypto.Encrypt(body.URL, cfg.Security.AESKey)
		if err != nil {
			resp.InternalError(c, "加密失败")
			return
		}
		ch := model.NotifyChannel{Name: body.Name, Type: body.Type, URL: encURL,
			Template: body.Template, Enabled: true}
		if err := repo.CreateNotifyChannel(c.Request.Context(), db, &ch); err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, channelResp{ID: ch.ID, Name: ch.Name, Type: ch.Type,
			Template: ch.Template, Enabled: ch.Enabled, CreatedAt: ch.CreatedAt})
	}
}

func updateChannel(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))
		ch, err := repo.GetNotifyChannelByID(c.Request.Context(), db, uint(id))
		if err != nil {
			resp.NotFound(c, "资源不存在")
			return
		}
		var body struct {
			Name     *string `json:"name"`
			URL      *string `json:"url"`
			Template *string `json:"template"`
			Enabled  *bool   `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, "请求体格式错误")
			return
		}
		if body.Name != nil {
			ch.Name = *body.Name
		}
		if body.URL != nil && *body.URL != "" {
			if err := safehttp.ValidateOutboundURL(*body.URL); err != nil {
				resp.BadRequest(c, "URL 不允许: "+err.Error())
				return
			}
			enc, err := crypto.Encrypt(*body.URL, cfg.Security.AESKey)
			if err == nil {
				ch.URL = enc
			}
		}
		if body.Template != nil {
			ch.Template = *body.Template
		}
		if body.Enabled != nil {
			ch.Enabled = *body.Enabled
		}
		if err := repo.SaveNotifyChannel(c.Request.Context(), db, &ch); err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, channelResp{ID: ch.ID, Name: ch.Name, Type: ch.Type,
			Template: ch.Template, Enabled: ch.Enabled, CreatedAt: ch.CreatedAt})
	}
}

func deleteChannel(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))
		_ = repo.DeleteNotifyChannel(c.Request.Context(), db, uint(id))
		resp.OK(c, nil)
	}
}

func testChannel(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))
		ch, err := repo.GetNotifyChannelByID(c.Request.Context(), db, uint(id))
		if err != nil {
			resp.NotFound(c, "资源不存在")
			return
		}
		rawURL, err := crypto.Decrypt(ch.URL, cfg.Security.AESKey)
		if err != nil {
			resp.InternalError(c, "解密失败")
			return
		}
		testRule := model.AlertRule{Metric: "cpu", Operator: "gt", Threshold: 80}
		testSrv := model.Server{Name: "测试服务器"}
		go sendTestWebhook(ch.Type, rawURL, ch.Template, testSrv, testRule)
		resp.OK(c, gin.H{"message": "测试通知已发送"})
	}
}

func sendTestWebhook(chType, rawURL, template string, srv model.Server, rule model.AlertRule) {
	msg := "[测试] ServerHub 告警通知测试消息"
	if template != "" {
		msg = strings.NewReplacer(
			"{{.Server}}", srv.Name,
			"{{.Metric}}", rule.Metric,
			"{{.Value}}", "99.9",
			"{{.Time}}", time.Now().Format("2006-01-02 15:04:05"),
		).Replace(template)
	}
	sendWebhookRaw(chType, rawURL, msg)
}

func sendWebhookRaw(chType, rawURL, text string) {
	if err := safehttp.ValidateOutboundURL(rawURL); err != nil {
		return
	}
	payload := scheduler.BuildWebhookPayload(chType, text)
	req, err := http.NewRequest("POST", rawURL, strings.NewReader(string(payload)))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	client := safehttp.Client(10 * time.Second)
	r, err := client.Do(req)
	if err == nil {
		r.Body.Close()
	}
}

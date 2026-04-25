package deploy

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/auditq"
	"github.com/serverhub/serverhub/pkg/deployer"
	"github.com/serverhub/serverhub/pkg/resp"
	"gorm.io/gorm"
)

// maxWebhookBody caps Git provider payloads we'll read into memory.
const maxWebhookBody = 1 << 20 // 1 MiB

func RegisterWebhookRoutes(r *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	r.POST("/:token", webhookHandler(db, cfg))
}

// webhookHandler authenticates the sender via either:
//   - X-Hub-Signature-256 (GitHub): HMAC-SHA256(webhookSecret, body)
//   - X-Gitlab-Token (GitLab): raw secret compared with constant-time
//
// Requests carrying neither header are rejected so a leaked URL alone cannot
// trigger a deploy. On auth success the Service's current Release is re-applied
// via deployer.ApplyRelease (M3: legacy deployer.Run is gone).
func webhookHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Param("token")
		var svc model.Service
		if err := db.Where("webhook_secret = ?", token).First(&svc).Error; err != nil {
			resp.NotFound(c, "资源不存在")
			return
		}

		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxWebhookBody)
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			resp.Fail(c, http.StatusRequestEntityTooLarge, 413, "请求体过大")
			return
		}

		ghSig := c.GetHeader("X-Hub-Signature-256")
		glTok := c.GetHeader("X-Gitlab-Token")
		if ghSig == "" && glTok == "" {
			auditq.Security("webhook", c.ClientIP(), "security:webhook_signature_failed", 401,
				map[string]any{"service_id": svc.ID, "reason": "missing_header"})
			resp.Fail(c, http.StatusUnauthorized, 401, "缺少签名或 Token 请求头")
			return
		}

		secret := []byte(svc.WebhookSecret)

		if ghSig != "" {
			mac := hmac.New(sha256.New, secret)
			mac.Write(body)
			expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
			if !hmac.Equal([]byte(ghSig), []byte(expected)) {
				auditq.Security("webhook", c.ClientIP(), "security:webhook_signature_failed", 401,
					map[string]any{"service_id": svc.ID, "provider": "github"})
				resp.Fail(c, http.StatusUnauthorized, 401, "签名验证失败")
				return
			}
		} else if glTok != "" {
			if !hmac.Equal([]byte(glTok), secret) {
				auditq.Security("webhook", c.ClientIP(), "security:webhook_signature_failed", 401,
					map[string]any{"service_id": svc.ID, "provider": "gitlab"})
				resp.Fail(c, http.StatusUnauthorized, 401, "Token 不匹配")
				return
			}
		}

		if svc.CurrentReleaseID == nil {
			resp.Fail(c, http.StatusConflict, 409, "Service 尚未绑定 Release，请先在服务详情页创建并应用一次 Release")
			return
		}
		go func(serviceID, releaseID uint) {
			_, _ = deployer.ApplyRelease(db, cfg, serviceID, releaseID, "webhook", nil)
		}(svc.ID, *svc.CurrentReleaseID)
		resp.OK(c, gin.H{"triggered": true, "release_id": *svc.CurrentReleaseID})
	}
}

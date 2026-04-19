package deploy

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/deployer"
	"github.com/serverhub/serverhub/pkg/resp"
	"gorm.io/gorm"
)

func RegisterWebhookRoutes(r *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	r.POST("/:token", webhookHandler(db, cfg))
}

func webhookHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Param("token")
		var d model.Deploy
		if err := db.Where("webhook_secret = ?", token).First(&d).Error; err != nil {
			resp.NotFound(c, "资源不存在")
			return
		}

		body, _ := io.ReadAll(c.Request.Body)

		// Verify GitHub signature if present
		if sig := c.GetHeader("X-Hub-Signature-256"); sig != "" {
			mac := hmac.New(sha256.New, []byte(token))
			mac.Write(body)
			expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
			if !hmac.Equal([]byte(sig), []byte(expected)) {
				resp.Fail(c, 401, 401, "签名验证失败")
				return
			}
		}

		// GitLab: X-Gitlab-Token is the raw token
		if gitlabToken := c.GetHeader("X-Gitlab-Token"); gitlabToken != "" {
			if gitlabToken != token {
				resp.Fail(c, 401, 401, "Token 不匹配")
				return
			}
		}

		go deployer.Run(db, cfg, d, nil)

		resp.OK(c, gin.H{"triggered": true})
	}
}

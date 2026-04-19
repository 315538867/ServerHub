package auth

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/middleware"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/pkg/totp"
	"gorm.io/gorm"
)

func totpSetupHandler(db *gorm.DB, _ *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := middleware.GetClaims(c)
		if claims == nil {
			resp.Unauthorized(c, "not authenticated")
			return
		}
		secret, err := totp.GenerateSecret()
		if err != nil {
			resp.InternalError(c, "生成密钥失败")
			return
		}
		uri := totp.OtpAuthURI(secret, claims.Username, "ServerHub")
		resp.OK(c, gin.H{"secret": secret, "uri": uri})
	}
}

func totpConfirmHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := middleware.GetClaims(c)
		if claims == nil {
			resp.Unauthorized(c, "not authenticated")
			return
		}
		var body struct {
			Secret string `json:"secret" binding:"required"`
			Code   string `json:"code"   binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, "密钥和验证码不能为空")
			return
		}
		if !totp.Verify(body.Secret, body.Code) {
			resp.Fail(c, http.StatusUnprocessableEntity, 1010, "验证码格式错误")
			return
		}
		encSecret, err := crypto.Encrypt(body.Secret, cfg.Security.AESKey)
		if err != nil {
			resp.InternalError(c, "加密失败")
			return
		}
		db.Model(&model.User{}).Where("id = ?", claims.UserID).Updates(map[string]any{
			"mfa_secret":  encSecret,
			"mfa_enabled": true,
		})
		resp.OK(c, nil)
	}
}

func totpDisableHandler(db *gorm.DB, _ *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := middleware.GetClaims(c)
		if claims == nil {
			resp.Unauthorized(c, "not authenticated")
			return
		}
		db.Model(&model.User{}).Where("id = ?", claims.UserID).Updates(map[string]any{
			"mfa_secret":  "",
			"mfa_enabled": false,
		})
		resp.OK(c, nil)
	}
}

func totpLoginHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			TmpToken string `json:"tmp_token" binding:"required"`
			Code     string `json:"code"      binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, "临时 Token 和验证码不能为空")
			return
		}
		// Parse tmp token (role=tmp_totp)
		claims, err := middleware.ParseToken(body.TmpToken, cfg.Security.JWTSecret)
		if err != nil || claims.Role != "tmp_totp" {
			resp.Fail(c, http.StatusUnauthorized, 1001, "Token 无效")
			return
		}
		var user model.User
		if err := db.First(&user, claims.UserID).Error; err != nil {
			resp.Fail(c, http.StatusUnauthorized, 1001, "用户不存在")
			return
		}
		secret, err := crypto.Decrypt(user.MFASecret, cfg.Security.AESKey)
		if err != nil {
			resp.Fail(c, http.StatusUnauthorized, 1011, "TOTP 密钥无效")
			return
		}
		if !totp.Verify(secret, body.Code) {
			resp.Fail(c, http.StatusUnauthorized, 1011, "验证码错误")
			return
		}
		token, err := signToken(&user, cfg.Security.JWTSecret)
		if err != nil {
			resp.InternalError(c, "生成 Token 失败")
			return
		}
		now := time.Now()
		db.Model(&user).Updates(model.User{LastLogin: &now, LastIP: c.ClientIP()})
		resp.OK(c, loginResp{Token: token, User: user})
	}
}

package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/middleware"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/pkg/totp"
	"github.com/serverhub/serverhub/usecase"
	"github.com/serverhub/serverhub/repo"
)

func totpSetupHandler(db repo.DB, _ *config.Config) gin.HandlerFunc {
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

func totpConfirmHandler(db repo.DB, cfg *config.Config) gin.HandlerFunc {
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
		status, code, err := usecase.ConfirmTOTP(c.Request.Context(), db, cfg, claims.UserID, body.Secret, body.Code)
		if err != nil {
			if status != 0 {
				resp.Fail(c, status, code, err.Error())
			} else {
				resp.InternalError(c, err.Error())
			}
			return
		}
		resp.OK(c, nil)
	}
}

func totpDisableHandler(db repo.DB, _ *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := middleware.GetClaims(c)
		if claims == nil {
			resp.Unauthorized(c, "not authenticated")
			return
		}
		if err := usecase.DisableTOTP(c.Request.Context(), db, claims.UserID); err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, nil)
	}
}

func totpLoginHandler(db repo.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			TmpToken string `json:"tmp_token" binding:"required"`
			Code     string `json:"code"      binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, "临时 Token 和验证码不能为空")
			return
		}
		out, status, code, err := usecase.LoginWithTOTP(
			c.Request.Context(), db, cfg,
			body.TmpToken, body.Code, c.ClientIP(),
			middleware.ParseToken, signToken,
		)
		if err != nil {
			if status != 0 {
				resp.Fail(c, status, code, err.Error())
			} else {
				resp.InternalError(c, err.Error())
			}
			return
		}
		resp.OK(c, loginResp{Token: out.Token, User: out.User})
	}
}

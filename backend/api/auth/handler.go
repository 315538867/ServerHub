package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/middleware"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/usecase"
	"gorm.io/gorm"
	"time"
)

func RegisterRoutes(r *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	r.POST("/login", loginHandler(db, cfg))
	r.POST("/logout", func(c *gin.Context) { resp.OK(c, nil) })
	r.GET("/me", middleware.Auth(cfg), meHandler(db))
	r.PUT("/profile", middleware.Auth(cfg), updateProfileHandler(db))
	r.POST("/totp/setup", middleware.Auth(cfg), totpSetupHandler(db, cfg))
	r.POST("/totp/confirm", middleware.Auth(cfg), totpConfirmHandler(db, cfg))
	r.POST("/totp/disable", middleware.Auth(cfg), totpDisableHandler(db, cfg))
	r.POST("/totp/login", totpLoginHandler(db, cfg))
}

type loginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type loginResp struct {
	Token string     `json:"token"`
	User  model.User `json:"user"`
}

func loginHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req loginReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, "用户名和密码不能为空")
			return
		}
		out, status, code, err := usecase.Login(
			c.Request.Context(), db, cfg,
			req.Username, req.Password, c.ClientIP(),
			signToken, signTmpToken,
		)
		if err != nil {
			if status != 0 {
				resp.Fail(c, status, code, err.Error())
			} else {
				resp.InternalError(c, err.Error())
			}
			return
		}
		if out.RequireTOTP {
			resp.OK(c, gin.H{"require_totp": true, "tmp_token": out.TmpToken})
			return
		}
		resp.OK(c, loginResp{Token: out.Token, User: out.User})
	}
}

func meHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := middleware.GetClaims(c)
		if claims == nil {
			resp.Unauthorized(c, "not authenticated")
			return
		}
		user, err := usecase.GetCurrentUser(c.Request.Context(), db, claims.UserID)
		if err != nil {
			resp.NotFound(c, "用户不存在")
			return
		}
		resp.OK(c, user)
	}
}

type updateProfileReq struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewUsername string `json:"new_username"`
	NewPassword string `json:"new_password"`
}

func updateProfileHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := middleware.GetClaims(c)
		if claims == nil {
			resp.Unauthorized(c, "not authenticated")
			return
		}
		var req updateProfileReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, "旧密码不能为空")
			return
		}
		user, status, code, err := usecase.UpdateProfile(
			c.Request.Context(), db, claims.UserID,
			req.OldPassword, req.NewUsername, req.NewPassword,
		)
		if err != nil {
			if status != 0 {
				resp.Fail(c, status, code, err.Error())
			} else {
				resp.InternalError(c, err.Error())
			}
			return
		}
		resp.OK(c, user)
	}
}

func signToken(user *model.User, secret string) (string, error) {
	claims := middleware.Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

func signTmpToken(user *model.User, secret string) (string, error) {
	claims := middleware.Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     "tmp_totp",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

package auth

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/middleware"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/resp"
	"gorm.io/gorm"
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

		var user model.User
		if err := db.Where("username = ?", req.Username).First(&user).Error; err != nil {
			resp.Fail(c, http.StatusUnauthorized, 1001, "用户名或密码错误")
			return
		}

		if !crypto.BcryptVerify(req.Password, user.Password) {
			resp.Fail(c, http.StatusUnauthorized, 1001, "用户名或密码错误")
			return
		}

		// MFA: return tmp token for second step
		if user.MFAEnabled && user.MFASecret != "" {
			tmpToken, err := signTmpToken(&user, cfg.Security.JWTSecret)
			if err != nil {
				resp.InternalError(c, "生成 Token 失败")
				return
			}
			resp.OK(c, gin.H{"require_totp": true, "tmp_token": tmpToken})
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

func meHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := middleware.GetClaims(c)
		if claims == nil {
			resp.Unauthorized(c, "not authenticated")
			return
		}
		var user model.User
		if err := db.First(&user, claims.UserID).Error; err != nil {
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

		var user model.User
		if err := db.First(&user, claims.UserID).Error; err != nil {
			resp.NotFound(c, "用户不存在")
			return
		}

		if !crypto.BcryptVerify(req.OldPassword, user.Password) {
			resp.Fail(c, http.StatusUnprocessableEntity, 1002, "旧密码错误")
			return
		}

		updates := map[string]any{}
		if req.NewUsername != "" && req.NewUsername != user.Username {
			var count int64
			db.Model(&model.User{}).Where("username = ? AND id != ?", req.NewUsername, user.ID).Count(&count)
			if count > 0 {
				resp.Fail(c, http.StatusConflict, 1003, "用户名已存在")
				return
			}
			updates["username"] = req.NewUsername
		}
		if req.NewPassword != "" {
			hash, err := crypto.BcryptHash(req.NewPassword)
			if err != nil {
				resp.InternalError(c, "密码加密失败")
				return
			}
			updates["password"] = hash
		}

		if len(updates) > 0 {
			if err := db.Model(&user).Updates(updates).Error; err != nil {
				resp.InternalError(c, "更新失败")
				return
			}
		}

		db.First(&user, claims.UserID)
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

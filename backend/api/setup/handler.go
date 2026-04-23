// Package setup implements the first-run wizard endpoints.
//
// Scope (intentionally minimal): the wizard exists solely to create the first
// admin account when no users exist yet. "Local server takeover" was removed
// in v0.3.7-beta.16 — the host is now managed via the docker.sock + /host
// mounts probed at boot by sysinfo.LocalCapability(), so no user-driven
// "bind SSH back to the host" step is necessary.
//
// Endpoints:
//
//	1. GET  /panel/api/v1/setup/status → { needs_admin, containerized }
//	2. POST /panel/api/v1/setup/admin  → creates the first admin user
//
// Safety gate: /admin is rejected once any user exists.
package setup

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/auditq"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/pkg/sysinfo"
	"gorm.io/gorm"
)

func RegisterRoutes(r *gin.RouterGroup, db *gorm.DB) {
	r.GET("/status", statusHandler(db))
	r.POST("/admin", createAdminHandler(db))
}

type statusResp struct {
	Containerized bool `json:"containerized"`
	NeedsAdmin    bool `json:"needs_admin"`
}

func statusHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var userCount int64
		db.Model(&model.User{}).Count(&userCount)
		resp.OK(c, statusResp{
			Containerized: sysinfo.IsContainerized(),
			NeedsAdmin:    userCount == 0,
		})
	}
}

type adminReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func createAdminHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req adminReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, "用户名和密码不能为空")
			return
		}
		req.Username = strings.TrimSpace(req.Username)
		if len(req.Username) < 3 || len(req.Password) < 6 {
			resp.BadRequest(c, "用户名至少 3 字符，密码至少 6 字符")
			return
		}

		var count int64
		db.Model(&model.User{}).Count(&count)
		if count > 0 {
			auditq.Security(req.Username, c.ClientIP(), "security:setup_admin_blocked", 409, nil)
			resp.Fail(c, http.StatusConflict, 1003, "已经初始化过管理员")
			return
		}

		hash, err := crypto.BcryptHash(req.Password)
		if err != nil {
			resp.InternalError(c, "密码加密失败")
			return
		}
		now := time.Now()
		user := model.User{
			Username:  req.Username,
			Password:  hash,
			Role:      "admin",
			LastLogin: &now,
		}
		if err := db.Create(&user).Error; err != nil {
			resp.InternalError(c, "创建管理员失败: "+err.Error())
			return
		}
		auditq.Security(req.Username, c.ClientIP(), "security:setup_admin_created", 200, nil)
		resp.OK(c, gin.H{"username": user.Username})
	}
}

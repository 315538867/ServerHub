// Package setup implements the first-run wizard endpoints.
//
// Scope (intentionally minimal): the wizard exists solely to create the first
// admin account when no users exist yet. "Local server takeover" was removed
// in v0.3.7-beta.16 — the host is now managed via the docker.sock + /host
// mounts probed at boot by sysinfo.LocalCapability(), so no user-driven
// "bind SSH back to the host" step is necessary.
package setup

import (
	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/pkg/sysinfo"
	"github.com/serverhub/serverhub/usecase"
	"github.com/serverhub/serverhub/repo"
)

func RegisterRoutes(r *gin.RouterGroup, db repo.DB) {
	r.GET("/status", statusHandler(db))
	r.POST("/admin", createAdminHandler(db))
}

type statusResp struct {
	Containerized bool `json:"containerized"`
	NeedsAdmin    bool `json:"needs_admin"`
}

func statusHandler(db repo.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		needsAdmin, err := usecase.SetupStatus(c.Request.Context(), db)
		if err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, statusResp{
			Containerized: sysinfo.IsContainerized(),
			NeedsAdmin:    needsAdmin,
		})
	}
}

type adminReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func createAdminHandler(db repo.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req adminReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, "用户名和密码不能为空")
			return
		}
		user, status, code, err := usecase.CreateFirstAdmin(c.Request.Context(), db, req.Username, req.Password, c.ClientIP())
		if err != nil {
			if status != 0 {
				resp.Fail(c, status, code, err.Error())
			} else {
				resp.InternalError(c, err.Error())
			}
			return
		}
		resp.OK(c, gin.H{"username": user.Username})
	}
}

// Package discovery (api) exposes the service-discovery HTTP endpoints.
//
// R4 起本 handler 不再直接 import pkg/discovery + pkg/takeover,改为编排
// usecase + adapters/source/<kind>。具体职责切分:
//   - 路由 + 入参 binding 在本文件
//   - 远端扫描 / 接管编排在 usecase.DiscoverServer / usecase.RunTakeover
//   - kind 派发 + 远端 step 链在 adapters/source/<kind>(由 init() 自注册到 source.Default)
package discovery

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/core/source"
	"github.com/serverhub/serverhub/infra"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/usecase"
	"gorm.io/gorm"
)

func RegisterRoutes(r *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	r.GET(":id/discover", scanHandler(db, cfg))
	r.POST(":id/discover/import", importHandler(db, cfg))
	r.POST(":id/discover/takeover", takeoverHandler(db, cfg))
}

func scanHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, ok := findServer(c, db)
		if !ok {
			return
		}
		r, err := infra.For(&s, cfg)
		if err != nil {
			resp.InternalError(c, "runner: "+err.Error())
			return
		}
		var kinds []string
		if raw := strings.TrimSpace(c.Query("kinds")); raw != "" {
			kinds = strings.Split(raw, ",")
		}
		out := usecase.DiscoverServer(c.Request.Context(), db, r, s.ID, kinds)
		resp.OK(c, out)
	}
}

type importReq struct {
	Docker  []source.Candidate `json:"docker"`
	Compose []source.Candidate `json:"compose"`
	Systemd []source.Candidate `json:"systemd"`
	Nginx   []source.Candidate `json:"nginx"`
}

func importHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, ok := findServer(c, db)
		if !ok {
			return
		}
		var req importReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		all := make([]source.Candidate, 0,
			len(req.Docker)+len(req.Compose)+len(req.Systemd)+len(req.Nginx))
		all = append(all, req.Docker...)
		all = append(all, req.Compose...)
		all = append(all, req.Systemd...)
		all = append(all, req.Nginx...)
		out := usecase.ImportCandidates(db, s.ID, all, cfg.Security.AESKey)
		resp.OK(c, out)
	}
}

type takeoverReq struct {
	Candidate  source.Candidate `json:"candidate" binding:"required"`
	TargetName string           `json:"target_name" binding:"required"`

	AppMode string `json:"app_mode,omitempty"`
	AppID   *uint  `json:"app_id,omitempty"`
	AppName string `json:"app_name,omitempty"`
}

func takeoverHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, ok := findServer(c, db)
		if !ok {
			return
		}
		var req takeoverReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		r, err := infra.For(&s, cfg)
		if err != nil {
			resp.InternalError(c, "runner: "+err.Error())
			return
		}
		out := usecase.RunTakeover(c.Request.Context(), db, cfg, &s, r, usecase.TakeoverRequest{
			Cand:       req.Candidate,
			TargetName: req.TargetName,
			AppMode:    req.AppMode,
			AppID:      req.AppID,
			AppName:    req.AppName,
		})
		resp.OK(c, out)
	}
}

func findServer(c *gin.Context, db *gorm.DB) (model.Server, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.BadRequest(c, "ID 格式错误")
		return model.Server{}, false
	}
	var s model.Server
	if err := db.First(&s, id).Error; err != nil {
		resp.NotFound(c, "服务器不存在")
		return model.Server{}, false
	}
	return s, true
}

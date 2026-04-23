// Package discovery (api) exposes the service-discovery HTTP endpoints. It
// scans a managed server for running Docker / compose / systemd services and
// lets the operator selectively import them as Deploy rows.
package discovery

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/discovery"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/pkg/runner"
	"github.com/serverhub/serverhub/pkg/takeover"
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
		rn, err := runner.For(&s, cfg)
		if err != nil {
			resp.InternalError(c, "runner: "+err.Error())
			return
		}
		var kinds []string
		if raw := strings.TrimSpace(c.Query("kinds")); raw != "" {
			kinds = strings.Split(raw, ",")
		}
		result := discovery.Scan(rn, kinds)
		resp.OK(c, result)
	}
}

type importReq struct {
	Docker  []discovery.Candidate `json:"docker"`
	Compose []discovery.Candidate `json:"compose"`
	Systemd []discovery.Candidate `json:"systemd"`
	Nginx   []discovery.Candidate `json:"nginx"`
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
		all := make([]discovery.Candidate, 0,
			len(req.Docker)+len(req.Compose)+len(req.Systemd)+len(req.Nginx))
		all = append(all, req.Docker...)
		all = append(all, req.Compose...)
		all = append(all, req.Systemd...)
		all = append(all, req.Nginx...)
		out := discovery.Import(db, s.ID, all, cfg.Security.AESKey)
		resp.OK(c, out)
	}
}

type takeoverReq struct {
	Candidate  discovery.Candidate `json:"candidate" binding:"required"`
	TargetName string              `json:"target_name" binding:"required"`
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
		out := takeover.Run(db, cfg, s, takeover.Request{
			Candidate:  req.Candidate,
			TargetName: req.TargetName,
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

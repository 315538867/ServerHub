// Package deploy 在 M3 阶段降级为旧 Deploy 数据的只读历史入口。
//
// 历史写路径（create/update/delete/run/rollback/upload/env PUT）已全部迁至
// Release 链路（见 backend/api/release + backend/pkg/deployer/release_apply.go）。
// 本包保留 GET 接口是为了让仪表盘继续展示 M2 迁移之前留下的 deploy_logs /
// deploy_versions / env_vars 内容；不新增写能力，并将在 M4 彻底移除。
package deploy

import (
	"encoding/json"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/resp"
	"gorm.io/gorm"
)

const legacyVersionPageLimit = 20

func RegisterRoutes(r *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	r.GET("", listHandler(db))
	r.GET("/:id", getHandler(db))
	r.GET("/:id/logs", logsHandler(db))
	r.GET("/:id/env", getEnvHandler(db, cfg))
	r.GET("/:id/versions", listVersionsHandler(db))
	r.GET("/:id/versions/:vid", getVersionHandler(db))
}

// ── Read-only handlers ────────────────────────────────────────────────────

func listHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var services []model.Service
		db.Order("id asc").Find(&services)
		resp.OK(c, services)
	}
}

func getHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		d, ok := findDeploy(c, db)
		if !ok {
			return
		}
		resp.OK(c, d)
	}
}

func logsHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		d, ok := findDeploy(c, db)
		if !ok {
			return
		}
		limit := 20
		if l := c.Query("limit"); l != "" {
			if v, _ := strconv.Atoi(l); v > 0 && v <= 100 {
				limit = v
			}
		}
		var logs []model.DeployLog
		db.Where("deploy_id = ?", d.ID).Order("created_at desc").Limit(limit).Find(&logs)
		resp.OK(c, logs)
	}
}

func listVersionsHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		d, ok := findDeploy(c, db)
		if !ok {
			return
		}
		var versions []model.DeployVersion
		db.Where("deploy_id = ?", d.ID).
			Order("created_at DESC").
			Limit(legacyVersionPageLimit).
			Find(&versions)
		resp.OK(c, versions)
	}
}

func getVersionHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		d, ok := findDeploy(c, db)
		if !ok {
			return
		}
		vid, err := strconv.Atoi(c.Param("vid"))
		if err != nil {
			resp.BadRequest(c, "版本 ID 格式错误")
			return
		}
		var v model.DeployVersion
		if err := db.Where("id = ? AND deploy_id = ?", vid, d.ID).First(&v).Error; err != nil {
			resp.NotFound(c, "版本不存在")
			return
		}
		resp.OK(c, v)
	}
}

// getEnvHandler 只读地解密 Service.EnvVars（M2 迁移前遗留字段），用于仪表盘历史查看。
// secret 字段一律以 "***" 屏蔽；编辑需走新的 EnvVarSet 接口。
func getEnvHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		d, ok := findDeploy(c, db)
		if !ok {
			return
		}
		if d.EnvVars == "" {
			resp.OK(c, []envVar{})
			return
		}
		decrypted, err := crypto.Decrypt(d.EnvVars, cfg.Security.AESKey)
		if err != nil {
			resp.InternalError(c, "解密失败")
			return
		}
		var vars []envVar
		if err := json.Unmarshal([]byte(decrypted), &vars); err != nil {
			resp.InternalError(c, "环境变量数据损坏")
			return
		}
		for i := range vars {
			if vars[i].Secret {
				vars[i].Value = "***"
			}
		}
		resp.OK(c, vars)
	}
}

// ── helpers ────────────────────────────────────────────────────────────────

type envVar struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Secret bool   `json:"secret"`
}

func findDeploy(c *gin.Context, db *gorm.DB) (model.Service, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.BadRequest(c, "ID 格式错误")
		return model.Service{}, false
	}
	var d model.Service
	if err := db.First(&d, id).Error; err != nil {
		resp.NotFound(c, "应用不存在")
		return model.Service{}, false
	}
	return d, true
}

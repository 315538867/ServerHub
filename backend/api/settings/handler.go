package settings

import (
	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/repo"
)

func RegisterRoutes(r *gin.RouterGroup, db repo.DB, _ *config.Config) {
	r.GET("", getSettings(db))
	r.PUT("", putSettings(db))
}

func getSettings(db repo.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		settings, err := repo.ListAllSettings(c.Request.Context(), db)
		if err != nil {
			resp.InternalError(c, "读取设置失败")
			return
		}
		result := make(map[string]string, len(settings))
		for _, s := range settings {
			result[s.Key] = s.Value
		}
		resp.OK(c, result)
	}
}

func putSettings(db repo.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body map[string]string
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, "请求体格式错误")
			return
		}
		if err := repo.UpsertSettingsBulk(c.Request.Context(), db, body); err != nil {
			resp.InternalError(c, "保存失败")
			return
		}
		resp.OK(c, nil)
	}
}

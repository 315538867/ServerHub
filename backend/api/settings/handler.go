package settings

import (
	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/resp"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func RegisterRoutes(r *gin.RouterGroup, db *gorm.DB, _ *config.Config) {
	r.GET("", getSettings(db))
	r.PUT("", putSettings(db))
}

func getSettings(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var settings []model.Setting
		db.Find(&settings)
		result := make(map[string]string, len(settings))
		for _, s := range settings {
			result[s.Key] = s.Value
		}
		resp.OK(c, result)
	}
}

func putSettings(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body map[string]string
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, "请求体格式错误")
			return
		}
		// Atomic upsert — without a transaction a mid-flight failure would
		// leave settings in a half-applied state.
		err := db.Transaction(func(tx *gorm.DB) error {
			for k, v := range body {
				s := model.Setting{Key: k, Value: v}
				if err := tx.Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "key"}},
					DoUpdates: clause.AssignmentColumns([]string{"value"}),
				}).Create(&s).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			resp.InternalError(c, "保存失败")
			return
		}
		resp.OK(c, nil)
	}
}

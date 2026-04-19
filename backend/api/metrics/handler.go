package metrics

import (
	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/resp"
	"gorm.io/gorm"
)

func RegisterRoutes(r *gin.RouterGroup, db *gorm.DB) {
	r.GET("/overview", overviewHandler(db))
}

type serverOverview struct {
	ID          uint          `json:"id"`
	Name        string        `json:"name"`
	Host        string        `json:"host"`
	Port        int           `json:"port"`
	Status      string        `json:"status"`
	LastCheckAt interface{}   `json:"last_check_at"`
	Metric      *model.Metric `json:"metric"`
}

func overviewHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var servers []model.Server
		db.Order("id asc").Find(&servers)

		result := make([]serverOverview, len(servers))
		for i, s := range servers {
			var m model.Metric
			var mp *model.Metric
			if err := db.Where("server_id = ?", s.ID).Order("created_at desc").First(&m).Error; err == nil {
				mp = &m
			}
			result[i] = serverOverview{
				ID:          s.ID,
				Name:        s.Name,
				Host:        s.Host,
				Port:        s.Port,
				Status:      s.Status,
				LastCheckAt: s.LastCheckAt,
				Metric:      mp,
			}
		}
		resp.OK(c, result)
	}
}

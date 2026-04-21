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

		// Fetch the latest metric per server in one query (was N+1 before:
		// one SELECT per server on large fleets).
		var latest []model.Metric
		db.Where("id IN (?)",
			db.Model(&model.Metric{}).
				Select("MAX(id)").
				Group("server_id"),
		).Find(&latest)
		byServer := make(map[uint]*model.Metric, len(latest))
		for i := range latest {
			byServer[latest[i].ServerID] = &latest[i]
		}

		result := make([]serverOverview, len(servers))
		for i, s := range servers {
			result[i] = serverOverview{
				ID:          s.ID,
				Name:        s.Name,
				Host:        s.Host,
				Port:        s.Port,
				Status:      s.Status,
				LastCheckAt: s.LastCheckAt,
				Metric:      byServer[s.ID],
			}
		}
		resp.OK(c, result)
	}
}

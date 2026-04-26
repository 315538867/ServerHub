package metrics

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/derive"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/resp"
	"gorm.io/gorm"
)

func RegisterRoutes(r *gin.RouterGroup, db *gorm.DB) {
	r.GET("/overview", overviewHandler(db))
}

// serverOverview 是 /api/metrics/overview 的列表项。
//
// R3 起 Status / LastCheckAt 不再来自 model.Server(列已下线),改由 derive.ServerStatus
// 从 server_probes 时序表派生。JSON 字段名保持向前兼容。
type serverOverview struct {
	ID          uint          `json:"id"`
	Name        string        `json:"name"`
	Host        string        `json:"host"`
	Port        int           `json:"port"`
	Status      string        `json:"status"`
	LastCheckAt *time.Time    `json:"last_check_at"`
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

		ids := make([]uint, len(servers))
		for i, s := range servers {
			ids[i] = s.ID
		}
		statusMap := derive.ServerStatus(db, ids, 0, 0)

		result := make([]serverOverview, len(servers))
		for i, s := range servers {
			st := statusMap[s.ID]
			ov := serverOverview{
				ID:     s.ID,
				Name:   s.Name,
				Host:   s.Host,
				Port:   s.Port,
				Status: st.Result,
				Metric: byServer[s.ID],
			}
			if !st.LastProbeAt.IsZero() {
				t := st.LastProbeAt
				ov.LastCheckAt = &t
			}
			result[i] = ov
		}
		resp.OK(c, result)
	}
}

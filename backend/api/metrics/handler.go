package metrics

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/derive"
	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/repo"
)

func RegisterRoutes(r *gin.RouterGroup, db repo.DB) {
	r.GET("/overview", overviewHandler(db))
}

// serverOverview 是 /api/metrics/overview 的列表项。
type serverOverview struct {
	ID          uint          `json:"id"`
	Name        string        `json:"name"`
	Host        string        `json:"host"`
	Port        int           `json:"port"`
	Status      string        `json:"status"`
	LastCheckAt *time.Time    `json:"last_check_at"`
	Metric      *domain.Metric `json:"metric"`
}

func overviewHandler(db repo.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		servers, err := repo.ListAllServers(ctx, db)
		if err != nil {
			resp.InternalError(c, err.Error())
			return
		}

		latest, err := repo.ListLatestMetricPerServer(ctx, db)
		if err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		byServer := make(map[uint]*domain.Metric, len(latest))
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

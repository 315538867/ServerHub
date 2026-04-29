package health

import (
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/repo"
)

var (
	startTime = time.Now()
	// Version can be set at process startup (typically from main.Version)
	// to surface the build tag in the wrapped /api/v1/health payload.
	Version = "dev"
)

type healthData struct {
	Version  string `json:"version"`
	Uptime   int64  `json:"uptime"`
	DBStatus string `json:"db_status"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
}

func Handler(cfg *config.Config, db repo.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		dbStatus := "ok"
		sqlDB, err := db.DB()
		if err != nil || sqlDB.Ping() != nil {
			dbStatus = "error"
		}

		resp.OK(c, healthData{
			Version:  Version,
			Uptime:   int64(time.Since(startTime).Seconds()),
			DBStatus: dbStatus,
			OS:       runtime.GOOS,
			Arch:     runtime.GOARCH,
		})
	}
}

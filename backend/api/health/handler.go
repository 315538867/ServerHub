package health

import (
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/pkg/resp"
	"gorm.io/gorm"
)

var startTime = time.Now()

type healthData struct {
	Version  string `json:"version"`
	Uptime   int64  `json:"uptime"`
	DBStatus string `json:"db_status"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
}

func Handler(cfg *config.Config, db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		dbStatus := "ok"
		sqlDB, err := db.DB()
		if err != nil || sqlDB.Ping() != nil {
			dbStatus = "error"
		}

		resp.OK(c, healthData{
			Version:  "dev",
			Uptime:   int64(time.Since(startTime).Seconds()),
			DBStatus: dbStatus,
			OS:       runtime.GOOS,
			Arch:     runtime.GOARCH,
		})
	}
}

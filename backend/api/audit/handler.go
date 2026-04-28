package audit

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/repo"
	"gorm.io/gorm"
)

func RegisterRoutes(r *gin.RouterGroup, db *gorm.DB) {
	r.GET("/logs", logsHandler(db))
}

func logsHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		size, _ := strconv.Atoi(c.DefaultQuery("size", "50"))
		if page < 1 {
			page = 1
		}
		if size < 1 || size > 200 {
			size = 50
		}

		logs, total, err := repo.ListAuditLogsFiltered(
			c.Request.Context(), db,
			c.Query("username"), c.Query("path"), c.Query("status"),
			(page-1)*size, size,
		)
		if err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, gin.H{"total": total, "logs": logs})
	}
}

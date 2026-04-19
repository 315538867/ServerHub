package audit

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/resp"
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

		q := db.Model(&model.AuditLog{})
		if u := c.Query("username"); u != "" {
			q = q.Where("username LIKE ?", "%"+u+"%")
		}
		if p := c.Query("path"); p != "" {
			q = q.Where("path LIKE ?", "%"+p+"%")
		}
		if s := c.Query("status"); s != "" {
			q = q.Where("status = ?", s)
		}

		var total int64
		q.Count(&total)

		var logs []model.AuditLog
		q.Order("created_at desc").Offset((page - 1) * size).Limit(size).Find(&logs)

		resp.OK(c, gin.H{"total": total, "logs": logs})
	}
}

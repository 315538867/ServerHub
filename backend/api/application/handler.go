package application

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/resp"
	"gorm.io/gorm"
)

func RegisterRoutes(r *gin.RouterGroup, db *gorm.DB, _ *config.Config) {
	r.GET("", listHandler(db))
	r.POST("", createHandler(db))
	r.GET("/:id", getHandler(db))
	r.PUT("/:id", updateHandler(db))
	r.DELETE("/:id", deleteHandler(db))
}

type appReq struct {
	Name          string `json:"name" binding:"required"`
	Description   string `json:"description"`
	ServerID      uint   `json:"server_id" binding:"required"`
	SiteName      string `json:"site_name"`
	Domain        string `json:"domain"`
	ContainerName string `json:"container_name"`
	DeployID      *uint  `json:"deploy_id"`
	DBConnID      *uint  `json:"db_conn_id"`
}

func listHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var apps []model.Application
		q := db.Order("id asc")
		if sid := c.Query("server_id"); sid != "" {
			q = q.Where("server_id = ?", sid)
		}
		q.Find(&apps)
		resp.OK(c, apps)
	}
}

func createHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req appReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		var server model.Server
		if err := db.First(&server, req.ServerID).Error; err != nil {
			resp.BadRequest(c, "服务器不存在")
			return
		}
		app := model.Application{
			Name:          req.Name,
			Description:   req.Description,
			ServerID:      req.ServerID,
			SiteName:      req.SiteName,
			Domain:        req.Domain,
			ContainerName: req.ContainerName,
			DeployID:      req.DeployID,
			DBConnID:      req.DBConnID,
			Status:        "unknown",
		}
		if err := db.Create(&app).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, app)
	}
}

func getHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			resp.BadRequest(c, "无效 ID")
			return
		}
		var app model.Application
		if err := db.First(&app, id).Error; err != nil {
			resp.NotFound(c, "应用不存在")
			return
		}
		resp.OK(c, app)
	}
}

func updateHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			resp.BadRequest(c, "无效 ID")
			return
		}
		var app model.Application
		if err := db.First(&app, id).Error; err != nil {
			resp.NotFound(c, "应用不存在")
			return
		}
		var req appReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		app.Name = req.Name
		app.Description = req.Description
		app.ServerID = req.ServerID
		app.SiteName = req.SiteName
		app.Domain = req.Domain
		app.ContainerName = req.ContainerName
		app.DeployID = req.DeployID
		app.DBConnID = req.DBConnID
		if err := db.Save(&app).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, app)
	}
}

func deleteHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			resp.BadRequest(c, "无效 ID")
			return
		}
		if err := db.Delete(&model.Application{}, id).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, nil)
	}
}

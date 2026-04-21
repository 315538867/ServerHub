package servers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/pkg/sshpool"
	"gorm.io/gorm"
)

func RegisterRoutes(r *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	r.GET("", listHandler(db))
	r.POST("", createHandler(db, cfg))
	r.GET("/:id", getHandler(db))
	r.PUT("/:id", updateHandler(db, cfg))
	r.DELETE("/:id", deleteHandler(db))
	r.POST("/:id/test", testHandler(db, cfg))
	r.POST("/:id/metrics/collect", collectMetricsHandler(db, cfg))
	r.GET("/:id/metrics", listMetricsHandler(db))
}

type serverResp struct {
	ID          uint       `json:"id"`
	Name        string     `json:"name"`
	Type        string     `json:"type"`
	Host        string     `json:"host"`
	Port        int        `json:"port"`
	Username    string     `json:"username"`
	AuthType    string     `json:"auth_type"`
	Remark      string     `json:"remark"`
	Status      string     `json:"status"`
	LastCheckAt *time.Time `json:"last_check_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func toResp(s model.Server) serverResp {
	return serverResp{
		ID: s.ID, Name: s.Name, Type: s.Type, Host: s.Host, Port: s.Port,
		Username: s.Username, AuthType: s.AuthType, Remark: s.Remark,
		Status: s.Status, LastCheckAt: s.LastCheckAt,
		CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

type createReq struct {
	Name       string `json:"name" binding:"required"`
	Host       string `json:"host" binding:"required"`
	Port       int    `json:"port"`
	Username   string `json:"username" binding:"required"`
	AuthType   string `json:"auth_type"`
	Password   string `json:"password"`
	PrivateKey string `json:"private_key"`
	Remark     string `json:"remark"`
}

func listHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var servers []model.Server
		db.Order("id asc").Find(&servers)
		out := make([]serverResp, len(servers))
		for i, s := range servers {
			out[i] = toResp(s)
		}
		resp.OK(c, out)
	}
}

func createHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		if req.Port == 0 {
			req.Port = 22
		}
		if req.AuthType == "" {
			req.AuthType = "password"
		}

		encPwd, encKey, err := encryptCreds(req.Password, req.PrivateKey, cfg.Security.AESKey)
		if err != nil {
			resp.InternalError(c, "加密凭据失败")
			return
		}

		s := model.Server{
			Name: req.Name, Host: req.Host, Port: req.Port,
			Username: req.Username, AuthType: req.AuthType,
			Password: encPwd, PrivateKey: encKey,
			Remark: req.Remark, Status: "unknown",
		}
		if err := db.Create(&s).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		c.JSON(http.StatusCreated, gin.H{"code": 0, "msg": "ok", "data": toResp(s)})
	}
}

func getHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, ok := findServer(c, db)
		if !ok {
			return
		}
		resp.OK(c, toResp(s))
	}
}

func updateHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, ok := findServer(c, db)
		if !ok {
			return
		}
		if s.Type == "local" {
			resp.Fail(c, http.StatusForbidden, 4030, "本机服务器不可编辑")
			return
		}

		var req createReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}

		encPwd, encKey, err := encryptCreds(req.Password, req.PrivateKey, cfg.Security.AESKey)
		if err != nil {
			resp.InternalError(c, "加密凭据失败")
			return
		}

		updates := map[string]any{
			"name": req.Name, "host": req.Host, "username": req.Username,
			"auth_type": req.AuthType, "remark": req.Remark, "status": "unknown",
		}
		if req.Port > 0 {
			updates["port"] = req.Port
		}
		if encPwd != "" {
			updates["password"] = encPwd
		}
		if encKey != "" {
			updates["private_key"] = encKey
		}

		sshpool.Remove(s.ID)
		if err := db.Model(&s).Updates(updates).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, toResp(s))
	}
}

func deleteHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, ok := findServer(c, db)
		if !ok {
			return
		}
		if s.Type == "local" {
			resp.Fail(c, http.StatusForbidden, 4030, "本机服务器不可删除")
			return
		}
		sshpool.Remove(s.ID)
		db.Delete(&s)
		sid := s.ID
		db.Where("server_id = ?", sid).Delete(&model.Metric{})
		db.Where("server_id = ?", sid).Delete(&model.Deploy{})
		db.Where("server_id = ?", sid).Delete(&model.DBConn{})
		db.Where("server_id = ?", sid).Delete(&model.AlertRule{})
		db.Where("server_id = ?", sid).Delete(&model.AlertEvent{})
		db.Where("server_id = ?", sid).Delete(&model.SSLCert{})
		resp.OK(c, nil)
	}
}

func testHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, ok := findServer(c, db)
		if !ok {
			return
		}
		now := time.Now()
		if s.Type == "local" {
			db.Model(&s).Updates(map[string]any{"status": "online", "last_check_at": now})
			resp.OK(c, gin.H{"status": "online"})
			return
		}

		cred, err := getDecryptedCred(s, cfg.Security.AESKey)
		if err != nil {
			resp.InternalError(c, "解密凭据失败")
			return
		}

		sshpool.Remove(s.ID)
		client, err := sshpool.Connect(s.ID, s.Host, s.Port, s.Username, s.AuthType, cred)
		status := "online"
		if err != nil {
			status = "offline"
			db.Model(&s).Updates(map[string]any{"status": status, "last_check_at": now})
			resp.OK(c, gin.H{"status": status, "error": err.Error()})
			return
		}
		_ = client
		db.Model(&s).Updates(map[string]any{"status": status, "last_check_at": now})
		resp.OK(c, gin.H{"status": status})
	}
}

func collectMetricsHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, ok := findServer(c, db)
		if !ok {
			return
		}

		var metrics *sshpool.MetricsResult
		var err error
		if s.Type == "local" {
			metrics, err = sshpool.CollectLocalMetrics()
		} else {
			cred, derr := getDecryptedCred(s, cfg.Security.AESKey)
			if derr != nil {
				resp.InternalError(c, "解密凭据失败")
				return
			}
			client, derr := sshpool.Connect(s.ID, s.Host, s.Port, s.Username, s.AuthType, cred)
			if derr != nil {
				resp.Fail(c, http.StatusServiceUnavailable, 503, "连接失败: "+derr.Error())
				return
			}
			metrics, err = sshpool.CollectMetrics(client)
		}
		if err != nil {
			resp.InternalError(c, "采集指标失败: "+err.Error())
			return
		}

		m := model.Metric{
			ServerID: s.ID,
			CPU:      metrics.CPU, Mem: metrics.Mem, Disk: metrics.Disk,
			Load1: metrics.Load1, Uptime: metrics.Uptime,
		}
		db.Create(&m)
		resp.OK(c, m)
	}
}

func listMetricsHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, ok := findServer(c, db)
		if !ok {
			return
		}

		limit := 60
		if l := c.Query("limit"); l != "" {
			if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 1440 {
				limit = v
			}
		}

		var metrics []model.Metric
		db.Where("server_id = ?", s.ID).Order("created_at desc").Limit(limit).Find(&metrics)
		resp.OK(c, metrics)
	}
}

// ── helpers ────────────────────────────────────────────────────────────────

func findServer(c *gin.Context, db *gorm.DB) (model.Server, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.BadRequest(c, "ID 格式错误")
		return model.Server{}, false
	}
	var s model.Server
	if err := db.First(&s, id).Error; err != nil {
		resp.NotFound(c, "服务器不存在")
		return model.Server{}, false
	}
	return s, true
}

func encryptCreds(password, privateKey, aesKey string) (encPwd, encKey string, err error) {
	if password != "" {
		encPwd, err = crypto.Encrypt(password, aesKey)
		if err != nil {
			return
		}
	}
	if privateKey != "" {
		encKey, err = crypto.Encrypt(privateKey, aesKey)
	}
	return
}

func getDecryptedCred(s model.Server, aesKey string) (string, error) {
	switch s.AuthType {
	case "key":
		if s.PrivateKey == "" {
			return "", nil
		}
		return crypto.Decrypt(s.PrivateKey, aesKey)
	default:
		if s.Password == "" {
			return "", nil
		}
		return crypto.Decrypt(s.Password, aesKey)
	}
}

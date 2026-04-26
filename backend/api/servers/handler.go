package servers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/derive"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/pkg/sshpool"
	"github.com/serverhub/serverhub/pkg/svcstatus"
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
	r.GET("/:id/services", listServicesHandler(db))
	r.GET("/:id/networks", listNetworksHandler(db))
	r.PUT("/:id/networks", updateNetworksHandler(db))
}

// serverResp 是 GET /servers 的响应 DTO。
//
// R3 起 Status / LastCheckAt 不再来自 model.Server(列已下线),改由 derive.ServerStatus
// 从 server_probes 时序表派生:
//   - Status      ← derive.ServerStatusEntry.Result(online|lagging|offline|unknown)
//   - LastCheckAt ← derive.ServerStatusEntry.LastProbeAt (zero → null)
//
// 兼容性:JSON 字段名保留,前端 TS 类型不需要改;只是来源改了。
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

func toResp(s model.Server, st derive.ServerStatusEntry) serverResp {
	r := serverResp{
		ID: s.ID, Name: s.Name, Type: s.Type, Host: s.Host, Port: s.Port,
		Username: s.Username, AuthType: s.AuthType, Remark: s.Remark,
		Status:    st.Result,
		CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
	if !st.LastProbeAt.IsZero() {
		t := st.LastProbeAt
		r.LastCheckAt = &t
	}
	return r
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
		ids := make([]uint, len(servers))
		for i, s := range servers {
			ids[i] = s.ID
		}
		statusMap := derive.ServerStatus(db, ids, 0, 0)
		out := make([]serverResp, len(servers))
		for i, s := range servers {
			out[i] = toResp(s, statusMap[s.ID])
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
		switch req.Host {
		case "127.0.0.1", "localhost", "::1", "0.0.0.0":
			resp.Fail(c, http.StatusForbidden, 4030, "请使用已自动创建的本机记录，不可重复添加 localhost")
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
			Remark: req.Remark,
		}
		if err := db.Create(&s).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		// 刚创建尚无 probe → unknown,无需查 derive
		c.JSON(http.StatusCreated, gin.H{"code": 0, "msg": "ok", "data": toResp(s, derive.ServerStatusEntry{Result: derive.ServerStatusUnknown})})
	}
}

func getHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, ok := findServer(c, db)
		if !ok {
			return
		}
		statusMap := derive.ServerStatus(db, []uint{s.ID}, 0, 0)
		resp.OK(c, toResp(s, statusMap[s.ID]))
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
			"auth_type": req.AuthType, "remark": req.Remark,
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
		statusMap := derive.ServerStatus(db, []uint{s.ID}, 0, 0)
		resp.OK(c, toResp(s, statusMap[s.ID]))
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
		db.Where("server_id = ?", sid).Delete(&model.Service{})
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
		if s.Type == "local" {
			db.Create(&model.ServerProbe{ServerID: s.ID, Result: "online", CreatedAt: time.Now()})
			resp.OK(c, gin.H{"status": "online"})
			return
		}

		cred, err := getDecryptedCred(s, cfg.Security.AESKey)
		if err != nil {
			resp.InternalError(c, "解密凭据失败")
			return
		}

		sshpool.Remove(s.ID)
		start := time.Now()
		client, err := sshpool.Connect(s.ID, s.Host, s.Port, s.Username, s.AuthType, cred)
		latencyMs := int(time.Since(start).Milliseconds())
		if err != nil {
			db.Create(&model.ServerProbe{
				ServerID: s.ID, Result: "offline",
				LatencyMs: latencyMs, ErrMsg: err.Error(), CreatedAt: time.Now(),
			})
			resp.OK(c, gin.H{"status": "offline", "error": err.Error()})
			return
		}
		_ = client
		db.Create(&model.ServerProbe{
			ServerID: s.ID, Result: "online",
			LatencyMs: latencyMs, CreatedAt: time.Now(),
		})
		resp.OK(c, gin.H{"status": "online"})
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

// listServicesHandler 返回该服务器上所有 Service（供 NginxRoutes 下拉用）。
// 同时附带 Application 名（nullable）便于前端展示归属关系。
func listServicesHandler(db *gorm.DB) gin.HandlerFunc {
	type svcItem struct {
		ID              uint   `json:"id"`
		Name            string `json:"name"`
		Type            string `json:"type"`
		ApplicationID   *uint  `json:"application_id"`
		ApplicationName string `json:"application_name,omitempty"`
		ExposedPort     int    `json:"exposed_port"`
		ImageName       string `json:"image_name,omitempty"`
		WorkDir         string `json:"work_dir,omitempty"`
		LastStatus      string `json:"last_status,omitempty"`
	}
	return func(c *gin.Context) {
		s, ok := findServer(c, db)
		if !ok {
			return
		}
		var svcs []model.Service
		db.Where("server_id = ?", s.ID).Order("id asc").Find(&svcs)

		// 一次性取出 Application 名称
		appIDs := make([]uint, 0, len(svcs))
		svcIDs := make([]uint, 0, len(svcs))
		for _, sv := range svcs {
			if sv.ApplicationID != nil {
				appIDs = append(appIDs, *sv.ApplicationID)
			}
			svcIDs = append(svcIDs, sv.ID)
		}
		nameByApp := make(map[uint]string, len(appIDs))
		if len(appIDs) > 0 {
			var apps []model.Application
			db.Where("id IN ?", appIDs).Find(&apps)
			for _, a := range apps {
				nameByApp[a.ID] = a.Name
			}
		}

		// 派生区:LastStatus 取最近一条 DeployRun.Status,无 run 视作 takeover 接管,
		// 默认 "success" 与 P-G 之前的写入语义对齐,避免前端从有值变 "—" 的 UX 退步。
		// ImageName 从 P-I 起也走派生:Service.CurrentReleaseID → Release.StartSpec.image,
		// 真值跟 buildStartPart 实际启动用的镜像对齐(而非历史 takeover 一次性快照)。
		//
		// 注意:Entry 现在多字段聚合,某 Service 可能只有 Image(还没部署过 → 没 DeployRun)
		// 或只有 Status(StartSpec 不是 docker 类型 → 没 image)。所以两个字段各自判断,
		// 不能用 ok-check 做"是否有 entry"的统一开关。
		latest := svcstatus.LatestByService(db, svcIDs)

		out := make([]svcItem, len(svcs))
		for i, sv := range svcs {
			e := latest[sv.ID]
			status := e.Status
			if status == "" {
				status = "success"
			}
			it := svcItem{
				ID: sv.ID, Name: sv.Name, Type: sv.Type,
				ApplicationID: sv.ApplicationID,
				ExposedPort:   sv.ExposedPort,
				ImageName:     e.Image,
				WorkDir:       sv.WorkDir,
				LastStatus:    status,
			}
			if sv.ApplicationID != nil {
				it.ApplicationName = nameByApp[*sv.ApplicationID]
			}
			out[i] = it
		}
		resp.OK(c, out)
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

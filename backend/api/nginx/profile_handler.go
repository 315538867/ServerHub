package nginx

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/serverhub/serverhub/adapters/ingress/nginx/profile"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/pkg/runner"
)

// ProfileResp 是返回给前端的 NginxProfile 视图。modules 解析成数组方便前端渲染。
// effective_* 字段把 Default + 用户覆盖合并后的实际生效值算出来，前端无需重复
// profile.NormalizeProfile 的逻辑。
type ProfileResp struct {
	EdgeServerID uint `json:"edge_server_id"`

	// 用户覆盖（DB 原始字段，空 = 用 default）
	NginxConfDir      string `json:"nginx_conf_dir"`
	SitesAvailableDir string `json:"sites_available_dir"`
	SitesEnabledDir   string `json:"sites_enabled_dir"`
	AppLocationsDir   string `json:"app_locations_dir"`
	StreamsConf       string `json:"streams_conf"`
	CertDir           string `json:"cert_dir"`
	NginxConfPath     string `json:"nginx_conf_path"`
	HubSiteName       string `json:"hub_site_name"`
	TestCmd           string `json:"test_cmd"`
	ReloadCmd         string `json:"reload_cmd"`

	// 合并后的有效值，给前端 placeholder / "实际生效" 标签用。
	Effective profile.Profile `json:"effective"`

	// probe 缓存（只读）
	BinaryPath  string     `json:"binary_path,omitempty"`
	Version     string     `json:"version,omitempty"`
	BuildPrefix string     `json:"build_prefix,omitempty"`
	BuildConf   string     `json:"build_conf,omitempty"`
	Modules     []string   `json:"modules,omitempty"`
	LastProbeAt *time.Time `json:"last_probe_at,omitempty"`
}

// RegisterProfileRoutes 注册 /servers/:id/nginx/profile 系列路由。
// 单独函数让 handler.go 的 RegisterRoutes 在底部一行追加即可，避免改动主流程。
func RegisterProfileRoutes(r *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	r.GET("/:id/nginx/profile", getProfileHandler(db))
	r.PUT("/:id/nginx/profile", putProfileHandler(db))
	r.POST("/:id/nginx/profile/probe", probeProfileHandler(db, cfg))
}

func parseEdgeID(c *gin.Context) (uint, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		resp.BadRequest(c, "服务器 ID 无效")
		return 0, false
	}
	return uint(id), true
}

// loadProfileRow 取出该 edge 的 NginxProfile 行；不存在时返回零值结构（不报错），
// 让 GET 在没有显式记录时也能返回 default placeholder。
func loadProfileRow(db *gorm.DB, edgeID uint) (model.NginxProfile, bool, error) {
	var np model.NginxProfile
	err := db.Where("edge_server_id = ?", edgeID).First(&np).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.NginxProfile{EdgeServerID: edgeID}, false, nil
	}
	if err != nil {
		return model.NginxProfile{}, false, err
	}
	return np, true, nil
}

func makeProfileResp(np model.NginxProfile, exists bool) ProfileResp {
	eff := profile.NormalizeProfile(profile.Profile{
		NginxConfDir:      np.NginxConfDir,
		SitesAvailableDir: np.SitesAvailableDir,
		SitesEnabledDir:   np.SitesEnabledDir,
		AppLocationsDir:   np.AppLocationsDir,
		StreamsConf:       np.StreamsConf,
		CertDir:           np.CertDir,
		NginxConfPath:     np.NginxConfPath,
		HubSiteName:       np.HubSiteName,
		TestCmd:           np.TestCmd,
		ReloadCmd:         np.ReloadCmd,
	})
	out := ProfileResp{
		EdgeServerID:      np.EdgeServerID,
		NginxConfDir:      np.NginxConfDir,
		SitesAvailableDir: np.SitesAvailableDir,
		SitesEnabledDir:   np.SitesEnabledDir,
		AppLocationsDir:   np.AppLocationsDir,
		StreamsConf:       np.StreamsConf,
		CertDir:           np.CertDir,
		NginxConfPath:     np.NginxConfPath,
		HubSiteName:       np.HubSiteName,
		TestCmd:           np.TestCmd,
		ReloadCmd:         np.ReloadCmd,
		Effective:         eff,
	}
	if exists {
		out.BinaryPath = np.BinaryPath
		out.Version = np.Version
		out.BuildPrefix = np.BuildPrefix
		out.BuildConf = np.BuildConf
		out.LastProbeAt = np.LastProbeAt
		if np.Modules != "" {
			var mods []string
			if err := json.Unmarshal([]byte(np.Modules), &mods); err == nil {
				out.Modules = mods
			}
		}
	}
	return out
}

func getProfileHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		edgeID, ok := parseEdgeID(c)
		if !ok {
			return
		}
		var s model.Server
		if err := db.First(&s, edgeID).Error; err != nil {
			resp.NotFound(c, "服务器不存在")
			return
		}
		np, exists, err := loadProfileRow(db, edgeID)
		if err != nil {
			resp.InternalError(c, "加载 nginx profile 失败: "+err.Error())
			return
		}
		resp.OK(c, makeProfileResp(np, exists))
	}
}

// ProfileUpdateBody 仅允许写"用户覆盖路径/命令"这十项；probe 字段由 probe 接口
// 写入，不能从这里改。空字符串等价于"用 default 兜底"。
type ProfileUpdateBody struct {
	NginxConfDir      string `json:"nginx_conf_dir"`
	SitesAvailableDir string `json:"sites_available_dir"`
	SitesEnabledDir   string `json:"sites_enabled_dir"`
	AppLocationsDir   string `json:"app_locations_dir"`
	StreamsConf       string `json:"streams_conf"`
	CertDir           string `json:"cert_dir"`
	NginxConfPath     string `json:"nginx_conf_path"`
	HubSiteName       string `json:"hub_site_name"`
	TestCmd           string `json:"test_cmd"`
	ReloadCmd         string `json:"reload_cmd"`
}

// validateProfileBody 拒绝相对路径与超长输入。命令字段允许任意 shell 串（用户
// 自行负责），但仍设上限避免恶意提交。
func validateProfileBody(b *ProfileUpdateBody) error {
	pathChecks := map[string]string{
		"nginx_conf_dir":      b.NginxConfDir,
		"sites_available_dir": b.SitesAvailableDir,
		"sites_enabled_dir":   b.SitesEnabledDir,
		"app_locations_dir":   b.AppLocationsDir,
		"streams_conf":        b.StreamsConf,
		"cert_dir":            b.CertDir,
		"nginx_conf_path":     b.NginxConfPath,
	}
	for name, v := range pathChecks {
		if v == "" {
			continue
		}
		if !strings.HasPrefix(v, "/") {
			return errors.New(name + " 必须是绝对路径")
		}
		if len(v) > 256 {
			return errors.New(name + " 长度超限 (>256)")
		}
		if strings.Contains(v, "\n") || strings.Contains(v, "\x00") {
			return errors.New(name + " 含非法字符")
		}
	}
	if b.HubSiteName != "" && (strings.ContainsAny(b.HubSiteName, " /\\\n") || len(b.HubSiteName) > 64) {
		return errors.New("hub_site_name 非法")
	}
	if len(b.TestCmd) > 1024 || len(b.ReloadCmd) > 1024 {
		return errors.New("test_cmd / reload_cmd 长度超限 (>1024)")
	}
	return nil
}

func putProfileHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		edgeID, ok := parseEdgeID(c)
		if !ok {
			return
		}
		var s model.Server
		if err := db.First(&s, edgeID).Error; err != nil {
			resp.NotFound(c, "服务器不存在")
			return
		}
		var body ProfileUpdateBody
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, "请求体格式错误: "+err.Error())
			return
		}
		if err := validateProfileBody(&body); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}

		var np model.NginxProfile
		err := db.Where("edge_server_id = ?", edgeID).First(&np).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			np = model.NginxProfile{EdgeServerID: edgeID}
		} else if err != nil {
			resp.InternalError(c, "查询 profile 失败: "+err.Error())
			return
		}

		np.NginxConfDir = body.NginxConfDir
		np.SitesAvailableDir = body.SitesAvailableDir
		np.SitesEnabledDir = body.SitesEnabledDir
		np.AppLocationsDir = body.AppLocationsDir
		np.StreamsConf = body.StreamsConf
		np.CertDir = body.CertDir
		np.NginxConfPath = body.NginxConfPath
		np.HubSiteName = body.HubSiteName
		np.TestCmd = body.TestCmd
		np.ReloadCmd = body.ReloadCmd

		if np.ID == 0 {
			if err := db.Create(&np).Error; err != nil {
				resp.InternalError(c, "保存 profile 失败: "+err.Error())
				return
			}
		} else {
			// 显式 Updates 防止把 probe 字段意外清空
			if err := db.Model(&np).Updates(map[string]any{
				"nginx_conf_dir":      np.NginxConfDir,
				"sites_available_dir": np.SitesAvailableDir,
				"sites_enabled_dir":   np.SitesEnabledDir,
				"app_locations_dir":   np.AppLocationsDir,
				"streams_conf":        np.StreamsConf,
				"cert_dir":            np.CertDir,
				"nginx_conf_path":     np.NginxConfPath,
				"hub_site_name":       np.HubSiteName,
				"test_cmd":            np.TestCmd,
				"reload_cmd":          np.ReloadCmd,
			}).Error; err != nil {
				resp.InternalError(c, "更新 profile 失败: "+err.Error())
				return
			}
		}

		// 重新读一遍返回最新值（含合并 effective）
		fresh, exists, err := loadProfileRow(db, edgeID)
		if err != nil {
			resp.InternalError(c, "回读 profile 失败: "+err.Error())
			return
		}
		resp.OK(c, makeProfileResp(fresh, exists))
	}
}

// probeProfileHandler 在远端跑 nginx -V，把解析结果回写到 NginxProfile.probe 字段。
// 不会改用户填的路径/命令字段。返回最新的 ProfileResp。
func probeProfileHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		edgeID, ok := parseEdgeID(c)
		if !ok {
			return
		}
		var s model.Server
		if err := db.First(&s, edgeID).Error; err != nil {
			resp.NotFound(c, "服务器不存在")
			return
		}
		rn, err := runner.For(&s, cfg)
		if err != nil {
			resp.Fail(c, http.StatusServiceUnavailable, 5003, "执行器获取失败: "+err.Error())
			return
		}

		pr, err := profile.ProbeNginxV(rn)
		if err != nil {
			resp.InternalError(c, "probe 失败: "+err.Error())
			return
		}

		modulesJSON, _ := json.Marshal(pr.Modules)
		now := time.Now()

		var np model.NginxProfile
		ferr := db.Where("edge_server_id = ?", edgeID).First(&np).Error
		if errors.Is(ferr, gorm.ErrRecordNotFound) {
			np = model.NginxProfile{
				EdgeServerID: edgeID,
				BinaryPath:   pr.BinaryPath,
				NginxVRaw:    pr.Raw,
				Version:      pr.Version,
				BuildPrefix:  pr.BuildPrefix,
				BuildConf:    pr.BuildConf,
				Modules:      string(modulesJSON),
				LastProbeAt:  &now,
			}
			if err := db.Create(&np).Error; err != nil {
				resp.InternalError(c, "保存 probe 失败: "+err.Error())
				return
			}
		} else if ferr != nil {
			resp.InternalError(c, "查询 profile 失败: "+ferr.Error())
			return
		} else {
			if err := db.Model(&np).Updates(map[string]any{
				"binary_path":   pr.BinaryPath,
				"nginx_v_raw":   pr.Raw,
				"version":       pr.Version,
				"build_prefix":  pr.BuildPrefix,
				"build_conf":    pr.BuildConf,
				"modules":       string(modulesJSON),
				"last_probe_at": &now,
			}).Error; err != nil {
				resp.InternalError(c, "更新 probe 失败: "+err.Error())
				return
			}
		}

		fresh, exists, err := loadProfileRow(db, edgeID)
		if err != nil {
			resp.InternalError(c, "回读 profile 失败: "+err.Error())
			return
		}
		resp.OK(c, makeProfileResp(fresh, exists))
	}
}

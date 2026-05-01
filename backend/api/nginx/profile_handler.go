package nginx

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/repo"

	"github.com/serverhub/serverhub/adapters/ingress/nginx/profile"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/usecase"
)

// ProfileResp 是返回给前端的 NginxProfile 视图。
type ProfileResp struct {
	EdgeServerID uint `json:"edge_server_id"`

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

	Effective profile.Profile `json:"effective"`

	BinaryPath  string     `json:"binary_path,omitempty"`
	Version     string     `json:"version,omitempty"`
	BuildPrefix string     `json:"build_prefix,omitempty"`
	BuildConf   string     `json:"build_conf,omitempty"`
	Modules     []string   `json:"modules,omitempty"`
	LastProbeAt *time.Time `json:"last_probe_at,omitempty"`
}

// RegisterProfileRoutes 注册 /servers/:id/nginx/profile 系列路由。
func RegisterProfileRoutes(r *gin.RouterGroup, db repo.DB, cfg *config.Config) {
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

func makeProfileResp(pr usecase.NginxProfileResult) ProfileResp {
	np := pr.Profile
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
	if pr.Exists {
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

func getProfileHandler(db repo.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		edgeID, ok := parseEdgeID(c)
		if !ok {
			return
		}
		if _, err := usecase.GetServerByID(c.Request.Context(), db, edgeID); err != nil {
			resp.NotFound(c, "服务器不存在")
			return
		}
		pr, err := usecase.GetNginxProfile(c.Request.Context(), db, edgeID)
		if err != nil {
			resp.InternalError(c, "加载 nginx profile 失败: "+err.Error())
			return
		}
		resp.OK(c, makeProfileResp(pr))
	}
}

// ProfileUpdateBody 仅允许写"用户覆盖路径/命令"这十项。
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

func putProfileHandler(db repo.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		edgeID, ok := parseEdgeID(c)
		if !ok {
			return
		}
		if _, err := usecase.GetServerByID(c.Request.Context(), db, edgeID); err != nil {
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

		if err := usecase.UpdateNginxProfile(c.Request.Context(), db, edgeID, usecase.UpdateNginxProfileInput{
			NginxConfDir:      body.NginxConfDir,
			SitesAvailableDir: body.SitesAvailableDir,
			SitesEnabledDir:   body.SitesEnabledDir,
			AppLocationsDir:   body.AppLocationsDir,
			StreamsConf:       body.StreamsConf,
			CertDir:           body.CertDir,
			NginxConfPath:     body.NginxConfPath,
			HubSiteName:       body.HubSiteName,
			TestCmd:           body.TestCmd,
			ReloadCmd:         body.ReloadCmd,
		}); err != nil {
			resp.InternalError(c, "保存 profile 失败: "+err.Error())
			return
		}

		pr, err := usecase.GetNginxProfile(c.Request.Context(), db, edgeID)
		if err != nil {
			resp.InternalError(c, "回读 profile 失败: "+err.Error())
			return
		}
		resp.OK(c, makeProfileResp(pr))
	}
}

func probeProfileHandler(db repo.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		edgeID, ok := parseEdgeID(c)
		if !ok {
			return
		}
		if _, err := usecase.GetServerByID(c.Request.Context(), db, edgeID); err != nil {
			resp.NotFound(c, "服务器不存在")
			return
		}
		pr, err := usecase.ProbeNginxProfile(c.Request.Context(), db, cfg, edgeID)
		if err != nil {
			resp.Fail(c, http.StatusServiceUnavailable, 5003, "probe 失败: "+err.Error())
			return
		}
		resp.OK(c, makeProfileResp(pr))
	}
}

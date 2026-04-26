package nginxrender

// Profile 描述一台 edge 上 nginx 的具体部署形态。
//
// 历史上 Render / Reconciler / Inspect 全部用包级 const 拼路径，等价于硬编码
// "Debian + 单 nginx" 的世界观。P3 起把这些路径与命令串集中到 Profile，调用
// 链上层（reconciler）一次 LoadProfile 就能把多实例 / 自编译 / 容器化场景下
// 的差异透明吸收掉。
//
// 字段语义：每个字段都是绝对路径或完整 shell 命令。Profile 的零值不可用——
// 必须经 DefaultProfile() 或 NormalizeProfile() 走一遍，确保所有字段非空。
type Profile struct {
	NginxConfDir      string `json:"nginx_conf_dir"`      // /etc/nginx，Snapshot/Restore 的根
	SitesAvailableDir string `json:"sites_available_dir"`
	SitesEnabledDir   string `json:"sites_enabled_dir"`
	AppLocationsDir   string `json:"app_locations_dir"`
	StreamsConf       string `json:"streams_conf"`    // tcp/udp 聚合文件绝对路径
	CertDir           string `json:"cert_dir"`        // 证书 canonical 落盘根
	NginxConfPath     string `json:"nginx_conf_path"` // 顶层 nginx.conf 绝对路径（stream include 注入点）
	HubSiteName       string `json:"hub_site_name"`   // sites-available 下 path 模式聚合站点的 stem 名

	// 命令串：调用方拿到原样 r.Run(...)。带 sudo / 2>&1 是默认契约。
	TestCmd   string `json:"test_cmd"`
	ReloadCmd string `json:"reload_cmd"`
}

// DefaultProfile 返回单 nginx + Debian 风格的默认配置——与 P2 之前的硬编码
// 完全等价，保证未配置 NginxProfile 的 edge 行为不变。
func DefaultProfile() Profile {
	return Profile{
		NginxConfDir:      "/etc/nginx",
		SitesAvailableDir: "/etc/nginx/sites-available",
		SitesEnabledDir:   "/etc/nginx/sites-enabled",
		AppLocationsDir:   "/etc/nginx/app-locations",
		StreamsConf:       "/etc/nginx/streams.conf",
		CertDir:           "/etc/nginx/cert",
		NginxConfPath:     "/etc/nginx/nginx.conf",
		HubSiteName:       "serverhub-app-hub",
		TestCmd:           "sudo -n nginx -t 2>&1",
		ReloadCmd:         "sudo -n nginx -s reload 2>&1",
	}
}

// NormalizeProfile 用 DefaultProfile 的值填补 p 中的空字段，返回填后的 Profile。
// 调用方传 model.NginxProfile.ToRenderProfile() 拿到的（可能多数字段为空）的
// Profile，经此函数变成"凡是用户没改的字段都跟默认一致"的可用 Profile。
func NormalizeProfile(p Profile) Profile {
	d := DefaultProfile()
	if p.NginxConfDir == "" {
		p.NginxConfDir = d.NginxConfDir
	}
	if p.SitesAvailableDir == "" {
		p.SitesAvailableDir = d.SitesAvailableDir
	}
	if p.SitesEnabledDir == "" {
		p.SitesEnabledDir = d.SitesEnabledDir
	}
	if p.AppLocationsDir == "" {
		p.AppLocationsDir = d.AppLocationsDir
	}
	if p.StreamsConf == "" {
		p.StreamsConf = d.StreamsConf
	}
	if p.CertDir == "" {
		p.CertDir = d.CertDir
	}
	if p.NginxConfPath == "" {
		p.NginxConfPath = d.NginxConfPath
	}
	if p.HubSiteName == "" {
		p.HubSiteName = d.HubSiteName
	}
	if p.TestCmd == "" {
		p.TestCmd = d.TestCmd
	}
	if p.ReloadCmd == "" {
		p.ReloadCmd = d.ReloadCmd
	}
	return p
}

// CertCanonicalPathsIn 同 CertCanonicalPaths，但允许调用方指定非默认 CertDir。
// 用于 reconciler / handler 走 profile 的路径，避免再读包级 CertDir 常量。
func CertCanonicalPathsIn(p Profile, domain string) (cert, key string) {
	return p.CertDir + "/" + domain + "/fullchain.pem",
		p.CertDir + "/" + domain + "/privkey.pem"
}

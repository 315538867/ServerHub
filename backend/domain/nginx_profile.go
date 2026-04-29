package domain

import "time"

// NginxProfile 描述一台 edge 上 nginx 的部署形态。
type NginxProfile struct {
	ID                uint       `json:"id"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	DeletedAt         *time.Time `json:"-"`
	EdgeServerID      uint       `json:"edge_server_id"`
	NginxConfDir      string     `json:"nginx_conf_dir"`
	SitesAvailableDir string     `json:"sites_available_dir"`
	SitesEnabledDir   string     `json:"sites_enabled_dir"`
	AppLocationsDir   string     `json:"app_locations_dir"`
	StreamsConf       string     `json:"streams_conf"`
	CertDir           string     `json:"cert_dir"`
	NginxConfPath     string     `json:"nginx_conf_path"`
	HubSiteName       string     `json:"hub_site_name"`
	TestCmd           string     `json:"test_cmd"`
	ReloadCmd         string     `json:"reload_cmd"`
	BinaryPath        string     `json:"binary_path"`
	NginxVRaw         string     `json:"-"`
	Version           string     `json:"version"`
	BuildPrefix       string     `json:"build_prefix,omitempty"`
	BuildConf         string     `json:"build_conf,omitempty"`
	Modules           string     `json:"modules,omitempty"`
	LastProbeAt       *time.Time `json:"last_probe_at,omitempty"`
}

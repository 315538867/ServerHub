package model

import (
	"time"

	"gorm.io/gorm"
)

// NginxProfile 描述一台 edge 上 nginx 的部署形态——路径、命令、探测元数据，
// 让 Reconciler 不再硬编码 `/etc/nginx` / `nginx -t` / `nginx -s reload`。
//
// 多实例 / 非常规发行版常见差异：
//   - 自编译 nginx 装在 /usr/local/nginx，sites-* 由用户自行模拟；
//   - Debian/Ubuntu 标准 sites-available + sites-enabled 双目录；
//   - RHEL/CentOS 默认只有 conf.d，沿用 nginx -s reload；
//   - 容器化场景需要 systemctl reload nginx 或 nginx -p <prefix>。
//
// 字段空字符串表示"用 nginxrender.DefaultProfile() 兜底"——这样新建一台 edge
// 不需要人工填表也能 Apply（行为与 P2 完全一致），只在用户真要改路径或命令
// 时才落库一行 NginxProfile。
//
// 一台 edge 至多一份 profile（EdgeServerID 唯一）；删除 server 时 profile 也
// 一并删除。
type NginxProfile struct {
	gorm.Model
	EdgeServerID uint `gorm:"uniqueIndex;not null" json:"edge_server_id"`

	// ── 远端路径覆盖。空 = 用 nginxrender DefaultProfile。 ─────────────
	NginxConfDir      string `json:"nginx_conf_dir"`      // 例：/etc/nginx
	SitesAvailableDir string `json:"sites_available_dir"` // 例：/etc/nginx/sites-available
	SitesEnabledDir   string `json:"sites_enabled_dir"`
	AppLocationsDir   string `json:"app_locations_dir"`
	StreamsConf       string `json:"streams_conf"`
	CertDir           string `json:"cert_dir"`
	NginxConfPath     string `json:"nginx_conf_path"` // 顶层 nginx.conf 绝对路径
	HubSiteName       string `json:"hub_site_name"`

	// ── 命令覆盖。 ───────────────────────────────────────────────────
	// 期望返回 stdout+stderr 合并，且应自带 sudo / 2>&1。空走默认。
	TestCmd   string `json:"test_cmd"`   // 例：sudo -n nginx -t 2>&1
	ReloadCmd string `json:"reload_cmd"` // 例：sudo -n systemctl reload nginx 2>&1

	// ── nginx -V probe 缓存。Probe 接口每次写入；reconciler 只读。 ─────
	BinaryPath  string     `json:"binary_path"`            // which nginx 的解析结果
	NginxVRaw   string     `gorm:"type:text" json:"-"`     // nginx -V 全文
	Version     string     `json:"version"`                // 1.24.0 等
	BuildPrefix string     `json:"build_prefix,omitempty"` // --prefix=
	BuildConf   string     `json:"build_conf,omitempty"`   // --conf-path=
	Modules     string     `gorm:"type:text" json:"modules,omitempty"`
	LastProbeAt *time.Time `json:"last_probe_at,omitempty"`
}

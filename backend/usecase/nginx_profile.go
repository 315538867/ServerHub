// Package usecase: nginx_profile.go 收口 Nginx Profile 子域业务逻辑。
//
// 包含 Profile 读取、用户覆盖更新、远端 nginx -V 探测与回写。
// handler 只负责 DTO 解析 / 参数校验 / 回响应。
//
// TODO R7: 切 ports interface，移除 db *gorm.DB 入参。
package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/serverhub/serverhub/adapters/ingress/nginx/profile"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/pkg/runner"
	"github.com/serverhub/serverhub/repo"
	"gorm.io/gorm"
)

// NginxProfileResult 封装 NginxProfile 查询结果，Exists=false 表示库中尚无显式记录。
type NginxProfileResult struct {
	Profile domain.NginxProfile
	Exists  bool
}

// ── 查询 ──────────────────────────────────────────────────────────────────────

// GetNginxProfile 获取 edge server 的 NginxProfile；不存在时返回零值不报错。
func GetNginxProfile(ctx context.Context, db *gorm.DB, edgeID uint) (NginxProfileResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	np, err := repo.GetNginxProfileByEdgeID(ctx, db, edgeID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return NginxProfileResult{Profile: domain.NginxProfile{EdgeServerID: edgeID}, Exists: false}, nil
	}
	if err != nil {
		return NginxProfileResult{}, err
	}
	return NginxProfileResult{Profile: np, Exists: true}, nil
}

// ── 更新用户覆盖 ─────────────────────────────────────────────────────────────

// UpdateNginxProfileInput 是前端可写的 10 项用户覆盖字段。
type UpdateNginxProfileInput struct {
	NginxConfDir      string
	SitesAvailableDir string
	SitesEnabledDir   string
	AppLocationsDir   string
	StreamsConf       string
	CertDir           string
	NginxConfPath     string
	HubSiteName       string
	TestCmd           string
	ReloadCmd         string
}

// UpdateNginxProfile 更新（不存在时创建）edge server 的用户覆盖字段。
// 只操作用户可写的字段，不覆盖 probe 缓存列。
func UpdateNginxProfile(ctx context.Context, db *gorm.DB, edgeID uint, input UpdateNginxProfileInput) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	updates := map[string]any{
		"nginx_conf_dir":      input.NginxConfDir,
		"sites_available_dir": input.SitesAvailableDir,
		"sites_enabled_dir":   input.SitesEnabledDir,
		"app_locations_dir":   input.AppLocationsDir,
		"streams_conf":        input.StreamsConf,
		"cert_dir":            input.CertDir,
		"nginx_conf_path":     input.NginxConfPath,
		"hub_site_name":       input.HubSiteName,
		"test_cmd":            input.TestCmd,
		"reload_cmd":          input.ReloadCmd,
	}
	return repo.UpsertNginxProfile(ctx, db, edgeID, updates)
}

// ── 探测 ─────────────────────────────────────────────────────────────────────

// ProbeNginxProfile 在远端执行 nginx -V，解析结果回写到 NginxProfile 表（probe 字段）。
// 不修改用户配置路径/命令字段。
func ProbeNginxProfile(ctx context.Context, db *gorm.DB, cfg *config.Config, edgeID uint) (NginxProfileResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	srv, err := repo.GetServerByID(ctx, db, edgeID)
	if err != nil {
		return NginxProfileResult{}, err
	}
	rn, err := runner.For(&srv, cfg)
	if err != nil {
		return NginxProfileResult{}, err
	}

	pr, err := profile.ProbeNginxV(rn)
	if err != nil {
		return NginxProfileResult{}, err
	}

	modulesJSON, _ := json.Marshal(pr.Modules)
	now := time.Now()

	if err := repo.UpsertNginxProfile(ctx, db, edgeID, map[string]any{
		"binary_path":   pr.BinaryPath,
		"nginx_v_raw":   pr.Raw,
		"version":       pr.Version,
		"build_prefix":  pr.BuildPrefix,
		"build_conf":    pr.BuildConf,
		"modules":       string(modulesJSON),
		"last_probe_at": &now,
	}); err != nil {
		return NginxProfileResult{}, err
	}

	return GetNginxProfile(ctx, db, edgeID)
}


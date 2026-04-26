package nginxops

import (
	"errors"

	"gorm.io/gorm"

	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/nginxrender"
)

// LoadProfile 取出 edge 的 NginxProfile 行（若有）并经 NormalizeProfile 兜底成
// 全字段非空的 nginxrender.Profile。
//
//   - 没有记录 → DefaultProfile（与 P2 行为完全一致，老 edge 无感）
//   - 有记录但部分字段为空 → 用 default 值填补该字段（用户只覆盖一两项的常见场景）
//
// 任何 DB 错误（除 RecordNotFound）原样返回，调用方决定是否中断 Apply——一般
// reconciler 会让它失败，避免拿一份拼凑的 profile 去写远端。
func LoadProfile(db *gorm.DB, edgeID uint) (nginxrender.Profile, *model.NginxProfile, error) {
	var np model.NginxProfile
	err := db.Where("edge_server_id = ?", edgeID).First(&np).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nginxrender.DefaultProfile(), nil, nil
	}
	if err != nil {
		return nginxrender.Profile{}, nil, err
	}
	rp := nginxrender.NormalizeProfile(nginxrender.Profile{
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
	return rp, &np, nil
}

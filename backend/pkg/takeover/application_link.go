package takeover

import (
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/discovery"
	"gorm.io/gorm"
)

// attachToApplication 在接管落库前给 Service 填充 ApplicationID / SourceFingerprint，
// 并确保同 TargetName 的 Application 存在（按 name 全局 unique 做 upsert）。
// 返回要写入的 Application.ID，让上层在 db.Create(&svc) 成功后还可以回写
// PrimaryServiceID（见 finalizeApplicationLink）。
//
// 说明：Application.Name 是全局 unique，但接管时要求前端传入的 TargetName 与
// Application 名同名（典型场景，一条 Service 首次建立；后续同名接管会复用）。
func attachToApplication(db *gorm.DB, svc *model.Service, cand discovery.Candidate, targetName string) (uint, error) {
	var app model.Application
	err := db.Where("name = ?", targetName).First(&app).Error
	if err == gorm.ErrRecordNotFound {
		app = model.Application{
			Name:     targetName,
			ServerID: svc.ServerID,
			Status:   "unknown",
		}
		if e := db.Create(&app).Error; e != nil {
			return 0, e
		}
	} else if err != nil {
		return 0, err
	} else if app.ServerID != svc.ServerID {
		// 同名 Application 已存在但挂在别的 Server 下：接管在错服务器上是歧义，
		// 直接报错让上层走回滚。
		return 0, gorm.ErrInvalidValue
	}

	appID := app.ID
	svc.ApplicationID = &appID
	svc.SourceFingerprint = discovery.Fingerprint(cand)
	return appID, nil
}

// finalizeApplicationLink 在 Service 写入成功后，如果对应 Application 还没有
// 主服务，就把当前 Service 设为 PrimaryService。已有主服务则保持不动。
func finalizeApplicationLink(db *gorm.DB, appID, serviceID uint) {
	db.Model(&model.Application{}).
		Where("id = ? AND primary_service_id IS NULL", appID).
		Update("primary_service_id", serviceID)
}

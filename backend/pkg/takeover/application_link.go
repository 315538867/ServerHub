package takeover

import (
	"errors"
	"fmt"

	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/discovery"
	"github.com/serverhub/serverhub/pkg/safeshell"
	"gorm.io/gorm"
)

// attachToApplication 在接管落库前给 Service 按 req.AppMode 决定归属策略，
// 并回填 SourceFingerprint。返回要关联的 Application.ID（0 表示 floating 模式
// 不绑定任何 App）。签名一次性覆盖 4 个 runner 的相同调用点。
//
// 三种模式：
//   - "floating"（或空字符串）：ApplicationID 保持 nil，不 upsert 任何 App
//   - "existing"：req.AppID 必填，App 必须存在且 ServerID 匹配
//   - "new"：req.AppName 必填且合法，全局 unique；若同名已存在则拒绝（避免误关联）
//
// 返回的 appID 若 > 0，上层 Service 写入成功后应调用 finalizeApplicationLink
// 把当前 Service 登记为 Application.PrimaryServiceID（若还没有主服务）。
func attachToApplication(db *gorm.DB, svc *model.Service, cand discovery.Candidate, req Request) (uint, error) {
	svc.SourceFingerprint = discovery.Fingerprint(cand)

	mode := req.AppMode
	if mode == "" {
		mode = "floating"
	}

	switch mode {
	case "floating":
		svc.ApplicationID = nil
		return 0, nil

	case "existing":
		if req.AppID == nil || *req.AppID == 0 {
			return 0, errors.New("app_mode=existing 需要提供 app_id")
		}
		var app model.Application
		if err := db.First(&app, *req.AppID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return 0, fmt.Errorf("app_id=%d 不存在", *req.AppID)
			}
			return 0, err
		}
		if app.ServerID != svc.ServerID {
			return 0, fmt.Errorf("app_id=%d 绑定在 server_id=%d，与当前接管目标 server_id=%d 不符",
				app.ID, app.ServerID, svc.ServerID)
		}
		appID := app.ID
		svc.ApplicationID = &appID
		return appID, nil

	case "new":
		name := req.AppName
		if name == "" {
			name = req.TargetName
		}
		if err := safeshell.ValidName(name, 64); err != nil {
			return 0, fmt.Errorf("app_name 非法: %w", err)
		}
		// Application.Name 全局 unique — 直接拒绝重名，避免误把新服务挂到别人的 App 上
		var existing model.Application
		err := db.Where("name = ?", name).First(&existing).Error
		if err == nil {
			return 0, fmt.Errorf("应用 %q 已存在（server_id=%d），请选择 existing 模式绑定到该应用",
				name, existing.ServerID)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, err
		}
		app := model.Application{
			Name:     name,
			ServerID: svc.ServerID,
		}
		if e := db.Create(&app).Error; e != nil {
			return 0, e
		}
		appID := app.ID
		svc.ApplicationID = &appID
		return appID, nil

	default:
		return 0, fmt.Errorf("未知 app_mode=%q（可选 floating|existing|new）", mode)
	}
}

// finalizeApplicationLink 在 Service 写入成功后，如果对应 Application 还没有
// 主服务，就把当前 Service 设为 PrimaryService。已有主服务则保持不动。
// 对 appID=0（floating 模式）是 no-op。
func finalizeApplicationLink(db *gorm.DB, appID, serviceID uint) {
	if appID == 0 {
		return
	}
	db.Model(&model.Application{}).
		Where("id = ? AND primary_service_id IS NULL", appID).
		Update("primary_service_id", serviceID)
}

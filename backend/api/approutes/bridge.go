package approutes

import (
	"errors"

	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

// syncToIngress 把 AppNginxRoute 的写入同步到 Ingress/IngressRoute 新模型。
//
// P0 桥接策略：
//   - app.ExposeMode=="none" 或 app.Domain=="" → 跳过（没法形成 Ingress）。
//   - ExposeMode "site" → MatchKind=domain；ExposeMode "path" → MatchKind=path。
//   - 用 (EdgeServerID=app.RunServerID, Domain=app.Domain) FirstOrCreate Ingress。
//   - IngressRoute 用 LegacyAppRouteID 唯一映射回旧表，存在则覆盖、否则新建。
//
// 失败永远返回错误但不破坏旧表写入：调用方应记录日志而非阻断 API。Renderer
// 在 P1 之前不读 Ingress 表，桥接错位也不会影响线上 nginx。
func syncToIngress(db *gorm.DB, app *model.Application, route *model.AppNginxRoute) error {
	if app == nil || route == nil {
		return errors.New("syncToIngress: nil arg")
	}
	if app.ExposeMode == "none" || app.ExposeMode == "" {
		return nil
	}
	if app.Domain == "" {
		// path 模式无域名时，旧逻辑落到 server_name _。新模型不允许空 domain，
		// 用 "_" 作占位，渲染时再判定。
		app.Domain = "_"
	}

	matchKind := "domain"
	if app.ExposeMode == "path" {
		matchKind = "path"
	}

	edge := app.RunServerID
	if edge == 0 {
		edge = app.ServerID
	}
	if edge == 0 {
		return errors.New("syncToIngress: app 没有 server_id")
	}

	ing := model.Ingress{
		EdgeServerID: edge,
		Domain:       app.Domain,
		MatchKind:    matchKind,
	}
	if err := db.Where(model.Ingress{EdgeServerID: edge, Domain: app.Domain}).
		Attrs(model.Ingress{MatchKind: matchKind, Status: "pending"}).
		FirstOrCreate(&ing).Error; err != nil {
		return err
	}

	// MatchKind 漂移：旧 Ingress 存在但 mode 变了，更新一下（同 domain 不允许混用）。
	if ing.MatchKind != matchKind {
		db.Model(&ing).Update("match_kind", matchKind)
	}

	upstream := model.IngressUpstream{Type: "raw", RawURL: route.Upstream}

	var existing model.IngressRoute
	err := db.Where("legacy_app_route_id = ?", route.ID).First(&existing).Error
	switch {
	case err == nil:
		existing.IngressID = ing.ID
		existing.Path = route.Path
		existing.Sort = route.Sort
		existing.Extra = route.Extra
		existing.Upstream = upstream
		return db.Save(&existing).Error
	case errors.Is(err, gorm.ErrRecordNotFound):
		legacyID := route.ID
		ir := model.IngressRoute{
			IngressID:        ing.ID,
			Sort:             route.Sort,
			Path:             route.Path,
			Protocol:         "http",
			Upstream:         upstream,
			Extra:            route.Extra,
			LegacyAppRouteID: &legacyID,
		}
		return db.Create(&ir).Error
	default:
		return err
	}
}

// resyncAppRoutes 在 ExposeMode 变化时重灌该 app 的所有桥接行。
//
//   - 新 mode = "none"：把该 app 全部桥接行清掉（含空壳 Ingress）。
//   - 新 mode = "path"/"site"：遍历 AppNginxRoute 全量调用 syncToIngress。
func resyncAppRoutes(db *gorm.DB, app *model.Application) error {
	if app == nil {
		return errors.New("resyncAppRoutes: nil app")
	}
	if app.ExposeMode == "none" || app.ExposeMode == "" {
		var routes []model.AppNginxRoute
		if err := db.Where("app_id = ?", app.ID).Find(&routes).Error; err != nil {
			return err
		}
		for _, r := range routes {
			if err := removeIngressRouteByLegacy(db, r.ID); err != nil {
				return err
			}
		}
		return nil
	}
	var routes []model.AppNginxRoute
	if err := db.Where("app_id = ?", app.ID).Find(&routes).Error; err != nil {
		return err
	}
	for i := range routes {
		if err := syncToIngress(db, app, &routes[i]); err != nil {
			return err
		}
	}
	return nil
}

// removeIngressRouteByLegacy 删除桥接的 IngressRoute 行。如果删除后该 Ingress
// 下再无任何 IngressRoute，把 Ingress 也一并清掉避免空壳。
func removeIngressRouteByLegacy(db *gorm.DB, legacyRouteID uint) error {
	var ir model.IngressRoute
	err := db.Where("legacy_app_route_id = ?", legacyRouteID).First(&ir).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	ingID := ir.IngressID
	if err := db.Delete(&ir).Error; err != nil {
		return err
	}
	var cnt int64
	db.Model(&model.IngressRoute{}).Where("ingress_id = ?", ingID).Count(&cnt)
	if cnt == 0 {
		db.Delete(&model.Ingress{}, ingID)
	}
	return nil
}

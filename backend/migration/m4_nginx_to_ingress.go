// Package migration / M4: 把现存 AppNginxRoute 灌进 Ingress / IngressRoute。
//
// 调用方式：
//
//	serverhub -migrate=m4-dryrun -config ...   只打印报告不写库
//	serverhub -migrate=m4        -config ...   正式执行（带幂等标记）
//
// 幂等保证：执行成功后在 settings 表写入 key=migration.m4.done。同一行被同步过
// 后会带 LegacyAppRouteID，重跑会跳过。
package migration

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

const migrationM4DoneKey = "migration.m4.done"

// M4Report 聚合一次迁移的写入计数。
type M4Report struct {
	DryRun           bool     `json:"dry_run"`
	AlreadyDone      bool     `json:"already_done"`
	RoutesSeen       int      `json:"routes_seen"`
	RoutesMigrated   int      `json:"routes_migrated"`
	RoutesSkipped    int      `json:"routes_skipped"`
	IngressesCreated int      `json:"ingresses_created"`
	Skipped          []string `json:"skipped"`
}

// RunM4 执行 M4 迁移。dryRun=true 只统计，不写库。
func RunM4(db *gorm.DB, dryRun bool) (*M4Report, error) {
	rep := &M4Report{DryRun: dryRun, Skipped: []string{}}

	if !dryRun {
		var s model.Setting
		if err := db.Where("key = ?", migrationM4DoneKey).First(&s).Error; err == nil {
			rep.AlreadyDone = true
			return rep, nil
		}
	}

	var routes []model.AppNginxRoute
	if err := db.Order("app_id asc, sort asc, id asc").Find(&routes).Error; err != nil {
		return nil, fmt.Errorf("load app_nginx_routes: %w", err)
	}
	rep.RoutesSeen = len(routes)

	tx := db
	if !dryRun {
		tx = db.Begin()
		if tx.Error != nil {
			return nil, tx.Error
		}
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
				panic(r)
			}
		}()
	}

	// 缓存：(edge_server_id, domain) → ingress.ID，避免每条 route 都 FirstOrCreate。
	type ingKey struct {
		edge   uint
		domain string
	}
	ingCache := map[ingKey]uint{}

	for _, rt := range routes {
		var app model.Application
		if err := tx.First(&app, rt.AppID).Error; err != nil {
			rep.RoutesSkipped++
			rep.Skipped = append(rep.Skipped, fmt.Sprintf("route=%d: app=%d 不存在", rt.ID, rt.AppID))
			continue
		}
		if app.ExposeMode == "none" || app.ExposeMode == "" {
			rep.RoutesSkipped++
			rep.Skipped = append(rep.Skipped, fmt.Sprintf("route=%d: app=%d expose_mode=%q 跳过", rt.ID, app.ID, app.ExposeMode))
			continue
		}
		domain := app.Domain
		if domain == "" {
			domain = "_"
		}
		edge := app.RunServerID
		if edge == 0 {
			edge = app.ServerID
		}
		if edge == 0 {
			rep.RoutesSkipped++
			rep.Skipped = append(rep.Skipped, fmt.Sprintf("route=%d: app=%d 没有 server_id", rt.ID, app.ID))
			continue
		}
		matchKind := "domain"
		if app.ExposeMode == "path" {
			matchKind = "path"
		}

		// 已迁移过则跳过（幂等）
		var existing model.IngressRoute
		err := tx.Where("legacy_app_route_id = ?", rt.ID).First(&existing).Error
		if err == nil {
			rep.RoutesSkipped++
			rep.Skipped = append(rep.Skipped, fmt.Sprintf("route=%d: 已迁移", rt.ID))
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			if !dryRun {
				return rollbackM4(tx, rep, err)
			}
			return nil, err
		}

		// Ingress：缓存命中或 FirstOrCreate
		key := ingKey{edge: edge, domain: domain}
		ingID, hit := ingCache[key]
		if !hit {
			ing := model.Ingress{
				EdgeServerID: edge,
				Domain:       domain,
				MatchKind:    matchKind,
				Status:       "pending",
			}
			if !dryRun {
				if err := tx.Where(model.Ingress{EdgeServerID: edge, Domain: domain}).
					Attrs(model.Ingress{MatchKind: matchKind, Status: "pending"}).
					FirstOrCreate(&ing).Error; err != nil {
					return rollbackM4(tx, rep, err)
				}
				if ing.MatchKind != matchKind {
					tx.Model(&ing).Update("match_kind", matchKind)
				}
				ingID = ing.ID
			} else {
				ingID = 0 // dryrun 不分配 ID
			}
			ingCache[key] = ingID
			rep.IngressesCreated++
		}

		legacyID := rt.ID
		ir := model.IngressRoute{
			IngressID:        ingID,
			Sort:             rt.Sort,
			Path:             rt.Path,
			Protocol:         "http",
			Upstream:         model.IngressUpstream{Type: "raw", RawURL: rt.Upstream},
			Extra:            rt.Extra,
			LegacyAppRouteID: &legacyID,
		}
		if !dryRun {
			if err := tx.Create(&ir).Error; err != nil {
				return rollbackM4(tx, rep, err)
			}
		}
		rep.RoutesMigrated++
	}

	if !dryRun {
		if err := tx.Commit().Error; err != nil {
			return rep, err
		}
		markDoneM4(db, rep)
	}
	return rep, nil
}

func rollbackM4(tx *gorm.DB, rep *M4Report, err error) (*M4Report, error) {
	_ = tx.Rollback()
	return rep, err
}

func markDoneM4(db *gorm.DB, rep *M4Report) {
	payload, _ := json.Marshal(rep)
	db.Save(&model.Setting{
		Key:       migrationM4DoneKey,
		Value:     string(payload),
		UpdatedAt: time.Now(),
	})
}

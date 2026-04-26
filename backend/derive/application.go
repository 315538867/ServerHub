package derive

import (
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/svcstatus"
	"gorm.io/gorm"
)

// AppStatusKind 是 application 的派生状态枚举。
type AppStatusKind = string

const (
	AppStatusError   AppStatusKind = "error"
	AppStatusSyncing AppStatusKind = "syncing"
	AppStatusRunning AppStatusKind = "running"
	AppStatusUnknown AppStatusKind = "unknown"
)

// AppStatusEntry 是某 application 的派生摘要。
type AppStatusEntry struct {
	Result AppStatusKind
}

// AppStatus 接收 applicationID 列表,返回 app_id → 派生状态。
//
// 派生规则(优先级降序):
//  1. 任一下属 Service 最新 DeployRun.Status == "failed"  → error
//  2. 任一下属 Service 最新 DeployRun.Status == "syncing" → syncing
//  3. 全部下属 Service 最新 DeployRun.Status == "success" → running
//     (无 DeployRun 视作 success,与 takeover 接管的隐含约定一致)
//  4. 其它(空 app / 全 unknown) → unknown
//
// 算法:
//   - 一条 SQL 拉 services WHERE application_id IN(取 app_id, service_id)
//   - svcstatus.LatestByService 一次拿到所有 service 的最近 DeployRun.Status
//   - 内存按 app_id 聚合
//
// 性能:app/svc 量级与现有 listHandler 相同,单 query + 单聚合调用,与原内联实现等价。
func AppStatus(db *gorm.DB, appIDs []uint) map[uint]AppStatusEntry {
	out := make(map[uint]AppStatusEntry, len(appIDs))
	if len(appIDs) == 0 {
		return out
	}

	type row struct {
		ApplicationID uint
		ServiceID     uint
	}
	var rows []row
	_ = db.Model(&model.Service{}).
		Select("application_id, id AS service_id").
		Where("application_id IN ?", appIDs).
		Scan(&rows).Error

	svcIDs := make([]uint, 0, len(rows))
	for _, r := range rows {
		svcIDs = append(svcIDs, r.ServiceID)
	}
	latest := svcstatus.LatestByService(db, svcIDs)

	type agg struct {
		hasFailed  bool
		hasSyncing bool
		allSuccess bool
		hasService bool
	}
	groups := make(map[uint]*agg, len(appIDs))
	for _, id := range appIDs {
		groups[id] = &agg{allSuccess: true}
	}
	for _, r := range rows {
		g := groups[r.ApplicationID]
		g.hasService = true
		// 无 DeployRun → success(takeover 接管约定,与 svcstatus 包注释一致)
		st := latest[r.ServiceID].Status
		if st == "" {
			st = "success"
		}
		switch st {
		case "failed":
			g.hasFailed = true
			g.allSuccess = false
		case "syncing":
			g.hasSyncing = true
			g.allSuccess = false
		case "success":
			// keep
		default:
			g.allSuccess = false
		}
	}

	for id, g := range groups {
		var kind AppStatusKind
		switch {
		case !g.hasService:
			kind = AppStatusUnknown
		case g.hasFailed:
			kind = AppStatusError
		case g.hasSyncing:
			kind = AppStatusSyncing
		case g.allSuccess:
			kind = AppStatusRunning
		default:
			kind = AppStatusUnknown
		}
		out[id] = AppStatusEntry{Result: kind}
	}
	return out
}

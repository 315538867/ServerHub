// Package svcstatus 提供 Service 表上"已下线但前端仍要展示"字段的派生工具。
//
// 历史背景:Service 表早期持有 LastStatus/LastRunAt/ImageName 等摘要列。M3 起
// 这些列陆续从 model 上下线,真值改由 DeployRun(状态)/ Release.StartSpec(image)
// 等下游表派生。本包是它们集中的派生入口,目的是让 listServicesHandler /
// Application 状态聚合在一次批量调用里就拿到所有派生字段,避免 N 次单点查询。
//
// 设计取舍:
//   - 单一批量 API LatestByService:接收 ServiceID 列表,返回 service_id → Entry,
//     Entry 是多字段聚合(Status/StartedAt 来自 DeployRun,Image 来自 Release.StartSpec)。
//     调用方无需关心数据来自哪张表,加新派生字段时调用方零改动。
//   - DeployRun 一行一次部署,deploy_runs(service_id, started_at DESC) 索引已建,
//     单条 LIMIT 1 查询 O(log n)。但 listServicesHandler / Application 聚合都是
//     批量场景,N 条 Service 各自单点查会放大成 N 次 round trip。因此走子查询
//     一次 GROUP BY 取 MAX(started_at)。
//   - Image 走 Service.CurrentReleaseID → Release.StartSpec(JSON) 解出来,
//     CurrentReleaseID 已建 idx_svc_current_release,联表是 O(N)。
//   - 两段 query 不复用同一条 SQL —— Status 来自 deploy_runs,Image 来自 releases,
//     物理表完全独立,join 在一起反而拉长执行计划。分开两段、最后合并 map,代码
//     直观、SQLite 上各 O(N)。
//   - "无 DeployRun 行 = success" 是 takeover 接管的隐含约定,本包不替 caller
//     做兜底,免得吞掉真正应该被看见的"服务存在但还没部署过"语义;Image 同理,
//     缺失就是 "" 让 caller 自决。
package svcstatus

import (
	"encoding/json"
	"time"

	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

// Entry 是某 Service 的派生摘要。
//
// 字段独立,任何字段缺失(对应底表无行 / JSON 无该 key)都用 zero value 表达,
// caller 通过 if e, ok := m[id]; ok 判断"该 service 至少有一项派生数据"。
// 想精确判断"是否有 DeployRun" 看 StartedAt.IsZero();判断"是否有 Image" 看
// Image == ""。
type Entry struct {
	Status    string
	StartedAt time.Time
	Image     string
}

// LatestByService 接收一组 ServiceID,返回 service_id → 派生摘要 Entry。
//
// 内部走两段独立查询合并:
//  1. deploy_runs 子查询取每个 service_id 的 MAX(started_at) → 填 Status/StartedAt
//  2. services join releases 取 current_release_id 对应 StartSpec → 解 JSON 填 Image
//
// 任一段没命中,对应字段就是 zero value;两段都没命中的 service_id 在 map 里缺席。
// caller 用 ok-check 判断有无任何派生数据,字段缺省语义自行裁定。
func LatestByService(db *gorm.DB, ids []uint) map[uint]Entry {
	out := make(map[uint]Entry, len(ids))
	if len(ids) == 0 {
		return out
	}

	// 段一:最近一条 DeployRun 的 status + started_at
	type runRow struct {
		ServiceID uint
		Status    string
		StartedAt time.Time
	}
	var runRows []runRow
	sub := db.Model(&model.DeployRun{}).
		Select("service_id, MAX(started_at) AS started_at").
		Where("service_id IN ?", ids).
		Group("service_id")
	_ = db.Table("deploy_runs AS r").
		Select("r.service_id, r.status, r.started_at").
		Joins("JOIN (?) AS m ON m.service_id = r.service_id AND m.started_at = r.started_at", sub).
		Scan(&runRows).Error
	for _, r := range runRows {
		e := out[r.ServiceID]
		e.Status = r.Status
		e.StartedAt = r.StartedAt
		out[r.ServiceID] = e
	}

	// 段二:CurrentReleaseID 指向的 Release.StartSpec → image
	// 仅查有 current_release_id 的 Service,无关行不进 join。
	type relRow struct {
		ServiceID uint
		StartSpec string
	}
	var relRows []relRow
	_ = db.Table("services AS s").
		Select("s.id AS service_id, rel.start_spec AS start_spec").
		Joins("JOIN releases AS rel ON rel.id = s.current_release_id").
		Where("s.id IN ? AND s.current_release_id IS NOT NULL", ids).
		Scan(&relRows).Error
	for _, r := range relRows {
		img := imageFromStartSpec(r.StartSpec)
		if img == "" {
			continue
		}
		e := out[r.ServiceID]
		e.Image = img
		out[r.ServiceID] = e
	}

	return out
}

// imageFromStartSpec 从 Release.StartSpec(JSON) 抠出 image 字段。
// 仅 docker 类型的 StartSpec 会有 image key;compose/native/static 该字段不存在,
// 返回 ""。JSON 解析失败也返回 "" —— 派生侧不替历史脏数据兜底。
func imageFromStartSpec(s string) string {
	if s == "" {
		return ""
	}
	var spec map[string]any
	if err := json.Unmarshal([]byte(s), &spec); err != nil {
		return ""
	}
	if v, ok := spec["image"].(string); ok {
		return v
	}
	return ""
}

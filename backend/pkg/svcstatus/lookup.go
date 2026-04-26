// Package svcstatus 提供 Service 运行状态的派生工具。P-G 起 Service 不再持有
// LastStatus/LastRunAt 摘要列,取而代之由"最近一条 DeployRun"作为单一事实源。
//
// 设计取舍:
//   - DeployRun 一行一次部署,deploy_runs(service_id, started_at DESC) 索引
//     已建,单条 LIMIT 1 查询 O(log n)。但 listServicesHandler / Application
//     状态聚合都是批量场景,N 条 Service 各自单点查会放大成 N 次 round trip。
//   - 因此本包暴露的是批量 API LatestStatusByService,内部一次 GROUP BY
//     subquery 取每个 service_id 的 MAX(started_at) 行。
//   - "无 DeployRun 行 = success" 是 takeover 接管的隐含约定 —— takeover 把
//     Service 直接落库不会创建 DeployRun,这种 Service 在派生侧应被视为正常
//     运行而非"未知"。caller 拿不到 entry 的时候自行回退即可,本包不替 caller
//     做兜底,免得吞掉真正应该被看见的"服务存在但还没部署过"语义。
package svcstatus

import (
	"time"

	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

// Entry 是某 Service 最近一条 DeployRun 的精简投影。
type Entry struct {
	Status    string
	StartedAt time.Time
}

// LatestByService 接收一组 ServiceID,返回 service_id → 最近一条 DeployRun 的
// {Status,StartedAt}。没有 DeployRun 的 Service 不会出现在 map 里 —— 调用方据此
// 决定是否把它视作"接管成功默认值"或者"未运行"。
//
// 实现走子查询:外层从 deploy_runs 取 (service_id, MAX(started_at)) 后内连接
// 自身回拿 status。比"按 ServiceID 循环 LIMIT 1"少 N-1 次往返,在 SQLite 上
// 也走 idx_run_svc(service_id, started_at DESC),不会全表扫描。
func LatestByService(db *gorm.DB, ids []uint) map[uint]Entry {
	out := make(map[uint]Entry, len(ids))
	if len(ids) == 0 {
		return out
	}
	// 子查询先取每个 service_id 的最大 started_at;外层连回 deploy_runs 拿 status。
	// 用 (service_id, started_at) 双键 join 避免重复 started_at 时取错行(同一
	// 微秒并发部署,理论上 ServerHub 单机串行不会发生,但保险起见 LIMIT 也无损)。
	type row struct {
		ServiceID uint
		Status    string
		StartedAt time.Time
	}
	var rows []row
	sub := db.Model(&model.DeployRun{}).
		Select("service_id, MAX(started_at) AS started_at").
		Where("service_id IN ?", ids).
		Group("service_id")
	if err := db.Table("deploy_runs AS r").
		Select("r.service_id, r.status, r.started_at").
		Joins("JOIN (?) AS m ON m.service_id = r.service_id AND m.started_at = r.started_at", sub).
		Scan(&rows).Error; err != nil {
		return out
	}
	for _, r := range rows {
		out[r.ServiceID] = Entry{Status: r.Status, StartedAt: r.StartedAt}
	}
	return out
}

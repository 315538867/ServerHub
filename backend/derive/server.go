// Package derive 是 ServerHub v2 的"读端派生层":输入若干 model 实体的 ID,
// 返回从底层时序/事实表聚合出来的派生状态。
//
// 真值/派生分离铁律(R3 起):
//   - 真值 = 不可派生 / 不可重算 的事实(用户输入、外部探测结果、部署事件)
//   - 派生 = 真值的某种聚合 / 投影(状态摘要、最近一次事件)
//
// model 包只装真值(append-only 时序表 + 用户实体);派生函数全在本包里。
// handler 调 derive.X 拿派生结果拼 DTO,不再 SELECT 摘要列、不再内联派生算法。
//
// 文件分布:
//   - server.go        -- ServerStatus(基于 server_probes 时序表)
//   - application.go   -- AppStatus(基于 services + deploy_runs 聚合)
//
// 后续 R6 把 *gorm.DB 收口进 repo 接口,本包将依赖 repo 而非 GORM,接口形态不变。
package derive

import (
	"time"

	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

// 探测阈值默认值。handler 可从 settings 表读自定义阈值后注入,缺省走这里。
//
// 选取依据:
//   - 心跳频率假设 30~60s 一次(metrics 采集 / 用户手测)
//   - 2 倍周期内未到视为 lagging,5 倍周期内未到视为 offline
const (
	DefaultLaggingAfter = 2 * time.Minute
	DefaultOfflineAfter = 5 * time.Minute
)

// ServerStatusKind 是 server 的派生状态枚举。
type ServerStatusKind = string

const (
	ServerStatusOnline  ServerStatusKind = "online"
	ServerStatusLagging ServerStatusKind = "lagging"
	ServerStatusOffline ServerStatusKind = "offline"
	ServerStatusUnknown ServerStatusKind = "unknown"
)

// ServerStatusEntry 是某 server 的派生摘要。
//
// LastProbeAt.IsZero() == true 表示该 server 从未有过探测记录(unknown)。
// Result 是派生后的最终状态(已套用阈值),不是 server_probes.result 原值。
type ServerStatusEntry struct {
	Result      ServerStatusKind
	LastProbeAt time.Time
	LatencyMs   int
	ErrMsg      string
}

// ServerStatus 接收 serverID 列表,返回 server_id → 派生状态。
//
// 算法:
//  1. 一条 SQL 取每个 server 的最近一条 probe(GROUP BY server_id + JOIN MAX(created_at))
//  2. 缺行 → unknown
//  3. result=offline → offline(阈值不影响显式离线)
//  4. result=online + age<lagging → online
//  5. result=online + age<offline → lagging
//  6. result=online + age≥offline → offline(久未续约,视同离线)
//
// 性能:server 数量假设 < 1000,GROUP BY 复合索引 (server_id, created_at desc) 命中,
// 单 SQL 完成。后续涨到万级可加 5s 进程内缓存。
func ServerStatus(db *gorm.DB, ids []uint, laggingAfter, offlineAfter time.Duration) map[uint]ServerStatusEntry {
	out := make(map[uint]ServerStatusEntry, len(ids))
	if len(ids) == 0 {
		return out
	}
	if laggingAfter <= 0 {
		laggingAfter = DefaultLaggingAfter
	}
	if offlineAfter <= 0 {
		offlineAfter = DefaultOfflineAfter
	}

	// 取每个 server 的最近一条 probe:子查询 MAX(created_at) GROUP BY server_id,
	// 再 JOIN 回 server_probes 拿全字段。与 svcstatus.LatestByService 同模式。
	type row struct {
		ServerID  uint
		Result    string
		LatencyMs int
		ErrMsg    string
		CreatedAt time.Time
	}
	var rows []row
	sub := db.Model(&model.ServerProbe{}).
		Select("server_id, MAX(created_at) AS created_at").
		Where("server_id IN ?", ids).
		Group("server_id")
	_ = db.Table("server_probes AS p").
		Select("p.server_id, p.result, p.latency_ms, p.err_msg, p.created_at").
		Joins("JOIN (?) AS m ON m.server_id = p.server_id AND m.created_at = p.created_at", sub).
		Scan(&rows).Error

	now := time.Now()
	for _, r := range rows {
		age := now.Sub(r.CreatedAt)
		var kind ServerStatusKind
		switch {
		case r.Result == "offline":
			kind = ServerStatusOffline
		case age >= offlineAfter:
			kind = ServerStatusOffline
		case age >= laggingAfter:
			kind = ServerStatusLagging
		default:
			kind = ServerStatusOnline
		}
		out[r.ServerID] = ServerStatusEntry{
			Result:      kind,
			LastProbeAt: r.CreatedAt,
			LatencyMs:   r.LatencyMs,
			ErrMsg:      r.ErrMsg,
		}
	}

	// 没行的 server → unknown(显式入 map 让 caller 不必再做 ok-check)
	for _, id := range ids {
		if _, ok := out[id]; !ok {
			out[id] = ServerStatusEntry{Result: ServerStatusUnknown}
		}
	}
	return out
}

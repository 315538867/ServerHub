// Package apprelease 实现 Phase M3 的 AppReleaseSet（App 级发布集）业务层。
//
// 一个 AppReleaseSet 把 App 下多个 Service 的 Release 选择快照成原子单元。
// Apply 按 Items 顺序串行调用 deployer.ApplyRelease，期间通过 emit 流式回调向
// HTTP SSE 推送事件。Rollback 针对上一次 Apply 成功的 Service 反向 Apply 到
// 先前历史 Release。
package apprelease

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/deployer"
	"gorm.io/gorm"
)

// Emit 是统一流式事件回调签名。name 为事件名，data 会被 JSON 序列化后推给客户端。
// 传 nil 表示调用方不关心事件（例如 HTTP 连接已断开或非流式入口）。
type Emit func(name string, data any)

// ─────────────────────────────── JSON helpers ────────────────────────────────

func parseItems(raw string) []model.AppReleaseSetItem {
	if raw == "" {
		return nil
	}
	var out []model.AppReleaseSetItem
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

func encodeItems(items []model.AppReleaseSetItem) string {
	b, _ := json.Marshal(items)
	return string(b)
}

func encodeSummary(items []model.AppReleaseSummaryItem) string {
	b, _ := json.Marshal(items)
	return string(b)
}

// ─────────────────────────────── Label ──────────────────────────────────────

// autoLabel 生成 YYYY-MM-DD-N 兜底标签（同 App 内当日递增）。
func autoLabel(db *gorm.DB, appID uint) string {
	today := time.Now().Format("2006-01-02")
	var n int64
	db.Model(&model.AppReleaseSet{}).
		Where("application_id = ? AND label LIKE ?", appID, today+"-%").Count(&n)
	return today + "-" + strconv.FormatInt(n+1, 10)
}

// ─────────────────────────── CreateFromCurrent ──────────────────────────────

// CreateFromCurrent 扫描 App 下所有已绑定 CurrentReleaseID 的 Service，生成一份
// Release 组合快照。未绑定 Release 的 Service 被跳过（不阻断创建）。
// 若 App 下没有任何可组合 Service，返回错误。
func CreateFromCurrent(db *gorm.DB, appID uint, label, note, createdBy string) (*model.AppReleaseSet, error) {
	var svcs []model.Service
	if err := db.Where("application_id = ?", appID).Find(&svcs).Error; err != nil {
		return nil, err
	}
	items := make([]model.AppReleaseSetItem, 0, len(svcs))
	for _, s := range svcs {
		if s.CurrentReleaseID == nil || *s.CurrentReleaseID == 0 {
			continue
		}
		items = append(items, model.AppReleaseSetItem{
			ServiceID: s.ID,
			ReleaseID: *s.CurrentReleaseID,
		})
	}
	if len(items) == 0 {
		return nil, errors.New("app has no services with current release")
	}
	if label == "" {
		label = autoLabel(db, appID)
	}
	set := &model.AppReleaseSet{
		ApplicationID: appID,
		Label:         label,
		Items:         encodeItems(items),
		Note:          note,
		CreatedBy:     createdBy,
		Status:        model.AppReleaseSetStatusDraft,
	}
	if err := db.Create(set).Error; err != nil {
		return nil, err
	}
	return set, nil
}

// ────────────────────────── Apply / Rollback 共享执行单元 ────────────────────

// runOne 执行单个 (serviceID, releaseID) 部署，返回该 Service 的 summary 项与
// 是否计为成功。emit 为 nil 时只跑业务不推 SSE。
func runOne(
	db *gorm.DB, cfg *config.Config,
	svcID, relID uint, idx, total int,
	triggerSource string, emit Emit,
) (model.AppReleaseSummaryItem, bool) {
	safeEmit(emit, "service_started", map[string]any{
		"service_id": svcID,
		"release_id": relID,
		"idx":        idx,
		"total":      total,
	})
	startedAt := time.Now()
	run, err := deployer.ApplyRelease(db, cfg, svcID, relID, triggerSource,
		func(line string) {
			safeEmit(emit, "service_line", map[string]any{
				"service_id": svcID,
				"line":       line,
			})
		})
	dur := int(time.Since(startedAt).Seconds())

	sItem := model.AppReleaseSummaryItem{ServiceID: svcID}
	if run != nil {
		sItem.RunID = &run.ID
		sItem.Status = run.Status
	}
	ok := false
	switch {
	case err != nil:
		sItem.Status = "failed"
		sItem.Error = err.Error()
	case run != nil && run.Status == model.DeployRunStatusSuccess:
		sItem.Status = "success"
		ok = true
	default:
		if sItem.Status == "" {
			sItem.Status = "failed"
		}
	}

	safeEmit(emit, "service_done", map[string]any{
		"service_id":   svcID,
		"run_id":       sItem.RunID,
		"status":       sItem.Status,
		"duration_sec": dur,
		"error":        sItem.Error,
	})
	return sItem, ok
}

// finalizeStatus 写回终态 + applied_at + last_summary，并对失败做日志。
// 用于 Apply 与 Rollback 的尾段；defer-recover 也通过它把 CAS 锁解开。
func finalizeStatus(
	db *gorm.DB, setID uint, status string,
	summary []model.AppReleaseSummaryItem,
) {
	now := time.Now()
	updates := map[string]any{
		"status":     status,
		"applied_at": &now,
	}
	// summary 为 nil 时（panic 路径）保留旧值不动，避免覆盖
	if summary != nil {
		updates["last_summary"] = encodeSummary(summary)
	}
	if err := db.Model(&model.AppReleaseSet{}).Where("id = ?", setID).
		Updates(updates).Error; err != nil {
		log.Printf("[apprelease] finalizeStatus set=%d status=%s: %v", setID, status, err)
	}
}

// ────────────────────────────── Apply ───────────────────────────────────────

// Apply 同步执行一个 AppReleaseSet，按 Items 顺序串行部署每个 Service。
// 单 Service 失败不中断后续，最终按成功/失败比例决算 status。
//
// emit 可为 nil；非 nil 时按以下事件名向上游推送：
//
//	set_started   {set_id,total,items:[{service_id,release_id}]}
//	service_started {service_id,release_id,idx,total}
//	service_line  {service_id,line}
//	service_done  {service_id,run_id,status,duration_sec,error?}
//	set_done      {status,summary}
//
// 并发控制：Status=applying 的 set 再次 Apply 返回 ErrAlreadyApplying。
// 故障安全：deployer panic 由 defer 接住并把状态写回 failed，避免 CAS 永久卡死。
var ErrAlreadyApplying = errors.New("app release set is currently applying")

// ErrNothingToRollback 表示 set 没有可回滚的目标（例如 last_summary 为空，
// 或所有 success 项都找不到历史 Release）。
var ErrNothingToRollback = errors.New("nothing to roll back")

func Apply(db *gorm.DB, cfg *config.Config, setID uint, triggerSource string, emit Emit) error {
	if err := casToApplying(db, setID); err != nil {
		return err
	}

	// finalize 兜底：无论正常返回还是 panic 都把 CAS 锁解开。
	// 终态在主流程结束前写入 status 局部变量；panic 路径写 failed。
	status := model.AppReleaseSetStatusFailed
	var summary []model.AppReleaseSummaryItem
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[apprelease] Apply panic set=%d: %v", setID, r)
			finalizeStatus(db, setID, model.AppReleaseSetStatusFailed, nil)
			panic(r) // re-panic 让上游 recover 记录堆栈
		}
		finalizeStatus(db, setID, status, summary)
	}()

	var set model.AppReleaseSet
	if err := db.First(&set, setID).Error; err != nil {
		return err
	}
	items := parseItems(set.Items)
	total := len(items)

	safeEmit(emit, "set_started", map[string]any{
		"set_id": set.ID,
		"total":  total,
		"items":  items,
	})

	summary = make([]model.AppReleaseSummaryItem, 0, total)
	successCnt, failCnt := 0, 0
	for idx, it := range items {
		sItem, ok := runOne(db, cfg, it.ServiceID, it.ReleaseID, idx, total, triggerSource, emit)
		if ok {
			successCnt++
		} else {
			failCnt++
		}
		summary = append(summary, sItem)
	}

	status = decideStatus(successCnt, failCnt, total)
	safeEmit(emit, "set_done", map[string]any{
		"status":  status,
		"summary": summary,
	})
	return nil
}

func casToApplying(db *gorm.DB, setID uint) error {
	res := db.Model(&model.AppReleaseSet{}).
		Where("id = ? AND status <> ?", setID, model.AppReleaseSetStatusApplying).
		Update("status", model.AppReleaseSetStatusApplying)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrAlreadyApplying
	}
	return nil
}

func decideStatus(success, fail, total int) string {
	switch {
	case total == 0:
		return model.AppReleaseSetStatusFailed
	case fail == 0:
		return model.AppReleaseSetStatusSuccess
	case success == 0:
		return model.AppReleaseSetStatusFailed
	default:
		return model.AppReleaseSetStatusPartial
	}
}

// ──────────────────────────── Rollback ──────────────────────────────────────

// Rollback 对上次 Apply 成功的 Service 反向 Apply 到此前历史 Release（即在
// Service 的 Release 时间线上，id 严格小于本次已应用 Release 的最近一条
// active/archived）。找不到历史的 Service 在 summary 中标 skipped。
// Rollback 结束后 Set 状态置 rolled_back（targets 为空时返回 ErrNothingToRollback）。
//
// 与 Apply 一样走 CAS 防重 + defer 兜底 panic。
func Rollback(db *gorm.DB, cfg *config.Config, setID uint, triggerSource string, emit Emit) error {
	var set model.AppReleaseSet
	if err := db.First(&set, setID).Error; err != nil {
		return err
	}
	if set.LastSummary == "" {
		return errors.New("set has not been applied yet; nothing to roll back")
	}
	var last []model.AppReleaseSummaryItem
	_ = json.Unmarshal([]byte(set.LastSummary), &last)
	items := parseItems(set.Items)
	itemByService := make(map[uint]uint, len(items))
	for _, it := range items {
		itemByService[it.ServiceID] = it.ReleaseID
	}

	type rbTarget struct {
		svcID uint
		relID uint
	}
	var targets []rbTarget
	var skipped []model.AppReleaseSummaryItem
	for _, s := range last {
		if s.Status != "success" {
			continue
		}
		applied := itemByService[s.ServiceID]
		prev := findPrevRelease(db, s.ServiceID, applied)
		if prev == 0 {
			skipped = append(skipped, model.AppReleaseSummaryItem{
				ServiceID: s.ServiceID,
				Status:    "skipped",
				Error:     "no previous release found",
			})
			continue
		}
		targets = append(targets, rbTarget{svcID: s.ServiceID, relID: prev})
	}

	if len(targets) == 0 {
		return fmt.Errorf("%w: no service has a prior release to roll back to", ErrNothingToRollback)
	}

	if err := casToApplying(db, setID); err != nil {
		return err
	}
	status := model.AppReleaseSetStatusFailed
	summary := append([]model.AppReleaseSummaryItem(nil), skipped...)
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[apprelease] Rollback panic set=%d: %v", setID, r)
			finalizeStatus(db, setID, model.AppReleaseSetStatusFailed, nil)
			panic(r)
		}
		finalizeStatus(db, setID, status, summary)
	}()

	total := len(targets)
	safeEmit(emit, "set_started", map[string]any{
		"set_id": set.ID,
		"total":  total,
	})
	// 先把 skipped 推给前端，保证 UI 能看到被跳过的 service
	for _, sk := range skipped {
		safeEmit(emit, "service_done", map[string]any{
			"service_id": sk.ServiceID,
			"status":     sk.Status,
			"error":      sk.Error,
		})
	}

	successCnt, failCnt := 0, 0
	for idx, t := range targets {
		sItem, ok := runOne(db, cfg, t.svcID, t.relID, idx, total, triggerSource, emit)
		if ok {
			successCnt++
		} else {
			failCnt++
		}
		summary = append(summary, sItem)
	}

	status = model.AppReleaseSetStatusRolledBack
	safeEmit(emit, "set_done", map[string]any{
		"status":  status,
		"summary": summary,
		"success": successCnt,
		"failed":  failCnt,
	})
	return nil
}

// findPrevRelease 返回 serviceID 下、id 严格小于 excludeID 的最近一条
// active/archived Release ID。找不到返回 0。
//
// 注意：使用 id < excludeID 而不是 id <> excludeID，避免在 Service 上
// 存在"晚于当前 release 但已 archived"的版本时 Rollback 反而向前跳。
func findPrevRelease(db *gorm.DB, serviceID, excludeID uint) uint {
	var r model.Release
	err := db.Where("service_id = ? AND id < ? AND status IN ?",
		serviceID, excludeID,
		[]string{model.ReleaseStatusActive, model.ReleaseStatusArchived}).
		Order("id desc").First(&r).Error
	if err != nil {
		return 0
	}
	return r.ID
}

// ─────────────────────────── Internal helpers ───────────────────────────────

func safeEmit(emit Emit, name string, data any) {
	if emit == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[apprelease] safeEmit panic event=%s: %v", name, r)
		}
	}()
	emit(name, data)
}

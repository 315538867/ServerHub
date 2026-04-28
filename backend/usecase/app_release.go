// Package usecase: app_release.go 实现 AppReleaseSet（App 级发布集）业务层。
//
// 一个 AppReleaseSet 把 App 下多个 Service 的 Release 选择快照成原子单元。
// Apply 按 Items 顺序串行调用 ApplyRelease，期间通过 emit 流式回调向
// HTTP SSE 推送事件。Rollback 针对上一次 Apply 成功的 Service 反向 Apply 到
// 先前历史 Release。
//
// 原先在 api/apprelease/service.go，R6-E 迁入 usecase。
//
// TODO R7: 切 ports interface，移除 db *gorm.DB 入参。
package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/repo"
	"gorm.io/gorm"
)

// AppReleaseEmit 是统一流式事件回调签名。
type AppReleaseEmit func(name string, data any)

// ErrAppReleaseAlreadyApplying 表示 set 正在 applying。
var ErrAppReleaseAlreadyApplying = repo.ErrAlreadyApplying

// ErrNothingToRollback 表示 set 没有可回滚的目标。
var ErrNothingToRollback = errors.New("nothing to roll back")

// ── JSON helpers ────────────────────────────────────────────────────────────

func parseAppReleaseItems(raw string) []model.AppReleaseSetItem {
	if raw == "" {
		return nil
	}
	var out []model.AppReleaseSetItem
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

func encodeAppReleaseItems(items []model.AppReleaseSetItem) string {
	b, _ := json.Marshal(items)
	return string(b)
}

func encodeAppReleaseSummary(items []model.AppReleaseSummaryItem) string {
	b, _ := json.Marshal(items)
	return string(b)
}

// ── autoLabel ───────────────────────────────────────────────────────────────

func appReleaseAutoLabel(ctx context.Context, db *gorm.DB, appID uint) string {
	today := time.Now().Format("2006-01-02")
	n, _ := repo.CountAppReleaseSetLabelLike(ctx, db, appID, today+"-%")
	return today + "-" + strconv.FormatInt(n+1, 10)
}

// ── CreateFromCurrent ───────────────────────────────────────────────────────

// CreateAppReleaseSetFromCurrent 扫描 App 下所有已绑定 CurrentReleaseID 的 Service，
// 生成一份 Release 组合快照。未绑定 Release 的 Service 被跳过。
func CreateAppReleaseSetFromCurrent(ctx context.Context, db *gorm.DB, appID uint, label, note, createdBy string) (*model.AppReleaseSet, error) {
	svcs, err := repo.ListServicesByApplicationID(ctx, db, appID)
	if err != nil {
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
		label = appReleaseAutoLabel(ctx, db, appID)
	}
	set := &model.AppReleaseSet{
		ApplicationID: appID,
		Label:         label,
		Items:         encodeAppReleaseItems(items),
		Note:          note,
		CreatedBy:     createdBy,
		Status:        model.AppReleaseSetStatusDraft,
	}
	if err := repo.CreateAppReleaseSet(ctx, db, set); err != nil {
		return nil, err
	}
	return set, nil
}

// ── Apply / Rollback 共享执行单元 ───────────────────────────────────────────

func appReleaseRunOne(
	db *gorm.DB, cfg *config.Config,
	svcID, relID uint, idx, total int,
	triggerSource string, emit AppReleaseEmit,
) (model.AppReleaseSummaryItem, bool) {
	safeAppReleaseEmit(emit, "service_started", map[string]any{
		"service_id": svcID,
		"release_id": relID,
		"idx":        idx,
		"total":      total,
	})
	startedAt := time.Now()
	run, err := ApplyRelease(db, cfg, svcID, relID, triggerSource,
		func(line string) {
			safeAppReleaseEmit(emit, "service_line", map[string]any{
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

	safeAppReleaseEmit(emit, "service_done", map[string]any{
		"service_id":   svcID,
		"run_id":       sItem.RunID,
		"status":       sItem.Status,
		"duration_sec": dur,
		"error":        sItem.Error,
	})
	return sItem, ok
}

func appReleaseFinalizeStatus(
	ctx context.Context, db *gorm.DB, setID uint, status string,
	summary []model.AppReleaseSummaryItem,
) {
	now := time.Now()
	updates := map[string]any{
		"status":     status,
		"applied_at": &now,
	}
	if summary != nil {
		updates["last_summary"] = encodeAppReleaseSummary(summary)
	}
	if err := repo.UpdateAppReleaseSetFields(ctx, db, setID, updates); err != nil {
		log.Printf("[apprelease] finalizeStatus set=%d status=%s: %v", setID, status, err)
	}
}

// ── Apply ───────────────────────────────────────────────────────────────────

// AppReleaseApply 同步执行一个 AppReleaseSet，按 Items 顺序串行部署。
func AppReleaseApply(ctx context.Context, db *gorm.DB, cfg *config.Config, setID uint, triggerSource string, emit AppReleaseEmit) error {
	if err := repo.CASAppReleaseSetToApplying(ctx, db, setID); err != nil {
		return err
	}

	status := model.AppReleaseSetStatusFailed
	var summary []model.AppReleaseSummaryItem
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[apprelease] Apply panic set=%d: %v", setID, r)
			appReleaseFinalizeStatus(ctx, db, setID, model.AppReleaseSetStatusFailed, nil)
			panic(r)
		}
		appReleaseFinalizeStatus(ctx, db, setID, status, summary)
	}()

	set, err := repo.GetAppReleaseSetByID(ctx, db, setID)
	if err != nil {
		return err
	}
	items := parseAppReleaseItems(set.Items)
	total := len(items)

	safeAppReleaseEmit(emit, "set_started", map[string]any{
		"set_id": set.ID,
		"total":  total,
		"items":  items,
	})

	summary = make([]model.AppReleaseSummaryItem, 0, total)
	successCnt, failCnt := 0, 0
	for idx, it := range items {
		sItem, ok := appReleaseRunOne(db, cfg, it.ServiceID, it.ReleaseID, idx, total, triggerSource, emit)
		if ok {
			successCnt++
		} else {
			failCnt++
		}
		summary = append(summary, sItem)
	}

	status = appReleaseDecideStatus(successCnt, failCnt, total)
	safeAppReleaseEmit(emit, "set_done", map[string]any{
		"status":  status,
		"summary": summary,
	})
	return nil
}

func appReleaseDecideStatus(success, fail, total int) string {
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

// ── Rollback ────────────────────────────────────────────────────────────────

// AppReleaseRollback 对上次 Apply 成功的 Service 反向 Apply 到此前历史 Release。
func AppReleaseRollback(ctx context.Context, db *gorm.DB, cfg *config.Config, setID uint, triggerSource string, emit AppReleaseEmit) error {
	set, err := repo.GetAppReleaseSetByID(ctx, db, setID)
	if err != nil {
		return err
	}
	if set.LastSummary == "" {
		return errors.New("set has not been applied yet; nothing to roll back")
	}
	var last []model.AppReleaseSummaryItem
	_ = json.Unmarshal([]byte(set.LastSummary), &last)
	items := parseAppReleaseItems(set.Items)
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
		prev := repo.FindPrevRelease(ctx, db, s.ServiceID, applied)
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

	if err := repo.CASAppReleaseSetToApplying(ctx, db, setID); err != nil {
		return err
	}
	status := model.AppReleaseSetStatusFailed
	summary := append([]model.AppReleaseSummaryItem(nil), skipped...)
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[apprelease] Rollback panic set=%d: %v", setID, r)
			appReleaseFinalizeStatus(ctx, db, setID, model.AppReleaseSetStatusFailed, nil)
			panic(r)
		}
		appReleaseFinalizeStatus(ctx, db, setID, status, summary)
	}()

	total := len(targets)
	safeAppReleaseEmit(emit, "set_started", map[string]any{
		"set_id": set.ID,
		"total":  total,
	})
	for _, sk := range skipped {
		safeAppReleaseEmit(emit, "service_done", map[string]any{
			"service_id": sk.ServiceID,
			"status":     sk.Status,
			"error":      sk.Error,
		})
	}

	successCnt, failCnt := 0, 0
	for idx, t := range targets {
		sItem, ok := appReleaseRunOne(db, cfg, t.svcID, t.relID, idx, total, triggerSource, emit)
		if ok {
			successCnt++
		} else {
			failCnt++
		}
		summary = append(summary, sItem)
	}

	status = model.AppReleaseSetStatusRolledBack
	safeAppReleaseEmit(emit, "set_done", map[string]any{
		"status":  status,
		"summary": summary,
		"success": successCnt,
		"failed":  failCnt,
	})
	return nil
}

// ── helpers ─────────────────────────────────────────────────────────────────

func safeAppReleaseEmit(emit AppReleaseEmit, name string, data any) {
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

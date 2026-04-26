package svcstatus

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/serverhub/serverhub/model"
)

// LatestByService 是 P-G 派生 LastStatus 的核心 helper。本文件钉死 4 条边界:
//
//  1. 空 IDs 输入直接返回空 map(短路,不打 DB)。
//  2. ID 没有任何 DeployRun → 在 map 里缺席(caller 自行兜底,本包不替你写
//     "success")。
//  3. 多条 DeployRun 时返回最近 started_at 那条 —— 不是 ID 最大、不是
//     CreatedAt 最大。
//  4. 同一调用可批量覆盖多个 ServiceID,各 ID 互不串味。
//
// 故意不测"两条 DeployRun started_at 完全相同"的 race 场景:ServerHub 单进程
// 串行写入,微秒级冲突理论不存在;若日后 reconciler 拆 HA 副本,再补这条用例。
func setupDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.DeployRun{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func mustCreateRun(t *testing.T, db *gorm.DB, svcID, relID uint, status string, startedAt time.Time) {
	t.Helper()
	r := model.DeployRun{
		ServiceID: svcID,
		ReleaseID: relID,
		Status:    status,
		StartedAt: startedAt,
	}
	if err := db.Create(&r).Error; err != nil {
		t.Fatalf("create run: %v", err)
	}
}

func TestLatestByService_EmptyIDs(t *testing.T) {
	db := setupDB(t)
	got := LatestByService(db, nil)
	if len(got) != 0 {
		t.Fatalf("nil IDs: want empty, got %v", got)
	}
	got = LatestByService(db, []uint{})
	if len(got) != 0 {
		t.Fatalf("empty IDs: want empty, got %v", got)
	}
}

func TestLatestByService_NoRun_AbsentFromMap(t *testing.T) {
	db := setupDB(t)
	// svc 7 有 run, svc 8 没有
	mustCreateRun(t, db, 7, 1, model.DeployRunStatusSuccess, time.Now())

	got := LatestByService(db, []uint{7, 8})
	if _, ok := got[7]; !ok {
		t.Fatalf("svc 7: expected entry, got none")
	}
	if _, ok := got[8]; ok {
		t.Fatalf("svc 8: expected absent (no DeployRun), got entry")
	}
}

func TestLatestByService_PicksMostRecentByStartedAt(t *testing.T) {
	db := setupDB(t)
	t0 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	// 故意倒序插入(老的后插),验证不是按 ID/CreatedAt 排序
	mustCreateRun(t, db, 42, 1, model.DeployRunStatusFailed, t0.Add(2*time.Hour))    // 最近
	mustCreateRun(t, db, 42, 1, model.DeployRunStatusSuccess, t0.Add(1*time.Hour))   // 中间
	mustCreateRun(t, db, 42, 1, model.DeployRunStatusRolledBack, t0)                 // 最早
	mustCreateRun(t, db, 42, 1, model.DeployRunStatusRunning, t0.Add(30*time.Minute)) // 早

	got := LatestByService(db, []uint{42})
	e, ok := got[42]
	if !ok {
		t.Fatalf("svc 42: expected entry")
	}
	if e.Status != model.DeployRunStatusFailed {
		t.Fatalf("svc 42: want status=%q (latest by started_at), got %q",
			model.DeployRunStatusFailed, e.Status)
	}
	if !e.StartedAt.Equal(t0.Add(2 * time.Hour)) {
		t.Fatalf("svc 42: want started_at=%v, got %v", t0.Add(2*time.Hour), e.StartedAt)
	}
}

func TestLatestByService_BatchIsolation(t *testing.T) {
	db := setupDB(t)
	t0 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	// 三个服务,各有自己的最近 run;验证 join 不串味
	mustCreateRun(t, db, 1, 100, model.DeployRunStatusSuccess, t0)
	mustCreateRun(t, db, 1, 100, model.DeployRunStatusFailed, t0.Add(time.Hour)) // svc 1 最新
	mustCreateRun(t, db, 2, 200, model.DeployRunStatusRunning, t0.Add(2*time.Hour))
	mustCreateRun(t, db, 3, 300, model.DeployRunStatusRolledBack, t0.Add(-time.Hour))

	got := LatestByService(db, []uint{1, 2, 3})
	if len(got) != 3 {
		t.Fatalf("want 3 entries, got %d: %v", len(got), got)
	}
	if got[1].Status != model.DeployRunStatusFailed {
		t.Fatalf("svc 1: want %q, got %q", model.DeployRunStatusFailed, got[1].Status)
	}
	if got[2].Status != model.DeployRunStatusRunning {
		t.Fatalf("svc 2: want %q, got %q", model.DeployRunStatusRunning, got[2].Status)
	}
	if got[3].Status != model.DeployRunStatusRolledBack {
		t.Fatalf("svc 3: want %q, got %q", model.DeployRunStatusRolledBack, got[3].Status)
	}
}

func TestLatestByService_QueriesOnlyRequestedIDs(t *testing.T) {
	db := setupDB(t)
	t0 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	mustCreateRun(t, db, 1, 1, model.DeployRunStatusSuccess, t0)
	mustCreateRun(t, db, 99, 1, model.DeployRunStatusFailed, t0.Add(time.Hour))

	// 只问 svc 1,99 不应出现在结果里
	got := LatestByService(db, []uint{1})
	if _, ok := got[99]; ok {
		t.Fatalf("svc 99 leaked into result: %v", got)
	}
	if _, ok := got[1]; !ok {
		t.Fatalf("svc 1 missing")
	}
}

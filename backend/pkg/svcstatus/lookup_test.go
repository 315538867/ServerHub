package svcstatus

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/serverhub/serverhub/model"
)

// LatestByService 是派生 Service 摘要的核心 helper。本文件钉死的边界分两段:
//
// Status/StartedAt 段(P-G 起):
//  1. 空 IDs 输入直接返回空 map(短路,不打 DB)。
//  2. ID 没有任何 DeployRun → 在 map 里缺席(caller 自行兜底,本包不替你写
//     "success")。
//  3. 多条 DeployRun 时返回最近 started_at 那条 —— 不是 ID 最大、不是
//     CreatedAt 最大。
//  4. 同一调用可批量覆盖多个 ServiceID,各 ID 互不串味。
//
// Image 段(P-I 起):
//  5. Service.CurrentReleaseID=NULL → Image 缺省 ""。
//  6. CurrentReleaseID 指向的 Release.StartSpec 没有 image key
//     (compose/native/static) → Image 缺省 ""。
//  7. docker StartSpec 有 image key → 提取该值。
//  8. Service 既有 DeployRun 又有 CurrentRelease → Entry 同时持有 Status 和 Image,
//     两段查询互不影响。
//  9. Service 仅有 CurrentRelease 没有 DeployRun → Entry 出现在 map 里,Status="" 而
//     Image 有值;caller 据此分别兜底。
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
	if err := db.AutoMigrate(&model.DeployRun{}, &model.Service{}, &model.Release{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// mustCreateService 建一条 Service,可选附加 CurrentReleaseID。
func mustCreateService(t *testing.T, db *gorm.DB, id uint, currentReleaseID *uint) {
	t.Helper()
	s := model.Service{
		ID:               id,
		Name:             "svc",
		ServerID:         1,
		CurrentReleaseID: currentReleaseID,
	}
	if err := db.Create(&s).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}
}

// mustCreateRelease 建一条 Release,StartSpec 直接写 JSON 字面量。
func mustCreateRelease(t *testing.T, db *gorm.DB, id, svcID uint, startSpec string) {
	t.Helper()
	r := model.Release{
		ID:         id,
		ServiceID:  svcID,
		ArtifactID: 1,
		StartSpec:  startSpec,
	}
	if err := db.Create(&r).Error; err != nil {
		t.Fatalf("create release: %v", err)
	}
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

// TestLatestByService_ImageFromCurrentRelease 钉死 docker StartSpec 的 image
// key 能被正确抠出。
func TestLatestByService_ImageFromCurrentRelease(t *testing.T) {
	db := setupDB(t)
	relID := uint(11)
	mustCreateRelease(t, db, relID, 5, `{"image":"nginx:1.27","cmd":"","args":[]}`)
	mustCreateService(t, db, 5, &relID)

	got := LatestByService(db, []uint{5})
	e, ok := got[5]
	if !ok {
		t.Fatalf("svc 5: expected entry")
	}
	if e.Image != "nginx:1.27" {
		t.Fatalf("svc 5: want image=nginx:1.27, got %q", e.Image)
	}
}

// TestLatestByService_ImageAbsent_NoCurrentRelease 钉死 CurrentReleaseID=NULL
// 时 Image 不会被填,且(没有 DeployRun 时)Service 整体不出现在 map 里。
func TestLatestByService_ImageAbsent_NoCurrentRelease(t *testing.T) {
	db := setupDB(t)
	mustCreateService(t, db, 6, nil)

	got := LatestByService(db, []uint{6})
	if _, ok := got[6]; ok {
		t.Fatalf("svc 6: expected absent (no run, no release), got %v", got[6])
	}
}

// TestLatestByService_ImageAbsent_NonDockerStartSpec 钉死 compose/native StartSpec
// 因为没有 image key,Image 留空;同时(无 DeployRun)Service 不进 map。
func TestLatestByService_ImageAbsent_NonDockerStartSpec(t *testing.T) {
	db := setupDB(t)
	relID := uint(21)
	// compose 形态:有 file_name,无 image
	mustCreateRelease(t, db, relID, 7, `{"file_name":"docker-compose.yml","compose_profile":""}`)
	mustCreateService(t, db, 7, &relID)

	got := LatestByService(db, []uint{7})
	if e, ok := got[7]; ok && e.Image != "" {
		t.Fatalf("svc 7: want image=\"\", got %q", e.Image)
	}
}

// TestLatestByService_StatusAndImageIndependent 钉死 Service 同时具备
// DeployRun 与 CurrentRelease 时,两段派生独立合入同一 Entry。
func TestLatestByService_StatusAndImageIndependent(t *testing.T) {
	db := setupDB(t)
	t0 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	relID := uint(31)
	mustCreateRelease(t, db, relID, 8, `{"image":"redis:7"}`)
	mustCreateService(t, db, 8, &relID)
	mustCreateRun(t, db, 8, relID, model.DeployRunStatusSuccess, t0)

	got := LatestByService(db, []uint{8})
	e, ok := got[8]
	if !ok {
		t.Fatalf("svc 8: expected entry")
	}
	if e.Status != model.DeployRunStatusSuccess {
		t.Fatalf("svc 8: want status=success, got %q", e.Status)
	}
	if e.Image != "redis:7" {
		t.Fatalf("svc 8: want image=redis:7, got %q", e.Image)
	}
	if !e.StartedAt.Equal(t0) {
		t.Fatalf("svc 8: want started_at=%v, got %v", t0, e.StartedAt)
	}
}

// TestLatestByService_OnlyImage_NoRun 钉死"Service 有 CurrentRelease 但从未
// 部署"的语义:Entry 出现在 map 里,Image 有值,Status 留空让 caller 兜底。
func TestLatestByService_OnlyImage_NoRun(t *testing.T) {
	db := setupDB(t)
	relID := uint(41)
	mustCreateRelease(t, db, relID, 9, `{"image":"alpine:3.20"}`)
	mustCreateService(t, db, 9, &relID)

	got := LatestByService(db, []uint{9})
	e, ok := got[9]
	if !ok {
		t.Fatalf("svc 9: expected entry (has image even without run)")
	}
	if e.Status != "" {
		t.Fatalf("svc 9: want status=\"\" (no run), got %q", e.Status)
	}
	if e.Image != "alpine:3.20" {
		t.Fatalf("svc 9: want image=alpine:3.20, got %q", e.Image)
	}
	if !e.StartedAt.IsZero() {
		t.Fatalf("svc 9: want started_at zero, got %v", e.StartedAt)
	}
}

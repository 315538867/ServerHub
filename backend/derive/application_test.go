// derive/application_test.go 钉死 AppStatus 派生算法的边界:
//
//  1. 空 IDs 短路返回空 map
//  2. App 没有任何 Service → unknown
//  3. 任一 Service DeployRun.Status=failed → error
//  4. 任一 Service DeployRun.Status=syncing(无 failed) → syncing
//  5. 全部 Service DeployRun.Status=success → running
//  6. 无 DeployRun 视作 success(takeover 接管约定)
//  7. 多 app 输入互不串味,优先级 error > syncing > running > unknown
package derive

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/serverhub/serverhub/model"
)

func setupAppDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Service{}, &model.DeployRun{}, &model.Release{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func mustCreateSvc(t *testing.T, db *gorm.DB, id, appID uint) {
	t.Helper()
	a := appID
	s := model.Service{ID: id, Name: "svc", ServerID: 1, ApplicationID: &a}
	if err := db.Create(&s).Error; err != nil {
		t.Fatalf("create svc: %v", err)
	}
}

func mustCreateRun(t *testing.T, db *gorm.DB, svcID uint, status string, startedAt time.Time) {
	t.Helper()
	r := model.DeployRun{ServiceID: svcID, Status: status, StartedAt: startedAt}
	if err := db.Create(&r).Error; err != nil {
		t.Fatalf("create run: %v", err)
	}
}

func TestAppStatus_EmptyIDs(t *testing.T) {
	db := setupAppDB(t)
	got := AppStatus(db, nil)
	if len(got) != 0 {
		t.Fatalf("expected empty, got %d", len(got))
	}
}

func TestAppStatus_NoService_Unknown(t *testing.T) {
	db := setupAppDB(t)
	got := AppStatus(db, []uint{42})
	if got[42].Result != AppStatusUnknown {
		t.Fatalf("want unknown, got %q", got[42].Result)
	}
}

func TestAppStatus_AnyFailed_IsError(t *testing.T) {
	db := setupAppDB(t)
	now := time.Now()
	mustCreateSvc(t, db, 1, 100)
	mustCreateSvc(t, db, 2, 100)
	mustCreateRun(t, db, 1, "failed", now)
	mustCreateRun(t, db, 2, "success", now)
	got := AppStatus(db, []uint{100})
	if got[100].Result != AppStatusError {
		t.Fatalf("want error, got %q", got[100].Result)
	}
}

func TestAppStatus_AnySyncing_IsSyncing(t *testing.T) {
	db := setupAppDB(t)
	now := time.Now()
	mustCreateSvc(t, db, 1, 100)
	mustCreateSvc(t, db, 2, 100)
	mustCreateRun(t, db, 1, "syncing", now)
	mustCreateRun(t, db, 2, "success", now)
	got := AppStatus(db, []uint{100})
	if got[100].Result != AppStatusSyncing {
		t.Fatalf("want syncing, got %q", got[100].Result)
	}
}

func TestAppStatus_AllSuccess_IsRunning(t *testing.T) {
	db := setupAppDB(t)
	now := time.Now()
	mustCreateSvc(t, db, 1, 100)
	mustCreateSvc(t, db, 2, 100)
	mustCreateRun(t, db, 1, "success", now)
	mustCreateRun(t, db, 2, "success", now)
	got := AppStatus(db, []uint{100})
	if got[100].Result != AppStatusRunning {
		t.Fatalf("want running, got %q", got[100].Result)
	}
}

func TestAppStatus_NoRun_TreatedAsSuccess(t *testing.T) {
	db := setupAppDB(t)
	mustCreateSvc(t, db, 1, 100)
	mustCreateSvc(t, db, 2, 100)
	// 无任何 DeployRun → 全部视作 success → running
	got := AppStatus(db, []uint{100})
	if got[100].Result != AppStatusRunning {
		t.Fatalf("want running (no-run-as-success), got %q", got[100].Result)
	}
}

func TestAppStatus_MultiApp_NoCrossTalk(t *testing.T) {
	db := setupAppDB(t)
	now := time.Now()
	// app 100: 1 svc, failed → error
	mustCreateSvc(t, db, 1, 100)
	mustCreateRun(t, db, 1, "failed", now)
	// app 200: 1 svc, syncing → syncing
	mustCreateSvc(t, db, 2, 200)
	mustCreateRun(t, db, 2, "syncing", now)
	// app 300: no svc → unknown
	got := AppStatus(db, []uint{100, 200, 300})
	if got[100].Result != AppStatusError {
		t.Fatalf("app 100: want error, got %q", got[100].Result)
	}
	if got[200].Result != AppStatusSyncing {
		t.Fatalf("app 200: want syncing, got %q", got[200].Result)
	}
	if got[300].Result != AppStatusUnknown {
		t.Fatalf("app 300: want unknown, got %q", got[300].Result)
	}
}

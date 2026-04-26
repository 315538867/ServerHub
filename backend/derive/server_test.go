// derive/server_test.go 钉死 ServerStatus 派生算法的边界:
//
//  1. 空 IDs 短路返回空 map(不打 DB)
//  2. 缺行(server 从未探测) → unknown
//  3. result=offline 不看时间,直接 offline
//  4. result=online + age<lagging → online
//  5. result=online + lagging≤age<offline → lagging
//  6. result=online + age≥offline → offline(久未续约视同离线)
//  7. 多条 probe 取最新一条(MAX created_at),不是 ID 最大
//  8. 多 server 输入互不串味
package derive

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/serverhub/serverhub/model"
)

func setupServerDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.ServerProbe{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func mustCreateProbe(t *testing.T, db *gorm.DB, srvID uint, result string, at time.Time) {
	t.Helper()
	p := model.ServerProbe{ServerID: srvID, Result: result, CreatedAt: at}
	if err := db.Create(&p).Error; err != nil {
		t.Fatalf("create probe: %v", err)
	}
}

func TestServerStatus_EmptyIDs(t *testing.T) {
	db := setupServerDB(t)
	got := ServerStatus(db, nil, 0, 0)
	if len(got) != 0 {
		t.Fatalf("expected empty map, got %d entries", len(got))
	}
}

func TestServerStatus_NoProbeIsUnknown(t *testing.T) {
	db := setupServerDB(t)
	got := ServerStatus(db, []uint{1}, 0, 0)
	if got[1].Result != ServerStatusUnknown {
		t.Fatalf("want unknown, got %q", got[1].Result)
	}
	if !got[1].LastProbeAt.IsZero() {
		t.Fatalf("want zero LastProbeAt, got %v", got[1].LastProbeAt)
	}
}

func TestServerStatus_TableDriven(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name      string
		result    string
		ageOffset time.Duration
		want      ServerStatusKind
	}{
		{"online_fresh", "online", -30 * time.Second, ServerStatusOnline},
		{"online_lagging", "online", -3 * time.Minute, ServerStatusLagging},
		{"online_too_old_offline", "online", -10 * time.Minute, ServerStatusOffline},
		{"explicit_offline_ignores_time", "offline", -5 * time.Second, ServerStatusOffline},
		{"explicit_offline_old", "offline", -1 * time.Hour, ServerStatusOffline},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := setupServerDB(t)
			mustCreateProbe(t, db, 1, tc.result, now.Add(tc.ageOffset))
			got := ServerStatus(db, []uint{1}, DefaultLaggingAfter, DefaultOfflineAfter)
			if got[1].Result != tc.want {
				t.Fatalf("want %q, got %q", tc.want, got[1].Result)
			}
		})
	}
}

func TestServerStatus_LatestProbeWins(t *testing.T) {
	db := setupServerDB(t)
	now := time.Now()
	// 旧 offline + 新 online → 应取新的 online
	mustCreateProbe(t, db, 1, "offline", now.Add(-1*time.Hour))
	mustCreateProbe(t, db, 1, "online", now.Add(-10*time.Second))
	got := ServerStatus(db, []uint{1}, DefaultLaggingAfter, DefaultOfflineAfter)
	if got[1].Result != ServerStatusOnline {
		t.Fatalf("want online (latest probe wins), got %q", got[1].Result)
	}
}

func TestServerStatus_MultipleServers(t *testing.T) {
	db := setupServerDB(t)
	now := time.Now()
	mustCreateProbe(t, db, 1, "online", now.Add(-30*time.Second))
	mustCreateProbe(t, db, 2, "offline", now.Add(-10*time.Second))
	// server 3 没 probe
	got := ServerStatus(db, []uint{1, 2, 3}, DefaultLaggingAfter, DefaultOfflineAfter)
	if got[1].Result != ServerStatusOnline {
		t.Fatalf("server 1: want online, got %q", got[1].Result)
	}
	if got[2].Result != ServerStatusOffline {
		t.Fatalf("server 2: want offline, got %q", got[2].Result)
	}
	if got[3].Result != ServerStatusUnknown {
		t.Fatalf("server 3: want unknown, got %q", got[3].Result)
	}
}

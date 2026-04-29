package usecase

import (
	"testing"

	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/repo"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newRetentionDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Service{}, &model.Release{}, &model.Artifact{},
		&model.EnvVarSet{}, &model.ConfigFileSet{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	return db
}

// INV-7: PruneReleases 保留近期 Release + active Release，淘汰超限旧版本

func TestPruneReleases_UnderLimitKeepsAll(t *testing.T) {
	db := newRetentionDB(t)
	ctx := db.Statement.Context

	// 创建 service + 3 个 release（<MaxReleasesPerService=10）
	svcM := model.Service{Name: "svc", ServerID: 1, Type: "docker"}
	db.Create(&svcM)

	for i := 0; i < 3; i++ {
		artM := model.Artifact{ServiceID: svcM.ID, Provider: "http", Ref: "https://x.com/x.tar"}
		db.Create(&artM)
		relM := model.Release{ServiceID: svcM.ID, ArtifactID: artM.ID, Label: "r"}
		db.Create(&relM)
	}

	PruneReleases(db, svcM.ID, MaxReleasesPerService)

	count, err := repo.CountReleasesByServiceID(ctx, db, svcM.ID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("expected 3 releases kept, got %d", count)
	}
}

func TestPruneReleases_ExcessRemovesOldest(t *testing.T) {
	db := newRetentionDB(t)
	ctx := db.Statement.Context

	svcM := model.Service{Name: "svc", ServerID: 1, Type: "native"}
	db.Create(&svcM)

	keep := 3
	// 创建 keep+2 个 release
	for i := 0; i < keep+2; i++ {
		artM := model.Artifact{ServiceID: svcM.ID, Provider: "http", Ref: "https://x.com/x.tar"}
		db.Create(&artM)
		relM := model.Release{ServiceID: svcM.ID, ArtifactID: artM.ID, Label: "r"}
		db.Create(&relM)
	}

	PruneReleases(db, svcM.ID, keep)

	count, err := repo.CountReleasesByServiceID(ctx, db, svcM.ID)
	if err != nil {
		t.Fatal(err)
	}
	if count != int64(keep) {
		t.Errorf("expected %d releases, got %d", keep, count)
	}
}

func TestPruneReleases_ProtectsActiveRelease(t *testing.T) {
	db := newRetentionDB(t)
	ctx := db.Statement.Context

	svcM := model.Service{Name: "svc", ServerID: 1, Type: "docker"}
	db.Create(&svcM)

	// 创建 1 个 active release + 3 个 archived
	var activeRelID uint
	for i := 0; i < 4; i++ {
		artM := model.Artifact{ServiceID: svcM.ID, Provider: "http", Ref: "https://x.com/x.tar"}
		db.Create(&artM)
		status := domain.ReleaseStatusArchived
		if i == 0 {
			status = domain.ReleaseStatusActive
		}
		relM := model.Release{ServiceID: svcM.ID, ArtifactID: artM.ID, Label: "r", Status: string(status)}
		db.Create(&relM)
		if i == 0 {
			activeRelID = relM.ID
		}
	}
	// 设置 current_release_id
	db.Model(&svcM).Update("current_release_id", activeRelID)

	PruneReleases(db, svcM.ID, 2)

	// active release 应该还在
	rel, err := repo.GetReleaseByServiceAndID(ctx, db, svcM.ID, activeRelID)
	if err != nil {
		t.Fatalf("active release should exist: %v", err)
	}
	if rel.Status != string(domain.ReleaseStatusActive) {
		t.Errorf("active release status should remain active, got %s", rel.Status)
	}
}

func TestPruneReleases_KeepZeroSkips(t *testing.T) {
	db := newRetentionDB(t)
	ctx := db.Statement.Context

	svcM := model.Service{Name: "svc", ServerID: 1, Type: "static"}
	db.Create(&svcM)
	artM := model.Artifact{ServiceID: svcM.ID, Provider: "http", Ref: "https://x.com/x.tar"}
	db.Create(&artM)
	relM := model.Release{ServiceID: svcM.ID, ArtifactID: artM.ID, Label: "r"}
	db.Create(&relM)

	PruneReleases(db, svcM.ID, 0)

	count, err := repo.CountReleasesByServiceID(ctx, db, svcM.ID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("keep=0 should skip pruning, got %d", count)
	}
}

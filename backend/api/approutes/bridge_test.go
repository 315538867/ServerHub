package approutes

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/serverhub/serverhub/model"
)

// bridge.go 是新老路由表的双写胶水：app 改动后必须能在 Ingress/IngressRoute 上
// 看到对等行；mode 翻 none 或 route 删除时要清空，避免渲染器读到空壳 Ingress。
// 单测不依赖 nginx runner，纯 GORM/SQLite 即可。

func bridgeDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Application{}, &model.AppNginxRoute{},
		&model.Ingress{}, &model.IngressRoute{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func mkApp(t *testing.T, db *gorm.DB, mode, domain string) *model.Application {
	t.Helper()
	app := &model.Application{
		Name:        "app-" + mode,
		ServerID:    1,
		RunServerID: 1,
		Domain:      domain,
		ExposeMode:  mode,
	}
	if err := db.Create(app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	return app
}

func mkRoute(t *testing.T, db *gorm.DB, appID uint, path, upstream string, sort int) *model.AppNginxRoute {
	t.Helper()
	r := &model.AppNginxRoute{AppID: appID, Path: path, Upstream: upstream, Sort: sort}
	if err := db.Create(r).Error; err != nil {
		t.Fatalf("create route: %v", err)
	}
	return r
}

// ── syncToIngress ─────────────────────────────────────────────────────────────

func TestSync_NilArgs(t *testing.T) {
	db := bridgeDB(t)
	if err := syncToIngress(db, nil, nil); err == nil {
		t.Fatal("nil 应报错")
	}
}

func TestSync_SkipsWhenModeNoneOrEmpty(t *testing.T) {
	db := bridgeDB(t)
	for _, mode := range []string{"", "none"} {
		app := mkApp(t, db, mode, "x.com")
		r := mkRoute(t, db, app.ID, "/", "http://up:1", 0)
		if err := syncToIngress(db, app, r); err != nil {
			t.Fatalf("mode=%q 不应报错: %v", mode, err)
		}
		var cnt int64
		db.Model(&model.IngressRoute{}).Count(&cnt)
		if cnt != 0 {
			t.Errorf("mode=%q 不该写桥接行, 但有 %d 条", mode, cnt)
		}
	}
}

func TestSync_NoServerID_Errors(t *testing.T) {
	db := bridgeDB(t)
	app := &model.Application{Name: "no-srv", Domain: "n.com", ExposeMode: "site"}
	// BeforeSave 钩子会让两个 server id 一起为 0，绕过 Create 直接构造对象
	r := &model.AppNginxRoute{ID: 1, Path: "/", Upstream: "http://x"}
	if err := syncToIngress(db, app, r); err == nil {
		t.Fatal("没有 server_id 应报错")
	}
}

func TestSync_SiteModeCreatesDomainIngress(t *testing.T) {
	db := bridgeDB(t)
	app := mkApp(t, db, "site", "site.example.com")
	r := mkRoute(t, db, app.ID, "/", "http://up:80", 0)

	if err := syncToIngress(db, app, r); err != nil {
		t.Fatalf("sync: %v", err)
	}
	var ig model.Ingress
	if err := db.First(&ig).Error; err != nil {
		t.Fatalf("ingress not created: %v", err)
	}
	if ig.MatchKind != "domain" || ig.Domain != "site.example.com" {
		t.Errorf("ingress 字段不对: %+v", ig)
	}
	var ir model.IngressRoute
	if err := db.First(&ir).Error; err != nil {
		t.Fatalf("route not created: %v", err)
	}
	if ir.IngressID != ig.ID || ir.Path != "/" || ir.Protocol != "http" {
		t.Errorf("ingress route 字段不对: %+v", ir)
	}
	if ir.Upstream.Type != "raw" || ir.Upstream.RawURL != "http://up:80" {
		t.Errorf("upstream 不对: %+v", ir.Upstream)
	}
	if ir.LegacyAppRouteID == nil || *ir.LegacyAppRouteID != r.ID {
		t.Errorf("legacy 映射缺失: %+v", ir.LegacyAppRouteID)
	}
}

func TestSync_PathModeMapsToPathKind(t *testing.T) {
	db := bridgeDB(t)
	app := mkApp(t, db, "path", "")
	r := mkRoute(t, db, app.ID, "/foo", "http://x:1", 5)

	if err := syncToIngress(db, app, r); err != nil {
		t.Fatalf("sync: %v", err)
	}
	var ig model.Ingress
	db.First(&ig)
	if ig.MatchKind != "path" {
		t.Errorf("path 模式应映射到 match_kind=path, got=%s", ig.MatchKind)
	}
	// 空 domain 应该被替换为 "_" 占位
	if ig.Domain != "_" {
		t.Errorf("空 domain 应占位为 _, got=%q", ig.Domain)
	}
}

func TestSync_UpdateExistingByLegacyID(t *testing.T) {
	db := bridgeDB(t)
	app := mkApp(t, db, "site", "u.com")
	r := mkRoute(t, db, app.ID, "/v1", "http://up:1", 0)

	if err := syncToIngress(db, app, r); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	// 改路由后再 sync —— 应当 update 同一条 IngressRoute 而不是新建
	r.Path = "/v2"
	r.Upstream = "http://up:2"
	r.Sort = 9
	db.Save(r)
	if err := syncToIngress(db, app, r); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	var rows []model.IngressRoute
	db.Find(&rows)
	if len(rows) != 1 {
		t.Fatalf("应仍只有 1 条 IngressRoute, got %d", len(rows))
	}
	if rows[0].Path != "/v2" || rows[0].Sort != 9 || rows[0].Upstream.RawURL != "http://up:2" {
		t.Errorf("update 未生效: %+v", rows[0])
	}
}

func TestSync_MatchKindDriftRewrites(t *testing.T) {
	db := bridgeDB(t)
	// 历史 Ingress 是 path 模式
	app := mkApp(t, db, "path", "shared.com")
	r1 := mkRoute(t, db, app.ID, "/a", "http://x:1", 0)
	if err := syncToIngress(db, app, r1); err != nil {
		t.Fatalf("first: %v", err)
	}
	// app 改成 site 模式，再写一条
	app.ExposeMode = "site"
	db.Save(app)
	r2 := mkRoute(t, db, app.ID, "/", "http://x:2", 0)
	if err := syncToIngress(db, app, r2); err != nil {
		t.Fatalf("second: %v", err)
	}
	var ig model.Ingress
	db.First(&ig)
	if ig.MatchKind != "domain" {
		t.Errorf("后写应把 match_kind 漂移到 domain, got=%s", ig.MatchKind)
	}
}

// ── resyncAppRoutes ───────────────────────────────────────────────────────────

func TestResync_NilApp(t *testing.T) {
	db := bridgeDB(t)
	if err := resyncAppRoutes(db, nil); err == nil {
		t.Fatal("nil 应报错")
	}
}

func TestResync_ModeNoneClearsBridge(t *testing.T) {
	db := bridgeDB(t)
	app := mkApp(t, db, "site", "c.com")
	r1 := mkRoute(t, db, app.ID, "/a", "http://x:1", 0)
	r2 := mkRoute(t, db, app.ID, "/b", "http://x:2", 0)
	if err := syncToIngress(db, app, r1); err != nil {
		t.Fatal(err)
	}
	if err := syncToIngress(db, app, r2); err != nil {
		t.Fatal(err)
	}

	// 翻成 none，bridge 应被清光（含 Ingress 空壳）
	app.ExposeMode = "none"
	db.Save(app)
	if err := resyncAppRoutes(db, app); err != nil {
		t.Fatalf("resync: %v", err)
	}
	var irCnt, igCnt int64
	db.Model(&model.IngressRoute{}).Count(&irCnt)
	db.Model(&model.Ingress{}).Count(&igCnt)
	if irCnt != 0 || igCnt != 0 {
		t.Errorf("mode=none 应清光，剩 ir=%d ig=%d", irCnt, igCnt)
	}
}

func TestResync_SiteModeReinstallsAll(t *testing.T) {
	db := bridgeDB(t)
	app := mkApp(t, db, "site", "r.com")
	mkRoute(t, db, app.ID, "/a", "http://x:1", 0)
	mkRoute(t, db, app.ID, "/b", "http://x:2", 5)

	if err := resyncAppRoutes(db, app); err != nil {
		t.Fatalf("resync: %v", err)
	}
	var rows []model.IngressRoute
	db.Find(&rows)
	if len(rows) != 2 {
		t.Errorf("应同步 2 条, got %d", len(rows))
	}
}

// ── removeIngressRouteByLegacy ────────────────────────────────────────────────

func TestRemove_MissingIsNoop(t *testing.T) {
	db := bridgeDB(t)
	if err := removeIngressRouteByLegacy(db, 9999); err != nil {
		t.Errorf("缺失行应静默成功, got %v", err)
	}
}

func TestRemove_DropsOrphanIngress(t *testing.T) {
	db := bridgeDB(t)
	app := mkApp(t, db, "site", "d.com")
	r := mkRoute(t, db, app.ID, "/", "http://x:1", 0)
	if err := syncToIngress(db, app, r); err != nil {
		t.Fatal(err)
	}
	// 唯一一条 IngressRoute 删了之后，空壳 Ingress 也要清
	if err := removeIngressRouteByLegacy(db, r.ID); err != nil {
		t.Fatalf("remove: %v", err)
	}
	var irCnt, igCnt int64
	db.Model(&model.IngressRoute{}).Count(&irCnt)
	db.Model(&model.Ingress{}).Count(&igCnt)
	if irCnt != 0 || igCnt != 0 {
		t.Errorf("应清空, 剩 ir=%d ig=%d", irCnt, igCnt)
	}
}

func TestRemove_KeepsIngressWithSiblings(t *testing.T) {
	db := bridgeDB(t)
	app := mkApp(t, db, "site", "k.com")
	r1 := mkRoute(t, db, app.ID, "/a", "http://x:1", 0)
	r2 := mkRoute(t, db, app.ID, "/b", "http://x:2", 0)
	if err := syncToIngress(db, app, r1); err != nil {
		t.Fatal(err)
	}
	if err := syncToIngress(db, app, r2); err != nil {
		t.Fatal(err)
	}

	if err := removeIngressRouteByLegacy(db, r1.ID); err != nil {
		t.Fatal(err)
	}
	var igCnt, irCnt int64
	db.Model(&model.Ingress{}).Count(&igCnt)
	db.Model(&model.IngressRoute{}).Count(&irCnt)
	if igCnt != 1 || irCnt != 1 {
		t.Errorf("还有兄弟时不应清 Ingress, 剩 ig=%d ir=%d", igCnt, irCnt)
	}
}

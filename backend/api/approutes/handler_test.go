package approutes

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
)

// 这层只覆盖 HTTP 路由 + DTO 校验 + DB 状态变化 + 桥接联动。
// apply 走 nginxops.Apply 需要 runner，已在 pkg/nginxops 单测里覆盖，这里跳过。

func setupHandler(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
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
	r := gin.New()
	g := r.Group("/apps")
	RegisterRoutes(g, db, &config.Config{})
	return r, db
}

func req(t *testing.T, r *gin.Engine, method, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}
	rq := httptest.NewRequest(method, path, &buf)
	rq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, rq)
	var out map[string]any
	if w.Body.Len() > 0 {
		_ = json.Unmarshal(w.Body.Bytes(), &out)
	}
	return w, out
}

func mkApplication(t *testing.T, db *gorm.DB, mode, domain string) *model.Application {
	t.Helper()
	a := &model.Application{Name: "app-" + mode + "-" + domain, ServerID: 1, RunServerID: 1, Domain: domain, ExposeMode: mode}
	if err := db.Create(a).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	return a
}

// ── GET /:id/nginx ────────────────────────────────────────────────────────────

func TestHandler_GetNginx_OK(t *testing.T) {
	r, db := setupHandler(t)
	a := mkApplication(t, db, "site", "g.com")
	db.Create(&model.AppNginxRoute{AppID: a.ID, Path: "/", Upstream: "http://up:1"})

	w, body := req(t, r, "GET", "/apps/"+itoa(a.ID)+"/nginx", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%v", w.Code, body)
	}
	d := body["data"].(map[string]any)
	if d["expose_mode"] != "site" {
		t.Errorf("expose_mode=%v", d["expose_mode"])
	}
	rs := d["routes"].([]any)
	if len(rs) != 1 {
		t.Errorf("want 1 route, got %d", len(rs))
	}
}

func TestHandler_GetNginx_AppMissing(t *testing.T) {
	r, _ := setupHandler(t)
	w, _ := req(t, r, "GET", "/apps/999/nginx", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expect 404, got %d", w.Code)
	}
}

func TestHandler_GetNginx_BadID(t *testing.T) {
	r, _ := setupHandler(t)
	w, _ := req(t, r, "GET", "/apps/abc/nginx", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expect 400, got %d", w.Code)
	}
}

// ── PUT /:id/nginx/mode ───────────────────────────────────────────────────────

func TestHandler_SetMode_OK_TriggersResync(t *testing.T) {
	r, db := setupHandler(t)
	a := mkApplication(t, db, "none", "m.com")
	rt := &model.AppNginxRoute{AppID: a.ID, Path: "/", Upstream: "http://up:1"}
	db.Create(rt)

	w, _ := req(t, r, "PUT", "/apps/"+itoa(a.ID)+"/nginx/mode", map[string]any{"mode": "site"})
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	var got model.Application
	db.First(&got, a.ID)
	if got.ExposeMode != "site" {
		t.Errorf("expose_mode=%s", got.ExposeMode)
	}
	// resync 应把 Ingress/IngressRoute 也同步起来
	var igCnt, irCnt int64
	db.Model(&model.Ingress{}).Count(&igCnt)
	db.Model(&model.IngressRoute{}).Count(&irCnt)
	if igCnt != 1 || irCnt != 1 {
		t.Errorf("setMode 应触发 resync, got ig=%d ir=%d", igCnt, irCnt)
	}
}

func TestHandler_SetMode_NoneClearsBridge(t *testing.T) {
	r, db := setupHandler(t)
	a := mkApplication(t, db, "site", "n.com")
	rt := &model.AppNginxRoute{AppID: a.ID, Path: "/", Upstream: "http://up:1"}
	db.Create(rt)
	if err := syncToIngress(db, a, rt); err != nil {
		t.Fatal(err)
	}

	w, _ := req(t, r, "PUT", "/apps/"+itoa(a.ID)+"/nginx/mode", map[string]any{"mode": "none"})
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	var igCnt int64
	db.Model(&model.Ingress{}).Count(&igCnt)
	if igCnt != 0 {
		t.Errorf("mode=none 后 Ingress 应被清, 剩 %d", igCnt)
	}
}

func TestHandler_SetMode_BadValue(t *testing.T) {
	r, db := setupHandler(t)
	a := mkApplication(t, db, "site", "b.com")
	w, _ := req(t, r, "PUT", "/apps/"+itoa(a.ID)+"/nginx/mode", map[string]any{"mode": "weird"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expect 400, got %d", w.Code)
	}
}

func TestHandler_SetMode_EmptyBody(t *testing.T) {
	r, db := setupHandler(t)
	a := mkApplication(t, db, "site", "e.com")
	w, _ := req(t, r, "PUT", "/apps/"+itoa(a.ID)+"/nginx/mode", map[string]any{})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expect 400, got %d", w.Code)
	}
}

// ── POST /:id/nginx/routes ────────────────────────────────────────────────────

func TestHandler_AddRoute_OK_Bridges(t *testing.T) {
	r, db := setupHandler(t)
	a := mkApplication(t, db, "site", "a.com")

	w, body := req(t, r, "POST", "/apps/"+itoa(a.ID)+"/nginx/routes", map[string]any{
		"path": "/api", "upstream": "http://up:1", "sort": 3,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%v", w.Code, body)
	}
	var legCnt, irCnt int64
	db.Model(&model.AppNginxRoute{}).Count(&legCnt)
	db.Model(&model.IngressRoute{}).Count(&irCnt)
	if legCnt != 1 || irCnt != 1 {
		t.Errorf("add 应同写老/新表, got leg=%d ir=%d", legCnt, irCnt)
	}
}

func TestHandler_AddRoute_RejectsBadPath(t *testing.T) {
	r, db := setupHandler(t)
	a := mkApplication(t, db, "site", "p.com")
	w, _ := req(t, r, "POST", "/apps/"+itoa(a.ID)+"/nginx/routes", map[string]any{
		"path": "/foo\nbar", "upstream": "http://up:1",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expect 400, got %d", w.Code)
	}
}

func TestHandler_AddRoute_RejectsBadExtra(t *testing.T) {
	r, db := setupHandler(t)
	a := mkApplication(t, db, "site", "x.com")
	w, _ := req(t, r, "POST", "/apps/"+itoa(a.ID)+"/nginx/routes", map[string]any{
		"path": "/", "upstream": "http://up:1", "extra": "} server { listen 1;",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expect 400, got %d", w.Code)
	}
}

func TestHandler_AddRoute_MissingFields(t *testing.T) {
	r, db := setupHandler(t)
	a := mkApplication(t, db, "site", "f.com")
	w, _ := req(t, r, "POST", "/apps/"+itoa(a.ID)+"/nginx/routes", map[string]any{"path": "/"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expect 400, got %d", w.Code)
	}
}

// ── PUT /:id/nginx/routes/:rid ────────────────────────────────────────────────

func TestHandler_UpdateRoute_OK(t *testing.T) {
	r, db := setupHandler(t)
	a := mkApplication(t, db, "site", "u.com")
	rt := &model.AppNginxRoute{AppID: a.ID, Path: "/v1", Upstream: "http://x:1"}
	db.Create(rt)
	if err := syncToIngress(db, a, rt); err != nil {
		t.Fatal(err)
	}

	w, _ := req(t, r, "PUT", "/apps/"+itoa(a.ID)+"/nginx/routes/"+itoa(rt.ID), map[string]any{
		"path": "/v2", "upstream": "http://x:2", "sort": 7,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	var got model.AppNginxRoute
	db.First(&got, rt.ID)
	if got.Path != "/v2" || got.Sort != 7 {
		t.Errorf("legacy 表未生效: %+v", got)
	}
	var ir model.IngressRoute
	db.Where("legacy_app_route_id = ?", rt.ID).First(&ir)
	if ir.Path != "/v2" || ir.Sort != 7 || ir.Upstream.RawURL != "http://x:2" {
		t.Errorf("桥接行未同步: %+v", ir)
	}
}

func TestHandler_UpdateRoute_BadRouteID(t *testing.T) {
	r, db := setupHandler(t)
	a := mkApplication(t, db, "site", "u2.com")
	w, _ := req(t, r, "PUT", "/apps/"+itoa(a.ID)+"/nginx/routes/abc", map[string]any{
		"path": "/", "upstream": "http://up:1",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expect 400, got %d", w.Code)
	}
}

func TestHandler_UpdateRoute_NotFound(t *testing.T) {
	r, db := setupHandler(t)
	a := mkApplication(t, db, "site", "u3.com")
	w, _ := req(t, r, "PUT", "/apps/"+itoa(a.ID)+"/nginx/routes/9999", map[string]any{
		"path": "/", "upstream": "http://up:1",
	})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expect 404, got %d", w.Code)
	}
}

// ── DELETE /:id/nginx/routes/:rid ─────────────────────────────────────────────

func TestHandler_DeleteRoute_DropsBoth(t *testing.T) {
	r, db := setupHandler(t)
	a := mkApplication(t, db, "site", "d.com")
	rt := &model.AppNginxRoute{AppID: a.ID, Path: "/", Upstream: "http://x:1"}
	db.Create(rt)
	if err := syncToIngress(db, a, rt); err != nil {
		t.Fatal(err)
	}

	w, _ := req(t, r, "DELETE", "/apps/"+itoa(a.ID)+"/nginx/routes/"+itoa(rt.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	var legCnt, irCnt, igCnt int64
	db.Model(&model.AppNginxRoute{}).Count(&legCnt)
	db.Model(&model.IngressRoute{}).Count(&irCnt)
	db.Model(&model.Ingress{}).Count(&igCnt)
	if legCnt != 0 || irCnt != 0 || igCnt != 0 {
		t.Errorf("delete 应清两表 + 空壳 Ingress, 剩 leg=%d ir=%d ig=%d", legCnt, irCnt, igCnt)
	}
}

func TestHandler_DeleteRoute_BadID(t *testing.T) {
	r, db := setupHandler(t)
	a := mkApplication(t, db, "site", "d2.com")
	w, _ := req(t, r, "DELETE", "/apps/"+itoa(a.ID)+"/nginx/routes/abc", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expect 400, got %d", w.Code)
	}
}

// ── apply: 仅校验失败路径（成功路径需要 runner，已在 nginxops 测） ────────────

func TestHandler_Apply_NoEdgeBound(t *testing.T) {
	r, db := setupHandler(t)
	// ServerID/RunServerID 都为 0 —— BeforeSave 不会改 0/0
	a := &model.Application{Name: "apply-noserver", Domain: "z.com", ExposeMode: "site"}
	db.Create(a)

	w, _ := req(t, r, "POST", "/apps/"+itoa(a.ID)+"/nginx/apply", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("无 edge 绑定应 400, got %d", w.Code)
	}
}

func TestHandler_Apply_BadAppName(t *testing.T) {
	r, db := setupHandler(t)
	// 名字含路径分隔符，validateAppName 应拒
	a := &model.Application{Name: "../etc/passwd", ServerID: 1, RunServerID: 1, ExposeMode: "site"}
	db.Create(a)
	w, _ := req(t, r, "POST", "/apps/"+itoa(a.ID)+"/nginx/apply", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("非法 app 名应 400, got %d", w.Code)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func itoa(u uint) string {
	if u == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for u > 0 {
		i--
		buf[i] = byte('0' + u%10)
		u /= 10
	}
	return string(buf[i:])
}

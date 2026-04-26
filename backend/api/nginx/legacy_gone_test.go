package nginx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
)

// Phase Nginx-P3F:legacy site CRUD 全部 410 Gone。这里只断言 status + 关键头,
// 不打 runner / DB 路径——410 必须在 handler 链最前段触发,不能依赖任何远端
// 可达性,否则就成了"sudo 失败时也吐 410"的脏 fallback。

func setupLegacyGone(t *testing.T) (*gin.Engine, uint) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Server{}, &model.NginxProfile{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	s := model.Server{Name: "edge", Host: "h"}
	db.Create(&s)
	r := gin.New()
	g := r.Group("/servers")
	RegisterRoutes(g, db, &config.Config{})
	return r, s.ID
}

func TestLegacySiteCRUD_AllReturnGone(t *testing.T) {
	r, sid := setupLegacyGone(t)
	idStr := func() string {
		// 走字符串拼接而不是 strconv,纯粹是测试可读性。
		return "/servers/" + itoa(sid) + "/nginx"
	}()
	cases := []struct {
		method, path string
	}{
		{"GET", idStr + "/sites"},
		{"POST", idStr + "/sites"},
		{"GET", idStr + "/sites/foo/config"},
		{"PUT", idStr + "/sites/foo/config"},
		{"DELETE", idStr + "/sites/foo"},
		{"POST", idStr + "/sites/foo/enable"},
		{"POST", idStr + "/sites/foo/disable"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusGone {
			t.Errorf("%s %s: code=%d, want 410; body=%s", tc.method, tc.path, w.Code, w.Body.String())
			continue
		}
		if w.Header().Get("Deprecation") != "true" {
			t.Errorf("%s %s: missing Deprecation header", tc.method, tc.path)
		}
		if w.Header().Get("Sunset") == "" {
			t.Errorf("%s %s: missing Sunset header", tc.method, tc.path)
		}
		if !strings.Contains(w.Header().Get("Link"), "successor-version") {
			t.Errorf("%s %s: Link header lacks successor-version: %q",
				tc.method, tc.path, w.Header().Get("Link"))
		}
		if !strings.Contains(w.Body.String(), "/api/v1/ingresses") {
			t.Errorf("%s %s: body lacks ingresses migration hint: %s",
				tc.method, tc.path, w.Body.String())
		}
	}
}

// reload/restart/logs/profile 这些非 legacy 路径不应被 410 误伤。logs 是
// websocket 不好直接探,这里只覆盖 reload/restart——它们走 getRunner,真
// runner 拉起会失败,但 status 不应是 410。
func TestNonLegacyRoutes_NotGone(t *testing.T) {
	r, sid := setupLegacyGone(t)
	base := "/servers/" + itoa(sid) + "/nginx"
	for _, p := range []string{"/reload", "/restart"} {
		req := httptest.NewRequest("POST", base+p, strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code == http.StatusGone {
			t.Errorf("POST %s: 不应该是 410: body=%s", base+p, w.Body.String())
		}
	}
}


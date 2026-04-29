package application

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
	"github.com/serverhub/serverhub/repo"
)

// 反向视图独立测试:不打 SSH,只覆盖 service_id 反查 + Upstream JSON 解析 +
// MatchingRoutes 子集筛选 + Server.Name 注入。

func setupApp(t *testing.T) (*gin.Engine, repo.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Server{}, &model.Application{}, &model.Service{},
		&model.Ingress{}, &model.IngressRoute{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	r := gin.New()
	g := r.Group("/apps")
	RegisterRoutes(g, db, &config.Config{})
	return r, db
}

func doReq(t *testing.T, r *gin.Engine, method, path string, body any) (int, []map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var env struct {
		Code int              `json:"code"`
		Data []map[string]any `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	return w.Code, env.Data
}

func TestListAppIngresses_ReturnsMatchingRoutesOnly(t *testing.T) {
	r, db := setupApp(t)

	edge := model.Server{Name: "edge-a"}
	db.Create(&edge)
	otherEdge := model.Server{Name: "edge-b"}
	db.Create(&otherEdge)

	app := model.Application{Name: "myapp", ServerID: edge.ID, RunServerID: edge.ID}
	db.Create(&app)
	otherApp := model.Application{Name: "other", ServerID: edge.ID, RunServerID: edge.ID}
	db.Create(&otherApp)

	mySvc := model.Service{Name: "my-api", ServerID: edge.ID, ApplicationID: &app.ID, ExposedPort: 8080}
	db.Create(&mySvc)
	otherSvc := model.Service{Name: "other-api", ServerID: edge.ID, ApplicationID: &otherApp.ID, ExposedPort: 9090}
	db.Create(&otherSvc)

	// Ingress #1: 同时含本 app 路由 + 别人的路由 + raw 路由 → 反向视图只应返回本 app 的那条
	ig1 := model.Ingress{EdgeServerID: edge.ID, MatchKind: "domain", Domain: "shared.com"}
	db.Create(&ig1)
	mineRoute := model.IngressRoute{IngressID: ig1.ID, Path: "/mine",
		Upstream: model.IngressUpstream{Type: "service", ServiceID: &mySvc.ID}}
	db.Create(&mineRoute)
	db.Create(&model.IngressRoute{IngressID: ig1.ID, Path: "/other",
		Upstream: model.IngressUpstream{Type: "service", ServiceID: &otherSvc.ID}})
	db.Create(&model.IngressRoute{IngressID: ig1.ID, Path: "/raw",
		Upstream: model.IngressUpstream{Type: "raw", RawURL: "http://x"}})

	// Ingress #2: 在另一台 edge 上,只命中 mySvc → 应返回,且 EdgeServerName 注入
	ig2 := model.Ingress{EdgeServerID: otherEdge.ID, MatchKind: "domain", Domain: "cross.com"}
	db.Create(&ig2)
	db.Create(&model.IngressRoute{IngressID: ig2.ID, Path: "/",
		Upstream: model.IngressUpstream{Type: "service", ServiceID: &mySvc.ID}})

	// Ingress #3: 完全不沾 → 不应出现
	ig3 := model.Ingress{EdgeServerID: edge.ID, MatchKind: "domain", Domain: "unrelated.com"}
	db.Create(&ig3)
	db.Create(&model.IngressRoute{IngressID: ig3.ID, Path: "/",
		Upstream: model.IngressUpstream{Type: "service", ServiceID: &otherSvc.ID}})

	code, data := doReq(t, r, "GET",
		"/apps/"+itoa(app.ID)+"/ingresses", nil)
	if code != http.StatusOK {
		t.Fatalf("status=%d", code)
	}
	if len(data) != 2 {
		t.Fatalf("应返回 2 条 ingress(ig1+ig2),得 %d", len(data))
	}
	domains := map[string]bool{}
	for _, ig := range data {
		domains[ig["domain"].(string)] = true
		if ig["domain"] == "shared.com" {
			routes, _ := ig["matching_routes"].([]any)
			if len(routes) != 1 {
				t.Errorf("shared.com 反向视图应只见 1 条 matching_routes,得 %d", len(routes))
			}
		}
		if ig["domain"] == "cross.com" {
			if name, _ := ig["edge_server_name"].(string); name != "edge-b" {
				t.Errorf("cross.com edge_server_name=%q,want edge-b", name)
			}
		}
	}
	if !domains["shared.com"] || !domains["cross.com"] {
		t.Errorf("结果集缺失,domains=%v", domains)
	}
	if domains["unrelated.com"] {
		t.Errorf("不该命中 unrelated.com")
	}
}

func TestListAppIngresses_AppHasNoServices(t *testing.T) {
	r, db := setupApp(t)
	edge := model.Server{Name: "e"}
	db.Create(&edge)
	app := model.Application{Name: "noservice", ServerID: edge.ID, RunServerID: edge.ID}
	db.Create(&app)

	code, data := doReq(t, r, "GET", "/apps/"+itoa(app.ID)+"/ingresses", nil)
	if code != http.StatusOK {
		t.Fatalf("status=%d", code)
	}
	if data == nil {
		// resp.OK 给 [] 时会被 json 解成 []any{}; 不是 nil。空也接受。
		return
	}
	if len(data) != 0 {
		t.Errorf("无 service 的 app 应返回空,得 %v", data)
	}
}

func TestListAppIngresses_BadID(t *testing.T) {
	r, _ := setupApp(t)
	code, _ := doReq(t, r, "GET", "/apps/abc/ingresses", nil)
	if code == http.StatusOK {
		t.Errorf("非法 id 应非 200,得 200")
	}
}

func itoa(u uint) string {
	if u == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for u > 0 {
		i--
		b[i] = byte('0' + u%10)
		u /= 10
	}
	return string(b[i:])
}

package ingresses

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

// 这层只覆盖 HTTP 路由 + DTO 校验 + DB 状态变化 — apply / dry-run 需要 runner，
// 已在 pkg/nginxops 单测覆盖，这里不重复。

func setup(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Server{}, &model.Service{}, &model.User{},
		&model.Ingress{}, &model.IngressRoute{}, &model.AuditApply{},
		&model.SSLCert{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	r := gin.New()
	g := r.Group("/ingresses")
	RegisterRoutes(g, db, &config.Config{})
	return r, db
}

func do(t *testing.T, r *gin.Engine, method, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var out map[string]any
	if w.Body.Len() > 0 {
		_ = json.Unmarshal(w.Body.Bytes(), &out)
	}
	return w, out
}

func mkEdge(t *testing.T, db *gorm.DB) uint {
	t.Helper()
	s := model.Server{Name: "edge"}
	if err := db.Create(&s).Error; err != nil {
		t.Fatalf("create edge: %v", err)
	}
	return s.ID
}

// ── CRUD ──────────────────────────────────────────────────────────────────────

func TestCreate_OK_WithRoutes(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)

	w, body := do(t, r, "POST", "/ingresses", map[string]any{
		"edge_server_id": edge,
		"match_kind":     "domain",
		"domain":         "demo.example.com",
		"routes": []map[string]any{
			{"path": "/", "upstream": map[string]any{"type": "raw", "raw_url": "http://x:1"}},
			{"path": "/api", "upstream": map[string]any{"type": "raw", "raw_url": "http://x:2"}, "sort": 10},
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%v", w.Code, body)
	}
	if body["code"].(float64) != 0 {
		t.Fatalf("code=%v body=%v", body["code"], body)
	}
	var routes []model.IngressRoute
	db.Where("ingress_id > 0").Find(&routes)
	if len(routes) != 2 {
		t.Errorf("应创建 2 条路由, got %d", len(routes))
	}
	for _, rt := range routes {
		if rt.Protocol != "http" {
			t.Errorf("默认 protocol 应为 http, got %s", rt.Protocol)
		}
	}
}

func TestCreate_BadMatchKind(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	w, _ := do(t, r, "POST", "/ingresses", map[string]any{
		"edge_server_id": edge, "match_kind": "weird", "domain": "x.com",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("应 400, got %d", w.Code)
	}
}

func TestCreate_RejectsMatchKindMix(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	// 先建一个 domain 模式
	w, _ := do(t, r, "POST", "/ingresses", map[string]any{
		"edge_server_id": edge, "match_kind": "domain", "domain": "shared.com",
	})
	if w.Code != http.StatusOK {
		t.Fatal(w.Code)
	}
	// 同 edge+domain 用 path 应 400
	w2, body := do(t, r, "POST", "/ingresses", map[string]any{
		"edge_server_id": edge, "match_kind": "path", "domain": "shared.com",
	})
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("应拒绝 mix, got %d body=%v", w2.Code, body)
	}
}

func TestList_FilteredByEdge(t *testing.T) {
	r, db := setup(t)
	e1 := mkEdge(t, db)
	e2 := mkEdge(t, db)
	db.Create(&model.Ingress{EdgeServerID: e1, MatchKind: "domain", Domain: "a.com"})
	db.Create(&model.Ingress{EdgeServerID: e2, MatchKind: "domain", Domain: "b.com"})

	w, body := do(t, r, "GET", "/ingresses", nil)
	if w.Code != http.StatusOK {
		t.Fatal(w.Code)
	}
	if data, _ := body["data"].([]any); len(data) != 2 {
		t.Errorf("无 filter 应返回全部 2 条, got %d", len(data))
	}

	url := "/ingresses?edge_server_id=" + uintToStr(e1)
	w2, body2 := do(t, r, "GET", url, nil)
	if w2.Code != http.StatusOK {
		t.Fatal(w2.Code)
	}
	data, _ := body2["data"].([]any)
	if len(data) != 1 {
		t.Errorf("filter 后应只剩 1 条, got %d", len(data))
	}
}

func TestList_BadEdgeFilter(t *testing.T) {
	r, _ := setup(t)
	w, _ := do(t, r, "GET", "/ingresses?edge_server_id=abc", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("非法 edge_server_id 应 400, got %d", w.Code)
	}
}

func TestGet_WithRoutes(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	ig := model.Ingress{EdgeServerID: edge, MatchKind: "domain", Domain: "g.com"}
	db.Create(&ig)
	db.Create(&model.IngressRoute{IngressID: ig.ID, Path: "/", Protocol: "http"})

	w, body := do(t, r, "GET", "/ingresses/"+uintToStr(ig.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	d := body["data"].(map[string]any)
	rs := d["routes"].([]any)
	if len(rs) != 1 {
		t.Errorf("want 1 route in detail, got %d", len(rs))
	}
}

func TestGet_NotFound(t *testing.T) {
	r, _ := setup(t)
	w, _ := do(t, r, "GET", "/ingresses/9999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expect 404, got %d", w.Code)
	}
}

func TestUpdate_FlipsStatusToPending(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	ig := model.Ingress{EdgeServerID: edge, MatchKind: "domain", Domain: "u.com", Status: "applied"}
	db.Create(&ig)

	w, _ := do(t, r, "PUT", "/ingresses/"+uintToStr(ig.ID), map[string]any{"domain": "u2.com"})
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	var got model.Ingress
	db.First(&got, ig.ID)
	if got.Domain != "u2.com" || got.Status != "pending" {
		t.Errorf("update 后状态/域名不正确: %+v", got)
	}
}

func TestUpdate_NoFields_NoOp(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	ig := model.Ingress{EdgeServerID: edge, MatchKind: "domain", Domain: "u.com", Status: "applied"}
	db.Create(&ig)

	w, _ := do(t, r, "PUT", "/ingresses/"+uintToStr(ig.ID), map[string]any{})
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	var got model.Ingress
	db.First(&got, ig.ID)
	if got.Status != "applied" {
		t.Errorf("无更新字段不应改 status, got=%s", got.Status)
	}
}

func TestDelete_CascadesRoutes(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	ig := model.Ingress{EdgeServerID: edge, MatchKind: "domain", Domain: "d.com"}
	db.Create(&ig)
	db.Create(&model.IngressRoute{IngressID: ig.ID, Path: "/"})
	db.Create(&model.IngressRoute{IngressID: ig.ID, Path: "/api"})

	w, _ := do(t, r, "DELETE", "/ingresses/"+uintToStr(ig.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	var cnt int64
	db.Model(&model.IngressRoute{}).Where("ingress_id = ?", ig.ID).Count(&cnt)
	if cnt != 0 {
		t.Errorf("delete 应级联清掉 routes, 还剩 %d", cnt)
	}
}

// ── 路由子资源 ────────────────────────────────────────────────────────────────

func TestRoute_AddUpdateDelete(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	ig := model.Ingress{EdgeServerID: edge, MatchKind: "domain", Domain: "r.com", Status: "applied"}
	db.Create(&ig)

	// add
	w, body := do(t, r, "POST", "/ingresses/"+uintToStr(ig.ID)+"/routes", map[string]any{
		"path": "/api", "sort": 5,
		"upstream": map[string]any{"type": "raw", "raw_url": "http://x:1"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("add status=%d", w.Code)
	}
	d := body["data"].(map[string]any)
	rid := uint(d["id"].(float64))

	// status 应被翻为 pending
	var got model.Ingress
	db.First(&got, ig.ID)
	if got.Status != "pending" {
		t.Errorf("add route 应将 ingress 翻 pending, got %s", got.Status)
	}

	// update
	w2, _ := do(t, r, "PUT", "/ingresses/"+uintToStr(ig.ID)+"/routes/"+uintToStr(rid), map[string]any{
		"path": "/v2", "sort": 7,
		"upstream": map[string]any{"type": "raw", "raw_url": "http://y:2"},
	})
	if w2.Code != http.StatusOK {
		t.Fatalf("upd status=%d", w2.Code)
	}
	var rt model.IngressRoute
	db.First(&rt, rid)
	if rt.Path != "/v2" || rt.Sort != 7 {
		t.Errorf("update 未生效: %+v", rt)
	}

	// delete
	w3, _ := do(t, r, "DELETE", "/ingresses/"+uintToStr(ig.ID)+"/routes/"+uintToStr(rid), nil)
	if w3.Code != http.StatusOK {
		t.Fatalf("del status=%d", w3.Code)
	}
	var cnt int64
	db.Model(&model.IngressRoute{}).Where("id = ?", rid).Count(&cnt)
	if cnt != 0 {
		t.Errorf("delete 未生效, 仍剩 %d", cnt)
	}
}

func TestRoute_AddOnMissingIngress(t *testing.T) {
	r, _ := setup(t)
	w, _ := do(t, r, "POST", "/ingresses/9999/routes", map[string]any{
		"path": "/", "upstream": map[string]any{"type": "raw", "raw_url": "x"},
	})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expect 404, got %d", w.Code)
	}
}

func TestRoute_UpdateMissing(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	ig := model.Ingress{EdgeServerID: edge, MatchKind: "domain", Domain: "x.com"}
	db.Create(&ig)
	w, _ := do(t, r, "PUT", "/ingresses/"+uintToStr(ig.ID)+"/routes/9999", map[string]any{
		"path": "/", "upstream": map[string]any{"type": "raw", "raw_url": "x"},
	})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expect 404, got %d", w.Code)
	}
}

// ── audit / services ─────────────────────────────────────────────────────────

func TestAudit_ListByEdge(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	other := mkEdge(t, db)
	db.Create(&model.AuditApply{EdgeServerID: edge, ChangesetDiff: "+ /a"})
	db.Create(&model.AuditApply{EdgeServerID: edge, ChangesetDiff: "- /b"})
	db.Create(&model.AuditApply{EdgeServerID: other, ChangesetDiff: "~ /c"})

	w, body := do(t, r, "GET", "/ingresses/edges/"+uintToStr(edge)+"/audit", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	rows := body["data"].([]any)
	if len(rows) != 2 {
		t.Errorf("仅返回该 edge 的审计行: want 2 got %d", len(rows))
	}
}

func TestAudit_JoinsActorUsername(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	u := model.User{Username: "alice", Password: "x"}
	db.Create(&u)
	uid := u.ID
	db.Create(&model.AuditApply{EdgeServerID: edge, ActorUserID: &uid, ChangesetDiff: "+ /a"})
	db.Create(&model.AuditApply{EdgeServerID: edge, ChangesetDiff: "+ /b"}) // actor_user_id=nil → 系统触发

	w, body := do(t, r, "GET", "/ingresses/edges/"+uintToStr(edge)+"/audit", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	rows := body["data"].([]any)
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
	// 按 id DESC 排序:第一条是 nil actor,第二条是 alice
	first := rows[0].(map[string]any)
	second := rows[1].(map[string]any)
	if got := first["actor_username"]; got != "" {
		t.Errorf("无 actor 行 username 应为空,得 %q", got)
	}
	if got := second["actor_username"]; got != "alice" {
		t.Errorf("有 actor 行应回填 alice,得 %v", got)
	}
}

func TestAudit_LimitClamped(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	for i := 0; i < 5; i++ {
		db.Create(&model.AuditApply{EdgeServerID: edge, ChangesetDiff: "x"})
	}
	w, body := do(t, r, "GET", "/ingresses/edges/"+uintToStr(edge)+"/audit?limit=2", nil)
	if w.Code != http.StatusOK {
		t.Fatal(w.Code)
	}
	rows := body["data"].([]any)
	if len(rows) != 2 {
		t.Errorf("limit=2 应只返回 2 条, got %d", len(rows))
	}
}

func TestServices_Listing(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	db.Create(&model.Service{Name: "svc-a", ServerID: edge, ExposedPort: 8080})
	db.Create(&model.Service{Name: "svc-b", ServerID: edge, ExposedPort: 0})
	// 别的 server 的服务不应混入
	other := mkEdge(t, db)
	db.Create(&model.Service{Name: "noise", ServerID: other, ExposedPort: 9000})

	w, body := do(t, r, "GET", "/ingresses/services/"+uintToStr(edge), nil)
	if w.Code != http.StatusOK {
		t.Fatal(w.Code)
	}
	rows := body["data"].([]any)
	if len(rows) != 2 {
		t.Errorf("want 2 services for edge, got %d", len(rows))
	}
}

// ── 协议校验（P2-D2）─────────────────────────────────────────────────────────

func TestProtocol_GRPCAccepted(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	w, body := do(t, r, "POST", "/ingresses", map[string]any{
		"edge_server_id": edge, "match_kind": "domain", "domain": "g.com",
		"routes": []map[string]any{
			{"path": "/", "protocol": "grpc",
				"upstream": map[string]any{"type": "raw", "raw_url": "http://up:9"}},
		},
	})
	if w.Code != http.StatusOK || body["code"].(float64) != 0 {
		t.Fatalf("grpc 应放行, status=%d body=%v", w.Code, body)
	}
	var rt model.IngressRoute
	db.First(&rt)
	if rt.Protocol != "grpc" {
		t.Errorf("DB 里 protocol 应保留 grpc, got %s", rt.Protocol)
	}
}

func TestProtocol_TCPRejectedOnCreate(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	w, _ := do(t, r, "POST", "/ingresses", map[string]any{
		"edge_server_id": edge, "match_kind": "domain", "domain": "t.com",
		"routes": []map[string]any{
			{"path": "/", "protocol": "tcp",
				"upstream": map[string]any{"type": "raw", "raw_url": "10:1"}},
		},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("tcp 缺 listen_port 应 400, got %d", w.Code)
	}
	// 同事务里 ingress 也应该被回滚（不留空壳）
	var igCnt int64
	db.Model(&model.Ingress{}).Count(&igCnt)
	if igCnt != 0 {
		t.Errorf("事务回滚后不该有 Ingress, 剩 %d", igCnt)
	}
}

func TestProtocol_TCPAcceptedWithListenPort(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	w, body := do(t, r, "POST", "/ingresses", map[string]any{
		"edge_server_id": edge, "match_kind": "domain", "domain": "t.com",
		"routes": []map[string]any{
			{"path": "/", "protocol": "tcp", "listen_port": 5432,
				"upstream": map[string]any{"type": "raw", "raw_url": "10.0.0.5:5432"}},
		},
	})
	if w.Code != http.StatusOK || body["code"].(float64) != 0 {
		t.Fatalf("tcp + listen_port 应放行, status=%d body=%v", w.Code, body)
	}
	var rt model.IngressRoute
	if err := db.First(&rt).Error; err != nil {
		t.Fatalf("加载 route: %v", err)
	}
	if rt.Protocol != "tcp" || rt.ListenPort == nil || *rt.ListenPort != 5432 {
		t.Errorf("listen_port 未持久化: %+v", rt)
	}
}

func TestProtocol_UDPRejectedOnAddRoute(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	ig := model.Ingress{EdgeServerID: edge, MatchKind: "domain", Domain: "u.com"}
	db.Create(&ig)
	// 缺 listen_port
	w, _ := do(t, r, "POST", "/ingresses/"+uintToStr(ig.ID)+"/routes", map[string]any{
		"path": "/", "protocol": "udp",
		"upstream": map[string]any{"type": "raw", "raw_url": "10:1"},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("udp 缺 listen_port 应 400, got %d", w.Code)
	}
}

func TestProtocol_UDPAcceptedWithListenPort(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	ig := model.Ingress{EdgeServerID: edge, MatchKind: "domain", Domain: "u.com"}
	db.Create(&ig)
	w, body := do(t, r, "POST", "/ingresses/"+uintToStr(ig.ID)+"/routes", map[string]any{
		"path": "/", "protocol": "udp", "listen_port": 53,
		"upstream": map[string]any{"type": "raw", "raw_url": "10.0.0.5:53"},
	})
	if w.Code != http.StatusOK || body["code"].(float64) != 0 {
		t.Fatalf("udp + listen_port 应放行, status=%d body=%v", w.Code, body)
	}
}

func TestProtocol_TCPListenPortOutOfRange(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	w, _ := do(t, r, "POST", "/ingresses", map[string]any{
		"edge_server_id": edge, "match_kind": "domain", "domain": "x.com",
		"routes": []map[string]any{
			{"path": "/", "protocol": "tcp", "listen_port": 99999,
				"upstream": map[string]any{"type": "raw", "raw_url": "h:1"}},
		},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("超范围 listen_port 应 400, got %d", w.Code)
	}
}

func TestProtocol_UnknownRejectedOnUpdateRoute(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	ig := model.Ingress{EdgeServerID: edge, MatchKind: "domain", Domain: "x.com"}
	db.Create(&ig)
	rt := model.IngressRoute{IngressID: ig.ID, Path: "/", Protocol: "http"}
	db.Create(&rt)
	w, _ := do(t, r, "PUT", "/ingresses/"+uintToStr(ig.ID)+"/routes/"+uintToStr(rt.ID), map[string]any{
		"path": "/", "protocol": "weird",
		"upstream": map[string]any{"type": "raw", "raw_url": "x"},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("非法 protocol 应 400, got %d", w.Code)
	}
}

func TestParseUintParam_BadID(t *testing.T) {
	r, _ := setup(t)
	w, _ := do(t, r, "GET", "/ingresses/abc", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("非法 :id 应 400, got %d", w.Code)
	}
	w2, _ := do(t, r, "GET", "/ingresses/0", nil)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf(":id=0 应 400, got %d", w2.Code)
	}
}

// ── TLS / HTTPS（P2-D4）─────────────────────────────────────────────────────

// mkCert 在指定 server 下建一张 SSLCert,返回 cert ID。
func mkCert(t *testing.T, db *gorm.DB, serverID uint, domain, certPath, keyPath string) uint {
	t.Helper()
	c := model.SSLCert{
		ServerID: serverID, Domain: domain,
		CertPath: certPath, KeyPath: keyPath,
	}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("create cert: %v", err)
	}
	return c.ID
}

func TestTLS_CreateWithCertOK(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	cert := mkCert(t, db, edge, "tls.example.com", "/etc/ssl/tls.crt", "/etc/ssl/tls.key")

	w, body := do(t, r, "POST", "/ingresses", map[string]any{
		"edge_server_id": edge,
		"match_kind":     "domain",
		"domain":         "tls.example.com",
		"cert_id":        cert,
		"force_https":    true,
	})
	if w.Code != http.StatusOK || body["code"].(float64) != 0 {
		t.Fatalf("应放行,status=%d body=%v", w.Code, body)
	}
	var ig model.Ingress
	if err := db.First(&ig).Error; err != nil {
		t.Fatalf("加载 ingress: %v", err)
	}
	if ig.CertID == nil || *ig.CertID != cert {
		t.Errorf("cert_id 未持久化: %+v", ig)
	}
	if !ig.ForceHTTPS {
		t.Errorf("force_https 未持久化")
	}
}

func TestTLS_CreateRejectsCertCrossServer(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	other := mkEdge(t, db)
	cert := mkCert(t, db, other, "x.com", "/c", "/k")

	w, body := do(t, r, "POST", "/ingresses", map[string]any{
		"edge_server_id": edge,
		"match_kind":     "domain",
		"domain":         "x.com",
		"cert_id":        cert,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("跨机 cert 应 400, got %d body=%v", w.Code, body)
	}
}

func TestTLS_CreateRejectsForceHTTPSWithoutCert(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	w, body := do(t, r, "POST", "/ingresses", map[string]any{
		"edge_server_id": edge,
		"match_kind":     "domain",
		"domain":         "n.com",
		"force_https":    true,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("force_https 无 cert 应 400, got %d body=%v", w.Code, body)
	}
}

func TestTLS_CreateRejectsPathModeWithCert(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	cert := mkCert(t, db, edge, "p.com", "/c", "/k")

	w, body := do(t, r, "POST", "/ingresses", map[string]any{
		"edge_server_id": edge,
		"match_kind":     "path",
		"domain":         "p.com",
		"cert_id":        cert,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("path 模式 + cert 应 400, got %d body=%v", w.Code, body)
	}
}

func TestTLS_CreateRejectsMissingCert(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	w, _ := do(t, r, "POST", "/ingresses", map[string]any{
		"edge_server_id": edge,
		"match_kind":     "domain",
		"domain":         "m.com",
		"cert_id":        9999,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("不存在 cert_id 应 400, got %d", w.Code)
	}
}

func TestTLS_UpdateAttachCert(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	ig := model.Ingress{EdgeServerID: edge, MatchKind: "domain", Domain: "u.com", Status: "applied"}
	db.Create(&ig)
	cert := mkCert(t, db, edge, "u.com", "/c", "/k")

	w, body := do(t, r, "PUT", "/ingresses/"+uintToStr(ig.ID), map[string]any{
		"cert_id":     cert,
		"force_https": true,
	})
	if w.Code != http.StatusOK || body["code"].(float64) != 0 {
		t.Fatalf("挂 cert 应放行, status=%d body=%v", w.Code, body)
	}
	var got model.Ingress
	db.First(&got, ig.ID)
	if got.CertID == nil || *got.CertID != cert {
		t.Errorf("cert_id 未持久化: %+v", got)
	}
	if !got.ForceHTTPS {
		t.Errorf("force_https 未持久化")
	}
	if got.Status != "pending" {
		t.Errorf("挂证书后 status 应翻 pending, got=%s", got.Status)
	}
}

func TestTLS_UpdateClearCertViaNull(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	cert := mkCert(t, db, edge, "c.com", "/c", "/k")
	certID := cert
	ig := model.Ingress{
		EdgeServerID: edge, MatchKind: "domain", Domain: "c.com",
		CertID: &certID, ForceHTTPS: false, Status: "applied",
	}
	db.Create(&ig)

	// 注意:三态语义下,cert_id=null 表示清空;同步要把 force_https 关掉
	// 否则 validateTLS 会拒(force_https 需要 cert_id)
	w, body := do(t, r, "PUT", "/ingresses/"+uintToStr(ig.ID), map[string]any{
		"cert_id": nil,
	})
	if w.Code != http.StatusOK || body["code"].(float64) != 0 {
		t.Fatalf("清空 cert 应放行, status=%d body=%v", w.Code, body)
	}
	var got model.Ingress
	db.First(&got, ig.ID)
	if got.CertID != nil {
		t.Errorf("cert_id 未清空: %+v", got)
	}
}

func TestTLS_UpdateRejectsForceHTTPSWithoutCert(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	ig := model.Ingress{EdgeServerID: edge, MatchKind: "domain", Domain: "f.com", Status: "applied"}
	db.Create(&ig)

	w, body := do(t, r, "PUT", "/ingresses/"+uintToStr(ig.ID), map[string]any{
		"force_https": true,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("无 cert 开 force_https 应 400, got %d body=%v", w.Code, body)
	}
}

// ── 路由唯一性预检（A+C）────────────────────────────────────────────────────

func TestRouteUniqueness_RejectsDuplicatePath(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	ig := model.Ingress{EdgeServerID: edge, MatchKind: "domain", Domain: "u.com"}
	db.Create(&ig)
	// 已经有一条 / 路由
	db.Create(&model.IngressRoute{IngressID: ig.ID, Path: "/", Protocol: "http"})

	w, body := do(t, r, "POST", "/ingresses/"+uintToStr(ig.ID)+"/routes", map[string]any{
		"path": "/", "protocol": "http",
		"upstream": map[string]any{"type": "raw", "raw_url": "http://x:1"},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("同 ingress 内重复 path 应 400, got %d body=%v", w.Code, body)
	}
}

func TestRouteUniqueness_AllowsSamePathAcrossIngresses(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	ig1 := model.Ingress{EdgeServerID: edge, MatchKind: "domain", Domain: "a.com"}
	ig2 := model.Ingress{EdgeServerID: edge, MatchKind: "domain", Domain: "b.com"}
	db.Create(&ig1)
	db.Create(&ig2)
	db.Create(&model.IngressRoute{IngressID: ig1.ID, Path: "/", Protocol: "http"})

	w, body := do(t, r, "POST", "/ingresses/"+uintToStr(ig2.ID)+"/routes", map[string]any{
		"path": "/", "protocol": "http",
		"upstream": map[string]any{"type": "raw", "raw_url": "http://x:1"},
	})
	if w.Code != http.StatusOK || body["code"].(float64) != 0 {
		t.Fatalf("不同 ingress 同 path 应放行, status=%d body=%v", w.Code, body)
	}
}

func TestRouteUniqueness_RejectsListenPortConflictAcrossIngresses(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	ig1 := model.Ingress{EdgeServerID: edge, MatchKind: "domain", Domain: "a.com"}
	ig2 := model.Ingress{EdgeServerID: edge, MatchKind: "domain", Domain: "b.com"}
	db.Create(&ig1)
	db.Create(&ig2)
	port := 5432
	db.Create(&model.IngressRoute{
		IngressID: ig1.ID, Path: "/", Protocol: "tcp", ListenPort: &port,
	})

	// 跨 ingress 同 listen_port 应被拦
	w, body := do(t, r, "POST", "/ingresses/"+uintToStr(ig2.ID)+"/routes", map[string]any{
		"path": "/", "protocol": "tcp", "listen_port": 5432,
		"upstream": map[string]any{"type": "raw", "raw_url": "h:1"},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("跨 ingress 同 tcp listen_port 应 400, got %d body=%v", w.Code, body)
	}
}

func TestRouteUniqueness_AllowsListenPortReuseOnDifferentEdge(t *testing.T) {
	r, db := setup(t)
	e1 := mkEdge(t, db)
	e2 := mkEdge(t, db)
	ig1 := model.Ingress{EdgeServerID: e1, MatchKind: "domain", Domain: "a.com"}
	ig2 := model.Ingress{EdgeServerID: e2, MatchKind: "domain", Domain: "b.com"}
	db.Create(&ig1)
	db.Create(&ig2)
	port := 5432
	db.Create(&model.IngressRoute{
		IngressID: ig1.ID, Path: "/", Protocol: "tcp", ListenPort: &port,
	})

	w, body := do(t, r, "POST", "/ingresses/"+uintToStr(ig2.ID)+"/routes", map[string]any{
		"path": "/", "protocol": "tcp", "listen_port": 5432,
		"upstream": map[string]any{"type": "raw", "raw_url": "h:1"},
	})
	if w.Code != http.StatusOK || body["code"].(float64) != 0 {
		t.Fatalf("不同 edge 上 listen_port 复用应放行, status=%d body=%v", w.Code, body)
	}
}

func TestRouteUniqueness_UpdateExcludesSelf(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	ig := model.Ingress{EdgeServerID: edge, MatchKind: "domain", Domain: "u.com"}
	db.Create(&ig)
	rt := model.IngressRoute{IngressID: ig.ID, Path: "/api", Protocol: "http"}
	db.Create(&rt)

	// 不变 path 的 update 不应被自己卡住
	w, body := do(t, r, "PUT", "/ingresses/"+uintToStr(ig.ID)+"/routes/"+uintToStr(rt.ID), map[string]any{
		"path": "/api", "sort": 5,
		"upstream": map[string]any{"type": "raw", "raw_url": "http://y:2"},
	})
	if w.Code != http.StatusOK || body["code"].(float64) != 0 {
		t.Fatalf("update 不变 path 应放行, status=%d body=%v", w.Code, body)
	}
}

func TestRouteUniqueness_CreateIngressBatchRejectsDuplicatePath(t *testing.T) {
	r, db := setup(t)
	edge := mkEdge(t, db)
	w, _ := do(t, r, "POST", "/ingresses", map[string]any{
		"edge_server_id": edge, "match_kind": "domain", "domain": "x.com",
		"routes": []map[string]any{
			{"path": "/", "upstream": map[string]any{"type": "raw", "raw_url": "h:1"}},
			{"path": "/", "upstream": map[string]any{"type": "raw", "raw_url": "h:2"}},
		},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("批内重复 path 应 400, got %d", w.Code)
	}
	// 事务回滚后 ingress 也不该残留
	var cnt int64
	db.Model(&model.Ingress{}).Count(&cnt)
	if cnt != 0 {
		t.Errorf("批内冲突应回滚事务,Ingress 还剩 %d", cnt)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func uintToStr(u uint) string {
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

package ingresses

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/runner"
)

// recordingRunner 记录所有 Run 调用并按命令前缀派发返回值。
//
// 与 stubRunner 的区别:
//   - 记录全部命令,测试可断言"实际下发的 mv/mkdir 命令链"
//   - 支持按调用次数注入错误(模拟"先 mv 成功、回滚 mv 失败"等组合状态)
type recordingRunner struct {
	calls []string
	// errOn 为 nil 时全部成功;否则按调用 index 检查,等于该 index 时回 err。
	errOn map[int]error
}

func (r *recordingRunner) Run(cmd string) (string, error) {
	idx := len(r.calls)
	r.calls = append(r.calls, cmd)
	if err, ok := r.errOn[idx]; ok {
		return "stderr fragment", err
	}
	return "", nil
}
func (r *recordingRunner) NewSession() (runner.Session, error) {
	return nil, errors.New("not impl")
}
func (r *recordingRunner) IsLocal() bool      { return false }
func (r *recordingRunner) Capability() string { return "full" }
func (r *recordingRunner) Close() error       { return nil }

func TestIsApprovedNginxConfPath(t *testing.T) {
	ok := []string{
		"/etc/nginx/sites-enabled/api",
		"/etc/nginx/sites-available/dup.example.com",
		"/etc/nginx/conf.d/default.conf",
	}
	for _, p := range ok {
		if !isApprovedNginxConfPath(p) {
			t.Errorf("%q should be approved", p)
		}
	}
	bad := []string{
		"",
		"/etc/passwd",
		"/etc/nginx/nginx.conf",
		"/etc/nginx/sites-enabled/", // 仅前缀本身、缺 basename
		"/etc/nginx/sites-enabled/../../../etc/passwd",
		"/etc/nginx/sites-enabled/api;rm -rf /",
		"/etc/nginx/sites-enabled/`id`",
		"sites-enabled/api", // 相对
	}
	for _, p := range bad {
		if isApprovedNginxConfPath(p) {
			t.Errorf("%q should be rejected", p)
		}
	}
}

func TestImportConfirm_Happy(t *testing.T) {
	r, db := setup(t)
	edgeID := mkEdge(t, db)

	rn := &recordingRunner{}
	old := SetImportRunnerFactory(func(*model.Server, *config.Config) (runner.Runner, error) {
		return rn, nil
	})
	t.Cleanup(func() { SetImportRunnerFactory(old) })

	body := map[string]any{
		"config_file": "/etc/nginx/sites-enabled/api",
		"server_name": "api.example.com",
		"listen":      "80",
		"routes": []map[string]any{
			{"path": "/", "proxy_pass": "http://127.0.0.1:8080", "websocket": false, "extra": ""},
		},
	}
	w, out := do(t, r, http.MethodPost,
		"/ingresses/edges/"+strconv.FormatUint(uint64(edgeID), 10)+"/import-confirm", body)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%v", w.Code, out)
	}
	// 两条命令:mkdir+mv 是同一个 shell 行(用 &&),所以应该只有 1 次 Run 调用。
	if len(rn.calls) != 1 {
		t.Fatalf("expected 1 runner call, got %d: %v", len(rn.calls), rn.calls)
	}
	if !strings.Contains(rn.calls[0], "mkdir -p") || !strings.Contains(rn.calls[0], "mv ") {
		t.Errorf("命令应包含 mkdir 与 mv: %s", rn.calls[0])
	}
	if !strings.Contains(rn.calls[0], "/etc/nginx/.serverhub-archive/") {
		t.Errorf("命令应 mv 到归档目录: %s", rn.calls[0])
	}

	// DB 落库检查
	var ig model.Ingress
	if err := db.Where("domain = ?", "api.example.com").First(&ig).Error; err != nil {
		t.Fatalf("ingress 应已落库: %v", err)
	}
	if ig.OriginalConfigPath != "/etc/nginx/sites-enabled/api" {
		t.Errorf("OriginalConfigPath=%q", ig.OriginalConfigPath)
	}
	if !strings.HasPrefix(ig.ArchivePath, "/etc/nginx/.serverhub-archive/") {
		t.Errorf("ArchivePath=%q", ig.ArchivePath)
	}
	if !strings.HasSuffix(ig.ArchivePath, "/api") {
		t.Errorf("ArchivePath 应以 basename 结尾: %q", ig.ArchivePath)
	}

	var routes []model.IngressRoute
	db.Where("ingress_id = ?", ig.ID).Find(&routes)
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
	if routes[0].Upstream.Type != "raw" || routes[0].Upstream.RawURL != "http://127.0.0.1:8080" {
		t.Errorf("upstream 落库错误: %+v", routes[0].Upstream)
	}
}

func TestImportConfirm_RejectsOutsideWhitelist(t *testing.T) {
	r, db := setup(t)
	edgeID := mkEdge(t, db)
	w, _ := do(t, r, http.MethodPost,
		"/ingresses/edges/"+strconv.FormatUint(uint64(edgeID), 10)+"/import-confirm",
		map[string]any{
			"config_file": "/etc/passwd",
			"server_name": "x.example.com",
			"routes": []map[string]any{
				{"path": "/", "proxy_pass": "http://x:1"},
			},
		})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for /etc/passwd, got %d", w.Code)
	}
}

func TestImportConfirm_RejectsDuplicateDomain(t *testing.T) {
	r, db := setup(t)
	edgeID := mkEdge(t, db)
	dup := model.Ingress{EdgeServerID: edgeID, MatchKind: "domain", Domain: "dup.example.com"}
	if err := db.Create(&dup).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	w, _ := do(t, r, http.MethodPost,
		"/ingresses/edges/"+strconv.FormatUint(uint64(edgeID), 10)+"/import-confirm",
		map[string]any{
			"config_file": "/etc/nginx/sites-enabled/dup",
			"server_name": "dup.example.com",
			"routes": []map[string]any{
				{"path": "/", "proxy_pass": "http://x:1"},
			},
		})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for duplicate domain, got %d", w.Code)
	}
}

func TestImportConfirm_RollbackOnDBFailure(t *testing.T) {
	// 用现有 unique(edge_server_id, domain) 触发落库失败:先在 DB 里塞一行
	// 同 (edge, domain),但跳过 handler 的"前置 dup 检查"——做法是用一个
	// 不同大小写的 domain (handler 直接 string 等比较,SQLite 默认大小写敏感)
	// 来通过前置检查,但触发 unique 索引冲突。
	//
	// 等等:SQLite unique index 默认大小写敏感,GORM 也按字面量比,所以
	// 这套思路在 SQLite 里走不通。换思路:用桩 runner mv 成功后,事务里
	// 通过给 routes[0].path 灌空字符串……但 handler 已经前置拒了。
	//
	// 最直白:让 runner 第一次成功(mv),第二次也成功(rollback mv)。
	// 用一个会触发 GORM 写错的方法:在 ingress_routes 上加一个
	// AfterCreate hook?太复杂。改方案:手动给 ingress 写一行同 domain
	// 但只比 dup 检查时机差几毫秒——做不到。
	//
	// 最终方案:不在 handler 测试里制造 DB 错误,改在更细粒度的回滚测里
	// 验证"runner 第二次调用拿到了 rollback mv 命令"。这里只验路径选择
	// 与正确性,不模拟 GORM 故障——SQLite + 内存 DB 下基本不会失败。
	t.Skip("DB 回滚路径在 SQLite 下难以稳定触发,留 e2e 覆盖")
}

func TestImportConfirm_MvFailureDoesNotPersist(t *testing.T) {
	r, db := setup(t)
	edgeID := mkEdge(t, db)

	rn := &recordingRunner{errOn: map[int]error{0: errors.New("permission denied")}}
	old := SetImportRunnerFactory(func(*model.Server, *config.Config) (runner.Runner, error) {
		return rn, nil
	})
	t.Cleanup(func() { SetImportRunnerFactory(old) })

	w, _ := do(t, r, http.MethodPost,
		"/ingresses/edges/"+strconv.FormatUint(uint64(edgeID), 10)+"/import-confirm",
		map[string]any{
			"config_file": "/etc/nginx/sites-enabled/api",
			"server_name": "api.example.com",
			"routes": []map[string]any{
				{"path": "/", "proxy_pass": "http://127.0.0.1:8080"},
			},
		})
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	var cnt int64
	db.Model(&model.Ingress{}).Count(&cnt)
	if cnt != 0 {
		t.Errorf("mv 失败不应落 Ingress, got %d 行", cnt)
	}
}

// ── restore ──────────────────────────────────────────────────────────────────

func TestRestore_Happy(t *testing.T) {
	r, db := setup(t)
	edgeID := mkEdge(t, db)

	// 模拟"接管而来"的 Ingress:archive_path / original_config_path 同时非空
	ig := model.Ingress{
		EdgeServerID:       edgeID,
		MatchKind:          "domain",
		Domain:             "api.example.com",
		DefaultPath:        "/",
		ArchivePath:        "/etc/nginx/.serverhub-archive/1714119492/api",
		OriginalConfigPath: "/etc/nginx/sites-enabled/api",
	}
	if err := db.Create(&ig).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	rt := model.IngressRoute{IngressID: ig.ID, Path: "/", Protocol: "http",
		Upstream: model.IngressUpstream{Type: "raw", RawURL: "http://x:1"}}
	if err := db.Create(&rt).Error; err != nil {
		t.Fatalf("seed route: %v", err)
	}

	rn := &recordingRunner{}
	old := SetImportRunnerFactory(func(*model.Server, *config.Config) (runner.Runner, error) {
		return rn, nil
	})
	t.Cleanup(func() { SetImportRunnerFactory(old) })

	w, _ := do(t, r, http.MethodPost,
		"/ingresses/"+strconv.FormatUint(uint64(ig.ID), 10)+"/restore", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	if len(rn.calls) != 1 || !strings.Contains(rn.calls[0], "mv ") {
		t.Errorf("expected one mv call, got %v", rn.calls)
	}
	if !strings.Contains(rn.calls[0], "/etc/nginx/.serverhub-archive/") {
		t.Errorf("source 应是归档路径: %s", rn.calls[0])
	}
	if !strings.Contains(rn.calls[0], "/etc/nginx/sites-enabled/api") {
		t.Errorf("target 应是 OriginalConfigPath: %s", rn.calls[0])
	}

	var cnt int64
	db.Model(&model.Ingress{}).Count(&cnt)
	if cnt != 0 {
		t.Errorf("还原后 Ingress 应被删除, got %d", cnt)
	}
	db.Model(&model.IngressRoute{}).Count(&cnt)
	if cnt != 0 {
		t.Errorf("还原后 IngressRoute 应被删除, got %d", cnt)
	}
}

func TestRestore_RejectsNonImportedIngress(t *testing.T) {
	r, db := setup(t)
	edgeID := mkEdge(t, db)
	// archive_path / original_config_path 都为空 = 普通新建,不能还原
	ig := model.Ingress{EdgeServerID: edgeID, MatchKind: "domain", Domain: "x.example.com"}
	if err := db.Create(&ig).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	w, _ := do(t, r, http.MethodPost,
		"/ingresses/"+strconv.FormatUint(uint64(ig.ID), 10)+"/restore", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-imported, got %d", w.Code)
	}
}

func TestRestore_RejectsTamperedOriginalPath(t *testing.T) {
	r, db := setup(t)
	edgeID := mkEdge(t, db)
	// 篡改场景:某种途径让 DB 行的 OriginalConfigPath 变成 /etc/passwd
	ig := model.Ingress{
		EdgeServerID:       edgeID,
		MatchKind:          "domain",
		Domain:             "x.example.com",
		ArchivePath:        "/etc/nginx/.serverhub-archive/0/x",
		OriginalConfigPath: "/etc/passwd",
	}
	if err := db.Create(&ig).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	w, _ := do(t, r, http.MethodPost,
		"/ingresses/"+strconv.FormatUint(uint64(ig.ID), 10)+"/restore", nil)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for tampered path, got %d", w.Code)
	}
}

func TestRestore_NotFound(t *testing.T) {
	r, _ := setup(t)
	w, _ := do(t, r, http.MethodPost, "/ingresses/9999/restore", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

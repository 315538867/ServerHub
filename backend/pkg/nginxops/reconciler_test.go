package nginxops

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/nginxrender"
	"github.com/serverhub/serverhub/pkg/runner"
	"github.com/serverhub/serverhub/pkg/sysinfo"
)

// newTestDB 创建一个内存 sqlite，AutoMigrate Reconciler 涉及的模型。
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Server{}, &model.Service{}, &model.Application{},
		&model.Ingress{}, &model.IngressRoute{},
		&model.AuditApply{}, &model.NginxCert{}, &model.NginxProfile{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// installFakeRunner 暂时把 defaultRunnerFactory 替换为返回 fake；测试结束恢复。
func installFakeRunner(t *testing.T, fr *fakeRunner) {
	t.Helper()
	old := SetRunnerFactory(func(*model.Server, *config.Config) (runner.Runner, error) { return fr, nil })
	t.Cleanup(func() { SetRunnerFactory(old) })
}

// inspectLine 帮单测拼一个 inspect 输出行。
func inspectLine(path, content string) string {
	return path + "\t" + hashHex(content) + "\t" + base64.StdEncoding.EncodeToString([]byte(content))
}

func TestApply_NoOp_OnEmptyEdge(t *testing.T) {
	db := newTestDB(t)
	edge := model.Server{Name: "e", Type: "ssh", Host: "h", Port: 22, Username: "x"}
	if err := db.Create(&edge).Error; err != nil {
		t.Fatal(err)
	}

	fr := newFakeRunner()
	fr.onContains("base64", "", nil)            // inspect 返回空
	fr.onContains("nginx -t", "ok", nil)        // 不会被调用，但放着无副作用
	fr.onContains("tar -C '/etc/nginx'", "", nil) // snapshot
	installFakeRunner(t, fr)

	res, err := Apply(context.Background(), db, &config.Config{}, edge.ID, nil)
	if err != nil {
		t.Fatalf("apply err: %v", err)
	}
	if !res.NoOp || len(res.Changes) != 0 {
		t.Errorf("空 edge 应 NoOp，got %+v", res)
	}
	if res.AuditID == 0 {
		t.Errorf("应已写入 audit_apply 占位")
	}
}

func TestApply_AddAndReload(t *testing.T) {
	db := newTestDB(t)
	// edge + 一个 service 在同 server 上（loopback 短路，避免 netresolve 失败）
	edge := model.Server{Name: "edge", Type: "ssh", Host: "h"}
	db.Create(&edge)
	svc := model.Service{Name: "app", ServerID: edge.ID, ExposedPort: 8080}
	db.Create(&svc)

	ig := model.Ingress{
		EdgeServerID: edge.ID,
		MatchKind:    nginxrender.MatchKindDomain,
		Domain:       "demo.example.com",
	}
	db.Create(&ig)
	rt := model.IngressRoute{
		IngressID: ig.ID, Path: "/",
		Upstream: model.IngressUpstream{Type: "service", ServiceID: &svc.ID},
	}
	db.Create(&rt)

	fr := newFakeRunner()
	fr.onContains("base64", "", nil)            // inspect: 远端无文件
	fr.onContains("tar -C '/etc/nginx'", "", nil) // snapshot
	fr.onContains("nginx -t", "syntax ok", nil)
	fr.onContains("nginx -s reload", "", nil)
	// mkdir / WriteRemoteFile / find / ln -sf 都默认成功
	installFakeRunner(t, fr)

	res, err := Apply(context.Background(), db, &config.Config{}, edge.ID, nil)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.NoOp {
		t.Fatalf("有 ingress 不应 NoOp")
	}
	if len(res.Changes) != 1 || res.Changes[0].Kind != ChangeAdd {
		t.Errorf("want 1 add, got %+v", res.Changes)
	}
	wantPath := "/etc/nginx/sites-available/demo_example_com-sh.conf"
	if res.Changes[0].Path != wantPath {
		t.Errorf("path: %q want %q", res.Changes[0].Path, wantPath)
	}

	// audit 字段已回填
	var audit model.AuditApply
	db.First(&audit, res.AuditID)
	if audit.RolledBack {
		t.Errorf("不应 rollback")
	}
	if audit.NginxTOutput == "" {
		t.Errorf("audit 未回填 nginx -t 输出")
	}

	// ingress.status 标记为 applied
	var got model.Ingress
	db.First(&got, ig.ID)
	if got.Status != "applied" || got.LastAppliedAt == nil {
		t.Errorf("ingress 状态未更新: %+v", got)
	}

	// 调用序列里应有 ln -sf 创建 symlink
	hasLn := false
	for _, c := range fr.calls {
		if strings.Contains(c, "ln -sf") {
			hasLn = true
			break
		}
	}
	if !hasLn {
		t.Errorf("缺少 sites-enabled 链接创建")
	}
}

func TestApply_NginxTFails_RollsBack(t *testing.T) {
	db := newTestDB(t)
	edge := model.Server{Name: "edge", Type: "ssh"}
	db.Create(&edge)
	svc := model.Service{Name: "app", ServerID: edge.ID, ExposedPort: 80}
	db.Create(&svc)
	ig := model.Ingress{EdgeServerID: edge.ID, MatchKind: nginxrender.MatchKindDomain, Domain: "x.com"}
	db.Create(&ig)
	rt := model.IngressRoute{IngressID: ig.ID, Path: "/", Upstream: model.IngressUpstream{Type: "service", ServiceID: &svc.ID}, Extra: "boom"}
	db.Create(&rt)

	fr := newFakeRunner()
	fr.onContains("base64", "", nil)
	fr.onContains("tar -C '/etc/nginx'", "", nil)
	fr.onContains("nginx -t", "test failed: bad", &fakeErr{"exit 1"})
	installFakeRunner(t, fr)

	res, err := Apply(context.Background(), db, &config.Config{}, edge.ID, nil)
	if err == nil {
		t.Fatal("应返回 nginx -t 错误")
	}
	if !res.RolledBack {
		t.Errorf("应标记 RolledBack")
	}
	// rollback 路径里会执行 rm 反向回滚已新增文件
	hasRm := false
	for _, c := range fr.calls {
		if strings.Contains(c, "rm -f") && strings.Contains(c, "x_com-sh.conf") {
			hasRm = true
			break
		}
	}
	if !hasRm {
		t.Errorf("rollback 未执行 rm 撤销新增")
	}
	var audit model.AuditApply
	db.First(&audit, res.AuditID)
	if !audit.RolledBack {
		t.Errorf("audit.RolledBack 未回填")
	}
}

// nginx -t 失败时,rollback 后应把 status=pending 的 ingress 翻 broken,
// applied 的不动。让 UI 上 broken 状态实际可见。
func TestApply_NginxTFails_MarksBrokenForPending(t *testing.T) {
	db := newTestDB(t)
	edge := model.Server{Name: "edge", Type: "ssh"}
	db.Create(&edge)
	svc := model.Service{Name: "app", ServerID: edge.ID, ExposedPort: 80}
	db.Create(&svc)
	// 用户改完待应用：pending
	pendingIg := model.Ingress{
		EdgeServerID: edge.ID, MatchKind: nginxrender.MatchKindDomain,
		Domain: "boom.com", Status: "pending",
	}
	db.Create(&pendingIg)
	db.Create(&model.IngressRoute{
		IngressID: pendingIg.ID, Path: "/",
		Upstream: model.IngressUpstream{Type: "service", ServiceID: &svc.ID}, Extra: "boom",
	})
	// 旁观者:已应用且不在本次变更里
	stableIg := model.Ingress{
		EdgeServerID: edge.ID, MatchKind: nginxrender.MatchKindDomain,
		Domain: "stable.com", Status: "applied",
	}
	db.Create(&stableIg)

	fr := newFakeRunner()
	fr.onContains("base64", "", nil)
	fr.onContains("tar -C '/etc/nginx'", "", nil)
	fr.onContains("nginx -t", "test failed", &fakeErr{"exit 1"})
	installFakeRunner(t, fr)

	res, err := Apply(context.Background(), db, &config.Config{}, edge.ID, nil)
	if err == nil {
		t.Fatal("应返回 nginx -t 错误")
	}
	if !res.RolledBack {
		t.Errorf("应标记 RolledBack")
	}
	// 各取一条新结构体,避免 db.First 复用同一结构体时旧字段残留导致误判。
	var gotPending model.Ingress
	if err := db.First(&gotPending, pendingIg.ID).Error; err != nil {
		t.Fatalf("reload pending: %v", err)
	}
	if gotPending.Status != "broken" {
		t.Errorf("pending ingress 应被翻 broken,得 %q", gotPending.Status)
	}
	var gotStable model.Ingress
	if err := db.First(&gotStable, stableIg.ID).Error; err != nil {
		t.Fatalf("reload stable: %v", err)
	}
	if gotStable.Status != "applied" {
		t.Errorf("已 applied 的旁观 ingress 不应被波及,得 %q", gotStable.Status)
	}
}

func TestApply_RejectsNonFullCapability(t *testing.T) {
	db := newTestDB(t)
	edge := model.Server{Name: "edge", Type: "local"}
	db.Create(&edge)

	fr := newFakeRunner()
	fr.cap = sysinfo.CapDocker
	installFakeRunner(t, fr)

	_, err := Apply(context.Background(), db, &config.Config{}, edge.ID, nil)
	if err == nil || !strings.Contains(err.Error(), "capability") {
		t.Fatalf("应拒绝非 full capability，err=%v", err)
	}
}

func TestApply_DryRun_DoesNotWrite(t *testing.T) {
	db := newTestDB(t)
	edge := model.Server{Name: "e"}
	db.Create(&edge)
	svc := model.Service{Name: "s", ServerID: edge.ID, ExposedPort: 80}
	db.Create(&svc)
	ig := model.Ingress{EdgeServerID: edge.ID, MatchKind: nginxrender.MatchKindDomain, Domain: "d.com"}
	db.Create(&ig)
	rt := model.IngressRoute{IngressID: ig.ID, Path: "/", Upstream: model.IngressUpstream{Type: "service", ServiceID: &svc.ID}}
	db.Create(&rt)

	fr := newFakeRunner()
	fr.onContains("base64", "", nil)
	installFakeRunner(t, fr)

	changes, err := DryRun(context.Background(), db, &config.Config{}, edge.ID)
	if err != nil {
		t.Fatalf("dryrun: %v", err)
	}
	if len(changes) != 1 || changes[0].Kind != ChangeAdd {
		t.Errorf("want 1 add, got %+v", changes)
	}
	for _, c := range fr.calls {
		if strings.Contains(c, "tee") || strings.Contains(c, "tar -C") {
			t.Errorf("DryRun 不应写盘 / 备份: %s", c)
		}
	}
	// 副作用:有差异时 ingress.status 应翻成 drift,且 last_applied_at 不动
	var got model.Ingress
	db.First(&got, ig.ID)
	if got.Status != "drift" {
		t.Errorf("DryRun 检出差异时应回写 status=drift,实得 %q", got.Status)
	}
	if got.LastAppliedAt != nil {
		t.Errorf("drift 时 last_applied_at 不应被刷新")
	}
}

// 没有差异时,DryRun 应把 status 校准回 applied(以防之前是 drift/pending)。
// 这样"扫描漂移"按钮可以双向同步状态字段,而不是只在 Apply 后才有 applied。
func TestDryRun_InSync_FlipsStatusToApplied(t *testing.T) {
	db := newTestDB(t)
	edge := model.Server{Name: "e"}
	db.Create(&edge)
	// 没有 ingress 时 desired 为空,远端也无文件 → 0 changes
	ig := model.Ingress{
		EdgeServerID: edge.ID, MatchKind: nginxrender.MatchKindDomain,
		Domain: "noop.com", Status: "drift", // 故意先标 drift
	}
	db.Create(&ig)
	// 删掉路由让 desired 为空,避免命中渲染产生 add
	db.Where("ingress_id = ?", ig.ID).Delete(&model.IngressRoute{})

	fr := newFakeRunner()
	fr.onContains("base64", "", nil) // 远端也是空
	installFakeRunner(t, fr)

	changes, err := DryRun(context.Background(), db, &config.Config{}, edge.ID)
	if err != nil {
		t.Fatalf("dryrun: %v", err)
	}
	if len(changes) != 0 {
		t.Errorf("空 desired + 空 actual 应 0 changes,得 %+v", changes)
	}
	var got model.Ingress
	db.First(&got, ig.ID)
	if got.Status != "applied" {
		t.Errorf("无差异时应翻成 applied,实得 %q", got.Status)
	}
	if got.LastAppliedAt == nil {
		t.Errorf("applied 状态应同步 last_applied_at")
	}
}

func TestLoadDesired_RawUpstream(t *testing.T) {
	db := newTestDB(t)
	edge := model.Server{Name: "e"}
	db.Create(&edge)
	ig := model.Ingress{EdgeServerID: edge.ID, MatchKind: nginxrender.MatchKindPath, Domain: "shared.com"}
	db.Create(&ig)
	rt := model.IngressRoute{
		IngressID: ig.ID, Path: "/api",
		Upstream: model.IngressUpstream{Type: "raw", RawURL: "http://outside:9000"},
	}
	db.Create(&rt)

	got, err := LoadDesired(db, &edge, "", nginxrender.DefaultProfile())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 1 || len(got[0].Routes) != 1 || got[0].Routes[0].UpstreamURL != "http://outside:9000" {
		t.Errorf("raw upstream 未透传：%+v", got)
	}
	// fileStem: path 模式应携带 ingress.id
	wantStem := "shared_com-" + itoa(ig.ID)
	if got[0].FileStem != wantStem {
		t.Errorf("FileStem=%q want %q", got[0].FileStem, wantStem)
	}
}

func TestLoadDesired_ServiceMissingPortErrors(t *testing.T) {
	db := newTestDB(t)
	edge := model.Server{Name: "e"}
	db.Create(&edge)
	svc := model.Service{Name: "noport", ServerID: edge.ID, ExposedPort: 0}
	db.Create(&svc)
	ig := model.Ingress{EdgeServerID: edge.ID, MatchKind: nginxrender.MatchKindDomain, Domain: "x.com"}
	db.Create(&ig)
	rt := model.IngressRoute{IngressID: ig.ID, Path: "/", Upstream: model.IngressUpstream{Type: "service", ServiceID: &svc.ID}}
	db.Create(&rt)

	if _, err := LoadDesired(db, &edge, "", nginxrender.DefaultProfile()); err == nil {
		t.Fatal("无 exposed_port 应报错")
	}
}

func TestInspect_ParsesRunnerOutput(t *testing.T) {
	fr := newFakeRunner()
	fr.onContains("base64", inspectLine("/etc/nginx/sites-available/foo-sh.conf", "C1")+"\n"+
		inspectLine("/etc/nginx/app-locations/bar.conf", "C2")+"\n", nil)

	got, err := Inspect(fr)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if got["/etc/nginx/sites-available/foo-sh.conf"].Content != "C1" {
		t.Errorf("foo content lost")
	}
	if got["/etc/nginx/app-locations/bar.conf"].Hash != hashHex("C2") {
		t.Errorf("bar hash mismatch")
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────

type fakeErr struct{ msg string }

func (e *fakeErr) Error() string { return e.msg }

// itoa 走标准库 strconv 即可，但为避免 import 噪音手写一个小的
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

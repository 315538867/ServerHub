package nginxops

import (
	"context"
	"strings"
	"testing"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/nginxrender"
)

func TestRestore_HappyPath(t *testing.T) {
	fr := newFakeRunner()
	if err := Restore(fr, "/var/lib/serverhub/nginx-bak/1-x.tar.gz"); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if len(fr.calls) != 1 || !strings.Contains(fr.calls[0], "tar -C /etc/nginx -xzf") {
		t.Errorf("restore 命令异常: %v", fr.calls)
	}
}

func TestRestore_RunFails(t *testing.T) {
	fr := newFakeRunner()
	fr.defErr = &fakeErr{"tar bad"}
	if err := Restore(fr, "/p"); err == nil {
		t.Fatal("应在 runner 失败时返回错误")
	}
}

func TestWriteChanges_AllKindsAndRollback(t *testing.T) {
	fr := newFakeRunner()
	changes := []Change{
		{Kind: ChangeAdd, Path: "/etc/nginx/sites-available/a-sh.conf", NewContent: "newA"},
		{Kind: ChangeUpdate, Path: "/etc/nginx/app-locations/b.conf", NewContent: "newB", OldContent: "oldB"},
		{Kind: ChangeDelete, Path: "/etc/nginx/app-locations/c.conf", OldContent: "oldC"},
	}
	if err := writeChanges(fr, changes); err != nil {
		t.Fatalf("writeChanges: %v", err)
	}
	// Add/Update 都要写文件，Delete 要 rm
	hasAdd, hasUpd, hasDel := false, false, false
	for _, c := range fr.calls {
		switch {
		case strings.Contains(c, "a-sh.conf") && strings.Contains(c, "tee"):
			hasAdd = true
		case strings.Contains(c, "b.conf") && strings.Contains(c, "tee"):
			hasUpd = true
		case strings.Contains(c, "c.conf") && strings.Contains(c, "rm -f"):
			hasDel = true
		}
	}
	if !hasAdd || !hasUpd || !hasDel {
		t.Errorf("writeChanges 未覆盖三种 Kind: add=%v upd=%v del=%v", hasAdd, hasUpd, hasDel)
	}

	fr2 := newFakeRunner()
	if err := rollback(fr2, changes); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	// rollback 应：rm /a-sh.conf；写回 /b.conf oldB；写回 /c.conf oldC
	hasRmA, hasWriteB, hasWriteC := false, false, false
	for _, c := range fr2.calls {
		switch {
		case strings.Contains(c, "a-sh.conf") && strings.Contains(c, "rm -f"):
			hasRmA = true
		case strings.Contains(c, "b.conf") && strings.Contains(c, "tee"):
			hasWriteB = true
		case strings.Contains(c, "c.conf") && strings.Contains(c, "tee"):
			hasWriteC = true
		}
	}
	if !hasRmA || !hasWriteB || !hasWriteC {
		t.Errorf("rollback 反向操作不全: rmA=%v writeB=%v writeC=%v", hasRmA, hasWriteB, hasWriteC)
	}
}

func TestRollback_AggregatesErrors(t *testing.T) {
	fr := newFakeRunner()
	fr.defErr = &fakeErr{"boom"}
	err := rollback(fr, []Change{
		{Kind: ChangeAdd, Path: "/p1"},
		{Kind: ChangeUpdate, Path: "/p2", OldContent: "x"},
	})
	if err == nil || !strings.Contains(err.Error(), ";") {
		t.Fatalf("期望聚合错误，got %v", err)
	}
}

func TestApply_SnapshotFails_Aborts(t *testing.T) {
	db := newTestDB(t)
	edge := model.Server{Name: "e"}
	db.Create(&edge)

	fr := newFakeRunner()
	fr.onContains("tar -C /etc/nginx", "no space", &fakeErr{"exit 1"})
	installFakeRunner(t, fr)

	res, err := Apply(context.Background(), db, &config.Config{}, edge.ID, nil)
	if err == nil || !strings.Contains(err.Error(), "snapshot") {
		t.Fatalf("应返回 snapshot 错误，err=%v", err)
	}
	if res.AuditID == 0 {
		t.Errorf("即便 snapshot 失败也应留下 audit")
	}
}

func TestApply_ReloadFails_RollsBack(t *testing.T) {
	db := newTestDB(t)
	edge := model.Server{Name: "e"}
	db.Create(&edge)
	svc := model.Service{Name: "s", ServerID: edge.ID, ExposedPort: 80}
	db.Create(&svc)
	ig := model.Ingress{EdgeServerID: edge.ID, MatchKind: nginxrender.MatchKindDomain, Domain: "r.com"}
	db.Create(&ig)
	rt := model.IngressRoute{IngressID: ig.ID, Path: "/", Upstream: model.IngressUpstream{Type: "service", ServiceID: &svc.ID}}
	db.Create(&rt)

	fr := newFakeRunner()
	fr.onContains("base64", "", nil)
	fr.onContains("tar -C /etc/nginx", "", nil)
	fr.onContains("nginx -t", "ok", nil)
	fr.onContains("nginx -s reload", "reload broke", &fakeErr{"reload exit 1"})
	installFakeRunner(t, fr)

	res, err := Apply(context.Background(), db, &config.Config{}, edge.ID, nil)
	if err == nil {
		t.Fatal("应返回 reload 错误")
	}
	if !res.RolledBack {
		t.Errorf("应回滚")
	}
}

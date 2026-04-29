package usecase

import (
	"strings"
	"testing"

	_ "github.com/serverhub/serverhub/adapters/runtime/compose"
	_ "github.com/serverhub/serverhub/adapters/runtime/docker"
	_ "github.com/serverhub/serverhub/adapters/runtime/native"
	_ "github.com/serverhub/serverhub/adapters/runtime/static"
	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Service{}, &model.Release{}, &model.Artifact{},
		&model.EnvVarSet{}, &model.ConfigFileSet{}, &model.DeployRun{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	return db
}

func TestMapServiceTypeToKind(t *testing.T) {
	cases := []struct {
		in   domain.ServiceType
		want string
	}{
		{domain.ServiceTypeDocker, string(domain.ServiceTypeDocker)},
		{domain.ServiceTypeDockerCompose, string(domain.ServiceTypeCompose)},
		{domain.ServiceTypeNative, string(domain.ServiceTypeNative)},
		{domain.ServiceTypeStatic, string(domain.ServiceTypeStatic)},
		{domain.ServiceTypePodman, string(domain.ServiceTypePodman)},
		{"unknown", ""},
	}
	for _, c := range cases {
		if got := mapServiceTypeToKind(c.in); got != c.want {
			t.Errorf("map(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

// domainToModel 系列辅助:测试里生成 domain 对象后,用 model.FromDomain* 落库。
func svcToModel(s domain.Service) model.Service    { return model.FromDomainService(s) }
func relToModel(r domain.Release) model.Release    { return model.FromDomainRelease(r) }
func artToModel(a domain.Artifact) model.Artifact  { return model.FromDomainArtifact(a) }

// TestBuildReleaseCmd_DockerHTTP 端到端验证 buildReleaseCmd 与 v1 一致:
// docker 类型 + http artifact + 无 env / config:
//
//	bash -c '<env>set -e; mkdir; cd; <fetch>; <start>'
func TestBuildReleaseCmd_DockerHTTP(t *testing.T) {
	db := newTestDB(t)
	art := domain.Artifact{ServiceID: 1, Provider: domain.ArtifactProviderHTTP, Ref: "https://example.com/x.tar"}
	am := artToModel(art)
	if err := db.Create(&am).Error; err != nil {
		t.Fatal(err)
	}
	art.ID = am.ID

	rel := domain.Release{ServiceID: 1, ArtifactID: art.ID, StartSpec: &domain.DockerSpec{Image: "nginx:1.27"}}
	rm := relToModel(rel)
	if err := db.Create(&rm).Error; err != nil {
		t.Fatal(err)
	}
	rel.ID = rm.ID

	svc := domain.Service{ID: 1, Type: domain.ServiceTypeDocker, WorkDir: "/opt/x"}

	cmd, err := buildReleaseCmd(svc, rel, art, db, "")
	if err != nil {
		t.Fatalf("buildReleaseCmd: %v", err)
	}
	const want = `bash -c 'set -e; mkdir -p '"'"'/opt/x'"'"' && cd '"'"'/opt/x'"'"' && curl -fsSL -o '"'"'artifact.bin'"'"' '"'"'https://example.com/x.tar'"'"' && docker rm -f '"'"'serverhub-svc-1'"'"' 2>/dev/null || true; docker run -d --name '"'"'serverhub-svc-1'"'"' '"'"'nginx:1.27'"'"' 2>&1'`
	if cmd != want {
		t.Errorf("\n got: %s\nwant: %s", cmd, want)
	}
}

// TestBuildReleaseCmd_StaticDefault: static + 缺省 WorkDir 退化 /tmp/serverhub-svc-<id>
func TestBuildReleaseCmd_StaticDefault(t *testing.T) {
	db := newTestDB(t)
	art := domain.Artifact{ServiceID: 5, Provider: domain.ArtifactProviderHTTP, Ref: "https://example.com/site.zip"}
	am := artToModel(art)
	db.Create(&am)
	art.ID = am.ID

	rel := domain.Release{ServiceID: 5, ArtifactID: art.ID}
	rm := relToModel(rel)
	db.Create(&rm)
	rel.ID = rm.ID

	svc := domain.Service{ID: 5, Type: domain.ServiceTypeStatic}

	cmd, err := buildReleaseCmd(svc, rel, art, db, "")
	if err != nil {
		t.Fatalf("buildReleaseCmd: %v", err)
	}
	if !strings.Contains(cmd, "/tmp/serverhub-svc-5") {
		t.Errorf("expected default workdir in cmd, got: %s", cmd)
	}
	if !strings.Contains(cmd, "static release prepared") {
		t.Errorf("expected static start phrase, got: %s", cmd)
	}
}

// TestBuildReleaseCmd_Compose: docker-compose 类型走 compose adapter
func TestBuildReleaseCmd_Compose(t *testing.T) {
	db := newTestDB(t)
	art := domain.Artifact{ServiceID: 2, Provider: domain.ArtifactProviderHTTP, Ref: "https://example.com/x.zip"}
	am := artToModel(art)
	db.Create(&am)
	art.ID = am.ID

	rel := domain.Release{ServiceID: 2, ArtifactID: art.ID, StartSpec: &domain.ComposeSpec{FileName: "prod.yml"}}
	rm := relToModel(rel)
	db.Create(&rm)
	rel.ID = rm.ID

	svc := domain.Service{ID: 2, Type: domain.ServiceTypeDockerCompose, WorkDir: "/srv/app"}

	cmd, err := buildReleaseCmd(svc, rel, art, db, "")
	if err != nil {
		t.Fatalf("buildReleaseCmd: %v", err)
	}
	if !strings.Contains(cmd, `docker compose -f '"'"'prod.yml'"'"' up -d --build`) {
		t.Errorf("expected compose start, got: %s", cmd)
	}
}

// TestBuildReleaseCmd_ImportedRejected: imported provider 在 ApplyRelease 入口被拒,
// 但 buildReleaseCmd 仍可装(provider 校验在调用方)
func TestBuildReleaseCmd_ImportedFailsAtFetch(t *testing.T) {
	db := newTestDB(t)
	art := domain.Artifact{ServiceID: 3, Provider: domain.ArtifactProviderImported, Ref: "legacy"}
	am := artToModel(art)
	db.Create(&am)
	art.ID = am.ID

	rel := domain.Release{ServiceID: 3, ArtifactID: art.ID}
	rm := relToModel(rel)
	db.Create(&rm)
	rel.ID = rm.ID

	svc := domain.Service{ID: 3, Type: domain.ServiceTypeDocker}

	if _, err := buildReleaseCmd(svc, rel, art, db, ""); err == nil {
		t.Fatal("expected error for imported provider at fetch")
	}
}

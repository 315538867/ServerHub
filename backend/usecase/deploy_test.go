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
	cases := []struct{ in, want string }{
		{model.ServiceTypeDocker, string(domain.ServiceTypeDocker)},
		{model.ServiceTypeDockerCompose, string(domain.ServiceTypeCompose)},
		{model.ServiceTypeNative, string(domain.ServiceTypeNative)},
		{model.ServiceTypeStatic, string(domain.ServiceTypeStatic)},
		{"unknown", ""},
	}
	for _, c := range cases {
		if got := mapServiceTypeToKind(c.in); got != c.want {
			t.Errorf("map(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

// TestBuildReleaseCmd_DockerHTTP 端到端验证 buildReleaseCmd 与 v1 一致:
// docker 类型 + http artifact + 无 env / config:
//   bash -c '<env>set -e; mkdir; cd; <fetch>; <start>'
func TestBuildReleaseCmd_DockerHTTP(t *testing.T) {
	db := newTestDB(t)
	art := model.Artifact{ServiceID: 1, Provider: model.ArtifactProviderHTTP, Ref: "https://example.com/x.tar"}
	if err := db.Create(&art).Error; err != nil {
		t.Fatal(err)
	}
	rel := model.Release{ServiceID: 1, ArtifactID: art.ID, StartSpec: `{"image":"nginx:1.27"}`}
	if err := db.Create(&rel).Error; err != nil {
		t.Fatal(err)
	}
	svc := model.Service{ID: 1, Type: model.ServiceTypeDocker, WorkDir: "/opt/x"}

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
	art := model.Artifact{ServiceID: 5, Provider: model.ArtifactProviderHTTP, Ref: "https://example.com/site.zip"}
	db.Create(&art)
	rel := model.Release{ServiceID: 5, ArtifactID: art.ID}
	db.Create(&rel)
	svc := model.Service{ID: 5, Type: model.ServiceTypeStatic}

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
	art := model.Artifact{ServiceID: 2, Provider: model.ArtifactProviderHTTP, Ref: "https://example.com/x.zip"}
	db.Create(&art)
	rel := model.Release{ServiceID: 2, ArtifactID: art.ID, StartSpec: `{"file_name":"prod.yml"}`}
	db.Create(&rel)
	svc := model.Service{ID: 2, Type: model.ServiceTypeDockerCompose, WorkDir: "/srv/app"}

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
	art := model.Artifact{ServiceID: 3, Provider: model.ArtifactProviderImported, Ref: "legacy"}
	db.Create(&art)
	rel := model.Release{ServiceID: 3, ArtifactID: art.ID}
	db.Create(&rel)
	svc := model.Service{ID: 3, Type: model.ServiceTypeDocker}

	if _, err := buildReleaseCmd(svc, rel, art, db, ""); err == nil {
		t.Fatal("expected error for imported provider at fetch")
	}
}

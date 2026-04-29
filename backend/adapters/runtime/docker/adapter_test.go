package docker

import (
	"testing"

	"github.com/serverhub/serverhub/domain"
)

// TestBuildStartCmd_Golden 锁定 docker 启动命令字节级一致(对比 v1 buildStartPart)。
func TestBuildStartCmd_Golden(t *testing.T) {
	cases := []struct {
		name string
		svc  domain.Service
		rel  domain.Release
		want string
	}{
		{
			name: "image_from_startspec",
			svc:  domain.Service{ID: 7, Type: domain.ServiceTypeDocker},
			rel:  domain.Release{StartSpec: &domain.DockerSpec{Image: "nginx:1.27"}},
			want: `docker rm -f 'serverhub-svc-7' 2>/dev/null || true; docker run -d --name 'serverhub-svc-7' 'nginx:1.27' 2>&1`,
		},
		{
			name: "image_fallback_artifactref",
			svc:  domain.Service{ID: 9, Type: domain.ServiceTypeDocker},
			rel:  domain.Release{ArtifactRef: "redis:7-alpine"},
			want: `docker rm -f 'serverhub-svc-9' 2>/dev/null || true; docker run -d --name 'serverhub-svc-9' 'redis:7-alpine' 2>&1`,
		},
		{
			name: "with_cmd",
			svc:  domain.Service{ID: 12, Type: domain.ServiceTypeDocker},
			rel:  domain.Release{StartSpec: &domain.DockerSpec{Image: "alpine:3.20", Cmd: "sh -c 'echo hi'"}},
			want: `docker rm -f 'serverhub-svc-12' 2>/dev/null || true; docker run -d --name 'serverhub-svc-12' 'alpine:3.20' sh -c 'echo hi' 2>&1`,
		},
	}
	a := Adapter{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := a.BuildStartCmd(&tc.svc, &tc.rel)
			if err != nil {
				t.Fatalf("BuildStartCmd: %v", err)
			}
			if got != tc.want {
				t.Errorf("\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

func TestBuildStartCmd_EmptyImage(t *testing.T) {
	a := Adapter{}
	_, err := a.BuildStartCmd(&domain.Service{ID: 1, Type: domain.ServiceTypeDocker},
		&domain.Release{StartSpec: &domain.DockerSpec{}})
	if err == nil {
		t.Fatal("expected error when image empty")
	}
}

func TestKind(t *testing.T) {
	if k := (Adapter{}).Kind(); k != "docker" {
		t.Fatalf("Kind=%q want docker", k)
	}
}

func TestPlanStart_SingleBashStep(t *testing.T) {
	a := Adapter{}
	steps, err := a.PlanStart(
		&domain.Service{ID: 1, Type: domain.ServiceTypeDocker},
		&domain.Release{StartSpec: &domain.DockerSpec{Image: "x"}},
	)
	if err != nil {
		t.Fatalf("PlanStart: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("steps=%d want 1", len(steps))
	}
	if steps[0].Name() != "docker-start" {
		t.Fatalf("step name=%q", steps[0].Name())
	}
}

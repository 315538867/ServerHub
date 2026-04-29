package podman

import (
	"testing"

	"github.com/serverhub/serverhub/domain"
)

func TestBuildStartCmd_Golden(t *testing.T) {
	a := Adapter{}
	got, err := a.BuildStartCmd(
		&domain.Service{ID: 1, Type: domain.ServiceTypePodman},
		&domain.Release{StartSpec: &domain.DockerSpec{Image: "nginx:1.27"}},
	)
	if err != nil {
		t.Fatalf("BuildStartCmd: %v", err)
	}
	want := `podman rm -f 'serverhub-svc-1' 2>/dev/null || true; podman run -d --name 'serverhub-svc-1' 'nginx:1.27' 2>&1`
	if got != want {
		t.Errorf("\n got: %q\nwant: %q", got, want)
	}
}

func TestBuildStartCmd_EmptyImage(t *testing.T) {
	a := Adapter{}
	_, err := a.BuildStartCmd(
		&domain.Service{ID: 1, Type: domain.ServiceTypePodman},
		&domain.Release{StartSpec: &domain.DockerSpec{}},
	)
	if err == nil {
		t.Fatal("expected error when image empty")
	}
}

func TestBuildStartCmd_FallbackArtifactRef(t *testing.T) {
	a := Adapter{}
	got, err := a.BuildStartCmd(
		&domain.Service{ID: 3, Type: domain.ServiceTypePodman},
		&domain.Release{ArtifactRef: "alpine:3.20"},
	)
	if err != nil {
		t.Fatalf("BuildStartCmd: %v", err)
	}
	want := `podman rm -f 'serverhub-svc-3' 2>/dev/null || true; podman run -d --name 'serverhub-svc-3' 'alpine:3.20' 2>&1`
	if got != want {
		t.Errorf("\n got: %q\nwant: %q", got, want)
	}
}

func TestKind(t *testing.T) {
	if k := (Adapter{}).Kind(); k != "podman" {
		t.Fatalf("Kind=%q want podman", k)
	}
}

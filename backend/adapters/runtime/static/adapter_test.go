package static

import (
	"testing"

	"github.com/serverhub/serverhub/domain"
)

func TestBuildStartCmd_Golden(t *testing.T) {
	got, err := (Adapter{}).BuildStartCmd(nil, &domain.Release{})
	if err != nil {
		t.Fatalf("BuildStartCmd: %v", err)
	}
	const want = `echo 'static release prepared'`
	if got != want {
		t.Errorf("\n got: %q\nwant: %q", got, want)
	}
}

func TestKind(t *testing.T) {
	if k := (Adapter{}).Kind(); k != "static" {
		t.Fatalf("Kind=%q want static", k)
	}
}

package compose

import (
	"testing"

	"github.com/serverhub/serverhub/domain"
)

func TestBuildStartCmd_Golden(t *testing.T) {
	cases := []struct {
		name string
		rel  domain.Release
		want string
	}{
		{
			name: "default_filename",
			rel:  domain.Release{StartSpec: ``},
			want: `docker compose -f 'docker-compose.yml' up -d --build 2>&1`,
		},
		{
			name: "custom_filename",
			rel:  domain.Release{StartSpec: `{"file_name":"prod.yml"}`},
			want: `docker compose -f 'prod.yml' up -d --build 2>&1`,
		},
	}
	a := Adapter{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := a.BuildStartCmd(nil, &tc.rel)
			if err != nil {
				t.Fatalf("BuildStartCmd: %v", err)
			}
			if got != tc.want {
				t.Errorf("\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

func TestKind(t *testing.T) {
	if k := (Adapter{}).Kind(); k != "compose" {
		t.Fatalf("Kind=%q want compose", k)
	}
}

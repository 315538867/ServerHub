package native

import (
	"testing"

	"github.com/serverhub/serverhub/domain"
)

func TestBuildStartCmd_Golden(t *testing.T) {
	cases := []struct {
		name    string
		rel     domain.Release
		want    string
		wantErr bool
	}{
		{
			name: "simple_cmd",
			rel:  domain.Release{StartSpec: &domain.NativeSpec{Cmd: "./start.sh"}},
			want: `./start.sh 2>&1`,
		},
		{
			name: "cmd_with_args",
			rel:  domain.Release{StartSpec: &domain.NativeSpec{Cmd: "./bin -p 8080 --foo=bar"}},
			want: `./bin -p 8080 --foo=bar 2>&1`,
		},
		{
			name:    "empty_cmd_errors",
			rel:     domain.Release{StartSpec: &domain.NativeSpec{}},
			wantErr: true,
		},
	}
	a := Adapter{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := a.BuildStartCmd(nil, &tc.rel)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Errorf("\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

func TestKind(t *testing.T) {
	if k := (Adapter{}).Kind(); k != "native" {
		t.Fatalf("Kind=%q want native", k)
	}
}

package accessurl

import (
	"testing"

	"github.com/serverhub/serverhub/domain"
)

func TestCompute(t *testing.T) {
	tests := []struct {
		name      string
		app       domain.Application
		ingresses []domain.Ingress
		want      string
	}{
		{
			name: "none mode returns empty",
			app: domain.Application{
				ExposeMode: "none",
				Domain:     "example.com",
				SiteName:   "myapp",
			},
			want: "",
		},
		{
			name: "empty expose_mode returns empty",
			app: domain.Application{
				ExposeMode: "",
				Domain:     "example.com",
			},
			want: "",
		},
		{
			name: "site mode with applied ingress",
			app: domain.Application{
				ExposeMode: "site",
				SiteName:   "myapp",
			},
			ingresses: []domain.Ingress{
				{MatchKind: "domain", Domain: "myapp.example.com", Status: "applied"},
			},
			want: "https://myapp.example.com",
		},
		{
			name: "site mode fallback to app domain",
			app: domain.Application{
				ExposeMode: "site",
				Domain:     "myapp.example.com",
			},
			ingresses: []domain.Ingress{},
			want:      "https://myapp.example.com",
		},
		{
			name: "path mode with applied ingress",
			app: domain.Application{
				ExposeMode: "path",
				SiteName:   "myapp",
			},
			ingresses: []domain.Ingress{
				{MatchKind: "path", Domain: "example.com", Status: "applied"},
			},
			want: "https://example.com/myapp",
		},
		{
			name: "path mode fallback to app domain",
			app: domain.Application{
				ExposeMode: "path",
				Domain:     "example.com",
				SiteName:   "myapp",
			},
			ingresses: []domain.Ingress{},
			want:      "https://example.com/myapp",
		},
		{
			name: "path mode without site_name fallback",
			app: domain.Application{
				ExposeMode: "path",
				Domain:     "example.com",
				SiteName:   "",
			},
			ingresses: []domain.Ingress{
				{MatchKind: "path", Domain: "example.com", Status: "applied"},
			},
			want: "",
		},
		{
			name: "skip draft ingress",
			app: domain.Application{
				ExposeMode: "site",
			},
			ingresses: []domain.Ingress{
				{MatchKind: "domain", Domain: "myapp.example.com", Status: "draft"},
			},
			want: "",
		},
		{
			name: "site mode no domain fallback",
			app: domain.Application{
				ExposeMode: "site",
				Domain:     "",
			},
			ingresses: []domain.Ingress{},
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Compute(tt.ingresses, tt.app)
			if got != tt.want {
				t.Errorf("Compute() = %q, want %q", got, tt.want)
			}
		})
	}
}

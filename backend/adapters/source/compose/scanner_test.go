package compose

import (
	"testing"

	"github.com/serverhub/serverhub/core/source"
)

func TestFingerprintByteCompatV1(t *testing.T) {
	cases := []struct {
		name string
		c    source.Candidate
		want string
	}{
		{
			name: "default compose file",
			c: source.Candidate{
				Kind: Kind,
				Suggested: source.SuggestedFields{
					ComposeFile: "docker-compose.yml",
				},
			},
			// printf '%s' "compose|docker-compose.yml" | shasum -a 1
			want: "4b8a6ce373697b5abaf86a32fb3455f831749a5e",
		},
		{
			name: "named stack",
			c: source.Candidate{
				Kind: Kind,
				Suggested: source.SuggestedFields{
					ComposeFile: "stack.yml",
				},
			},
			// printf '%s' "compose|stack.yml" | shasum -a 1
			want: "5bb5bcc77b45897b682b2567fe51f9202c9bcbcc",
		},
	}
	s := Scanner{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := s.Fingerprint(tc.c)
			if got != tc.want {
				t.Errorf("fingerprint drift: got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestKindRegistered(t *testing.T) {
	if got := (Scanner{}).Kind(); got != "compose" {
		t.Errorf("Kind=%q, want compose", got)
	}
	got, err := source.Default.Get("compose")
	if err != nil {
		t.Fatalf("source.Default.Get(compose) failed: %v", err)
	}
	if got.Kind() != "compose" {
		t.Errorf("registered scanner Kind=%q", got.Kind())
	}
}

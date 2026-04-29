package domain

import (
	"encoding/json"
	"testing"
)

func TestUnmarshalStartSpec_AutoInfer(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string // Kind()
	}{
		{name: "docker by image", raw: `{"image":"nginx:1.27"}`, want: "docker"},
		{name: "compose by file_name", raw: `{"file_name":"prod.yml"}`, want: "compose"},
		{name: "native by cmd", raw: `{"cmd":"./start.sh"}`, want: "native"},
		{name: "empty string → static", raw: ``, want: "static"},
		{name: "empty object → static", raw: `{}`, want: "static"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec, err := UnmarshalStartSpec(tc.raw)
			if err != nil {
				t.Fatalf("UnmarshalStartSpec: %v", err)
			}
			if spec.Kind() != tc.want {
				t.Errorf("Kind()=%q want %q", spec.Kind(), tc.want)
			}
		})
	}
}

func TestUnmarshalStartSpecByKind(t *testing.T) {
	cases := []struct {
		name string
		kind string
		raw  string
		want StartSpec
	}{
		{
			name: "docker",
			kind: "docker",
			raw:  `{"image":"nginx:1.27","cmd":"echo hi"}`,
			want: &DockerSpec{Image: "nginx:1.27", Cmd: "echo hi"},
		},
		{
			name: "compose",
			kind: "compose",
			raw:  `{"file_name":"prod.yml"}`,
			want: &ComposeSpec{FileName: "prod.yml"},
		},
		{
			name: "native",
			kind: "native",
			raw:  `{"cmd":"./start.sh"}`,
			want: &NativeSpec{Cmd: "./start.sh"},
		},
		{
			name: "static",
			kind: "static",
			raw:  ``,
			want: &StaticSpec{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := UnmarshalStartSpecByKind(tc.kind, tc.raw)
			if err != nil {
				t.Fatalf("UnmarshalStartSpecByKind: %v", err)
			}
			if got.Kind() != tc.want.Kind() {
				t.Errorf("Kind()=%q want %q", got.Kind(), tc.want.Kind())
			}
		})
	}
}

func TestUnmarshalStartSpecByKind_Unknown(t *testing.T) {
	_, err := UnmarshalStartSpecByKind("podman", `{}`)
	if err == nil {
		t.Fatal("expected error for unknown kind")
	}
}

func TestUnmarshalStartSpec_InvalidJSON(t *testing.T) {
	_, err := UnmarshalStartSpec(`{bad`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestUnmarshalStartSpecByKind_InvalidJSON(t *testing.T) {
	_, err := UnmarshalStartSpecByKind("docker", `{bad`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestStartSpecRoundTrip_JSON(t *testing.T) {
	// domain typed → json.Marshal → UnmarshalStartSpec → 验证 Kind + Validate
	cases := []StartSpec{
		&DockerSpec{Image: "nginx:1.27", Cmd: "echo hi"},
		&ComposeSpec{FileName: "prod.yml"},
		&NativeSpec{Cmd: "./start.sh --port 8080"},
		&StaticSpec{},
	}
	for _, spec := range cases {
		t.Run(spec.Kind(), func(t *testing.T) {
			if err := spec.Validate(); err != nil {
				t.Fatalf("Validate: %v", err)
			}
			raw, err := json.Marshal(spec)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			back, err := UnmarshalStartSpec(string(raw))
			if err != nil {
				t.Fatalf("UnmarshalStartSpec: %v", err)
			}
			if back.Kind() != spec.Kind() {
				t.Errorf("Kind round-trip: %q → %q", spec.Kind(), back.Kind())
			}
		})
	}
}

func TestDockerSpec_Validate(t *testing.T) {
	if err := (&DockerSpec{}).Validate(); err == nil {
		t.Fatal("expected error for empty image")
	}
	if err := (&DockerSpec{Image: "x"}).Validate(); err != nil {
		t.Fatalf("expected no error: %v", err)
	}
}

func TestComposeSpec_Validate(t *testing.T) {
	if err := (&ComposeSpec{}).Validate(); err == nil {
		t.Fatal("expected error for empty file_name")
	}
	if err := (&ComposeSpec{FileName: "x.yml"}).Validate(); err != nil {
		t.Fatalf("expected no error: %v", err)
	}
}

func TestNativeSpec_Validate(t *testing.T) {
	if err := (&NativeSpec{}).Validate(); err == nil {
		t.Fatal("expected error for empty cmd")
	}
	if err := (&NativeSpec{Cmd: "./x"}).Validate(); err != nil {
		t.Fatalf("expected no error: %v", err)
	}
}

func TestStaticSpec_Validate(t *testing.T) {
	if err := (&StaticSpec{}).Validate(); err != nil {
		t.Fatalf("expected no error: %v", err)
	}
}

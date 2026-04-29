package cmdbuild

import (
	"strings"
	"testing"

	"github.com/serverhub/serverhub/domain"
)

// INV-8: BuildFetchPart 按 provider 生成正确的 fetch 命令

func TestBuildFetchPart_Docker(t *testing.T) {
	art := domain.Artifact{Provider: domain.ArtifactProviderDocker, Ref: "nginx:1.27"}
	out, err := BuildFetchPart(art)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "docker pull") {
		t.Errorf("expected docker pull, got %s", out)
	}
	if !strings.Contains(out, "nginx:1.27") {
		t.Errorf("expected ref in output, got %s", out)
	}
}

func TestBuildFetchPart_DockerEmptyRef(t *testing.T) {
	art := domain.Artifact{Provider: domain.ArtifactProviderDocker, Ref: ""}
	_, err := BuildFetchPart(art)
	if err == nil {
		t.Fatal("expected error for empty docker ref")
	}
}

func TestBuildFetchPart_HTTP(t *testing.T) {
	art := domain.Artifact{Provider: domain.ArtifactProviderHTTP, Ref: "https://example.com/pkg.tar.gz"}
	out, err := BuildFetchPart(art)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "curl -fsSL -o") {
		t.Errorf("expected curl -fsSL -o, got %s", out)
	}
	if !strings.Contains(out, "artifact.bin") {
		t.Errorf("expected artifact.bin, got %s", out)
	}
}

func TestBuildFetchPart_HTTPEmptyRef(t *testing.T) {
	art := domain.Artifact{Provider: domain.ArtifactProviderHTTP, Ref: ""}
	_, err := BuildFetchPart(art)
	if err == nil {
		t.Fatal("expected error for empty http ref")
	}
}

func TestBuildFetchPart_Script(t *testing.T) {
	art := domain.Artifact{Provider: domain.ArtifactProviderScript, PullScript: "echo hello"}
	out, err := BuildFetchPart(art)
	if err != nil {
		t.Fatal(err)
	}
	if out != "echo hello" {
		t.Errorf("expected script unchanged, got %s", out)
	}
}

func TestBuildFetchPart_ScriptEmpty(t *testing.T) {
	art := domain.Artifact{Provider: domain.ArtifactProviderScript, PullScript: ""}
	_, err := BuildFetchPart(art)
	if err == nil {
		t.Fatal("expected error for empty script")
	}
}

func TestBuildFetchPart_Upload(t *testing.T) {
	art := domain.Artifact{Provider: domain.ArtifactProviderUpload, Ref: "artifacts/myapp.tar.gz"}
	out, err := BuildFetchPart(art)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "test -f") {
		t.Errorf("expected test -f check, got %s", out)
	}
}

func TestBuildFetchPart_UploadEmptyRef(t *testing.T) {
	art := domain.Artifact{Provider: domain.ArtifactProviderUpload, Ref: ""}
	_, err := BuildFetchPart(art)
	if err == nil {
		t.Fatal("expected error for empty upload ref")
	}
}

func TestBuildFetchPart_Git(t *testing.T) {
	art := domain.Artifact{Provider: domain.ArtifactProviderGit, Ref: "https://github.com/a/b"}
	out, err := BuildFetchPart(art)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "git clone --depth 1") {
		t.Errorf("expected git clone --depth 1 for bare repo, got %s", out)
	}
}

func TestBuildFetchPart_GitWithRef(t *testing.T) {
	art := domain.Artifact{Provider: domain.ArtifactProviderGit, Ref: "https://github.com/a/b@main"}
	out, err := BuildFetchPart(art)
	if err != nil {
		t.Fatal(err)
	}
	// ShellQuote 会给 ref 加引号
	if !strings.Contains(out, "git checkout 'main'") && !strings.Contains(out, "git checkout main") {
		t.Errorf("expected git checkout main, got %s", out)
	}
}

func TestBuildFetchPart_GitWithHashSep(t *testing.T) {
	art := domain.Artifact{Provider: domain.ArtifactProviderGit, Ref: "https://github.com/a/b#v2.0.0"}
	out, err := BuildFetchPart(art)
	if err != nil {
		t.Fatal(err)
	}
	// ShellQuote 会给 ref 加引号
	if !strings.Contains(out, "git checkout 'v2.0.0'") && !strings.Contains(out, "git checkout v2.0.0") {
		t.Errorf("expected git checkout v2.0.0, got %s", out)
	}
}

func TestBuildFetchPart_GitEmptyRef(t *testing.T) {
	art := domain.Artifact{Provider: domain.ArtifactProviderGit, Ref: ""}
	_, err := BuildFetchPart(art)
	if err == nil {
		t.Fatal("expected error for empty git ref")
	}
}

func TestBuildFetchPart_UnsupportedProvider(t *testing.T) {
	art := domain.Artifact{Provider: "unknown", Ref: "x"}
	_, err := BuildFetchPart(art)
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
}

package cmdbuild

import (
	"strings"
	"testing"

	"github.com/serverhub/serverhub/domain"
)

// INV-8: WorkdirSetup 和 Assemble 正确性

func TestWorkdirSetup_ExplicitDir(t *testing.T) {
	svc := domain.Service{ID: 1, WorkDir: "/opt/app"}
	parts := WorkdirSetup(svc)
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}
	if !strings.Contains(parts[0], "mkdir -p") || !strings.Contains(parts[0], "/opt/app") {
		t.Errorf("part[0] should be mkdir -p /opt/app, got %s", parts[0])
	}
	if !strings.Contains(parts[1], "cd ") || !strings.Contains(parts[1], "/opt/app") {
		t.Errorf("part[1] should be cd ... /opt/app, got %s", parts[1])
	}
}

func TestWorkdirSetup_DefaultFallback(t *testing.T) {
	svc := domain.Service{ID: 42, WorkDir: ""}
	parts := WorkdirSetup(svc)
	if !strings.Contains(parts[0], "/tmp/serverhub-svc-42") {
		t.Errorf("expected default workdir, got %s", parts[0])
	}
}

func TestWorkdir(t *testing.T) {
	// 显式 WorkDir
	svc := domain.Service{ID: 1, WorkDir: "/opt/app"}
	if wd := Workdir(svc); wd != "/opt/app" {
		t.Errorf("expected /opt/app, got %s", wd)
	}
	// 默认退化
	svc2 := domain.Service{ID: 7}
	if wd := Workdir(svc2); wd != "/tmp/serverhub-svc-7" {
		t.Errorf("expected /tmp/serverhub-svc-7, got %s", wd)
	}
}

func TestAssemble_CoreStructure(t *testing.T) {
	cmd := Assemble("export FOO=bar; ", "pwd", "ls -la")
	if !strings.HasPrefix(cmd, "bash -c ") {
		t.Errorf("expected bash -c prefix, got %s", cmd)
	}
	if !strings.Contains(cmd, "set -e;") {
		t.Errorf("expected set -e, got %s", cmd)
	}
	if !strings.Contains(cmd, "pwd && ls -la") {
		t.Errorf("expected parts joined with &&, got %s", cmd)
	}
}

func TestAssemble_NoEnvPrefix(t *testing.T) {
	cmd := Assemble("", "echo ok")
	if !strings.Contains(cmd, "set -e; echo ok") {
		t.Errorf("expected set -e; echo ok, got %s", cmd)
	}
}

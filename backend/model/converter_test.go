package model

import (
	"testing"

	"github.com/serverhub/serverhub/domain"
)

// INV-3: model↔domain 双向转换一致性（round-trip 保真）

func TestServiceRoundTrip(t *testing.T) {
	currentReleaseID := uint(30)
	appID := uint(100)

	s := Service{
		ID:                 1,
		Name:               "test-svc",
		ServerID:           5,
		Type:               "docker",
		WorkDir:            "/opt/app",
		ApplicationID:      &appID,
		ExposedPort:        8080,
		SourceKind:         "auto",
		SourceID:           "auto-1",
		SourceFingerprint:  "abc123",
		CurrentReleaseID:   &currentReleaseID,
		AutoRollbackOnFail: true,
		AutoSync:           true,
		SyncInterval:       120,
		SyncStatus:         "synced",
	}

	// model → domain
	d := ToDomainService(s)

	// domain → model 回
	back := FromDomainService(d)

	if back.ID != s.ID {
		t.Errorf("ID: got %d want %d", back.ID, s.ID)
	}
	if back.Name != s.Name {
		t.Errorf("Name: got %s want %s", back.Name, s.Name)
	}
	if back.ServerID != s.ServerID {
		t.Errorf("ServerID: got %d want %d", back.ServerID, s.ServerID)
	}
	if back.Type != s.Type {
		t.Errorf("Type: got %s want %s", back.Type, s.Type)
	}
	if back.WorkDir != s.WorkDir {
		t.Errorf("WorkDir: got %s want %s", back.WorkDir, s.WorkDir)
	}
	if back.SourceKind != s.SourceKind {
		t.Errorf("SourceKind: got %s want %s", back.SourceKind, s.SourceKind)
	}
	if back.SourceID != s.SourceID {
		t.Errorf("SourceID: got %s want %s", back.SourceID, s.SourceID)
	}
	if back.SourceFingerprint != s.SourceFingerprint {
		t.Errorf("SourceFingerprint: got %s want %s", back.SourceFingerprint, s.SourceFingerprint)
	}
	if back.ExposedPort != s.ExposedPort {
		t.Errorf("ExposedPort: got %d want %d", back.ExposedPort, s.ExposedPort)
	}
	if *back.ApplicationID != *s.ApplicationID {
		t.Errorf("ApplicationID: got %d want %d", *back.ApplicationID, *s.ApplicationID)
	}
	if *back.CurrentReleaseID != *s.CurrentReleaseID {
		t.Errorf("CurrentReleaseID: got %d want %d", *back.CurrentReleaseID, *s.CurrentReleaseID)
	}
	if back.AutoRollbackOnFail != s.AutoRollbackOnFail {
		t.Errorf("AutoRollbackOnFail: got %v want %v", back.AutoRollbackOnFail, s.AutoRollbackOnFail)
	}
	if back.AutoSync != s.AutoSync {
		t.Errorf("AutoSync: got %v want %v", back.AutoSync, s.AutoSync)
	}
}

func TestServiceRoundTripNilPointers(t *testing.T) {
	s := Service{
		ID:         2,
		Name:       "min-svc",
		ServerID:   10,
		Type:       "native",
		SourceKind: "manual",
	}

	d := ToDomainService(s)
	back := FromDomainService(d)

	if back.ID != s.ID {
		t.Errorf("ID: got %d want %d", back.ID, s.ID)
	}
	if back.ApplicationID != nil {
		t.Errorf("ApplicationID should be nil, got %v", back.ApplicationID)
	}
	if back.CurrentReleaseID != nil {
		t.Errorf("CurrentReleaseID should be nil, got %v", back.CurrentReleaseID)
	}
}

func TestArtifactRoundTrip(t *testing.T) {
	a := Artifact{
		ID:        1,
		ServiceID: 5,
		Provider:  "http",
		Ref:       "https://example.com/pkg.tar.gz",
		Checksum:  "sha256:abc123",
		SizeBytes: 1024000,
	}

	d := ToDomainArtifact(a)
	back := FromDomainArtifact(d)

	if back.ID != a.ID {
		t.Errorf("ID: %d != %d", back.ID, a.ID)
	}
	if back.Provider != a.Provider {
		t.Errorf("Provider: %s != %s", back.Provider, a.Provider)
	}
	if back.Ref != a.Ref {
		t.Errorf("Ref: %s != %s", back.Ref, a.Ref)
	}
	if back.SizeBytes != a.SizeBytes {
		t.Errorf("SizeBytes: %d != %d", back.SizeBytes, a.SizeBytes)
	}
}

func TestReleaseRoundTrip(t *testing.T) {
	envSetID := uint(10)
	cfgSetID := uint(20)
	rel := Release{
		ID:          1,
		ServiceID:   5,
		ArtifactID:  10,
		EnvSetID:    &envSetID,
		ConfigSetID: &cfgSetID,
		Label:       "v1.0.0",
		StartSpec:   `{"image":"nginx:1.27"}`,
		Status:      string(domain.ReleaseStatusActive),
	}

	d := ToDomainRelease(rel)
	back := FromDomainRelease(d)

	if back.ID != rel.ID {
		t.Errorf("ID: %d != %d", back.ID, rel.ID)
	}
	if back.ServiceID != rel.ServiceID {
		t.Errorf("ServiceID: %d != %d", back.ServiceID, rel.ServiceID)
	}
	if back.ArtifactID != rel.ArtifactID {
		t.Errorf("ArtifactID: %d != %d", back.ArtifactID, rel.ArtifactID)
	}
	if *back.EnvSetID != *rel.EnvSetID {
		t.Errorf("EnvSetID: %d != %d", *back.EnvSetID, *rel.EnvSetID)
	}
	if *back.ConfigSetID != *rel.ConfigSetID {
		t.Errorf("ConfigSetID: %d != %d", *back.ConfigSetID, *rel.ConfigSetID)
	}
	if back.Label != rel.Label {
		t.Errorf("Label: %s != %s", back.Label, rel.Label)
	}
	if back.StartSpec != rel.StartSpec {
		t.Errorf("StartSpec: %s != %s", back.StartSpec, rel.StartSpec)
	}
	if back.Status != rel.Status {
		t.Errorf("Status: %s != %s", back.Status, rel.Status)
	}
}

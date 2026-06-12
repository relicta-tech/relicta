package manager

import (
	"strings"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/pkg/plugin"
)

func sampleRegistry() *Registry {
	return &Registry{
		Version: "1.0",
		Plugins: []PluginInfo{
			{
				Name:          "github",
				Description:   "Create GitHub releases",
				Repository:    "relicta-tech/plugin-github",
				Version:       "v2.0.3",
				Category:      "vcs",
				Author:        "Relicta Team",
				Homepage:      "https://github.com/relicta-tech/plugin-github",
				License:       "MIT",
				AuditStatus:   "verified",
				MinSDKVersion: 1,
				Checksums: map[string]string{
					"darwin_aarch64": strings.Repeat("a", 64),
					"linux_x86_64":   strings.Repeat("b", 64),
					"windows_x86_64": strings.Repeat("c", 64),
				},
				Hooks: []plugin.Hook{plugin.HookPostPublish},
			},
		},
	}
}

func TestBuildIndexFromRegistry(t *testing.T) {
	now := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	idx, err := BuildIndexFromRegistry(sampleRegistry(), now)
	if err != nil {
		t.Fatalf("BuildIndexFromRegistry: %v", err)
	}

	if idx.SchemaVersion != IndexSchemaVersion {
		t.Errorf("schema_version = %q, want %q", idx.SchemaVersion, IndexSchemaVersion)
	}
	if len(idx.Plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(idx.Plugins))
	}

	p := idx.Plugins[0]
	if p.AuditStatus != "verified" {
		t.Errorf("audit_status = %q, want verified", p.AuditStatus)
	}
	if len(p.Versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(p.Versions))
	}

	v := p.Versions[0]
	if v.Version != "2.0.3" {
		t.Errorf("version = %q, want 2.0.3 (no leading v)", v.Version)
	}
	if len(v.Artifacts) != 3 {
		t.Fatalf("expected 3 artifacts, got %d", len(v.Artifacts))
	}

	darwin := v.ArtifactFor("darwin", "arm64")
	if darwin == nil {
		t.Fatal("missing darwin/arm64 artifact")
	}
	wantURL := "https://github.com/relicta-tech/plugin-github/releases/download/v2.0.3/github_2.0.3_darwin_aarch64.tar.gz"
	if darwin.URL != wantURL {
		t.Errorf("darwin URL = %q, want %q", darwin.URL, wantURL)
	}
	if darwin.Digest != "sha256:"+strings.Repeat("a", 64) {
		t.Errorf("darwin digest = %q", darwin.Digest)
	}
	if !darwin.HasValidDigest() {
		t.Error("darwin digest should be valid")
	}

	win := v.ArtifactFor("windows", "amd64")
	if win == nil {
		t.Fatal("missing windows/amd64 artifact")
	}
	if !strings.HasSuffix(win.URL, ".zip") {
		t.Errorf("windows artifact should be a zip: %q", win.URL)
	}
}

func TestIndexRoundTrip(t *testing.T) {
	idx, err := BuildIndexFromRegistry(sampleRegistry(), time.Now())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	data, err := MarshalIndex(idx)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	parsed, err := ParseIndex(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(parsed.Plugins) != len(idx.Plugins) {
		t.Errorf("round-trip lost plugins: %d != %d", len(parsed.Plugins), len(idx.Plugins))
	}
}

func TestParseIndexRejectsWrongSchema(t *testing.T) {
	if _, err := ParseIndex([]byte(`{"schema_version":"1","plugins":[]}`)); err == nil {
		t.Fatal("expected error for schema_version 1")
	}
}

func TestArtifactDigestValidation(t *testing.T) {
	cases := []struct {
		digest string
		valid  bool
	}{
		{"sha256:" + strings.Repeat("a", 64), true},
		{DigestPlaceholder, false},
		{"", false},
		{strings.Repeat("a", 64), false},            // missing prefix
		{"sha256:" + strings.Repeat("a", 8), false}, // truncated
	}
	for _, c := range cases {
		a := IndexArtifact{Digest: c.digest}
		if a.HasValidDigest() != c.valid {
			t.Errorf("HasValidDigest(%q) = %v, want %v", c.digest, !c.valid, c.valid)
		}
	}
}

func TestArtifactIsSigned(t *testing.T) {
	unsigned := IndexArtifact{}
	if unsigned.IsSigned() {
		t.Error("artifact without cosign metadata must not report signed")
	}
	signed := IndexArtifact{
		CosignBundleURL:          "https://example.com/checksums.txt.sigstore.json",
		CosignCertIdentityRegexp: "https://github.com/relicta-tech/plugin-github/.*",
		CosignOIDCIssuer:         "https://token.actions.githubusercontent.com",
	}
	if !signed.IsSigned() {
		t.Error("artifact with full cosign metadata must report signed")
	}
}

func TestBuildIndexFromFullRegistryFile(t *testing.T) {
	// The real registry.yaml must convert cleanly — this is the generation
	// path for the published index.
	reg, err := LoadRegistryFromFile("../../../plugins/registry.yaml")
	if err != nil {
		t.Skipf("registry.yaml not loadable from test dir: %v", err)
	}
	idx, err := BuildIndexFromRegistry(reg, time.Now())
	if err != nil {
		t.Fatalf("real registry.yaml does not convert: %v", err)
	}
	if len(idx.Plugins) == 0 {
		t.Fatal("expected plugins from real registry")
	}
	for _, p := range idx.Plugins {
		for _, v := range p.Versions {
			for _, a := range v.Artifacts {
				if !a.HasValidDigest() {
					t.Errorf("plugin %s artifact %s/%s has invalid digest %q", p.Name, a.OS, a.Arch, a.Digest)
				}
			}
		}
	}
}

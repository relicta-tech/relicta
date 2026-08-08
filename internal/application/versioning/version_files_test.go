package versioning

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

func mustVersion(t *testing.T, s string) version.SemanticVersion {
	t.Helper()
	v, err := version.Parse(s)
	if err != nil {
		t.Fatalf("version.Parse(%q) error = %v", s, err)
	}
	return v
}

func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	return string(data)
}

// TestApply_StreamDeckShape is the exact case from issue #195: two manifests that
// must stay in step, in two different formats, where writing only one ships a
// build the platform rejects.
func TestApply_StreamDeckShape(t *testing.T) {
	dir := t.TempDir()
	pkg := write(t, dir, "package.json", "{\n  \"name\": \"plugin\",\n  \"version\": \"2.7.14\"\n}\n")
	man := write(t, dir, "com.example.sdPlugin/manifest.json", "{\n  \"Name\": \"Plugin\",\n  \"Version\": \"2.7.14.0\"\n}\n")

	targets := []config.VersionTarget{
		{Path: "package.json", Format: config.VersionFormatSemver, Key: "version"},
		{Path: "com.example.sdPlugin/manifest.json", Format: config.VersionFormatSemverBuild, Key: "Version"},
	}

	written, err := NewVersionFileWriter(dir).Apply(targets, mustVersion(t, "2.7.15"))
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if len(written) != 2 {
		t.Fatalf("wrote %d targets, want 2", len(written))
	}

	if got := read(t, pkg); !strings.Contains(got, `"version": "2.7.15"`) {
		t.Errorf("package.json = %s, want version 2.7.15", got)
	}
	if got := read(t, man); !strings.Contains(got, `"Version": "2.7.15.0"`) {
		t.Errorf("manifest.json = %s, want Version 2.7.15.0", got)
	}
}

// The central guarantee: a bad target must leave every file untouched, because a
// half-bump is worse than none.
func TestApply_IsAllOrNothing(t *testing.T) {
	dir := t.TempDir()
	pkg := write(t, dir, "package.json", "{\n  \"version\": \"1.0.0\"\n}\n")
	before := read(t, pkg)

	targets := []config.VersionTarget{
		{Path: "package.json", Key: "version"},
		// Second target is broken: the key does not exist.
		{Path: "package.json", Key: "nope"},
	}

	if _, err := NewVersionFileWriter(dir).Apply(targets, mustVersion(t, "1.1.0")); err == nil {
		t.Fatal("Apply() should fail when a target cannot be rendered")
	}

	if after := read(t, pkg); after != before {
		t.Errorf("package.json was modified despite the failure:\nbefore: %s\nafter:  %s", before, after)
	}
}

func TestApply_MissingFileFailsBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	pkg := write(t, dir, "package.json", "{\n  \"version\": \"1.0.0\"\n}\n")
	before := read(t, pkg)

	targets := []config.VersionTarget{
		{Path: "package.json", Key: "version"},
		{Path: "does-not-exist.json", Key: "version"},
	}

	_, err := NewVersionFileWriter(dir).Apply(targets, mustVersion(t, "1.1.0"))
	if err == nil {
		t.Fatal("Apply() should fail for a missing file")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error = %v, want it to name the missing file", err)
	}
	if after := read(t, pkg); after != before {
		t.Error("the existing file was modified even though another target was missing")
	}
}

func TestApply_Formats(t *testing.T) {
	tests := []struct {
		name    string
		target  config.VersionTarget
		version string
		want    string
	}{
		{
			name:    "semver",
			target:  config.VersionTarget{Path: "f.json", Key: "v"},
			version: "2.7.15",
			want:    "2.7.15",
		},
		{
			name:    "semver.build adds a fourth component",
			target:  config.VersionTarget{Path: "f.json", Key: "v", Format: config.VersionFormatSemverBuild},
			version: "2.7.15",
			want:    "2.7.15.0",
		},
		{
			name:    "template",
			target:  config.VersionTarget{Path: "f.json", Key: "v", Format: config.VersionFormatTemplate, Template: "${major}.${minor}.${patch}.0"},
			version: "2.7.15",
			want:    "2.7.15.0",
		},
		{
			name:    "template with version",
			target:  config.VersionTarget{Path: "f.json", Key: "v", Format: config.VersionFormatTemplate, Template: "v${version}-rc"},
			version: "1.2.3",
			want:    "v1.2.3-rc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := write(t, dir, "f.json", "{\n  \"v\": \"0.0.0\"\n}\n")

			if _, err := NewVersionFileWriter(dir).Apply([]config.VersionTarget{tt.target}, mustVersion(t, tt.version)); err != nil {
				t.Fatalf("Apply() error = %v", err)
			}

			var doc map[string]any
			if err := json.Unmarshal([]byte(read(t, path)), &doc); err != nil {
				t.Fatalf("result is not valid json: %v", err)
			}
			if got := doc["v"]; got != tt.want {
				t.Errorf("v = %v, want %v", got, tt.want)
			}
		})
	}
}

// Android's versionCode: an integer that must rise, written as a number rather
// than a quoted string, and derived from the file rather than the version.
func TestApply_IntegerIncrement(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "app.json", "{\n  \"versionCode\": 41\n}\n")

	target := config.VersionTarget{
		Path:     "app.json",
		Key:      "versionCode",
		Format:   config.VersionFormatInteger,
		Strategy: config.StrategyIncrement,
	}

	if _, err := NewVersionFileWriter(dir).Apply([]config.VersionTarget{target}, mustVersion(t, "2.7.15")); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	got := read(t, path)
	if !strings.Contains(got, `"versionCode": 42`) {
		t.Errorf("app.json = %s, want versionCode 42 as a number", got)
	}
	if strings.Contains(got, `"42"`) {
		t.Error("versionCode was written as a string; an integer field rejects a quoted value")
	}
}

// Helm's Chart.yaml carries both version and appVersion, which is exactly why the
// key is required rather than guessed.
func TestApply_YAMLNamedKey(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "Chart.yaml", "name: mychart\nversion: 0.1.0\nappVersion: 1.0.0\n")

	target := config.VersionTarget{Path: "Chart.yaml", Key: "appVersion"}
	if _, err := NewVersionFileWriter(dir).Apply([]config.VersionTarget{target}, mustVersion(t, "2.0.0")); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	got := read(t, path)
	if !strings.Contains(got, "appVersion: 2.0.0") {
		t.Errorf("Chart.yaml = %s, want appVersion 2.0.0", got)
	}
	if !strings.Contains(got, "version: 0.1.0") {
		t.Errorf("Chart.yaml = %s, want chart version left at 0.1.0", got)
	}
}

// Cargo.toml keeps its version under [package].
func TestApply_TOMLNestedKey(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "Cargo.toml", "[package]\nname = \"app\"\nversion = \"0.1.0\"\n")

	target := config.VersionTarget{Path: "Cargo.toml", Key: "package.version"}
	if _, err := NewVersionFileWriter(dir).Apply([]config.VersionTarget{target}, mustVersion(t, "0.2.0")); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	if got := read(t, path); !strings.Contains(got, `version = '0.2.0'`) && !strings.Contains(got, `version = "0.2.0"`) {
		t.Errorf("Cargo.toml = %s, want package.version 0.2.0", got)
	}
}

func TestApply_JSONPointerKey(t *testing.T) {
	dir := t.TempDir()
	// A key containing a slash is only addressable via a JSON Pointer.
	path := write(t, dir, "odd.json", "{\n  \"a/b\": \"0.0.0\"\n}\n")

	target := config.VersionTarget{Path: "odd.json", Key: "/a~1b"}
	if _, err := NewVersionFileWriter(dir).Apply([]config.VersionTarget{target}, mustVersion(t, "1.2.3")); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got := read(t, path); !strings.Contains(got, `"a/b": "1.2.3"`) {
		t.Errorf("odd.json = %s, want a/b set to 1.2.3", got)
	}
}

func TestApply_PlainTextFile(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "VERSION", "1.0.0\n")

	if _, err := NewVersionFileWriter(dir).Apply([]config.VersionTarget{{Path: "VERSION"}}, mustVersion(t, "1.1.0")); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got := read(t, path); got != "1.1.0\n" {
		t.Errorf("VERSION = %q, want %q (trailing newline preserved)", got, "1.1.0\n")
	}
}

// package.json is conventionally two-space indented; rewriting it should not
// reformat the whole file.
func TestApply_PreservesJSONIndent(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "four.json", "{\n    \"version\": \"1.0.0\",\n    \"name\": \"x\"\n}\n")

	if _, err := NewVersionFileWriter(dir).Apply([]config.VersionTarget{{Path: "four.json", Key: "version"}}, mustVersion(t, "1.1.0")); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got := read(t, path); !strings.Contains(got, "\n    \"") {
		t.Errorf("four.json = %s, want four-space indentation preserved", got)
	}
}

func TestApply_Rejections(t *testing.T) {
	tests := []struct {
		name    string
		target  config.VersionTarget
		file    string
		content string
		wantErr string
	}{
		{
			name:    "structured file without a key",
			target:  config.VersionTarget{Path: "f.json"},
			file:    "f.json",
			content: "{\"version\":\"1.0.0\"}",
			wantErr: "key is required",
		},
		{
			name:    "key on a plain-text file",
			target:  config.VersionTarget{Path: "VERSION", Key: "version"},
			file:    "VERSION",
			content: "1.0.0\n",
			wantErr: "keys apply to json, yaml and toml",
		},
		{
			name:    "template format without a template",
			target:  config.VersionTarget{Path: "f.json", Key: "v", Format: config.VersionFormatTemplate},
			file:    "f.json",
			content: "{\"v\":\"1.0.0\"}",
			wantErr: "requires a template",
		},
		{
			name:    "increment without integer format",
			target:  config.VersionTarget{Path: "f.json", Key: "v", Strategy: config.StrategyIncrement},
			file:    "f.json",
			content: "{\"v\":1}",
			wantErr: "requires format 'integer'",
		},
		{
			name:    "unknown format",
			target:  config.VersionTarget{Path: "f.json", Key: "v", Format: "nonsense"},
			file:    "f.json",
			content: "{\"v\":\"1.0.0\"}",
			wantErr: "unknown format",
		},
		{
			name:    "path escaping the repository",
			target:  config.VersionTarget{Path: "../outside.json", Key: "v"},
			file:    "f.json",
			content: "{}",
			wantErr: "escapes the repository root",
		},
		{
			name:    "absolute path",
			target:  config.VersionTarget{Path: "/etc/passwd", Key: "v"},
			file:    "f.json",
			content: "{}",
			wantErr: "must be relative",
		},
		{
			name:    "unparseable json",
			target:  config.VersionTarget{Path: "f.json", Key: "v"},
			file:    "f.json",
			content: "{not json",
			wantErr: "parsing json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			write(t, dir, tt.file, tt.content)

			_, err := NewVersionFileWriter(dir).Apply([]config.VersionTarget{tt.target}, mustVersion(t, "1.1.0"))
			if err == nil {
				t.Fatalf("Apply() should have failed with %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

// Plan must report what would be written without touching anything.
func TestPlan_DoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "package.json", "{\n  \"version\": \"1.0.0\"\n}\n")
	before := read(t, path)

	planned, err := NewVersionFileWriter(dir).Plan(
		[]config.VersionTarget{{Path: "package.json", Key: "version"}},
		mustVersion(t, "1.1.0"),
	)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if len(planned) != 1 || planned[0].Value != "1.1.0" {
		t.Errorf("planned = %+v, want one entry with value 1.1.0", planned)
	}
	if read(t, path) != before {
		t.Error("Plan() modified the file")
	}
}

// The deprecated single versionfile must keep working, as a one-entry list.
func TestResolvedVersionFiles_BackwardCompatible(t *testing.T) {
	single := &config.VersioningConfig{VersionFile: "VERSION"}
	got := single.ResolvedVersionFiles()
	if len(got) != 1 || got[0].Path != "VERSION" || got[0].Format != config.VersionFormatSemver {
		t.Errorf("ResolvedVersionFiles() = %+v, want one semver entry for VERSION", got)
	}

	// The list wins when both are set.
	both := &config.VersioningConfig{
		VersionFile:  "VERSION",
		VersionFiles: []config.VersionTarget{{Path: "package.json", Key: "version"}},
	}
	if got := both.ResolvedVersionFiles(); len(got) != 1 || got[0].Path != "package.json" {
		t.Errorf("ResolvedVersionFiles() = %+v, want the explicit list to win", got)
	}

	if got := (&config.VersioningConfig{}).ResolvedVersionFiles(); got != nil {
		t.Errorf("ResolvedVersionFiles() = %+v, want nil when nothing is configured", got)
	}
}

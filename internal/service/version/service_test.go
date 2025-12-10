// Package version provides version management for ReleasePilot.
package version

import (
	"testing"

	"github.com/felixgeelhaar/release-pilot/internal/service/git"
)

func TestVersion_String(t *testing.T) {
	tests := []struct {
		name    string
		version Version
		want    string
	}{
		{
			name:    "basic version",
			version: Version{Major: 1, Minor: 2, Patch: 3},
			want:    "1.2.3",
		},
		{
			name:    "zero version",
			version: Version{Major: 0, Minor: 0, Patch: 0},
			want:    "0.0.0",
		},
		{
			name:    "with prerelease",
			version: Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha"},
			want:    "1.0.0-alpha",
		},
		{
			name:    "large numbers",
			version: Version{Major: 100, Minor: 200, Patch: 300},
			want:    "100.200.300",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.version.String(); got != tt.want {
				t.Errorf("Version.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVersion_StringWithPrefix(t *testing.T) {
	v := Version{Major: 1, Minor: 2, Patch: 3}

	tests := []struct {
		prefix string
		want   string
	}{
		{"v", "v1.2.3"},
		{"V", "V1.2.3"},
		{"release-", "release-1.2.3"},
		{"", "1.2.3"},
	}

	for _, tt := range tests {
		t.Run(tt.prefix, func(t *testing.T) {
			if got := v.StringWithPrefix(tt.prefix); got != tt.want {
				t.Errorf("StringWithPrefix(%q) = %v, want %v", tt.prefix, got, tt.want)
			}
		})
	}
}

func TestVersion_IsZero(t *testing.T) {
	tests := []struct {
		name    string
		version Version
		want    bool
	}{
		{
			name:    "zero version",
			version: Version{Major: 0, Minor: 0, Patch: 0},
			want:    true,
		},
		{
			name:    "non-zero major",
			version: Version{Major: 1, Minor: 0, Patch: 0},
			want:    false,
		},
		{
			name:    "non-zero minor",
			version: Version{Major: 0, Minor: 1, Patch: 0},
			want:    false,
		},
		{
			name:    "non-zero patch",
			version: Version{Major: 0, Minor: 0, Patch: 1},
			want:    false,
		},
		{
			name:    "zero with prerelease",
			version: Version{Major: 0, Minor: 0, Patch: 0, Prerelease: "alpha"},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.version.IsZero(); got != tt.want {
				t.Errorf("Version.IsZero() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultBumpOptions(t *testing.T) {
	opts := DefaultBumpOptions()

	if opts.ReleaseType != git.ReleaseTypePatch {
		t.Errorf("Default ReleaseType = %v, want patch", opts.ReleaseType)
	}
	if opts.Prefix != "v" {
		t.Errorf("Default Prefix = %v, want v", opts.Prefix)
	}
	if !opts.CreateTag {
		t.Error("Default CreateTag should be true")
	}
	if opts.PushTag {
		t.Error("Default PushTag should be false")
	}
	if opts.DryRun {
		t.Error("Default DryRun should be false")
	}
}

func TestBumpOptions_Fields(t *testing.T) {
	opts := BumpOptions{
		ReleaseType: git.ReleaseTypeMajor,
		Prerelease:  "beta.1",
		Metadata:    "build.123",
		Prefix:      "release-",
		CreateTag:   true,
		TagMessage:  "Release message",
		PushTag:     true,
		UpdateFile:  "version.json",
		DryRun:      true,
	}

	if opts.ReleaseType != git.ReleaseTypeMajor {
		t.Errorf("ReleaseType = %v, want major", opts.ReleaseType)
	}
	if opts.Prerelease != "beta.1" {
		t.Errorf("Prerelease = %v, want beta.1", opts.Prerelease)
	}
	if opts.Metadata != "build.123" {
		t.Errorf("Metadata = %v, want build.123", opts.Metadata)
	}
	if opts.Prefix != "release-" {
		t.Errorf("Prefix = %v, want release-", opts.Prefix)
	}
	if !opts.CreateTag {
		t.Error("CreateTag should be true")
	}
	if opts.TagMessage != "Release message" {
		t.Errorf("TagMessage = %v, want Release message", opts.TagMessage)
	}
	if !opts.PushTag {
		t.Error("PushTag should be true")
	}
	if opts.UpdateFile != "version.json" {
		t.Errorf("UpdateFile = %v, want version.json", opts.UpdateFile)
	}
	if !opts.DryRun {
		t.Error("DryRun should be true")
	}
}

func TestFormatOptions_Fields(t *testing.T) {
	opts := FormatOptions{
		IncludePrefix:   true,
		Prefix:          "v",
		IncludeMetadata: true,
	}

	if !opts.IncludePrefix {
		t.Error("IncludePrefix should be true")
	}
	if opts.Prefix != "v" {
		t.Errorf("Prefix = %v, want v", opts.Prefix)
	}
	if !opts.IncludeMetadata {
		t.Error("IncludeMetadata should be true")
	}
}

func TestDefaultChangelogOptions(t *testing.T) {
	opts := DefaultChangelogOptions()

	if opts.Format != "keep-a-changelog" {
		t.Errorf("Default Format = %v, want keep-a-changelog", opts.Format)
	}
	if opts.GroupBy != "type" {
		t.Errorf("Default GroupBy = %v, want type", opts.GroupBy)
	}
	if !opts.IncludeCommitHash {
		t.Error("Default IncludeCommitHash should be true")
	}
	if opts.IncludeAuthor {
		t.Error("Default IncludeAuthor should be false")
	}
	if !opts.LinkCommits {
		t.Error("Default LinkCommits should be true")
	}
	if !opts.LinkIssues {
		t.Error("Default LinkIssues should be true")
	}
	if len(opts.Categories) == 0 {
		t.Error("Default Categories should not be empty")
	}
	if opts.Categories["feat"] != "Features" {
		t.Errorf("Categories[feat] = %v, want Features", opts.Categories["feat"])
	}
	if len(opts.Exclude) == 0 {
		t.Error("Default Exclude should not be empty")
	}
}

func TestChangelogOptions_Fields(t *testing.T) {
	opts := ChangelogOptions{
		Version:           &Version{Major: 1, Minor: 0, Patch: 0},
		PreviousVersion:   &Version{Major: 0, Minor: 9, Patch: 0},
		Date:              "2024-01-15",
		RepositoryURL:     "https://github.com/user/repo",
		IssueURL:          "https://github.com/user/repo/issues/{id}",
		Format:            "conventional",
		GroupBy:           "scope",
		IncludeCommitHash: true,
		IncludeAuthor:     true,
		LinkCommits:       true,
		LinkIssues:        true,
		Categories:        map[string]string{"feat": "New Features"},
		Exclude:           []string{"ci"},
		Template:          "custom.tmpl",
		CompareURL:        "https://github.com/user/repo/compare/v0.9.0...v1.0.0",
	}

	if opts.Version.String() != "1.0.0" {
		t.Errorf("Version = %v, want 1.0.0", opts.Version.String())
	}
	if opts.Date != "2024-01-15" {
		t.Errorf("Date = %v, want 2024-01-15", opts.Date)
	}
	if opts.Format != "conventional" {
		t.Errorf("Format = %v, want conventional", opts.Format)
	}
	if opts.GroupBy != "scope" {
		t.Errorf("GroupBy = %v, want scope", opts.GroupBy)
	}
}

func TestFormatVersionString(t *testing.T) {
	tests := []struct {
		name    string
		version *Version
		opts    FormatOptions
		want    string
	}{
		{
			name:    "basic version",
			version: &Version{Major: 1, Minor: 2, Patch: 3},
			opts:    FormatOptions{},
			want:    "1.2.3",
		},
		{
			name:    "with v prefix",
			version: &Version{Major: 1, Minor: 2, Patch: 3},
			opts:    FormatOptions{IncludePrefix: true, Prefix: "v"},
			want:    "v1.2.3",
		},
		{
			name:    "with custom prefix",
			version: &Version{Major: 1, Minor: 0, Patch: 0},
			opts:    FormatOptions{IncludePrefix: true, Prefix: "release-"},
			want:    "release-1.0.0",
		},
		{
			name:    "with default prefix",
			version: &Version{Major: 1, Minor: 0, Patch: 0},
			opts:    FormatOptions{IncludePrefix: true},
			want:    "v1.0.0",
		},
		{
			name:    "with prerelease",
			version: &Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha.1"},
			opts:    FormatOptions{},
			want:    "1.0.0-alpha.1",
		},
		{
			name:    "with metadata included",
			version: &Version{Major: 1, Minor: 0, Patch: 0, Metadata: "build.123"},
			opts:    FormatOptions{IncludeMetadata: true},
			want:    "1.0.0+build.123",
		},
		{
			name:    "with metadata not included",
			version: &Version{Major: 1, Minor: 0, Patch: 0, Metadata: "build.123"},
			opts:    FormatOptions{IncludeMetadata: false},
			want:    "1.0.0",
		},
		{
			name:    "full version",
			version: &Version{Major: 2, Minor: 1, Patch: 0, Prerelease: "rc.1", Metadata: "20240115"},
			opts:    FormatOptions{IncludePrefix: true, Prefix: "v", IncludeMetadata: true},
			want:    "v2.1.0-rc.1+20240115",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatVersionString(tt.version, tt.opts); got != tt.want {
				t.Errorf("FormatVersionString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *Version
		wantErr bool
	}{
		{
			name:  "basic version",
			input: "1.2.3",
			want:  &Version{Major: 1, Minor: 2, Patch: 3},
		},
		{
			name:  "with v prefix",
			input: "v1.2.3",
			want:  &Version{Major: 1, Minor: 2, Patch: 3},
		},
		{
			name:  "with V prefix",
			input: "V1.2.3",
			want:  &Version{Major: 1, Minor: 2, Patch: 3},
		},
		{
			name:  "with prerelease",
			input: "1.0.0-alpha",
			want:  &Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha"},
		},
		{
			name:  "with metadata",
			input: "1.0.0+build.123",
			want:  &Version{Major: 1, Minor: 0, Patch: 0, Metadata: "build.123"},
		},
		{
			name:  "with prerelease and metadata",
			input: "1.0.0-beta.1+build.456",
			want:  &Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "beta.1", Metadata: "build.456"},
		},
		{
			name:  "with whitespace",
			input: "  1.0.0  ",
			want:  &Version{Major: 1, Minor: 0, Patch: 0},
		},
		{
			name:    "invalid format",
			input:   "invalid",
			wantErr: true,
		},
		{
			name:    "missing patch",
			input:   "1.0",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("Parse() error = %v", err)
				return
			}
			if got.Major != tt.want.Major || got.Minor != tt.want.Minor || got.Patch != tt.want.Patch {
				t.Errorf("Parse() = %v.%v.%v, want %v.%v.%v",
					got.Major, got.Minor, got.Patch,
					tt.want.Major, tt.want.Minor, tt.want.Patch)
			}
			if got.Prerelease != tt.want.Prerelease {
				t.Errorf("Parse().Prerelease = %v, want %v", got.Prerelease, tt.want.Prerelease)
			}
			if got.Metadata != tt.want.Metadata {
				t.Errorf("Parse().Metadata = %v, want %v", got.Metadata, tt.want.Metadata)
			}
		})
	}
}

func TestDefaultServiceConfig(t *testing.T) {
	cfg := DefaultServiceConfig()

	if cfg.DefaultPrefix != "v" {
		t.Errorf("Default Prefix = %v, want v", cfg.DefaultPrefix)
	}
	if cfg.VersionSource != "tag" {
		t.Errorf("Default VersionSource = %v, want tag", cfg.VersionSource)
	}
}

func TestServiceOptions(t *testing.T) {
	cfg := DefaultServiceConfig()

	WithDefaultPrefix("release-")(&cfg)
	WithVersionSource("file")(&cfg)
	WithVersionFile("VERSION")(&cfg)

	if cfg.DefaultPrefix != "release-" {
		t.Errorf("DefaultPrefix = %v, want release-", cfg.DefaultPrefix)
	}
	if cfg.VersionSource != "file" {
		t.Errorf("VersionSource = %v, want file", cfg.VersionSource)
	}
	if cfg.VersionFile != "VERSION" {
		t.Errorf("VersionFile = %v, want VERSION", cfg.VersionFile)
	}
}

func TestParseError_Error(t *testing.T) {
	err := &parseError{
		version: "invalid",
		message: "bad format",
	}

	expected := "parse version invalid: bad format"
	if err.Error() != expected {
		t.Errorf("Error() = %v, want %v", err.Error(), expected)
	}
}

func TestTrimSpace(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"  hello  ", "hello"},
		{"\t\nhello\r\n", "hello"},
		{"", ""},
		{"   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := trimSpace(tt.input); got != tt.want {
				t.Errorf("trimSpace(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTrimPrefix(t *testing.T) {
	tests := []struct {
		s      string
		prefix string
		want   string
	}{
		{"v1.0.0", "v", "1.0.0"},
		{"v1.0.0", "V", "v1.0.0"},
		{"hello", "h", "ello"},
		{"hello", "x", "hello"},
		{"", "v", ""},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.prefix, func(t *testing.T) {
			if got := trimPrefix(tt.s, tt.prefix); got != tt.want {
				t.Errorf("trimPrefix(%q, %q) = %q, want %q", tt.s, tt.prefix, got, tt.want)
			}
		})
	}
}

func TestIndexByte(t *testing.T) {
	tests := []struct {
		s    string
		c    byte
		want int
	}{
		{"hello", 'l', 2},
		{"hello", 'x', -1},
		{"", 'a', -1},
		{"a.b.c", '.', 1},
	}

	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			if got := indexByte(tt.s, tt.c); got != tt.want {
				t.Errorf("indexByte(%q, %c) = %d, want %d", tt.s, tt.c, got, tt.want)
			}
		})
	}
}

func TestSplitDots(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"1.2.3", []string{"1", "2", "3"}},
		{"a.b", []string{"a", "b"}},
		{"single", []string{"single"}},
		{"", []string{""}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitDots(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("splitDots(%q) length = %d, want %d", tt.input, len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitDots(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestParseUint(t *testing.T) {
	tests := []struct {
		input  string
		want   uint64
		wantOK bool
	}{
		{"0", 0, true},
		{"123", 123, true},
		{"1000000", 1000000, true},
		{"", 0, false},
		{"abc", 0, false},
		{"12a", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := parseUint(tt.input)
			if ok != tt.wantOK {
				t.Errorf("parseUint(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
				return
			}
			if ok && got != tt.want {
				t.Errorf("parseUint(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatUint64(t *testing.T) {
	tests := []struct {
		input uint64
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{123, "123"},
		{1000000, "1000000"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := formatUint64(tt.input); got != tt.want {
				t.Errorf("formatUint64(%d) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSplitVersion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *Version
		wantNil bool
	}{
		{
			name:  "basic",
			input: "1.2.3",
			want:  &Version{Major: 1, Minor: 2, Patch: 3},
		},
		{
			name:  "with prerelease",
			input: "1.0.0-alpha",
			want:  &Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha"},
		},
		{
			name:  "with metadata",
			input: "1.0.0+build",
			want:  &Version{Major: 1, Minor: 0, Patch: 0, Metadata: "build"},
		},
		{
			name:  "full",
			input: "1.0.0-beta+123",
			want:  &Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "beta", Metadata: "123"},
		},
		{
			name:    "invalid - missing parts",
			input:   "1.0",
			wantNil: true,
		},
		{
			name:    "invalid - non-numeric",
			input:   "a.b.c",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitVersion(tt.input)
			if tt.wantNil {
				if got != nil {
					t.Errorf("splitVersion(%q) = %v, want nil", tt.input, got)
				}
				return
			}
			if got == nil {
				t.Errorf("splitVersion(%q) = nil, want non-nil", tt.input)
				return
			}
			if got.Major != tt.want.Major || got.Minor != tt.want.Minor || got.Patch != tt.want.Patch {
				t.Errorf("splitVersion(%q) = %d.%d.%d, want %d.%d.%d",
					tt.input, got.Major, got.Minor, got.Patch,
					tt.want.Major, tt.want.Minor, tt.want.Patch)
			}
			if got.Prerelease != tt.want.Prerelease {
				t.Errorf("splitVersion(%q).Prerelease = %q, want %q", tt.input, got.Prerelease, tt.want.Prerelease)
			}
			if got.Metadata != tt.want.Metadata {
				t.Errorf("splitVersion(%q).Metadata = %q, want %q", tt.input, got.Metadata, tt.want.Metadata)
			}
		})
	}
}

// Package cli provides the command-line interface for Relicta.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/internal/application/governance"
	"github.com/relicta-tech/relicta/internal/application/versioning"
	"github.com/relicta-tech/relicta/internal/config"
	domainrelease "github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/internal/domain/version"
	"github.com/relicta-tech/relicta/internal/infrastructure/ai"
	servicerelease "github.com/relicta-tech/relicta/internal/service/release"
)

func TestParseBumpLevel(t *testing.T) {
	tests := []struct {
		name        string
		level       string
		wantType    version.BumpType
		wantAuto    bool
		wantErr     bool
		errContains string
	}{
		{
			name:     "major bump",
			level:    "major",
			wantType: version.BumpMajor,
			wantAuto: false,
			wantErr:  false,
		},
		{
			name:     "minor bump",
			level:    "minor",
			wantType: version.BumpMinor,
			wantAuto: false,
			wantErr:  false,
		},
		{
			name:     "patch bump",
			level:    "patch",
			wantType: version.BumpPatch,
			wantAuto: false,
			wantErr:  false,
		},
		{
			name:     "empty string for auto-detection",
			level:    "",
			wantType: version.BumpType(""),
			wantAuto: true,
			wantErr:  false,
		},
		{
			name:        "invalid level",
			level:       "invalid",
			wantType:    version.BumpType(""),
			wantAuto:    false,
			wantErr:     true,
			errContains: "invalid bump level",
		},
		{
			name:        "typo in major",
			level:       "magor",
			wantType:    version.BumpType(""),
			wantAuto:    false,
			wantErr:     true,
			errContains: "invalid bump level",
		},
		{
			name:        "uppercase not supported",
			level:       "MAJOR",
			wantType:    version.BumpType(""),
			wantAuto:    false,
			wantErr:     true,
			errContains: "invalid bump level",
		},
		{
			name:        "numeric input",
			level:       "1",
			wantType:    version.BumpType(""),
			wantAuto:    false,
			wantErr:     true,
			errContains: "invalid bump level",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotAuto, err := parseBumpLevel(tt.level)

			if tt.wantErr {
				if err == nil {
					t.Errorf("parseBumpLevel() expected error, got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("parseBumpLevel() error = %v, should contain %q", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("parseBumpLevel() unexpected error: %v", err)
				}
			}

			if gotType != tt.wantType {
				t.Errorf("parseBumpLevel() type = %v, want %v", gotType, tt.wantType)
			}

			if gotAuto != tt.wantAuto {
				t.Errorf("parseBumpLevel() auto = %v, want %v", gotAuto, tt.wantAuto)
			}
		})
	}
}

func TestBumpCommand_FlagsExist(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
	}{
		{"level flag", "level"},
		{"prerelease flag", "prerelease"},
		{"build flag", "build"},
		{"force flag", "force"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := bumpCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("bump command missing %s flag", tt.flagName)
			}
		})
	}
}

func TestBumpCommand_DescriptionContent(t *testing.T) {
	if bumpCmd.Short == "" {
		t.Error("bump command should have a short description")
	}
	if bumpCmd.Long == "" {
		t.Error("bump command should have a long description")
	}
}

func TestBumpCommand_HasAliases(t *testing.T) {
	if len(bumpCmd.Aliases) == 0 {
		t.Error("bump command should have aliases")
	}

	hasVersionBumpAlias := false
	for _, alias := range bumpCmd.Aliases {
		if alias == "version-bump" {
			hasVersionBumpAlias = true
			break
		}
	}

	if !hasVersionBumpAlias {
		t.Error("bump command should have 'version-bump' alias")
	}
}

func TestBuildCalculateVersionInput(t *testing.T) {
	// Setup test config
	originalCfg := cfg
	defer func() { cfg = originalCfg }()

	cfg = &config.Config{
		Versioning: config.VersioningConfig{
			TagPrefix: "v",
		},
	}

	// Save and restore bumpPrerelease
	originalPrerelease := bumpPrerelease
	defer func() { bumpPrerelease = originalPrerelease }()

	tests := []struct {
		name           string
		bumpType       version.BumpType
		auto           bool
		prerelease     string
		wantPrerelease bool
	}{
		{
			name:           "major bump without prerelease",
			bumpType:       version.BumpMajor,
			auto:           false,
			prerelease:     "",
			wantPrerelease: false,
		},
		{
			name:           "minor bump with prerelease",
			bumpType:       version.BumpMinor,
			auto:           false,
			prerelease:     "beta.1",
			wantPrerelease: true,
		},
		{
			name:           "auto detection",
			bumpType:       version.BumpType(""),
			auto:           true,
			prerelease:     "",
			wantPrerelease: false,
		},
		{
			name:           "patch with alpha prerelease",
			bumpType:       version.BumpPatch,
			auto:           false,
			prerelease:     "alpha",
			wantPrerelease: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bumpPrerelease = tt.prerelease
			input := buildCalculateVersionInput(tt.bumpType, tt.auto)

			if input.BumpType != tt.bumpType {
				t.Errorf("buildCalculateVersionInput() BumpType = %v, want %v", input.BumpType, tt.bumpType)
			}

			if input.Auto != tt.auto {
				t.Errorf("buildCalculateVersionInput() Auto = %v, want %v", input.Auto, tt.auto)
			}

			if input.TagPrefix != "v" {
				t.Errorf("buildCalculateVersionInput() TagPrefix = %v, want v", input.TagPrefix)
			}

			if tt.wantPrerelease {
				if string(input.Prerelease) != tt.prerelease {
					t.Errorf("buildCalculateVersionInput() Prerelease = %v, want %v", input.Prerelease, tt.prerelease)
				}
			} else {
				if input.Prerelease != "" {
					t.Errorf("buildCalculateVersionInput() Prerelease = %v, want empty", input.Prerelease)
				}
			}
		})
	}
}

func TestOutputBumpJSON(t *testing.T) {
	// Setup test config
	originalCfg := cfg
	defer func() { cfg = originalCfg }()

	cfg = &config.Config{
		Versioning: config.VersioningConfig{
			TagPrefix: "v",
		},
	}

	current, _ := version.Parse("1.2.3")
	next, _ := version.Parse("1.3.0")

	tests := []struct {
		name         string
		current      version.SemanticVersion
		next         version.SemanticVersion
		bumpType     version.BumpType
		autoDetected bool
	}{
		{
			name:         "minor bump auto-detected",
			current:      current,
			next:         next,
			bumpType:     version.BumpMinor,
			autoDetected: true,
		},
		{
			name:         "manual major bump",
			current:      current,
			next:         next,
			bumpType:     version.BumpMajor,
			autoDetected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify it doesn't panic and returns no error
			err := outputBumpJSON(tt.current, tt.next, tt.bumpType, tt.autoDetected)
			if err != nil {
				t.Errorf("outputBumpJSON() error = %v", err)
			}
		})
	}
}

// bumpTestApp is a test cliApp implementation for bump tests.
type bumpTestApp struct {
	gitRepo   sourcecontrol.GitRepository
	calculate calculateVersionUseCase
}

func (b bumpTestApp) Close() error                                      { return nil }
func (b bumpTestApp) GitAdapter() sourcecontrol.GitRepository           { return b.gitRepo }
func (b bumpTestApp) ReleaseRepository() domainrelease.Repository       { return nil }
func (b bumpTestApp) ReleaseAnalyzer() *servicerelease.Analyzer         { return nil }
func (b bumpTestApp) CalculateVersion() calculateVersionUseCase         { return b.calculate }
func (b bumpTestApp) HasAI() bool                                       { return false }
func (b bumpTestApp) AI() ai.Service                                    { return nil }
func (b bumpTestApp) HasGovernance() bool                               { return false }
func (b bumpTestApp) GovernanceService() *governance.Service            { return nil }
func (b bumpTestApp) InitReleaseServices(context.Context, string) error { return nil }
func (b bumpTestApp) ReleaseServices() *domainrelease.Services          { return nil }
func (b bumpTestApp) HasReleaseServices() bool                          { return false }

// bumpGitRepo is a stub git repo for bump tests.
type bumpGitRepo struct {
	bumpStubGitRepo
	deleteTagErr error
}

func (b bumpGitRepo) DeleteTag(ctx context.Context, name string) error {
	return b.deleteTagErr
}

// bumpStubGitRepo provides minimal git repo stub for bump tests.
type bumpStubGitRepo struct{}

func (bumpStubGitRepo) GetInfo(ctx context.Context) (*sourcecontrol.RepositoryInfo, error) {
	return &sourcecontrol.RepositoryInfo{
		Path:          ".",
		Name:          "repo",
		CurrentBranch: "main",
		RemoteURL:     "https://example.com",
	}, nil
}
func (bumpStubGitRepo) GetRemotes(ctx context.Context) ([]sourcecontrol.RemoteInfo, error) {
	return nil, nil
}
func (bumpStubGitRepo) GetBranches(ctx context.Context) ([]sourcecontrol.BranchInfo, error) {
	return nil, nil
}
func (bumpStubGitRepo) GetCurrentBranch(ctx context.Context) (string, error) { return "main", nil }
func (bumpStubGitRepo) GetCommit(ctx context.Context, hash sourcecontrol.CommitHash) (*sourcecontrol.Commit, error) {
	return nil, nil
}
func (bumpStubGitRepo) GetCommitsBetween(ctx context.Context, from, to string) ([]*sourcecontrol.Commit, error) {
	return nil, nil
}
func (bumpStubGitRepo) GetCommitsSince(ctx context.Context, ref string) ([]*sourcecontrol.Commit, error) {
	return nil, nil
}
func (bumpStubGitRepo) GetLatestCommit(ctx context.Context, branch string) (*sourcecontrol.Commit, error) {
	return nil, nil
}
func (bumpStubGitRepo) GetCommitDiffStats(ctx context.Context, hash sourcecontrol.CommitHash) (*sourcecontrol.DiffStats, error) {
	return nil, nil
}
func (bumpStubGitRepo) GetCommitPatch(ctx context.Context, hash sourcecontrol.CommitHash) (string, error) {
	return "", nil
}
func (bumpStubGitRepo) GetFileAtRef(ctx context.Context, ref, path string) ([]byte, error) {
	return nil, nil
}
func (bumpStubGitRepo) GetTags(ctx context.Context) (sourcecontrol.TagList, error) { return nil, nil }
func (bumpStubGitRepo) GetTag(ctx context.Context, name string) (*sourcecontrol.Tag, error) {
	return nil, nil
}
func (bumpStubGitRepo) GetLatestVersionTag(ctx context.Context, prefix string) (*sourcecontrol.Tag, error) {
	return nil, nil
}
func (bumpStubGitRepo) CreateTag(ctx context.Context, name string, hash sourcecontrol.CommitHash, message string) (*sourcecontrol.Tag, error) {
	return nil, nil
}
func (bumpStubGitRepo) DeleteTag(ctx context.Context, name string) error              { return nil }
func (bumpStubGitRepo) PushTag(ctx context.Context, name string, remote string) error { return nil }
func (bumpStubGitRepo) IsDirty(ctx context.Context) (bool, error)                     { return false, nil }
func (bumpStubGitRepo) GetStatus(ctx context.Context) (*sourcecontrol.WorkingTreeStatus, error) {
	return &sourcecontrol.WorkingTreeStatus{IsClean: true}, nil
}
func (bumpStubGitRepo) Fetch(ctx context.Context, remote string) error        { return nil }
func (bumpStubGitRepo) Pull(ctx context.Context, remote, branch string) error { return nil }
func (bumpStubGitRepo) Push(ctx context.Context, remote, branch string) error { return nil }

// stubCalculateUseCase is a stub for the calculate version use case.
type stubCalculateUseCase struct {
	output *versioning.CalculateVersionOutput
	err    error
}

func (s *stubCalculateUseCase) Execute(ctx context.Context, input versioning.CalculateVersionInput) (*versioning.CalculateVersionOutput, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.output != nil {
		return s.output, nil
	}
	return &versioning.CalculateVersionOutput{
		CurrentVersion: version.MustParse("1.0.0"),
		NextVersion:    version.MustParse("1.1.0"),
		BumpType:       version.BumpMinor,
		AutoDetected:   true,
	}, nil
}

// captureStdoutBump captures stdout for bump tests.
func captureStdoutBump(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	fn()
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	os.Stdout = old
	return buf.String()
}

func TestRunBumpTagPush_VersionsMatch(t *testing.T) {
	origCfg := cfg
	origOutputJSON := outputJSON
	origDryRun := dryRun
	defer func() {
		cfg = origCfg
		outputJSON = origOutputJSON
		dryRun = origDryRun
	}()

	cfg = &config.Config{Versioning: config.VersioningConfig{TagPrefix: "v"}}
	outputJSON = true
	dryRun = true // Skip actual repo updates

	existingVer := version.MustParse("1.1.0")
	calcUC := &stubCalculateUseCase{
		output: &versioning.CalculateVersionOutput{
			CurrentVersion: version.MustParse("1.0.0"),
			NextVersion:    existingVer, // Same as existing tag
			BumpType:       version.BumpMinor,
			AutoDetected:   true,
		},
	}

	app := bumpTestApp{
		gitRepo:   bumpGitRepo{},
		calculate: calcUC,
	}

	out := captureStdoutBump(func() {
		if err := runBumpTagPush(context.Background(), app, existingVer); err != nil {
			t.Fatalf("runBumpTagPush error: %v", err)
		}
	})

	// Extract JSON from output
	jsonStart := strings.Index(out, "{")
	if jsonStart == -1 {
		t.Fatalf("no JSON found in output: %q", out)
	}
	jsonOutput := out[jsonStart:]

	var decoded map[string]any
	if err := json.Unmarshal([]byte(jsonOutput), &decoded); err != nil {
		t.Fatalf("json unmarshal error: %v\nJSON: %q", err, jsonOutput)
	}
	if decoded["mode"] != "tag-push" {
		t.Fatalf("expected mode=tag-push, got %v", decoded["mode"])
	}
	// When versions match, next_version should equal current
	// Note: tag_created field removed - tags are now created during publish
	if decoded["next_version"] != existingVer.String() {
		t.Fatalf("expected next_version=%s when versions match, got %v", existingVer.String(), decoded["next_version"])
	}
}

func TestRunBumpTagPush_VersionsDiffer(t *testing.T) {
	origCfg := cfg
	origOutputJSON := outputJSON
	origDryRun := dryRun
	defer func() {
		cfg = origCfg
		outputJSON = origOutputJSON
		dryRun = origDryRun
	}()

	cfg = &config.Config{Versioning: config.VersioningConfig{TagPrefix: "v"}}
	outputJSON = true
	dryRun = true // Don't actually try to create tags

	existingVer := version.MustParse("1.0.0")
	newVer := version.MustParse("1.1.0")
	calcUC := &stubCalculateUseCase{
		output: &versioning.CalculateVersionOutput{
			CurrentVersion: version.MustParse("0.9.0"),
			NextVersion:    newVer, // Different from existing tag
			BumpType:       version.BumpMinor,
			AutoDetected:   true,
		},
	}

	app := bumpTestApp{
		gitRepo:   bumpGitRepo{},
		calculate: calcUC,
	}

	out := captureStdoutBump(func() {
		if err := runBumpTagPush(context.Background(), app, existingVer); err != nil {
			t.Fatalf("runBumpTagPush error: %v", err)
		}
	})

	// Extract JSON from output
	jsonStart := strings.Index(out, "{")
	if jsonStart == -1 {
		t.Fatalf("no JSON found in output: %q", out)
	}
	jsonOutput := out[jsonStart:]

	var decoded map[string]any
	if err := json.Unmarshal([]byte(jsonOutput), &decoded); err != nil {
		t.Fatalf("json unmarshal error: %v\nJSON: %q", err, jsonOutput)
	}
	if decoded["mode"] != "tag-push" {
		t.Fatalf("expected mode=tag-push, got %v", decoded["mode"])
	}
	// next_version should be the calculated version
	if decoded["next_version"] != newVer.String() {
		t.Fatalf("expected next_version=%s, got %v", newVer.String(), decoded["next_version"])
	}
}

func TestRunBumpTagPush_CalculateError(t *testing.T) {
	origCfg := cfg
	origOutputJSON := outputJSON
	origDryRun := dryRun
	defer func() {
		cfg = origCfg
		outputJSON = origOutputJSON
		dryRun = origDryRun
	}()

	cfg = &config.Config{Versioning: config.VersioningConfig{TagPrefix: "v"}}
	outputJSON = true
	dryRun = true // Skip actual repo updates

	existingVer := version.MustParse("1.0.0")
	calcUC := &stubCalculateUseCase{
		err: errCalculateFailed,
	}

	app := bumpTestApp{
		gitRepo:   bumpGitRepo{},
		calculate: calcUC,
	}

	out := captureStdoutBump(func() {
		if err := runBumpTagPush(context.Background(), app, existingVer); err != nil {
			t.Fatalf("runBumpTagPush error: %v", err)
		}
	})

	// Should fall back to using existing version
	jsonStart := strings.Index(out, "{")
	if jsonStart == -1 {
		t.Fatalf("no JSON found in output: %q", out)
	}
	jsonOutput := out[jsonStart:]

	var decoded map[string]any
	if err := json.Unmarshal([]byte(jsonOutput), &decoded); err != nil {
		t.Fatalf("json unmarshal error: %v\nJSON: %q", err, jsonOutput)
	}
	// Should use existing version when calculate fails
	if decoded["next_version"] != existingVer.String() {
		t.Fatalf("expected next_version=%s on error fallback, got %v", existingVer.String(), decoded["next_version"])
	}
}

var errCalculateFailed = &calcError{s: "calculate failed"}

type calcError struct {
	s string
}

func (e *calcError) Error() string {
	return e.s
}

func TestFinishBumpTagPush_JSONOutput(t *testing.T) {
	origCfg := cfg
	origOutputJSON := outputJSON
	origDryRun := dryRun
	defer func() {
		cfg = origCfg
		outputJSON = origOutputJSON
		dryRun = origDryRun
	}()

	cfg = &config.Config{Versioning: config.VersioningConfig{TagPrefix: "v"}}
	outputJSON = true
	dryRun = true

	existingVer := version.MustParse("1.0.0")
	targetVer := version.MustParse("1.1.0")

	app := bumpTestApp{gitRepo: bumpGitRepo{}}

	out := captureStdoutBump(func() {
		if err := finishBumpTagPush(context.Background(), app, existingVer, targetVer, true); err != nil {
			t.Fatalf("finishBumpTagPush error: %v", err)
		}
	})

	var decoded map[string]any
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("json unmarshal error: %v\nJSON: %q", err, out)
	}
	if decoded["mode"] != "tag-push" {
		t.Fatalf("expected mode=tag-push, got %v", decoded["mode"])
	}
	if decoded["existing_tag"] != "v1.0.0" {
		t.Fatalf("expected existing_tag=v1.0.0, got %v", decoded["existing_tag"])
	}
	if decoded["current_version"] != "1.0.0" {
		t.Fatalf("expected current_version=1.0.0, got %v", decoded["current_version"])
	}
	if decoded["next_version"] != "1.1.0" {
		t.Fatalf("expected next_version=1.1.0, got %v", decoded["next_version"])
	}
	if decoded["tag_name"] != "v1.1.0" {
		t.Fatalf("expected tag_name=v1.1.0, got %v", decoded["tag_name"])
	}
}

func TestFinishBumpTagPush_TextOutput(t *testing.T) {
	origCfg := cfg
	origOutputJSON := outputJSON
	origDryRun := dryRun
	defer func() {
		cfg = origCfg
		outputJSON = origOutputJSON
		dryRun = origDryRun
	}()

	cfg = &config.Config{Versioning: config.VersioningConfig{TagPrefix: "v"}}
	outputJSON = false
	dryRun = true

	existingVer := version.MustParse("1.0.0")
	targetVer := version.MustParse("1.1.0")

	app := bumpTestApp{gitRepo: bumpGitRepo{}}

	out := captureStdoutBump(func() {
		if err := finishBumpTagPush(context.Background(), app, existingVer, targetVer, false); err != nil {
			t.Fatalf("finishBumpTagPush error: %v", err)
		}
	})

	// Should contain next steps
	if !strings.Contains(out, "Next Steps") {
		t.Fatalf("expected Next Steps in output, got %q", out)
	}
	if !strings.Contains(out, "relicta notes") {
		t.Fatalf("expected relicta notes in output, got %q", out)
	}
}

// Package release provides release analysis service tests.
package release

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/analysis"
	analysisfactory "github.com/relicta-tech/relicta/v4/internal/analysis/factory"
	"github.com/relicta-tech/relicta/v4/internal/domain/changes"
	"github.com/relicta-tech/relicta/v4/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

// mockGitRepo implements sourcecontrol.GitRepository for testing.
type mockGitRepo struct {
	info    *sourcecontrol.RepositoryInfo
	tags    sourcecontrol.TagList
	commits []*sourcecontrol.Commit
	err     error
}

func (m *mockGitRepo) GetInfo(ctx context.Context) (*sourcecontrol.RepositoryInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.info, nil
}

func (m *mockGitRepo) GetTags(ctx context.Context) (sourcecontrol.TagList, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tags, nil
}

func (m *mockGitRepo) GetCommitsBetween(ctx context.Context, from, to string) ([]*sourcecontrol.Commit, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.commits, nil
}

func (m *mockGitRepo) GetCommitDiffStats(ctx context.Context, hash sourcecontrol.CommitHash) (*sourcecontrol.DiffStats, error) {
	return &sourcecontrol.DiffStats{}, nil
}

func (m *mockGitRepo) GetBatchCommitDiffStats(ctx context.Context, hashes []sourcecontrol.CommitHash) (map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats, error) {
	result := make(map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats)
	for _, hash := range hashes {
		result[hash] = &sourcecontrol.DiffStats{}
	}
	return result, nil
}

// Remaining interface methods (not used in tests)
func (m *mockGitRepo) GetRemotes(ctx context.Context) ([]sourcecontrol.RemoteInfo, error) {
	return nil, nil
}
func (m *mockGitRepo) GetBranches(ctx context.Context) ([]sourcecontrol.BranchInfo, error) {
	return nil, nil
}
func (m *mockGitRepo) GetCurrentBranch(ctx context.Context) (string, error) { return "main", nil }
func (m *mockGitRepo) GetCommit(ctx context.Context, hash sourcecontrol.CommitHash) (*sourcecontrol.Commit, error) {
	return nil, nil
}
func (m *mockGitRepo) GetCommitsSince(ctx context.Context, ref string) ([]*sourcecontrol.Commit, error) {
	return nil, nil
}
func (m *mockGitRepo) GetLatestCommit(ctx context.Context, branch string) (*sourcecontrol.Commit, error) {
	return nil, nil
}
func (m *mockGitRepo) GetCommitPatch(ctx context.Context, hash sourcecontrol.CommitHash) (string, error) {
	return "", nil
}
func (m *mockGitRepo) GetFileAtRef(ctx context.Context, ref, path string) ([]byte, error) {
	return nil, nil
}
func (m *mockGitRepo) GetTag(ctx context.Context, name string) (*sourcecontrol.Tag, error) {
	return nil, nil
}
func (m *mockGitRepo) GetLatestVersionTag(ctx context.Context, prefix string) (*sourcecontrol.Tag, error) {
	return nil, nil
}
func (m *mockGitRepo) CreateTag(ctx context.Context, name string, hash sourcecontrol.CommitHash, message string) (*sourcecontrol.Tag, error) {
	return nil, nil
}
func (m *mockGitRepo) DeleteTag(ctx context.Context, name string) error { return nil }
func (m *mockGitRepo) PushTag(ctx context.Context, name string, remote string) error {
	return nil
}
func (m *mockGitRepo) IsDirty(ctx context.Context) (bool, error) { return false, nil }
func (m *mockGitRepo) GetStatus(ctx context.Context) (*sourcecontrol.WorkingTreeStatus, error) {
	return &sourcecontrol.WorkingTreeStatus{IsClean: true}, nil
}
func (m *mockGitRepo) Fetch(ctx context.Context, remote string) error        { return nil }
func (m *mockGitRepo) Pull(ctx context.Context, remote, branch string) error { return nil }
func (m *mockGitRepo) Push(ctx context.Context, remote, branch string) error { return nil }

// testVersionCalc implements version.VersionCalculator for testing.
type testVersionCalc struct {
	nextVersion version.SemanticVersion
}

func newTestVersionCalc() *testVersionCalc {
	v, _ := version.Parse("1.0.0")
	return &testVersionCalc{nextVersion: v}
}

func (m *testVersionCalc) CalculateNextVersion(current version.SemanticVersion, bump version.BumpType) version.SemanticVersion {
	return m.nextVersion
}

func (m *testVersionCalc) DetermineRequiredBump(hasBreaking, hasFeature, hasFix bool) version.BumpType {
	if hasBreaking {
		return version.BumpMajor
	}
	if hasFeature {
		return version.BumpMinor
	}
	if hasFix {
		return version.BumpPatch
	}
	return version.BumpPatch
}

// Helper to create test commits
func newTestCommit(hash, message string) *sourcecontrol.Commit {
	author := sourcecontrol.Author{Name: "Test User", Email: "test@example.com"}
	return sourcecontrol.NewCommit(
		sourcecontrol.CommitHash(hash),
		message,
		author,
		time.Now(),
	)
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MinConfidence != 0.7 {
		t.Errorf("expected MinConfidence 0.7, got %f", cfg.MinConfidence)
	}
	if !cfg.EnableAI {
		t.Error("expected EnableAI to be true")
	}
	if len(cfg.Languages) != 3 {
		t.Errorf("expected 3 languages, got %d", len(cfg.Languages))
	}
}

func TestAnalyzeInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   AnalyzeInput
		wantErr bool
	}{
		{
			name:    "empty input is valid",
			input:   AnalyzeInput{},
			wantErr: false,
		},
		{
			name: "valid input with all fields",
			input: AnalyzeInput{
				RepositoryPath: "/path/to/repo",
				Branch:         "main",
				FromRef:        "v1.0.0",
				ToRef:          "HEAD",
				TagPrefix:      "v",
			},
			wantErr: false,
		},
		{
			name: "path traversal in repository path",
			input: AnalyzeInput{
				RepositoryPath: "../../../etc/passwd",
			},
			wantErr: true,
		},
		{
			name: "invalid branch name with tilde",
			input: AnalyzeInput{
				Branch: "feature~test",
			},
			wantErr: true,
		},
		{
			name: "invalid branch name with caret",
			input: AnalyzeInput{
				Branch: "feature^test",
			},
			wantErr: true,
		},
		{
			name: "branch starts with slash",
			input: AnalyzeInput{
				Branch: "/feature",
			},
			wantErr: true,
		},
		{
			name: "branch ends with slash",
			input: AnalyzeInput{
				Branch: "feature/",
			},
			wantErr: true,
		},
		{
			name: "branch with double dots",
			input: AnalyzeInput{
				Branch: "feature..test",
			},
			wantErr: true,
		},
		{
			name: "tag prefix too long",
			input: AnalyzeInput{
				TagPrefix: "this-is-a-very-long-tag-prefix-that-exceeds-limit",
			},
			wantErr: true,
		},
		{
			name: "tag prefix with invalid characters",
			input: AnalyzeInput{
				TagPrefix: "v*",
			},
			wantErr: true,
		},
		{
			name: "invalid from reference",
			input: AnalyzeInput{
				FromRef: "tag:invalid",
			},
			wantErr: true,
		},
		{
			name: "invalid to reference",
			input: AnalyzeInput{
				ToRef: "ref?bad",
			},
			wantErr: true,
		},
		{
			name: "feature branch is valid",
			input: AnalyzeInput{
				Branch: "feature/add-user-auth",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewAnalyzer(t *testing.T) {
	gitRepo := &mockGitRepo{}
	versionCalc := newTestVersionCalc()
	factory := analysisfactory.NewFactory(nil)

	analyzer := NewAnalyzer(gitRepo, versionCalc, factory)

	if analyzer == nil {
		t.Fatal("expected non-nil analyzer")
	}
	if analyzer.gitRepo != gitRepo {
		t.Error("expected gitRepo to be set")
	}
}

func TestAnalyzer_Analyze_Success(t *testing.T) {
	v1, _ := version.Parse("1.0.0")

	gitRepo := &mockGitRepo{
		info: &sourcecontrol.RepositoryInfo{
			Name:          "test-repo",
			Owner:         "owner",
			CurrentBranch: "main",
		},
		tags: sourcecontrol.TagList{},
		commits: []*sourcecontrol.Commit{
			newTestCommit("abc123", "feat: add new feature"),
			newTestCommit("def456", "fix: fix bug"),
		},
	}
	versionCalc := &testVersionCalc{nextVersion: v1}
	factory := analysisfactory.NewFactory(nil)

	analyzer := NewAnalyzer(gitRepo, versionCalc, factory)

	input := AnalyzeInput{
		RepositoryPath: "/test/repo",
		Branch:         "main",
		TagPrefix:      "v",
	}

	output, err := analyzer.Analyze(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output == nil {
		t.Fatal("expected non-nil output")
	}
	if output.RepositoryName != "owner/test-repo" {
		t.Errorf("expected RepositoryName 'owner/test-repo', got %q", output.RepositoryName)
	}
	if output.Branch != "main" {
		t.Errorf("expected Branch 'main', got %q", output.Branch)
	}
	if output.ChangeSet == nil {
		t.Error("expected non-nil ChangeSet")
	}
}

func TestAnalyzer_Analyze_InvalidInput(t *testing.T) {
	gitRepo := &mockGitRepo{}
	versionCalc := newTestVersionCalc()
	factory := analysisfactory.NewFactory(nil)

	analyzer := NewAnalyzer(gitRepo, versionCalc, factory)

	input := AnalyzeInput{
		Branch: "feature~invalid",
	}

	_, err := analyzer.Analyze(context.Background(), input)
	if err == nil {
		t.Error("expected error for invalid input")
	}
}

func TestAnalyzer_Analyze_EmptyChangeSet(t *testing.T) {
	gitRepo := &mockGitRepo{
		info: &sourcecontrol.RepositoryInfo{
			Name:          "test-repo",
			CurrentBranch: "main",
		},
		tags:    sourcecontrol.TagList{},
		commits: []*sourcecontrol.Commit{},
	}
	versionCalc := newTestVersionCalc()
	factory := analysisfactory.NewFactory(nil)

	analyzer := NewAnalyzer(gitRepo, versionCalc, factory)

	input := AnalyzeInput{
		RepositoryPath: "/test/repo",
	}

	_, err := analyzer.Analyze(context.Background(), input)
	if err == nil {
		t.Error("expected error for empty commits")
	}
}

func TestAnalyzer_Analyze_ManualClassifications(t *testing.T) {
	v1, _ := version.Parse("1.0.0")

	gitRepo := &mockGitRepo{
		info: &sourcecontrol.RepositoryInfo{
			Name:          "test-repo",
			CurrentBranch: "main",
		},
		tags: sourcecontrol.TagList{},
		commits: []*sourcecontrol.Commit{
			newTestCommit("abc123", "some commit message"),
		},
	}
	versionCalc := &testVersionCalc{nextVersion: v1}
	factory := analysisfactory.NewFactory(nil)

	analyzer := NewAnalyzer(gitRepo, versionCalc, factory)

	input := AnalyzeInput{
		RepositoryPath: "/test/repo",
		CommitClassifications: map[sourcecontrol.CommitHash]*analysis.CommitClassification{
			"abc123": {
				Type:       changes.CommitTypeFeat,
				Scope:      "core",
				Method:     analysis.MethodManual,
				Confidence: 1.0,
			},
		},
	}

	output, err := analyzer.Analyze(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.ChangeSet == nil {
		t.Fatal("expected non-nil ChangeSet")
	}
}

func TestAnalyzer_AnalyzeCommits(t *testing.T) {
	gitRepo := &mockGitRepo{
		info: &sourcecontrol.RepositoryInfo{
			Name:          "test-repo",
			CurrentBranch: "main",
		},
		tags: sourcecontrol.TagList{},
		commits: []*sourcecontrol.Commit{
			newTestCommit("abc123", "feat: add feature"),
			newTestCommit("def456", "fix: fix issue"),
		},
	}
	versionCalc := newTestVersionCalc()
	factory := analysisfactory.NewFactory(nil)

	analyzer := NewAnalyzer(gitRepo, versionCalc, factory)

	input := AnalyzeInput{
		RepositoryPath: "/test/repo",
	}

	result, commitInfos, err := analyzer.AnalyzeCommits(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Error("expected non-nil result")
	}
	if len(commitInfos) != 2 {
		t.Errorf("expected 2 commit infos, got %d", len(commitInfos))
	}
}

func TestGetSubject(t *testing.T) {
	tests := []struct {
		message  string
		expected string
	}{
		{
			message:  "feat: add feature",
			expected: "feat: add feature",
		},
		{
			message:  "feat: add feature\n\nThis is the body",
			expected: "feat: add feature",
		},
		{
			message:  "  trimmed subject  \n\nbody",
			expected: "trimmed subject",
		},
		{
			message:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			result := getSubject(tt.message)
			if result != tt.expected {
				t.Errorf("getSubject(%q) = %q, want %q", tt.message, result, tt.expected)
			}
		})
	}
}

func TestClassificationToCommit(t *testing.T) {
	commit := newTestCommit("abc123", "some message")

	classification := &analysis.CommitClassification{
		Type:       changes.CommitTypeFeat,
		Scope:      "core",
		IsBreaking: false,
		Confidence: 0.9,
		Method:     analysis.MethodConventional,
	}

	result := classificationToCommit(commit, classification)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Type() != changes.CommitTypeFeat {
		t.Errorf("expected type 'feat', got %q", result.Type())
	}
	if result.Scope() != "core" {
		t.Errorf("expected scope 'core', got %q", result.Scope())
	}
}

func TestClassificationToCommit_Breaking(t *testing.T) {
	commit := newTestCommit("abc123", "feat!: breaking change")

	classification := &analysis.CommitClassification{
		Type:       changes.CommitTypeFeat,
		IsBreaking: true,
		Confidence: 1.0,
		Method:     analysis.MethodManual,
	}

	result := classificationToCommit(commit, classification)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.IsBreaking() {
		t.Error("expected breaking change to be set")
	}
}

func TestClassificationToCommit_EmptyType(t *testing.T) {
	commit := newTestCommit("abc123", "unknown format commit")

	classification := &analysis.CommitClassification{
		Type:       "",
		Confidence: 0.5,
		Method:     analysis.MethodHeuristic,
	}

	result := classificationToCommit(commit, classification)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// Empty type defaults to chore
	if result.Type() != changes.CommitTypeChore {
		t.Errorf("expected type 'chore' for empty classification, got %q", result.Type())
	}
}

func TestAnalyzer_GetCommitFiles(t *testing.T) {
	diffStats := &sourcecontrol.DiffStats{
		Files: []sourcecontrol.FileStats{
			{Path: "main.go"},
			{Path: "README.md"},
		},
	}

	gitRepo := &mockGitRepoWithDiffStats{
		mockGitRepo: mockGitRepo{
			info: &sourcecontrol.RepositoryInfo{
				Name:          "test-repo",
				CurrentBranch: "main",
			},
			tags:    sourcecontrol.TagList{},
			commits: []*sourcecontrol.Commit{},
		},
		diffStats: diffStats,
	}
	factory := analysisfactory.NewFactory(nil)
	analyzer := NewAnalyzer(gitRepo, newTestVersionCalc(), factory)

	files := analyzer.getCommitFiles(context.Background(), "abc123")
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}
	if len(files) > 0 && files[0] != "main.go" {
		t.Errorf("expected first file 'main.go', got %q", files[0])
	}
}

func TestAnalyzer_GetCommitFiles_Error(t *testing.T) {
	gitRepo := &mockGitRepoWithDiffStats{
		mockGitRepo: mockGitRepo{
			info: &sourcecontrol.RepositoryInfo{
				Name:          "test-repo",
				CurrentBranch: "main",
			},
		},
		diffStatsErr: fmt.Errorf("diff stats error"),
	}
	factory := analysisfactory.NewFactory(nil)
	analyzer := NewAnalyzer(gitRepo, newTestVersionCalc(), factory)

	files := analyzer.getCommitFiles(context.Background(), "abc123")
	if files != nil {
		t.Errorf("expected nil files on error, got %v", files)
	}
}

func TestAnalyzer_GetBatchCommitFiles_Empty(t *testing.T) {
	gitRepo := &mockGitRepo{}
	factory := analysisfactory.NewFactory(nil)
	analyzer := NewAnalyzer(gitRepo, newTestVersionCalc(), factory)

	result := analyzer.getBatchCommitFiles(context.Background(), nil)
	if result != nil {
		t.Errorf("expected nil for empty commits, got %v", result)
	}
}

func TestAnalyzer_GetBatchCommitFiles_Error(t *testing.T) {
	commits := []*sourcecontrol.Commit{
		newTestCommit("abc123", "feat: test"),
	}

	gitRepo := &mockGitRepoWithDiffStats{
		mockGitRepo: mockGitRepo{
			info: &sourcecontrol.RepositoryInfo{
				Name:          "test-repo",
				CurrentBranch: "main",
			},
		},
		batchDiffStatsErr: fmt.Errorf("batch error"),
		diffStats:         &sourcecontrol.DiffStats{Files: []sourcecontrol.FileStats{{Path: "main.go"}}},
	}
	factory := analysisfactory.NewFactory(nil)
	analyzer := NewAnalyzer(gitRepo, newTestVersionCalc(), factory)

	result := analyzer.getBatchCommitFiles(context.Background(), commits)
	if result == nil {
		t.Fatal("expected non-nil result from fallback")
	}
	// Should have fallen back to individual queries
	if len(result) != 1 {
		t.Errorf("expected 1 entry from fallback, got %d", len(result))
	}
}

func TestAnalyzer_Analyze_GitRepoError(t *testing.T) {
	gitRepo := &mockGitRepo{
		err: fmt.Errorf("git error"),
	}
	factory := analysisfactory.NewFactory(nil)
	analyzer := NewAnalyzer(gitRepo, newTestVersionCalc(), factory)

	input := AnalyzeInput{
		RepositoryPath: "/test/repo",
	}

	_, err := analyzer.Analyze(context.Background(), input)
	if err == nil {
		t.Error("expected error when git repo fails")
	}
}

// mockGitRepoWithDiffStats extends mockGitRepo with configurable diff stats.
type mockGitRepoWithDiffStats struct {
	mockGitRepo
	diffStats         *sourcecontrol.DiffStats
	diffStatsErr      error
	batchDiffStatsErr error
}

func (m *mockGitRepoWithDiffStats) GetCommitDiffStats(ctx context.Context, hash sourcecontrol.CommitHash) (*sourcecontrol.DiffStats, error) {
	if m.diffStatsErr != nil {
		return nil, m.diffStatsErr
	}
	return m.diffStats, nil
}

func (m *mockGitRepoWithDiffStats) GetBatchCommitDiffStats(ctx context.Context, hashes []sourcecontrol.CommitHash) (map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats, error) {
	if m.batchDiffStatsErr != nil {
		return nil, m.batchDiffStatsErr
	}
	result := make(map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats)
	for _, hash := range hashes {
		result[hash] = m.diffStats
	}
	return result, nil
}

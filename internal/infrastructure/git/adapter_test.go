package git

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/felixgeelhaar/release-pilot/internal/domain/sourcecontrol"
)

// mockService is a mock implementation of the Service interface for testing.
type mockService struct {
	// Repository info
	repoInfo    *RepositoryInfo
	repoInfoErr error
	repoRoot    string
	repoRootErr error
	isClean     bool
	isCleanErr  error

	// Commits
	commit          *Commit
	commitErr       error
	commits         []Commit
	commitsErr      error
	headCommit      *Commit
	headCommitErr   error
	branchCommit    *Commit
	branchCommitErr error

	// Tags
	tags         []Tag
	tagsErr      error
	tag          *Tag
	tagErr       error
	latestTag    *Tag
	latestTagErr error
	createTagErr error
	deleteTagErr error
	pushTagErr   error

	// Branches
	currentBranch    string
	currentBranchErr error
	defaultBranch    string
	defaultBranchErr error
	branches         []Branch
	branchesErr      error

	// Remote operations
	remoteURL    string
	remoteURLErr error
	pushErr      error
	pullErr      error
	fetchErr     error

	// Diff
	diffStats    *DiffStats
	diffStatsErr error
}

func (m *mockService) GetRepositoryRoot(ctx context.Context) (string, error) {
	return m.repoRoot, m.repoRootErr
}

func (m *mockService) GetRepositoryInfo(ctx context.Context) (*RepositoryInfo, error) {
	return m.repoInfo, m.repoInfoErr
}

func (m *mockService) IsClean(ctx context.Context) (bool, error) {
	return m.isClean, m.isCleanErr
}

func (m *mockService) GetCommit(ctx context.Context, hash string) (*Commit, error) {
	return m.commit, m.commitErr
}

func (m *mockService) GetCommitsSince(ctx context.Context, ref string) ([]Commit, error) {
	return m.commits, m.commitsErr
}

func (m *mockService) GetCommitsBetween(ctx context.Context, from, to string) ([]Commit, error) {
	return m.commits, m.commitsErr
}

func (m *mockService) GetHeadCommit(ctx context.Context) (*Commit, error) {
	return m.headCommit, m.headCommitErr
}

func (m *mockService) GetBranchCommit(ctx context.Context, branch string) (*Commit, error) {
	return m.branchCommit, m.branchCommitErr
}

func (m *mockService) GetLatestTag(ctx context.Context) (*Tag, error) {
	return m.latestTag, m.latestTagErr
}

func (m *mockService) GetLatestVersionTag(ctx context.Context, prefix string) (*Tag, error) {
	return m.latestTag, m.latestTagErr
}

func (m *mockService) ListTags(ctx context.Context) ([]Tag, error) {
	return m.tags, m.tagsErr
}

func (m *mockService) ListVersionTags(ctx context.Context, prefix string) ([]Tag, error) {
	return m.tags, m.tagsErr
}

func (m *mockService) GetTag(ctx context.Context, name string) (*Tag, error) {
	return m.tag, m.tagErr
}

func (m *mockService) CreateTag(ctx context.Context, name, message string, opts TagOptions) error {
	return m.createTagErr
}

func (m *mockService) DeleteTag(ctx context.Context, name string) error {
	return m.deleteTagErr
}

func (m *mockService) PushTag(ctx context.Context, name string, opts PushOptions) error {
	return m.pushTagErr
}

func (m *mockService) GetCurrentBranch(ctx context.Context) (string, error) {
	return m.currentBranch, m.currentBranchErr
}

func (m *mockService) GetDefaultBranch(ctx context.Context) (string, error) {
	return m.defaultBranch, m.defaultBranchErr
}

func (m *mockService) ListBranches(ctx context.Context) ([]Branch, error) {
	return m.branches, m.branchesErr
}

func (m *mockService) GetRemoteURL(ctx context.Context, name string) (string, error) {
	return m.remoteURL, m.remoteURLErr
}

func (m *mockService) Push(ctx context.Context, opts PushOptions) error {
	return m.pushErr
}

func (m *mockService) Pull(ctx context.Context, opts PullOptions) error {
	return m.pullErr
}

func (m *mockService) Fetch(ctx context.Context, opts FetchOptions) error {
	return m.fetchErr
}

func (m *mockService) GetDiffStats(ctx context.Context, from, to string) (*DiffStats, error) {
	return m.diffStats, m.diffStatsErr
}

func (m *mockService) ParseConventionalCommit(message string) (*ConventionalCommit, error) {
	return nil, nil
}

func (m *mockService) ParseConventionalCommits(commits []Commit, opts ParseOptions) ([]ConventionalCommit, error) {
	return nil, nil
}

func (m *mockService) DetectReleaseType(commits []ConventionalCommit) ReleaseType {
	return ReleaseTypePatch
}

func (m *mockService) CategorizeCommits(commits []ConventionalCommit) *CategorizedChanges {
	return nil
}

func (m *mockService) FilterCommits(commits []ConventionalCommit, filter CommitFilter) []ConventionalCommit {
	return nil
}

func TestNewAdapter(t *testing.T) {
	mock := &mockService{}
	adapter := NewAdapter(mock)
	if adapter == nil {
		t.Fatal("NewAdapter returned nil")
	}
	if adapter.svc != mock {
		t.Error("adapter service not set correctly")
	}
}

func TestAdapter_GetInfo(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockService{
			repoInfo: &RepositoryInfo{
				Root:          "/path/to/repo",
				CurrentBranch: "main",
				DefaultBranch: "main",
				IsDirty:       false,
				Remotes: []RemoteInfo{
					{Name: "origin", URL: "https://github.com/owner/repo.git"},
				},
			},
		}
		adapter := NewAdapter(mock)
		ctx := context.Background()

		info, err := adapter.GetInfo(ctx)
		if err != nil {
			t.Fatalf("GetInfo failed: %v", err)
		}

		if info.Path != "/path/to/repo" {
			t.Errorf("expected path /path/to/repo, got %s", info.Path)
		}
		if info.Name != "repo" {
			t.Errorf("expected name repo, got %s", info.Name)
		}
		if info.CurrentBranch != "main" {
			t.Errorf("expected branch main, got %s", info.CurrentBranch)
		}
		if info.RemoteURL != "https://github.com/owner/repo.git" {
			t.Errorf("expected remote URL, got %s", info.RemoteURL)
		}
		if info.Owner != "owner" {
			t.Errorf("expected owner owner, got %s", info.Owner)
		}
	})

	t.Run("error", func(t *testing.T) {
		mock := &mockService{
			repoInfoErr: errors.New("repo error"),
		}
		adapter := NewAdapter(mock)
		ctx := context.Background()

		_, err := adapter.GetInfo(ctx)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestAdapter_GetRemotes(t *testing.T) {
	mock := &mockService{
		repoInfo: &RepositoryInfo{
			Remotes: []RemoteInfo{
				{Name: "origin", URL: "https://github.com/owner/repo.git"},
				{Name: "upstream", URL: "https://github.com/upstream/repo.git"},
			},
		},
	}
	adapter := NewAdapter(mock)
	ctx := context.Background()

	remotes, err := adapter.GetRemotes(ctx)
	if err != nil {
		t.Fatalf("GetRemotes failed: %v", err)
	}

	if len(remotes) != 2 {
		t.Fatalf("expected 2 remotes, got %d", len(remotes))
	}
	if remotes[0].Name != "origin" {
		t.Errorf("expected first remote name origin, got %s", remotes[0].Name)
	}
	if remotes[1].Name != "upstream" {
		t.Errorf("expected second remote name upstream, got %s", remotes[1].Name)
	}
}

func TestAdapter_GetBranches(t *testing.T) {
	mock := &mockService{
		branches: []Branch{
			{Name: "main", Hash: "abc123", IsRemote: false},
			{Name: "feature", Hash: "def456", IsRemote: false},
			{Name: "origin/main", Hash: "abc123", IsRemote: true},
		},
		currentBranch: "main",
	}
	adapter := NewAdapter(mock)
	ctx := context.Background()

	branches, err := adapter.GetBranches(ctx)
	if err != nil {
		t.Fatalf("GetBranches failed: %v", err)
	}

	if len(branches) != 3 {
		t.Fatalf("expected 3 branches, got %d", len(branches))
	}
	if !branches[0].IsCurrent {
		t.Error("main branch should be current")
	}
	if branches[1].IsCurrent {
		t.Error("feature branch should not be current")
	}
	if !branches[2].IsRemote {
		t.Error("origin/main should be remote")
	}
}

func TestAdapter_GetCurrentBranch(t *testing.T) {
	mock := &mockService{currentBranch: "develop"}
	adapter := NewAdapter(mock)
	ctx := context.Background()

	branch, err := adapter.GetCurrentBranch(ctx)
	if err != nil {
		t.Fatalf("GetCurrentBranch failed: %v", err)
	}
	if branch != "develop" {
		t.Errorf("expected develop, got %s", branch)
	}
}

func TestAdapter_GetCommit(t *testing.T) {
	testTime := time.Now()
	mock := &mockService{
		commit: &Commit{
			Hash:    "abc123",
			Message: "test commit",
			Author:  Author{Name: "Test", Email: "test@test.com"},
			Date:    testTime,
		},
	}
	adapter := NewAdapter(mock)
	ctx := context.Background()

	commit, err := adapter.GetCommit(ctx, "abc123")
	if err != nil {
		t.Fatalf("GetCommit failed: %v", err)
	}
	if string(commit.Hash()) != "abc123" {
		t.Errorf("expected hash abc123, got %s", commit.Hash())
	}
	if commit.Message() != "test commit" {
		t.Errorf("expected message 'test commit', got %s", commit.Message())
	}
}

func TestAdapter_GetCommitsBetween(t *testing.T) {
	mock := &mockService{
		commits: []Commit{
			{Hash: "abc123", Message: "first"},
			{Hash: "def456", Message: "second"},
		},
	}
	adapter := NewAdapter(mock)
	ctx := context.Background()

	commits, err := adapter.GetCommitsBetween(ctx, "v1.0.0", "HEAD")
	if err != nil {
		t.Fatalf("GetCommitsBetween failed: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}
}

func TestAdapter_GetCommitsSince(t *testing.T) {
	mock := &mockService{
		commits: []Commit{
			{Hash: "abc123", Message: "first"},
		},
	}
	adapter := NewAdapter(mock)
	ctx := context.Background()

	commits, err := adapter.GetCommitsSince(ctx, "v1.0.0")
	if err != nil {
		t.Fatalf("GetCommitsSince failed: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}
}

func TestAdapter_GetLatestCommit(t *testing.T) {
	testTime := time.Now()

	t.Run("HEAD", func(t *testing.T) {
		mock := &mockService{
			headCommit: &Commit{Hash: "head123", Message: "head", Date: testTime},
		}
		adapter := NewAdapter(mock)
		ctx := context.Background()

		commit, err := adapter.GetLatestCommit(ctx, "")
		if err != nil {
			t.Fatalf("GetLatestCommit failed: %v", err)
		}
		if string(commit.Hash()) != "head123" {
			t.Errorf("expected head123, got %s", commit.Hash())
		}
	})

	t.Run("specific branch", func(t *testing.T) {
		mock := &mockService{
			branchCommit: &Commit{Hash: "branch123", Message: "branch", Date: testTime},
		}
		adapter := NewAdapter(mock)
		ctx := context.Background()

		commit, err := adapter.GetLatestCommit(ctx, "develop")
		if err != nil {
			t.Fatalf("GetLatestCommit failed: %v", err)
		}
		if string(commit.Hash()) != "branch123" {
			t.Errorf("expected branch123, got %s", commit.Hash())
		}
	})
}

func TestAdapter_GetTags(t *testing.T) {
	mock := &mockService{
		tags: []Tag{
			{Name: "v1.0.0", Hash: "abc123"},
			{Name: "v1.1.0", Hash: "def456"},
		},
	}
	adapter := NewAdapter(mock)
	ctx := context.Background()

	tags, err := adapter.GetTags(ctx)
	if err != nil {
		t.Fatalf("GetTags failed: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(tags))
	}
	if tags[0].Name() != "v1.0.0" {
		t.Errorf("expected v1.0.0, got %s", tags[0].Name())
	}
}

func TestAdapter_GetTag(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		mock := &mockService{
			tag: &Tag{Name: "v1.0.0", Hash: "abc123"},
		}
		adapter := NewAdapter(mock)
		ctx := context.Background()

		tag, err := adapter.GetTag(ctx, "v1.0.0")
		if err != nil {
			t.Fatalf("GetTag failed: %v", err)
		}
		if tag.Name() != "v1.0.0" {
			t.Errorf("expected v1.0.0, got %s", tag.Name())
		}
	})

	t.Run("not found", func(t *testing.T) {
		mock := &mockService{
			tag: nil,
		}
		adapter := NewAdapter(mock)
		ctx := context.Background()

		_, err := adapter.GetTag(ctx, "v1.0.0")
		if err != sourcecontrol.ErrTagNotFound {
			t.Errorf("expected ErrTagNotFound, got %v", err)
		}
	})
}

func TestAdapter_GetLatestVersionTag(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		mock := &mockService{
			latestTag: &Tag{Name: "v1.2.3", Hash: "abc123"},
		}
		adapter := NewAdapter(mock)
		ctx := context.Background()

		tag, err := adapter.GetLatestVersionTag(ctx, "v")
		if err != nil {
			t.Fatalf("GetLatestVersionTag failed: %v", err)
		}
		if tag.Name() != "v1.2.3" {
			t.Errorf("expected v1.2.3, got %s", tag.Name())
		}
	})

	t.Run("not found", func(t *testing.T) {
		mock := &mockService{
			latestTag: nil,
		}
		adapter := NewAdapter(mock)
		ctx := context.Background()

		tag, err := adapter.GetLatestVersionTag(ctx, "v")
		if err != nil {
			t.Fatalf("GetLatestVersionTag should not error: %v", err)
		}
		if tag != nil {
			t.Error("expected nil tag")
		}
	})
}

func TestAdapter_CreateTag(t *testing.T) {
	mock := &mockService{
		tag: &Tag{Name: "v1.0.0", Hash: "abc123"},
	}
	adapter := NewAdapter(mock)
	ctx := context.Background()

	tag, err := adapter.CreateTag(ctx, "v1.0.0", "abc123", "Release v1.0.0")
	if err != nil {
		t.Fatalf("CreateTag failed: %v", err)
	}
	if tag.Name() != "v1.0.0" {
		t.Errorf("expected v1.0.0, got %s", tag.Name())
	}
}

func TestAdapter_DeleteTag(t *testing.T) {
	mock := &mockService{}
	adapter := NewAdapter(mock)
	ctx := context.Background()

	err := adapter.DeleteTag(ctx, "v1.0.0")
	if err != nil {
		t.Fatalf("DeleteTag failed: %v", err)
	}
}

func TestAdapter_PushTag(t *testing.T) {
	mock := &mockService{}
	adapter := NewAdapter(mock)
	ctx := context.Background()

	err := adapter.PushTag(ctx, "v1.0.0", "origin")
	if err != nil {
		t.Fatalf("PushTag failed: %v", err)
	}
}

func TestAdapter_IsDirty(t *testing.T) {
	t.Run("clean", func(t *testing.T) {
		mock := &mockService{isClean: true}
		adapter := NewAdapter(mock)
		ctx := context.Background()

		dirty, err := adapter.IsDirty(ctx)
		if err != nil {
			t.Fatalf("IsDirty failed: %v", err)
		}
		if dirty {
			t.Error("expected not dirty")
		}
	})

	t.Run("dirty", func(t *testing.T) {
		mock := &mockService{isClean: false}
		adapter := NewAdapter(mock)
		ctx := context.Background()

		dirty, err := adapter.IsDirty(ctx)
		if err != nil {
			t.Fatalf("IsDirty failed: %v", err)
		}
		if !dirty {
			t.Error("expected dirty")
		}
	})
}

func TestAdapter_GetStatus(t *testing.T) {
	mock := &mockService{isClean: true}
	adapter := NewAdapter(mock)
	ctx := context.Background()

	status, err := adapter.GetStatus(ctx)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if !status.IsClean {
		t.Error("expected clean status")
	}
}

func TestAdapter_Fetch(t *testing.T) {
	mock := &mockService{}
	adapter := NewAdapter(mock)
	ctx := context.Background()

	err := adapter.Fetch(ctx, "origin")
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
}

func TestAdapter_Pull(t *testing.T) {
	mock := &mockService{}
	adapter := NewAdapter(mock)
	ctx := context.Background()

	err := adapter.Pull(ctx, "origin", "main")
	if err != nil {
		t.Fatalf("Pull failed: %v", err)
	}
}

func TestAdapter_Push(t *testing.T) {
	mock := &mockService{}
	adapter := NewAdapter(mock)
	ctx := context.Background()

	err := adapter.Push(ctx, "origin", "main")
	if err != nil {
		t.Fatalf("Push failed: %v", err)
	}
}

func TestConvertCommit(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		result := convertCommit(nil)
		if result != nil {
			t.Error("expected nil for nil input")
		}
	})

	t.Run("valid", func(t *testing.T) {
		testTime := time.Now()
		commit := &Commit{
			Hash:      "abc123",
			Message:   "test",
			Author:    Author{Name: "Author", Email: "author@test.com"},
			Committer: Author{Name: "Committer", Email: "committer@test.com"},
			Date:      testTime,
		}
		result := convertCommit(commit)
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if string(result.Hash()) != "abc123" {
			t.Errorf("expected abc123, got %s", result.Hash())
		}
	})
}

func TestExtractRepoName(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/path/to/repo", "repo"},
		{"C:\\Users\\test\\repo", "repo"},
		{"repo", "repo"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := extractRepoName(tt.path)
			if result != tt.expected {
				t.Errorf("extractRepoName(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestExtractOwner(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"https://github.com/owner/repo.git", "owner"},
		{"git@github.com:owner/repo.git", "owner"},
		{"ssh://git@github.com/owner/repo.git", "owner"},
		{"https://gitlab.com/owner/repo", "owner"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			result := extractOwner(tt.url)
			if result != tt.expected {
				t.Errorf("extractOwner(%q) = %q, want %q", tt.url, result, tt.expected)
			}
		})
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		path     string
		expected int
	}{
		{"/path/to/repo", 3},
		{"C:\\Users\\test\\repo", 4},
		{"single", 1},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := splitPath(tt.path)
			if len(result) != tt.expected {
				t.Errorf("splitPath(%q) returned %d parts, want %d", tt.path, len(result), tt.expected)
			}
		})
	}
}

func TestWithLocalTimeout(t *testing.T) {
	t.Run("applies timeout", func(t *testing.T) {
		ctx := context.Background()
		newCtx, cancel := withLocalTimeout(ctx)
		defer cancel()

		deadline, ok := newCtx.Deadline()
		if !ok {
			t.Error("expected deadline to be set")
		}
		if time.Until(deadline) > DefaultLocalTimeout {
			t.Error("deadline should be within DefaultLocalTimeout")
		}
	})

	t.Run("preserves shorter deadline", func(t *testing.T) {
		shortTimeout := 5 * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), shortTimeout)
		defer cancel()

		newCtx, newCancel := withLocalTimeout(ctx)
		defer newCancel()

		deadline, _ := newCtx.Deadline()
		originalDeadline, _ := ctx.Deadline()

		// The deadline should be the same (shorter one preserved)
		if !deadline.Equal(originalDeadline) {
			t.Error("shorter deadline should be preserved")
		}
	})
}

func TestWithRemoteTimeout(t *testing.T) {
	t.Run("applies timeout", func(t *testing.T) {
		ctx := context.Background()
		newCtx, cancel := withRemoteTimeout(ctx)
		defer cancel()

		deadline, ok := newCtx.Deadline()
		if !ok {
			t.Error("expected deadline to be set")
		}
		if time.Until(deadline) > DefaultRemoteTimeout {
			t.Error("deadline should be within DefaultRemoteTimeout")
		}
	})
}

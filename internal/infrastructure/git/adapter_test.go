// Package git provides infrastructure adapters for git operations.
package git

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/felixgeelhaar/release-pilot/internal/domain/sourcecontrol"
	gitservice "github.com/felixgeelhaar/release-pilot/internal/service/git"
)

// mockGitService is a mock implementation of gitservice.Service for testing.
type mockGitService struct {
	repoInfo       *gitservice.RepositoryInfo
	repoRoot       string
	currentBranch  string
	defaultBranch  string
	branches       []gitservice.Branch
	commits        []gitservice.Commit
	headCommit     *gitservice.Commit
	branchCommit   *gitservice.Commit
	tags           []gitservice.Tag
	latestTag      *gitservice.Tag
	latestVerTag   *gitservice.Tag
	tag            *gitservice.Tag
	isClean        bool
	diffStats      *gitservice.DiffStats
	remoteURL      string
	err            error
	createTagError error
	deleteTagError error
	pushTagError   error
	fetchError     error
	pullError      error
	pushError      error
}

func (m *mockGitService) GetRepositoryRoot(ctx context.Context) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.repoRoot, nil
}

func (m *mockGitService) GetRepositoryInfo(ctx context.Context) (*gitservice.RepositoryInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.repoInfo, nil
}

func (m *mockGitService) IsClean(ctx context.Context) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return m.isClean, nil
}

func (m *mockGitService) GetCommit(ctx context.Context, hash string) (*gitservice.Commit, error) {
	if m.err != nil {
		return nil, m.err
	}
	for i := range m.commits {
		if m.commits[i].Hash == hash {
			return &m.commits[i], nil
		}
	}
	return nil, nil
}

func (m *mockGitService) GetCommitsSince(ctx context.Context, ref string) ([]gitservice.Commit, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.commits, nil
}

func (m *mockGitService) GetCommitsBetween(ctx context.Context, from, to string) ([]gitservice.Commit, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.commits, nil
}

func (m *mockGitService) GetHeadCommit(ctx context.Context) (*gitservice.Commit, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.headCommit, nil
}

func (m *mockGitService) GetBranchCommit(ctx context.Context, branch string) (*gitservice.Commit, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.branchCommit, nil
}

func (m *mockGitService) GetLatestTag(ctx context.Context) (*gitservice.Tag, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.latestTag, nil
}

func (m *mockGitService) GetLatestVersionTag(ctx context.Context, prefix string) (*gitservice.Tag, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.latestVerTag, nil
}

func (m *mockGitService) ListTags(ctx context.Context) ([]gitservice.Tag, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tags, nil
}

func (m *mockGitService) ListVersionTags(ctx context.Context, prefix string) ([]gitservice.Tag, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tags, nil
}

func (m *mockGitService) GetTag(ctx context.Context, name string) (*gitservice.Tag, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tag, nil
}

func (m *mockGitService) CreateTag(ctx context.Context, name, message string, opts gitservice.TagOptions) error {
	if m.createTagError != nil {
		return m.createTagError
	}
	return nil
}

func (m *mockGitService) DeleteTag(ctx context.Context, name string) error {
	if m.deleteTagError != nil {
		return m.deleteTagError
	}
	return nil
}

func (m *mockGitService) PushTag(ctx context.Context, name string, opts gitservice.PushOptions) error {
	if m.pushTagError != nil {
		return m.pushTagError
	}
	return nil
}

func (m *mockGitService) GetCurrentBranch(ctx context.Context) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.currentBranch, nil
}

func (m *mockGitService) GetDefaultBranch(ctx context.Context) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.defaultBranch, nil
}

func (m *mockGitService) ListBranches(ctx context.Context) ([]gitservice.Branch, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.branches, nil
}

func (m *mockGitService) GetRemoteURL(ctx context.Context, name string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.remoteURL, nil
}

func (m *mockGitService) Push(ctx context.Context, opts gitservice.PushOptions) error {
	if m.pushError != nil {
		return m.pushError
	}
	return nil
}

func (m *mockGitService) Pull(ctx context.Context, opts gitservice.PullOptions) error {
	if m.pullError != nil {
		return m.pullError
	}
	return nil
}

func (m *mockGitService) Fetch(ctx context.Context, opts gitservice.FetchOptions) error {
	if m.fetchError != nil {
		return m.fetchError
	}
	return nil
}

func (m *mockGitService) GetDiffStats(ctx context.Context, from, to string) (*gitservice.DiffStats, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.diffStats, nil
}

func (m *mockGitService) ParseConventionalCommit(message string) (*gitservice.ConventionalCommit, error) {
	return nil, nil
}

func (m *mockGitService) ParseConventionalCommits(commits []gitservice.Commit, opts gitservice.ParseOptions) ([]gitservice.ConventionalCommit, error) {
	return nil, nil
}

func (m *mockGitService) DetectReleaseType(commits []gitservice.ConventionalCommit) gitservice.ReleaseType {
	return gitservice.ReleaseTypeNone
}

func (m *mockGitService) CategorizeCommits(commits []gitservice.ConventionalCommit) *gitservice.CategorizedChanges {
	return nil
}

func (m *mockGitService) FilterCommits(commits []gitservice.ConventionalCommit, filter gitservice.CommitFilter) []gitservice.ConventionalCommit {
	return nil
}

// TestNewAdapter tests the NewAdapter constructor.
func TestNewAdapter(t *testing.T) {
	mockSvc := &mockGitService{}
	adapter := NewAdapter(mockSvc)
	assert.NotNil(t, adapter)
	assert.Equal(t, mockSvc, adapter.svc)
}

// TestAdapterGetInfo tests the Adapter.GetInfo method.
func TestAdapterGetInfo(t *testing.T) {
	mockSvc := &mockGitService{
		repoInfo: &gitservice.RepositoryInfo{
			Root:          "/path/to/repo",
			CurrentBranch: "main",
			DefaultBranch: "main",
			IsDirty:       false,
			Remotes: []gitservice.RemoteInfo{
				{Name: "origin", URL: "https://github.com/owner/repo.git"},
			},
		},
	}

	adapter := NewAdapter(mockSvc)
	ctx := context.Background()

	info, err := adapter.GetInfo(ctx)
	require.NoError(t, err)
	assert.Equal(t, "/path/to/repo", info.Path)
	assert.Equal(t, "repo", info.Name)
	assert.Equal(t, "main", info.CurrentBranch)
	assert.Equal(t, "main", info.DefaultBranch)
	assert.False(t, info.IsDirty)
	assert.Equal(t, "https://github.com/owner/repo.git", info.RemoteURL)
	assert.Equal(t, "owner", info.Owner)
}

// TestAdapterGetRemotes tests the Adapter.GetRemotes method.
func TestAdapterGetRemotes(t *testing.T) {
	mockSvc := &mockGitService{
		repoInfo: &gitservice.RepositoryInfo{
			Remotes: []gitservice.RemoteInfo{
				{Name: "origin", URL: "https://github.com/owner/repo.git"},
				{Name: "upstream", URL: "https://github.com/upstream/repo.git"},
			},
		},
	}

	adapter := NewAdapter(mockSvc)
	ctx := context.Background()

	remotes, err := adapter.GetRemotes(ctx)
	require.NoError(t, err)
	assert.Len(t, remotes, 2)
	assert.Equal(t, "origin", remotes[0].Name)
	assert.Equal(t, "upstream", remotes[1].Name)
}

// TestAdapterGetBranches tests the Adapter.GetBranches method.
func TestAdapterGetBranches(t *testing.T) {
	mockSvc := &mockGitService{
		currentBranch: "main",
		branches: []gitservice.Branch{
			{Name: "main", Hash: "abc123", IsRemote: false},
			{Name: "feature", Hash: "def456", IsRemote: false},
			{Name: "origin/main", Hash: "abc123", IsRemote: true},
		},
	}

	adapter := NewAdapter(mockSvc)
	ctx := context.Background()

	branches, err := adapter.GetBranches(ctx)
	require.NoError(t, err)
	assert.Len(t, branches, 3)
	assert.True(t, branches[0].IsCurrent)
	assert.False(t, branches[1].IsCurrent)
	assert.True(t, branches[2].IsRemote)
}

// TestAdapterGetCurrentBranch tests the Adapter.GetCurrentBranch method.
func TestAdapterGetCurrentBranch(t *testing.T) {
	mockSvc := &mockGitService{
		currentBranch: "develop",
	}

	adapter := NewAdapter(mockSvc)
	ctx := context.Background()

	branch, err := adapter.GetCurrentBranch(ctx)
	require.NoError(t, err)
	assert.Equal(t, "develop", branch)
}

// TestAdapterGetCommit tests the Adapter.GetCommit method.
func TestAdapterGetCommit(t *testing.T) {
	now := time.Now()
	mockSvc := &mockGitService{
		commits: []gitservice.Commit{
			{
				Hash:    "abc123def456",
				Message: "feat: add new feature",
				Author:  gitservice.Author{Name: "John Doe", Email: "john@example.com"},
				Date:    now,
			},
		},
	}

	adapter := NewAdapter(mockSvc)
	ctx := context.Background()

	commit, err := adapter.GetCommit(ctx, "abc123def456")
	require.NoError(t, err)
	assert.Equal(t, sourcecontrol.CommitHash("abc123def456"), commit.Hash())
	assert.Equal(t, "feat: add new feature", commit.Message())
	assert.Equal(t, "John Doe", commit.Author().Name)
}

// TestAdapterGetCommitsBetween tests the Adapter.GetCommitsBetween method.
func TestAdapterGetCommitsBetween(t *testing.T) {
	now := time.Now()
	mockSvc := &mockGitService{
		commits: []gitservice.Commit{
			{Hash: "abc123", Message: "feat: first", Author: gitservice.Author{Name: "Author"}, Date: now},
			{Hash: "def456", Message: "fix: second", Author: gitservice.Author{Name: "Author"}, Date: now},
		},
	}

	adapter := NewAdapter(mockSvc)
	ctx := context.Background()

	commits, err := adapter.GetCommitsBetween(ctx, "v1.0.0", "HEAD")
	require.NoError(t, err)
	assert.Len(t, commits, 2)
}

// TestAdapterGetCommitsSince tests the Adapter.GetCommitsSince method.
func TestAdapterGetCommitsSince(t *testing.T) {
	now := time.Now()
	mockSvc := &mockGitService{
		commits: []gitservice.Commit{
			{Hash: "abc123", Message: "feat: first", Author: gitservice.Author{Name: "Author"}, Date: now},
		},
	}

	adapter := NewAdapter(mockSvc)
	ctx := context.Background()

	commits, err := adapter.GetCommitsSince(ctx, "v1.0.0")
	require.NoError(t, err)
	assert.Len(t, commits, 1)
}

// TestAdapterGetLatestCommit tests the Adapter.GetLatestCommit method.
func TestAdapterGetLatestCommit(t *testing.T) {
	now := time.Now()

	t.Run("empty branch gets HEAD", func(t *testing.T) {
		mockSvc := &mockGitService{
			headCommit: &gitservice.Commit{
				Hash:    "head123",
				Message: "HEAD commit",
				Author:  gitservice.Author{Name: "Author"},
				Date:    now,
			},
		}

		adapter := NewAdapter(mockSvc)
		ctx := context.Background()

		commit, err := adapter.GetLatestCommit(ctx, "")
		require.NoError(t, err)
		assert.Equal(t, sourcecontrol.CommitHash("head123"), commit.Hash())
	})

	t.Run("HEAD string gets HEAD", func(t *testing.T) {
		mockSvc := &mockGitService{
			headCommit: &gitservice.Commit{
				Hash:    "head123",
				Message: "HEAD commit",
				Author:  gitservice.Author{Name: "Author"},
				Date:    now,
			},
		}

		adapter := NewAdapter(mockSvc)
		ctx := context.Background()

		commit, err := adapter.GetLatestCommit(ctx, "HEAD")
		require.NoError(t, err)
		assert.Equal(t, sourcecontrol.CommitHash("head123"), commit.Hash())
	})

	t.Run("branch name gets branch commit", func(t *testing.T) {
		mockSvc := &mockGitService{
			branchCommit: &gitservice.Commit{
				Hash:    "branch123",
				Message: "Branch commit",
				Author:  gitservice.Author{Name: "Author"},
				Date:    now,
			},
		}

		adapter := NewAdapter(mockSvc)
		ctx := context.Background()

		commit, err := adapter.GetLatestCommit(ctx, "develop")
		require.NoError(t, err)
		assert.Equal(t, sourcecontrol.CommitHash("branch123"), commit.Hash())
	})
}

// TestAdapterGetTags tests the Adapter.GetTags method.
func TestAdapterGetTags(t *testing.T) {
	mockSvc := &mockGitService{
		tags: []gitservice.Tag{
			{Name: "v1.0.0", Hash: "abc123"},
			{Name: "v1.1.0", Hash: "def456"},
		},
	}

	adapter := NewAdapter(mockSvc)
	ctx := context.Background()

	tags, err := adapter.GetTags(ctx)
	require.NoError(t, err)
	assert.Len(t, tags, 2)
	assert.Equal(t, "v1.0.0", tags[0].Name())
	assert.Equal(t, sourcecontrol.CommitHash("abc123"), tags[0].Hash())
}

// TestAdapterGetTag tests the Adapter.GetTag method.
func TestAdapterGetTag(t *testing.T) {
	t.Run("existing tag", func(t *testing.T) {
		mockSvc := &mockGitService{
			tag: &gitservice.Tag{Name: "v1.0.0", Hash: "abc123"},
		}

		adapter := NewAdapter(mockSvc)
		ctx := context.Background()

		tag, err := adapter.GetTag(ctx, "v1.0.0")
		require.NoError(t, err)
		assert.Equal(t, "v1.0.0", tag.Name())
	})

	t.Run("non-existing tag", func(t *testing.T) {
		mockSvc := &mockGitService{
			tag: nil,
		}

		adapter := NewAdapter(mockSvc)
		ctx := context.Background()

		_, err := adapter.GetTag(ctx, "v2.0.0")
		assert.Equal(t, sourcecontrol.ErrTagNotFound, err)
	})
}

// TestAdapterGetLatestVersionTag tests the Adapter.GetLatestVersionTag method.
func TestAdapterGetLatestVersionTag(t *testing.T) {
	t.Run("with existing tags", func(t *testing.T) {
		mockSvc := &mockGitService{
			latestVerTag: &gitservice.Tag{Name: "v1.2.3", Hash: "abc123"},
		}

		adapter := NewAdapter(mockSvc)
		ctx := context.Background()

		tag, err := adapter.GetLatestVersionTag(ctx, "v")
		require.NoError(t, err)
		assert.Equal(t, "v1.2.3", tag.Name())
	})

	t.Run("without tags", func(t *testing.T) {
		mockSvc := &mockGitService{
			latestVerTag: nil,
		}

		adapter := NewAdapter(mockSvc)
		ctx := context.Background()

		tag, err := adapter.GetLatestVersionTag(ctx, "v")
		require.NoError(t, err)
		assert.Nil(t, tag)
	})
}

// TestAdapterCreateTag tests the Adapter.CreateTag method.
func TestAdapterCreateTag(t *testing.T) {
	mockSvc := &mockGitService{
		tag: &gitservice.Tag{Name: "v1.0.0", Hash: "abc123"},
	}

	adapter := NewAdapter(mockSvc)
	ctx := context.Background()

	tag, err := adapter.CreateTag(ctx, "v1.0.0", "abc123", "Release 1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "v1.0.0", tag.Name())
}

// TestAdapterDeleteTag tests the Adapter.DeleteTag method.
func TestAdapterDeleteTag(t *testing.T) {
	mockSvc := &mockGitService{}

	adapter := NewAdapter(mockSvc)
	ctx := context.Background()

	err := adapter.DeleteTag(ctx, "v1.0.0")
	require.NoError(t, err)
}

// TestAdapterPushTag tests the Adapter.PushTag method.
func TestAdapterPushTag(t *testing.T) {
	mockSvc := &mockGitService{}

	adapter := NewAdapter(mockSvc)
	ctx := context.Background()

	err := adapter.PushTag(ctx, "v1.0.0", "origin")
	require.NoError(t, err)
}

// TestAdapterIsDirty tests the Adapter.IsDirty method.
func TestAdapterIsDirty(t *testing.T) {
	t.Run("clean repo", func(t *testing.T) {
		mockSvc := &mockGitService{isClean: true}
		adapter := NewAdapter(mockSvc)

		dirty, err := adapter.IsDirty(context.Background())
		require.NoError(t, err)
		assert.False(t, dirty)
	})

	t.Run("dirty repo", func(t *testing.T) {
		mockSvc := &mockGitService{isClean: false}
		adapter := NewAdapter(mockSvc)

		dirty, err := adapter.IsDirty(context.Background())
		require.NoError(t, err)
		assert.True(t, dirty)
	})
}

// TestAdapterGetStatus tests the Adapter.GetStatus method.
func TestAdapterGetStatus(t *testing.T) {
	mockSvc := &mockGitService{isClean: true}
	adapter := NewAdapter(mockSvc)

	status, err := adapter.GetStatus(context.Background())
	require.NoError(t, err)
	assert.True(t, status.IsClean)
}

// TestAdapterFetch tests the Adapter.Fetch method.
func TestAdapterFetch(t *testing.T) {
	mockSvc := &mockGitService{}
	adapter := NewAdapter(mockSvc)

	err := adapter.Fetch(context.Background(), "origin")
	require.NoError(t, err)
}

// TestAdapterPull tests the Adapter.Pull method.
func TestAdapterPull(t *testing.T) {
	mockSvc := &mockGitService{}
	adapter := NewAdapter(mockSvc)

	err := adapter.Pull(context.Background(), "origin", "main")
	require.NoError(t, err)
}

// TestAdapterPush tests the Adapter.Push method.
func TestAdapterPush(t *testing.T) {
	mockSvc := &mockGitService{}
	adapter := NewAdapter(mockSvc)

	err := adapter.Push(context.Background(), "origin", "main")
	require.NoError(t, err)
}

// TestExtractRepoName tests the extractRepoName helper function.
func TestExtractRepoName(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/Users/dev/projects/my-repo", "my-repo"},
		{"/home/user/repo", "repo"},
		{"C:\\Users\\dev\\repo", "repo"},
		{"/single", "single"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := extractRepoName(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestExtractOwner tests the extractOwner helper function.
func TestExtractOwner(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"https://github.com/owner/repo.git", "owner"},
		{"https://github.com/owner/repo", "owner"},
		{"git@github.com:owner/repo.git", "owner"},
		{"git@github.com:owner/repo", "owner"},
		{"ssh://git@github.com/owner/repo.git", "owner"},
		{"https://gitlab.com/group/subgroup/repo.git", "group"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			result := extractOwner(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestSplitPath tests the splitPath helper function.
func TestSplitPath(t *testing.T) {
	tests := []struct {
		path     string
		expected []string
	}{
		{"/Users/dev/projects", []string{"Users", "dev", "projects"}},
		{"C:\\Users\\dev\\projects", []string{"C:", "Users", "dev", "projects"}},
		{"/single", []string{"single"}},
		{"", []string{}}, // strings.FieldsFunc returns empty slice, not nil
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := splitPath(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestWithLocalTimeout tests the withLocalTimeout helper function.
func TestWithLocalTimeout(t *testing.T) {
	t.Run("applies default timeout", func(t *testing.T) {
		ctx := context.Background()
		ctxWithTimeout, cancel := withLocalTimeout(ctx)
		defer cancel()

		deadline, ok := ctxWithTimeout.Deadline()
		assert.True(t, ok)
		assert.WithinDuration(t, time.Now().Add(DefaultLocalTimeout), deadline, time.Second)
	})

	t.Run("respects shorter existing deadline", func(t *testing.T) {
		shortTimeout := 5 * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), shortTimeout)
		defer cancel()

		ctxWithTimeout, cancelNew := withLocalTimeout(ctx)
		defer cancelNew()

		deadline, ok := ctxWithTimeout.Deadline()
		assert.True(t, ok)
		assert.WithinDuration(t, time.Now().Add(shortTimeout), deadline, time.Second)
	})
}

// TestWithRemoteTimeout tests the withRemoteTimeout helper function.
func TestWithRemoteTimeout(t *testing.T) {
	t.Run("applies default timeout", func(t *testing.T) {
		ctx := context.Background()
		ctxWithTimeout, cancel := withRemoteTimeout(ctx)
		defer cancel()

		deadline, ok := ctxWithTimeout.Deadline()
		assert.True(t, ok)
		assert.WithinDuration(t, time.Now().Add(DefaultRemoteTimeout), deadline, time.Second)
	})

	t.Run("respects shorter existing deadline", func(t *testing.T) {
		shortTimeout := 10 * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), shortTimeout)
		defer cancel()

		ctxWithTimeout, cancelNew := withRemoteTimeout(ctx)
		defer cancelNew()

		deadline, ok := ctxWithTimeout.Deadline()
		assert.True(t, ok)
		assert.WithinDuration(t, time.Now().Add(shortTimeout), deadline, time.Second)
	})
}

// TestConvertCommit tests the convertCommit helper function.
func TestConvertCommit(t *testing.T) {
	t.Run("nil commit", func(t *testing.T) {
		result := convertCommit(nil)
		assert.Nil(t, result)
	})

	t.Run("valid commit", func(t *testing.T) {
		now := time.Now()
		commit := &gitservice.Commit{
			Hash:      "abc123def456",
			Message:   "feat: add feature",
			Author:    gitservice.Author{Name: "Author", Email: "author@example.com"},
			Committer: gitservice.Author{Name: "Committer", Email: "committer@example.com"},
			Date:      now,
		}

		result := convertCommit(commit)
		assert.NotNil(t, result)
		assert.Equal(t, sourcecontrol.CommitHash("abc123def456"), result.Hash())
		assert.Equal(t, "feat: add feature", result.Message())
		assert.Equal(t, "Author", result.Author().Name)
		assert.Equal(t, "author@example.com", result.Author().Email)
	})
}

// TestConvertCommits tests the convertCommits helper function.
func TestConvertCommits(t *testing.T) {
	now := time.Now()
	commits := []gitservice.Commit{
		{Hash: "abc123", Message: "first", Author: gitservice.Author{Name: "Author"}, Date: now},
		{Hash: "def456", Message: "second", Author: gitservice.Author{Name: "Author"}, Date: now},
	}

	result := convertCommits(commits)
	assert.Len(t, result, 2)
	assert.Equal(t, sourcecontrol.CommitHash("abc123"), result[0].Hash())
	assert.Equal(t, sourcecontrol.CommitHash("def456"), result[1].Hash())
}

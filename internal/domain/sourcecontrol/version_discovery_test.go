package sourcecontrol

import (
	"context"
	"errors"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

// stubGitRepo is a minimal stub satisfying GitRepository for VersionDiscovery tests.
type stubGitRepo struct {
	latestTag    *Tag
	latestTagErr error
	tags         TagList
	tagsErr      error
}

func (s *stubGitRepo) GetInfo(context.Context) (*RepositoryInfo, error)       { return nil, nil }
func (s *stubGitRepo) GetRemotes(context.Context) ([]RemoteInfo, error)       { return nil, nil }
func (s *stubGitRepo) GetBranches(context.Context) ([]BranchInfo, error)      { return nil, nil }
func (s *stubGitRepo) GetCurrentBranch(context.Context) (string, error)       { return "", nil }
func (s *stubGitRepo) GetCommit(context.Context, CommitHash) (*Commit, error) { return nil, nil }
func (s *stubGitRepo) GetCommitsBetween(context.Context, string, string) ([]*Commit, error) {
	return nil, nil
}
func (s *stubGitRepo) GetCommitsSince(context.Context, string) ([]*Commit, error) { return nil, nil }
func (s *stubGitRepo) GetLatestCommit(context.Context, string) (*Commit, error)   { return nil, nil }
func (s *stubGitRepo) GetCommitDiffStats(context.Context, CommitHash) (*DiffStats, error) {
	return nil, nil
}
func (s *stubGitRepo) GetBatchCommitDiffStats(context.Context, []CommitHash) (map[CommitHash]*DiffStats, error) {
	return nil, nil
}
func (s *stubGitRepo) GetCommitPatch(context.Context, CommitHash) (string, error) { return "", nil }
func (s *stubGitRepo) GetFileAtRef(context.Context, string, string) ([]byte, error) {
	return nil, nil
}
func (s *stubGitRepo) GetTags(_ context.Context) (TagList, error) { return s.tags, s.tagsErr }
func (s *stubGitRepo) GetTag(context.Context, string) (*Tag, error) {
	return nil, nil
}
func (s *stubGitRepo) GetLatestVersionTag(_ context.Context, _ string) (*Tag, error) {
	return s.latestTag, s.latestTagErr
}
func (s *stubGitRepo) CreateTag(context.Context, string, CommitHash, string) (*Tag, error) {
	return nil, nil
}
func (s *stubGitRepo) DeleteTag(context.Context, string) error       { return nil }
func (s *stubGitRepo) PushTag(context.Context, string, string) error { return nil }
func (s *stubGitRepo) IsDirty(context.Context) (bool, error)         { return false, nil }
func (s *stubGitRepo) GetStatus(context.Context) (*WorkingTreeStatus, error) {
	return nil, nil
}
func (s *stubGitRepo) Fetch(context.Context, string) error        { return nil }
func (s *stubGitRepo) Pull(context.Context, string, string) error { return nil }
func (s *stubGitRepo) Push(context.Context, string, string) error { return nil }

func TestDiscoverCurrentVersion_ReturnsTagVersion(t *testing.T) {
	tag := NewTag("v1.2.3", CommitHash("abc"))
	repo := &stubGitRepo{latestTag: tag}
	vd := NewVersionDiscovery("v")

	got, err := vd.DiscoverCurrentVersion(context.Background(), repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := version.MustParse("1.2.3")
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestDiscoverCurrentVersion_NilTag(t *testing.T) {
	repo := &stubGitRepo{latestTag: nil}
	vd := NewVersionDiscovery("v")

	got, err := vd.DiscoverCurrentVersion(context.Background(), repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != version.Initial {
		t.Errorf("got %v, want Initial (%v)", got, version.Initial)
	}
}

func TestDiscoverCurrentVersion_Error(t *testing.T) {
	repo := &stubGitRepo{latestTagErr: errors.New("git error")}
	vd := NewVersionDiscovery("v")

	_, err := vd.DiscoverCurrentVersion(context.Background(), repo)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDiscoverAllVersions_Success(t *testing.T) {
	tags := TagList{
		NewTag("v1.0.0", CommitHash("a")),
		NewTag("v2.0.0", CommitHash("b")),
		NewTag("not-a-version", CommitHash("c")),
	}
	repo := &stubGitRepo{tags: tags}
	vd := NewVersionDiscovery("v")

	got, err := vd.DiscoverAllVersions(context.Background(), repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("got %d versions, want 2", len(got))
	}
}

func TestDiscoverAllVersions_Error(t *testing.T) {
	repo := &stubGitRepo{tagsErr: errors.New("tags error")}
	vd := NewVersionDiscovery("v")

	_, err := vd.DiscoverAllVersions(context.Background(), repo)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDiscoverAllVersions_Empty(t *testing.T) {
	repo := &stubGitRepo{tags: TagList{}}
	vd := NewVersionDiscovery("v")

	got, err := vd.DiscoverAllVersions(context.Background(), repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d versions, want 0", len(got))
	}
}

package versioning

import (
	"context"
	"errors"
	"testing"

	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

// mockGitRepo is a minimal GitRepository implementation for testing.
type mockGitRepo struct {
	tags      sourcecontrol.TagList
	tagErr    error
	getTag    *sourcecontrol.Tag
	getErr    error
	created   *sourcecontrol.Tag
	createErr error
}

func (m *mockGitRepo) GetTags(_ context.Context) (sourcecontrol.TagList, error) {
	return m.tags, m.tagErr
}

func (m *mockGitRepo) GetTag(_ context.Context, name string) (*sourcecontrol.Tag, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.getTag != nil {
		return m.getTag, nil
	}
	// Search in tags list.
	for _, t := range m.tags {
		if t.Name() == name {
			return t, nil
		}
	}
	return nil, errors.New("tag not found: " + name)
}

func (m *mockGitRepo) GetLatestVersionTag(_ context.Context, _ string) (*sourcecontrol.Tag, error) {
	return m.getTag, m.getErr
}

func (m *mockGitRepo) CreateTag(_ context.Context, name string, hash sourcecontrol.CommitHash, _ string) (*sourcecontrol.Tag, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	t := sourcecontrol.NewTag(name, hash)
	m.created = t
	return t, nil
}

func (m *mockGitRepo) DeleteTag(_ context.Context, _ string) error  { return nil }
func (m *mockGitRepo) PushTag(_ context.Context, _, _ string) error { return nil }
func (m *mockGitRepo) GetInfo(_ context.Context) (*sourcecontrol.RepositoryInfo, error) {
	return &sourcecontrol.RepositoryInfo{}, nil
}
func (m *mockGitRepo) GetRemotes(_ context.Context) ([]sourcecontrol.RemoteInfo, error) {
	return nil, nil
}
func (m *mockGitRepo) GetBranches(_ context.Context) ([]sourcecontrol.BranchInfo, error) {
	return nil, nil
}
func (m *mockGitRepo) GetCurrentBranch(_ context.Context) (string, error) { return "main", nil }
func (m *mockGitRepo) GetCommit(_ context.Context, _ sourcecontrol.CommitHash) (*sourcecontrol.Commit, error) {
	return nil, nil
}
func (m *mockGitRepo) GetCommitsBetween(_ context.Context, _, _ string) ([]*sourcecontrol.Commit, error) {
	return nil, nil
}
func (m *mockGitRepo) GetCommitsSince(_ context.Context, _ string) ([]*sourcecontrol.Commit, error) {
	return nil, nil
}
func (m *mockGitRepo) GetLatestCommit(_ context.Context, _ string) (*sourcecontrol.Commit, error) {
	return nil, nil
}
func (m *mockGitRepo) GetCommitDiffStats(_ context.Context, _ sourcecontrol.CommitHash) (*sourcecontrol.DiffStats, error) {
	return nil, nil
}
func (m *mockGitRepo) GetBatchCommitDiffStats(_ context.Context, _ []sourcecontrol.CommitHash) (map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats, error) {
	return nil, nil
}
func (m *mockGitRepo) GetCommitPatch(_ context.Context, _ sourcecontrol.CommitHash) (string, error) {
	return "", nil
}
func (m *mockGitRepo) GetFileAtRef(_ context.Context, _, _ string) ([]byte, error) { return nil, nil }
func (m *mockGitRepo) IsDirty(_ context.Context) (bool, error)                     { return false, nil }
func (m *mockGitRepo) GetStatus(_ context.Context) (*sourcecontrol.WorkingTreeStatus, error) {
	return nil, nil
}
func (m *mockGitRepo) Fetch(_ context.Context, _ string) error   { return nil }
func (m *mockGitRepo) Pull(_ context.Context, _, _ string) error { return nil }
func (m *mockGitRepo) Push(_ context.Context, _, _ string) error { return nil }

// TestFindLatestChannelVersion_Alpha exercises the findLatestChannelVersion for alpha channel.
func TestFindLatestChannelVersion_Alpha(t *testing.T) {
	t.Parallel()

	tags := sourcecontrol.TagList{
		sourcecontrol.NewTag("v1.0.0-alpha.1", "hash1"),
		sourcecontrol.NewTag("v1.0.0-alpha.2", "hash2"),
		sourcecontrol.NewTag("v1.0.0-alpha.3", "hash3"),
		sourcecontrol.NewTag("v1.0.0-beta.1", "hash4"), // different channel
		sourcecontrol.NewTag("v1.0.0", "hash5"),        // stable
	}

	repo := &mockGitRepo{tags: tags}
	registry := version.NewChannelRegistry()
	uc := NewPromoteReleaseUseCase(repo, registry)

	input := PromoteReleaseInput{
		FromChannel: "alpha",
		ToChannel:   "beta",
		DryRun:      true,
	}

	output, err := uc.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if output.SourceVersion.String() != "1.0.0-alpha.3" {
		t.Errorf("SourceVersion = %s, want 1.0.0-alpha.3", output.SourceVersion.String())
	}
}

// TestFindLatestChannelVersion_Next promotes from "next" (rc) to stable by finding
// the latest rc tag automatically.
func TestFindLatestChannelVersion_Stable(t *testing.T) {
	t.Parallel()

	// Test by promoting from "next" (rc) to stable without specifying version.
	tags2 := sourcecontrol.TagList{
		sourcecontrol.NewTag("v1.0.0-rc.1", "hash1"),
		sourcecontrol.NewTag("v1.0.0-rc.2", "hash2"),
	}
	repo2 := &mockGitRepo{tags: tags2}
	registry2 := version.NewChannelRegistry()
	uc2 := NewPromoteReleaseUseCase(repo2, registry2)

	input := PromoteReleaseInput{
		FromChannel: "next",
		ToChannel:   "stable",
		DryRun:      true,
	}

	output, err := uc2.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if output.SourceVersion.String() != "1.0.0-rc.2" {
		t.Errorf("SourceVersion = %s, want 1.0.0-rc.2", output.SourceVersion.String())
	}
	if output.TargetVersion.String() != "1.0.0" {
		t.Errorf("TargetVersion = %s, want 1.0.0", output.TargetVersion.String())
	}
}

// TestFindLatestChannelVersion_NoVersionsFound returns error when no matching versions exist.
func TestFindLatestChannelVersion_NoVersionsFound(t *testing.T) {
	t.Parallel()

	// Only beta tags, looking for alpha.
	tags := sourcecontrol.TagList{
		sourcecontrol.NewTag("v1.0.0-beta.1", "hash1"),
	}

	repo := &mockGitRepo{tags: tags}
	registry := version.NewChannelRegistry()
	uc := NewPromoteReleaseUseCase(repo, registry)

	input := PromoteReleaseInput{
		FromChannel: "alpha",
		ToChannel:   "beta",
		DryRun:      true,
	}

	_, err := uc.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error when no alpha versions found")
	}
}

// TestFindLatestChannelVersion_GetTagsError returns error when GetTags fails.
func TestFindLatestChannelVersion_GetTagsError(t *testing.T) {
	t.Parallel()

	repo := &mockGitRepo{tagErr: errors.New("git error")}
	registry := version.NewChannelRegistry()
	uc := NewPromoteReleaseUseCase(repo, registry)

	input := PromoteReleaseInput{
		FromChannel: "alpha",
		ToChannel:   "beta",
		DryRun:      true,
	}

	_, err := uc.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error when GetTags fails")
	}
}

// TestPromoteReleaseUseCase_NonDryRun exercises the actual tag creation path.
func TestPromoteReleaseUseCase_NonDryRun(t *testing.T) {
	t.Parallel()

	hash := sourcecontrol.CommitHash("abc123")
	sourceTag := sourcecontrol.NewTag("v1.0.0-alpha.1", hash)

	repo := &mockGitRepo{
		tags: sourcecontrol.TagList{sourceTag},
	}
	registry := version.NewChannelRegistry()
	uc := NewPromoteReleaseUseCase(repo, registry)

	input := PromoteReleaseInput{
		FromChannel: "alpha",
		ToChannel:   "beta",
		Version:     "1.0.0-alpha.1",
		DryRun:      false,
	}

	output, err := uc.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if output.TargetVersion.String() != "1.0.0-beta.1" {
		t.Errorf("TargetVersion = %s, want 1.0.0-beta.1", output.TargetVersion.String())
	}

	if repo.created == nil {
		t.Error("expected a tag to be created")
	}
}

// TestPromoteReleaseUseCase_NonDryRun_GetTagFails tests error when source tag not found.
func TestPromoteReleaseUseCase_NonDryRun_GetTagFails(t *testing.T) {
	t.Parallel()

	repo := &mockGitRepo{getErr: errors.New("tag not found")}
	registry := version.NewChannelRegistry()
	uc := NewPromoteReleaseUseCase(repo, registry)

	input := PromoteReleaseInput{
		FromChannel: "alpha",
		ToChannel:   "beta",
		Version:     "1.0.0-alpha.1",
		DryRun:      false,
	}

	_, err := uc.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error when source tag not found")
	}
}

// TestPromoteReleaseUseCase_NonDryRun_CreateTagFails tests error on tag creation failure.
func TestPromoteReleaseUseCase_NonDryRun_CreateTagFails(t *testing.T) {
	t.Parallel()

	hash := sourcecontrol.CommitHash("abc123")
	sourceTag := sourcecontrol.NewTag("v1.0.0-alpha.1", hash)

	repo := &mockGitRepo{
		tags:      sourcecontrol.TagList{sourceTag},
		createErr: errors.New("push failed"),
	}
	registry := version.NewChannelRegistry()
	uc := NewPromoteReleaseUseCase(repo, registry)

	input := PromoteReleaseInput{
		FromChannel: "alpha",
		ToChannel:   "beta",
		Version:     "1.0.0-alpha.1",
		DryRun:      false,
	}

	_, err := uc.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error when CreateTag fails")
	}
}

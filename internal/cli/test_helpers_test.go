// Package cli provides the command-line interface for Relicta.
package cli

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta/internal/application/governance"
	"github.com/relicta-tech/relicta/internal/config"
	domainrelease "github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/internal/domain/version"
	"github.com/relicta-tech/relicta/internal/infrastructure/ai"
	servicerelease "github.com/relicta-tech/relicta/internal/service/release"
)

// commandTestApp provides a minimal cliApp implementation for testing.
// Embed this in test-specific app types to satisfy the interface.
type commandTestApp struct {
	gitRepo       sourcecontrol.GitRepository
	releaseRepo   domainrelease.Repository
	govSvc        *governance.Service
	hasGov        bool
	setVersionUC  setVersionUseCase
	calcVersionUC calculateVersionUseCase
}

func (a commandTestApp) Close() error                                      { return nil }
func (a commandTestApp) GitAdapter() sourcecontrol.GitRepository           { return a.gitRepo }
func (a commandTestApp) ReleaseRepository() domainrelease.Repository       { return a.releaseRepo }
func (a commandTestApp) ReleaseAnalyzer() *servicerelease.Analyzer         { return nil }
func (a commandTestApp) CalculateVersion() calculateVersionUseCase         { return a.calcVersionUC }
func (a commandTestApp) SetVersion() setVersionUseCase                     { return a.setVersionUC }
func (a commandTestApp) HasAI() bool                                       { return false }
func (a commandTestApp) AI() ai.Service                                    { return nil }
func (a commandTestApp) HasGovernance() bool                               { return a.hasGov }
func (a commandTestApp) GovernanceService() *governance.Service            { return a.govSvc }
func (a commandTestApp) InitReleaseServices(context.Context, string) error { return nil }
func (a commandTestApp) ReleaseServices() *domainrelease.Services          { return nil }
func (a commandTestApp) HasReleaseServices() bool                          { return false }

// testCLIApp is an alias for commandTestApp for backward compatibility.
type testCLIApp = commandTestApp

// stubGitRepo provides a minimal git repository stub for testing.
type stubGitRepo struct{}

// RepositoryInfoReader methods
func (stubGitRepo) GetInfo(ctx context.Context) (*sourcecontrol.RepositoryInfo, error) {
	return &sourcecontrol.RepositoryInfo{
		Path:          ".",
		Name:          "test-repo",
		CurrentBranch: "main",
		RemoteURL:     "https://example.com/test-repo.git",
	}, nil
}
func (stubGitRepo) GetRemotes(ctx context.Context) ([]sourcecontrol.RemoteInfo, error) {
	return nil, nil
}
func (stubGitRepo) GetBranches(ctx context.Context) ([]sourcecontrol.BranchInfo, error) {
	return nil, nil
}
func (stubGitRepo) GetCurrentBranch(ctx context.Context) (string, error) { return "main", nil }

// CommitReader methods
func (stubGitRepo) GetCommit(ctx context.Context, hash sourcecontrol.CommitHash) (*sourcecontrol.Commit, error) {
	return nil, nil
}
func (stubGitRepo) GetCommitsBetween(ctx context.Context, from, to string) ([]*sourcecontrol.Commit, error) {
	return nil, nil
}
func (stubGitRepo) GetCommitsSince(ctx context.Context, ref string) ([]*sourcecontrol.Commit, error) {
	return nil, nil
}
func (stubGitRepo) GetLatestCommit(ctx context.Context, branch string) (*sourcecontrol.Commit, error) {
	return nil, nil
}

// DiffReader methods
func (stubGitRepo) GetCommitDiffStats(ctx context.Context, hash sourcecontrol.CommitHash) (*sourcecontrol.DiffStats, error) {
	return nil, nil
}
func (stubGitRepo) GetCommitPatch(ctx context.Context, hash sourcecontrol.CommitHash) (string, error) {
	return "", nil
}
func (stubGitRepo) GetFileAtRef(ctx context.Context, ref, path string) ([]byte, error) {
	return nil, nil
}

// TagReader methods
func (stubGitRepo) GetTags(ctx context.Context) (sourcecontrol.TagList, error) {
	return nil, nil
}
func (stubGitRepo) GetTag(ctx context.Context, name string) (*sourcecontrol.Tag, error) {
	return nil, nil
}
func (stubGitRepo) GetLatestVersionTag(ctx context.Context, prefix string) (*sourcecontrol.Tag, error) {
	return nil, nil
}

// TagWriter methods
func (stubGitRepo) CreateTag(ctx context.Context, name string, hash sourcecontrol.CommitHash, message string) (*sourcecontrol.Tag, error) {
	tag := sourcecontrol.NewTag(name, hash)
	tag.SetMessage(message)
	return tag, nil
}
func (stubGitRepo) DeleteTag(ctx context.Context, name string) error { return nil }
func (stubGitRepo) PushTag(ctx context.Context, name string, remote string) error {
	return nil
}

// WorkingTreeInspector methods
func (stubGitRepo) IsDirty(ctx context.Context) (bool, error) { return false, nil }
func (stubGitRepo) GetStatus(ctx context.Context) (*sourcecontrol.WorkingTreeStatus, error) {
	return &sourcecontrol.WorkingTreeStatus{IsClean: true}, nil
}

// RemoteOperator methods
func (stubGitRepo) Fetch(ctx context.Context, remote string) error        { return nil }
func (stubGitRepo) Pull(ctx context.Context, remote, branch string) error { return nil }
func (stubGitRepo) Push(ctx context.Context, remote, branch string) error { return nil }

// testReleaseRepo provides a minimal release repository stub for testing.
type testReleaseRepo struct {
	latest *domainrelease.ReleaseRun
	err    error
}

func (r testReleaseRepo) FindByID(ctx context.Context, id domainrelease.RunID) (*domainrelease.ReleaseRun, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.latest, nil
}
func (r testReleaseRepo) FindActive(ctx context.Context) ([]*domainrelease.ReleaseRun, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.latest != nil {
		return []*domainrelease.ReleaseRun{r.latest}, nil
	}
	return nil, nil
}
func (r testReleaseRepo) FindLatest(ctx context.Context, repoPath string) (*domainrelease.ReleaseRun, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.latest, nil
}
func (r testReleaseRepo) FindByState(ctx context.Context, state domainrelease.RunState) ([]*domainrelease.ReleaseRun, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.latest != nil {
		return []*domainrelease.ReleaseRun{r.latest}, nil
	}
	return nil, nil
}
func (r testReleaseRepo) FindBySpecification(ctx context.Context, spec domainrelease.Specification) ([]*domainrelease.ReleaseRun, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.latest != nil {
		return []*domainrelease.ReleaseRun{r.latest}, nil
	}
	return nil, nil
}
func (r testReleaseRepo) Save(ctx context.Context, release *domainrelease.ReleaseRun) error {
	return r.err
}
func (r testReleaseRepo) List(ctx context.Context, repoPath string) ([]domainrelease.RunID, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.latest != nil {
		return []domainrelease.RunID{r.latest.ID()}, nil
	}
	return nil, nil
}
func (r testReleaseRepo) Delete(ctx context.Context, id domainrelease.RunID) error {
	return r.err
}

// withStubContainerApp replaces newContainerApp with a test implementation.
func withStubContainerApp(t *testing.T, app cliApp) {
	t.Helper()
	orig := newContainerApp
	newContainerApp = func(ctx context.Context, cfg *config.Config) (cliApp, error) {
		return app, nil
	}
	t.Cleanup(func() {
		newContainerApp = orig
	})
}

// newTestRelease creates a test release in draft state for testing.
func newTestRelease(t *testing.T, id string) *domainrelease.ReleaseRun {
	t.Helper()
	return domainrelease.NewReleaseRunForTest(domainrelease.RunID(id), "main", ".")
}

// newTestReleaseWithCommits creates a test release with commit data for governance testing.
func newTestReleaseWithCommits(t *testing.T, id string) *domainrelease.ReleaseRun {
	t.Helper()
	return domainrelease.NewReleaseRunForTestWithCommits(domainrelease.RunID(id), "main", ".")
}

// newNotesReadyRelease creates a test release in notes_ready state for testing approvals.
func newNotesReadyRelease(t *testing.T, id string) *domainrelease.ReleaseRun {
	t.Helper()
	rel := domainrelease.NewReleaseRunForTest(domainrelease.RunID(id), "main", ".")
	if err := rel.Plan("test"); err != nil {
		t.Fatalf("failed to plan release: %v", err)
	}
	ver, err := version.Parse("1.0.0")
	if err != nil {
		t.Fatalf("failed to parse version: %v", err)
	}
	if err := rel.SetVersion(ver, "v1.0.0"); err != nil {
		t.Fatalf("failed to version release: %v", err)
	}
	if err := rel.Bump("test"); err != nil {
		t.Fatalf("failed to bump release: %v", err)
	}
	notes := &domain.ReleaseNotes{Text: "Test release notes"}
	if err := rel.GenerateNotes(notes, "test-hash", "test"); err != nil {
		t.Fatalf("failed to add notes: %v", err)
	}
	return rel
}

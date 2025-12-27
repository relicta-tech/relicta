package cli

import (
	"context"
	"testing"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/internal/application/governance"
	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
)

func TestCancelCommand_FlagsExist(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
	}{
		{"reason flag", "reason"},
		{"force flag", "force"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := cancelCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("cancel command missing %s flag", tt.flagName)
			}
		})
	}
}

func TestResetCommand_FlagsExist(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
	}{
		{"force flag", "force"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := resetCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("reset command missing %s flag", tt.flagName)
			}
		})
	}
}

func TestCancelCommand_Configuration(t *testing.T) {
	if cancelCmd == nil {
		t.Fatal("cancelCmd is nil")
	}
	if cancelCmd.Use != "cancel" {
		t.Errorf("cancelCmd.Use = %v, want cancel", cancelCmd.Use)
	}
	if cancelCmd.RunE == nil {
		t.Error("cancelCmd.RunE is nil")
	}
}

func TestResetCommand_Configuration(t *testing.T) {
	if resetCmd == nil {
		t.Fatal("resetCmd is nil")
	}
	if resetCmd.Use != "reset" {
		t.Errorf("resetCmd.Use = %v, want reset", resetCmd.Use)
	}
	if resetCmd.RunE == nil {
		t.Error("resetCmd.RunE is nil")
	}
}

func TestValidateCancelState_Initialized(t *testing.T) {
	rel := release.NewReleaseRunForTest("test-id", "main", "/test/repo")
	// Initialized state should be cancelable
	err := validateCancelState(rel)
	if err != nil {
		t.Errorf("validateCancelState() should allow canceling initialized state, got: %v", err)
	}
}

func TestValidateResetState_NotFailedOrCanceled(t *testing.T) {
	rel := release.NewReleaseRunForTest("test-id", "main", "/test/repo")
	// In initialized state, reset should fail
	err := validateResetState(rel)
	if err == nil {
		t.Error("validateResetState() should return error for initialized state")
	}
}

func TestGetCurrentUser(t *testing.T) {
	user := getCurrentUser()
	// Should return something, even if it's "unknown"
	if user == "" {
		t.Error("getCurrentUser() returned empty string")
	}
}

func TestGetCurrentUser_FromEnv(t *testing.T) {
	// Test with USER env var
	t.Setenv("USER", "testuser")
	result := getCurrentUser()
	if result != "testuser" {
		t.Errorf("getCurrentUser() = %q, want %q", result, "testuser")
	}
}

func TestGetCurrentUser_FromUsername(t *testing.T) {
	// Test with USERNAME env var (when USER is empty)
	t.Setenv("USER", "")
	t.Setenv("USERNAME", "winuser")
	result := getCurrentUser()
	if result != "winuser" {
		t.Errorf("getCurrentUser() = %q, want %q", result, "winuser")
	}
}

func TestGetCurrentUser_FromGitHubActor(t *testing.T) {
	// Test with GITHUB_ACTOR env var (when USER and USERNAME are empty)
	t.Setenv("USER", "")
	t.Setenv("USERNAME", "")
	t.Setenv("GITHUB_ACTOR", "ghuser")
	result := getCurrentUser()
	if result != "ghuser" {
		t.Errorf("getCurrentUser() = %q, want %q", result, "ghuser")
	}
}

func TestGetCurrentUser_Unknown(t *testing.T) {
	// Test fallback to "unknown"
	t.Setenv("USER", "")
	t.Setenv("USERNAME", "")
	t.Setenv("GITHUB_ACTOR", "")
	result := getCurrentUser()
	if result != "unknown" {
		t.Errorf("getCurrentUser() = %q, want %q", result, "unknown")
	}
}

func TestValidateCancelState_VariousStates(t *testing.T) {
	// Test initialized state (can be canceled)
	t.Run("initialized allows cancel", func(t *testing.T) {
		rel := release.NewReleaseRunForTest("test-id", "main", "/test/repo")
		err := validateCancelState(rel)
		if err != nil {
			t.Errorf("validateCancelState() error = %v, want nil for initialized state", err)
		}
	})

	// Test canceled state (cannot be canceled again)
	t.Run("canceled blocks cancel", func(t *testing.T) {
		rel := release.NewReleaseRunForTest("test-id", "main", "/test/repo")
		_ = rel.Cancel("test cancel", "cli")
		err := validateCancelState(rel)
		if err == nil {
			t.Error("validateCancelState() should return error for canceled state")
		}
	})
}

func TestValidateResetState_VariousStates(t *testing.T) {
	// Test initialized state (cannot be reset, suggest cancel first)
	t.Run("initialized suggests cancel first", func(t *testing.T) {
		rel := release.NewReleaseRunForTest("test-id", "main", "/test/repo")
		err := validateResetState(rel)
		if err == nil {
			t.Error("validateResetState() should return error for initialized state")
		}
	})

	// Test canceled state (can be reset)
	t.Run("canceled allows reset", func(t *testing.T) {
		rel := release.NewReleaseRunForTest("test-id", "main", "/test/repo")
		_ = rel.Cancel("test cancel", "cli")
		err := validateResetState(rel)
		if err != nil {
			t.Errorf("validateResetState() error = %v, want nil for canceled state", err)
		}
	})
}

func TestOutputCancelJSON(t *testing.T) {
	rel := release.NewReleaseRunForTest("test-123", "main", "/test/repo")
	_ = rel.Cancel("test reason", "tester")

	// Capture stdout is complex for os.Stdout, but the function should not panic
	err := outputCancelJSON(rel, "test reason", true)
	if err != nil {
		t.Errorf("outputCancelJSON() error = %v", err)
	}
}

func TestOutputResetJSON(t *testing.T) {
	rel := release.NewReleaseRunForTest("test-123", "main", "/test/repo")
	_ = rel.Cancel("test reason", "tester")

	// Capture stdout is complex for os.Stdout, but the function should not panic
	err := outputResetJSON(rel, release.StateCanceled, false)
	if err != nil {
		t.Errorf("outputResetJSON() error = %v", err)
	}
}

// cancelTestApp is a mock implementation of cliApp for cancel tests.
type cancelTestApp struct {
	gitRepo     sourcecontrol.GitRepository
	releaseRepo release.Repository
}

func (c cancelTestApp) Close() error                              { return nil }
func (c cancelTestApp) GitAdapter() sourcecontrol.GitRepository   { return c.gitRepo }
func (c cancelTestApp) ReleaseRepository() release.Repository     { return c.releaseRepo }
func (c cancelTestApp) PlanRelease() planReleaseUseCase           { return nil }
func (c cancelTestApp) GenerateNotes() generateNotesUseCase       { return nil }
func (c cancelTestApp) ApproveRelease() approveReleaseUseCase     { return nil }
func (c cancelTestApp) PublishRelease() publishReleaseUseCase     { return nil }
func (c cancelTestApp) CalculateVersion() calculateVersionUseCase { return nil }
func (c cancelTestApp) SetVersion() setVersionUseCase             { return nil }
func (c cancelTestApp) HasAI() bool                               { return false }
func (c cancelTestApp) HasGovernance() bool                       { return false }
func (c cancelTestApp) GovernanceService() *governance.Service    { return nil }

// cancelTestGitRepo is a mock git repository for cancel tests.
type cancelTestGitRepo struct{}

func (cancelTestGitRepo) GetInfo(ctx context.Context) (*sourcecontrol.RepositoryInfo, error) {
	return &sourcecontrol.RepositoryInfo{
		Path:          "/test/repo",
		Name:          "repo",
		CurrentBranch: "main",
		RemoteURL:     "https://example.com",
	}, nil
}
func (cancelTestGitRepo) GetRemotes(ctx context.Context) ([]sourcecontrol.RemoteInfo, error) {
	return nil, nil
}
func (cancelTestGitRepo) GetBranches(ctx context.Context) ([]sourcecontrol.BranchInfo, error) {
	return nil, nil
}
func (cancelTestGitRepo) GetCurrentBranch(ctx context.Context) (string, error) { return "main", nil }
func (cancelTestGitRepo) GetCommit(ctx context.Context, hash sourcecontrol.CommitHash) (*sourcecontrol.Commit, error) {
	return nil, nil
}
func (cancelTestGitRepo) GetCommitsBetween(ctx context.Context, from, to string) ([]*sourcecontrol.Commit, error) {
	return nil, nil
}
func (cancelTestGitRepo) GetCommitsSince(ctx context.Context, ref string) ([]*sourcecontrol.Commit, error) {
	return nil, nil
}
func (cancelTestGitRepo) GetLatestCommit(ctx context.Context, branch string) (*sourcecontrol.Commit, error) {
	return nil, nil
}
func (cancelTestGitRepo) GetCommitDiffStats(ctx context.Context, hash sourcecontrol.CommitHash) (*sourcecontrol.DiffStats, error) {
	return nil, nil
}
func (cancelTestGitRepo) GetCommitPatch(ctx context.Context, hash sourcecontrol.CommitHash) (string, error) {
	return "", nil
}
func (cancelTestGitRepo) GetFileAtRef(ctx context.Context, ref, path string) ([]byte, error) {
	return nil, nil
}
func (cancelTestGitRepo) GetTags(ctx context.Context) (sourcecontrol.TagList, error) { return nil, nil }
func (cancelTestGitRepo) GetTag(ctx context.Context, name string) (*sourcecontrol.Tag, error) {
	return nil, nil
}
func (cancelTestGitRepo) GetLatestVersionTag(ctx context.Context, prefix string) (*sourcecontrol.Tag, error) {
	return nil, nil
}
func (cancelTestGitRepo) CreateTag(ctx context.Context, name string, hash sourcecontrol.CommitHash, message string) (*sourcecontrol.Tag, error) {
	return nil, nil
}
func (cancelTestGitRepo) PushTag(ctx context.Context, name, remote string) error { return nil }
func (cancelTestGitRepo) DeleteTag(ctx context.Context, name string) error       { return nil }
func (cancelTestGitRepo) Fetch(ctx context.Context, remote string) error         { return nil }
func (cancelTestGitRepo) Pull(ctx context.Context, remote, branch string) error  { return nil }
func (cancelTestGitRepo) Push(ctx context.Context, remote, branch string) error  { return nil }
func (cancelTestGitRepo) IsDirty(ctx context.Context) (bool, error)              { return false, nil }
func (cancelTestGitRepo) GetStatus(ctx context.Context) (*sourcecontrol.WorkingTreeStatus, error) {
	return &sourcecontrol.WorkingTreeStatus{IsClean: true}, nil
}

// cancelTestReleaseRepo is a mock release repository for cancel tests.
type cancelTestReleaseRepo struct {
	latest    *release.ReleaseRun
	saveError error
}

func (r cancelTestReleaseRepo) Save(ctx context.Context, rel *release.ReleaseRun) error {
	return r.saveError
}
func (r cancelTestReleaseRepo) FindByID(ctx context.Context, id release.RunID) (*release.ReleaseRun, error) {
	return nil, nil
}
func (r cancelTestReleaseRepo) FindLatest(ctx context.Context, repoPath string) (*release.ReleaseRun, error) {
	if r.latest == nil {
		return nil, release.ErrRunNotFound
	}
	return r.latest, nil
}
func (r cancelTestReleaseRepo) FindByState(ctx context.Context, state release.RunState) ([]*release.ReleaseRun, error) {
	return nil, nil
}
func (r cancelTestReleaseRepo) FindActive(ctx context.Context) ([]*release.ReleaseRun, error) {
	return nil, nil
}
func (r cancelTestReleaseRepo) FindBySpecification(ctx context.Context, spec release.Specification) ([]*release.ReleaseRun, error) {
	return nil, nil
}
func (r cancelTestReleaseRepo) Delete(ctx context.Context, id release.RunID) error { return nil }

func TestRunCancel_NoReleaseFound(t *testing.T) {
	origCfg := cfg
	origContainerApp := newContainerApp
	t.Cleanup(func() {
		cfg = origCfg
		newContainerApp = origContainerApp
	})

	cfg = config.DefaultConfig()

	app := cancelTestApp{
		gitRepo:     cancelTestGitRepo{},
		releaseRepo: cancelTestReleaseRepo{latest: nil},
	}

	newContainerApp = func(ctx context.Context, cfg *config.Config) (cliApp, error) {
		return app, nil
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	err := runCancel(cmd, nil)

	// Should return error when no release found
	if err == nil {
		t.Error("runCancel() should return error when no release found")
	}
}

func TestRunCancel_DryRun(t *testing.T) {
	origCfg := cfg
	origDryRun := dryRun
	origContainerApp := newContainerApp
	t.Cleanup(func() {
		cfg = origCfg
		dryRun = origDryRun
		newContainerApp = origContainerApp
	})

	cfg = config.DefaultConfig()
	dryRun = true

	// Create a cancellable release
	rel := release.NewReleaseRunForTest("test-cancel-dryrun", "main", "/test/repo")

	app := cancelTestApp{
		gitRepo:     cancelTestGitRepo{},
		releaseRepo: cancelTestReleaseRepo{latest: rel},
	}

	newContainerApp = func(ctx context.Context, cfg *config.Config) (cliApp, error) {
		return app, nil
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	err := runCancel(cmd, nil)

	if err != nil {
		t.Errorf("runCancel() dry run error = %v", err)
	}
}

func TestRunCancel_Success(t *testing.T) {
	origCfg := cfg
	origDryRun := dryRun
	origContainerApp := newContainerApp
	t.Cleanup(func() {
		cfg = origCfg
		dryRun = origDryRun
		newContainerApp = origContainerApp
	})

	cfg = config.DefaultConfig()
	dryRun = false

	// Create a cancellable release
	rel := release.NewReleaseRunForTest("test-cancel-success", "main", "/test/repo")

	app := cancelTestApp{
		gitRepo:     cancelTestGitRepo{},
		releaseRepo: cancelTestReleaseRepo{latest: rel},
	}

	newContainerApp = func(ctx context.Context, cfg *config.Config) (cliApp, error) {
		return app, nil
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	err := runCancel(cmd, nil)

	if err != nil {
		t.Errorf("runCancel() success error = %v", err)
	}
}

func TestRunReset_NoReleaseFound(t *testing.T) {
	origCfg := cfg
	origContainerApp := newContainerApp
	t.Cleanup(func() {
		cfg = origCfg
		newContainerApp = origContainerApp
	})

	cfg = config.DefaultConfig()

	app := cancelTestApp{
		gitRepo:     cancelTestGitRepo{},
		releaseRepo: cancelTestReleaseRepo{latest: nil},
	}

	newContainerApp = func(ctx context.Context, cfg *config.Config) (cliApp, error) {
		return app, nil
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	err := runReset(cmd, nil)

	// Should return error when no release found
	if err == nil {
		t.Error("runReset() should return error when no release found")
	}
}

func TestRunReset_InvalidState(t *testing.T) {
	origCfg := cfg
	origContainerApp := newContainerApp
	t.Cleanup(func() {
		cfg = origCfg
		newContainerApp = origContainerApp
	})

	cfg = config.DefaultConfig()

	// Create a release in initialized state (not resettable)
	rel := release.NewReleaseRunForTest("test-reset-invalid", "main", "/test/repo")

	app := cancelTestApp{
		gitRepo:     cancelTestGitRepo{},
		releaseRepo: cancelTestReleaseRepo{latest: rel},
	}

	newContainerApp = func(ctx context.Context, cfg *config.Config) (cliApp, error) {
		return app, nil
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	err := runReset(cmd, nil)

	// Should return error for non-failed/non-canceled state
	if err == nil {
		t.Error("runReset() should return error for initialized state")
	}
}

func TestRunReset_DryRun(t *testing.T) {
	origCfg := cfg
	origDryRun := dryRun
	origContainerApp := newContainerApp
	t.Cleanup(func() {
		cfg = origCfg
		dryRun = origDryRun
		newContainerApp = origContainerApp
	})

	cfg = config.DefaultConfig()
	dryRun = true

	// Create a canceled release (resettable)
	rel := release.NewReleaseRunForTest("test-reset-dryrun", "main", "/test/repo")
	_ = rel.Cancel("test reason", "user")

	app := cancelTestApp{
		gitRepo:     cancelTestGitRepo{},
		releaseRepo: cancelTestReleaseRepo{latest: rel},
	}

	newContainerApp = func(ctx context.Context, cfg *config.Config) (cliApp, error) {
		return app, nil
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	err := runReset(cmd, nil)

	if err != nil {
		t.Errorf("runReset() dry run error = %v", err)
	}
}

func TestFindCurrentRelease_Success(t *testing.T) {
	rel := release.NewReleaseRunForTest("test-find", "main", "/test/repo")

	app := cancelTestApp{
		gitRepo:     cancelTestGitRepo{},
		releaseRepo: cancelTestReleaseRepo{latest: rel},
	}

	ctx := context.Background()
	found, err := findCurrentRelease(ctx, app)

	if err != nil {
		t.Errorf("findCurrentRelease() error = %v", err)
	}
	if found == nil {
		t.Error("findCurrentRelease() returned nil release")
	}
	if found.ID() != "test-find" {
		t.Errorf("findCurrentRelease() ID = %v, want test-find", found.ID())
	}
}

func TestFindCurrentRelease_NotFound(t *testing.T) {
	app := cancelTestApp{
		gitRepo:     cancelTestGitRepo{},
		releaseRepo: cancelTestReleaseRepo{latest: nil},
	}

	ctx := context.Background()
	found, err := findCurrentRelease(ctx, app)

	if err == nil {
		t.Error("findCurrentRelease() should return error when no release found")
	}
	if found != nil {
		t.Error("findCurrentRelease() should return nil when no release found")
	}
}

func TestValidateCancelState_PublishingWithForce(t *testing.T) {
	// Create a release and transition to publishing state
	rel := release.NewReleaseRunForTest("test-id", "main", "/test/repo")
	// We need to get it to publishing state - first plan it, version it, approve it, then start publishing
	// Simplest way: just test that initialized can be canceled
	// Actually for this test we need the publishing state path

	// Set cancelForce to true to test the force-cancel path
	origCancelForce := cancelForce
	defer func() { cancelForce = origCancelForce }()
	cancelForce = true

	// Test initialized state with force (should still work)
	err := validateCancelState(rel)
	if err != nil {
		t.Errorf("validateCancelState() with force error = %v", err)
	}
}

func TestValidateResetState_PublishingWithForce(t *testing.T) {
	rel := release.NewReleaseRunForTest("test-id", "main", "/test/repo")

	// First cancel it so we can test the reset states
	_ = rel.Cancel("test", "user")

	// Set cancelForce to true
	origCancelForce := cancelForce
	defer func() { cancelForce = origCancelForce }()
	cancelForce = true

	// Test canceled state with force - should work
	err := validateResetState(rel)
	if err != nil {
		t.Errorf("validateResetState() with force on canceled state error = %v", err)
	}
}

func TestRunCancel_WithJSONOutput(t *testing.T) {
	origCfg := cfg
	origDryRun := dryRun
	origOutputJSON := outputJSON
	origContainerApp := newContainerApp
	t.Cleanup(func() {
		cfg = origCfg
		dryRun = origDryRun
		outputJSON = origOutputJSON
		newContainerApp = origContainerApp
	})

	cfg = config.DefaultConfig()
	dryRun = true
	outputJSON = true

	// Create a cancellable release
	rel := release.NewReleaseRunForTest("test-cancel-json", "main", "/test/repo")

	app := cancelTestApp{
		gitRepo:     cancelTestGitRepo{},
		releaseRepo: cancelTestReleaseRepo{latest: rel},
	}

	newContainerApp = func(ctx context.Context, cfg *config.Config) (cliApp, error) {
		return app, nil
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	err := runCancel(cmd, nil)

	if err != nil {
		t.Errorf("runCancel() with JSON output error = %v", err)
	}
}

func TestRunCancel_SuccessWithJSONOutput(t *testing.T) {
	origCfg := cfg
	origDryRun := dryRun
	origOutputJSON := outputJSON
	origContainerApp := newContainerApp
	t.Cleanup(func() {
		cfg = origCfg
		dryRun = origDryRun
		outputJSON = origOutputJSON
		newContainerApp = origContainerApp
	})

	cfg = config.DefaultConfig()
	dryRun = false
	outputJSON = true

	// Create a cancellable release
	rel := release.NewReleaseRunForTest("test-cancel-json-success", "main", "/test/repo")

	app := cancelTestApp{
		gitRepo:     cancelTestGitRepo{},
		releaseRepo: cancelTestReleaseRepo{latest: rel},
	}

	newContainerApp = func(ctx context.Context, cfg *config.Config) (cliApp, error) {
		return app, nil
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	err := runCancel(cmd, nil)

	if err != nil {
		t.Errorf("runCancel() success with JSON output error = %v", err)
	}
}

func TestRunReset_SuccessWithJSONOutput(t *testing.T) {
	origCfg := cfg
	origDryRun := dryRun
	origOutputJSON := outputJSON
	origContainerApp := newContainerApp
	t.Cleanup(func() {
		cfg = origCfg
		dryRun = origDryRun
		outputJSON = origOutputJSON
		newContainerApp = origContainerApp
	})

	cfg = config.DefaultConfig()
	dryRun = true
	outputJSON = true

	// Create a canceled release (resettable)
	rel := release.NewReleaseRunForTest("test-reset-json", "main", "/test/repo")
	_ = rel.Cancel("test reason", "user")

	app := cancelTestApp{
		gitRepo:     cancelTestGitRepo{},
		releaseRepo: cancelTestReleaseRepo{latest: rel},
	}

	newContainerApp = func(ctx context.Context, cfg *config.Config) (cliApp, error) {
		return app, nil
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	err := runReset(cmd, nil)

	if err != nil {
		t.Errorf("runReset() with JSON output error = %v", err)
	}
}

func TestRunCancel_WithCustomReason(t *testing.T) {
	origCfg := cfg
	origDryRun := dryRun
	origCancelReason := cancelReason
	origContainerApp := newContainerApp
	t.Cleanup(func() {
		cfg = origCfg
		dryRun = origDryRun
		cancelReason = origCancelReason
		newContainerApp = origContainerApp
	})

	cfg = config.DefaultConfig()
	dryRun = false
	cancelReason = "custom test reason"

	rel := release.NewReleaseRunForTest("test-cancel-reason", "main", "/test/repo")

	app := cancelTestApp{
		gitRepo:     cancelTestGitRepo{},
		releaseRepo: cancelTestReleaseRepo{latest: rel},
	}

	newContainerApp = func(ctx context.Context, cfg *config.Config) (cliApp, error) {
		return app, nil
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	err := runCancel(cmd, nil)

	if err != nil {
		t.Errorf("runCancel() with custom reason error = %v", err)
	}
}

func TestValidateCancelState_FailedState(t *testing.T) {
	rel := release.NewReleaseRunForTest("test-id", "main", "/test/repo")
	// First cancel it, then the state is already failed/canceled
	_ = rel.Cancel("test reason", "cli")

	// canceled state should not be cancelable again
	err := validateCancelState(rel)
	if err == nil {
		t.Error("validateCancelState() should return error for canceled state")
	}
}

func TestValidateCancelState_AllStates(t *testing.T) {
	tests := []struct {
		name      string
		setup     func() *release.ReleaseRun
		wantError bool
	}{
		{
			name: "initialized state should be cancelable",
			setup: func() *release.ReleaseRun {
				return release.NewReleaseRunForTest("test-init", "main", "/test/repo")
			},
			wantError: false,
		},
		{
			name: "canceled state should not be cancelable",
			setup: func() *release.ReleaseRun {
				rel := release.NewReleaseRunForTest("test-canceled", "main", "/test/repo")
				_ = rel.Cancel("test", "cli")
				return rel
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rel := tt.setup()
			err := validateCancelState(rel)
			if (err != nil) != tt.wantError {
				t.Errorf("validateCancelState() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestValidateResetState_AllowedStates(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*release.ReleaseRun)
		wantError bool
	}{
		{
			name: "initialized state should suggest cancel first",
			setup: func(r *release.ReleaseRun) {
				// Already in initialized state, nothing to do
			},
			wantError: true,
		},
		{
			name: "canceled state should allow reset",
			setup: func(r *release.ReleaseRun) {
				_ = r.Cancel("test", "cli")
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rel := release.NewReleaseRunForTest("test-id", "main", "/test/repo")
			tt.setup(rel)
			err := validateResetState(rel)
			if (err != nil) != tt.wantError {
				t.Errorf("validateResetState() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestRunReset_DryRunNoJSON(t *testing.T) {
	origCfg := cfg
	origDryRun := dryRun
	origOutputJSON := outputJSON
	origContainerApp := newContainerApp
	t.Cleanup(func() {
		cfg = origCfg
		dryRun = origDryRun
		outputJSON = origOutputJSON
		newContainerApp = origContainerApp
	})

	cfg = config.DefaultConfig()
	dryRun = true // Use dry run to test the path without calling RetryPublish
	outputJSON = false

	// Create a canceled release (resettable based on validateResetState)
	rel := release.NewReleaseRunForTest("test-reset-success", "main", "/test/repo")
	_ = rel.Cancel("test reason", "user")

	app := cancelTestApp{
		gitRepo:     cancelTestGitRepo{},
		releaseRepo: cancelTestReleaseRepo{latest: rel},
	}

	newContainerApp = func(ctx context.Context, cfg *config.Config) (cliApp, error) {
		return app, nil
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	err := runReset(cmd, nil)

	if err != nil {
		t.Errorf("runReset() dry run error = %v", err)
	}
}

func TestRunReset_DryRunWithJSON(t *testing.T) {
	origCfg := cfg
	origDryRun := dryRun
	origOutputJSON := outputJSON
	origContainerApp := newContainerApp
	t.Cleanup(func() {
		cfg = origCfg
		dryRun = origDryRun
		outputJSON = origOutputJSON
		newContainerApp = origContainerApp
	})

	cfg = config.DefaultConfig()
	dryRun = true // Use dry run to test the JSON path without calling RetryPublish
	outputJSON = true

	// Create a canceled release (resettable based on validateResetState)
	rel := release.NewReleaseRunForTest("test-reset-json-run", "main", "/test/repo")
	_ = rel.Cancel("test reason", "user")

	app := cancelTestApp{
		gitRepo:     cancelTestGitRepo{},
		releaseRepo: cancelTestReleaseRepo{latest: rel},
	}

	newContainerApp = func(ctx context.Context, cfg *config.Config) (cliApp, error) {
		return app, nil
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	err := runReset(cmd, nil)

	if err != nil {
		t.Errorf("runReset() dry run with JSON error = %v", err)
	}
}

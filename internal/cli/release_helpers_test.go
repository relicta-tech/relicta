package cli

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/application/governance"
	apprelease "github.com/relicta-tech/relicta/internal/application/release"
	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

type fakeGenerateNotesUseCase struct {
	executeCalled bool
	input         apprelease.GenerateNotesInput
}

func (f *fakeGenerateNotesUseCase) Execute(ctx context.Context, input apprelease.GenerateNotesInput) (*apprelease.GenerateNotesOutput, error) {
	f.executeCalled = true
	f.input = input
	return &apprelease.GenerateNotesOutput{}, nil
}

type releaseTestApp struct {
	plan        planReleaseUseCase
	generate    generateNotesUseCase
	approve     approveReleaseUseCase
	publish     publishReleaseUseCase
	setVersion  setVersionUseCase
	gitRepo     sourcecontrol.GitRepository
	releaseRepo release.Repository
}

func (r releaseTestApp) Close() error                              { return nil }
func (r releaseTestApp) GitAdapter() sourcecontrol.GitRepository   { return r.gitRepo }
func (r releaseTestApp) ReleaseRepository() release.Repository     { return r.releaseRepo }
func (r releaseTestApp) PlanRelease() planReleaseUseCase           { return r.plan }
func (r releaseTestApp) GenerateNotes() generateNotesUseCase       { return r.generate }
func (r releaseTestApp) ApproveRelease() approveReleaseUseCase     { return r.approve }
func (r releaseTestApp) PublishRelease() publishReleaseUseCase     { return r.publish }
func (r releaseTestApp) CalculateVersion() calculateVersionUseCase { return nil }
func (r releaseTestApp) SetVersion() setVersionUseCase             { return r.setVersion }
func (r releaseTestApp) HasAI() bool                               { return false }
func (r releaseTestApp) HasGovernance() bool                       { return false }
func (r releaseTestApp) GovernanceService() *governance.Service    { return nil }

func TestRunReleasePlanExecutesUseCase(t *testing.T) {
	origCfg := cfg
	defer func() { cfg = origCfg }()
	cfg = config.DefaultConfig()

	planned := newPlanOutput()

	app := releaseTestApp{
		gitRepo: stubGitRepo{},
		plan:    &fakePlanUseCase{executeOutput: planned},
	}

	// Create a workflow context for the test (normal release mode)
	wfCtx := &releaseWorkflowContext{
		mode: releaseModeNew,
	}

	out, err := runReleasePlan(context.Background(), app, wfCtx)
	if err != nil {
		t.Fatalf("runReleasePlan error: %v", err)
	}
	if out != planned {
		t.Fatalf("unexpected output: %#v", out)
	}
}

func TestRunReleaseBumpUsesForcedVersion(t *testing.T) {
	origForce := releaseForce
	origCfg := cfg
	defer func() {
		releaseForce = origForce
		cfg = origCfg
	}()

	releaseForce = "2.3.4"
	cfg = config.DefaultConfig()
	cfg.Versioning.GitTag = true
	cfg.Versioning.GitPush = true

	setVersion := &fakeSetVersionUseCase{}
	plan := &apprelease.PlanReleaseOutput{ReleaseID: "release", NextVersion: version.MustParse("1.0.0")}
	app := releaseTestApp{
		gitRepo:    stubGitRepo{},
		setVersion: setVersion,
		releaseRepo: testReleaseRepo{
			latest: newPlannedRelease(t, "bump-release"),
		},
	}

	// Create a workflow context for the test (normal release mode)
	wfCtx := &releaseWorkflowContext{
		mode: releaseModeNew,
	}

	out, err := runReleaseBump(context.Background(), app, wfCtx, plan)
	if err != nil {
		t.Fatalf("runReleaseBump error: %v", err)
	}
	if !setVersion.executeCalled {
		t.Fatal("expected set version execute")
	}
	if out.Version.String() != "2.3.4" {
		t.Fatalf("unexpected version %s", out.Version.String())
	}
}

func TestRunReleaseNotesCallsGeneration(t *testing.T) {
	origCfg := cfg
	defer func() { cfg = origCfg }()
	cfg = config.DefaultConfig()

	fake := &fakeGenerateNotesUseCase{}
	app := releaseTestApp{
		generate: fake,
	}

	plan := &apprelease.PlanReleaseOutput{ReleaseID: "rls"}
	_, err := runReleaseNotes(context.Background(), app, plan)
	if err != nil {
		t.Fatalf("runReleaseNotes error: %v", err)
	}
	if !fake.executeCalled {
		t.Fatal("expected generate notes to run")
	}
	if fake.input.ReleaseID != "rls" {
		t.Fatalf("unexpected plan ID %s", fake.input.ReleaseID)
	}
}

func TestRunReleaseApproveAutoApprove(t *testing.T) {
	fake := &fakeApproveReleaseUseCase{}
	app := releaseTestApp{
		approve: fake,
	}

	plan := &apprelease.PlanReleaseOutput{ReleaseID: "approve-rls"}
	notes := &apprelease.GenerateNotesOutput{}

	approved, err := runReleaseApprove(context.Background(), app, plan, notes, true)
	if err != nil {
		t.Fatalf("runReleaseApprove error: %v", err)
	}
	if !approved {
		t.Fatal("expected release to be approved")
	}
	if !fake.executeCalled {
		t.Fatal("expected approve use case to execute")
	}
}

func TestRunReleaseApproveRequiresTerminal(t *testing.T) {
	oldStdout := os.Stdout
	tmpFile, err := os.CreateTemp("", "stdout")
	if err != nil {
		t.Fatalf("CreateTemp error: %v", err)
	}
	t.Cleanup(func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		os.Stdout = oldStdout
	})

	os.Stdout = tmpFile

	app := releaseTestApp{}
	plan := &apprelease.PlanReleaseOutput{ReleaseID: "approve-term"}
	notes := &apprelease.GenerateNotesOutput{}

	approved, err := runReleaseApprove(context.Background(), app, plan, notes, false)
	if err == nil {
		t.Fatal("expected error when not running in terminal")
	}
	if approved {
		t.Fatal("expected approval to be false")
	}
}

func TestRunReleaseApproveAuto(t *testing.T) {
	app := releaseTestApp{
		approve: &fakeApproveReleaseUseCase{},
	}
	plan := &apprelease.PlanReleaseOutput{ReleaseID: "auto"}
	ok, err := runReleaseApprove(context.Background(), app, plan, &apprelease.GenerateNotesOutput{}, true)
	if err != nil {
		t.Fatalf("runReleaseApprove error: %v", err)
	}
	if !ok {
		t.Fatal("expected approval success")
	}
}

func TestRunReleasePublishInvokesUseCase(t *testing.T) {
	cfg = config.DefaultConfig()
	cfg.Versioning.GitTag = true
	cfg.Versioning.GitPush = true

	fakePub := &fakePublishReleaseUseCase{}
	app := releaseTestApp{
		publish: fakePub,
	}
	plan := &apprelease.PlanReleaseOutput{ReleaseID: "publish-release"}

	if _, err := runReleasePublish(context.Background(), app, plan); err != nil {
		t.Fatalf("runReleasePublish error: %v", err)
	}
	if !fakePub.executeCalled {
		t.Fatal("expected publish execute called")
	}
}

// tagAwareGitRepo is a stub that can return specific tags and commits for testing.
type tagAwareGitRepo struct {
	stubGitRepo
	tags       sourcecontrol.TagList
	headCommit *sourcecontrol.Commit
	tagsErr    error
	commitErr  error
}

func (r tagAwareGitRepo) GetTags(ctx context.Context) (sourcecontrol.TagList, error) {
	if r.tagsErr != nil {
		return nil, r.tagsErr
	}
	return r.tags, nil
}

func (r tagAwareGitRepo) GetLatestCommit(ctx context.Context, branch string) (*sourcecontrol.Commit, error) {
	if r.commitErr != nil {
		return nil, r.commitErr
	}
	return r.headCommit, nil
}

func newTestCommit(hash sourcecontrol.CommitHash) *sourcecontrol.Commit {
	return sourcecontrol.NewCommit(hash, "Test commit", sourcecontrol.Author{}, time.Now())
}

func TestDetectReleaseModeNewRelease(t *testing.T) {
	origCfg := cfg
	defer func() { cfg = origCfg }()
	cfg = config.DefaultConfig()

	headHash := sourcecontrol.CommitHash("abc123")
	app := releaseTestApp{
		gitRepo: tagAwareGitRepo{
			headCommit: newTestCommit(headHash),
			tags: sourcecontrol.TagList{
				sourcecontrol.NewTag("v1.0.0", sourcecontrol.CommitHash("other")),
				sourcecontrol.NewTag("v1.1.0", sourcecontrol.CommitHash("different")),
			},
		},
	}

	mode, ver, err := detectReleaseMode(context.Background(), app, "v")
	if err != nil {
		t.Fatalf("detectReleaseMode error: %v", err)
	}
	if mode != releaseModeNew {
		t.Fatalf("expected releaseModeNew, got %v", mode)
	}
	if ver != nil {
		t.Fatalf("expected nil version, got %v", ver)
	}
}

func TestDetectReleaseModeTagPush(t *testing.T) {
	origCfg := cfg
	defer func() { cfg = origCfg }()
	cfg = config.DefaultConfig()

	headHash := sourcecontrol.CommitHash("abc123")
	app := releaseTestApp{
		gitRepo: tagAwareGitRepo{
			headCommit: newTestCommit(headHash),
			tags: sourcecontrol.TagList{
				sourcecontrol.NewTag("v1.0.0", sourcecontrol.CommitHash("other")),
				sourcecontrol.NewTag("v2.0.0", headHash), // This tag points to HEAD
			},
		},
	}

	mode, ver, err := detectReleaseMode(context.Background(), app, "v")
	if err != nil {
		t.Fatalf("detectReleaseMode error: %v", err)
	}
	if mode != releaseModeTagPush {
		t.Fatalf("expected releaseModeTagPush, got %v", mode)
	}
	if ver == nil {
		t.Fatal("expected version, got nil")
	}
	if ver.String() != "2.0.0" {
		t.Fatalf("expected version 2.0.0, got %s", ver.String())
	}
}

func TestDetectReleaseModeHandlesGetTagsError(t *testing.T) {
	app := releaseTestApp{
		gitRepo: tagAwareGitRepo{
			headCommit: newTestCommit("abc"),
			tagsErr:    os.ErrNotExist,
		},
	}

	mode, ver, err := detectReleaseMode(context.Background(), app, "v")
	if err != nil {
		t.Fatalf("detectReleaseMode error: %v", err)
	}
	// Should fall back to new release mode on error
	if mode != releaseModeNew {
		t.Fatalf("expected releaseModeNew on error, got %v", mode)
	}
	if ver != nil {
		t.Fatalf("expected nil version on error, got %v", ver)
	}
}

func TestDetectReleaseModeHandlesGetCommitError(t *testing.T) {
	app := releaseTestApp{
		gitRepo: tagAwareGitRepo{
			commitErr: os.ErrNotExist,
		},
	}

	mode, ver, err := detectReleaseMode(context.Background(), app, "v")
	if err != nil {
		t.Fatalf("detectReleaseMode error: %v", err)
	}
	if mode != releaseModeNew {
		t.Fatalf("expected releaseModeNew on error, got %v", mode)
	}
	if ver != nil {
		t.Fatalf("expected nil version on error, got %v", ver)
	}
}

func TestFindPreviousVersionTag(t *testing.T) {
	origCfg := cfg
	defer func() { cfg = origCfg }()
	cfg = config.DefaultConfig()

	currentVer := version.MustParse("2.0.0")
	app := releaseTestApp{
		gitRepo: tagAwareGitRepo{
			tags: sourcecontrol.TagList{
				sourcecontrol.NewTag("v1.0.0", sourcecontrol.CommitHash("a")),
				sourcecontrol.NewTag("v1.5.0", sourcecontrol.CommitHash("b")),
				sourcecontrol.NewTag("v1.9.0", sourcecontrol.CommitHash("c")),
				sourcecontrol.NewTag("v2.0.0", sourcecontrol.CommitHash("d")),
				sourcecontrol.NewTag("v2.1.0", sourcecontrol.CommitHash("e")),
			},
		},
	}

	prevTag, err := findPreviousVersionTag(context.Background(), app, &currentVer)
	if err != nil {
		t.Fatalf("findPreviousVersionTag error: %v", err)
	}
	if prevTag != "v1.9.0" {
		t.Fatalf("expected v1.9.0, got %s", prevTag)
	}
}

func TestFindPreviousVersionTagNoPrevious(t *testing.T) {
	origCfg := cfg
	defer func() { cfg = origCfg }()
	cfg = config.DefaultConfig()

	currentVer := version.MustParse("1.0.0")
	app := releaseTestApp{
		gitRepo: tagAwareGitRepo{
			tags: sourcecontrol.TagList{
				sourcecontrol.NewTag("v1.0.0", sourcecontrol.CommitHash("a")),
				sourcecontrol.NewTag("v2.0.0", sourcecontrol.CommitHash("b")),
			},
		},
	}

	prevTag, err := findPreviousVersionTag(context.Background(), app, &currentVer)
	if err != nil {
		t.Fatalf("findPreviousVersionTag error: %v", err)
	}
	if prevTag != "" {
		t.Fatalf("expected empty string, got %s", prevTag)
	}
}

func TestFindPreviousVersionTagHandlesError(t *testing.T) {
	origCfg := cfg
	defer func() { cfg = origCfg }()
	cfg = config.DefaultConfig()

	currentVer := version.MustParse("2.0.0")
	app := releaseTestApp{
		gitRepo: tagAwareGitRepo{
			tagsErr: os.ErrNotExist,
		},
	}

	_, err := findPreviousVersionTag(context.Background(), app, &currentVer)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDetectWorkflowContextNewRelease(t *testing.T) {
	origCfg := cfg
	defer func() { cfg = origCfg }()
	cfg = config.DefaultConfig()

	headHash := sourcecontrol.CommitHash("abc123")
	app := releaseTestApp{
		gitRepo: tagAwareGitRepo{
			headCommit: newTestCommit(headHash),
			tags: sourcecontrol.TagList{
				sourcecontrol.NewTag("v1.0.0", sourcecontrol.CommitHash("other")),
			},
		},
	}

	wfCtx, err := detectWorkflowContext(context.Background(), app)
	if err != nil {
		t.Fatalf("detectWorkflowContext error: %v", err)
	}
	if wfCtx.mode != releaseModeNew {
		t.Fatalf("expected releaseModeNew, got %v", wfCtx.mode)
	}
	if wfCtx.existingVersion != nil {
		t.Fatalf("expected nil existingVersion, got %v", wfCtx.existingVersion)
	}
	if wfCtx.prevTagName != "" {
		t.Fatalf("expected empty prevTagName, got %s", wfCtx.prevTagName)
	}
}

func TestDetectWorkflowContextTagPush(t *testing.T) {
	origCfg := cfg
	defer func() { cfg = origCfg }()
	cfg = config.DefaultConfig()

	headHash := sourcecontrol.CommitHash("abc123")
	app := releaseTestApp{
		gitRepo: tagAwareGitRepo{
			headCommit: newTestCommit(headHash),
			tags: sourcecontrol.TagList{
				sourcecontrol.NewTag("v1.0.0", sourcecontrol.CommitHash("prev")),
				sourcecontrol.NewTag("v2.0.0", headHash), // HEAD is tagged
			},
		},
	}

	wfCtx, err := detectWorkflowContext(context.Background(), app)
	if err != nil {
		t.Fatalf("detectWorkflowContext error: %v", err)
	}
	if wfCtx.mode != releaseModeTagPush {
		t.Fatalf("expected releaseModeTagPush, got %v", wfCtx.mode)
	}
	if wfCtx.existingVersion == nil {
		t.Fatal("expected existingVersion, got nil")
	}
	if wfCtx.existingVersion.String() != "2.0.0" {
		t.Fatalf("expected version 2.0.0, got %s", wfCtx.existingVersion.String())
	}
	if wfCtx.prevTagName != "v1.0.0" {
		t.Fatalf("expected prevTagName v1.0.0, got %s", wfCtx.prevTagName)
	}
}

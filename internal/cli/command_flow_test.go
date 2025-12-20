package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/internal/analysis"
	"github.com/relicta-tech/relicta/internal/application/governance"
	apprelease "github.com/relicta-tech/relicta/internal/application/release"
	"github.com/relicta-tech/relicta/internal/application/versioning"
	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

func runPlanExecutesPlan(t *testing.T, useAnalyze bool) {
	t.Helper()
	origCfg := cfg
	t.Cleanup(func() { cfg = origCfg })

	cfg = config.DefaultConfig()
	outputJSON = true
	t.Cleanup(func() { outputJSON = false })
	planAnalyze = useAnalyze
	planReview = false
	t.Cleanup(func() {
		planAnalyze = false
		planReview = false
	})

	fakePlan := &fakePlanUseCase{
		analysisResult: &analysis.AnalysisResult{},
		commitInfos: []analysis.CommitInfo{
			{Hash: sourcecontrol.CommitHash("abc"), Subject: "feat"},
		},
		executeOutput: &apprelease.PlanReleaseOutput{
			ReleaseID:      release.ReleaseID("release-1"),
			CurrentVersion: version.MustParse("0.1.0"),
			NextVersion:    version.MustParse("0.2.0"),
			ReleaseType:    changes.ReleaseTypeMinor,
			ChangeSet:      newTestChangeSet(),
			RepositoryName: "repo",
			Branch:         "main",
		},
	}

	app := commandTestApp{
		gitRepo: stubGitRepo{},
		plan:    fakePlan,
	}

	withStubContainerApp(t, app)

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	if err := runPlan(cmd, nil); err != nil {
		t.Fatalf("runPlan error: %v", err)
	}

	if useAnalyze {
		if !fakePlan.analyzeCalled {
			t.Fatal("expected analyze path to be executed")
		}
		return
	}
	if !fakePlan.executeCalled {
		t.Fatal("expected plan use case to execute")
	}
}

func TestRunPlanExecutesPlanDefault(t *testing.T) {
	runPlanExecutesPlan(t, false)
}

func TestRunPlanAnalyzeMode(t *testing.T) {
	runPlanExecutesPlan(t, true)
}

func TestRunVersionExecutesLicensing(t *testing.T) {
	t.Cleanup(func() {
		bumpLevel = ""
		bumpForce = ""
		outputJSON = false
		dryRun = false
	})
	origCreateTag := bumpCreateTag
	t.Cleanup(func() { bumpCreateTag = origCreateTag })
	bumpCreateTag = true

	cfg = config.DefaultConfig()
	cfg.Versioning.GitTag = true
	outputJSON = true
	t.Cleanup(func() {
		outputJSON = false
		dryRun = false
	})

	fakeCalc := &fakeCalculateVersionUseCase{}
	fakeSet := &fakeSetVersionUseCase{}
	release := newTestRelease(t, "version-1")

	app := commandTestApp{
		gitRepo:     stubGitRepo{},
		calculate:   fakeCalc,
		setVersion:  fakeSet,
		releaseRepo: testReleaseRepo{latest: release},
	}

	withStubContainerApp(t, app)

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	if err := runVersion(cmd, nil); err != nil {
		t.Fatalf("runVersion error: %v", err)
	}

	if !fakeCalc.executeCalled {
		t.Fatal("expected calculate version to be executed")
	}
	if !fakeSet.executeCalled {
		t.Fatal("expected set version to be executed via applyVersionTag")
	}
}

func TestRunVersionDryRunJSON(t *testing.T) {
	origCfg := cfg
	origOutputJSON := outputJSON
	origDryRun := dryRun
	origNewContainerApp := newContainerApp
	defer func() {
		cfg = origCfg
		outputJSON = origOutputJSON
		dryRun = origDryRun
		newContainerApp = origNewContainerApp
	}()

	cfg = &config.Config{Versioning: config.VersioningConfig{TagPrefix: "v", GitTag: true, GitPush: true}}
	dryRun = true
	outputJSON = true

	fakeCalc := &fakeCalculateVersionUseCase{
		output: &versioning.CalculateVersionOutput{
			CurrentVersion: version.MustParse("0.1.0"),
			NextVersion:    version.MustParse("0.2.0"),
			BumpType:       version.BumpMinor,
			AutoDetected:   true,
		},
	}

	app := commandTestApp{
		gitRepo:    stubGitRepo{},
		plan:       nil,
		calculate:  fakeCalc,
		setVersion: &fakeSetVersionUseCase{},
		releaseRepo: testReleaseRepo{
			latest: newTestRelease(t, "version-dry-run"),
		},
	}

	withStubContainerApp(t, app)

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	out := captureCLIStdout(func() {
		if err := runVersion(cmd, nil); err != nil {
			t.Fatalf("runVersion error: %v", err)
		}
	})

	if !strings.Contains(out, "\"current_version\"") {
		t.Fatalf("expected JSON output, got: %s", out)
	}
	if !fakeCalc.executeCalled {
		t.Fatal("expected calculate version to be invoked")
	}
}

func TestRunVersionAppliesTag(t *testing.T) {
	origCfg := cfg
	origBumpCreate := bumpCreateTag
	origBumpPush := bumpPush
	origOutputJSON := outputJSON
	origNewContainerApp := newContainerApp
	defer func() {
		cfg = origCfg
		bumpCreateTag = origBumpCreate
		bumpPush = origBumpPush
		outputJSON = origOutputJSON
		newContainerApp = origNewContainerApp
	}()

	cfg = &config.Config{Versioning: config.VersioningConfig{TagPrefix: "v", GitTag: true, GitPush: true}}
	bumpCreateTag = true
	bumpPush = true
	outputJSON = false

	fakeCalc := &fakeCalculateVersionUseCase{
		output: &versioning.CalculateVersionOutput{
			CurrentVersion: version.MustParse("0.1.0"),
			NextVersion:    version.MustParse("0.2.0"),
			BumpType:       version.BumpMinor,
			AutoDetected:   false,
		},
	}
	setVersion := &fakeSetVersionUseCase{}

	app := commandTestApp{
		calculate:  fakeCalc,
		setVersion: setVersion,
		gitRepo:    stubGitRepo{},
		releaseRepo: testReleaseRepo{
			latest: newTestRelease(t, "version-tag"),
		},
	}

	withStubContainerApp(t, app)

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	if err := runVersion(cmd, nil); err != nil {
		t.Fatalf("runVersion error: %v", err)
	}
	if !setVersion.executeCalled {
		t.Fatal("expected SetVersion to be called for tagging")
	}
	if !setVersion.input.PushTag {
		t.Fatalf("expected push tag true, got %+v", setVersion.input)
	}
	if !setVersion.input.CreateTag {
		t.Fatalf("expected create tag true, got %+v", setVersion.input)
	}
}

func TestRunApproveExecutesApproval(t *testing.T) {
	t.Cleanup(func() {
		approveYes = false
	})

	approveYes = true
	cfg = config.DefaultConfig()

	release := newTestRelease(t, "approve-1")

	app := commandTestApp{
		gitRepo:     stubGitRepo{},
		releaseRepo: testReleaseRepo{latest: release},
		approve:     &fakeApproveReleaseUseCase{},
	}

	withStubContainerApp(t, app)

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	if err := runApprove(cmd, nil); err != nil {
		t.Fatalf("runApprove error: %v", err)
	}

	if !app.approve.(*fakeApproveReleaseUseCase).executeCalled {
		t.Fatal("expected approve use case to be executed")
	}
}

func TestRunPublishExecutesPublishUseCase(t *testing.T) {
	cfg = config.DefaultConfig()
	cfg.Changelog.File = ""

	release := newTestRelease(t, "publish-1")
	_ = release.Approve("tester", false)

	app := commandTestApp{
		gitRepo:     stubGitRepo{},
		releaseRepo: testReleaseRepo{latest: release},
		publish:     &fakePublishReleaseUseCase{},
	}

	withStubContainerApp(t, app)

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	if err := runPublish(cmd, nil); err != nil {
		t.Fatalf("runPublish error: %v", err)
	}

	if !app.publish.(*fakePublishReleaseUseCase).executeCalled {
		t.Fatal("expected publish use case to execute")
	}
}

func TestRunApproveOutputsJSON(t *testing.T) {
	origCfg := cfg
	origOutputJSON := outputJSON
	origNewContainerApp := newContainerApp
	defer func() {
		cfg = origCfg
		outputJSON = origOutputJSON
		newContainerApp = origNewContainerApp
	}()

	cfg = config.DefaultConfig()
	outputJSON = true

	rel := newTestRelease(t, "approve-json")

	app := commandTestApp{
		gitRepo:     stubGitRepo{},
		releaseRepo: testReleaseRepo{latest: rel},
	}
	withStubContainerApp(t, app)

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	out := captureCLIStdout(func() {
		if err := runApprove(cmd, nil); err != nil {
			t.Fatalf("runApprove error: %v", err)
		}
	})

	if !strings.Contains(out, "\"release_id\": \"approve-json\"") {
		t.Fatalf("expected release_id in JSON output, got: %s", out)
	}
	if !strings.Contains(out, "\"approved\": false") {
		t.Fatalf("expected approved=false output, got: %s", out)
	}
}

func TestRunPublishOutputsJSON(t *testing.T) {
	origCfg := cfg
	origOutputJSON := outputJSON
	origDryRun := dryRun
	origNewContainerApp := newContainerApp
	defer func() {
		cfg = origCfg
		outputJSON = origOutputJSON
		dryRun = origDryRun
		newContainerApp = origNewContainerApp
	}()

	cfg = config.DefaultConfig()
	outputJSON = true
	dryRun = false

	rel := newTestRelease(t, "publish-json")
	_ = rel.Approve("tester", false)

	fakePub := &fakePublishReleaseUseCase{}
	app := commandTestApp{
		gitRepo:     stubGitRepo{},
		releaseRepo: testReleaseRepo{latest: rel},
		publish:     fakePub,
	}
	withStubContainerApp(t, app)

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	dryRun = false
	out := captureCLIStdout(func() {
		if err := runPublish(cmd, nil); err != nil {
			t.Fatalf("runPublish error: %v", err)
		}
	})

	if !strings.Contains(out, "\"release_id\": \"publish-json\"") {
		t.Fatalf("expected release_id in JSON output, got: %s", out)
	}
}

func TestRunPublishDryRunSkipsExecution(t *testing.T) {
	origCfg := cfg
	origDryRun := dryRun
	origNewContainerApp := newContainerApp
	defer func() {
		cfg = origCfg
		dryRun = origDryRun
		newContainerApp = origNewContainerApp
	}()

	cfg = config.DefaultConfig()
	dryRun = true

	rel := newTestRelease(t, "publish-dryrun")
	_ = rel.Approve("tester", false)

	fakePub := &fakePublishReleaseUseCase{}
	app := commandTestApp{
		gitRepo:     stubGitRepo{},
		releaseRepo: testReleaseRepo{latest: rel},
		publish:     fakePub,
	}
	withStubContainerApp(t, app)

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	if err := runPublish(cmd, nil); err != nil {
		t.Fatalf("runPublish error: %v", err)
	}

	if fakePub.executeCalled {
		t.Fatal("expected publish use case to be skipped in dry run")
	}
}

type fakeCalculateVersionUseCase struct {
	output        *versioning.CalculateVersionOutput
	executeCalled bool
}

func (f *fakeCalculateVersionUseCase) Execute(ctx context.Context, input versioning.CalculateVersionInput) (*versioning.CalculateVersionOutput, error) {
	f.executeCalled = true
	if f.output != nil {
		return f.output, nil
	}
	return &versioning.CalculateVersionOutput{
		CurrentVersion: version.Initial,
		NextVersion:    version.MustParse("0.1.0"),
		BumpType:       version.BumpMinor,
		AutoDetected:   true,
	}, nil
}

type fakeApproveReleaseUseCase struct {
	executeCalled bool
}

func (f *fakeApproveReleaseUseCase) Execute(ctx context.Context, input apprelease.ApproveReleaseInput) (*apprelease.ApproveReleaseOutput, error) {
	f.executeCalled = true
	return &apprelease.ApproveReleaseOutput{}, nil
}

type fakePublishReleaseUseCase struct {
	executeCalled bool
}

func (f *fakePublishReleaseUseCase) Execute(ctx context.Context, input apprelease.PublishReleaseInput) (*apprelease.PublishReleaseOutput, error) {
	f.executeCalled = true
	return &apprelease.PublishReleaseOutput{
		TagName: "v0.1.0",
	}, nil
}

type commandTestApp struct {
	plan        planReleaseUseCase
	approve     approveReleaseUseCase
	publish     publishReleaseUseCase
	calculate   calculateVersionUseCase
	setVersion  setVersionUseCase
	gitRepo     sourcecontrol.GitRepository
	releaseRepo release.Repository
	hasGov      bool
	govSvc      *governance.Service
}

func (c commandTestApp) Close() error                              { return nil }
func (c commandTestApp) GitAdapter() sourcecontrol.GitRepository   { return c.gitRepo }
func (c commandTestApp) ReleaseRepository() release.Repository     { return c.releaseRepo }
func (c commandTestApp) PlanRelease() planReleaseUseCase           { return c.plan }
func (c commandTestApp) GenerateNotes() generateNotesUseCase       { return nil }
func (c commandTestApp) ApproveRelease() approveReleaseUseCase     { return c.approve }
func (c commandTestApp) PublishRelease() publishReleaseUseCase     { return c.publish }
func (c commandTestApp) CalculateVersion() calculateVersionUseCase { return c.calculate }
func (c commandTestApp) SetVersion() setVersionUseCase             { return c.setVersion }
func (c commandTestApp) HasAI() bool                               { return false }
func (c commandTestApp) HasGovernance() bool                       { return c.hasGov }
func (c commandTestApp) GovernanceService() *governance.Service    { return c.govSvc }

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

type testReleaseRepo struct {
	latest *release.Release
}

func (r testReleaseRepo) Save(ctx context.Context, rel *release.Release) error { return nil }
func (testReleaseRepo) FindByID(ctx context.Context, id release.ReleaseID) (*release.Release, error) {
	return nil, nil
}
func (r testReleaseRepo) FindLatest(ctx context.Context, repoPath string) (*release.Release, error) {
	if r.latest == nil {
		return nil, release.ErrReleaseNotFound
	}
	return r.latest, nil
}
func (testReleaseRepo) FindByState(ctx context.Context, state release.ReleaseState) ([]*release.Release, error) {
	return nil, nil
}
func (testReleaseRepo) FindActive(ctx context.Context) ([]*release.Release, error) { return nil, nil }
func (testReleaseRepo) FindBySpecification(ctx context.Context, spec release.Specification) ([]*release.Release, error) {
	return nil, nil
}
func (testReleaseRepo) Delete(ctx context.Context, id release.ReleaseID) error { return nil }

func newTestRelease(t *testing.T, id string) *release.Release {
	t.Helper()
	rel := release.NewRelease(release.ReleaseID(id), "main", ".")
	cs := changes.NewChangeSet(changes.ChangeSetID("cs-"+id), "main", "HEAD")
	cs.AddCommit(changes.NewConventionalCommit("abc", changes.CommitTypeFeat, "feature"))
	plan := release.NewReleasePlan(version.Initial, version.MustParse("0.1.0"), changes.ReleaseTypeMinor, cs, false)
	if err := rel.SetPlan(plan); err != nil {
		t.Fatalf("SetPlan failed: %v", err)
	}
	if err := rel.SetVersion(plan.NextVersion, cfg.Versioning.TagPrefix+plan.NextVersion.String()); err != nil {
		t.Fatalf("SetVersion failed: %v", err)
	}
	notes := &release.ReleaseNotes{
		Changelog:   "changelog",
		Summary:     "summary",
		AIGenerated: true,
		GeneratedAt: time.Now(),
	}
	if err := rel.SetNotes(notes); err != nil {
		t.Fatalf("SetNotes failed: %v", err)
	}
	return rel
}

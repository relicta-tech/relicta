package cli

import (
	"context"
	"os"
	"testing"

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
	planned := newPlanOutput()

	app := releaseTestApp{
		gitRepo: stubGitRepo{},
		plan:    &fakePlanUseCase{executeOutput: planned},
	}

	out, err := runReleasePlan(context.Background(), app)
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
			latest: newTestRelease(t, "bump-release"),
		},
	}

	out, err := runReleaseBump(context.Background(), app, plan)
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

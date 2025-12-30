package cli

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta/internal/application/governance"
	"github.com/relicta-tech/relicta/internal/cgp/evaluator"
	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/internal/infrastructure/ai"
)

type govTestApp struct {
	gitRepo     sourcecontrol.GitRepository
	releaseRepo release.Repository
	govSvc      *governance.Service
	hasGov      bool
}

func (a govTestApp) Close() error                              { return nil }
func (a govTestApp) GitAdapter() sourcecontrol.GitRepository   { return a.gitRepo }
func (a govTestApp) ReleaseRepository() release.Repository     { return a.releaseRepo }
func (a govTestApp) PlanRelease() planReleaseUseCase           { return nil }
func (a govTestApp) GenerateNotes() generateNotesUseCase       { return nil }
func (a govTestApp) ApproveRelease() approveReleaseUseCase     { return nil }
func (a govTestApp) PublishRelease() publishReleaseUseCase     { return nil }
func (a govTestApp) CalculateVersion() calculateVersionUseCase { return nil }
func (a govTestApp) SetVersion() setVersionUseCase             { return nil }
func (a govTestApp) HasAI() bool                               { return false }
func (a govTestApp) AI() ai.Service                            { return nil }
func (a govTestApp) HasGovernance() bool                       { return a.hasGov }
func (a govTestApp) GovernanceService() *governance.Service    { return a.govSvc }
func (a govTestApp) InitReleaseServices(context.Context, string) error {
	return nil
}
func (a govTestApp) ReleaseServices() *release.Services { return nil }
func (a govTestApp) HasReleaseServices() bool           { return false }

func newGovernanceService(t *testing.T) *governance.Service {
	t.Helper()
	eval := evaluator.New()
	return governance.NewService(eval)
}

func TestEvaluateGovernanceReturnsResult(t *testing.T) {
	cfg = config.DefaultConfig()
	rel := newTestRelease(t, "gov-1")
	app := govTestApp{
		gitRepo:     stubGitRepo{},
		releaseRepo: testReleaseRepo{latest: rel},
		govSvc:      newGovernanceService(t),
		hasGov:      true,
	}

	ctx := context.Background()
	got, err := evaluateGovernance(ctx, app, rel)
	if err != nil {
		t.Fatalf("evaluateGovernance failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected governance output, got nil")
	}
}

func TestEvaluateGovernanceErrorWithoutService(t *testing.T) {
	rel := newTestRelease(t, "gov-2")
	app := govTestApp{gitRepo: stubGitRepo{}, releaseRepo: testReleaseRepo{latest: rel}}

	if _, err := evaluateGovernance(context.Background(), app, rel); err == nil {
		t.Fatal("expected error when governance service missing")
	}
}

func TestBuildGovernanceSummaryForTUI(t *testing.T) {
	rel := newTestRelease(t, "gov-3")
	app := govTestApp{
		gitRepo:     stubGitRepo{},
		releaseRepo: testReleaseRepo{latest: rel},
		govSvc:      newGovernanceService(t),
		hasGov:      true,
	}

	summary := buildGovernanceSummaryForTUI(context.Background(), app, rel)
	if summary == nil {
		t.Fatal("expected summary to be built")
	}
}

func TestEvaluateGovernanceForPublish(t *testing.T) {
	rel := newTestRelease(t, "gov-4")
	app := govTestApp{
		gitRepo:     stubGitRepo{},
		releaseRepo: testReleaseRepo{latest: rel},
		govSvc:      newGovernanceService(t),
		hasGov:      true,
	}

	ctx := context.Background()
	if _, err := evaluateGovernanceForPublish(ctx, app, rel); err != nil {
		t.Fatalf("evaluateGovernanceForPublish failed: %v", err)
	}
}

func TestEvaluateGovernanceForPublishError(t *testing.T) {
	rel := newTestRelease(t, "gov-5")
	app := govTestApp{
		gitRepo:     stubGitRepo{},
		releaseRepo: testReleaseRepo{latest: rel},
	}

	if _, err := evaluateGovernanceForPublish(context.Background(), app, rel); err == nil {
		t.Fatal("expected error without governance service")
	}
}

func TestRecordReleaseOutcomeAndPublishOutcome(t *testing.T) {
	origCfg := cfg
	defer func() { cfg = origCfg }()
	cfg = config.DefaultConfig()

	rel := newTestRelease(t, "gov-6")
	govSvc := newGovernanceService(t)
	app := govTestApp{
		gitRepo:     stubGitRepo{},
		releaseRepo: testReleaseRepo{latest: rel},
		govSvc:      govSvc,
		hasGov:      true,
	}

	ctx := context.Background()
	govResult, err := evaluateGovernance(ctx, app, rel)
	if err != nil {
		t.Fatalf("setup governance failed: %v", err)
	}

	recordReleaseOutcome(ctx, app, rel, govResult, true)
	recordPublishOutcome(ctx, app, rel, govResult, true, 0)
}

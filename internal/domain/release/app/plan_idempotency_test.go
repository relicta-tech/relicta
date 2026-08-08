package app

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

// Re-planning the same commits must not undo work already done.
//
// A run's ID is derived from its plan hash at construction, so planning the same
// base and HEAD twice produces the same ID — and saving then overwrote the stored
// run with a fresh, freshly-Planned one. Found by dogfooding: with a release
// sitting at `approved`, `relicta plan` reset it to `planned` and erased the
// state-machine history recording the bump, the notes and the approval. No error,
// no warning, and for a tool whose product is the audit trail that is the worst
// available outcome.

func planInputFor(t *testing.T) PlanReleaseInput {
	t.Helper()
	return PlanReleaseInput{
		RepoRoot:       "/path/to/repo",
		ConfigHash:     "config-hash",
		PluginPlanHash: "plugin-hash",
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "user@example.com",
		},
	}
}

// advanceToApproved drives a run through the lifecycle the way the CLI does, so
// the test protects the real sequence rather than a hand-set state field.
func advanceToApproved(t *testing.T, run *domain.ReleaseRun) {
	t.Helper()
	if err := run.SetVersion(version.MustParse("1.1.0"), "v1.1.0"); err != nil {
		t.Fatalf("SetVersion: %v", err)
	}
	if err := run.Bump("user@example.com"); err != nil {
		t.Fatalf("Bump: %v", err)
	}
	if err := run.GenerateNotes(&domain.ReleaseNotes{}, "notes-hash", "user@example.com"); err != nil {
		t.Fatalf("GenerateNotes: %v", err)
	}
	if err := run.Approve("user@example.com", false); err != nil {
		t.Fatalf("Approve: %v", err)
	}
}

func TestPlanReleaseUseCase_RePlanPreservesAdvancedRun(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()
	uc := NewPlanReleaseUseCase(repo, inspector, nil)

	first, err := uc.Execute(ctx, planInputFor(t))
	if err != nil {
		t.Fatalf("first Execute: %v", err)
	}

	stored := repo.runs[first.RunID]
	if stored == nil {
		t.Fatalf("run %s not stored", first.RunID)
	}
	advanceToApproved(t, stored)
	advancedState := stored.State()
	if advancedState == domain.StatePlanned {
		t.Fatalf("precondition failed: run should have advanced past planned, got %s", advancedState)
	}

	second, err := uc.Execute(ctx, planInputFor(t))
	if err != nil {
		t.Fatalf("second Execute: %v", err)
	}

	if second.RunID != first.RunID {
		t.Fatalf("identical inputs should address the same run: got %s then %s", first.RunID, second.RunID)
	}
	if !second.AlreadyExisted {
		t.Error("AlreadyExisted must be true so the CLI does not claim it saved a new plan")
	}
	if second.ExistingState != advancedState {
		t.Errorf("ExistingState = %s, want %s", second.ExistingState, advancedState)
	}
	if got := repo.runs[first.RunID].State(); got != advancedState {
		t.Errorf("stored run state = %s, want %s — re-planning discarded progress", got, advancedState)
	}
}

// DiscardExisting is the deliberate escape hatch, wired to `relicta plan --force`.
func TestPlanReleaseUseCase_RePlanWithDiscardExistingResets(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()
	uc := NewPlanReleaseUseCase(repo, inspector, nil)

	first, err := uc.Execute(ctx, planInputFor(t))
	if err != nil {
		t.Fatalf("first Execute: %v", err)
	}
	advanceToApproved(t, repo.runs[first.RunID])

	input := planInputFor(t)
	input.DiscardExisting = true
	second, err := uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("second Execute: %v", err)
	}

	if second.AlreadyExisted {
		t.Error("with DiscardExisting the run is rewritten, so AlreadyExisted must be false")
	}
	if got := repo.runs[second.RunID].State(); got != domain.StatePlanned {
		t.Errorf("stored run state = %s, want planned — --force should reset it", got)
	}
}

// A draft run carries no work worth protecting, so it may be replaced freely.
func TestPlanReleaseUseCase_RePlanOverwritesDraft(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()
	uc := NewPlanReleaseUseCase(repo, inspector, nil)

	first, err := uc.Execute(ctx, planInputFor(t))
	if err != nil {
		t.Fatalf("first Execute: %v", err)
	}
	// Leave it at planned and re-plan: still idempotent, because planned is
	// already past draft and there is no reason to rewrite it.
	second, err := uc.Execute(ctx, planInputFor(t))
	if err != nil {
		t.Fatalf("second Execute: %v", err)
	}
	if second.RunID != first.RunID {
		t.Errorf("same inputs should address the same run")
	}
}

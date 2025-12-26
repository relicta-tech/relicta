// Package app provides application services (use cases) for release governance.
package app

import (
	"context"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/domain/release/ports"
)

// GetStatusInput contains the input for getting release status.
type GetStatusInput struct {
	RepoRoot string
	RunID    domain.RunID // If empty, uses latest
}

// GetStatusOutput contains the status of a release run.
type GetStatusOutput struct {
	RunID          domain.RunID
	State          domain.RunState
	HeadSHA        domain.CommitSHA
	PlanHash       string
	VersionCurrent string
	VersionNext    string
	TagName        string
	BumpKind       domain.BumpKind
	RiskScore      float64
	CommitCount    int
	StepsTotal     int
	StepsDone      int
	StepsFailed    int
	StepsPending   int
	NextAction     string
	CanBump        bool
	CanApprove     bool
	CanPublish     bool
	CanRetry       bool
	Stale          bool
	Warning        string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	PublishedAt    *time.Time
	LastError      string
}

// GetStatusUseCase handles the get status use case.
type GetStatusUseCase struct {
	repo          ports.ReleaseRunRepository
	repoInspector ports.RepoInspector
}

// NewGetStatusUseCase creates a new GetStatusUseCase.
func NewGetStatusUseCase(
	repo ports.ReleaseRunRepository,
	repoInspector ports.RepoInspector,
) *GetStatusUseCase {
	return &GetStatusUseCase{
		repo:          repo,
		repoInspector: repoInspector,
	}
}

// Execute gets the status of a release run.
func (uc *GetStatusUseCase) Execute(ctx context.Context, input GetStatusInput) (*GetStatusOutput, error) {
	// Load the run
	run, err := uc.loadRun(ctx, input.RepoRoot, input.RunID)
	if err != nil {
		return nil, err
	}

	summary := run.Summary()

	// Calculate step stats
	stepsPending := summary.StepsTotal - summary.StepsDone - summary.StepsFailed

	// Determine next action
	nextAction := determineNextAction(run.State())

	// Check for staleness
	stale := false
	warning := ""
	if !run.State().IsFinal() {
		staleThreshold := time.Now().Add(-1 * time.Hour)
		if run.UpdatedAt().Before(staleThreshold) {
			stale = true
			warning = "Release was last updated over 1 hour ago. Consider running 'relicta plan' to refresh state."
		}
	}

	// Check if HEAD has drifted
	currentHead, err := uc.repoInspector.HeadSHA(ctx)
	if err == nil && currentHead != run.HeadSHA() && !run.State().IsFinal() {
		if warning != "" {
			warning += " "
		}
		warning += "HEAD has changed since plan was created."
	}

	return &GetStatusOutput{
		RunID:          run.ID(),
		State:          run.State(),
		HeadSHA:        run.HeadSHA(),
		PlanHash:       run.PlanHash(),
		VersionCurrent: summary.VersionCurrent,
		VersionNext:    summary.VersionNext,
		TagName:        run.TagName(),
		BumpKind:       summary.BumpKind,
		RiskScore:      summary.RiskScore,
		CommitCount:    summary.CommitCount,
		StepsTotal:     summary.StepsTotal,
		StepsDone:      summary.StepsDone,
		StepsFailed:    summary.StepsFailed,
		StepsPending:   stepsPending,
		NextAction:     nextAction,
		CanBump:        run.State() == domain.StatePlanned,
		CanApprove:     run.State() == domain.StateNotesReady,
		CanPublish:     run.State() == domain.StateApproved,
		CanRetry:       run.State() == domain.StateFailed,
		Stale:          stale,
		Warning:        warning,
		CreatedAt:      run.CreatedAt(),
		UpdatedAt:      run.UpdatedAt(),
		PublishedAt:    run.PublishedAt(),
		LastError:      run.LastError(),
	}, nil
}

// determineNextAction returns the suggested next action based on state.
func determineNextAction(state domain.RunState) string {
	switch state {
	case domain.StateDraft:
		return "plan"
	case domain.StatePlanned:
		return "bump"
	case domain.StateVersioned:
		return "notes"
	case domain.StateNotesReady:
		return "approve"
	case domain.StateApproved:
		return "publish"
	case domain.StatePublishing:
		return "wait"
	case domain.StatePublished:
		return "done"
	case domain.StateFailed:
		return "retry or cancel"
	case domain.StateCancelled:
		return "plan"
	default:
		return ""
	}
}

// loadRun loads a run by ID or the latest run.
func (uc *GetStatusUseCase) loadRun(ctx context.Context, repoRoot string, runID domain.RunID) (*domain.ReleaseRun, error) {
	if runID != "" {
		if fileRepo, ok := uc.repo.(interface {
			LoadFromRepo(context.Context, string, domain.RunID) (*domain.ReleaseRun, error)
		}); ok {
			return fileRepo.LoadFromRepo(ctx, repoRoot, runID)
		}
		return uc.repo.Load(ctx, runID)
	}
	return uc.repo.LoadLatest(ctx, repoRoot)
}

// Package app provides application services (use cases) for release governance.
package app

import (
	"context"
	"fmt"

	"github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/domain/release/ports"
)

// ApproveReleaseInput contains the input for approving a release.
type ApproveReleaseInput struct {
	RepoRoot    string
	RunID       domain.RunID // If empty, uses latest
	Actor       ports.ActorInfo
	AutoApprove bool   // CI/--yes mode
	Force       bool   // Force approval even if HEAD changed
}

// ApproveReleaseOutput contains the output from approving a release.
type ApproveReleaseOutput struct {
	RunID       domain.RunID
	PlanHash    string // The plan hash that was approved
	Approved    bool
	ApprovedBy  string
	VersionNext string
}

// ApproveReleaseUseCase handles the approve release use case.
type ApproveReleaseUseCase struct {
	repo          ports.ReleaseRunRepository
	repoInspector ports.RepoInspector
	lockManager   ports.LockManager
	stateMachine  *domain.StateMachineService
}

// NewApproveReleaseUseCase creates a new ApproveReleaseUseCase.
func NewApproveReleaseUseCase(
	repo ports.ReleaseRunRepository,
	repoInspector ports.RepoInspector,
	lockManager ports.LockManager,
	stateMachine *domain.StateMachineService,
) *ApproveReleaseUseCase {
	return &ApproveReleaseUseCase{
		repo:          repo,
		repoInspector: repoInspector,
		lockManager:   lockManager,
		stateMachine:  stateMachine,
	}
}

// Execute approves a release.
func (uc *ApproveReleaseUseCase) Execute(ctx context.Context, input ApproveReleaseInput) (*ApproveReleaseOutput, error) {
	// Load the run
	run, err := uc.loadRun(ctx, input.RepoRoot, input.RunID)
	if err != nil {
		return nil, err
	}

	// Acquire lock
	if uc.lockManager != nil {
		release, err := uc.lockManager.Acquire(ctx, input.RepoRoot, run.ID())
		if err != nil {
			return nil, fmt.Errorf("failed to acquire lock: %w", err)
		}
		defer release()
	}

	// Validate HEAD matches unless forced
	if !input.Force {
		currentHead, err := uc.repoInspector.HeadSHA(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get current HEAD: %w", err)
		}
		if err := run.ValidateHeadMatch(currentHead); err != nil {
			return nil, fmt.Errorf("%w (use --force to override)", err)
		}
	}

	// Check if auto-approve is allowed
	if input.AutoApprove {
		if !run.CanAutoApprove() {
			return nil, fmt.Errorf("auto-approve not allowed: risk score %.2f exceeds threshold", run.RiskScore())
		}
	}

	// Approve the release
	if err := run.Approve(input.Actor.ID, input.AutoApprove); err != nil {
		return nil, fmt.Errorf("failed to approve: %w", err)
	}

	// Save the run
	if err := uc.repo.Save(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to save run: %w", err)
	}

	return &ApproveReleaseOutput{
		RunID:       run.ID(),
		PlanHash:    run.PlanHash(),
		Approved:    true,
		ApprovedBy:  input.Actor.ID,
		VersionNext: run.VersionNext().String(),
	}, nil
}

// loadRun loads a run by ID or the latest run.
func (uc *ApproveReleaseUseCase) loadRun(ctx context.Context, repoRoot string, runID domain.RunID) (*domain.ReleaseRun, error) {
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

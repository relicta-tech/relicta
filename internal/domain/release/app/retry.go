// Package app provides application services (use cases) for release governance.
package app

import (
	"context"
	"fmt"

	"github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/domain/release/ports"
)

// RetryPublishInput contains the input for retrying a failed publish.
type RetryPublishInput struct {
	RepoRoot string
	RunID    domain.RunID // If empty, uses latest
	Actor    ports.ActorInfo
	Force    bool // Force retry even if HEAD changed
}

// RetryPublishOutput contains the output from retrying a publish.
type RetryPublishOutput struct {
	RunID       domain.RunID
	Published   bool
	StepResults []StepResult
	VersionNext string
}

// RetryPublishUseCase handles the retry publish use case.
type RetryPublishUseCase struct {
	repo          ports.ReleaseRunRepository
	repoInspector ports.RepoInspector
	lockManager   ports.LockManager
	publisher     ports.Publisher
	stateMachine  *domain.StateMachineService
}

// NewRetryPublishUseCase creates a new RetryPublishUseCase.
func NewRetryPublishUseCase(
	repo ports.ReleaseRunRepository,
	repoInspector ports.RepoInspector,
	lockManager ports.LockManager,
	publisher ports.Publisher,
	stateMachine *domain.StateMachineService,
) *RetryPublishUseCase {
	return &RetryPublishUseCase{
		repo:          repo,
		repoInspector: repoInspector,
		lockManager:   lockManager,
		publisher:     publisher,
		stateMachine:  stateMachine,
	}
}

// Execute retries a failed publish.
func (uc *RetryPublishUseCase) Execute(ctx context.Context, input RetryPublishInput) (*RetryPublishOutput, error) {
	// Load the run
	run, err := uc.loadRun(ctx, input.RepoRoot, input.RunID)
	if err != nil {
		return nil, err
	}

	// Check run state
	if run.State() != domain.StateFailed {
		return nil, fmt.Errorf("cannot retry from state %s (must be failed)", run.State())
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

	// Retry the publish
	if err := run.RetryPublish(input.Actor.ID); err != nil {
		return nil, fmt.Errorf("failed to retry: %w", err)
	}

	// Save the run
	if err := uc.repo.Save(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to save run: %w", err)
	}

	// Now use the publish use case to continue
	publishUC := NewPublishReleaseUseCase(
		uc.repo,
		uc.repoInspector,
		nil, // Don't acquire lock again
		uc.publisher,
		uc.stateMachine,
	)

	publishOutput, err := publishUC.Execute(ctx, PublishReleaseInput{
		RepoRoot: input.RepoRoot,
		RunID:    run.ID(),
		Actor:    input.Actor,
		Force:    input.Force,
	})

	if err != nil {
		return &RetryPublishOutput{
			RunID:       run.ID(),
			Published:   false,
			VersionNext: run.VersionNext().String(),
		}, err
	}

	return &RetryPublishOutput{
		RunID:       publishOutput.RunID,
		Published:   publishOutput.Published,
		StepResults: publishOutput.StepResults,
		VersionNext: publishOutput.VersionNext,
	}, nil
}

// loadRun loads a run by ID or the latest run.
func (uc *RetryPublishUseCase) loadRun(ctx context.Context, repoRoot string, runID domain.RunID) (*domain.ReleaseRun, error) {
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

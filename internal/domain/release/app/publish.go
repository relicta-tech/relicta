// Package app provides application services (use cases) for release governance.
package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/domain/release/ports"
)

// PublishReleaseInput contains the input for publishing a release.
type PublishReleaseInput struct {
	RepoRoot string
	RunID    domain.RunID // If empty, uses latest
	Actor    ports.ActorInfo
	Force    bool // Force publishing even if HEAD changed
	DryRun   bool // Simulate without making changes
}

// PublishReleaseOutput contains the output from publishing a release.
type PublishReleaseOutput struct {
	RunID       domain.RunID
	Published   bool
	StepResults []StepResult
	VersionNext string
}

// StepResult contains the result of executing a step.
type StepResult struct {
	StepName string
	Success  bool
	Skipped  bool // True if step was already done (idempotency)
	Output   string
	Error    string
}

// PublishReleaseUseCase handles the publish release use case.
type PublishReleaseUseCase struct {
	repo          ports.ReleaseRunRepository
	repoInspector ports.RepoInspector
	lockManager   ports.LockManager
	publisher     ports.Publisher
	stateMachine  *domain.StateMachineService
}

// NewPublishReleaseUseCase creates a new PublishReleaseUseCase.
func NewPublishReleaseUseCase(
	repo ports.ReleaseRunRepository,
	repoInspector ports.RepoInspector,
	lockManager ports.LockManager,
	publisher ports.Publisher,
	stateMachine *domain.StateMachineService,
) *PublishReleaseUseCase {
	return &PublishReleaseUseCase{
		repo:          repo,
		repoInspector: repoInspector,
		lockManager:   lockManager,
		publisher:     publisher,
		stateMachine:  stateMachine,
	}
}

// Execute publishes a release with step-level idempotency.
func (uc *PublishReleaseUseCase) Execute(ctx context.Context, input PublishReleaseInput) (*PublishReleaseOutput, error) {
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

	// Check run state
	if run.State() == domain.StatePublished {
		return &PublishReleaseOutput{
			RunID:       run.ID(),
			Published:   true,
			VersionNext: run.VersionNext().String(),
		}, nil // Already published - idempotent success
	}

	if run.State() != domain.StateApproved && run.State() != domain.StatePublishing {
		return nil, fmt.Errorf("cannot publish from state %s (must be approved or publishing)", run.State())
	}

	// Transition to Publishing if not already
	if run.State() == domain.StateApproved {
		if err := run.StartPublishing(input.Actor.ID); err != nil {
			return nil, fmt.Errorf("failed to start publishing: %w", err)
		}
		if err := uc.repo.Save(ctx, run); err != nil {
			return nil, fmt.Errorf("failed to save run: %w", err)
		}
	}

	// Execute steps with idempotency
	var stepResults []StepResult
	for {
		step := run.NextPendingStep()
		if step == nil {
			break // All steps done
		}

		result, err := uc.executeStep(ctx, run, step, input.DryRun)
		stepResults = append(stepResults, *result)

		if err != nil || !result.Success {
			// Step failed - save state and return
			if err := uc.repo.Save(ctx, run); err != nil {
				return nil, fmt.Errorf("failed to save run after step failure: %w", err)
			}
			return &PublishReleaseOutput{
				RunID:       run.ID(),
				Published:   false,
				StepResults: stepResults,
				VersionNext: run.VersionNext().String(),
			}, fmt.Errorf("step %s failed: %w", step.Name, err)
		}

		// Save after each successful step for resumability
		if err := uc.repo.Save(ctx, run); err != nil {
			return nil, fmt.Errorf("failed to save run after step: %w", err)
		}
	}

	// All steps completed - mark as published
	if run.AllStepsSucceeded() {
		if err := run.MarkPublished(input.Actor.ID); err != nil {
			return nil, fmt.Errorf("failed to mark as published: %w", err)
		}
		if err := uc.repo.Save(ctx, run); err != nil {
			return nil, fmt.Errorf("failed to save published run: %w", err)
		}
	}

	return &PublishReleaseOutput{
		RunID:       run.ID(),
		Published:   run.State() == domain.StatePublished,
		StepResults: stepResults,
		VersionNext: run.VersionNext().String(),
	}, nil
}

// executeStep executes a single step with idempotency checks.
func (uc *PublishReleaseUseCase) executeStep(ctx context.Context, run *domain.ReleaseRun, step *domain.StepPlan, dryRun bool) (*StepResult, error) {
	result := &StepResult{
		StepName: step.Name,
	}

	// Check if step is already done (idempotency)
	if uc.publisher != nil {
		alreadyDone, err := uc.publisher.CheckIdempotency(ctx, run, step)
		if err == nil && alreadyDone {
			// Step already completed externally
			if err := run.MarkStepSkipped(step.Name, "Already completed externally"); err != nil {
				return result, err
			}
			result.Success = true
			result.Skipped = true
			result.Output = "Skipped: already completed"
			return result, nil
		}
	}

	// Mark step as started
	if err := run.MarkStepStarted(step.Name); err != nil {
		return result, err
	}

	// Dry run mode - don't actually execute
	if dryRun {
		if err := run.MarkStepSkipped(step.Name, "Dry run mode"); err != nil {
			return result, err
		}
		result.Success = true
		result.Skipped = true
		result.Output = "Dry run: would execute " + step.Name
		return result, nil
	}

	// Execute the step
	if uc.publisher == nil {
		return result, fmt.Errorf("no publisher configured")
	}

	stepResult, err := uc.publisher.ExecuteStep(ctx, run, step)
	if err != nil {
		if markErr := run.MarkStepFailed(step.Name, err); markErr != nil {
			return result, errors.Join(fmt.Errorf("step failed: %w", err), fmt.Errorf("failed to mark step: %w", markErr))
		}
		result.Error = err.Error()
		return result, err
	}

	if !stepResult.Success {
		stepErr := fmt.Errorf("step returned failure: %w", stepResult.Error)
		if markErr := run.MarkStepFailed(step.Name, stepErr); markErr != nil {
			return result, errors.Join(fmt.Errorf("step failed: %w", stepErr), fmt.Errorf("failed to mark step: %w", markErr))
		}
		result.Error = stepResult.Error.Error()
		return result, stepErr
	}

	// Mark step as done
	if err := run.MarkStepDone(step.Name, stepResult.Output); err != nil {
		return result, err
	}

	result.Success = true
	result.Skipped = stepResult.AlreadyDone
	result.Output = stepResult.Output
	return result, nil
}

// loadRun loads a run by ID or the latest run.
func (uc *PublishReleaseUseCase) loadRun(ctx context.Context, repoRoot string, runID domain.RunID) (*domain.ReleaseRun, error) {
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

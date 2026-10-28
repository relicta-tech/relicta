// Package app provides application services (use cases) for release governance.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
)

// PublishReleaseInput contains the input for publishing a release.
type PublishReleaseInput struct {
	RepoRoot string
	RunID    domain.RunID // If empty, uses latest
	Actor    ports.ActorInfo
	Force    bool // Force publishing even if HEAD changed
	DryRun   bool // Simulate without making changes
	// ExternalEvidence is stored on the existing tag step as the external
	// publisher's report of publication and artifact digests.
	ExternalEvidence string
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

	// Validate approval is bound to current plan hash (idempotency check)
	if err := run.ValidateApprovalPlanHash(); err != nil {
		return nil, fmt.Errorf("approval validation failed: %w", err)
	}

	// An empty execution plan means approve never planned any steps —
	// publishing would be a no-op that still reports success. Fail loudly
	// instead of marking a release published with zero side effects.
	if len(run.Steps()) == 0 {
		return nil, fmt.Errorf("no execution steps planned for run %s: re-run 'relicta approve' to build the execution plan", run.ID())
	}
	if input.ExternalEvidence != "" {
		var status *domain.StepStatus
		if len(run.Steps()) == 1 {
			status = run.StepStatus(run.Steps()[0].Name)
		}
		if run.State() != domain.StateApproved || len(run.Steps()) != 1 || run.Steps()[0].Type != domain.StepTypeTag || status == nil || status.State != domain.StepPending {
			return nil, fmt.Errorf("external publication evidence requires an approved run with only its tag step")
		}
		if err := validateExternalEvidenceBinding(run, input.ExternalEvidence); err != nil {
			return nil, err
		}
	}

	// Dry run: report what would execute without mutating the run. The
	// previous implementation marked every pending step as skipped and
	// persisted the run, so the dry run itself advanced the release to
	// published and the real publish that followed became an idempotent
	// no-op — no tag, no plugins, state=published (issue #126).
	if input.DryRun {
		var stepResults []StepResult
		for _, step := range run.Steps() {
			status := run.StepStatus(step.Name)
			if status != nil && status.State != domain.StepPending && status.State != domain.StepFailed {
				continue // already executed in a previous (real) attempt
			}
			stepResults = append(stepResults, StepResult{
				StepName: step.Name,
				Success:  true,
				Skipped:  true,
				Output:   "Dry run: would execute " + step.Name,
			})
		}
		return &PublishReleaseOutput{
			RunID:       run.ID(),
			Published:   false,
			StepResults: stepResults,
			VersionNext: run.VersionNext().String(),
		}, nil
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

		result, err := uc.executeStep(ctx, run, step, input.ExternalEvidence)
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

// RecordExternalFailure marks a CI release run failed after its external publisher
// reports failure. It is intentionally limited to the tag-push plan's single tag
// step; other execution plans must use the normal publish path.
func (uc *PublishReleaseUseCase) RecordExternalFailure(ctx context.Context, input PublishReleaseInput) (*PublishReleaseOutput, error) {
	run, err := uc.loadRun(ctx, input.RepoRoot, input.RunID)
	if err != nil {
		return nil, err
	}
	if run.State() != domain.StateApproved || len(run.Steps()) != 1 || run.Steps()[0].Type != domain.StepTypeTag {
		return nil, fmt.Errorf("external publication failure requires an approved run with only its tag step")
	}
	if status := run.StepStatus(run.Steps()[0].Name); status == nil || status.State != domain.StepPending {
		return nil, fmt.Errorf("external publication failure requires a pending tag step")
	}
	if input.ExternalEvidence == "" {
		return nil, fmt.Errorf("external publication failure requires evidence")
	}
	if err := validateExternalEvidenceBinding(run, input.ExternalEvidence); err != nil {
		return nil, err
	}
	if uc.repoInspector == nil {
		return nil, fmt.Errorf("repository inspector not configured")
	}
	currentHead, err := uc.repoInspector.HeadSHA(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get current HEAD: %w", err)
	}
	if err := run.ValidateHeadMatch(currentHead); err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	if uc.lockManager != nil {
		release, err := uc.lockManager.Acquire(ctx, input.RepoRoot, run.ID())
		if err != nil {
			return nil, fmt.Errorf("failed to acquire lock: %w", err)
		}
		defer release()
	}
	if err := run.StartPublishing(input.Actor.ID); err != nil {
		return nil, fmt.Errorf("failed to start publishing: %w", err)
	}
	step := run.NextPendingStep()
	if step == nil || step.Type != domain.StepTypeTag {
		return nil, fmt.Errorf("external publication run has no pending tag step")
	}
	if err := run.MarkStepStarted(step.Name); err != nil {
		return nil, fmt.Errorf("failed to start external tag step: %w", err)
	}
	failure := fmt.Errorf("external publisher failed: %s", input.ExternalEvidence)
	if err := run.MarkStepFailed(step.Name, failure); err != nil {
		return nil, fmt.Errorf("failed to record external publication failure: %w", err)
	}
	if err := run.MarkFailed(failure.Error(), input.Actor.ID); err != nil {
		return nil, fmt.Errorf("failed to mark release failed: %w", err)
	}
	if err := uc.repo.Save(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to save external publication failure: %w", err)
	}
	return &PublishReleaseOutput{
		RunID:       run.ID(),
		Published:   false,
		StepResults: []StepResult{{StepName: step.Name, Success: false, Error: failure.Error()}},
		VersionNext: run.VersionNext().String(),
	}, nil
}

// validateExternalEvidenceBinding ensures evidence accepted for the tag step
// identifies the exact release represented by the approved run. The CLI also
// checks this before dispatch, but the use case is a public application
// boundary and must enforce the invariant independently.
func validateExternalEvidenceBinding(run *domain.ReleaseRun, evidence string) error {
	var binding struct {
		Tag    string `json:"tag"`
		Commit string `json:"commit"`
	}
	if err := json.Unmarshal([]byte(evidence), &binding); err != nil {
		return fmt.Errorf("decode external publication evidence binding: %w", err)
	}
	if binding.Tag == "" || binding.Commit == "" {
		return fmt.Errorf("external publication evidence requires tag and commit")
	}
	if binding.Tag != run.TagName() || binding.Commit != run.HeadSHA().String() {
		return fmt.Errorf("external publication evidence does not match approved tag %q and commit %q", run.TagName(), run.HeadSHA())
	}
	return nil
}

// executeStep executes a single step with idempotency checks.
func (uc *PublishReleaseUseCase) executeStep(ctx context.Context, run *domain.ReleaseRun, step *domain.StepPlan, externalEvidence string) (*StepResult, error) {
	result := &StepResult{
		StepName: step.Name,
	}

	// Check if step is already done (idempotency)
	if uc.publisher != nil {
		alreadyDone, err := uc.publisher.CheckIdempotency(ctx, run, step)
		if step.Type == domain.StepTypeTag && externalEvidence != "" && (err != nil || !alreadyDone) {
			if err != nil {
				return result, fmt.Errorf("verify externally published tag: %w", err)
			}
			return result, fmt.Errorf("externally published tag is not present; refusing to create it")
		}
		if err == nil && alreadyDone {
			// Step already completed externally
			output := "Already completed externally"
			if step.Type == domain.StepTypeTag && externalEvidence != "" {
				output = externalEvidence
			}
			if err := run.MarkStepSkipped(step.Name, output); err != nil {
				return result, err
			}
			result.Success = true
			result.Skipped = true
			result.Output = output
			return result, nil
		}
	}

	// Mark step as started
	if err := run.MarkStepStarted(step.Name); err != nil {
		return result, err
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

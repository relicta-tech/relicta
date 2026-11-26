// Package app provides application services (use cases) for release governance.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/relicta-tech/relicta/v5/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v5/internal/domain/release/ports"
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

// ExternalPublisherResult records one independently executed publisher.
type ExternalPublisherResult struct {
	Status string `json:"status"`
	URL    string `json:"url,omitempty"`
	Digest string `json:"digest,omitempty"`
}

// ExternalReleaseEvidence binds the release workflow's publisher outcomes to
// the exact approved release run. Publisher statuses are published, failed,
// or skipped (for configured optional publishers such as Homebrew prereleases).
type ExternalReleaseEvidence struct {
	Outcome    string                             `json:"outcome"`
	Provider   string                             `json:"provider"`
	RunURL     string                             `json:"run_url"`
	Tag        string                             `json:"tag"`
	Commit     string                             `json:"commit"`
	Publishers map[string]ExternalPublisherResult `json:"publishers"`
	Failure    string                             `json:"failure,omitempty"`
}

var externalDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// Validate checks the evidence shape and rejects incomplete success reports.
func (e ExternalReleaseEvidence) Validate() error {
	if e.Outcome != "published" && e.Outcome != "failed" {
		return fmt.Errorf("external outcome must be published or failed")
	}
	if strings.TrimSpace(e.Provider) == "" || strings.TrimSpace(e.Tag) == "" || strings.TrimSpace(e.Commit) == "" {
		return fmt.Errorf("external evidence requires provider, tag, and commit")
	}
	u, err := url.ParseRequestURI(e.RunURL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("external evidence run_url must be an HTTPS URL")
	}
	if len(e.Publishers) == 0 {
		return fmt.Errorf("external evidence must include publisher results")
	}
	for _, required := range []string{"github_release", "container", "homebrew"} {
		if _, ok := e.Publishers[required]; !ok {
			return fmt.Errorf("external evidence is missing required publisher %q", required)
		}
	}
	if len(e.Publishers) != 3 {
		return fmt.Errorf("external evidence must contain exactly github_release, container, and homebrew results")
	}
	failed := false
	published := false
	for name, result := range e.Publishers {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("external evidence contains an empty publisher name")
		}
		switch result.Status {
		case "published":
			published = true
			if strings.TrimSpace(result.URL) == "" {
				return fmt.Errorf("published publisher %q requires a URL", name)
			}
		case "failed":
			failed = true
		case "skipped":
		default:
			return fmt.Errorf("publisher %q has invalid status %q", name, result.Status)
		}
		if result.URL != "" {
			publisherURL, err := url.ParseRequestURI(result.URL)
			if err != nil || publisherURL.Scheme != "https" || publisherURL.Host == "" {
				return fmt.Errorf("publisher %q URL must be HTTPS", name)
			}
		}
	}
	for _, required := range []string{"github_release", "container"} {
		if e.Publishers[required].Status == "skipped" {
			return fmt.Errorf("required publisher %q cannot be skipped", required)
		}
	}
	if e.Outcome == "published" && (failed || !published) {
		return fmt.Errorf("published outcome requires at least one published publisher and no failures")
	}
	if e.Outcome == "published" && (e.Publishers["github_release"].Status != "published" || e.Publishers["container"].Status != "published") {
		return fmt.Errorf("published outcome requires both GitHub release and container publication")
	}
	if e.Outcome == "published" && !externalDigestPattern.MatchString(e.Publishers["container"].Digest) {
		return fmt.Errorf("published container evidence requires a sha256 digest")
	}
	if e.Outcome == "failed" && (!failed || strings.TrimSpace(e.Failure) == "") {
		return fmt.Errorf("failed outcome requires a failed publisher and failure description")
	}
	return nil
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

		result, err := uc.executeStep(ctx, run, step)
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

// RecordExternalOutcome finalizes a release after an external CI workflow has
// completed its publishers and supplies outcome data. Only the built-in tag
// step and optional attestation step may be finalized externally.
func (uc *PublishReleaseUseCase) RecordExternalOutcome(ctx context.Context, input PublishReleaseInput, evidence ExternalReleaseEvidence) (*PublishReleaseOutput, error) {
	if err := evidence.Validate(); err != nil {
		return nil, err
	}
	run, err := uc.loadRun(ctx, input.RepoRoot, input.RunID)
	if err != nil {
		return nil, err
	}
	if uc.lockManager != nil {
		release, err := uc.lockManager.Acquire(ctx, input.RepoRoot, run.ID())
		if err != nil {
			return nil, fmt.Errorf("failed to acquire lock: %w", err)
		}
		defer release()
	}
	if len(run.Steps()) < 1 || len(run.Steps()) > 2 {
		return nil, fmt.Errorf("external outcome requires a release plan with a tag step and at most one attestation step")
	}
	tagFound := false
	for _, step := range run.Steps() {
		switch step.Type {
		case domain.StepTypeTag:
			if tagFound {
				return nil, fmt.Errorf("external outcome cannot record duplicate tag steps")
			}
			tagFound = true
		case domain.StepTypeAttestation:
		default:
			return nil, fmt.Errorf("external outcome cannot mark planned %s step complete", step.Type)
		}
	}
	if !tagFound {
		return nil, fmt.Errorf("external outcome requires a tag step")
	}
	if evidence.Tag != run.TagName() || evidence.Commit != run.HeadSHA().String() {
		return nil, fmt.Errorf("external evidence does not match approved tag %q and commit %q", run.TagName(), run.HeadSHA())
	}
	if evidence.Publishers["homebrew"].Status == "skipped" && !run.VersionNext().IsPrerelease() {
		return nil, fmt.Errorf("homebrew may only be skipped for a prerelease")
	}
	canonical, err := json.Marshal(evidence)
	if err != nil {
		return nil, fmt.Errorf("encode external evidence: %w", err)
	}
	if run.State() == domain.StatePublished && run.ExternalPublicationEvidence() == string(canonical) {
		return &PublishReleaseOutput{RunID: run.ID(), Published: true, VersionNext: run.VersionNext().String()}, nil
	}
	if run.State() == domain.StateFailed && run.ExternalPublicationEvidence() == string(canonical) {
		return &PublishReleaseOutput{RunID: run.ID(), Published: false, VersionNext: run.VersionNext().String()}, nil
	}
	if run.State() != domain.StateApproved {
		return nil, fmt.Errorf("external outcome requires an approved run, got %s", run.State())
	}
	var resultErr error
	if evidence.Outcome == "failed" {
		resultErr = run.RecordExternalPublicationFailure(input.Actor.ID, string(canonical), evidence.Failure)
	} else {
		resultErr = run.MarkExternallyPublished(input.Actor.ID, string(canonical))
	}
	if resultErr != nil {
		return nil, resultErr
	}
	if err := uc.repo.Save(ctx, run); err != nil {
		return nil, fmt.Errorf("save external publication outcome: %w", err)
	}
	return &PublishReleaseOutput{RunID: run.ID(), Published: evidence.Outcome == "published", VersionNext: run.VersionNext().String()}, nil
}

// executeStep executes a single step with idempotency checks.
func (uc *PublishReleaseUseCase) executeStep(ctx context.Context, run *domain.ReleaseRun, step *domain.StepPlan) (*StepResult, error) {
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

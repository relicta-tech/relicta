// Package ports defines the interfaces (ports) for the release governance bounded context.
package ports

import (
	"context"

	"github.com/relicta-tech/relicta/internal/releasegov/domain"
)

// StepResult represents the result of executing a publishing step.
type StepResult struct {
	Success bool
	Output  string
	Error   error

	// AlreadyDone is true if the step was already completed (idempotency).
	AlreadyDone bool
}

// Publisher executes publishing steps.
type Publisher interface {
	// ExecuteStep executes a single step in the publishing plan.
	// Implementations should check for idempotency before executing.
	ExecuteStep(ctx context.Context, run *domain.ReleaseRun, step *domain.StepPlan) (*StepResult, error)

	// CheckIdempotency checks if a step has already been executed.
	// Returns true if the step output already exists.
	CheckIdempotency(ctx context.Context, run *domain.ReleaseRun, step *domain.StepPlan) (bool, error)
}

// NotesGenerator generates release notes.
type NotesGenerator interface {
	// Generate creates release notes for the given run.
	Generate(ctx context.Context, run *domain.ReleaseRun, options NotesOptions) (*domain.ReleaseNotes, error)

	// ComputeInputsHash computes a hash of the inputs used to generate notes.
	// This is used to detect when notes need to be regenerated.
	ComputeInputsHash(run *domain.ReleaseRun, options NotesOptions) string
}

// NotesOptions configures notes generation.
type NotesOptions struct {
	AudiencePreset string // e.g., "developer", "user", "all"
	TonePreset     string // e.g., "formal", "casual"
	UseAI          bool
	Provider       string
	Model          string
	RepositoryURL  string
}

// VersionCalculator calculates the next version.
type VersionCalculator interface {
	// Calculate computes the next version based on the commits.
	Calculate(ctx context.Context, run *domain.ReleaseRun) error
}

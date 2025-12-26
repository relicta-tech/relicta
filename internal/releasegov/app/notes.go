// Package app provides application services (use cases) for release governance.
package app

import (
	"context"
	"fmt"

	"github.com/relicta-tech/relicta/internal/releasegov/domain"
	"github.com/relicta-tech/relicta/internal/releasegov/ports"
)

// GenerateNotesInput contains the input for generating release notes.
type GenerateNotesInput struct {
	RepoRoot string
	RunID    domain.RunID // If empty, uses latest
	Options  ports.NotesOptions
	Actor    ports.ActorInfo
	Force    bool // Force regeneration even if HEAD changed
}

// GenerateNotesOutput contains the output from generating release notes.
type GenerateNotesOutput struct {
	RunID      domain.RunID
	Notes      *domain.ReleaseNotes
	InputsHash string
}

// GenerateNotesUseCase handles the generate notes use case.
type GenerateNotesUseCase struct {
	repo          ports.ReleaseRunRepository
	repoInspector ports.RepoInspector
	notesGen      ports.NotesGenerator
	stateMachine  *domain.StateMachineService
}

// NewGenerateNotesUseCase creates a new GenerateNotesUseCase.
func NewGenerateNotesUseCase(
	repo ports.ReleaseRunRepository,
	repoInspector ports.RepoInspector,
	notesGen ports.NotesGenerator,
	stateMachine *domain.StateMachineService,
) *GenerateNotesUseCase {
	return &GenerateNotesUseCase{
		repo:          repo,
		repoInspector: repoInspector,
		notesGen:      notesGen,
		stateMachine:  stateMachine,
	}
}

// Execute generates release notes for a run.
func (uc *GenerateNotesUseCase) Execute(ctx context.Context, input GenerateNotesInput) (*GenerateNotesOutput, error) {
	// Load the run
	run, err := uc.loadRun(ctx, input.RepoRoot, input.RunID)
	if err != nil {
		return nil, err
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

	// Generate notes
	notes, err := uc.notesGen.Generate(ctx, run, input.Options)
	if err != nil {
		return nil, fmt.Errorf("failed to generate notes: %w", err)
	}

	// Compute inputs hash
	inputsHash := uc.notesGen.ComputeInputsHash(run, input.Options)

	// Update the run with notes
	if err := run.GenerateNotes(notes, inputsHash, input.Actor.ID); err != nil {
		return nil, fmt.Errorf("failed to update run with notes: %w", err)
	}

	// Save the run
	if err := uc.repo.Save(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to save run: %w", err)
	}

	return &GenerateNotesOutput{
		RunID:      run.ID(),
		Notes:      notes,
		InputsHash: inputsHash,
	}, nil
}

// loadRun loads a run by ID or the latest run.
func (uc *GenerateNotesUseCase) loadRun(ctx context.Context, repoRoot string, runID domain.RunID) (*domain.ReleaseRun, error) {
	if runID != "" {
		// Load specific run
		if fileRepo, ok := uc.repo.(interface {
			LoadFromRepo(context.Context, string, domain.RunID) (*domain.ReleaseRun, error)
		}); ok {
			return fileRepo.LoadFromRepo(ctx, repoRoot, runID)
		}
		return uc.repo.Load(ctx, runID)
	}

	// Load latest
	return uc.repo.LoadLatest(ctx, repoRoot)
}

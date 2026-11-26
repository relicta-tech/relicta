// Package app provides application services (use cases) for release governance.
package app

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/relicta-tech/relicta/v5/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v5/internal/domain/release/ports"
)

// GenerateNotesInput contains the input for generating release notes.
type GenerateNotesInput struct {
	RepoRoot  string
	RunID     domain.RunID // If empty, uses latest
	Options   ports.NotesOptions
	DraftText string
	Actor     ports.ActorInfo
	Force     bool // Force regeneration even if HEAD changed
}

// GenerateNotesOutput contains the output from generating release notes.
type GenerateNotesOutput struct {
	RunID       domain.RunID
	VersionNext string
	Notes       *domain.ReleaseNotes
	InputsHash  string
}

// GenerateNotesUseCase handles the generate notes use case.
type GenerateNotesUseCase struct {
	repo          ports.ReleaseRunRepository
	repoInspector ports.RepoInspector
	notesGen      ports.NotesGenerator
	stateMachine  *domain.StateMachineService
	lockManager   ports.LockManager
}

// NewGenerateNotesUseCase creates a new GenerateNotesUseCase.
func NewGenerateNotesUseCase(
	repo ports.ReleaseRunRepository,
	repoInspector ports.RepoInspector,
	notesGen ports.NotesGenerator,
	stateMachine *domain.StateMachineService,
	lockManagers ...ports.LockManager,
) *GenerateNotesUseCase {
	uc := &GenerateNotesUseCase{
		repo:          repo,
		repoInspector: repoInspector,
		notesGen:      notesGen,
		stateMachine:  stateMachine,
	}
	if len(lockManagers) > 0 {
		uc.lockManager = lockManagers[0]
	}
	return uc
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
	runID := run.ID()
	originalState := run.State()
	originalPlanHash := run.PlanHash()
	originalHeadSHA := run.HeadSHA()
	originalVersionNext := run.VersionNext()
	originalNotesText := ""
	if existing := run.Notes(); existing != nil {
		originalNotesText = existing.Text
	}

	var notes *domain.ReleaseNotes
	var inputsHash string
	if strings.TrimSpace(input.DraftText) != "" {
		// MCP callers can submit prose authored in the active agent session. Keep
		// the normal aggregate transition and persistence path, while marking its
		// origin clearly and binding the stored hash to the exact draft text.
		notes = &domain.ReleaseNotes{
			Text:           input.DraftText,
			AudiencePreset: input.Options.AudiencePreset,
			TonePreset:     input.Options.TonePreset,
			Provider:       "agent",
			GeneratedAt:    time.Now(),
		}
		baseHash := fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s",
			run.ID(), run.PlanHash(), run.HeadSHA(), run.VersionCurrent(), run.VersionNext(),
			input.Options.AudiencePreset, input.Options.TonePreset)
		if uc.notesGen != nil {
			baseHash = uc.notesGen.ComputeInputsHash(run, input.Options)
		}
		inputsHash = fmt.Sprintf("%x", sha256.Sum256([]byte(baseHash+"\x00"+notes.Text)))
	} else {
		if uc.notesGen == nil {
			return nil, fmt.Errorf("notes generator is not configured and no agent draft was provided")
		}
		var err error
		notes, err = uc.notesGen.Generate(ctx, run, input.Options)
		if err != nil {
			return nil, fmt.Errorf("failed to generate notes: %w", err)
		}
		inputsHash = uc.notesGen.ComputeInputsHash(run, input.Options)
	}

	// Generation may call a remote AI provider. Do that without holding the
	// release lock, then serialize only the state check and persistence.
	if uc.lockManager != nil {
		release, err := uc.lockManager.Acquire(ctx, input.RepoRoot, runID)
		if err != nil {
			return nil, fmt.Errorf("failed to acquire release lock: %w", err)
		}
		defer release()

		fresh, err := uc.loadRun(ctx, input.RepoRoot, runID)
		if err != nil {
			return nil, err
		}
		freshNotesText := ""
		if existing := fresh.Notes(); existing != nil {
			freshNotesText = existing.Text
		}
		if fresh.State() != originalState || fresh.PlanHash() != originalPlanHash ||
			fresh.HeadSHA() != originalHeadSHA || fresh.VersionNext() != originalVersionNext ||
			freshNotesText != originalNotesText {
			return nil, fmt.Errorf("release changed while notes were being prepared; reload the release and try again")
		}
		if !input.Force {
			currentHead, err := uc.repoInspector.HeadSHA(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get current HEAD: %w", err)
			}
			if err := fresh.ValidateHeadMatch(currentHead); err != nil {
				return nil, fmt.Errorf("%w (use --force to override)", err)
			}
		}
		run = fresh
	}

	// Generate the first draft from Versioned, or replace an existing draft while
	// keeping NotesReady so the caller can approve the exact updated text.
	if run.State() == domain.StateNotesReady {
		if err := run.UpdateNotesWithHash(notes, inputsHash, input.Actor.ID); err != nil {
			return nil, fmt.Errorf("failed to update run with notes: %w", err)
		}
	} else if err := run.GenerateNotes(notes, inputsHash, input.Actor.ID); err != nil {
		return nil, fmt.Errorf("failed to update run with notes: %w", err)
	}

	// Save the run
	if err := uc.repo.Save(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to save run: %w", err)
	}

	return &GenerateNotesOutput{
		RunID:       run.ID(),
		VersionNext: run.VersionNext().String(),
		Notes:       notes,
		InputsHash:  inputsHash,
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

// Package app provides application services (use cases) for release governance.
package app

import (
	"context"
	"fmt"

	"github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

// BumpVersionInput contains the input for bumping the version.
type BumpVersionInput struct {
	RepoRoot string
	RunID    domain.RunID // If empty, uses latest
	Actor    ports.ActorInfo
	Force    bool // Force bump even if HEAD changed

	// Optional: if not provided, uses the version proposal from planning
	OverrideVersion *version.SemanticVersion
	OverrideTagName string
}

// BumpVersionOutput contains the output from bumping the version.
type BumpVersionOutput struct {
	RunID       domain.RunID
	VersionNext string
	TagName     string
	BumpKind    domain.BumpKind
}

// BumpVersionUseCase handles the bump version use case.
type BumpVersionUseCase struct {
	repo          ports.ReleaseRunRepository
	repoInspector ports.RepoInspector
	lockManager   ports.LockManager
	versionWriter ports.VersionWriter
	stateMachine  *domain.StateMachineService
}

// NewBumpVersionUseCase creates a new BumpVersionUseCase.
func NewBumpVersionUseCase(
	repo ports.ReleaseRunRepository,
	repoInspector ports.RepoInspector,
	lockManager ports.LockManager,
	versionWriter ports.VersionWriter,
	stateMachine *domain.StateMachineService,
) *BumpVersionUseCase {
	return &BumpVersionUseCase{
		repo:          repo,
		repoInspector: repoInspector,
		lockManager:   lockManager,
		versionWriter: versionWriter,
		stateMachine:  stateMachine,
	}
}

// Execute bumps the version for a release run.
func (uc *BumpVersionUseCase) Execute(ctx context.Context, input BumpVersionInput) (*BumpVersionOutput, error) {
	// Load the run
	run, err := uc.loadRun(ctx, input.RepoRoot, input.RunID)
	if err != nil {
		return nil, err
	}

	// Check run state
	if run.State() != domain.StatePlanned {
		return nil, fmt.Errorf("cannot bump from state %s (must be planned)", run.State())
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

	// Determine the version to use
	versionNext := run.VersionNext()
	tagName := run.TagName()

	if input.OverrideVersion != nil {
		versionNext = *input.OverrideVersion
	}
	if input.OverrideTagName != "" {
		tagName = input.OverrideTagName
	}

	// If tag name not set, derive from version
	if tagName == "" {
		tagName = "v" + versionNext.String()
	}

	// Set the version on the run
	if err := run.SetVersion(versionNext, tagName); err != nil {
		return nil, fmt.Errorf("failed to set version: %w", err)
	}

	// Write version files if writer is configured
	if uc.versionWriter != nil {
		if err := uc.versionWriter.WriteVersion(ctx, versionNext); err != nil {
			return nil, fmt.Errorf("failed to write version files: %w", err)
		}
	}

	// Transition to versioned state
	if err := run.Bump(input.Actor.ID); err != nil {
		return nil, fmt.Errorf("failed to bump version: %w", err)
	}

	// Save the run
	if err := uc.repo.Save(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to save run: %w", err)
	}

	// Set as latest
	if err := uc.repo.SetLatest(ctx, input.RepoRoot, run.ID()); err != nil {
		return nil, fmt.Errorf("failed to set latest pointer: %w", err)
	}

	return &BumpVersionOutput{
		RunID:       run.ID(),
		VersionNext: versionNext.String(),
		TagName:     tagName,
		BumpKind:    run.BumpKind(),
	}, nil
}

// loadRun loads a run by ID or the latest run.
func (uc *BumpVersionUseCase) loadRun(ctx context.Context, repoRoot string, runID domain.RunID) (*domain.ReleaseRun, error) {
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

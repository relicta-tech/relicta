// Package app provides application services (use cases) for release governance.
package app

import (
	"context"
	"fmt"

	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

// PlanReleaseInput contains the input for planning a release.
type PlanReleaseInput struct {
	RepoRoot       string
	RepoID         string
	BaseRef        string // Base reference (tag or commit) - if empty, auto-detect
	ConfigHash     string // Hash of the config snapshot
	PluginPlanHash string // Hash of the plugin configuration
	Actor          ports.ActorInfo
	Force          bool // Force planning even if there's an active run

	// Optional pre-computed data from commit analysis
	// If provided, these bypass the basic commit resolution and enable full release planning
	ChangeSet      *changes.ChangeSet       // Pre-computed changeset from analysis
	CurrentVersion *version.SemanticVersion // Current version
	NextVersion    *version.SemanticVersion // Proposed next version
	BumpKind       *domain.BumpKind         // Determined bump type (major/minor/patch)
	Confidence     float64                  // Version calculation confidence (0.0-1.0)

	// Tag-push mode: when HEAD is already tagged, skip directly to versioned state
	// This enables notes/approve/publish without running bump
	TagPushMode bool   // If true, transition directly to versioned state
	TagName     string // The existing tag name (required if TagPushMode is true)
}

// PlanReleaseOutput contains the output from planning a release.
type PlanReleaseOutput struct {
	RunID          domain.RunID
	HeadSHA        domain.CommitSHA
	Commits        []domain.CommitSHA
	PlanHash       string
	CurrentVersion version.SemanticVersion
	VersionNext    version.SemanticVersion
	BumpKind       domain.BumpKind
	RiskScore      float64
	ChangeSet      *changes.ChangeSet
}

// PlanReleaseUseCase handles the plan release use case.
type PlanReleaseUseCase struct {
	repo          ports.ReleaseRunRepository
	repoInspector ports.RepoInspector
	stateMachine  *domain.StateMachineService
}

// NewPlanReleaseUseCase creates a new PlanReleaseUseCase.
func NewPlanReleaseUseCase(
	repo ports.ReleaseRunRepository,
	repoInspector ports.RepoInspector,
	stateMachine *domain.StateMachineService,
) *PlanReleaseUseCase {
	return &PlanReleaseUseCase{
		repo:          repo,
		repoInspector: repoInspector,
		stateMachine:  stateMachine,
	}
}

// Execute plans a new release.
func (uc *PlanReleaseUseCase) Execute(ctx context.Context, input PlanReleaseInput) (*PlanReleaseOutput, error) {
	// Validate Actor ID for audit trail
	if input.Actor.ID == "" {
		return nil, ErrActorIDRequired
	}

	// Validate tag-push mode requirements
	if input.TagPushMode && input.NextVersion == nil {
		return nil, ErrTagPushMissingVersion
	}

	// Check for existing active run
	if !input.Force {
		activeRuns, err := uc.repo.FindActive(ctx, input.RepoRoot)
		if err == nil && len(activeRuns) > 0 {
			return nil, fmt.Errorf("active release run exists: %s (use --clean to clear, or 'relicta cancel' to abort)", activeRuns[0].ID())
		}
	}

	// Get current HEAD SHA
	headSHA, err := uc.repoInspector.HeadSHA(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD SHA: %w", err)
	}

	// Get base ref if not provided
	baseRef := input.BaseRef
	if baseRef == "" {
		tag, err := uc.repoInspector.GetLatestVersionTag(ctx, "v")
		if err != nil {
			// No previous version tag - use initial commit or empty
			baseRef = ""
		} else {
			baseRef = tag
		}
	}

	// Resolve commits between base and head
	commits, err := uc.repoInspector.ResolveCommits(ctx, baseRef, headSHA)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve commits: %w", err)
	}

	// Get repo ID if not provided
	repoID := input.RepoID
	if repoID == "" {
		url, err := uc.repoInspector.GetRemoteURL(ctx)
		if err == nil {
			repoID = url
		} else {
			repoID = input.RepoRoot // Fallback to path
		}
	}

	// Create the release run aggregate
	run := domain.NewReleaseRun(
		repoID,
		input.RepoRoot,
		baseRef,
		headSHA,
		commits,
		input.ConfigHash,
		input.PluginPlanHash,
	)

	// Set actor
	run.SetActor(input.Actor.Type, input.Actor.ID)

	// Set pre-computed ChangeSet if provided
	if input.ChangeSet != nil {
		run.SetChangeSet(input.ChangeSet)
	}

	// Set version proposal if provided
	if input.CurrentVersion != nil && input.NextVersion != nil && input.BumpKind != nil {
		if err := run.SetVersionProposal(*input.CurrentVersion, *input.NextVersion, *input.BumpKind, input.Confidence); err != nil {
			return nil, fmt.Errorf("failed to set version proposal: %w", err)
		}
	}

	// Transition to Planned state
	if err := run.Plan(input.Actor.ID); err != nil {
		return nil, fmt.Errorf("failed to transition to planned state: %w", err)
	}

	// Handle tag-push mode: transition directly to versioned state
	// This enables notes/approve/publish workflow without running bump
	if input.TagPushMode && input.NextVersion != nil {
		tagName := input.TagName
		if tagName == "" {
			tagName = "v" + input.NextVersion.String()
		}

		// Set the version on the run
		if err := run.SetVersion(*input.NextVersion, tagName); err != nil {
			return nil, fmt.Errorf("tag-push mode: failed to set version: %w", err)
		}

		// Transition to versioned state (skipping the need for bump command)
		if err := run.Bump(input.Actor.ID); err != nil {
			return nil, fmt.Errorf("tag-push mode: failed to transition to versioned state: %w", err)
		}

		// Record tag-push mode for audit trail
		run.RecordTagPushMode(tagName, input.Actor.ID)
	}

	// Save the run
	if err := uc.repo.Save(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to save run: %w", err)
	}

	// Set as latest
	if err := uc.repo.SetLatest(ctx, input.RepoRoot, run.ID()); err != nil {
		return nil, fmt.Errorf("failed to set latest run: %w", err)
	}

	// Export state machine JSON
	if uc.stateMachine != nil {
		if machineJSON, err := uc.stateMachine.ExportMachineJSON(); err == nil {
			// Best effort - don't fail if export fails
			if fileRepo, ok := uc.repo.(interface {
				SaveMachineJSON(string, domain.RunID, []byte) error
			}); ok {
				_ = fileRepo.SaveMachineJSON(input.RepoRoot, run.ID(), machineJSON)
			}
		}
	}

	return &PlanReleaseOutput{
		RunID:          run.ID(),
		HeadSHA:        run.HeadSHA(),
		Commits:        run.Commits(),
		PlanHash:       run.PlanHash(),
		CurrentVersion: run.VersionCurrent(),
		VersionNext:    run.VersionNext(),
		BumpKind:       run.BumpKind(),
		RiskScore:      run.RiskScore(),
		ChangeSet:      input.ChangeSet,
	}, nil
}

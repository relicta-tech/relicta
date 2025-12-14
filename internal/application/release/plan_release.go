// Package release provides application use cases for release management.
package release

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/felixgeelhaar/release-pilot/internal/domain/changes"
	"github.com/felixgeelhaar/release-pilot/internal/domain/release"
	"github.com/felixgeelhaar/release-pilot/internal/domain/sourcecontrol"
	"github.com/felixgeelhaar/release-pilot/internal/domain/version"
)

// PlanReleaseInput represents the input for the PlanRelease use case.
type PlanReleaseInput struct {
	RepositoryPath string
	Branch         string
	FromRef        string
	ToRef          string
	DryRun         bool
	TagPrefix      string
}

// Validate validates the PlanReleaseInput.
func (i *PlanReleaseInput) Validate() error {
	// Repository path validation
	if i.RepositoryPath != "" {
		// Check for path traversal attempts
		cleanPath := filepath.Clean(i.RepositoryPath)
		if strings.Contains(cleanPath, "..") {
			return fmt.Errorf("repository path contains invalid traversal: %s", i.RepositoryPath)
		}
	}

	// Branch name validation (git ref name restrictions)
	if i.Branch != "" {
		if strings.ContainsAny(i.Branch, "~^:?*[\\ ") {
			return fmt.Errorf("invalid branch name: %s", i.Branch)
		}
		if strings.HasPrefix(i.Branch, "/") || strings.HasSuffix(i.Branch, "/") {
			return fmt.Errorf("branch name cannot start or end with /: %s", i.Branch)
		}
		if strings.Contains(i.Branch, "..") {
			return fmt.Errorf("branch name cannot contain '..': %s", i.Branch)
		}
	}

	// Tag prefix validation
	if i.TagPrefix != "" {
		if len(i.TagPrefix) > 32 {
			return fmt.Errorf("tag prefix too long (max 32 characters): %s", i.TagPrefix)
		}
		if strings.ContainsAny(i.TagPrefix, "~^:?*[\\ ") {
			return fmt.Errorf("tag prefix contains invalid characters: %s", i.TagPrefix)
		}
	}

	// Git ref validation
	// Allow ~ and ^ for git revision navigation (e.g., HEAD~1, main^)
	// Reject other special characters that could cause issues
	invalidRefChars := ":?*[\\ "
	if i.FromRef != "" {
		if strings.ContainsAny(i.FromRef, invalidRefChars) {
			return fmt.Errorf("invalid from reference: %s", i.FromRef)
		}
	}

	if i.ToRef != "" {
		if strings.ContainsAny(i.ToRef, invalidRefChars) {
			return fmt.Errorf("invalid to reference: %s", i.ToRef)
		}
	}

	return nil
}

// PlanReleaseOutput represents the output of the PlanRelease use case.
type PlanReleaseOutput struct {
	ReleaseID      release.ReleaseID
	CurrentVersion version.SemanticVersion
	NextVersion    version.SemanticVersion
	ReleaseType    changes.ReleaseType
	ChangeSet      *changes.ChangeSet
	RepositoryName string
	Branch         string
}

// PlanReleaseUseCase implements the plan release use case.
type PlanReleaseUseCase struct {
	releaseRepo    release.Repository
	unitOfWork     release.UnitOfWork
	gitRepo        sourcecontrol.GitRepository
	versionCalc    version.VersionCalculator
	eventPublisher release.EventPublisher
	logger         *slog.Logger
}

// NewPlanReleaseUseCase creates a new PlanReleaseUseCase.
func NewPlanReleaseUseCase(
	releaseRepo release.Repository,
	gitRepo sourcecontrol.GitRepository,
	versionCalc version.VersionCalculator,
	eventPublisher release.EventPublisher,
) *PlanReleaseUseCase {
	return &PlanReleaseUseCase{
		releaseRepo:    releaseRepo,
		gitRepo:        gitRepo,
		versionCalc:    versionCalc,
		eventPublisher: eventPublisher,
		logger:         slog.Default().With("usecase", "plan_release"),
	}
}

// NewPlanReleaseUseCaseWithUoW creates a new PlanReleaseUseCase with UnitOfWork support.
func NewPlanReleaseUseCaseWithUoW(
	unitOfWork release.UnitOfWork,
	gitRepo sourcecontrol.GitRepository,
	versionCalc version.VersionCalculator,
	eventPublisher release.EventPublisher,
) *PlanReleaseUseCase {
	return &PlanReleaseUseCase{
		unitOfWork:     unitOfWork,
		gitRepo:        gitRepo,
		versionCalc:    versionCalc,
		eventPublisher: eventPublisher,
		logger:         slog.Default().With("usecase", "plan_release"),
	}
}

// Execute executes the plan release use case.
func (uc *PlanReleaseUseCase) Execute(ctx context.Context, input PlanReleaseInput) (*PlanReleaseOutput, error) {
	// Validate input
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	// Get repository info
	repoInfo, err := uc.gitRepo.GetInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository info: %w", err)
	}

	// Check for dirty working tree
	if repoInfo.IsDirty && !input.DryRun {
		return nil, sourcecontrol.ErrWorkingTreeDirty
	}

	// Determine current version from tags
	tagPrefix := input.TagPrefix
	if tagPrefix == "" {
		tagPrefix = "v"
	}

	versionDiscovery := sourcecontrol.NewVersionDiscovery(tagPrefix)
	currentVersion, err := versionDiscovery.DiscoverCurrentVersion(ctx, uc.gitRepo)
	if err != nil {
		// If no version found, start with initial
		currentVersion = version.Initial
	}

	// Determine the from reference
	fromRef := input.FromRef
	if fromRef == "" {
		// Use latest version tag
		latestTag, tagErr := uc.gitRepo.GetLatestVersionTag(ctx, tagPrefix)
		if tagErr == nil && latestTag != nil {
			fromRef = latestTag.Name()
		}
	}

	// Get commits since last version
	var commits []*sourcecontrol.Commit
	if fromRef != "" {
		toRef := input.ToRef
		if toRef == "" {
			toRef = "HEAD"
		}
		commits, err = uc.gitRepo.GetCommitsBetween(ctx, fromRef, toRef)
	} else {
		// No previous version, get all commits
		commits, err = uc.gitRepo.GetCommitsSince(ctx, "")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get commits: %w", err)
	}

	if len(commits) == 0 {
		return nil, changes.ErrNoCommitsFound
	}

	// Parse commits as conventional commits and build changeset
	changeSetID := changes.ChangeSetID(fmt.Sprintf("cs-%d", time.Now().UnixNano()))
	changeSet := changes.NewChangeSet(changeSetID, fromRef, input.ToRef)

	for _, commit := range commits {
		conventionalCommit := changes.ParseConventionalCommit(
			string(commit.Hash()),
			commit.Message(),
			changes.WithAuthor(commit.Author().Name, commit.Author().Email),
			changes.WithDate(commit.Date()),
		)
		if conventionalCommit != nil {
			changeSet.AddCommit(conventionalCommit)
		}
	}

	if changeSet.IsEmpty() {
		return nil, changes.ErrEmptyChangeSet
	}

	// Determine release type and next version
	releaseType := changeSet.ReleaseType()
	nextVersion := uc.versionCalc.CalculateNextVersion(currentVersion, releaseType.ToBumpType())

	// Create release aggregate
	releaseID := release.ReleaseID(fmt.Sprintf("rel-%d", time.Now().UnixNano()))
	branch := input.Branch
	if branch == "" {
		branch = repoInfo.CurrentBranch
	}

	rel := release.NewRelease(releaseID, branch, input.RepositoryPath)
	rel.SetRepositoryName(repoInfo.Name)

	// Set release plan using constructor for proper aggregate references
	plan := release.NewReleasePlan(
		currentVersion,
		nextVersion,
		releaseType,
		changeSet,
		input.DryRun,
	)

	if err := rel.SetPlan(plan); err != nil {
		return nil, fmt.Errorf("failed to set release plan: %w", err)
	}

	// Save release using UnitOfWork if available
	if !input.DryRun {
		if err := uc.saveRelease(ctx, rel); err != nil {
			return nil, err
		}
	}

	return &PlanReleaseOutput{
		ReleaseID:      releaseID,
		CurrentVersion: currentVersion,
		NextVersion:    nextVersion,
		ReleaseType:    releaseType,
		ChangeSet:      plan.GetChangeSet(),
		RepositoryName: repoInfo.Name,
		Branch:         branch,
	}, nil
}

// saveRelease saves the release using UnitOfWork if available, otherwise uses repository directly.
func (uc *PlanReleaseUseCase) saveRelease(ctx context.Context, rel *release.Release) error {
	// Use UnitOfWork if available
	if uc.unitOfWork != nil {
		return uc.saveReleaseWithUoW(ctx, rel)
	}

	// Legacy path without UnitOfWork
	if err := uc.releaseRepo.Save(ctx, rel); err != nil {
		return fmt.Errorf("failed to save release: %w", err)
	}

	// Publish domain events
	if uc.eventPublisher != nil {
		if err := uc.eventPublisher.Publish(ctx, rel.DomainEvents()...); err != nil {
			uc.logger.Warn("failed to publish domain events",
				"error", err,
				"release_id", rel.ID())
		}
		rel.ClearDomainEvents()
	}

	return nil
}

// saveReleaseWithUoW saves the release with transactional boundaries.
func (uc *PlanReleaseUseCase) saveReleaseWithUoW(ctx context.Context, rel *release.Release) error {
	// Begin transaction
	uow, err := uc.unitOfWork.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = uow.Rollback()
	}()

	// Get repository from UnitOfWork
	repo := uow.ReleaseRepository()

	// Save release - events are collected by UoW
	if err := repo.Save(ctx, rel); err != nil {
		return fmt.Errorf("failed to save release: %w", err)
	}

	// Commit transaction - events are published on commit
	if err := uow.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

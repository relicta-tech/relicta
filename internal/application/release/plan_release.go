// Package release provides application use cases for release management.
package release

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/relicta-tech/relicta/internal/analysis"
	analysisfactory "github.com/relicta-tech/relicta/internal/analysis/factory"
	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

// PlanReleaseInput represents the input for the PlanRelease use case.
type PlanReleaseInput struct {
	RepositoryPath string
	Branch         string
	FromRef        string
	ToRef          string
	DryRun         bool
	TagPrefix      string

	// AnalysisConfig overrides smart commit analysis defaults.
	AnalysisConfig *analysis.AnalyzerConfig
	// CommitClassifications overrides analysis results keyed by commit hash.
	CommitClassifications map[sourcecontrol.CommitHash]*analysis.CommitClassification
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
	ReleaseID      release.RunID
	CurrentVersion version.SemanticVersion
	NextVersion    version.SemanticVersion
	ReleaseType    changes.ReleaseType
	ChangeSet      *changes.ChangeSet
	RepositoryName string
	Branch         string

	// Analysis contains optional smart commit analysis results.
	Analysis *analysis.AnalysisResult
}

// PlanReleaseUseCase implements the plan release use case.
type PlanReleaseUseCase struct {
	releaseRepo       release.Repository
	unitOfWorkFactory release.UnitOfWorkFactory
	gitRepo           sourcecontrol.GitRepository
	versionCalc       version.VersionCalculator
	eventPublisher    release.EventPublisher
	analysisFactory   *analysisfactory.Factory
	logger            *slog.Logger
}

// NewPlanReleaseUseCase creates a new PlanReleaseUseCase.
func NewPlanReleaseUseCase(
	releaseRepo release.Repository,
	gitRepo sourcecontrol.GitRepository,
	versionCalc version.VersionCalculator,
	eventPublisher release.EventPublisher,
	analysisFactory *analysisfactory.Factory,
) *PlanReleaseUseCase {
	return &PlanReleaseUseCase{
		releaseRepo:     releaseRepo,
		gitRepo:         gitRepo,
		versionCalc:     versionCalc,
		eventPublisher:  eventPublisher,
		analysisFactory: analysisFactory,
		logger:          slog.Default().With("usecase", "plan_release"),
	}
}

// NewPlanReleaseUseCaseWithUoW creates a new PlanReleaseUseCase with UnitOfWork support.
func NewPlanReleaseUseCaseWithUoW(
	unitOfWorkFactory release.UnitOfWorkFactory,
	gitRepo sourcecontrol.GitRepository,
	versionCalc version.VersionCalculator,
	eventPublisher release.EventPublisher,
	analysisFactory *analysisfactory.Factory,
) *PlanReleaseUseCase {
	return &PlanReleaseUseCase{
		unitOfWorkFactory: unitOfWorkFactory,
		gitRepo:           gitRepo,
		versionCalc:       versionCalc,
		eventPublisher:    eventPublisher,
		analysisFactory:   analysisFactory,
		logger:            slog.Default().With("usecase", "plan_release"),
	}
}

// Execute executes the plan release use case.
func (uc *PlanReleaseUseCase) Execute(ctx context.Context, input PlanReleaseInput) (*PlanReleaseOutput, error) {
	// Validate input
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	repoInfo, currentVersion, fromRef, commits, err := uc.collectCommits(ctx, input)
	if err != nil {
		return nil, err
	}

	// Check for dirty working tree
	if repoInfo.IsDirty && !input.DryRun {
		return nil, sourcecontrol.ErrWorkingTreeDirty
	}

	// Parse commits as conventional commits and build changeset
	changeSetID := changes.ChangeSetID(fmt.Sprintf("cs-%d", time.Now().UnixNano()))
	changeSet := changes.NewChangeSet(changeSetID, fromRef, input.ToRef)

	analysisResult, classifications, err := uc.prepareCommitClassifications(ctx, commits, input)
	if err != nil {
		return nil, err
	}

	minConfidence := analysis.DefaultConfig().MinConfidence
	if input.AnalysisConfig != nil {
		minConfidence = input.AnalysisConfig.MinConfidence
	}

	for _, commit := range commits {
		conventionalCommit := changes.ParseConventionalCommit(
			string(commit.Hash()),
			commit.Message(),
			changes.WithAuthor(commit.Author().Name, commit.Author().Email),
			changes.WithDate(commit.Date()),
			changes.WithRawMessage(commit.Message()),
		)
		if conventionalCommit != nil {
			changeSet.AddCommit(conventionalCommit)
			continue
		}

		classification := classifications[commit.Hash()]
		if classification == nil || classification.ShouldSkip {
			continue
		}

		useClassification := classification
		if classification.Method != analysis.MethodManual && classification.Confidence < minConfidence {
			lowConfidence := *classification
			lowConfidence.Type = changes.CommitType("")
			useClassification = &lowConfidence
		}

		inferred := classificationToCommit(commit, useClassification)
		changeSet.AddCommit(inferred)
	}

	if changeSet.IsEmpty() {
		return nil, changes.ErrEmptyChangeSet
	}

	// Determine release type and next version
	releaseType := changeSet.ReleaseType()
	nextVersion := uc.versionCalc.CalculateNextVersion(currentVersion, releaseType.ToBumpType())

	// Create release aggregate
	releaseID := release.RunID(fmt.Sprintf("rel-%d", time.Now().UnixNano()))
	branch := input.Branch
	if branch == "" {
		branch = repoInfo.CurrentBranch
	}

	rel := release.NewRelease(releaseID, branch, input.RepositoryPath)

	// Set release plan using constructor for proper aggregate references
	plan := release.NewReleasePlan(
		currentVersion,
		nextVersion,
		releaseType,
		changeSet,
		input.DryRun,
	)

	if err := release.SetPlan(rel, plan); err != nil {
		return nil, fmt.Errorf("failed to set release plan: %w", err)
	}

	// Always save release for workflow state tracking.
	// DryRun affects external actions (tags, pushes), not internal state.
	if err := uc.saveRelease(ctx, rel); err != nil {
		return nil, err
	}

	return &PlanReleaseOutput{
		ReleaseID:      releaseID,
		CurrentVersion: currentVersion,
		NextVersion:    nextVersion,
		ReleaseType:    releaseType,
		ChangeSet:      plan.GetChangeSet(),
		RepositoryName: repoInfo.Name,
		Branch:         branch,
		Analysis:       analysisResult,
	}, nil
}

// AnalyzeCommits runs smart commit analysis without creating a release plan.
func (uc *PlanReleaseUseCase) AnalyzeCommits(ctx context.Context, input PlanReleaseInput) (*analysis.AnalysisResult, []analysis.CommitInfo, error) {
	if err := input.Validate(); err != nil {
		return nil, nil, fmt.Errorf("invalid input: %w", err)
	}

	_, _, _, commits, err := uc.collectCommits(ctx, input)
	if err != nil {
		return nil, nil, err
	}

	cfg := analysis.DefaultConfig()
	if input.AnalysisConfig != nil {
		cfg = *input.AnalysisConfig
	}

	if uc.analysisFactory == nil {
		return nil, nil, fmt.Errorf("analysis is not configured")
	}

	commitInfos, err := uc.buildCommitInfos(ctx, commits, cfg)
	if err != nil {
		return nil, nil, err
	}

	analyzer := uc.analysisFactory.NewAnalyzer(cfg)
	result, err := analyzer.AnalyzeAll(ctx, commitInfos)
	if err != nil {
		return nil, nil, err
	}

	return result, commitInfos, nil
}

func (uc *PlanReleaseUseCase) collectCommits(ctx context.Context, input PlanReleaseInput) (*sourcecontrol.RepositoryInfo, version.SemanticVersion, string, []*sourcecontrol.Commit, error) {
	repoInfo, err := uc.gitRepo.GetInfo(ctx)
	if err != nil {
		return nil, version.Zero, "", nil, fmt.Errorf("failed to get repository info: %w", err)
	}

	tagPrefix := input.TagPrefix
	if tagPrefix == "" {
		tagPrefix = "v"
	}

	versionDiscovery := sourcecontrol.NewVersionDiscovery(tagPrefix)
	currentVersion, err := versionDiscovery.DiscoverCurrentVersion(ctx, uc.gitRepo)
	if err != nil {
		currentVersion = version.Initial
	}

	fromRef := input.FromRef
	if fromRef == "" {
		latestTag, tagErr := uc.gitRepo.GetLatestVersionTag(ctx, tagPrefix)
		if tagErr == nil && latestTag != nil {
			fromRef = latestTag.Name()
		}
	}

	var commits []*sourcecontrol.Commit
	if fromRef != "" {
		toRef := input.ToRef
		if toRef == "" {
			toRef = "HEAD"
		}
		commits, err = uc.gitRepo.GetCommitsBetween(ctx, fromRef, toRef)
	} else {
		commits, err = uc.gitRepo.GetCommitsSince(ctx, "")
	}
	if err != nil {
		return nil, version.Zero, "", nil, fmt.Errorf("failed to get commits: %w", err)
	}

	if len(commits) == 0 {
		return nil, version.Zero, "", nil, changes.ErrNoCommitsFound
	}

	return repoInfo, currentVersion, fromRef, commits, nil
}

func (uc *PlanReleaseUseCase) prepareCommitClassifications(ctx context.Context, commits []*sourcecontrol.Commit, input PlanReleaseInput) (*analysis.AnalysisResult, map[sourcecontrol.CommitHash]*analysis.CommitClassification, error) {
	if input.CommitClassifications != nil {
		return nil, input.CommitClassifications, nil
	}

	if uc.analysisFactory == nil {
		return nil, map[sourcecontrol.CommitHash]*analysis.CommitClassification{}, nil
	}

	cfg := analysis.DefaultConfig()
	if input.AnalysisConfig != nil {
		cfg = *input.AnalysisConfig
	}

	commitInfos, err := uc.buildCommitInfos(ctx, commits, cfg)
	if err != nil {
		return nil, nil, err
	}

	analyzer := uc.analysisFactory.NewAnalyzer(cfg)
	result, err := analyzer.AnalyzeAll(ctx, commitInfos)
	if err != nil {
		return nil, nil, err
	}

	return result, result.Classifications, nil
}

func (uc *PlanReleaseUseCase) buildCommitInfos(ctx context.Context, commits []*sourcecontrol.Commit, cfg analysis.AnalyzerConfig) ([]analysis.CommitInfo, error) {
	infos := make([]analysis.CommitInfo, 0, len(commits))

	aiAvailable := cfg.EnableAI && uc.analysisFactory != nil && uc.analysisFactory.AIAvailable()

	for _, commit := range commits {
		info := analysis.CommitInfo{
			Hash:        commit.Hash(),
			Message:     commit.Message(),
			Subject:     commit.Subject(),
			IsMerge:     commit.IsMergeCommit(),
			ParentCount: len(commit.Parents()),
		}

		stats, err := uc.gitRepo.GetCommitDiffStats(ctx, commit.Hash())
		if err != nil {
			uc.logger.Debug("commit diff stats unavailable", "error", err, "commit", commit.Hash().Short())
		} else if stats != nil {
			info.Stats = analysis.DiffStats{
				Additions:    stats.Additions,
				Deletions:    stats.Deletions,
				FilesChanged: stats.FilesChanged,
			}
			info.Files = make([]string, 0, len(stats.Files))
			for _, file := range stats.Files {
				if shouldSkipPath(file.Path, cfg) {
					continue
				}
				info.Files = append(info.Files, file.Path)
			}

			if cfg.EnableAST {
				parentRef := firstParent(commit)
				for _, file := range stats.Files {
					if !shouldAnalyzeAST(file.Path, cfg) {
						continue
					}
					var before []byte
					if parentRef != "" {
						before, _ = uc.gitRepo.GetFileAtRef(ctx, parentRef, file.Path)
					}
					after, _ := uc.gitRepo.GetFileAtRef(ctx, commit.Hash().String(), file.Path)
					info.FileDiffs = append(info.FileDiffs, analysis.FileDiff{
						Path:   file.Path,
						Before: before,
						After:  after,
					})
				}
			}
		}

		if aiAvailable {
			patch, err := uc.gitRepo.GetCommitPatch(ctx, commit.Hash())
			if err == nil {
				info.Diff = patch
			}
		}

		infos = append(infos, info)
	}

	return infos, nil
}

func classificationToCommit(commit *sourcecontrol.Commit, classification *analysis.CommitClassification) *changes.ConventionalCommit {
	commitType := classification.Type
	if commitType == "" {
		commitType = changes.CommitType("")
	}

	opts := []changes.ConventionalCommitOption{
		changes.WithAuthor(commit.Author().Name, commit.Author().Email),
		changes.WithDate(commit.Date()),
		changes.WithRawMessage(commit.Message()),
	}

	if classification.Scope != "" {
		opts = append(opts, changes.WithScope(classification.Scope))
	}

	if classification.IsBreaking {
		reason := classification.BreakingReason
		if reason == "" {
			reason = "breaking change detected"
		}
		opts = append(opts, changes.WithBreaking(reason))
	}

	return changes.NewConventionalCommit(
		string(commit.Hash()),
		commitType,
		commit.Subject(),
		opts...,
	)
}

func shouldAnalyzeAST(path string, cfg analysis.AnalyzerConfig) bool {
	if !cfg.EnableAST {
		return false
	}
	if shouldSkipPath(path, cfg) {
		return false
	}
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, "_test.go") {
		return false
	}
	if strings.HasSuffix(lower, ".go") {
		return languageAllowed("go", cfg.Languages)
	}
	return false
}

func languageAllowed(lang string, allowed []string) bool {
	if lang == "" || len(allowed) == 0 {
		return false
	}
	for _, entry := range allowed {
		if strings.EqualFold(entry, lang) {
			return true
		}
	}
	return false
}

func shouldSkipPath(path string, cfg analysis.AnalyzerConfig) bool {
	if len(cfg.SkipPaths) == 0 {
		return false
	}
	normalized := filepath.ToSlash(path)
	for _, pattern := range cfg.SkipPaths {
		if matchGlob(normalized, pattern) {
			return true
		}
	}
	return false
}

func matchGlob(file, pattern string) bool {
	file = filepath.ToSlash(file)
	pattern = filepath.ToSlash(pattern)

	if strings.Contains(pattern, "**") {
		return matchDoubleGlob(file, pattern)
	}

	matched, err := filepath.Match(pattern, file)
	if err == nil && matched {
		return true
	}

	base := filepath.Base(file)
	matched, err = filepath.Match(pattern, base)
	return err == nil && matched
}

func matchDoubleGlob(file, pattern string) bool {
	parts := strings.Split(pattern, "**")
	if len(parts) != 2 {
		return false
	}

	prefix := parts[0]
	suffix := strings.TrimPrefix(parts[1], "/")

	if prefix != "" && !strings.HasPrefix(file, prefix) {
		return false
	}

	if suffix == "" {
		return true
	}

	remaining := strings.TrimPrefix(file, prefix)
	pathParts := strings.Split(remaining, "/")
	for i := range pathParts {
		testPath := strings.Join(pathParts[i:], "/")
		if matched, _ := filepath.Match(suffix, testPath); matched {
			return true
		}
		if i == len(pathParts)-1 {
			if matched, _ := filepath.Match(suffix, pathParts[i]); matched {
				return true
			}
		}
	}

	return false
}

func firstParent(commit *sourcecontrol.Commit) string {
	parents := commit.Parents()
	if len(parents) == 0 {
		return ""
	}
	return parents[0].String()
}

// saveRelease saves the release using UnitOfWork if available, otherwise uses repository directly.
func (uc *PlanReleaseUseCase) saveRelease(ctx context.Context, rel *release.ReleaseRun) error {
	// Use UnitOfWork if available
	if uc.unitOfWorkFactory != nil {
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
func (uc *PlanReleaseUseCase) saveReleaseWithUoW(ctx context.Context, rel *release.ReleaseRun) error {
	// Begin transaction via factory
	uow, err := uc.unitOfWorkFactory.Begin(ctx)
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

// Package monorepo provides application services for multi-package/monorepo versioning.
package monorepo

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/relicta-tech/relicta/v4/internal/analysis"
	analysisfactory "github.com/relicta-tech/relicta/v4/internal/analysis/factory"
	"github.com/relicta-tech/relicta/v4/internal/domain/changes"
	"github.com/relicta-tech/relicta/v4/internal/domain/monorepo"
	"github.com/relicta-tech/relicta/v4/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
	"github.com/relicta-tech/relicta/v4/internal/domain/workspace"
)

// PackageAnalysisResult contains analysis results for a single package.
type PackageAnalysisResult struct {
	// PackagePath is the path to the package.
	PackagePath string
	// PackageName is the display name.
	PackageName string
	// PackageType is the type of package.
	PackageType monorepo.PackageType
	// Commits are the commits affecting this package.
	Commits []*sourcecontrol.Commit
	// Classifications are the commit classifications.
	Classifications map[sourcecontrol.CommitHash]*analysis.CommitClassification
	// ChangeSet contains the categorized changes.
	ChangeSet *changes.ChangeSet
	// ReleaseType is the calculated release type.
	ReleaseType changes.ReleaseType
	// BumpType is the version bump required.
	BumpType monorepo.BumpType
	// CurrentVersion is the current version of the package.
	CurrentVersion version.SemanticVersion
	// NextVersion is the calculated next version.
	NextVersion version.SemanticVersion
	// ChangedFiles lists files changed in this package.
	ChangedFiles []string
	// RiskScore is the CGP risk assessment (0.0-1.0).
	RiskScore float64
}

// MonorepoAnalyzeInput contains input parameters for monorepo analysis.
type MonorepoAnalyzeInput struct {
	// RepositoryPath is the path to the repository.
	RepositoryPath string
	// FromRef is the base reference (e.g., previous release tag).
	FromRef string
	// ToRef is the target reference (e.g., HEAD).
	ToRef string
	// Workspace is the detected workspace configuration.
	Workspace *workspace.Workspace
	// Strategy is the versioning strategy to use.
	Strategy monorepo.MonorepoStrategy
	// TagPrefix is the prefix for version tags.
	TagPrefix string
	// AnalysisConfig overrides analyzer defaults.
	AnalysisConfig *analysis.AnalyzerConfig
}

// MonorepoAnalyzeOutput contains the results of monorepo analysis.
type MonorepoAnalyzeOutput struct {
	// Packages contains analysis results per package.
	Packages []*PackageAnalysisResult
	// TotalCommits is the total number of commits analyzed.
	TotalCommits int
	// AffectedPackages is the count of packages with changes.
	AffectedPackages int
	// RepositoryName is the repository identifier.
	RepositoryName string
}

// MonorepoAnalyzer orchestrates per-package analysis for monorepos.
type MonorepoAnalyzer struct {
	gitRepo         sourcecontrol.GitRepository
	versionCalc     version.VersionCalculator
	analysisFactory *analysisfactory.Factory
	versionReader   VersionReader
	logger          *slog.Logger
}

// VersionReader reads package versions from manifest files.
type VersionReader interface {
	ReadVersion(ctx context.Context, pkgPath string, pkgType monorepo.PackageType) (version.SemanticVersion, error)
}

// NewMonorepoAnalyzer creates a new monorepo analyzer.
func NewMonorepoAnalyzer(
	gitRepo sourcecontrol.GitRepository,
	versionCalc version.VersionCalculator,
	analysisFactory *analysisfactory.Factory,
	versionReader VersionReader,
) *MonorepoAnalyzer {
	return &MonorepoAnalyzer{
		gitRepo:         gitRepo,
		versionCalc:     versionCalc,
		analysisFactory: analysisFactory,
		versionReader:   versionReader,
		logger:          slog.Default().With("service", "monorepo_analyzer"),
	}
}

// Analyze performs per-package analysis for a monorepo.
func (a *MonorepoAnalyzer) Analyze(ctx context.Context, input MonorepoAnalyzeInput) (*MonorepoAnalyzeOutput, error) {
	if input.Workspace == nil {
		return nil, fmt.Errorf("workspace is required for monorepo analysis")
	}

	a.logger.Info("starting monorepo analysis",
		"repository", input.RepositoryPath,
		"from", input.FromRef,
		"to", input.ToRef,
		"packages", len(input.Workspace.Packages),
	)

	// Collect all commits
	toRef := input.ToRef
	if toRef == "" {
		toRef = "HEAD"
	}

	commits, err := a.gitRepo.GetCommitsBetween(ctx, input.FromRef, toRef)
	if err != nil {
		return nil, fmt.Errorf("failed to collect commits: %w", err)
	}

	a.logger.Info("collected commits", "count", len(commits))

	// Get batch diff stats for efficiency
	commitHashes := make([]sourcecontrol.CommitHash, len(commits))
	for i, c := range commits {
		commitHashes[i] = c.Hash()
	}

	batchDiffStats, err := a.gitRepo.GetBatchCommitDiffStats(ctx, commitHashes)
	if err != nil {
		a.logger.Warn("failed to get batch diff stats, falling back to individual calls",
			"error", err,
		)
		batchDiffStats = nil
	}

	// Map commits to packages
	packageCommits := a.mapCommitsToPackages(ctx, commits, batchDiffStats, input.Workspace)

	// Analyze each package
	var results []*PackageAnalysisResult
	affectedCount := 0

	for _, pkg := range input.Workspace.Packages {
		pkgCommits := packageCommits[pkg.Path]
		if len(pkgCommits) == 0 && input.Strategy == monorepo.StrategyIndependent {
			// Skip packages with no changes in independent mode
			continue
		}

		result, err := a.analyzePackage(ctx, pkg, pkgCommits, batchDiffStats, input)
		if err != nil {
			a.logger.Warn("failed to analyze package",
				"package", pkg.Path,
				"error", err,
			)
			continue
		}

		if result.BumpType != monorepo.BumpTypeNone {
			affectedCount++
		}
		results = append(results, result)
	}

	// For lockstep strategy, synchronize versions
	if input.Strategy == monorepo.StrategyLockstep {
		a.synchronizeLockstepVersions(results)
	}

	return &MonorepoAnalyzeOutput{
		Packages:         results,
		TotalCommits:     len(commits),
		AffectedPackages: affectedCount,
		RepositoryName:   filepath.Base(input.RepositoryPath),
	}, nil
}

// mapCommitsToPackages maps each commit to the packages it affects.
func (a *MonorepoAnalyzer) mapCommitsToPackages(
	ctx context.Context,
	commits []*sourcecontrol.Commit,
	batchDiffStats map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats,
	ws *workspace.Workspace,
) map[string][]*sourcecontrol.Commit {
	packageCommits := make(map[string][]*sourcecontrol.Commit)

	// Initialize empty slices for all packages
	for _, pkg := range ws.Packages {
		packageCommits[pkg.Path] = make([]*sourcecontrol.Commit, 0)
	}

	for _, commit := range commits {
		// Get diff stats for this commit
		var diffStats *sourcecontrol.DiffStats
		if batchDiffStats != nil {
			diffStats = batchDiffStats[commit.Hash()]
		} else {
			var err error
			diffStats, err = a.gitRepo.GetCommitDiffStats(ctx, commit.Hash())
			if err != nil {
				a.logger.Warn("failed to get diff stats",
					"commit", commit.Hash(),
					"error", err,
				)
				continue
			}
		}

		// Check which packages are affected
		for _, pkg := range ws.Packages {
			if a.commitAffectsPackage(diffStats, pkg.Path, ws.RootPath) {
				packageCommits[pkg.Path] = append(packageCommits[pkg.Path], commit)
			}
		}
	}

	return packageCommits
}

// commitAffectsPackage checks if a commit affects a specific package.
func (a *MonorepoAnalyzer) commitAffectsPackage(
	diffStats *sourcecontrol.DiffStats,
	pkgPath string,
	rootPath string,
) bool {
	if diffStats == nil || len(diffStats.Files) == 0 {
		return false
	}

	// Normalize package path
	normalizedPkgPath := strings.TrimPrefix(pkgPath, rootPath)
	normalizedPkgPath = strings.TrimPrefix(normalizedPkgPath, "/")

	for _, file := range diffStats.Files {
		// Check if the file is within the package directory
		if strings.HasPrefix(file.Path, normalizedPkgPath+"/") || file.Path == normalizedPkgPath {
			return true
		}
		// Also check for exact match (for files directly in package root)
		if filepath.Dir(file.Path) == normalizedPkgPath {
			return true
		}
	}

	return false
}

// analyzePackage analyzes a single package.
func (a *MonorepoAnalyzer) analyzePackage(
	ctx context.Context,
	pkg *workspace.Package,
	commits []*sourcecontrol.Commit,
	batchDiffStats map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats,
	input MonorepoAnalyzeInput,
) (*PackageAnalysisResult, error) {
	result := &PackageAnalysisResult{
		PackagePath:     pkg.Path,
		PackageName:     pkg.Name,
		PackageType:     monorepo.PackageTypeFromString(string(input.Workspace.PackageManager)),
		Commits:         commits,
		Classifications: make(map[sourcecontrol.CommitHash]*analysis.CommitClassification),
		ChangedFiles:    make([]string, 0),
	}

	// Read current version
	if a.versionReader != nil {
		ver, err := a.versionReader.ReadVersion(ctx, pkg.Path, result.PackageType)
		if err == nil {
			result.CurrentVersion = ver
		}
	}

	// If no commits, return with no changes
	if len(commits) == 0 {
		result.BumpType = monorepo.BumpTypeNone
		result.ReleaseType = changes.ReleaseTypeNone
		return result, nil
	}

	// Build changeset for this package
	changeSetID := changes.ChangeSetID(fmt.Sprintf("cs-%s-%d", pkg.Path, len(commits)))
	changeSet := changes.NewChangeSet(changeSetID, input.FromRef, input.ToRef)

	// Get the analyzer configuration
	var analyzerCfg analysis.AnalyzerConfig
	if input.AnalysisConfig != nil {
		analyzerCfg = *input.AnalysisConfig
	} else {
		analyzerCfg = analysis.DefaultConfig()
	}

	// Create analyzer if factory is available
	var commitAnalyzer *analysis.CommitAnalyzer
	if a.analysisFactory != nil {
		commitAnalyzer = a.analysisFactory.NewAnalyzer(analyzerCfg)
	}

	// Classify commits
	hasBreaking := false
	hasFeature := false
	hasFix := false

	for _, commit := range commits {
		var classification *analysis.CommitClassification

		// Use the commit analyzer if available
		if commitAnalyzer != nil {
			// Convert sourcecontrol.Commit to analysis.CommitInfo
			commitInfo := analysis.CommitInfo{
				Hash:    commit.Hash(),
				Message: commit.Message(),
				Subject: commit.Subject(),
			}
			analysisResult, err := commitAnalyzer.Analyze(ctx, commitInfo)
			if err == nil && analysisResult != nil {
				classification = analysisResult
			}
		}

		if classification == nil {
			// Fallback to basic conventional commit parsing
			classification = a.parseConventionalCommit(commit)
		}

		result.Classifications[commit.Hash()] = classification

		// Build conventional commit for changeset
		opts := []changes.ConventionalCommitOption{
			changes.WithScope(classification.Scope),
		}
		if classification.IsBreaking {
			opts = append(opts, changes.WithBreaking(changes.BreakingMessageFromMessage(commit.Message())))
		}
		convCommit := changes.NewConventionalCommit(
			string(commit.Hash()),
			classification.Type,
			commit.Subject(),
			opts...,
		)
		changeSet.AddCommit(convCommit)

		// Track bump requirements
		if classification.IsBreaking {
			hasBreaking = true
		}
		switch classification.Type {
		case changes.CommitTypeFeat:
			hasFeature = true
		case changes.CommitTypeFix:
			hasFix = true
		}

		// Collect changed files for this package
		var diffStats *sourcecontrol.DiffStats
		if batchDiffStats != nil {
			diffStats = batchDiffStats[commit.Hash()]
		}
		if diffStats != nil {
			for _, file := range diffStats.Files {
				if strings.HasPrefix(file.Path, pkg.Path) {
					result.ChangedFiles = append(result.ChangedFiles, file.Path)
				}
			}
		}
	}

	result.ChangeSet = changeSet

	// Determine release type and bump
	result.ReleaseType = changeSet.ReleaseType()
	result.BumpType = a.determineBumpType(hasBreaking, hasFeature, hasFix)

	// Calculate next version
	if result.BumpType != monorepo.BumpTypeNone {
		result.NextVersion = monorepo.CalculateNextVersion(result.CurrentVersion, result.BumpType)
	}

	// Calculate risk score
	result.RiskScore = a.calculateRiskScore(result)

	return result, nil
}

// parseConventionalCommit performs basic conventional commit parsing.
func (a *MonorepoAnalyzer) parseConventionalCommit(commit *sourcecontrol.Commit) *analysis.CommitClassification {
	classification := &analysis.CommitClassification{
		CommitHash: commit.Hash(),
		Type:       changes.CommitTypeChore,
		Confidence: 0.5,
		Method:     analysis.MethodConventional,
	}

	msg := commit.Message()
	if strings.HasPrefix(msg, "feat") {
		classification.Type = changes.CommitTypeFeat
		classification.Confidence = 0.8
	} else if strings.HasPrefix(msg, "fix") {
		classification.Type = changes.CommitTypeFix
		classification.Confidence = 0.8
	} else if strings.HasPrefix(msg, "docs") {
		classification.Type = changes.CommitTypeDocs
		classification.Confidence = 0.8
	} else if strings.HasPrefix(msg, "test") {
		classification.Type = changes.CommitTypeTest
		classification.Confidence = 0.8
	} else if strings.HasPrefix(msg, "refactor") {
		classification.Type = changes.CommitTypeRefactor
		classification.Confidence = 0.8
	}

	if strings.Contains(msg, "BREAKING CHANGE") || strings.Contains(msg, "!:") {
		classification.IsBreaking = true
	}

	return classification
}

// determineBumpType determines the version bump type based on changes.
func (a *MonorepoAnalyzer) determineBumpType(hasBreaking, hasFeature, hasFix bool) monorepo.BumpType {
	if hasBreaking {
		return monorepo.BumpTypeMajor
	}
	if hasFeature {
		return monorepo.BumpTypeMinor
	}
	if hasFix {
		return monorepo.BumpTypePatch
	}
	return monorepo.BumpTypeNone
}

// calculateRiskScore calculates a risk score for the package changes.
func (a *MonorepoAnalyzer) calculateRiskScore(result *PackageAnalysisResult) float64 {
	score := 0.0

	// Factor 1: Bump type severity
	switch result.BumpType {
	case monorepo.BumpTypeMajor:
		score += 0.4
	case monorepo.BumpTypeMinor:
		score += 0.2
	case monorepo.BumpTypePatch:
		score += 0.1
	}

	// Factor 2: Number of commits
	commitFactor := float64(len(result.Commits)) / 20.0
	if commitFactor > 0.3 {
		commitFactor = 0.3
	}
	score += commitFactor

	// Factor 3: Number of changed files
	fileFactor := float64(len(result.ChangedFiles)) / 50.0
	if fileFactor > 0.3 {
		fileFactor = 0.3
	}
	score += fileFactor

	// Clamp to [0, 1]
	if score > 1.0 {
		score = 1.0
	}
	return score
}

// synchronizeLockstepVersions synchronizes versions across all packages in lockstep mode.
func (a *MonorepoAnalyzer) synchronizeLockstepVersions(results []*PackageAnalysisResult) {
	if len(results) == 0 {
		return
	}

	// Find the highest bump type across all packages
	highestBump := monorepo.BumpTypeNone
	for _, result := range results {
		if compareBumpTypes(result.BumpType, highestBump) > 0 {
			highestBump = result.BumpType
		}
	}

	// Find the highest current version
	var highestVersion version.SemanticVersion
	for _, result := range results {
		if compareVersions(result.CurrentVersion, highestVersion) > 0 {
			highestVersion = result.CurrentVersion
		}
	}

	// Apply the same bump to all packages
	nextVersion := monorepo.CalculateNextVersion(highestVersion, highestBump)
	for _, result := range results {
		result.BumpType = highestBump
		result.NextVersion = nextVersion
	}
}

// compareBumpTypes compares two bump types (-1, 0, 1).
func compareBumpTypes(a, b monorepo.BumpType) int {
	priority := map[monorepo.BumpType]int{
		monorepo.BumpTypeNone:  0,
		monorepo.BumpTypePatch: 1,
		monorepo.BumpTypeMinor: 2,
		monorepo.BumpTypeMajor: 3,
	}
	if priority[a] < priority[b] {
		return -1
	}
	if priority[a] > priority[b] {
		return 1
	}
	return 0
}

// compareVersions compares two semantic versions (-1, 0, 1).
func compareVersions(a, b version.SemanticVersion) int {
	if a.Major() != b.Major() {
		if a.Major() < b.Major() {
			return -1
		}
		return 1
	}
	if a.Minor() != b.Minor() {
		if a.Minor() < b.Minor() {
			return -1
		}
		return 1
	}
	if a.Patch() != b.Patch() {
		if a.Patch() < b.Patch() {
			return -1
		}
		return 1
	}
	return 0
}

// CreateMonorepoRelease creates a MonorepoRelease from analysis results.
func (a *MonorepoAnalyzer) CreateMonorepoRelease(
	ctx context.Context,
	input MonorepoAnalyzeInput,
	output *MonorepoAnalyzeOutput,
) (*monorepo.MonorepoRelease, error) {
	release := monorepo.NewMonorepoRelease(
		output.RepositoryName,
		input.FromRef,
		input.ToRef,
		input.Strategy,
	)

	for _, result := range output.Packages {
		pkg := monorepo.NewPackageRelease(
			result.PackagePath,
			result.PackageName,
			result.PackageType,
			result.CurrentVersion,
		)

		if result.BumpType != monorepo.BumpTypeNone {
			if err := pkg.SetVersion(result.NextVersion, result.BumpType); err != nil {
				return nil, fmt.Errorf("failed to set version for %s: %w", result.PackagePath, err)
			}
		} else {
			if err := pkg.Skip(); err != nil {
				return nil, fmt.Errorf("failed to skip %s: %w", result.PackagePath, err)
			}
		}

		// Set metadata
		pkg.CommitCount = len(result.Commits)
		pkg.RiskScore = result.RiskScore
		for _, file := range result.ChangedFiles {
			pkg.AddChangedFile(file)
		}

		if err := release.AddPackage(pkg); err != nil {
			return nil, fmt.Errorf("failed to add package %s: %w", result.PackagePath, err)
		}
	}

	if err := release.Plan(); err != nil {
		return nil, fmt.Errorf("failed to plan release: %w", err)
	}

	return release, nil
}

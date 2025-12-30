// Package release provides release analysis services.
// This package handles commit collection, classification, and version calculation
// without managing state - state is handled by the DDD domain layer.
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
	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

// AnalyzerConfig holds configuration for the release analyzer.
type AnalyzerConfig struct {
	MinConfidence float64
	EnableAI      bool
	Languages     []string
}

// DefaultConfig returns the default analyzer configuration.
func DefaultConfig() AnalyzerConfig {
	return AnalyzerConfig{
		MinConfidence: 0.7,
		EnableAI:      true,
		Languages:     []string{"go", "typescript", "python"},
	}
}

// AnalyzeInput contains input parameters for release analysis.
type AnalyzeInput struct {
	RepositoryPath string
	Branch         string
	FromRef        string
	ToRef          string
	TagPrefix      string

	// AnalysisConfig overrides analyzer defaults.
	AnalysisConfig *analysis.AnalyzerConfig

	// CommitClassifications allows manual overrides keyed by commit hash.
	CommitClassifications map[sourcecontrol.CommitHash]*analysis.CommitClassification
}

// Validate validates the input parameters.
func (i *AnalyzeInput) Validate() error {
	if i.RepositoryPath != "" {
		cleanPath := filepath.Clean(i.RepositoryPath)
		if strings.Contains(cleanPath, "..") {
			return fmt.Errorf("repository path contains invalid traversal: %s", i.RepositoryPath)
		}
	}

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

	if i.TagPrefix != "" {
		if len(i.TagPrefix) > 32 {
			return fmt.Errorf("tag prefix too long (max 32 characters): %s", i.TagPrefix)
		}
		if strings.ContainsAny(i.TagPrefix, "~^:?*[\\ ") {
			return fmt.Errorf("tag prefix contains invalid characters: %s", i.TagPrefix)
		}
	}

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

// AnalyzeOutput contains the results of release analysis.
type AnalyzeOutput struct {
	CurrentVersion version.SemanticVersion
	NextVersion    version.SemanticVersion
	ReleaseType    changes.ReleaseType
	ChangeSet      *changes.ChangeSet
	RepositoryName string
	Branch         string
	Commits        []*sourcecontrol.Commit

	// Analysis contains detailed classification results.
	Analysis *analysis.AnalysisResult
}

// Analyzer orchestrates commit collection, classification, and version calculation.
type Analyzer struct {
	gitRepo         sourcecontrol.GitRepository
	versionCalc     version.VersionCalculator
	analysisFactory *analysisfactory.Factory
	logger          *slog.Logger
}

// NewAnalyzer creates a new release analyzer.
func NewAnalyzer(
	gitRepo sourcecontrol.GitRepository,
	versionCalc version.VersionCalculator,
	analysisFactory *analysisfactory.Factory,
) *Analyzer {
	return &Analyzer{
		gitRepo:         gitRepo,
		versionCalc:     versionCalc,
		analysisFactory: analysisFactory,
		logger:          slog.Default().With("service", "release_analyzer"),
	}
}

// Analyze performs release analysis: collects commits, classifies them, and calculates version.
func (a *Analyzer) Analyze(ctx context.Context, input AnalyzeInput) (*AnalyzeOutput, error) {
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	repoInfo, currentVersion, fromRef, commits, err := a.collectCommits(ctx, input)
	if err != nil {
		return nil, err
	}

	// Build changeset from commits
	changeSetID := changes.ChangeSetID(fmt.Sprintf("cs-%d", time.Now().UnixNano()))
	changeSet := changes.NewChangeSet(changeSetID, fromRef, input.ToRef)

	analysisResult, classifications, err := a.prepareCommitClassifications(ctx, commits, input)
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

	// Calculate version
	releaseType := changeSet.ReleaseType()
	nextVersion := a.versionCalc.CalculateNextVersion(currentVersion, releaseType.ToBumpType())

	branch := input.Branch
	if branch == "" {
		branch = repoInfo.CurrentBranch
	}

	// Construct repository name from owner/name
	repoName := repoInfo.Name
	if repoInfo.Owner != "" {
		repoName = repoInfo.Owner + "/" + repoInfo.Name
	}

	return &AnalyzeOutput{
		CurrentVersion: currentVersion,
		NextVersion:    nextVersion,
		ReleaseType:    releaseType,
		ChangeSet:      changeSet,
		RepositoryName: repoName,
		Branch:         branch,
		Commits:        commits,
		Analysis:       analysisResult,
	}, nil
}

// AnalyzeCommits runs commit analysis without building a full plan.
// Useful for --analyze and --review modes.
func (a *Analyzer) AnalyzeCommits(ctx context.Context, input AnalyzeInput) (*analysis.AnalysisResult, []analysis.CommitInfo, error) {
	if err := input.Validate(); err != nil {
		return nil, nil, fmt.Errorf("invalid input: %w", err)
	}

	_, _, _, commits, err := a.collectCommits(ctx, input)
	if err != nil {
		return nil, nil, err
	}

	commitInfos := make([]analysis.CommitInfo, 0, len(commits))
	for _, c := range commits {
		// Get file list via diff stats if available
		files := a.getCommitFiles(ctx, c.Hash())

		info := analysis.CommitInfo{
			Hash:    c.Hash(),
			Message: c.Message(),
			Subject: getSubject(c.Message()),
			Files:   files,
		}
		commitInfos = append(commitInfos, info)
	}

	analyzerCfg := analysis.DefaultConfig()
	if input.AnalysisConfig != nil {
		analyzerCfg = *input.AnalysisConfig
	}

	commitAnalyzer := a.analysisFactory.NewAnalyzer(analyzerCfg)
	result, err := commitAnalyzer.AnalyzeAll(ctx, commitInfos)
	if err != nil {
		return nil, nil, fmt.Errorf("commit analysis failed: %w", err)
	}

	return result, commitInfos, nil
}

// getCommitFiles retrieves the list of files changed in a commit.
func (a *Analyzer) getCommitFiles(ctx context.Context, hash sourcecontrol.CommitHash) []string {
	stats, err := a.gitRepo.GetCommitDiffStats(ctx, hash)
	if err != nil {
		return nil
	}

	files := make([]string, 0, len(stats.Files))
	for _, f := range stats.Files {
		files = append(files, f.Path)
	}
	return files
}

func (a *Analyzer) collectCommits(ctx context.Context, input AnalyzeInput) (*sourcecontrol.RepositoryInfo, version.SemanticVersion, string, []*sourcecontrol.Commit, error) {
	repoInfo, err := a.gitRepo.GetInfo(ctx)
	if err != nil {
		return nil, version.SemanticVersion{}, "", nil, fmt.Errorf("failed to get repository info: %w", err)
	}

	tags, err := a.gitRepo.GetTags(ctx)
	if err != nil {
		return nil, version.SemanticVersion{}, "", nil, fmt.Errorf("failed to get tags: %w", err)
	}

	var currentVersion version.SemanticVersion
	fromRef := input.FromRef

	if fromRef == "" {
		versionTags := tags.FilterByPrefix(input.TagPrefix).VersionTags()
		if latestTag := versionTags.Latest(); latestTag != nil {
			fromRef = latestTag.Name()
			if v := latestTag.Version(); v != nil {
				currentVersion = *v
			}
		}
	} else {
		if v, err := version.Parse(strings.TrimPrefix(fromRef, input.TagPrefix)); err == nil {
			currentVersion = v
		}
	}

	toRef := input.ToRef
	if toRef == "" {
		toRef = "HEAD"
	}

	commits, err := a.gitRepo.GetCommitsBetween(ctx, fromRef, toRef)
	if err != nil {
		return nil, version.SemanticVersion{}, "", nil, fmt.Errorf("failed to get commits: %w", err)
	}

	if len(commits) == 0 {
		return nil, version.SemanticVersion{}, "", nil, sourcecontrol.ErrNoCommits
	}

	return repoInfo, currentVersion, fromRef, commits, nil
}

func (a *Analyzer) prepareCommitClassifications(ctx context.Context, commits []*sourcecontrol.Commit, input AnalyzeInput) (*analysis.AnalysisResult, map[sourcecontrol.CommitHash]*analysis.CommitClassification, error) {
	// If manual classifications provided, use them
	if len(input.CommitClassifications) > 0 {
		result := &analysis.AnalysisResult{
			Classifications: make(map[sourcecontrol.CommitHash]*analysis.CommitClassification),
			Stats: analysis.AnalysisStats{
				TotalCommits:    len(commits),
				MethodBreakdown: make(map[analysis.ClassifyMethod]int),
			},
		}
		for hash, class := range input.CommitClassifications {
			result.Classifications[hash] = class
			if class.Method == analysis.MethodManual {
				result.Stats.MethodBreakdown[analysis.MethodManual]++
			}
		}
		return result, input.CommitClassifications, nil
	}

	// Build commit info for analysis
	commitInfos := make([]analysis.CommitInfo, 0, len(commits))
	for _, c := range commits {
		// Get file list via diff stats if available
		files := a.getCommitFiles(ctx, c.Hash())

		info := analysis.CommitInfo{
			Hash:    c.Hash(),
			Message: c.Message(),
			Subject: getSubject(c.Message()),
			Files:   files,
		}
		commitInfos = append(commitInfos, info)
	}

	// Run analysis
	analyzerCfg := analysis.DefaultConfig()
	if input.AnalysisConfig != nil {
		analyzerCfg = *input.AnalysisConfig
	}

	commitAnalyzer := a.analysisFactory.NewAnalyzer(analyzerCfg)
	result, err := commitAnalyzer.AnalyzeAll(ctx, commitInfos)
	if err != nil {
		return nil, nil, fmt.Errorf("commit analysis failed: %w", err)
	}

	return result, result.Classifications, nil
}

func getSubject(message string) string {
	lines := strings.SplitN(message, "\n", 2)
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return message
}

func classificationToCommit(commit *sourcecontrol.Commit, classification *analysis.CommitClassification) *changes.ConventionalCommit {
	commitType := classification.Type
	if commitType == "" {
		commitType = changes.CommitTypeChore
	}

	opts := []changes.ConventionalCommitOption{
		changes.WithScope(classification.Scope),
		changes.WithAuthor(commit.Author().Name, commit.Author().Email),
		changes.WithDate(commit.Date()),
		changes.WithRawMessage(commit.Message()),
	}

	if classification.IsBreaking {
		opts = append(opts, changes.WithBreaking("breaking change"))
	}

	return changes.NewConventionalCommit(
		string(commit.Hash()),
		commitType,
		getSubject(commit.Message()),
		opts...,
	)
}

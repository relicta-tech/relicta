// Package analysis provides intelligent commit classification
// for repositories that don't follow conventional commit standards.
package analysis

import (
	"context"

	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
)

// ClassifyMethod indicates how a commit was classified.
type ClassifyMethod string

const (
	// MethodConventional means the commit already followed conventional commit format.
	MethodConventional ClassifyMethod = "conventional"

	// MethodHeuristic means the commit was classified using keyword/path patterns.
	MethodHeuristic ClassifyMethod = "heuristic"

	// MethodAST means the commit was classified using AST analysis.
	MethodAST ClassifyMethod = "ast"

	// MethodAI means the commit was classified using AI.
	MethodAI ClassifyMethod = "ai"

	// MethodManual means the user manually specified the classification.
	MethodManual ClassifyMethod = "manual"

	// MethodSkipped means the commit should be skipped (merge commits, etc.).
	MethodSkipped ClassifyMethod = "skipped"
)

// String returns the string representation of the method.
func (m ClassifyMethod) String() string {
	return string(m)
}

// ShortString returns an abbreviated version for display.
func (m ClassifyMethod) ShortString() string {
	switch m {
	case MethodConventional:
		return "conv"
	case MethodHeuristic:
		return "heur"
	case MethodAST:
		return "ast"
	case MethodAI:
		return "ai"
	case MethodManual:
		return "man"
	case MethodSkipped:
		return "skip"
	default:
		return "?"
	}
}

// CommitClassification represents the analyzed classification of a commit.
type CommitClassification struct {
	// CommitHash is the SHA of the classified commit.
	CommitHash sourcecontrol.CommitHash

	// Type is the inferred commit type (feat, fix, docs, etc.).
	Type changes.CommitType

	// Scope is the inferred scope (e.g., "auth", "api", "core").
	Scope string

	// Confidence is the classification confidence from 0.0 to 1.0.
	Confidence float64

	// Method indicates how this classification was determined.
	Method ClassifyMethod

	// Reasoning provides a human-readable explanation.
	Reasoning string

	// IsBreaking indicates whether this is a breaking change.
	IsBreaking bool

	// BreakingReason explains why this is a breaking change.
	BreakingReason string

	// ShouldSkip indicates whether this commit should be excluded from release notes.
	ShouldSkip bool

	// SkipReason explains why this commit is skipped.
	SkipReason string
}

// IsHighConfidence returns true if confidence meets the threshold.
func (c *CommitClassification) IsHighConfidence(threshold float64) bool {
	return c.Confidence >= threshold
}

// CommitInfo holds the information needed to classify a commit.
type CommitInfo struct {
	// Hash is the commit SHA.
	Hash sourcecontrol.CommitHash

	// Message is the full commit message.
	Message string

	// Subject is the first line of the commit message.
	Subject string

	// Files is the list of files changed in this commit.
	Files []string

	// FileDiffs contains per-file before/after content (optional).
	FileDiffs []FileDiff

	// Diff contains the actual diff content (optional, for AI analysis).
	Diff string

	// Stats contains diff statistics.
	Stats DiffStats

	// IsMerge indicates if this is a merge commit.
	IsMerge bool

	// ParentCount is the number of parent commits.
	ParentCount int
}

// FileDiff contains before/after content for a file.
type FileDiff struct {
	// Path is the file path.
	Path string
	// Before is the file content before the change.
	Before []byte
	// After is the file content after the change.
	After []byte
}

// DiffStats contains statistics about a commit's changes.
type DiffStats struct {
	// Additions is the number of lines added.
	Additions int

	// Deletions is the number of lines deleted.
	Deletions int

	// FilesChanged is the number of files modified.
	FilesChanged int
}

// AnalysisResult contains the results of analyzing a set of commits.
type AnalysisResult struct {
	// Classifications maps commit SHA to classification.
	Classifications map[sourcecontrol.CommitHash]*CommitClassification

	// Stats provides aggregate statistics about the analysis.
	Stats AnalysisStats
}

// AnalysisStats provides aggregate statistics about commit analysis.
type AnalysisStats struct {
	// TotalCommits is the number of commits analyzed.
	TotalCommits int

	// ConventionalCount is the number of commits that were already conventional.
	ConventionalCount int

	// HeuristicCount is the number classified by heuristics.
	HeuristicCount int

	// ASTCount is the number classified by AST analysis.
	ASTCount int

	// AICount is the number classified by AI.
	AICount int

	// SkippedCount is the number of skipped commits.
	SkippedCount int

	// LowConfidenceCount is the number below the confidence threshold.
	LowConfidenceCount int

	// LowConfidenceCommits lists commits below the confidence threshold.
	LowConfidenceCommits []sourcecontrol.CommitHash

	// AverageConfidence is the average confidence across all classifications.
	AverageConfidence float64

	// MethodBreakdown shows count by classification method.
	MethodBreakdown map[ClassifyMethod]int
}

// Analyzer is the interface for commit classification.
type Analyzer interface {
	// Analyze classifies a single commit.
	Analyze(ctx context.Context, commit CommitInfo) (*CommitClassification, error)

	// AnalyzeAll classifies multiple commits.
	AnalyzeAll(ctx context.Context, commits []CommitInfo) (*AnalysisResult, error)
}

// HeuristicsAnalyzer provides fast, rule-based classification.
type HeuristicsAnalyzer interface {
	// Classify classifies a commit using heuristics.
	Classify(commit CommitInfo) *CommitClassification
}

// ASTAnalyzer provides language-specific semantic analysis.
type ASTAnalyzer interface {
	// Analyze compares before/after code and returns analysis.
	Analyze(ctx context.Context, before, after []byte, path string) (*ASTAnalysis, error)

	// SupportsFile returns true if this analyzer can handle the file.
	SupportsFile(path string) bool
}

// ASTAnalysis contains the results of AST-based analysis.
type ASTAnalysis struct {
	// AddedExports lists new public functions/types.
	AddedExports []string

	// RemovedExports lists removed public functions/types.
	RemovedExports []string

	// ModifiedExports lists changed signatures.
	ModifiedExports []string

	// IsBreaking indicates if this is a breaking change.
	IsBreaking bool

	// BreakingReasons explains why changes are breaking.
	BreakingReasons []string

	// SuggestedType is the inferred commit type.
	SuggestedType changes.CommitType

	// Confidence is the analysis confidence.
	Confidence float64
}

// AIClassifier provides AI-based classification for ambiguous cases.
type AIClassifier interface {
	// Classify uses AI to classify a commit.
	Classify(ctx context.Context, commit CommitInfo) (*CommitClassification, error)
}

// AnalyzerConfig contains configuration for the analyzer.
type AnalyzerConfig struct {
	// MinConfidence is the minimum confidence threshold to accept a classification.
	MinConfidence float64

	// EnableHeuristics enables the heuristics layer.
	EnableHeuristics bool

	// EnableAST enables the AST analysis layer.
	EnableAST bool

	// EnableAI enables the AI classification layer.
	EnableAI bool

	// Languages lists languages to support in AST analysis.
	Languages []string

	// CustomKeywords allows custom keyword patterns.
	CustomKeywords map[changes.CommitType][]string

	// SkipPaths lists paths to always skip.
	SkipPaths []string
}

// DefaultConfig returns the default analyzer configuration.
func DefaultConfig() AnalyzerConfig {
	return AnalyzerConfig{
		MinConfidence:    0.7,
		EnableHeuristics: true,
		EnableAST:        true,
		EnableAI:         true,
		Languages:        []string{"go", "typescript", "python"},
		CustomKeywords:   nil,
		SkipPaths:        []string{"vendor/*", "node_modules/*", "*.generated.go"},
	}
}

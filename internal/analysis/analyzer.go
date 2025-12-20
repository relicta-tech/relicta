// Package analysis provides intelligent commit classification
// for repositories that don't follow conventional commit standards.
package analysis

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
)

// CommitAnalyzer orchestrates heuristic, AST, and AI classification.
type CommitAnalyzer struct {
	cfg          AnalyzerConfig
	heuristics   HeuristicsAnalyzer
	astAnalyzers map[string]ASTAnalyzer
	aiClassifier AIClassifier
	logger       *slog.Logger
}

// AnalyzerOption configures the commit analyzer.
type AnalyzerOption func(*CommitAnalyzer)

// WithHeuristics sets the heuristics analyzer.
func WithHeuristics(h HeuristicsAnalyzer) AnalyzerOption {
	return func(a *CommitAnalyzer) {
		a.heuristics = h
	}
}

// WithASTAnalyzers sets the AST analyzers by language.
func WithASTAnalyzers(analyzers map[string]ASTAnalyzer) AnalyzerOption {
	return func(a *CommitAnalyzer) {
		a.astAnalyzers = analyzers
	}
}

// WithAIClassifier sets the AI classifier.
func WithAIClassifier(ai AIClassifier) AnalyzerOption {
	return func(a *CommitAnalyzer) {
		a.aiClassifier = ai
	}
}

// WithLogger sets the logger.
func WithLogger(logger *slog.Logger) AnalyzerOption {
	return func(a *CommitAnalyzer) {
		a.logger = logger
	}
}

// NewAnalyzer creates a new commit analyzer with the provided config.
func NewAnalyzer(cfg AnalyzerConfig, opts ...AnalyzerOption) *CommitAnalyzer {
	analyzer := &CommitAnalyzer{
		cfg:          cfg,
		astAnalyzers: make(map[string]ASTAnalyzer),
		logger:       slog.Default().With("component", "commit_analyzer"),
	}

	for _, opt := range opts {
		opt(analyzer)
	}

	return analyzer
}

// Analyze classifies a single commit.
func (a *CommitAnalyzer) Analyze(ctx context.Context, commit CommitInfo) (*CommitClassification, error) {
	if classification := a.classifyConventional(commit); classification != nil {
		return classification, nil
	}

	var best *CommitClassification

	if a.cfg.EnableHeuristics && a.heuristics != nil {
		best = a.heuristics.Classify(commit)
		if best != nil {
			if best.Method == MethodSkipped {
				return best, nil
			}
			if best.Confidence >= a.cfg.MinConfidence {
				return best, nil
			}
		}
	}

	if a.cfg.EnableAST {
		if astResult := a.classifyWithAST(ctx, commit); astResult != nil {
			if best == nil || astResult.Confidence > best.Confidence {
				best = astResult
			}
			if astResult.Confidence >= a.cfg.MinConfidence {
				return astResult, nil
			}
		}
	}

	if a.cfg.EnableAI && a.aiClassifier != nil {
		aiResult, err := a.aiClassifier.Classify(ctx, commit)
		if err != nil {
			a.logger.Debug("ai classification failed", "error", err, "commit", commit.Hash.String())
		} else if aiResult != nil {
			if best == nil || aiResult.Confidence > best.Confidence {
				best = aiResult
			}
			if aiResult.Confidence >= a.cfg.MinConfidence || aiResult.Method == MethodSkipped {
				return aiResult, nil
			}
		}
	}

	if best != nil {
		return best, nil
	}

	return &CommitClassification{
		CommitHash: commit.Hash,
		Method:     MethodHeuristic,
		Confidence: 0.0,
		Reasoning:  "unable to classify commit",
	}, nil
}

// AnalyzeAll classifies multiple commits.
func (a *CommitAnalyzer) AnalyzeAll(ctx context.Context, commits []CommitInfo) (*AnalysisResult, error) {
	result := &AnalysisResult{
		Classifications: make(map[sourcecontrol.CommitHash]*CommitClassification, len(commits)),
		Stats: AnalysisStats{
			MethodBreakdown: make(map[ClassifyMethod]int),
		},
	}

	var totalConfidence float64

	for _, commit := range commits {
		classification, err := a.Analyze(ctx, commit)
		if err != nil {
			return nil, err
		}
		result.Classifications[commit.Hash] = classification
		result.Stats.TotalCommits++

		if classification != nil {
			totalConfidence += classification.Confidence
			result.Stats.MethodBreakdown[classification.Method]++

			switch classification.Method {
			case MethodConventional:
				result.Stats.ConventionalCount++
			case MethodHeuristic:
				result.Stats.HeuristicCount++
			case MethodAST:
				result.Stats.ASTCount++
			case MethodAI:
				result.Stats.AICount++
			case MethodSkipped:
				result.Stats.SkippedCount++
			}

			if classification.Method != MethodSkipped && classification.Confidence < a.cfg.MinConfidence {
				result.Stats.LowConfidenceCount++
				result.Stats.LowConfidenceCommits = append(result.Stats.LowConfidenceCommits, commit.Hash)
			}
		}
	}

	if result.Stats.TotalCommits > 0 {
		result.Stats.AverageConfidence = totalConfidence / float64(result.Stats.TotalCommits)
	}

	return result, nil
}

func (a *CommitAnalyzer) classifyConventional(commit CommitInfo) *CommitClassification {
	if commit.Message == "" {
		return nil
	}

	conventional := changes.ParseConventionalCommit(commit.Hash.String(), commit.Message, changes.WithRawMessage(commit.Message))
	if conventional == nil {
		return nil
	}

	return &CommitClassification{
		CommitHash: commit.Hash,
		Type:       conventional.Type(),
		Scope:      conventional.Scope(),
		Confidence: 1.0,
		Method:     MethodConventional,
		Reasoning:  "conventional commit",
		IsBreaking: conventional.IsBreaking(),
	}
}

func (a *CommitAnalyzer) classifyWithAST(ctx context.Context, commit CommitInfo) *CommitClassification {
	if len(commit.FileDiffs) == 0 || len(a.astAnalyzers) == 0 {
		return nil
	}

	analysis := &ASTAnalysis{}
	var found bool

	for _, diff := range commit.FileDiffs {
		lang := detectLanguage(diff.Path)
		if !a.languageEnabled(lang) {
			continue
		}
		analyzer, ok := a.astAnalyzers[lang]
		if !ok || !analyzer.SupportsFile(diff.Path) {
			continue
		}
		fileAnalysis, err := analyzer.Analyze(ctx, diff.Before, diff.After, diff.Path)
		if err != nil || fileAnalysis == nil {
			continue
		}
		mergeASTAnalysis(analysis, fileAnalysis)
		found = true
	}

	if !found {
		return nil
	}

	classification := astToClassification(commit.Hash, analysis)
	classification.Method = MethodAST
	return classification
}

func (a *CommitAnalyzer) languageEnabled(lang string) bool {
	if lang == "" {
		return false
	}
	for _, allowed := range a.cfg.Languages {
		if strings.EqualFold(allowed, lang) {
			return true
		}
	}
	return false
}

func detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".ts", ".tsx", ".js", ".jsx":
		return "typescript"
	case ".py":
		return "python"
	default:
		return ""
	}
}

func mergeASTAnalysis(target, incoming *ASTAnalysis) {
	if incoming == nil {
		return
	}

	target.AddedExports = append(target.AddedExports, incoming.AddedExports...)
	target.RemovedExports = append(target.RemovedExports, incoming.RemovedExports...)
	target.ModifiedExports = append(target.ModifiedExports, incoming.ModifiedExports...)
	target.BreakingReasons = append(target.BreakingReasons, incoming.BreakingReasons...)

	if incoming.IsBreaking {
		target.IsBreaking = true
	}
	if incoming.Confidence > target.Confidence {
		target.Confidence = incoming.Confidence
		target.SuggestedType = incoming.SuggestedType
	}
}

func astToClassification(hash sourcecontrol.CommitHash, analysis *ASTAnalysis) *CommitClassification {
	if analysis == nil {
		return nil
	}

	reason := buildASTReasoning(analysis)
	commitType := analysis.SuggestedType
	if commitType == "" {
		commitType = changes.CommitTypeRefactor
	}

	return &CommitClassification{
		CommitHash:     hash,
		Type:           commitType,
		Confidence:     analysis.Confidence,
		Method:         MethodAST,
		Reasoning:      reason,
		IsBreaking:     analysis.IsBreaking,
		BreakingReason: strings.Join(analysis.BreakingReasons, "; "),
	}
}

func buildASTReasoning(analysis *ASTAnalysis) string {
	var parts []string
	if len(analysis.AddedExports) > 0 {
		parts = append(parts, fmt.Sprintf("added exports: %s", strings.Join(analysis.AddedExports, ", ")))
	}
	if len(analysis.RemovedExports) > 0 {
		parts = append(parts, fmt.Sprintf("removed exports: %s", strings.Join(analysis.RemovedExports, ", ")))
	}
	if len(analysis.ModifiedExports) > 0 {
		parts = append(parts, fmt.Sprintf("modified exports: %s", strings.Join(analysis.ModifiedExports, ", ")))
	}
	if len(parts) == 0 {
		return "no public API changes detected"
	}
	return strings.Join(parts, "; ")
}

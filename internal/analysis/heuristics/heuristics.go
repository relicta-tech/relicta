// Package heuristics provides fast, rule-based commit classification.
package heuristics

import (
	"strings"

	"github.com/relicta-tech/relicta/internal/analysis"
	"github.com/relicta-tech/relicta/internal/domain/changes"
)

// Analyzer provides heuristic-based commit classification.
type Analyzer struct {
	keywordDetector *KeywordDetector
	pathDetector    *PathDetector
	patternDetector *PatternDetector
	customKeywords  map[changes.CommitType][]string
}

// NewAnalyzer creates a new heuristics analyzer.
func NewAnalyzer(customKeywords map[changes.CommitType][]string) *Analyzer {
	return &Analyzer{
		keywordDetector: NewKeywordDetector(),
		pathDetector:    NewPathDetector(),
		patternDetector: NewPatternDetector(),
		customKeywords:  customKeywords,
	}
}

// Classify classifies a commit using heuristics.
func (a *Analyzer) Classify(commit analysis.CommitInfo) *analysis.CommitClassification {
	result := &analysis.CommitClassification{
		CommitHash: commit.Hash,
		Method:     analysis.MethodHeuristic,
	}

	// Check if this commit should be skipped
	if skip, reason := a.patternDetector.ShouldSkip(commit); skip {
		result.ShouldSkip = true
		result.SkipReason = reason
		result.Method = analysis.MethodSkipped
		result.Confidence = 1.0
		return result
	}

	// Try keyword detection first (highest signal from commit message)
	if classification := a.classifyByKeywords(commit); classification != nil {
		return classification
	}

	// Try path-based detection
	if classification := a.classifyByPaths(commit); classification != nil {
		return classification
	}

	// Try diff size heuristics
	if classification := a.classifyByDiffSize(commit); classification != nil {
		return classification
	}

	// Unable to classify with high confidence
	result.Type = ""
	result.Confidence = 0.0
	result.Reasoning = "unable to classify from message, paths, or diff size"
	return result
}

// classifyByKeywords attempts to classify based on commit message keywords.
func (a *Analyzer) classifyByKeywords(commit analysis.CommitInfo) *analysis.CommitClassification {
	subject := strings.ToLower(commit.Subject)

	// Check custom keywords first
	for commitType, keywords := range a.customKeywords {
		for _, kw := range keywords {
			if strings.Contains(subject, strings.ToLower(kw)) {
				return &analysis.CommitClassification{
					CommitHash: commit.Hash,
					Type:       commitType,
					Confidence: 0.85,
					Method:     analysis.MethodHeuristic,
					Reasoning:  "matched custom keyword: " + kw,
				}
			}
		}
	}

	// Use standard keyword detection
	if result := a.keywordDetector.Detect(subject); result != nil {
		return &analysis.CommitClassification{
			CommitHash: commit.Hash,
			Type:       result.Type,
			Scope:      result.Scope,
			Confidence: result.Confidence,
			Method:     analysis.MethodHeuristic,
			Reasoning:  result.Reasoning,
			IsBreaking: result.IsBreaking,
		}
	}

	return nil
}

// classifyByPaths attempts to classify based on file paths.
func (a *Analyzer) classifyByPaths(commit analysis.CommitInfo) *analysis.CommitClassification {
	if len(commit.Files) == 0 {
		return nil
	}

	result := a.pathDetector.Detect(commit.Files)
	if result == nil {
		return nil
	}

	return &analysis.CommitClassification{
		CommitHash: commit.Hash,
		Type:       result.Type,
		Scope:      result.Scope,
		Confidence: result.Confidence,
		Method:     analysis.MethodHeuristic,
		Reasoning:  result.Reasoning,
	}
}

// classifyByDiffSize attempts to classify based on diff statistics.
func (a *Analyzer) classifyByDiffSize(commit analysis.CommitInfo) *analysis.CommitClassification {
	stats := commit.Stats

	// Need meaningful changes
	if stats.Additions == 0 && stats.Deletions == 0 {
		return nil
	}

	result := a.patternDetector.ClassifyByDiffSize(stats)
	if result == nil {
		return nil
	}

	return &analysis.CommitClassification{
		CommitHash: commit.Hash,
		Type:       result.Type,
		Confidence: result.Confidence,
		Method:     analysis.MethodHeuristic,
		Reasoning:  result.Reasoning,
	}
}

// DetectionResult holds the result of a detection attempt.
type DetectionResult struct {
	Type       changes.CommitType
	Scope      string
	Confidence float64
	Reasoning  string
	IsBreaking bool
}

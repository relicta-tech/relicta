// Package factory builds commit analyzers with shared dependencies.
package factory

import (
	"log/slog"

	"github.com/relicta-tech/relicta/internal/analysis"
	"github.com/relicta-tech/relicta/internal/analysis/ast"
	"github.com/relicta-tech/relicta/internal/analysis/heuristics"
	"github.com/relicta-tech/relicta/internal/infrastructure/ai"
)

// Factory builds commit analyzers with shared dependencies.
type Factory struct {
	astAnalyzers map[string]analysis.ASTAnalyzer
	aiService    ai.Service
	logger       *slog.Logger
}

// NewFactory creates a new analyzer factory.
func NewFactory(aiService ai.Service) *Factory {
	return &Factory{
		astAnalyzers: map[string]analysis.ASTAnalyzer{
			"go": ast.NewGoAnalyzer(),
		},
		aiService: aiService,
		logger:    slog.Default().With("component", "analysis_factory"),
	}
}

// AIAvailable reports whether the factory can supply an AI classifier.
func (f *Factory) AIAvailable() bool {
	return f.aiService != nil && f.aiService.IsAvailable()
}

// NewAnalyzer creates a configured analyzer.
func (f *Factory) NewAnalyzer(cfg analysis.AnalyzerConfig) *analysis.CommitAnalyzer {
	var aiClassifier analysis.AIClassifier
	if cfg.EnableAI && f.aiService != nil && f.aiService.IsAvailable() {
		aiClassifier = analysis.NewAIClassifier(f.aiService)
	}

	heuristicsAnalyzer := heuristics.NewAnalyzer(cfg.CustomKeywords)

	return analysis.NewAnalyzer(
		cfg,
		analysis.WithHeuristics(heuristicsAnalyzer),
		analysis.WithASTAnalyzers(f.astAnalyzers),
		analysis.WithAIClassifier(aiClassifier),
		analysis.WithLogger(f.logger),
	)
}

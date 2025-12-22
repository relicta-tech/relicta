package factory

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta/internal/analysis"
	"github.com/relicta-tech/relicta/internal/infrastructure/ai"
	"github.com/relicta-tech/relicta/internal/infrastructure/git"
)

type stubAIService struct {
	ready bool
}

func (s *stubAIService) GenerateChangelog(_ context.Context, _ *git.CategorizedChanges, _ ai.GenerateOptions) (string, error) {
	return "", nil
}

func (s *stubAIService) GenerateReleaseNotes(_ context.Context, _ string, _ ai.GenerateOptions) (string, error) {
	return "", nil
}

func (s *stubAIService) GenerateMarketingBlurb(_ context.Context, _ string, _ ai.GenerateOptions) (string, error) {
	return "", nil
}

func (s *stubAIService) SummarizeChanges(_ context.Context, _ *git.CategorizedChanges, _ ai.GenerateOptions) (string, error) {
	return "", nil
}

func (s *stubAIService) Complete(_ context.Context, _ string, _ string) (string, error) {
	return `{"type":"fix","scope":"","confidence":0.9,"reasoning":"ok","is_breaking":false,"breaking_reason":"","should_skip":false,"skip_reason":""}`, nil
}

func (s *stubAIService) IsAvailable() bool {
	return s.ready
}

func TestFactory_NewAnalyzer_UsesAIWhenAvailable(t *testing.T) {
	service := &stubAIService{ready: true}
	factory := NewFactory(service)

	cfg := analysis.DefaultConfig()
	cfg.EnableAI = true
	analyzer := factory.NewAnalyzer(cfg)
	if analyzer == nil {
		t.Fatal("expected analyzer")
	}
}

func TestFactory_NewAnalyzer_NoAIWhenUnavailable(t *testing.T) {
	service := &stubAIService{ready: false}
	factory := NewFactory(service)

	cfg := analysis.DefaultConfig()
	cfg.EnableAI = true
	analyzer := factory.NewAnalyzer(cfg)
	if analyzer == nil {
		t.Fatal("expected analyzer")
	}
}

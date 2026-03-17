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

func TestFactory_AIAvailable(t *testing.T) {
	tests := []struct {
		name      string
		aiService ai.Service
		want      bool
	}{
		{
			name:      "nil_service",
			aiService: nil,
			want:      false,
		},
		{
			name:      "service_not_ready",
			aiService: &stubAIService{ready: false},
			want:      false,
		},
		{
			name:      "service_ready",
			aiService: &stubAIService{ready: true},
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory(tt.aiService)
			if got := factory.AIAvailable(); got != tt.want {
				t.Errorf("AIAvailable() = %v, want %v", got, tt.want)
			}
		})
	}
}

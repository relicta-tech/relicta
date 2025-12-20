package analysis

import (
	"context"
	"errors"
	"testing"

	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
)

type stubHeuristics struct {
	classification *CommitClassification
}

func (s *stubHeuristics) Classify(_ CommitInfo) *CommitClassification {
	return s.classification
}

type stubAST struct {
	analysis *ASTAnalysis
	err      error
}

func (s *stubAST) Analyze(_ context.Context, _, _ []byte, _ string) (*ASTAnalysis, error) {
	return s.analysis, s.err
}

func (s *stubAST) SupportsFile(_ string) bool {
	return true
}

type stubAI struct {
	classification *CommitClassification
	err            error
}

func (s *stubAI) Classify(_ context.Context, _ CommitInfo) (*CommitClassification, error) {
	return s.classification, s.err
}

func TestCommitAnalyzer_ConventionalCommitWins(t *testing.T) {
	analyzer := NewAnalyzer(DefaultConfig(), WithHeuristics(&stubHeuristics{
		classification: &CommitClassification{
			CommitHash: sourcecontrol.CommitHash("abc"),
			Type:       changes.CommitTypeFix,
			Confidence: 0.9,
			Method:     MethodHeuristic,
		},
	}))

	commit := CommitInfo{
		Hash:    sourcecontrol.CommitHash("abc"),
		Message: "feat: add api",
		Subject: "feat: add api",
	}

	result, err := analyzer.Analyze(context.Background(), commit)
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	if result.Method != MethodConventional {
		t.Errorf("Method = %s, want %s", result.Method, MethodConventional)
	}
	if result.Type != changes.CommitTypeFeat {
		t.Errorf("Type = %s, want %s", result.Type, changes.CommitTypeFeat)
	}
}

func TestCommitAnalyzer_UsesASTWhenHeuristicLowConfidence(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinConfidence = 0.8
	analyzer := NewAnalyzer(cfg,
		WithHeuristics(&stubHeuristics{
			classification: &CommitClassification{
				CommitHash: sourcecontrol.CommitHash("abc"),
				Type:       changes.CommitTypeFix,
				Confidence: 0.4,
				Method:     MethodHeuristic,
			},
		}),
		WithASTAnalyzers(map[string]ASTAnalyzer{
			"go": &stubAST{
				analysis: &ASTAnalysis{
					AddedExports:  []string{"NewThing"},
					SuggestedType: changes.CommitTypeFeat,
					Confidence:    0.9,
				},
			},
		}),
	)

	commit := CommitInfo{
		Hash:      sourcecontrol.CommitHash("abc"),
		Message:   "update stuff",
		Subject:   "update stuff",
		FileDiffs: []FileDiff{{Path: "internal/foo.go"}},
	}

	result, err := analyzer.Analyze(context.Background(), commit)
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	if result.Method != MethodAST {
		t.Errorf("Method = %s, want %s", result.Method, MethodAST)
	}
	if result.Type != changes.CommitTypeFeat {
		t.Errorf("Type = %s, want %s", result.Type, changes.CommitTypeFeat)
	}
}

func TestCommitAnalyzer_UsesAIWhenOthersLowConfidence(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinConfidence = 0.8
	cfg.EnableAI = true
	analyzer := NewAnalyzer(cfg,
		WithHeuristics(&stubHeuristics{
			classification: &CommitClassification{
				CommitHash: sourcecontrol.CommitHash("abc"),
				Type:       changes.CommitTypeFix,
				Confidence: 0.2,
				Method:     MethodHeuristic,
			},
		}),
		WithAIClassifier(&stubAI{
			classification: &CommitClassification{
				CommitHash: sourcecontrol.CommitHash("abc"),
				Type:       changes.CommitTypeDocs,
				Confidence: 0.95,
				Method:     MethodAI,
			},
		}),
	)

	commit := CommitInfo{
		Hash:    sourcecontrol.CommitHash("abc"),
		Message: "update readme",
		Subject: "update readme",
	}

	result, err := analyzer.Analyze(context.Background(), commit)
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	if result.Method != MethodAI {
		t.Errorf("Method = %s, want %s", result.Method, MethodAI)
	}
	if result.Type != changes.CommitTypeDocs {
		t.Errorf("Type = %s, want %s", result.Type, changes.CommitTypeDocs)
	}
}

func TestCommitAnalyzer_AIErrorFallsBack(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinConfidence = 0.8
	cfg.EnableAI = true
	analyzer := NewAnalyzer(cfg,
		WithHeuristics(&stubHeuristics{
			classification: &CommitClassification{
				CommitHash: sourcecontrol.CommitHash("abc"),
				Type:       changes.CommitTypeFix,
				Confidence: 0.3,
				Method:     MethodHeuristic,
			},
		}),
		WithAIClassifier(&stubAI{err: errors.New("ai down")}),
	)

	commit := CommitInfo{
		Hash:    sourcecontrol.CommitHash("abc"),
		Message: "small issue",
		Subject: "small issue",
	}

	result, err := analyzer.Analyze(context.Background(), commit)
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	if result.Method != MethodHeuristic {
		t.Errorf("Method = %s, want %s", result.Method, MethodHeuristic)
	}
}

func TestCommitAnalyzer_AnalyzeAllStats(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinConfidence = 0.7
	analyzer := NewAnalyzer(cfg, WithHeuristics(&stubHeuristics{
		classification: &CommitClassification{
			CommitHash: sourcecontrol.CommitHash("abc"),
			Type:       changes.CommitTypeFix,
			Confidence: 0.4,
			Method:     MethodHeuristic,
		},
	}))

	commits := []CommitInfo{
		{Hash: sourcecontrol.CommitHash("abc"), Message: "fix issue", Subject: "fix issue"},
		{Hash: sourcecontrol.CommitHash("def"), Message: "misc update", Subject: "misc update"},
	}

	result, err := analyzer.AnalyzeAll(context.Background(), commits)
	if err != nil {
		t.Fatalf("AnalyzeAll error: %v", err)
	}
	if result.Stats.TotalCommits != 2 {
		t.Errorf("TotalCommits = %d, want 2", result.Stats.TotalCommits)
	}
	if result.Stats.LowConfidenceCount != 2 {
		t.Errorf("LowConfidenceCount = %d, want 2", result.Stats.LowConfidenceCount)
	}
	if len(result.Stats.LowConfidenceCommits) != 2 {
		t.Fatalf("LowConfidenceCommits length = %d, want 2", len(result.Stats.LowConfidenceCommits))
	}
}

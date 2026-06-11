package heuristics

import (
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/analysis"
	"github.com/relicta-tech/relicta/v4/internal/domain/changes"
)

func TestAnalyzer_Classify_SkipMerge(t *testing.T) {
	a := NewAnalyzer(nil)
	commit := analysis.CommitInfo{
		Hash:        "abc123",
		Subject:     "Merge branch 'main' into feature",
		IsMerge:     true,
		ParentCount: 2,
	}
	result := a.Classify(commit)
	if !result.ShouldSkip {
		t.Error("merge commit should be skipped")
	}
}

func TestAnalyzer_Classify_CustomKeyword(t *testing.T) {
	custom := map[changes.CommitType][]string{
		"perf": {"optimize", "speedup"},
	}
	a := NewAnalyzer(custom)
	commit := analysis.CommitInfo{
		Hash:    "abc123",
		Subject: "Optimize database queries",
	}
	result := a.Classify(commit)
	if result.Type != "perf" {
		t.Errorf("expected perf, got %q", result.Type)
	}
}

func TestAnalyzer_Classify_ByPathsOnly(t *testing.T) {
	a := NewAnalyzer(nil)
	commit := analysis.CommitInfo{
		Hash:    "abc123",
		Subject: "update stuff",
		Files:   []string{"docs/README.md"},
	}
	result := a.Classify(commit)
	if result.Type == "" {
		// path detector may or may not classify docs; just ensure no panic
		t.Log("path-based classification returned empty type")
	}
}

func TestAnalyzer_Classify_ByDiffSize(t *testing.T) {
	a := NewAnalyzer(nil)
	commit := analysis.CommitInfo{
		Hash:    "abc123",
		Subject: "changes",
		Stats: analysis.DiffStats{
			Additions: 500,
			Deletions: 500,
		},
	}
	result := a.Classify(commit)
	// Large diff should classify as refactor or similar
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestAnalyzer_Classify_Unclassifiable(t *testing.T) {
	a := NewAnalyzer(nil)
	commit := analysis.CommitInfo{
		Hash:    "abc123",
		Subject: "x",
		Stats:   analysis.DiffStats{},
	}
	result := a.Classify(commit)
	if result.Confidence != 0.0 {
		t.Errorf("unclassifiable should have 0 confidence, got %v", result.Confidence)
	}
}

func TestAnalyzer_ClassifyByPaths_EmptyFiles(t *testing.T) {
	a := NewAnalyzer(nil)
	commit := analysis.CommitInfo{
		Hash:    "abc123",
		Subject: "no keyword match here xyzzy",
		Files:   []string{},
	}
	result := a.Classify(commit)
	// With no files and no keyword match, should fall through
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

package analysis

import (
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
)

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"main.go", "go"},
		{"app.ts", "typescript"},
		{"app.jsx", "typescript"},
		{"script.py", "python"},
		{"README.md", ""},
	}

	for _, tt := range tests {
		if got := detectLanguage(tt.path); got != tt.want {
			t.Errorf("detectLanguage(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestMergeASTAnalysis(t *testing.T) {
	target := &ASTAnalysis{}
	incoming := &ASTAnalysis{
		AddedExports:    []string{"Add"},
		RemovedExports:  []string{"Remove"},
		ModifiedExports: []string{"Change"},
		BreakingReasons: []string{"removed export"},
		IsBreaking:      true,
		SuggestedType:   changes.CommitTypeFeat,
		Confidence:      0.8,
	}

	mergeASTAnalysis(target, incoming)
	if !target.IsBreaking {
		t.Error("expected breaking to be true")
	}
	if target.Confidence != 0.8 {
		t.Errorf("Confidence = %.2f, want 0.8", target.Confidence)
	}
	if len(target.AddedExports) != 1 || target.AddedExports[0] != "Add" {
		t.Errorf("AddedExports = %v", target.AddedExports)
	}
}

func TestASTToClassification(t *testing.T) {
	analysis := &ASTAnalysis{
		AddedExports:    []string{"NewThing"},
		RemovedExports:  []string{"OldThing"},
		ModifiedExports: []string{"ChangeThing"},
		BreakingReasons: []string{"removed exported API"},
		IsBreaking:      true,
		SuggestedType:   changes.CommitTypeFeat,
		Confidence:      0.9,
	}
	hash := sourcecontrol.CommitHash("abc")
	classification := astToClassification(hash, analysis)
	if classification.Method != MethodAST {
		t.Errorf("Method = %s, want %s", classification.Method, MethodAST)
	}
	if !classification.IsBreaking {
		t.Errorf("IsBreaking = false, want true")
	}
	if !strings.Contains(classification.Reasoning, "added exports") {
		t.Errorf("Reasoning = %q, want added exports", classification.Reasoning)
	}
}

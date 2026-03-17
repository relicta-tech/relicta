package analysis

import (
	"testing"
)

func TestClassifyMethod_String(t *testing.T) {
	tests := []struct {
		method ClassifyMethod
		want   string
	}{
		{MethodConventional, "conventional"},
		{MethodHeuristic, "heuristic"},
		{MethodAST, "ast"},
		{MethodAI, "ai"},
		{MethodManual, "manual"},
		{MethodSkipped, "skipped"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.method.String(); got != tt.want {
				t.Errorf("ClassifyMethod.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClassifyMethod_ShortString(t *testing.T) {
	tests := []struct {
		method ClassifyMethod
		want   string
	}{
		{MethodConventional, "conv"},
		{MethodHeuristic, "heur"},
		{MethodAST, "ast"},
		{MethodAI, "ai"},
		{MethodManual, "man"},
		{MethodSkipped, "skip"},
		{ClassifyMethod("unknown"), "?"},
	}

	for _, tt := range tests {
		t.Run(string(tt.method), func(t *testing.T) {
			if got := tt.method.ShortString(); got != tt.want {
				t.Errorf("ClassifyMethod.ShortString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCommitClassification_IsHighConfidence(t *testing.T) {
	tests := []struct {
		name       string
		confidence float64
		threshold  float64
		want       bool
	}{
		{"above threshold", 0.9, 0.8, true},
		{"at threshold", 0.8, 0.8, true},
		{"below threshold", 0.7, 0.8, false},
		{"zero confidence", 0.0, 0.5, false},
		{"perfect confidence", 1.0, 0.5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &CommitClassification{
				Confidence: tt.confidence,
			}
			if got := c.IsHighConfidence(tt.threshold); got != tt.want {
				t.Errorf("CommitClassification.IsHighConfidence(%v) = %v, want %v",
					tt.threshold, got, tt.want)
			}
		})
	}
}

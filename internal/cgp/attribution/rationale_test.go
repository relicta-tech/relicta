package attribution

import (
	"testing"
	"time"
)

func TestRationaleBuilder(t *testing.T) {
	rationale := NewRationale(RationaleTypeAnalysis, "Detected breaking API changes").
		WithConfidence(0.85).
		WithDetail("changedSymbols", 3).
		AddFileSource("api/users.go", "Modified UserService interface").
		AddCommitSource("abc123", "Introduced new parameter").
		Build()

	if rationale.Type != RationaleTypeAnalysis {
		t.Errorf("Type = %s, want %s", rationale.Type, RationaleTypeAnalysis)
	}
	if rationale.Summary != "Detected breaking API changes" {
		t.Errorf("Summary = %s, want 'Detected breaking API changes'", rationale.Summary)
	}
	if rationale.Confidence != 0.85 {
		t.Errorf("Confidence = %f, want 0.85", rationale.Confidence)
	}
	if rationale.Details["changedSymbols"] != 3 {
		t.Errorf("Details[changedSymbols] = %v, want 3", rationale.Details["changedSymbols"])
	}
	if len(rationale.Sources) != 2 {
		t.Errorf("Sources count = %d, want 2", len(rationale.Sources))
	}
	if rationale.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}

func TestRationale_ConfidenceLevels(t *testing.T) {
	tests := []struct {
		confidence float64
		wantHigh   bool
		wantMed    bool
		wantLow    bool
	}{
		{0.9, true, false, false},
		{0.8, true, false, false},
		{0.7, false, true, false},
		{0.5, false, true, false},
		{0.4, false, false, true},
		{0.1, false, false, true},
	}

	for _, tt := range tests {
		r := NewRationale(RationaleTypeAnalysis, "test").
			WithConfidence(tt.confidence).
			Build()

		if r.IsHighConfidence() != tt.wantHigh {
			t.Errorf("IsHighConfidence() for %f = %v, want %v", tt.confidence, r.IsHighConfidence(), tt.wantHigh)
		}
		if r.IsMediumConfidence() != tt.wantMed {
			t.Errorf("IsMediumConfidence() for %f = %v, want %v", tt.confidence, r.IsMediumConfidence(), tt.wantMed)
		}
		if r.IsLowConfidence() != tt.wantLow {
			t.Errorf("IsLowConfidence() for %f = %v, want %v", tt.confidence, r.IsLowConfidence(), tt.wantLow)
		}
	}
}

func TestRationale_ConfidenceClamping(t *testing.T) {
	// Test that confidence is clamped to [0, 1]
	r1 := NewRationale(RationaleTypeAnalysis, "test").WithConfidence(-0.5).Build()
	if r1.Confidence != 0 {
		t.Errorf("Negative confidence should be clamped to 0, got %f", r1.Confidence)
	}

	r2 := NewRationale(RationaleTypeAnalysis, "test").WithConfidence(1.5).Build()
	if r2.Confidence != 1 {
		t.Errorf("Confidence > 1 should be clamped to 1, got %f", r2.Confidence)
	}
}

func TestRationale_SourcesByType(t *testing.T) {
	rationale := NewRationale(RationaleTypeHistory, "Based on historical patterns").
		AddFileSource("src/main.go", "Main entry point").
		AddFileSource("src/utils.go", "Utility functions").
		AddCommitSource("def456", "Recent refactor").
		AddURLSource("https://docs.example.com", "API documentation").
		Build()

	fileSources := rationale.SourcesByType("file")
	if len(fileSources) != 2 {
		t.Errorf("SourcesByType(file) returned %d, want 2", len(fileSources))
	}

	commitSources := rationale.SourcesByType("commit")
	if len(commitSources) != 1 {
		t.Errorf("SourcesByType(commit) returned %d, want 1", len(commitSources))
	}

	urlSources := rationale.SourcesByType("url")
	if len(urlSources) != 1 {
		t.Errorf("SourcesByType(url) returned %d, want 1", len(urlSources))
	}

	nonExistent := rationale.SourcesByType("nonexistent")
	if len(nonExistent) != 0 {
		t.Errorf("SourcesByType(nonexistent) returned %d, want 0", len(nonExistent))
	}
}

func TestRationale_HasSources(t *testing.T) {
	withSources := NewRationale(RationaleTypeAnalysis, "test").
		AddFileSource("file.go", "context").
		Build()

	if !withSources.HasSources() {
		t.Error("HasSources() should return true when sources exist")
	}

	withoutSources := NewRationale(RationaleTypeAnalysis, "test").Build()
	if withoutSources.HasSources() {
		t.Error("HasSources() should return false when no sources")
	}
}

func TestRationale_WithTimestamp(t *testing.T) {
	customTime := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	rationale := NewRationale(RationaleTypeUserRequest, "User requested feature").
		WithTimestamp(customTime).
		Build()

	if !rationale.Timestamp.Equal(customTime) {
		t.Errorf("Timestamp = %v, want %v", rationale.Timestamp, customTime)
	}
}

func TestRationaleTypes(t *testing.T) {
	types := []RationaleType{
		RationaleTypeAnalysis,
		RationaleTypeConvention,
		RationaleTypeUserRequest,
		RationaleTypeAutomation,
		RationaleTypeHistory,
		RationaleTypeDocumentation,
	}

	for _, rt := range types {
		rationale := NewRationale(rt, "test").Build()
		if rationale.Type != rt {
			t.Errorf("Type = %s, want %s", rationale.Type, rt)
		}
	}
}

func TestRationale_AllSourceTypes(t *testing.T) {
	rationale := NewRationale(RationaleTypeAnalysis, "comprehensive test").
		AddFileSource("path/to/file.go", "file context").
		AddCommitSource("abc123", "commit context").
		AddDocSource("README.md", "doc context").
		AddURLSource("https://example.com", "url context").
		AddConversationSource("session-123", "conversation context").
		Build()

	if len(rationale.Sources) != 5 {
		t.Errorf("Expected 5 sources, got %d", len(rationale.Sources))
	}

	expectedTypes := map[string]bool{
		"file":          false,
		"commit":        false,
		"documentation": false,
		"url":           false,
		"conversation":  false,
	}

	for _, s := range rationale.Sources {
		expectedTypes[s.Type] = true
	}

	for st, found := range expectedTypes {
		if !found {
			t.Errorf("Source type %s not found", st)
		}
	}
}

func TestRationaleBuilder_WithDetails(t *testing.T) {
	details := map[string]any{"key1": "val1", "key2": 42}
	r := NewRationale("test", "conclusion").
		WithDetails(details).
		Build()
	if r.Details["key1"] != "val1" {
		t.Errorf("Details[key1] = %v, want val1", r.Details["key1"])
	}
	if r.Details["key2"] != 42 {
		t.Errorf("Details[key2] = %v, want 42", r.Details["key2"])
	}
}

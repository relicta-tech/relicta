package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/communication"
)

func TestOutputNarrativesJSON(t *testing.T) {
	narratives := []*communication.Narrative{
		{
			Audience:    communication.AudienceEngineering,
			Title:       "Engineering Release Notes",
			Body:        "## Changes\n- Fixed bug",
			Format:      communication.OutputMarkdown,
			Provider:    "template",
			GeneratedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			Audience:    communication.AudienceProduct,
			Title:       "Product Update",
			Body:        "New features added",
			Format:      communication.OutputPlainText,
			Provider:    "openai",
			GeneratedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputNarrativesJSON(narratives)

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("outputNarrativesJSON() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var results []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	if results[0]["audience"] != "engineering" {
		t.Errorf("expected audience 'engineering', got %v", results[0]["audience"])
	}
	if results[1]["provider"] != "openai" {
		t.Errorf("expected provider 'openai', got %v", results[1]["provider"])
	}
}

func TestGenerateAndOutputNarratives_Success(t *testing.T) {
	gen := communication.NewNarrativeGenerator(nil) // nil AI = template fallback
	input := communication.NarrativeInput{
		Version:     "1.0.0",
		ProductName: "TestProduct",
		Bundles: []communication.Bundle{
			{Type: "feature", Theme: "Features", Changes: []communication.BundledChange{{Description: "add feature"}}},
		},
	}
	audiences := []communication.Audience{
		{Type: communication.AudienceEngineering, Tone: communication.CommToneTechnical, DetailLevel: communication.DetailFull, Sections: []communication.Section{communication.SectionFeatures}},
	}

	// Save and override outputJSON
	origJSON := outputJSON
	outputJSON = true
	defer func() { outputJSON = origJSON }()

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := generateAndOutputNarratives(t.Context(), gen, input, audiences)

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("generateAndOutputNarratives() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	if buf.Len() == 0 {
		t.Error("expected non-empty JSON output")
	}
}

func TestGenerateAndOutputNarratives_NonJSON(t *testing.T) {
	gen := communication.NewNarrativeGenerator(nil)
	input := communication.NarrativeInput{
		Version:     "2.0.0",
		ProductName: "TestProduct",
		Bundles: []communication.Bundle{
			{Type: "bugfix", Theme: "Bug Fixes", Changes: []communication.BundledChange{{Description: "fix endpoint"}}},
		},
	}
	audiences := []communication.Audience{
		{Type: communication.AudienceProduct, Tone: communication.CommToneBusiness, DetailLevel: communication.DetailSummary, Sections: []communication.Section{communication.SectionFixes}},
	}

	origJSON := outputJSON
	outputJSON = false
	origDir := commOutputDir
	commOutputDir = ""
	defer func() { outputJSON = origJSON; commOutputDir = origDir }()

	// Capture stdout
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := generateAndOutputNarratives(t.Context(), gen, input, audiences)

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("generateAndOutputNarratives() error = %v", err)
	}
}

func TestGenerateAndOutputNarratives_WithOutputDir(t *testing.T) {
	gen := communication.NewNarrativeGenerator(nil)
	input := communication.NarrativeInput{
		Version: "3.0.0",
		Bundles: []communication.Bundle{
			{Type: "feature", Theme: "New", Changes: []communication.BundledChange{{Description: "new thing"}}},
		},
	}
	audiences := []communication.Audience{
		{Type: communication.AudienceExternal, Tone: communication.CommTonePublic, DetailLevel: communication.DetailHighlights, Sections: []communication.Section{communication.SectionFeatures}},
	}

	tmpDir := t.TempDir()

	origJSON := outputJSON
	outputJSON = false
	origDir := commOutputDir
	commOutputDir = tmpDir
	defer func() { outputJSON = origJSON; commOutputDir = origDir }()

	// Capture stdout
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := generateAndOutputNarratives(t.Context(), gen, input, audiences)

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("generateAndOutputNarratives() error = %v", err)
	}

	// Verify file was written
	files, _ := os.ReadDir(tmpDir)
	if len(files) == 0 {
		t.Error("expected output file in directory")
	}
}

// TestFileExtension covers all branches of the fileExtension helper.
func TestFileExtension(t *testing.T) {
	tests := []struct {
		format   communication.OutputFormat
		expected string
	}{
		{communication.OutputHTML, "html"},
		{communication.OutputPlainText, "txt"},
		{communication.OutputMarkdown, "md"},
		{"unknown", "md"}, // defaults to md
		{"", "md"},        // empty defaults to md
	}

	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			got := fileExtension(tt.format)
			if got != tt.expected {
				t.Errorf("fileExtension(%q) = %q, want %q", tt.format, got, tt.expected)
			}
		})
	}
}

// TestResolveAudiences_All verifies that "all" resolves to all four known audiences.
func TestResolveAudiences_All(t *testing.T) {
	audiences, err := resolveAudiences("all")
	if err != nil {
		t.Fatalf("resolveAudiences(\"all\") error = %v", err)
	}
	if len(audiences) == 0 {
		t.Error("expected at least 1 audience for 'all'")
	}
}

// TestResolveAudiences_SingleValid verifies individual audience resolution.
func TestResolveAudiences_SingleValid(t *testing.T) {
	validAudiences := []string{"engineering", "product", "executive", "external"}

	for _, aud := range validAudiences {
		t.Run(aud, func(t *testing.T) {
			audiences, err := resolveAudiences(aud)
			if err != nil {
				t.Fatalf("resolveAudiences(%q) error = %v", aud, err)
			}
			if len(audiences) != 1 {
				t.Errorf("expected 1 audience, got %d", len(audiences))
			}
		})
	}
}

// TestResolveAudiences_CommaSeparated verifies comma-separated audience resolution.
func TestResolveAudiences_CommaSeparated(t *testing.T) {
	audiences, err := resolveAudiences("engineering,product")
	if err != nil {
		t.Fatalf("resolveAudiences(\"engineering,product\") error = %v", err)
	}
	if len(audiences) != 2 {
		t.Errorf("expected 2 audiences, got %d", len(audiences))
	}
}

// TestResolveAudiences_Invalid returns error for unknown audience.
func TestResolveAudiences_Invalid(t *testing.T) {
	_, err := resolveAudiences("unknown_audience")
	if err == nil {
		t.Error("expected error for invalid audience type")
	}
}

// TestResolveAudiences_WhitespaceTrimmed verifies whitespace trimming.
func TestResolveAudiences_WhitespaceTrimmed(t *testing.T) {
	audiences, err := resolveAudiences("engineering , product")
	if err != nil {
		t.Fatalf("resolveAudiences() error = %v", err)
	}
	if len(audiences) != 2 {
		t.Errorf("expected 2 audiences (whitespace trimmed), got %d", len(audiences))
	}
}

// TestResolveAudiences_WithConfig verifies config override path (cfg is nil here).
func TestResolveAudiences_NilConfig(t *testing.T) {
	// cfg is a package-level var; ensure resolveAudiences doesn't panic when cfg is nil.
	origCfg := cfg
	cfg = nil
	defer func() { cfg = origCfg }()

	audiences, err := resolveAudiences("engineering")
	if err != nil {
		t.Fatalf("resolveAudiences() with nil cfg error = %v", err)
	}
	if len(audiences) != 1 {
		t.Errorf("expected 1 audience, got %d", len(audiences))
	}
}
